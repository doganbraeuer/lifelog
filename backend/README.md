# Lifelog backend (Go)

REST API as specified in [docs/API.md](../docs/API.md). Names follow
[docs/GLOSSARY.md](../docs/GLOSSARY.md).

## Run

1. Create `.env` in the repository root (template: `.env.example`).
2. In the folder `backend/`:

   ```sh
   go run ./cmd/server
   curl localhost:8080/api/v1/status
   ```

By default the server only listens on `127.0.0.1:8080`. The Android emulator
reaches it via `http://10.0.2.2:8080`. For a real phone on the Wi-Fi:
`ADDRESS=0.0.0.0 go run ./cmd/server` and use the computer's IP address. The
backend is then reachable from the whole network, so only do this for
testing.

`/status` is the only endpoint besides the login that works without a
token. Everything else needs `Authorization: Bearer <token>`, see
[accounts and login](#accounts-and-login).

Manual upload (synthetic documents only; `TOKEN` from the login, see
below):

```sh
curl -H "Authorization: Bearer $TOKEN" -F file=@testdata/lab_report.pdf localhost:8080/api/v1/entries   # file
curl -H "Authorization: Bearer $TOKEN" -F "text=<mytext.txt" localhost:8080/api/v1/entries              # text import
```

For the text import, `curl` reads the file content with `<` into the form
field `text` (not as a file attachment). It is stored as `text.txt`.

## Accounts and login

Since 2026-11-18 every endpoint except `GET /api/v1/status` and
`POST /api/v1/auth/login` needs a token (docs/API.md, section 1 and
3.10–3.15). **The app needs a login from now on:** until the app sends a
token, the current app gets `401` for everything except `/status`.

How it works:

- Accounts are created only with `cmd/users`; there is no registration.
  Passwords are stored as bcrypt hashes (cost 12), at least 10 characters
  and at most 72 bytes.
- A login returns a token (32 random bytes) exactly once. The database
  keeps only its SHA-256 hash in the collection `sessions`; a TTL index
  deletes sessions 30 days after the login.
- The middleware (`internal/api/auth.go`) checks every request: token
  hash, expiry, account exists and is not disabled. It locks by default: a
  new route needs a token unless it is added to the two public ones. There
  is no switch to turn login off, not even for development.
- An account with a start password (`mustChangePassword`) can only call
  `GET /me`, `POST /me/password` and `POST /auth/logout` until the password
  is changed; everything else is `403 PASSWORD_CHANGE_REQUIRED`.
- The log never contains passwords, tokens, the `Authorization` header or
  the username of a failed login, only user IDs.

### The first admin account

Run `cmd/users` where the backend runs, in its `backend/` folder: it uses
the same `.env` (`MONGODB_URI`, `MONGODB_DB`) as the server, so the account
lands in the server's database. On the test server that is the server's
`.env`, not the one on your own computer.

```sh
go run ./cmd/users add --username admin1 --role admin
```

This prints a generated password of 24 characters **once**; put it into a
password manager right away. An admin has no forced password change. To
choose the password yourself, give it on standard input, never as an
argument (it would end up in the shell history):

```sh
# Git Bash / macOS / Linux: read -s does not show the input
read -rs PW && printf '%s\n' "$PW" | go run ./cmd/users add --username admin1 --role admin --password-stdin; unset PW
```

```powershell
# PowerShell: hidden input; UTF-8 so that umlauts survive the pipe
$OutputEncoding = [Text.UTF8Encoding]::new($false)
$p = Read-Host -AsSecureString
[Net.NetworkCredential]::new('', $p).Password | go run ./cmd/users add --username admin1 --role admin --password-stdin
Remove-Variable p
```

### Test accounts

```sh
go run ./cmd/users add-batch --prefix tester --count 10
```

creates `tester01` … `tester10` and prints one list of usernames and start
passwords (12 characters, without the easily confused 0, O, 1 and l),
**only this once**. Give every tester only their own line. At the first
login the app gets `mustChangePassword: true` and asks for a new password.
A single account: `go run ./cmd/users add --username tester11
[--display-name Erika]`.

### Managing accounts

```sh
go run ./cmd/users list                                 # username, displayName, role, disabled, mustChangePassword, entries, createdAt (never hashes)
go run ./cmd/users reset-password --username tester03   # new start password (admin: 24 characters), ends all sessions
go run ./cmd/users disable --username tester03          # locked at once: every session ends, login refused
go run ./cmd/users enable --username tester03
go run ./cmd/users revoke-sessions --username tester03  # log out every device of one account
go run ./cmd/users revoke-sessions --all                # log out everybody
go run ./cmd/users delete --username tester03           # the account, its sessions and ALL its data
go run ./cmd/users assign-orphans --username admin2     # give data without an owner to this account (see below)
```

Every action writes an entry to the collection `auditLog`: time, action,
target username and operator (operating system user and computer name),
never passwords. `delete` removes the account with its sessions and all
its data: entries, chunks, OCR words, original files and daily counters.
It then counts what is left and only deletes the account itself if
nothing is; it prints the numbers and "remaining: 0".

### Per-user data

Since 2026-11-18 every entry belongs to the account that uploaded it
(docs/API.md, section 1 "Per-user data"). The owner `userId` is stored on
the entry, its chunks, its OCR words and in the metadata of its original
file. Every store function that reads, changes or deletes them takes the
user as a required parameter and filters by it; an empty user is an error,
never "all users". Another user's entry answers 404, questions only search
the user's own documents (the vector search filters by `userId` and
`module`), and a duplicate only counts within the own account. Admins have
no additional rights through the API.

**Data from before 2026-11-18 has no owner and is visible to nobody**
until an administrator gives it to an account:

```sh
go run ./cmd/users assign-orphans --username admin2
```

It gives every entry, chunk, OCR document and original without an owner to
this account and prints the numbers; run it a second time to check that it
shows 0 everywhere. Data that already has an owner is never touched.

> **Dogan:** run this once for your database `lifelog` with your own
> account (in the `backend/` folder with your `.env`), otherwise you no
> longer see your demo data in the app:
> `go run ./cmd/users assign-orphans --username <your account>`.
> `go run ./cmd/users list` then shows the number of entries per account.

### Daily limits

Each user may make a limited number of costly requests per calendar day
(Europe/Vienna, the day starts at 00:00 local time); see docs/API.md,
"Daily limits". Admins have the same limits.

| Limit | Requests | Default | Variable |
|---|---|---|---|
| `uploads` | `POST /entries` | 30 | `LIMIT_UPLOADS_PER_DAY` |
| `questions` | `POST /questions` | 60 | `LIMIT_QUESTIONS_PER_DAY` |
| `transcriptions` | `POST /transcriptions` | 30 | `LIMIT_TRANSCRIPTIONS_PER_DAY` |

A request counts once its input has been checked and before any model is
called; whether it succeeds afterwards does not matter. A duplicate upload
does not count. At the limit the answer is `429 LIMIT_REACHED`. The
counters are in the collection `usage` (one document per user, day and
limit, counted atomically), so a restart does not reset them; MongoDB
deletes old counters two days after their day ends.

### Failed attempts

5 failed logins for the same username from the same IP address within 15
minutes lock this pair for 1 minute; every further lock doubles the time
(2, 4, 8, at most 15 minutes). 30 failed attempts from one IP address lock
it for 15 minutes. A wrong current password at `POST /me/password` counts
too. Someone guessing from another address cannot lock out a tester. The
counters live in memory: **a restart of the backend resets them.**

### Behind a reverse proxy (Caddy) and HTTPS

On the server on the internet, tokens and passwords may only travel over
HTTPS. The backend itself speaks plain HTTP and stays on `127.0.0.1`
(`ADDRESS`, the default); Caddy in front of it gets a certificate and
forwards the requests:

```text
lifelog.example.org {
    reverse_proxy 127.0.0.1:8080
}
```

Behind Caddy every request comes from `127.0.0.1`. So that the limits
count the real clients and not one shared address, set in the server's
`.env`:

```sh
TRUSTED_PROXIES=127.0.0.1,::1
```

The backend then takes the client address from `X-Forwarded-For`, which
Caddy writes; without `TRUSTED_PROXIES` that header is ignored, because
every client could send it. On start the server logs
`x_forwarded_for=true` or `false`.

### Trying it with curl

```sh
TOKEN=$(printf '{"username":"%s","password":"%s"}' "tester01" "$PW" \
  | curl -s -H 'Content-Type: application/json' --data-binary @- localhost:8080/api/v1/auth/login \
  | sed -n 's/.*"token":"\([^"]*\)".*/\1/p')
curl -H "Authorization: Bearer $TOKEN" localhost:8080/api/v1/me
curl -X POST -H "Authorization: Bearer $TOKEN" localhost:8080/api/v1/auth/logout
```

(`PW` read with `read -rs PW` as above.)

## Test

```sh
gofmt -l . && go vet ./... && go test ./...
```

Live tests against Gemini (cost requests from the quota, send only the
synthetic files from `testdata/`). They also check that the full text stays
German and that answers are German with references:

```sh
LIFELOG_LIVE=1 go test ./internal/llm/gemini/ -run Live -v
```

Integration tests against MongoDB (use a database of their own,
`<MONGODB_DB>_integration_test`, and drop it afterwards):

```sh
LIFELOG_MONGO_TEST=1 go test ./internal/store/ -run Integration -v
```

## Gemini models

```sh
go run ./cmd/models   # lists the models your own key can see
```

The key comes from `.env` and is never printed. Whether a model can be used
in the free tier and which limits apply is only shown in AI Studio or on the
pricing page. On 429/503 the backend retries up to twice; with
`GEMINI_FALLBACK_MODEL` it then tries another model once for reading. For
that, 20 s of the read time limit (50 s) are reserved by default
(`GEMINI_FALLBACK_RESERVE_SECONDS`); the main model gets 30 s.

## Verification (text layer and OCR)

A second reader checks the values the vision LLM extracted
(docs/API.md, section 2, verification). It runs locally, with open source
programs, and costs no model call:

```sh
brew install poppler tesseract tesseract-lang          # macOS
sudo apt install poppler-utils tesseract-ocr tesseract-ocr-deu   # Debian/Ubuntu
```

- PDF pages with a text layer: `pdftotext -bbox-layout` (Poppler).
- PDF pages without a text layer: `pdftoppm` renders the page, Tesseract
  (`TESSERACT_LANG`, default `deu`) reads it.
- Photos (JPEG, PNG, HEIC, WebP) are prepared in Go first
  (`ocr.PrepareImage`): turned upright (EXIF orientation, HEIC rotation),
  scaled to a long side between 2500 (at most twice the size) and 4096
  pixels, and handed to Tesseract as an RGB PNG with `--dpi 300`. Images
  over 50 megapixels are not decoded. HEIC is decoded by
  `github.com/gen2brain/heic` (a decoder compiled to WebAssembly, no cgo,
  nothing to install). The settings were chosen by measurement
  (`docs/ocr/README.md`).

It runs at the same time as the vision LLM (limit 30 s), so an upload does
not take longer. The words with their positions are stored in the
collection `ocr`; PATCH checks corrected values against them. If a program
is missing, the server still starts and logs a warning; the affected
entries are stored `unchecked` with `reason: ocr_unavailable`. `OCR_ENGINE=none`
switches the check off. The tests that need the programs skip themselves if
they are missing.

```sh
go run ./cmd/ocrcheck            # show what it would fill in for old entries, and measure demo/
go run ./cmd/ocrcheck -write     # store the verification of entries that were never checked
go run ./cmd/ocrcheck -recheck   # show what checking every entry again would change (+ -write: store it)
go run ./cmd/ocrcheck -values -out ../docs/ocr/<date>-tesseract.md   # measurement report
go run ./cmd/ocrcheck -upscale 3000                    # measure another scaling (0 = off)
go run ./cmd/ocrcheck -downscale 3000                  # measure scaling large photos down (0 = off)
go run ./cmd/ocrcheck -extra ~/Desktop/01_lab_report_2026-01-20_phone.jpg   # a real photo
```

`cmd/ocrcheck` calls no model. Results and their assessment: `docs/ocr/`.

## Speech (transcription)

`POST /api/v1/transcriptions` turns a spoken note into text
(docs/API.md, 3.9). It runs locally, costs no model call and stores
nothing. Two programs are needed:

- **ffmpeg** converts the recording (M4A from the phone, WAV, OGG, …) to
  16 kHz mono PCM. The backend calls it like Tesseract.
- **whisper.cpp** recognizes the speech. It runs as **its own server**
  (`whisper-server`), so the model (1.6 GB) is loaded once and not for
  every recording. It listens only on `127.0.0.1:8178`.

**macOS** (whisper.cpp 1.9.5 via Homebrew, uses the GPU through Metal):

```sh
brew install whisper-cpp ffmpeg
backend/scripts/whisper-server.sh download   # models to ~/.local/share/lifelog/whisper, SHA-256 checked
backend/scripts/whisper-server.sh            # start; keep this terminal open
```

**Windows** (not tested by us):

```powershell
winget install Gyan.FFmpeg
# whisper.cpp: download whisper-bin-x64.zip (or the CUDA build for an NVIDIA
# card) from https://github.com/ggml-org/whisper.cpp/releases, unpack it and
# put the folder with whisper-server.exe into PATH (or set WHISPER_SERVER).
powershell -File backend\scripts\whisper-server.ps1 download
powershell -File backend\scripts\whisper-server.ps1
```

Without a graphics card large-v3-turbo is slow on Windows. The quantized
`ggml-large-v3-turbo-q5_0.bin` (about 0.55 GB, same Hugging Face folder) is
faster; it is not measured yet and the scripts do not download it.

The models (`ggml-large-v3-turbo.bin`, `ggml-silero-v5.1.2.bin` for the
voice activity detection) stay outside the repository.

What the server is started with (scripts): German only (`--language de`),
no temperature fallback (`--no-fallback`), non-speech tokens suppressed
(`--suppress-nst`), voice activity detection (`--vad`), and the prompt kept
for every 30-second window (`--carry-initial-prompt`). The backend sends a
list of medical terms as prompt with every recording
(`internal/speech/whisper/vocabulary.go`), so Whisper writes „1-0-1“
instead of „101“ and knows terms like „Gonarthrose“. `--max-context 0`
would switch that prompt off; see `docs/speech/README.md` for the
measurement. The backend adds its own checks: recordings with less than 0.3 s of sound are
answered with `NO_SPEECH` without asking Whisper, and known invented
sentences ("Untertitel im Auftrag des ZDF") are removed.

Privacy: the recording stays in memory, except for one temporary file for
ffmpeg (mode 0600, deleted right after, also on errors). whisper-server
gets the fixed file name `audio.wav` (it logs the file name, not the
text). Our log has size, length and result, never the text.

If ffmpeg is missing or whisper-server is not running, the backend still
starts, `/status` says `"speech": "unavailable"` and transcriptions answer
503. `SPEECH_ENGINE=none` switches it off (`"speech": "off"`).

The start scripts keep whisper-server running: after the start they send
`scripts/warmup.wav` once (a synthetic voice: „Guten Tag, das ist ein
kurzer Test.“), and if whisper-server stops, they start it again after 2 s
(up to 60 s if it keeps stopping) and warm it up again. Ctrl-C stops both.
While it is down, the backend answers as above and is back to `"ok"` by
itself; a transcription that is running when it crashes gets 503. The
Windows script needs `curl.exe` (part of Windows 10 and 11).

Known bug of whisper.cpp 1.9.5 (issue
[#4156](https://github.com/ggml-org/whisper.cpp/issues/4156), fixed in
[#4158](https://github.com/ggml-org/whisper.cpp/pull/4158) after 1.9.5): a
freshly started whisper-server crashes when the first recording has no
speech, because `verbose_json` runs a language detection on every request.
The backend sends `no_language_probabilities=true`, which avoids the crash
and saves about 0.6 s per recording (we fix the language anyway); the
warm-up in the scripts protects other clients. Both can stay after an
update.

```sh
LIFELOG_LIVE=1 go test ./internal/speech/whisper -run Live -v   # silence against the running whisper-server
```

Measure recognition, speed and silence on known sentences (results and
evaluation in `docs/speech/README.md`):

```sh
go run ./cmd/speechcheck                                       # synthetic voice and generated audio
go run ./cmd/speechcheck -own ~/Projekte/lifelog-demo/speech   # plus own recordings 01.m4a … 10.m4a
```

## Vector index

Search (`POST /api/v1/questions`) needs the Atlas Vector Search index
`chunks_vector` on the collection `chunks`. The definition is in
[`internal/store/chunks_vector.json`](internal/store/chunks_vector.json):
`embedding` as vector (768 dimensions, cosine), `userId` and `module` as
filter fields (`userId` since 2026-11-18).

**Option A (automatic):** the server creates the index on start if it is
missing, and **updates** it if it lacks a filter field (an index from
before 2026-11-18 has no `userId`); it logs this once. Atlas builds a new
or updated index in the background (about 30 s to a few minutes in our
tests). **Questions only work again once the index is `READY`:** until
then `POST /questions` answers `503 MODEL_UNAVAILABLE` with „Die Suche wird
gerade aktualisiert. Bitte versuche es in ein paar Minuten erneut.“, it does
not just find nothing. The server asks Atlas every 15 s and logs
`vector index READY, questions work again`. Upload, timeline and everything
else work normally meanwhile. Creating was tested on the free tier (M0) on
2026-11-04, updating on 2026-11-18.

While Atlas rebuilds, it still serves the old definition, and a search
with the `userId` filter fails ("Path 'userId' needs to be indexed as
filter"). The backend knows this from the index status and answers 503
before any model call, so such a question costs nothing and does not count
against the daily limit. If the status cannot be read (e.g. missing
permission), the search itself reports it and the answer is the same 503;
the embedding of the question has then already been computed.

**Option B (Atlas UI),** if option A fails (warning on start):

1. Open the cluster in Atlas, tab "Atlas Search" (depending on the UI
   "Search & Vector Search"), "Create Search Index" (or "Edit" on an
   existing `chunks_vector`).
2. Choose "Atlas Vector Search" and "JSON Editor".
3. Database = value of `MONGODB_DB`, collection `chunks`, name
   `chunks_vector`.
4. Paste the content of `internal/store/chunks_vector.json` (with the
   filter field `userId`), save.
5. Wait until the status is "Active"/`READY`.

The free tier allows only a few search indexes per cluster. Indexes of
dropped collections disappear with them.

If `llm.EmbeddingDimensions` changes, the index must be recreated and every
chunk embedded again.

## Synthetic data

The demo data is in [`demo/`](../demo/README.md). To fill a database in one
step (seed entries plus the uploads used for the calibration), with the
server running:

```sh
../demo/upload.sh
```

Only the seed entries, so that timeline and search show something without
upload and without the vision LLM. `--username` is required since
2026-11-18: it names the account that owns them.

```sh
go run ./cmd/seed --username admin2              # 4 invented entries with chunks and embeddings (can be run again)
go run ./cmd/seed --username admin2 -no-chunks   # the same without Gemini; search does not find them then
go run ./cmd/seed --username admin2 -remove      # delete them again, with their chunks
```

The seed entries (`internal/sampledata`) have fixed IDs, so only one
account per database can hold them. Other entries stay untouched. The
chunks are created as on upload: same splitting, same embedding model.

To try out `DELETE /entries/{id}` (e.g. from the app):

```sh
go run ./cmd/seed --username admin2 -throwaway          # creates a throwaway entry with a new ID and prints the ID
curl -i -X DELETE -H "Authorization: Bearer $TOKEN" localhost:8080/api/v1/entries/<ID>   # 204, a second time 404
```

The throwaway entry needs no Gemini: its chunk has a fixed vector instead of
a real embedding, and search does not show it. Only delete the printed ID,
not the fixed seed entries or real uploads.

## Calibrate the threshold

```sh
../demo/upload.sh                                         # once: seed entries and demo uploads
go run ./cmd/calibrate --username admin2                  # evaluates with core.MinScore
go run ./cmd/calibrate --username admin2 -threshold 0.82
```

`--username` (required since 2026-11-18) names the account whose entries
are measured, as `POST /questions` would search them; use the account that
`demo/upload.sh` filled.

Asks 45 fixed questions (`cmd/calibrate/questions.go`): 30 relevant ones (6
of them with the answer in two entries), 7 health questions whose answer is
in no document ("near"), and 8 unrelated ones. The questions are German,
because users ask in German. The program finds the seed entries by their
fixed IDs and the demo uploads by the file name of the original; the two
text notes are all called `text.txt` and are found by their document date.
If an entry is missing or there twice, it stops before the first embedding.
It only reads from the database.

For each question it shows the best score and whether an expected entry is
on top. With two expected entries the weaker one counts, because both must
reach the language model. Then it shows which thresholds separate relevant
from unrelated questions. Cost: one embedding per question. The language
model is not called, so check the answers (German, with `[n]`, fixed
sentence for "near" questions) by hand via `POST /api/v1/questions`.

Measurements so far are in [`docs/calibration/`](../docs/calibration/).

`testdata/lab_report.{txt,pdf,png}` is an invented report for uploads and
live tests. PDF and PNG were generated from the `.txt`:

```sh
cupsfilter -m application/pdf lab_report.txt > lab_report.pdf
sips -s format png lab_report.pdf --out lab_report.png
```

## Structure

| Package | Purpose |
|---|---|
| `cmd/server` | Start: configuration, MongoDB, HTTP server, clean shutdown |
| `cmd/seed` | Insert or remove the synthetic seed entries |
| `cmd/users` | Create, reset, disable, enable, list and delete accounts; end sessions |
| `cmd/calibrate` | Measure vector search scores for fixed questions (threshold) |
| `cmd/models` | List available Gemini models |
| `cmd/ocrcheck` | Fill in (`-write`) or redo (`-recheck -write`) the verification of stored entries, measure OCR on `demo/` |
| `cmd/speechcheck` | Measure the transcription on the sentences in `demo/speech/` (key terms, speed, silence) |
| `scripts/whisper-server.sh`, `.ps1` | Download and verify the Whisper models, start whisper-server |
| `internal/sampledata` | The invented seed entries, shared by seed and calibrate |
| `internal/auth` | Password hashes (bcrypt), tokens, generated passwords, the counter for failed attempts, the client IP, the user in the request context |
| `internal/config` | Read and check environment variables, mask secrets |
| `internal/core` | Domain logic without I/O: types, modules, sorting, file type, chunks, checking the extraction, questions, corrections, verification against the original, audio type, loudness check and cleaning of transcripts |
| `internal/llm` | Replaceable interface to the models (`Reader`, `Embedder`, `Answerer`) |
| `internal/llm/gemini` | Implementation with Gemini, including retry and fallback model |
| `internal/llm/fake` | Fake models for tests |
| `internal/ocr` | Replaceable second reader: `Recognizer` (file), `Engine` (image), `PDF`; `Pipeline` uses the text layer where possible and OCR where needed |
| `internal/ocr/poppler`, `internal/ocr/tesseract` | Implementations with the command line programs (stdin/stdout, no temporary files) |
| `internal/ocr/fake` | Fake second reader for tests |
| `internal/speech` | Replaceable speech recognition: `Transcriber`, `Converter` (audio → 16 kHz PCM), `Engine`; `Pipeline` checks length and loudness and cleans the text |
| `internal/speech/ffmpeg`, `internal/speech/whisper` | Implementations: ffmpeg (one temporary file, deleted right after), whisper-server over HTTP |
| `internal/speech/fake` | Fake transcriber for tests |
| `internal/service` | Use cases: upload, text import, question, correction, delete, login and account |
| `internal/store` | MongoDB: entries, chunks with embeddings, words read from the originals (`ocr`), original files (GridFS), all filtered by owner; users, sessions, audit log, daily counters (`usage`), indexes |
| `internal/api` | Router, auth middleware, handlers, error format, JSON types |
