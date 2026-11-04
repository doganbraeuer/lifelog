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

Live-Tests gegen Gemini (kosten Anfragen aus dem Kontingent, schicken nur
die synthetischen Dateien aus `testdaten/`):

```sh
LIFELOG_LIVE=1 go test ./internal/modell/gemini/ -run Live -v
```

Integrationstests gegen MongoDB (benutzen eine eigene Datenbank
`<MONGODB_DB>_integrationstest` und löschen sie danach wieder):

```sh
LIFELOG_MONGO_TEST=1 go test ./internal/speicher/ -run Integration -v
```

## Gemini-Modelle

```sh
go run ./cmd/modelle   # listet die Modelle, die der eigene Schlüssel sieht
```

Der Schlüssel kommt aus `.env` und wird nie ausgegeben. Ob ein Modell in der
kostenlosen Stufe nutzbar ist und welche Limits gelten, steht nur in AI Studio
bzw. auf der Preisseite. Bei 429/503 versucht das Backend es bis zu zweimal
erneut; mit `GEMINI_MODELL_ERSATZ` danach einmal ein anderes Modell zum Lesen.

## Synthetische Testdaten

Damit die Zeitleiste ohne Upload und ohne Gemini etwas zeigt:

```sh
go run ./cmd/testdaten              # 4 erfundene Einträge einfügen (mehrfach ausführbar)
go run ./cmd/testdaten -entfernen   # wieder löschen
```

Die Testeinträge haben feste IDs. Andere Einträge bleiben unberührt.

`testdaten/laborbefund.{txt,pdf,png}` ist ein erfundener Befund für Uploads
und Live-Tests. PDF und PNG sind aus der `.txt` erzeugt:

```sh
cupsfilter -m application/pdf laborbefund.txt > laborbefund.pdf
sips -s format png laborbefund.pdf --out laborbefund.png
```

## Aufbau

| Paket | Aufgabe |
|---|---|
| `cmd/server` | Start: Konfiguration, MongoDB, HTTP-Server, sauberes Beenden |
| `cmd/testdaten` | Synthetische Einträge einfügen oder entfernen |
| `cmd/modelle` | Verfügbare Gemini-Modelle auflisten |
| `internal/konfig` | Umgebungsvariablen lesen und prüfen, Geheimnisse maskieren |
| `internal/kern` | Fachlogik ohne I/O: Typen, Module, Sortierung, Dateityp, Abschnitte, Prüfung der Erkennung |
| `internal/modell` | Austauschbare Schnittstelle zu den Modellen (`Leser`, `Einbetter`) |
| `internal/modell/gemini` | Umsetzung mit Gemini, inkl. Wiederholung und Ersatzmodell |
| `internal/modell/attrappe` | Modell-Ersatz für Tests |
| `internal/speicher` | MongoDB: Einträge, Abschnitte mit Embeddings, Originaldateien (GridFS), Indexe |
| `internal/api` | Router, Handler, Fehlerformat, JSON-Typen |
