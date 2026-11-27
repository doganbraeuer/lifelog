# Glossary: German → English

Status: 2026-11-09. Prof. Pree requires all code in English. This table maps
every German name used so far to its English replacement, so that the
backend (Dogan) and the app (Dylan) use the same names. docs/API.md is the
contract; this glossary only lists names.

Names marked "proposed" (routine suggestions, stage 2, 2026-11-23) are a
proposal that is not built yet; the routine names of stage 1 are binding
since 2026-11-23. See [Routines](#routines-2026-11-23).

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
| Herkunft des Dokumentdatums: aus dem Dokument / Tag des Imports (since 2026-11-23) | document date source: `document` / `import` | `DocumentDateSource`; Go `core.DateSourceDocument`, `core.DateSourceImport`; Dart `DocumentDateSource.document`, `.importDay` (`import` is no valid name in Dart), `EntrySummary.hasImportDate`, time of the import `formatLocalTime`; app text „(Importdatum)“ |
| Importtag einsetzen (Textimport ohne Datum) | import date | `core.ImportDate`; day in Europe/Vienna: `core.LocalDay`, `core.LocalTime` |
| relatives Datumswort („heute“, „gestern“, „vorgestern“, Wochentag) | relative date word | `core.RelativeDateWords` (verification of text imports) |
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
| – | `POST /auth/login`, `POST /auth/logout` (since 2026-11-18, see "Accounts and login") |
| – | `GET /me`, `PATCH /me`, `POST /me/password` (since 2026-11-18) |
| – | `PUT /me/consent` (since 2026-11-18) |
| – | `GET /routines`, `POST /routines`, `GET /routines/{id}`, `PATCH /routines/{id}`, `DELETE /routines/{id}` (since 2026-11-23, see "Routines") |
| – | `POST /routines/{id}/pause`, `POST /routines/{id}/resume` (since 2026-11-23) |
| – | `GET /routines/occurrences` (since 2026-11-23) |
| – | `GET /routines/{id}/logs`, `POST /routines/{id}/logs`, `DELETE /routines/{id}/logs/{logId}` (since 2026-11-23) |
| – | `POST /entries/{id}/routine-suggestions/{suggestionId}/confirm`, `DELETE /entries/{id}/routine-suggestions/{suggestionId}` (proposed, stage 2) |

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
| – | `documentDateSource`: `document`, `import` (since 2026-11-23; full entry and short form; BSON field of the same name, missing in older entries = `document`) |
| – | `routineSuggestions` (proposed, stage 2, 2026-11-23, see "Routines") |

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
| – | `details.retryAfterSeconds` (since 2026-11-18, for `TOO_MANY_ATTEMPTS`; same value as the header `Retry-After`) |
| – | `details.limit` (since 2026-11-18, for `LIMIT_REACHED`: `uploads`, `questions`, `transcriptions`; since 2026-11-23 also for `ROUTINE_LIMIT_REACHED`: `routines`, `pauses`, `logs`) |
| – | `details.max` (since 2026-11-23, for `ROUTINE_LIMIT_REACHED`: the maximum) |
| – | `details.routineId` (since 2026-11-23, for `ROUTINE_NAME_TAKEN`) |
| – | `details.logId` (since 2026-11-23, for `ROUTINE_LOG_EXISTS`) |
| – | `details.suggestionId` (proposed, stage 2, for `ROUTINE_SUGGESTION_NOT_FOUND`) |

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
| 400 | – | `WRONG_PASSWORD` (since 2026-11-18) |
| 400 | – | `WEAK_PASSWORD` (since 2026-11-18) |
| 401 | – | `INVALID_CREDENTIALS` (since 2026-11-18) |
| 401 | – | `UNAUTHORIZED` (since 2026-11-18) |
| 403 | – | `PASSWORD_CHANGE_REQUIRED` (since 2026-11-18) |
| 429 | – | `TOO_MANY_ATTEMPTS` (since 2026-11-18) |
| 429 | – | `LIMIT_REACHED` (since 2026-11-18) |
| 404 | – | `ROUTINE_NOT_FOUND` (since 2026-11-23) |
| 404 | – | `ROUTINE_LOG_NOT_FOUND` (since 2026-11-23) |
| 404 | – | `ROUTINE_SUGGESTION_NOT_FOUND` (proposed, stage 2) |
| 409 | – | `ROUTINE_LOG_EXISTS` (since 2026-11-23) |
| 409 | – | `ROUTINE_NAME_TAKEN` (since 2026-11-23) |
| 409 | – | `ROUTINE_LIMIT_REACHED` (since 2026-11-23) |

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
| index `datei_sha256_eindeutig` on `datei.sha256` | `file_sha256_unique` on `file.sha256`; replaced on 2026-11-18 by `user_file_sha256_unique` on `(userId, file.sha256)` and dropped on start |
| index `modul` | `module` |
| index `eintragId` | `entryId` |
| vector index `abschnitte_vektor` (filter `modul`) | `chunks_vector` (filter `module`; since 2026-11-18 also `userId`) |
| file `abschnitte_vektor.json` | `chunks_vector.json` |
| integration test database `<MONGODB_DB>_integrationstest` | `<MONGODB_DB>_integration_test` |
| – | collections `users`, `sessions`, `auditLog` (since 2026-11-18, fields see "Accounts and login") |
| – | index `username_unique` on `users.username` |
| – | index `token_hash_unique` on `sessions.tokenHash` |
| – | TTL index `expires_at_ttl` on `sessions.expiresAt` (`expireAfterSeconds: 0`: MongoDB deletes a session once it has expired) |
| – | index `userId` on `sessions.userId` |
| – | field `userId` (owner, ObjectId) on `entries`, `chunks`, `ocr`, and `metadata.userId` on `originals.files` (since 2026-11-18, see "Accounts and login") |
| – | index `user_module` on `entries` `(userId, module)`, `user_entry` on `chunks` `(userId, entryId)`, `userId` on `ocr`, `metadata_userId` on `originals.files` |
| – | collection `usage` (daily limits): `userId`, `day`, `kind`, `count`, `expiresAt`; index `usage_unique` on `(userId, day, kind)`, TTL index `expires_at_ttl` on `expiresAt` |
| – | collections `routines` and `routineLogs` (since 2026-11-23, fields see "Routines") |
| – | `routines`: index `user_module` on `(userId, module)`; unique index `user_module_name_unique` on `(userId, module, nameKey)` (`nameKey`: the name in lower case, for the uniqueness check) (since 2026-11-23) |
| – | `routineLogs`: unique index `routine_due_unique` on `(userId, routineId, dueLocal)`, partial `{dueLocal: {$type: "string"}}` (only logs with an occurrence; a log without one has no `dueLocal` at all; `dueLocal`: local date and time of the occurrence in the log's `timeZone`, e.g. `2026-11-23T08:00`); index `user_routine_taken` on `(userId, routineId, takenAt)` (since 2026-11-23); index `user_entry` on `(userId, entryId)` (deleting an entry; proposed, comes with stage 2) |
| – | field `timeZone` on `users` (since 2026-11-23; missing = `Europe/Vienna`) |

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
| – | `SPEECH_ENGINE`, `WHISPER_URL` (since 2026-11-12, see "Speech"; value `gemini` since 2026-11-20) |
| – | `TRUSTED_PROXIES` (since 2026-11-18, see "Accounts and login") |
| – | `LIFELOG_TOKEN` (since 2026-11-18, `demo/upload.sh` only: the token it sends) |
| – | `LIMIT_UPLOADS_PER_DAY`, `LIMIT_QUESTIONS_PER_DAY`, `LIMIT_TRANSCRIPTIONS_PER_DAY` (since 2026-11-18, daily limits, defaults 30, 60, 30) |

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
| – | `cmd/users` (since 2026-11-18, see "Accounts and login") |
| – | `cmd/seed --username`, `cmd/calibrate --username` (since 2026-11-18: required, the account whose entries they use) |

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
| – | `internal/auth` (since 2026-11-18: passwords, tokens, failed attempts, client IP) |
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
| heutiges Datum im Text-Prompt (since 2026-11-23) | today in the text instruction | `textInstruction(allowedTypes, now)`; `Reader.ReadText(…, now)` |
| Schutz des kalibrierten Datei-Prompts | read prompt guard | `readPromptSHA256`, `TestReadPrompt_Unchanged` (`internal/llm/gemini/prompt_guard_test.go`) |

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
| Auszug einer Quelle im Volltext finden (App) | locate excerpt | Dart `locateExcerpt`; `app/lib/model/excerpt_location.dart` |
| Verweis [n] im Antworttext (App) | citation | Dart `Citation`, `findCitations`; `app/lib/model/citations.dart` |
| Markierung einer Quelle im Volltext: eigene Zeilen stark, die anderer Quellen desselben Eintrags schwach (App) | source marks, mark sources, mark passage | Dart `SourceMarks`, `markSources`, `markPassage`; `app/lib/model/source_marks.dart` |
| Anker: die Antwortsätze, die eine Quelle mit [n] zitieren (App) | anchor | Dart `anchorText` |
| Text in Sätze teilen (App) | split sentences | Dart `splitSentences` |
| Markierabschnitt: Zeile, lange Zeile als Satz (App) | segment | Dart `splitSegments`, `maxSegmentLength` |
| Begriff (Zahl, Datum, Einheit, Fachwort) (App) | term, term kind | Dart `Term`, `TermKind` (`number`, `date`, `unit`, `word`), `findTerms`, `Term.baseWeight` |
| Seltenheit eines Begriffs (1 / Zahl der Abschnitte mit ihm) (App) | rarity | Dart `minRarity` |
| Grenzen der Markierung (App) | — | Dart `maxMarkedSegments`, `minScoreShare`, `maxMarkedShare` |
| stark markierte Fundstellen in der Textansicht (App) | strong locations | Dart `TextLocationView.strongLocations` |
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
| Spracherkennung durch das Sprachmodell | LLM engine | `SPEECH_ENGINE=gemini` (since 2026-11-20); `llm.AudioTranscriber`, `TranscribeAudio`; package `internal/speech/llmengine` |
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
| erkannten Text an der Cursorposition einfügen (App) | insert transcript | Dart `insertTranscript`; `app/lib/screens/voice_note_view.dart` |

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

## Accounts and login

Added 2026-11-18. Every user has an account and logs in with username and
password; every request except `/status` and the login carries a token
(docs/API.md, section 1 "Login and tokens" and 3.10–3.15). Accounts are
created by an administrator with `cmd/users`; there is no registration
through the API.

| German | English | Go type / field |
|---|---|---|
| Benutzer, Nutzer, Konto | user, account | `core.User`; collection `users` |
| Sitzung (Anmeldung auf einem Gerät) | session | `core.Session`; collection `sessions` |
| Benutzername | username | `username` (3–32 characters, `a-z` and `0-9`, stored lowercase) |
| Anzeigename | display name | `displayName` |
| Passwort-Hash (bcrypt) | password hash | `passwordHash` (never in the API, never in the log) |
| Rolle: Nutzer, Administrator | role: `user`, `admin` | `core.Role` (`core.RoleUser`, `core.RoleAdmin`); `role` |
| Passwort muss geändert werden | must change password | `mustChangePassword` |
| gesperrt (Konto) | disabled | `disabled` |
| Version der Einwilligung | consent version | `consentVersion` |
| Zeitpunkt der Einwilligung | consent at | `consentAt` |
| Einwilligung | consent | `PUT /me/consent`, field `version`; `Accounts.RecordConsent` |
| Zugangsschlüssel nach dem Login | token | `token`; `auth.NewToken` |
| Hash des Tokens (SHA-256, hex) | token hash | `tokenHash`; `auth.HashToken` |
| Ablaufzeitpunkt | expires at | `expiresAt` |
| Anlegezeitpunkt | created at | `createdAt` |
| Protokoll der Verwaltungsaktionen | audit log | collection `auditLog` |
| wer `cmd/users` ausführt (Betriebssystem-Nutzer und Rechner) | operator | `operator` (`"<os user>@<host>"`) |
| betroffener Benutzername | target username | `targetUsername` |
| Anmelden / Abmelden | log in / log out (nouns: login / logout) | `POST /auth/login`, `POST /auth/logout` |
| eigenes Konto | the logged-in user ("me") | `GET /me`, `PATCH /me`; `auth.UserFrom` (user from the request context) |
| Startpasswort | start password | generated by `cmd/users`; the account has `mustChangePassword: true` |
| Passwort ändern: bisheriges, neues Passwort | change the password: current password, new password | `POST /me/password`; `currentPassword`, `newPassword` |
| Fehlversuch | failed attempt | |
| Sperre nach Fehlversuchen, Wartezeit | lockout, retry after | `auth.Limiter`; header `Retry-After`, `details.retryAfterSeconds` |
| vertrauenswürdiger Proxy (z. B. Caddy) | trusted proxy | env `TRUSTED_PROXIES` |
| IP-Adresse des Clients | client IP | `auth.ClientIP` |
| Daten pro Nutzer, Eigentümer eines Eintrags | per-user data, owner | `userId` (ObjectId) on `entries`, `chunks`, `ocr`, `metadata.userId` on `originals.files`; Go `core.Entry.UserID` |
| fehlender Nutzer (Programmierfehler) | no user | `core.ErrNoUser` |
| Altdaten ohne Eigentümer | orphaned data (orphans) | documents without `userId`; visible to nobody |
| Altdaten einem Konto zuordnen | assign orphans | `cmd/users assign-orphans`; `store.AssignOrphans` |
| Tageslimit | daily limit | `core.LimitKind` (`uploads`, `questions`, `transcriptions`), `core.LimitReachedError`; `429 LIMIT_REACHED`, `details.limit` |
| Verbrauch, Zähler pro Tag | usage | collection `usage`; `service.Usage`, `store.IncrementUsage` |
| Kalendertag (Europe/Vienna) | day | `day` (`YYYY-MM-DD`); `core.UsageDay` |
| Art des Limits (Feld im Zähler) | kind | `kind` (same values as `details.limit`) |
| Suchindex wird neu gebaut | search index updating | `core.ErrSearchUpdating`; `503 MODEL_UNAVAILABLE` |

| Item | Value |
|---|---|
| login request | `username`, `password` |
| login response | `token`, `expiresAt`, `mustChangePassword`, `user` (`id`, `username`, `displayName`, `consentVersion`) |
| `GET /me` and `PATCH /me` response | `id`, `username`, `displayName`, `mustChangePassword`, `consentVersion` |
| `PATCH /me` request | `displayName` |
| `POST /me/password` request | `currentPassword`, `newPassword` |
| `PUT /me/consent` request | `version` |
| headers | `Authorization: Bearer <token>`, `WWW-Authenticate: Bearer`, `Retry-After`, `Cache-Control: no-store` (login) |
| error codes | `WRONG_PASSWORD`, `WEAK_PASSWORD`, `INVALID_CREDENTIALS`, `UNAUTHORIZED`, `PASSWORD_CHANGE_REQUIRED`, `TOO_MANY_ATTEMPTS`, `LIMIT_REACHED` |
| `details.limit` values | `uploads`, `questions`, `transcriptions` |
| `users` fields | `_id`, `username`, `displayName`, `passwordHash`, `role`, `mustChangePassword`, `disabled`, `createdAt`, `consentVersion`, `consentAt` |
| `sessions` fields | `_id`, `tokenHash`, `userId`, `createdAt`, `expiresAt` (the token itself is never stored) |
| `auditLog` fields | `_id`, `time`, `action`, `targetUsername`, `operator` (never passwords or tokens) |
| `auditLog.action` values | `add`, `reset_password`, `disable`, `enable`, `revoke_sessions`, `revoke_all_sessions`, `list`, `delete`, `assign_orphans` |
| `usage` fields | `_id`, `userId`, `day`, `kind`, `count`, `expiresAt` |
| env | `TRUSTED_PROXIES` (default empty: the client IP is the address of the connection; behind Caddy on the same machine `127.0.0.1,::1`); `LIMIT_UPLOADS_PER_DAY` (30), `LIMIT_QUESTIONS_PER_DAY` (60), `LIMIT_TRANSCRIPTIONS_PER_DAY` (30) |

### Commands (cmd/users)

| Command | Flags | Note |
|---|---|---|
| `add` | `--username`, `--display-name`, `--role` (`user` default, `admin`), `--password-stdin` | `user`: generated start password (12 characters), `mustChangePassword: true`; `admin`: generated password (24 characters), `mustChangePassword: false` |
| `add-batch` | `--prefix`, `--count` | e.g. `--prefix tester --count 10` creates `tester01` … `tester10` |
| `reset-password` | `--username`, `--password-stdin` | new password like `add`; also ends all sessions of the account |
| `disable`, `enable` | `--username` | `disable` also ends all sessions |
| `revoke-sessions` | `--username` or `--all` | |
| `list` | – | never shows hashes; column `entries` with the number of entries |
| `delete` | `--username` | the account, its sessions and all its data (entries, chunks, OCR words, originals, usage) |
| `assign-orphans` | `--username` | gives every entry, chunk, OCR document and original without an owner to this account; can be run again |

`--password-stdin` reads the password from standard input. A password is
never accepted as a command argument, so it does not end up in the shell
history.

### App (login)

Added 2026-11-18. Names in the app (`app/lib/`), all app-internal. Display
texts are German and quoted as they appear.

| German | English | Dart type / file |
|---|---|---|
| Anmeldung, Sitzung der App | session | `SessionController`, `SessionScope`; `session/session_controller.dart` |
| Zustand der Sitzung | session phase | `SessionPhase` (`loading`, `offline`, `signedOut`, `mustChangePassword`, `needsConsent`, `ready`) |
| gespeicherte Anmeldung (Token und Ablauf) | stored session | `StoredSession`; `session/session_store.dart` |
| sicherer Speicher für das Token | session store | `SessionStore`, `SecureSessionStore` (flutter_secure_storage) |
| zentrale Stelle für 401 und 403 | guarded API client | `GuardedApiClient`; `session/guarded_api_client.dart` |
| lokale Daten (temporäre Dateien, Bild-Cache) | local data | `LocalData`, `PluginLocalData`; `session/local_data.dart` |
| Generation (neuer Schlüssel der MaterialApp) | generation | `SessionController.generation` |
| angemeldeter Nutzer | user | `User`; `model/user.dart` |
| Antwort des Logins | login result | `LoginResult`; `model/user.dart` |
| Token setzen | use token | `ApiClient.useToken` |
| Anmelden (Bildschirm) | login screen | `LoginScreen`; „Anmelden“ |
| Eigenes Passwort festlegen / Passwort ändern | change password screen | `ChangePasswordScreen(mandatory: …)` |
| Einwilligung, Hinweis zum Datenschutz | consent screen | `ConsentScreen(readOnly: …)`; „Bevor du startest“, „Zustimmen“, „Nicht jetzt“ |
| Text und Version der Einwilligung | consent text, consent version | `consentTitle`, `consentParagraphs`, `consentVersion` (`"2026-11-26"`, with routines; before `"2026-11-23"`, `"2026-11-18"`); `session/consent_text.dart` |
| Einstellungen | settings screen | `SettingsScreen`; „Einstellungen“, `settingsIconSize` (36) |
| Passwortfeld mit Augen-Symbol | password field | `PasswordField`; `widgets/password_field.dart` |
| Passwortregeln der App | password rules | `checkNewPassword`, `minPasswordLength` (10), `maxPasswordBytes` (72); `session/password_rules.dart` |
| Wartezeit in Minuten | wait hint | `waitHint` („Bitte warte etwa 1 Minute.“ / „… 3 Minuten.“) |
| Hinweis nach Ende der Sitzung | relogin notice | `reloginNotice` („Bitte melde dich erneut an.“) |
| Meldung nach Passwortwechsel | password changed message | `passwordChangedMessage` („Passwort geändert. Andere Geräte wurden abgemeldet.“) |
| Anzeigename prüfen | check display name | `checkDisplayName`, `maxDisplayNameLength` (50) |
| Testkonten des Mocks | mock accounts | `tester01` / `MockApiClient.mockStartPassword`, `admin` / `MockApiClient.mockAdminPassword` |
| Auslöser Tageslimit im Mock | limit trigger | `limitreached` in file name, text, question or recording name |

## Routines (2026-11-23)

docs/API.md 3.16–3.29, section 2 "Routine suggestions", `timeZone` in
3.12/3.13. A routine is like an alarm clock for something done regularly;
its occurrences are reminded locally by the app, and logs record what was
done.

Two stages. **Stage 1 is binding since 2026-11-23** (implemented in the
backend). **Stage 2 is proposed** (not built): the routine suggestions:
`routineSuggestions`, `quote`, `timeStated`, `takenDate`, `takenTime`, the
routine key, confirm and reject, `ROUTINE_SUGGESTION_NOT_FOUND`,
`details.suggestionId`, `source: entry`, the index `user_entry`. Everything
else below is stage 1.

| German | English | Go / Dart type, field | Display text in the app |
|---|---|---|---|
| Routine / Routinen | routine / routines | `core.Routine`; Dart `Routine`; collection `routines` | „Routine“, „Routinen“, „Neue Routine“ |
| Zeitplan | schedule | `core.Schedule`; `schedule` | „Rhythmus“ |
| Häufigkeit | frequency: `hourly`, `daily`, `weekly` | `core.Frequency`; `schedule.frequency` | „stündlich“, „täglich“, „wöchentlich“ |
| Intervall (alle N Stunden, Tage, Wochen) | interval | `interval` | „alle 2 Stunden“, „jeden 2. Tag“, „jede 2. Woche“ |
| Uhrzeiten | times (`HH:MM`) | `times` | „Uhrzeiten“ |
| Wochentage | weekdays: `mon`, `tue`, `wed`, `thu`, `fri`, `sat`, `sun` | `weekdays` | „Mo“, „Di“, „Mi“, „Do“, „Fr“, „Sa“, „So“ |
| Zeitfenster (stündlich) | window start, window end | `windowStart`, `windowEnd` | „von 08:00 bis 20:00“ |
| Startdatum, Enddatum | start date, end date | `startDate`, `endDate` | „Beginn“, „Ende“ |
| Zielmenge pro Termin, Einheit | amount, unit | `amount`, `unit` | „Menge“, „Einheit“ |
| Termin (geplanter Zeitpunkt) | occurrence | `core.Occurrence`; Dart `Occurrence` | „Termin“ |
| fällig um | due at | `dueAt` (RFC 3339, UTC) | |
| Ortsdatum und Ortszeit eines Termins | date, time | `date`, `time` (in the user's time zone) | |
| Status eines Termins | `open`, `done`, `skipped`, `paused` | `status` | „offen“, „erledigt“, „ausgelassen“, „pausiert“ |
| Verlaufseintrag (never just „Eintrag“: that is a document) | log (routine log) | `core.RoutineLog`; Dart `RoutineLog`; collection `routineLogs` | „Verlaufseintrag“; the list „Verlauf“ |
| erledigt / ausgelassen | `done` / `skipped` | `status` | „erledigt“ / „ausgelassen“ |
| Zeitpunkt der Erledigung | taken at | `takenAt` (RFC 3339, UTC) | |
| tatsächliche Menge | amount | `amount` | |
| Herkunft eines Verlaufseintrags | source: `manual`, `notification`, `entry` | `source` (not the `sources` of an answer, not the verification `source`) | |
| Eintrag, aus dem ein Verlaufseintrag stammt | entry ID | `entryId` | |
| Ortsschlüssel eines Termins (nur MongoDB) | local due key | `dueLocal` (`YYYY-MM-DDTHH:MM`) | |
| Pause, pausieren, fortsetzen | pause, pause, resume | `pauses` (`from`, `until`); `POST …/pause`, `POST …/resume` | „Pausieren“, „Fortsetzen“, „pausiert bis …“ |
| Status einer Routine | `upcoming`, `active`, `paused`, `ended` | `status` | „geplant“, „aktiv“, „pausiert“, „beendet“ |
| Fortschritt: heute, Woche, Zeitraum | progress: `today`, `week`, `period` | `progress` | „heute 1 von 2“, „Woche 5 von 7“, „Tag 6 von 14“ |
| erledigt, ausgelassen, fällig (Zähler) | `done`, `skipped`, `due` | | |
| erledigte Menge, Zielmenge gesamt | amount done, amount target | `amountDone`, `amountTarget` | „heute 1.250 von 3.250 ml“ |
| Tag des Zeitraums, Zahl der Tage | day, days | `day`, `days` (only in `period`) | |
| Routinenvorschlag (am Eintrag) | routine suggestion | `routineSuggestions`; `core.RoutineSuggestion`; IDs `s1`, `s2`, … | „Vorschlag“ |
| Zitat (Grundlage eines Vorschlags) | quote | `quote` | |
| Uhrzeit im Text genannt | time stated | `timeStated` | „Uhrzeit nicht genannt“ |
| Datum und Uhrzeit laut Text (only in the model's answer) | taken date, taken time | `takenDate`, `takenTime` | |
| Kurzschlüssel einer Routine im Prompt | routine key | `r1`, `r2`, … | |
| bestätigen / ablehnen | confirm / reject | `POST …/confirm`, `DELETE …/routine-suggestions/{suggestionId}` | „Übernehmen“, „Ablehnen“ |
| Zuordnung eines Vorschlags zum Termin | assignment | | |
| Erinnerung (lokal in der App) | reminder | | „Erinnerung“; buttons „Ja“, „Nein“, „Andere Menge“ |
| Zeitzone des Nutzers | time zone | `timeZone` (IANA name, default `Europe/Vienna`) on the user and on each log | |

| Item | Value |
|---|---|
| routine fields | `id`, `module`, `name`, `amount`, `unit`, `schedule`, `startDate`, `endDate`, `status`, `pauses`, `createdAt`, `progress` |
| `schedule` fields | `frequency`, `interval`, `times`, `weekdays`, `windowStart`, `windowEnd` |
| occurrence fields | `routineId`, `dueAt`, `date`, `time`, `status`, `logId` |
| log fields | `id`, `routineId`, `dueAt`, `takenAt`, `status`, `amount`, `source`, `entryId`, `timeZone`, `createdAt` |
| suggestion fields | `id`, `routineId`, `amount`, `takenAt`, `timeStated`, `quote` |
| progress fields | `done`, `skipped`, `due`, `amountDone`, `amountTarget`; in `period` also `day`, `days` |
| list responses | `routines` + `count`; `occurrences` + `count` + `from` + `to`; `logs` + `count` |
| request fields | pause: `until`; log: `status`, `source`, `dueAt`, `takenAt`, `amount`; confirm: `amount`, `takenAt`, `dueAt`; `PATCH /me`: `timeZone` |
| query parameters | `module`, `from`, `to` |
| error codes | `ROUTINE_NOT_FOUND`, `ROUTINE_LOG_NOT_FOUND`, `ROUTINE_SUGGESTION_NOT_FOUND`, `ROUTINE_LOG_EXISTS`, `ROUTINE_NAME_TAKEN`, `ROUTINE_LIMIT_REACHED` |
| `details` keys | `routineId`, `logId`, `suggestionId`, `max`; `limit`: `routines`, `pauses`, `logs` |
| app packages | `flutter_local_notifications` 23.0.4 (reminders), `flutter_timezone` 5.1.1 (IANA name of the phone; `DateTime.timeZoneName` only gives abbreviations), `timezone` 0.11.2 (zone database for the mock and for planning in UTC) |
| backend, internal only | `core.Date` (plain date, days since 1970-01-01), `core.ClockTime` (minutes after midnight), `core.RoutineView` (routine with status and progress), `core.ParameterError` (`INVALID_PARAMETER`), `core.CheckTimeZone`, `core.UserTimeZone`, `core.DayStart`; `service.Routines`; test package `internal/routinetest`; test cases `backend/internal/core/testdata/routine_occurrences.json` |

### App (routines)

Added 2026-11-27. Names in the app (`app/lib/`), all app-internal. Display
texts are German and quoted as they appear.

| German | English | Dart type / file |
|---|---|---|
| Reiter einer Modul-Shell | module page | `ModulePage` (`timeline`, `routines`, `upload`, `questions`); `screens/module_shell.dart` |
| Reiter „Routinen“ | routines screen | `RoutinesScreen`; „Routinen“, „Heute“, „Weitere Routinen“, „Neue Routine“, „Noch keine Routinen“ |
| Formular einer Routine | routine form screen | `RoutineFormScreen`; „Neue Routine“, „Routine bearbeiten“, „Speichern“ |
| Detailseite einer Routine | routine detail screen | `RoutineDetailScreen`; „Fortschritt“, „Termine heute“, „Nächste Termine“, „Verlauf“, „Aktionen“, „Zusätzlich eintragen“ |
| Termin eintragen (Blatt) | occurrence sheet | `OccurrenceSheet`, `showOccurrenceSheet`; „Termin: 08:00“, „Ja, erledigt“, „Nein, ausgelassen“, „Menge speichern“ |
| Termin eintragen (Anfrage) und Ergebnis | log an occurrence, log result | `logOccurrence`, `LogResult` (`409 ROUTINE_LOG_EXISTS` = already logged, shown as information); `screens/routine_actions.dart` |
| Meldung unten, mit „Rückgängig“ | routine snack bar | `showRoutineSnackBar`, `undoSnackBarDuration` (8 s); `screens/routine_actions.dart` |
| Vorschläge für die Einheit | unit suggestions | `unitSuggestions` („Stück“, „ml“, „g“, „mg“, „Tropfen“); `screens/routine_form_screen.dart` |
| größtes Intervall pro Häufigkeit | max intervals | `maxIntervals` |
| Texte zu Routinen | routine texts | `model/routine_texts.dart`: `describeSchedule` („täglich um 08:00“), `describeRoutinePlan`, `intervalLabel` („jede Stunde“, „jeden 3. Tag“), `formatAmount` and `parseAmount` (German numbers: „1.250“, „0,5“), `amountWithUnit` („2 Stück“), `amountText` and `countText` („heute 1 von 2“), `periodDayText` („Tag 6 von 14“), `occurrenceWhen`, `weekdayDate` („Fr, 27.11.“), `weekdayLabels`, `routineStatusLabel`, `occurrenceStatusLabel` |
| Zeitzone des Handys | device time zone | `DeviceTimeZone`, `PluginDeviceTimeZone` (flutter_timezone); `platform/device_time_zone.dart`; `SessionController.updateTimeZone` |
| Abgleich von Zeitzone und Erinnerungen | routine sync | `RoutineSync`, `RoutineSyncScope`; `reminders/routine_sync.dart`; `refresh`, `clear`, `pendingTap` (a tap on a reminder not yet handled), `dataChanged` (the tab reloads) |
| Erinnerungen (Schnittstelle zum Paket) | reminders | `Reminders`, `PluginReminders`, `ReminderPermission` (`granted`, `denied`); `platform/reminders.dart` |
| Erinnerungsplan | reminder plan | `planReminders`, `PlannedReminder`, `maxPlannedReminders` (60); `reminders/reminder_plan.dart` |
| neu planen vom Server | replan reminders | `replanReminders` (loads, plans, applies; used by `RoutineSync` and the reminder buttons in the background); `reminders/replan_reminders.dart`; `ModuleCatalog.activeIds` |
| Erinnerung für heute (nur Debug) | reminder for today | `reminderForToday`, `RoutineSync.showNextNow`; „Nächste Erinnerung jetzt zeigen“, „Heute ist kein Termin mehr offen.“ |
| ID und Inhalt einer Erinnerung | reminder ID, occurrence key, payload | `reminderId` (31-bit FNV-1a), `occurrenceKey` (`"<routineId>\|<dueAt>"`), `reminderPayload` (`{"m": module, "r": routineId, "d": dueAt}`, no name) |
| Antippen einer Erinnerung oder eines Knopfs | reminder tap | `ReminderTap` |
| Knöpfe der Erinnerung | reminder actions | `reminderActionDone` (`done`, „Ja“), `reminderActionSkipped` (`skipped`, „Nein“), `reminderActionAmount` (`amount`, „Andere Menge“) |
| Knöpfe im Hintergrund | reminder button in the background | `reminderButtonInBackground` (entry point of the isolate), `logFromReminderButton`, `reminderPortName` (`lifelog_reminder_logged`); `reminders/reminder_action.dart` |
| Kanal der Erinnerungen (Android) | notification channel | `routine_reminders`, „Erinnerungen an Routinen“; icon `ic_stat_lifelog` |
| Kategorien der Knöpfe (iOS) | notification categories | `routine_reminder` („Ja“, „Nein“), `routine_reminder_amount` (also „Andere Menge“) |
| Hinweis ohne Erlaubnis | reminders off notice | „Erinnerungen sind ausgeschaltet. Du kannst Routinen trotzdem eintragen.“, „Einschalten“ |
| Frage nach der Erlaubnis | permission question | „Erinnerungen“, „Damit Lifelog dich an deine Routinen erinnern kann, braucht es die Erlaubnis für Mitteilungen.“, „Später“, „Weiter“ |
| Routinen im Mock | mock routines | `MockRoutines`, `expandSchedule`, `addSamples` („Vitamin D“, „Blutdruck messen“, „Wasser“); `api/mock/mock_routines.dart`; `ensureTimeZones`, `isAcceptedTimeZone`, `locationOrNull`; `api/mock/mock_time_zones.dart` |
