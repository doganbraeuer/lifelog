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
Dafür sind vom Lese-Limit (50 s) standardmäßig 20 s reserviert
(`GEMINI_ERSATZ_RESERVE_SEKUNDEN`), das Hauptmodell bekommt 30 s.

## Vektorindex

Die Suche (`POST /api/v1/fragen`) braucht in Atlas den Vector-Search-Index
`abschnitte_vektor` auf der Collection `abschnitte`. Die Definition steht in
[`internal/speicher/abschnitte_vektor.json`](internal/speicher/abschnitte_vektor.json):
`embedding` als Vektor (768 Dimensionen, cosine), `modul` als Filterfeld.

**Weg A (automatisch):** Der Server legt den Index beim Start an, wenn er
fehlt, und loggt den Status. Atlas baut ihn im Hintergrund (im Test ~30 s);
bis er `READY` ist, findet die Suche nichts. Auf dem Free Tier (M0) getestet
am 04.11.2026.

**Weg B (Atlas-Oberfläche),** falls Weg A scheitert (Warnung beim Start):

1. In Atlas den Cluster öffnen, Reiter „Atlas Search“ (je nach Oberfläche
   „Search & Vector Search“), „Create Search Index“.
2. „Atlas Vector Search“ und „JSON Editor“ wählen.
3. Datenbank = Wert von `MONGODB_DB`, Collection `abschnitte`, Name
   `abschnitte_vektor`.
4. Inhalt von `internal/speicher/abschnitte_vektor.json` einfügen, speichern.
5. Warten, bis der Status „Active“/`READY` ist.

Wird `modell.EmbeddingDimensionen` geändert, muss der Index neu angelegt und
jeder Abschnitt neu eingebettet werden.

## Synthetische Testdaten

Damit Zeitleiste und Suche ohne Upload und ohne Vision-LLM etwas zeigen:

```sh
go run ./cmd/testdaten                   # 4 erfundene Einträge mit Abschnitten und Embeddings (mehrfach ausführbar)
go run ./cmd/testdaten -ohne-abschnitte  # dasselbe ohne Gemini; die Suche findet sie dann nicht
go run ./cmd/testdaten -entfernen        # wieder löschen, samt Abschnitten
```

Die Testeinträge (`internal/beispieldaten`) haben feste IDs. Andere Einträge
bleiben unberührt. Die Abschnitte entstehen wie beim Upload: gleiches
Zerlegen, gleiches Embedding-Modell.

## Schwelle kalibrieren

```sh
go run ./cmd/testdaten       # einmal, mit Abschnitten
go run ./cmd/kalibrieren     # wertet mit kern.MinScore aus
go run ./cmd/kalibrieren -schwelle 0.82
```

Stellt 18 feste Fragen (`cmd/kalibrieren/fragen.go`): 10, die die Testdaten
beantworten, 3 zu Gesundheit, die nicht drinstehen, und 5 fachfremde. Pro
Frage zeigt es den besten Score der Vektorsuche und ob der richtige Eintrag
oben steht. Danach folgt, welche Schwellen passend von fachfremd trennen.
Kosten: ein Embedding pro Frage. Das Sprachmodell wird nicht aufgerufen.

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
| `cmd/kalibrieren` | Scores der Vektorsuche für feste Fragen messen (Schwelle) |
| `internal/beispieldaten` | Die erfundenen Testeinträge, gemeinsam für beide Programme |
| `cmd/modelle` | Verfügbare Gemini-Modelle auflisten |
| `internal/konfig` | Umgebungsvariablen lesen und prüfen, Geheimnisse maskieren |
| `internal/kern` | Fachlogik ohne I/O: Typen, Module, Sortierung, Dateityp, Abschnitte, Prüfung der Erkennung |
| `internal/modell` | Austauschbare Schnittstelle zu den Modellen (`Leser`, `Einbetter`) |
| `internal/modell/gemini` | Umsetzung mit Gemini, inkl. Wiederholung und Ersatzmodell |
| `internal/modell/attrappe` | Modell-Ersatz für Tests |
| `internal/speicher` | MongoDB: Einträge, Abschnitte mit Embeddings, Originaldateien (GridFS), Indexe |
| `internal/api` | Router, Handler, Fehlerformat, JSON-Typen |
