# Lifelog-Backend (Go)

REST-API nach [docs/API.md](../docs/API.md).

## Starten

1. `.env` im Repo-Wurzelverzeichnis anlegen (Vorlage: `.env.example`).
2. Im Ordner `backend/`:

   ```sh
   go run ./cmd/server
   curl localhost:8080/api/v1/status
   ```

Standardmäßig lauscht der Server nur auf `127.0.0.1:8080`. Der
Android-Emulator erreicht ihn über `http://10.0.2.2:8080`. Für ein echtes Handy
im WLAN: `ADRESSE=0.0.0.0 go run ./cmd/server` und die IP des Rechners
verwenden. Dann ist das Backend im ganzen Netz erreichbar, also nur zum Testen.

## Testen

```sh
go vet ./... && go test ./...
```

## Aufbau

| Paket | Aufgabe |
|---|---|
| `cmd/server` | Start: Konfiguration, MongoDB, HTTP-Server, sauberes Beenden |
| `internal/konfig` | Umgebungsvariablen lesen und prüfen |
| `internal/api` | Router, Handler, Fehlerformat |
