# Glossary: German → English

Status: 2026-11-09. Prof. Pree requires all code in English. This table maps
every German name used so far to its English replacement, so that the
backend (Dogan) and the app (Dylan) use the same names. docs/API.md is the
contract; this glossary only lists names.

## Conventions

- JSON keys, BSON fields, query parameters: camelCase (`documentDate`).
- Code values (modules, types, sort values): lowercase, snake_case if more
  than one word (`lab_report`).
- Error codes: UPPER_SNAKE_CASE (`ENTRY_NOT_FOUND`).
- Stays German: everything the user sees — the API field `message`, the
  fixed answer "Dazu finde ich nichts in deinen Dokumenten.", model answers,
  app UI texts, display names of modules and types — and document content
  (sample data, test documents, calibration questions).

## Domain terms

| German | English | Go / Dart type |
|---|---|---|
| Eintrag / Einträge | entry / entries | `Entry` |
| Eintrag (Kurzform, Zeitleiste) | entry summary | `EntrySummary` |
| Modul | module | `Module` |
| Typ (Dokumenttyp) | type (document type) | `Type` |
| Dokumentdatum | document date | `DocumentDate` |
| Importdatum | imported at | `ImportedAt` |
| Quelle (ausstellende Stelle) | issuer | `Issuer` |
| Volltext | full text | `FullText` |
| Info / Infos | fact / facts | `Fact` |
| Name, Wert, Einheit | name, value, unit | |
| Datei, Originaldatei | file, original file | `File` |
| Dateiname, MIME-Typ, Größe | file name, MIME type, size | |
| Korrektur / Korrekturen | correction / corrections | `Correction` |
| Feld, erkannt, korrigiert, Zeitpunkt | field, recognized, corrected, corrected at | |
| Erkennung (Ergebnis des Lesens) | extraction | `Extraction` |
| erkannte Info | extracted fact | `ExtractedFact` |
| Abschnitt / Abschnitte | chunk / chunks | `Chunk` |
| Frage | question | |
| Antwort | answer | `Answer` |
| Fundstelle (zitierte Quelle) | source | `Source` |
| Auszug | excerpt | |
| Treffer (Vektorsuche) | hit | `Hit` |
| fester Satz | not-found answer | `NotFoundAnswer` |
| Schwelle | threshold | `MinScore` (unchanged) |
| Zeitleiste | timeline | |
| Hochladen / Text-Import | upload / text import | |
| Korrigieren / Löschen | correct / delete | |
| Duplikat | duplicate | |
| Sortierung / Richtung | sort / order | `Sort` / `SortOrder` |
| absteigend / aufsteigend | descending (`desc`) / ascending (`asc`) | |

## REST API

Base path stays `/api/v1`.

### Endpoints

| German | English |
|---|---|
| `POST /eintraege` | `POST /entries` |
| `GET /eintraege` | `GET /entries` |
| `GET /eintraege/{id}` | `GET /entries/{id}` |
| `GET /eintraege/{id}/datei` | `GET /entries/{id}/file` |
| `PATCH /eintraege/{id}` | `PATCH /entries/{id}` |
| `DELETE /eintraege/{id}` | `DELETE /entries/{id}` |
| `POST /fragen` | `POST /questions` |
| `GET /status` | `GET /status` (unchanged) |

### Upload (multipart form fields)

| German | English |
|---|---|
| `datei` | `file` |
| `text` | `text` (unchanged) |
| `modul` | `module` |

The stored file name for a text import stays `text.txt`.

### List query parameters

| German | English |
|---|---|
| `modul` | `module` |
| `sortierung=dokumentdatum` / `importdatum` | `sort=documentDate` / `importedAt` |
| `richtung=absteigend` / `aufsteigend` | `order=desc` / `asc` |

### Entry JSON

| German | English |
|---|---|
| `id` | `id` |
| `modul` | `module` |
| `typ` | `type` |
| `dokumentdatum` | `documentDate` |
| `importdatum` | `importedAt` |
| `quelle` | `issuer` |
| `volltext` | `fullText` |
| `infos` | `facts` |
| `infos[].id` (`i1`, `i2`, …) | `facts[].id` (`f1`, `f2`, …) |
| `infos[].name` / `wert` / `einheit` | `facts[].name` / `value` / `unit` |
| `datei` | `file` |
| `datei.id` / `dateiname` / `mimeTyp` / `groesse` / `sha256` | `file.id` / `fileName` / `mimeType` / `size` / `sha256` |
| `korrekturen` | `corrections` |
| `korrekturen[].feld` / `erkannt` / `korrigiert` / `zeitpunkt` | `corrections[].field` / `recognized` / `corrected` / `correctedAt` |

List response: `eintraege` → `entries`, `anzahl` → `count`.

### Correction field paths (`corrections[].field`, `details.field`)

| German | English |
|---|---|
| `typ` | `type` |
| `dokumentdatum` | `documentDate` |
| `quelle` | `issuer` |
| `infos.<id>.wert` | `facts.<id>.value` |
| `infos.<id>.einheit` | `facts.<id>.unit` |
| `infos.<id>.name` | `facts.<id>.name` |
| `volltext` | `fullText` |

The PATCH body uses the same keys as the entry JSON (`type`,
`documentDate`, `issuer`, `facts[].id/value/unit`).

### Question and answer JSON

| German | English |
|---|---|
| `frage` | `question` |
| `modul` | `module` |
| `antwort` | `answer` |
| `gefunden` | `found` |
| `quellen` | `sources` |
| `quellen[].nr` | `sources[].number` |
| `quellen[].eintragId` | `sources[].entryId` |
| `quellen[].modul` / `typ` / `quelle` / `dokumentdatum` | `sources[].module` / `type` / `issuer` / `documentDate` |
| `quellen[].auszug` | `sources[].excerpt` |

### Error format

| German | English |
|---|---|
| `fehler` | `error` |
| `fehler.code` | `error.code` |
| `fehler.nachricht` | `error.message` (text stays German) |
| `fehler.details` | `error.details` |
| `details.feld` | `details.field` |
| `details.wert` | `details.value` |
| `details.parameter` | `details.parameter` |
| `details.infoId` | `details.factId` |
| `details.eintragId` | `details.entryId` |
| `details.maxZeichen` | `details.maxChars` |

### Error codes

| HTTP | German | English |
|---|---|---|
| 400 | `UNGUELTIGE_ANFRAGE` | `INVALID_REQUEST` |
| 400 | `UNGUELTIGER_PARAMETER` | `INVALID_PARAMETER` |
| 400 | `UNGUELTIGE_ID` | `INVALID_ID` |
| 400 | `DATEI_FEHLT` | `FILE_MISSING` |
| 400 | `FELD_NICHT_KORRIGIERBAR` | `FIELD_NOT_CORRECTABLE` |
| 400 | `FRAGE_FEHLT` | `QUESTION_MISSING` |
| 400 | `FRAGE_ZU_LANG` | `QUESTION_TOO_LONG` |
| 404 | `EINTRAG_NICHT_GEFUNDEN` | `ENTRY_NOT_FOUND` |
| 404 | `INFO_NICHT_GEFUNDEN` | `FACT_NOT_FOUND` |
| 409 | `DUPLIKAT` | `DUPLICATE` |
| 413 | `DATEI_ZU_GROSS` | `FILE_TOO_LARGE` |
| 415 | `DATEITYP_NICHT_UNTERSTUETZT` | `UNSUPPORTED_FILE_TYPE` |
| 422 | `NICHT_LESBAR` | `UNREADABLE` |
| 500 | `INTERNER_FEHLER` | `INTERNAL_ERROR` |
| 502 | `AUSLESEN_FEHLGESCHLAGEN` | `EXTRACTION_FAILED` |
| 503 | `MODELL_NICHT_ERREICHBAR` | `MODEL_UNAVAILABLE` |

### Status

| German | English |
|---|---|
| `"status": "ok"` / `"fehler"` | `"status": "ok"` / `"error"` |
| `"datenbank": "ok"` / `"nicht erreichbar"` | `"database": "ok"` / `"unreachable"` |

## Module and type codes

Display names in the app stay German.

| Module (German code) | English code | Display name |
|---|---|---|
| `gesundheit` | `health` | Gesundheit |
| `ernaehrung` | `nutrition` | Ernährung |
| `rezepte` | `recipes` | Rezepte |
| `sport` | `sport` | Sport |

| Type in `health` (German code) | English code | Display name |
|---|---|---|
| `arztbrief` | `doctor_letter` | Arztbrief |
| `laborbefund` | `lab_report` | Laborbefund |
| `rezept` | `prescription` | Rezept |
| `sonstiges` | `other` | Sonstiges |

## App checklist (Dylan)

Every place where the app talks to the backend:

| What | Where in app/lib (today) | Change |
|---|---|---|
| Paths `/eintraege`, `/eintraege/{id}/datei`, `/fragen` | `api/http_api_client.dart`, `api/api_client.dart` (doc comments), `api/mock_api_client.dart` | `/entries`, `/entries/{id}/file`, `/questions` |
| Multipart fields `datei`, `text`, `modul` | `api/http_api_client.dart` | `file`, `text`, `module` |
| Query `modul`, `sortierung`, `richtung` | `api/http_api_client.dart` | `module`, `sort`, `order` |
| Enum values sent as query (`Sortierung.name`, `Richtung.name`) | `api/api_client.dart` | `documentDate`/`importedAt`, `desc`/`asc` |
| List keys `eintraege`, `anzahl` | `api/http_api_client.dart`, mock | `entries`, `count` |
| Entry keys | `model/eintrag.dart`, mock data | see "Entry JSON" |
| Correction prefix check `infos.$infoId.` | `model/eintrag.dart` | `facts.$factId.` |
| PATCH body keys | `model/eintrag_aenderung.dart` | `type`, `documentDate`, `issuer`, `facts`, `id`/`value`/`unit` |
| Question body `frage`, `modul` | `api/http_api_client.dart` | `question`, `module` |
| Answer keys | `model/antwort.dart` | see "Question and answer JSON" |
| Error envelope `fehler`/`code`/`nachricht`/`details` | `api/http_api_client.dart` | `error`/`code`/`message`/`details` |
| `details['eintragId']` (duplicate) | `screens/upload_screen.dart` | `details['entryId']` |
| Error code constants | `api/api_fehler.dart`, mock | see "Error codes" |
| App-only codes `NETZWERK_FEHLER`, `UNBEKANNTE_ANTWORT`, `details.grund` | `api/api_fehler.dart`, `api/http_api_client.dart` | `NETWORK_ERROR`, `UNEXPECTED_RESPONSE`, `details.reason` |
| Module and type codes | `module/modul_katalog.dart`, tests | see "Module and type codes" |
| Fixed sentence | `api/mock_api_client.dart` | unchanged (German) |
| `--dart-define=API_BASE_URL` | `api/api_client_auswahl.dart` | unchanged |

## MongoDB

| German | English |
|---|---|
| collection `eintraege` | `entries` |
| collection `abschnitte` | `chunks` |
| GridFS bucket `dateien` (`dateien.files`, `dateien.chunks`) | `originals` (`originals.files`, `originals.chunks`) — not `files`, to avoid `files.chunks` next to `chunks` |
| GridFS metadata `mimeTyp` | `mimeType` |
| entry fields | same as entry JSON (`_id` stays) |
| chunk `eintragId` / `modul` / `nr` / `text` / `embedding` | `entryId` / `module` / `position` / `text` / `embedding` |
| index `datei_sha256_eindeutig` on `datei.sha256` | `file_sha256_unique` on `file.sha256` |
| index `modul` | `module` |
| index `eintragId` | `entryId` |
| vector index `abschnitte_vektor` (filter `modul`) | `chunks_vector` (filter `module`) |
| file `abschnitte_vektor.json` | `chunks_vector.json` |
| integration test database `<MONGODB_DB>_integrationstest` | `<MONGODB_DB>_integration_test` |

## Environment variables

| German | English |
|---|---|
| `MONGODB_URI`, `MONGODB_DB`, `GEMINI_API_KEY`, `PORT` | unchanged |
| `ADRESSE` | `ADDRESS` |
| `GEMINI_MODELL` | `GEMINI_MODEL` |
| `GEMINI_EMBEDDING_MODELL` | `GEMINI_EMBEDDING_MODEL` |
| `GEMINI_MODELL_ERSATZ` | `GEMINI_FALLBACK_MODEL` |
| `GEMINI_ERSATZ_RESERVE_SEKUNDEN` | `GEMINI_FALLBACK_RESERVE_SECONDS` |
| `LIFELOG_LIVE`, `LIFELOG_MONGO_TEST`, `LIFELOG_DIAGNOSE`, `LIFELOG_DIAGNOSE_THINKING` | unchanged |
| `LIFELOG_DIAGNOSE_DATEI` / `_MODELL` / `_VARIANTE` | `LIFELOG_DIAGNOSE_FILE` / `_MODEL` / `_VARIANT` |
| `API_BASE_URL` (app) | unchanged |

## Commands (backend/cmd)

| German | English |
|---|---|
| `cmd/server` | unchanged |
| `cmd/kalibrieren`, flag `-schwelle` | `cmd/calibrate`, flag `-threshold` |
| `cmd/modelle` | `cmd/models` |
| `cmd/testdaten` | `cmd/seed` (not `testdata`: Go skips folders with that name) |
| `-entfernen` / `-ohne-abschnitte` / `-wegwerf` | `-remove` / `-no-chunks` / `-throwaway` |
| calibration groups `passend` / `nah` / `fachfremd` | `relevant` / `near` / `unrelated` |

## Backend packages and folders

| German | English |
|---|---|
| `internal/kern` | `internal/core` |
| `internal/konfig` | `internal/config` |
| `internal/dienst` | `internal/service` |
| `internal/speicher` | `internal/store` |
| `internal/modell` | `internal/llm` |
| `internal/modell/attrappe` | `internal/llm/fake` |
| `internal/modell/gemini` | `internal/llm/gemini` |
| `internal/beispieldaten` | `internal/sampledata` |
| `internal/api` | unchanged |
| `backend/testdaten` (`laborbefund.*`) | `backend/testdata` (`lab_report.*`) |

## Demo data (demo/)

Until 2026-11-11 outside the repository in `lifelog-demo` and
`lifelog-live`. Content unchanged, file names English; `fotos/` → `photos/`,
`texte/` → `texts/`, suffix `_foto` → `_photo`.

| German | English |
|---|---|
| `hochladen.sh` | `demo/upload.sh` (also runs `cmd/seed`) |
| `LIESMICH.txt` (both folders) | `demo/README.md` |
| `01_laborbefund_2026-01-20` | `01_lab_report_2026-01-20` |
| `02_arztbrief_orthopaedie_2026-03-05` | `02_doctor_letter_orthopedics_2026-03-05` |
| `03_ueberweisung_physiotherapie_2026-03-05` | `03_referral_physiotherapy_2026-03-05` |
| `04_rezept_2026-05-18` | `04_prescription_2026-05-18` |
| `05_impfbestaetigung_fsme_2026-06-10` | `05_vaccination_tbe_2026-06-10` |
| `06_laborbefund_kontrolle_2026-07-22` | `06_lab_report_followup_2026-07-22` |
| `07_notiz_physiotherapie_2026-08-12.txt` | `07_note_physiotherapy_2026-08-12.txt` |
| `08_notiz_blutdruck_2026-09-23.txt` | `08_note_blood_pressure_2026-09-23.txt` |
| `lifelog_live_demo/` | `demo/live/` |
| `L1_laborbefund_2026-10-28` | `L1_lab_report_2026-10-28` |
| `L2_arztbrief_kontrolle_2026-04-30` | `L2_doctor_letter_followup_2026-04-30` |
| `L3_sehtest_2026-09-15_foto` | `L3_eye_test_2026-09-15_photo` |
| app uploads `image_picker_….png` (2026-11-09) | `demo/live/*_app.png` |

## Model interface (internal/llm)

| German | English |
|---|---|
| `Leser`, `DokumentLesen`, `TextLesen` | `Reader`, `ReadDocument`, `ReadText` |
| `Einbetter`, `AbschnitteEinbetten`, `FrageEinbetten` | `Embedder`, `EmbedChunks`, `EmbedQuestion` |
| `Antworter`, `Antworten` | `Answerer`, `Answer` |
| `ErrNichtErreichbar`, `ErrAntwortUngueltig` | `ErrUnavailable`, `ErrInvalidResponse` |
| Ersatzmodell, Reserve | fallback model, fallback reserve |
| Wiederholung | retry |
| Lese-, Text-, Antwort-Anweisung | read, text, answer instruction (prompt) |
| `lesbar` (schema field) | `readable` |

## Verification (OCR)

Added 2026-11-11. A second, independent reader (local OCR) checks the
values the vision LLM extracted (docs/API.md, section 2, `verification`):
the text layer of a PDF, or OCR for photos and scans.

| German | English | Go / Dart type |
|---|---|---|
| Texterkennung | OCR (optical character recognition) | package `internal/ocr` |
| OCR-Schnittstelle (ganzes Dokument), erkennen | recognizer, recognize | `ocr.Recognizer`, `Recognize` |
| OCR-Programm für ein Bild (Tesseract, später PaddleOCR) | engine | `ocr.Engine`, `RecognizeImage` |
| Ablauf Textebene → Rastern → OCR | pipeline | `ocr.Pipeline` |
| Textebene (PDF) | text layer | `poppler.TextLayer` |
| OCR-Ergebnis | OCR result | `OCRResult` |
| erkanntes Wort | OCR word | `OCRWord` |
| Seite / Zeile | page / line | `OCRPage`, `Page`, `Line` |
| sichtbare Zeile (aus Positionen gebildet) | row | `arrangeRows` |
| Rechteck (Wortposition, Pixel-unabhängig) | box | `Box` |
| Konfidenz | confidence (0–100) | `Confidence` |
| Fundstelle im Original | location | `Location` |
| Prüfung (Abgleich LLM ↔ OCR) | verification | `Verification` |
| Prüfung eines Felds | field check | `FieldCheck` |
| geprüft / nicht geprüft | `checked` / `unchecked` | |
| bestätigt / unbestätigt | `confirmed` / `unconfirmed` | |
| korrigiert und bestätigt / korrigiert und unbestätigt | `corrected_confirmed` / `corrected_unconfirmed` | |
| Grund (warum nicht geprüft) | reason | |
| Übereinstimmung in Prozent | match percent | `matchPercent` |
| Prüfquelle | source: `pdf_text` (text layer), `ocr`, `text` (text import), `mixed` (entry only) | `Source` |
| Programme mit Version und Sprache (Anzeige) | engine | `OCRResult.Engine` |
| Rastern (PDF-Seite → Bild) | render | `poppler.RenderPage` |
| Hochskalieren kleiner Bilder vor OCR | upscale | `tesseract.MinLongSide`, `DefaultMinLongSide`; `cmd/ocrcheck -upscale` |
| echtes Handyfoto eines ausgedruckten Demo-Dokuments | phone photo, suffix `_phone` | `cmd/ocrcheck -extra` |

### API (entry JSON)

| Field | Values |
|---|---|
| `verification.status` | `checked`, `unchecked` |
| `verification.reason` | `null`, `ocr_unavailable`, `ocr_failed`, `unsupported_file_type`, `too_many_pages`, `no_text`, `not_run` |
| `verification.source` | `pdf_text`, `ocr`, `text`, `mixed`, `null` |
| `verification.fields[].source` | `pdf_text`, `ocr`, `text`, `null` |
| `verification.engine`, `checkedAt`, `confirmed`, `corrected`, `total`, `matchPercent` | |
| `verification.fields[].status` | `confirmed`, `unconfirmed`, `corrected_confirmed`, `corrected_unconfirmed` |
| `verification.fields[].field` | `documentDate`, `issuer`, `facts.<id>` |
| `verification.fields[].confidence` / `locations` | |
| location in a file | `page`, `x`, `y`, `width`, `height` (fractions of the page) |
| location in a text import | `start`, `end` (code point offsets in `fullText`) |

### Backend

| German | English |
|---|---|
| collection mit den gelesenen Wörtern (Textebene oder OCR) | `ocr` (`_id` = entry ID; `engine`, `pages`, `words`) |
| entry field in MongoDB | `verification` (missing in old entries → `unchecked`/`not_run`) |
| Messung der OCR-Qualität, Prüfung nachtragen | `cmd/ocrcheck` (`-write`, `-values`, `-out`, `-demo`) |
| Prüfung nachtragen (Service) | verifier | `service.Verifier`, `Check`, `Save`, `NeedsCheck` |
| Messergebnisse | `docs/ocr/` |
| OCR wählen / ausschalten | env `OCR_ENGINE` (`tesseract`, `none`) |
| Sprachpaket für Tesseract | env `TESSERACT_LANG` (default `deu`) |
