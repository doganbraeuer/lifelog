# Test server

How the test server for real testers is set up and run (since 2026-11-22).
No secrets in this file: the repository is public. Access data lives in the
team's password manager only.

## Overview

| Part | What |
|---|---|
| Machine | netcup VPS 1000 G12.5 (4 vCPU AMD EPYC Rome, 8 GB RAM, 128 GB SSD), location EU, monthly contract; data processing agreement (Art. 28 GDPR) signed in the netcup CCP |
| System | Debian 13, automatic security updates (`unattended-upgrades`), 2 GB swap |
| Address | `https://lifelog-test.duckdns.org` (DuckDNS, A and AAAA record) |
| Database | MongoDB Atlas, own project `lifelog-test`, M0 cluster in Frankfurt, database `lifelog_test`; user `lifelog-server` with `readWrite` on that database only; network access only from the server's IPv4 address |
| Model | Gemini API, paid tier (Google does not use the content to improve its products), spend cap 20 € per month; key restricted to the Gemini API and to the server's IPv4 and IPv6 address |

```
App ──HTTPS──▶ Caddy :443 ──▶ lifelog-server 127.0.0.1:8080 ──▶ MongoDB Atlas (TLS)
                (Let's Encrypt,                              └──▶ Gemini API (TLS)
                 HSTS)                                         └──▶ tesseract, pdftotext, ffmpeg (local)
```

## Security measures

- **SSH:** key only (`PasswordAuthentication no`, `KbdInteractiveAuthentication no`), no root login, `AllowUsers` whitelist; file `/etc/ssh/sshd_config.d/10-lifelog-hardening.conf` (read before other drop-ins, the first value wins).
- **Firewall (ufw):** incoming denied except 22 (rate limit: 6 connections in 30 s per address), 80 (Let's Encrypt, redirect) and 443; IPv4 and IPv6.
- **Services** run as the system user `lifelog` (no login shell) with systemd sandboxing: `ProtectSystem=strict`, `ProtectHome`, `PrivateTmp`, `PrivateDevices`, `NoNewPrivileges`, restricted address families. Programs and models belong to root; `lifelog` can run them but not change them.
- **Secrets** in `/etc/lifelog/lifelog.env`, owner root, mode 600. systemd reads it as root (`EnvironmentFile=`) and hands the values to the process; the service user cannot read the file.
- **Backend** listens on 127.0.0.1 only; Caddy is the only way in. `TRUSTED_PROXIES=127.0.0.1,::1`, so the failed-login lock counts the real client address (`x_forwarded_for=true` in the log).
- **Caddy** sets `Strict-Transport-Security`, `X-Content-Type-Options: nosniff`, `Referrer-Policy: no-referrer`, removes the `Server` header and refuses request bodies over 30 MB.

## Files

| Path | What |
|---|---|
| `/opt/lifelog/bin/lifelog-server`, `lifelog-users`, `lifelog-speechcheck` | the programs, built on a Mac (below) |
| `/etc/lifelog/lifelog.env` | configuration and secrets (root, 600) |
| `/etc/systemd/system/lifelog.service` | the backend service |
| `/etc/caddy/Caddyfile` | reverse proxy and TLS |
| `/usr/local/bin/lifelog-users` | wrapper: runs `lifelog-users` with the configuration, as the calling user (audit log) |
| `/etc/systemd/system/whisper.service`, `/opt/lifelog/whisper/` | whisper-server, **disabled**: too slow without a GPU (docs/speech/README.md) |

Configuration in `lifelog.env` (values in the password manager):

```
MONGODB_URI=…
MONGODB_DB=lifelog_test
GEMINI_API_KEY=…
GEMINI_MODEL=gemini-3.1-flash-lite
GEMINI_FALLBACK_MODEL=gemini-3.8-flash
SPEECH_ENGINE=gemini
ADDRESS=127.0.0.1
PORT=8080
TRUSTED_PROXIES=127.0.0.1,::1
```

`GEMINI_MODEL` is Flash-Lite since 2026-11-22: that evening gemini-3.8-flash
answered 503, gemini-3.6-flash took 49 s and gemini-3.5-flash did not answer
within 60 s for the prompt „Antworte nur mit OK.“, while Flash-Lite took 4 s.
Reading with Flash-Lite was checked on two demo documents (5 of 5 and 4 of 4
values confirmed by OCR). To be measured properly with `cmd/calibrate`.

## Everyday tasks

Logs (live, Ctrl-C to stop; HTTP requests from scanners answered 401 are normal):

```sh
sudo journalctl -u lifelog -n 50 -f
sudo journalctl -u caddy -n 50 --no-pager
```

Accounts (the wrapper needs `sudo`; passwords are shown once, straight into the password manager):

```sh
sudo lifelog-users list
sudo lifelog-users add-batch --prefix tester --count 5
sudo lifelog-users reset-password --username tester01
sudo lifelog-users disable --username tester01
sudo lifelog-users delete --username tester05      # with all its data
```

Change the configuration without showing secrets:

```sh
sudo nano /etc/lifelog/lifelog.env                 # paste secrets in the editor, never with echo
sudo awk -F= '{print $1, "=", length($0)-length($1)-1, "chars"}' /etc/lifelog/lifelog.env
sudo systemctl restart lifelog
```

Which Gemini models answer right now, with the server key:

```sh
sudo bash -c 'set -a; . /etc/lifelog/lifelog.env; for m in gemini-3.8-flash gemini-3.6-flash gemini-3.1-flash-lite; do curl -s -o /dev/null --max-time 60 -w "$m HTTP %{http_code} %{time_total}s\n" -H "x-goog-api-key: $GEMINI_API_KEY" -H "Content-Type: application/json" -d "{\"contents\":[{\"parts\":[{\"text\":\"Antworte nur mit OK.\"}]}]}" https://generativelanguage.googleapis.com/v1beta/models/$m:generateContent; done'
```

## Deploying a new backend

On the Mac, from `backend/` on the branch to deploy:

```sh
mkdir -p /tmp/lifelog-build
for cmd in server users speechcheck; do
  GOOS=linux GOARCH=amd64 CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" \
    -o /tmp/lifelog-build/lifelog-$cmd ./cmd/$cmd
done
shasum -a 256 /tmp/lifelog-build/*
scp /tmp/lifelog-build/lifelog-* lifelog:/tmp/        # Host "lifelog" in ~/.ssh/config
```

On the server:

```sh
sha256sum /tmp/lifelog-*                              # must match the Mac
sudo install -o root -g root -m 755 /tmp/lifelog-server /tmp/lifelog-users /tmp/lifelog-speechcheck /opt/lifelog/bin/
rm /tmp/lifelog-*
sudo systemctl restart lifelog
curl -s http://127.0.0.1:8080/api/v1/status; echo
```

The app is built with the server address:

```sh
cd app && flutter build apk --release --dart-define=API_BASE_URL=https://lifelog-test.duckdns.org
```

## Security check before testers get access

- [ ] `ssh root@…` and `ssh -o PubkeyAuthentication=no …` are refused
- [ ] `sudo ufw status verbose`: only 22 (LIMIT), 80, 443
- [ ] `ss -tlnp`: 8080 only on 127.0.0.1, nothing else open to the outside
- [ ] `sudo ls -l /etc/lifelog/lifelog.env`: `-rw------- root root`
- [ ] `curl -sI https://lifelog-test.duckdns.org/api/v1/status`: HSTS, no `Server` header
- [ ] Every endpoint except `/status` and `/auth/login` answers 401 without a token
- [ ] Two accounts: neither sees, opens, changes or deletes the other's entries (IDOR, 404)
- [ ] Login lock after repeated wrong passwords works behind Caddy
- [ ] The log contains no texts, transcripts, tokens or keys
- [ ] Atlas: network access only the server address; user only `readWrite` on `lifelog_test`
- [ ] Gemini key: only the Gemini API, only the server addresses; spend cap set
