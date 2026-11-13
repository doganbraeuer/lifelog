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
| erlaubte Dateitypen beim Upload | `image/jpeg`, `image/png`, `image/heic`, `image/webp` (since 2026-11-11), `application/pdf` | `MimeJPEG`, `MimePNG`, `MimeHEIC`, `MimeWebP`, `MimePDF` |
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
| – | `POST /transcriptions` (since 2026-11-12, see "Speech") |

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
| – | `details.maxSeconds` (since 2026-11-12, for `AUDIO_TOO_LONG`) |

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
| 413 | – | `AUDIO_TOO_LONG` (since 2026-11-12) |
| 422 | – | `NO_SPEECH` (since 2026-11-12) |

### App-only error codes

The backend never sends these; the app sets them itself in an
`ApiException` and shows its German `message`.

| German (until 2026-11-09) | English | When | Where in app/lib |
|---|---|---|---|
| `NETZWERK_FEHLER` | `NETWORK_ERROR` | No connection or timeout | `api/api_exception.dart` (`ErrorCode.networkError`) |
| `UNBEKANNTE_ANTWORT` | `UNEXPECTED_RESPONSE` | The response does not match the contract | `api/api_exception.dart` (`ErrorCode.unexpectedResponse`) |
| – | `PICK_FAILED` | Choosing a photo or file failed (camera, photo library or file picker), e.g. access denied | `api/api_exception.dart` (`ErrorCode.pickFailed`), `screens/upload_screen.dart` |
| – | `RECORDING_FAILED` | Recording a spoken note failed: microphone access denied, or the recording could not be started or saved | `api/api_exception.dart` (`ErrorCode.recordingFailed`), `screens/upload_screen.dart`, `screens/voice_note_view.dart` |

### Status

| German | English |
|---|---|
| `"status": "ok"` / `"fehler"` | `"status": "ok"` / `"error"` |
| `"datenbank": "ok"` / `"nicht erreichbar"` | `"database": "ok"` / `"unreachable"` |
| – | `"speech": "ok"` / `"unavailable"` / `"off"` (since 2026-11-12) |
| – | Dart: `ServerStatus` (response of `GET /status`), `SpeechStatus` (`ok`, `unavailable`, `off`); `app/lib/model/server_status.dart` |

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
| – | `SPEECH_ENGINE`, `WHISPER_URL` (since 2026-11-12, see "Speech") |

## Commands (backend/cmd)

| German | English |
|---|---|
| `cmd/server` | unchanged |
| `cmd/kalibrieren`, flag `-schwelle` | `cmd/calibrate`, flag `-threshold` |
| `cmd/modelle` | `cmd/models` |
| `cmd/testdaten` | `cmd/seed` (not `testdata`: Go skips folders with that name) |
| `-entfernen` / `-ohne-abschnitte` / `-wegwerf` | `-remove` / `-no-chunks` / `-throwaway` |
| calibration groups `passend` / `nah` / `fachfremd` | `relevant` / `near` / `unrelated` |
| – | `cmd/speechcheck` (since 2026-11-12, see "Speech") |

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
| Status der Prüfung (App) | verification status | Dart `VerificationStatus` (`checked`, `unchecked`); `app/lib/model/verification.dart` |
| Prüfquelle (App) | verification source | Dart `VerificationSource` (`pdfText`, `ocr`, `text`, `mixed`, `unknown`) |
| Grund, warum nicht geprüft (App) | verification reason | Dart `VerificationReason` (one value per `reason`, plus `unknown`) |
| Status eines Felds (App) | field check status | Dart `FieldCheckStatus` (`confirmed`, `unconfirmed`, `correctedConfirmed`, `correctedUnconfirmed`, `unknown`) |
| Fundstelle in einer Datei / in einem Text-Import (App) | page location / text location | Dart `PageLocation`, `TextLocation` (both a `Location`) |
| Fundstelle im Original zeigen, Sprung ins Original (App) | show in original | Dart `OriginalLocationScreen`; `app/lib/screens/original_location_screen.dart`; hint `showInOriginalHint` („Im Original zeigen“) |
| Foto bzw. Text-Import mit markierter Fundstelle (App) | photo location view / text location view | Dart `PhotoLocationView`, `TextLocationView` |
| Markierung der Fundstellen auf einer Seite oder einem Foto (App) | location overlay | Dart `LocationOverlay`, `LocationMarksPainter`, `fittedPageRect`, `locationRect`, `focusTransform`; `app/lib/widgets/location_overlay.dart` |
| als Bild gerenderte PDF-Seite (App) | rendered PDF page | Dart `RenderedPdfPage`, `PdfViewer.renderPage` |
| Codepoint-Bereich als UTF-16-Bereich (App) | UTF-16 range | Dart `TextLocation.utf16Range` |
| Mock-Foto mit Wortblöcken, Layout einer Mock-Seite (App) | mock photo, mock layout | Dart `createMockPhoto`, `MockLayout` (`pdfA4`, `photo`) |
| geprüft / nicht geprüft | `checked` / `unchecked` | |
| bestätigt / unbestätigt | `confirmed` / `unconfirmed` | |
| korrigiert und bestätigt / korrigiert und unbestätigt | `corrected_confirmed` / `corrected_unconfirmed` | |
| Grund (warum nicht geprüft) | reason | |
| Übereinstimmung in Prozent | match percent | `matchPercent` |
| Prüfquelle | source: `pdf_text` (text layer), `ocr`, `text` (text import), `mixed` (entry only) | `Source` |
| Programme mit Version und Sprache (Anzeige) | engine | `OCRResult.Engine` |
| Rastern (PDF-Seite → Bild) | render | `poppler.RenderPage` |
| Bild für OCR aufbereiten (einlesen, aufrichten, Größe anpassen, als RGB-PNG an das OCR-Programm) | prepare image | `ocr.PrepareImage`, `ocr.ImageOptions`, `DefaultImageOptions` |
| Auflösung, die Tesseract mitgeteilt wird | dpi | `tesseract.DPI` (300, `--dpi`) |
| Ausrichtung (EXIF bei JPEG, Drehung in der HEIC-Datei) | orientation | `ocr.Orientation`, `exifOrientation` |
| Hochskalieren kleiner Bilder vor OCR | upscale | `ImageOptions.MinLongSide`, `DefaultMinLongSide`; `cmd/ocrcheck -upscale` |
| Verkleinern großer Bilder vor OCR | downscale | `ImageOptions.MaxLongSide`, `DefaultMaxLongSide`; `cmd/ocrcheck -downscale` |
| Bild zu groß zum Einlesen (Pixelzahl) | image too large | `ImageOptions.MaxPixels`, `ocr.ErrImageTooLarge`, reason `image_too_large` |
| echtes Handyfoto eines Demo-Dokuments auf Papier | phone photo of paper, suffix `_paper` | `demo/phone/`, `cmd/ocrcheck -extra`, kind `Phone photo of paper (real)` |
| echtes Handyfoto eines Demo-Dokuments auf dem Bildschirm | phone photo of a screen, suffix `_phone` | `demo/phone/`, kind `Phone photo of a screen (real)` |

### API (entry JSON)

| Field | Values |
|---|---|
| `verification.status` | `checked`, `unchecked` |
| `verification.reason` | `null`, `ocr_unavailable`, `ocr_failed`, `unsupported_file_type`, `too_many_pages`, `image_too_large`, `no_text`, `not_run` |
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
| Messung der OCR-Qualität, Prüfung nachtragen | `cmd/ocrcheck` (`-write`, `-values`, `-out`, `-demo`, `-upscale`, `-downscale`, `-extra`) |
| alle Einträge neu prüfen | recheck | `cmd/ocrcheck -recheck` (stores only with `-write`) |
| Prüfung nachtragen (Service) | verifier | `service.Verifier`, `Check`, `Save`, `NeedsCheck` |
| Messergebnisse | `docs/ocr/` |
| OCR wählen / ausschalten | env `OCR_ENGINE` (`tesseract`, `none`) |
| Sprachpaket für Tesseract | env `TESSERACT_LANG` (default `deu`) |

## Speech (transcription)

Added 2026-11-12. Spoken notes become text: the app uploads a recording,
the backend returns only the recognized text (nothing is stored), the
user checks and edits it, and the app then sends it through the normal
text import (docs/API.md, 3.9).

| German | English | Go / Dart type |
|---|---|---|
| Spracherkennung | speech recognition | package `internal/speech` |
| Transkription, transkribieren | transcription, transcribe | `speech.Transcriber`, `Transcribe`; endpoint `POST /transcriptions` |
| Aufnahme | recording | form field `audio` |
| erkannter Text | text | `text`, `core.Transcript` |
| Dauer der Aufnahme | duration | `durationSeconds` |
| Abschnitt der Erkennung (mit Start und Ende) | segment | `core.Segment` |
| Umwandeln in 16 kHz mono PCM | convert | `speech.Converter` (ffmpeg) |
| Spracherkennungsprogramm (whisper.cpp) | engine | `speech.Engine`, `Recognize` |
| Ablauf Umwandeln → Prüfen → Erkennen → Bereinigen | pipeline | `speech.Pipeline` |
| Sprachaktivitätserkennung | voice activity detection (VAD) | whisper.cpp `--vad` (Silero) |
| hörbarer Anteil (Pegelprüfung) | speech seconds | `core.SpeechSeconds` |
| keine Sprache, Stille | no speech | `NO_SPEECH`, `speech.ErrNoSpeech` |
| zu lange Aufnahme | too long | `AUDIO_TOO_LONG`, `speech.ErrTooLong`, `core.MaxAudioSeconds` (300) |
| erfundener Satz der Erkennung | hallucination | `core.CleanTranscript` |
| Fachwortliste (Prompt für Whisper) | vocabulary | `whisper.Vocabulary`, `whisper.VocabularyTerms`; form field `prompt`; `cmd/speechcheck -vocabulary=false` measures without it. German on purpose: sample text, not an instruction |
| Schlüsselbegriff (Messung) | key term | `cmd/speechcheck`, compared with `core.TextContains` |
| Wortfehlerrate (Messung) | word error rate (WER) | `core.WordErrorRate` |
| Audioformat erkennen | detect audio type | `core.DetectAudioType`, `core.CheckAudio` |
| Aufnahmegerät (Schnittstelle der App) | voice recorder | Dart `VoiceRecorder`; `app/lib/platform/voice_recorder.dart` |
| Aufnahme mit dem Paket `record` | plugin voice recorder | Dart `PluginVoiceRecorder` |
| fertige Aufnahme einer Sprachnotiz (App) | recording | Dart `Recording` |
| Ordner der Aufnahmen, Aufräumen (App) | recording files | Dart `RecordingFiles` |
| Aufnahme nicht möglich (App) | recorder exception | Dart `RecorderException`, shown as `RECORDING_FAILED` |
| Ergebnis der Transkription (App) | transcript | Dart `Transcript`; `app/lib/model/transcript.dart` |
| Zahlen und Einheiten im erkannten Text markieren | quantity highlight | Dart `QuantityHighlightController`; `app/lib/widgets/quantity_highlight.dart` |

| Item | Value |
|---|---|
| allowed MIME types | `audio/mp4`, `audio/3gpp`, `audio/amr`, `audio/wav`, `audio/ogg`, `audio/webm`, `audio/mpeg`, `audio/flac` |
| response fields | `text`, `durationSeconds`, `language` (always `de`), `engine` |
| status field | `speech`: `ok`, `unavailable`, `off` |
| env | `SPEECH_ENGINE` (`whisper` default, `none`), `WHISPER_URL` (default `http://127.0.0.1:8178`) |
| start script | `backend/scripts/whisper-server.sh` / `.ps1` (`download` fetches and verifies the models; without it: start, warm up with `scripts/warmup.wav`, start again after a crash); its env: `WHISPER_MODEL_DIR`, `WHISPER_PORT`, `WHISPER_SERVER` (Windows) |
| local server | `whisper-server` (whisper.cpp), model `ggml-large-v3-turbo.bin`, VAD model `ggml-silero-*.bin` |
| measurement | `cmd/speechcheck`, results in `docs/speech/`, reference texts in `demo/speech/` |

### App checklist (speech)

| What | Note |
|---|---|
| Recording | package `record`, `AudioEncoder.aacLc` (M4A), mono, 16 kHz, about 32 kbit/s |
| Permissions | iOS `NSMicrophoneUsageDescription` in `Info.plist`; Android `RECORD_AUDIO` |
| Upload | `POST /transcriptions`, form field `audio`, timeout at least 130 s |
| Result | show `text` editable; save only on confirmation via `POST /entries` with `text` |
| Microphone button | hide or disable when `/status` says `speech` is not `ok` |
| Clean up | delete the local recording after the transcription |
| New codes | `AUDIO_TOO_LONG`, `NO_SPEECH` (constants in `api/api_exception.dart`) |
