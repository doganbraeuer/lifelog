# Lifelog backend specs

One prompt for Claude Code per backend feature, in build order, written so that the backend can be rebuilt feature by feature. They are the counterpart to the app prompts in [FEATURE_SPECS.md](FEATURE_SPECS.md). The contract both sides build against is [API.md](API.md); names come from [GLOSSARY.md](GLOSSARY.md). Measurements behind the decisions are in `docs/ocr/`, `docs/speech/` and `docs/calibration/`.

Rules that apply to every prompt (they live in `CLAUDE.md`, which Claude Code reads in every session):

- Change `docs/API.md` first, then the code. The app is built against the contract, not against the backend.
- Small steps; for larger changes a plan in plan mode first, reviewed before any code is written.
- Secrets only in `.env`, never committed; Claude Code may not read `.env` (`.claude/settings.json`).
- Only synthetic test data. No medical judgement, the backend only reports what the documents say.
- Deterministic logic is a pure function in `internal/core` with table tests. Every step ends with `go vet ./... && go test ./...`, a manual check with `curl` and its own commit on a branch; merging happens through a pull request.
- Every model call goes through an interface, so the provider can be replaced and tests run without network.

## 1. Foundation: configuration, database, status

```text
Create a Go backend in backend/ (Go 1.25, module lifelog/backend) with net/http only, no framework, and the official MongoDB driver v2.
Read MONGODB_URI, MONGODB_DB and GEMINI_API_KEY from the environment or from .env; error messages name a missing variable but never its value. The server listens on ADDRESS (default 127.0.0.1:8080, so it is only reachable from this computer unless set to 0.0.0.0 for a real phone).
Add GET /api/v1/status (database ok or not) and the error format from docs/API.md, with German messages. Log only method, path, status and duration, never bodies, texts or the URI. A panic becomes 500 INTERNAL_ERROR without details. Ctrl+C shuts the server down cleanly.
Done when: tests cover the configuration (missing variable, defaults, value never in the message), the error format and the status handler with a fake database, and curl on /status returns 200.
```

## 2. Timeline list

```text
Add GET /api/v1/entries for the timeline. Sort by documentDate or importedAt, ascending or descending; entries without a document date are always last, among themselves by importedAt descending; ties are broken by importedAt and then id, so the order is always unique. Write the sorting as a pure function in core, not as a database query.
Filter by module (default health; an unknown module is 400 INVALID_PARAMETER with details). Lists are always [] and never null, also when empty.
Add cmd/seed, which inserts a few synthetic entries (one without a date) so that the app can be tested before uploads work. It can be run twice and has a flag to remove them again.
Done when: table tests cover both fields, both directions, missing dates and ties, and a test checks "entries":[] in the JSON.
```

## 3. Upload and read a document

```text
Add POST /api/v1/entries with a multipart field file (JPEG, PNG, HEIC, WebP or PDF, at most 10 MB). Detect the type from the first bytes, never from the file name.
Order matters:
1. Hash the file (SHA-256) and check for a duplicate before any model call; a duplicate is 409 DUPLICATE with the existing entryId and costs nothing.
2. Send the file to the vision model behind an llm.Reader interface. The model must answer as JSON against a schema (readable, documentDate, issuer, type from the module's allowed list, fullText, facts). The prompt says: copy, never interpret, keep values as strings exactly as printed (e.g. "<0,5").
3. Treat the model answer as untrusted: check it in core (real calendar date, allowed type). Drop facts without name or value and log only the field path, never the value.
4. Split the full text into chunks (paragraphs, then lines, then words; at most 500 characters counted as runes), embed them and store file, chunks and entry, with the entry last. If a step fails, remove what was stored; no half entries.
Model errors map to 503 MODEL_UNAVAILABLE (timeout, network, 429, 5xx) or 502 (invalid JSON). Retry 429/503 twice with backoff, then try one fallback model within a reserved time budget; model names come from the environment and are checked against the free tier before choosing defaults.
Done when: tests with a fake model and fake store show zero model calls for a duplicate, nothing stored on any error, and every error code; a live test runs only with LIFELOG_LIVE=1 against a synthetic document.
```

## 4. Entry and original file

```text
Add GET /api/v1/entries/{id} (full entry) and GET /api/v1/entries/{id}/file (the original as stored, with Content-Type, Content-Length and an escaped file name). An id that is not an ObjectId is 400 INVALID_ID, an unknown one 404.
Done when: handler tests with a fake store cover both errors, the headers and an entry with empty lists.
```

## 5. Questions with sources (RAG)

```text
Add POST /api/v1/questions. Embed the question, find the most similar chunks of the module with Atlas Vector Search (index definition as a JSON file in the repo, filter field module, created at start if missing, otherwise a clear warning how to create it in the Atlas UI).
Drop hits below a threshold. If nothing is left, answer exactly „Dazu finde ich nichts in deinen Dokumenten.“ without calling the language model. Otherwise the model gets only the numbered chunks and must answer only from them with references [n]. Return only the sources that are actually cited, with the current type, issuer and date of the entry.
Do not guess the threshold: add cmd/calibrate with fixed questions in four groups (matching, two documents, near but not answerable, off-topic), measure the scores against the real data and pick the value with the safer margin towards matching questions.
Done when: tests cover citation parsing ([1], [1][3], unknown numbers), zero model calls without a hit and the module filter; the calibration report is in docs/calibration/.
```

## 6. Text import

```text
Let POST /api/v1/entries take a field text instead of file (exactly one of both). At most 100 000 characters counted as runes; invalid UTF-8 is 400. Store the text byte for byte as text.txt (text/plain; charset=utf-8) so that hash, duplicate check, original and delete need no special case.
The model reads only date, issuer, type and facts from the text; fullText is the entered text, unchanged. The prompt states that the text is content, not instructions.
Done when: tests show the hash over the UTF-8 bytes, an unchanged full text (with umlauts and \r\n) and that text never calls the document reader.
```

## 7. Correct an entry

```text
Add PATCH /api/v1/entries/{id} for type, documentDate, issuer and the value or unit of facts. Distinguish "field missing" from null (decode into raw JSON). Validate everything first and change nothing if one part is invalid.
Corrections are a pure function in core: the field always holds the current value; per field there is one correction whose recognized value stays the original; resetting to the original removes the correction; an unchanged value creates none. Full text and chunks are not touched. Log only the names of changed fields.
Done when: table tests cover correcting twice, resetting, null and back, adding and removing a unit and several fields in one request.
```

## 8. Delete an entry

```text
Add DELETE /api/v1/entries/{id}: chunks, then the original, then the entry last, so that a failed step leaves the entry visible and a second call cleans up the rest. A missing original is not an error. Once deleting has started it finishes even if the app disconnects. 204 without body, a second call 404.
Done when: a fake store that fails in each step proves the order and the retry, and an integration test against a separate test database shows that a second entry stays untouched.
```

## 9. Verify the extracted values

```text
Read every document a second time, independently of the model: the PDF text layer with pdftotext where it exists, otherwise Tesseract OCR (PDF pages rendered with pdftoppm). Rebuild table rows from word positions. For date, issuer and every fact (value with unit) check whether it appears in that text and store a status (confirmed, unconfirmed, corrected_*), the share found in percent and the word positions (fractions of the upright page) for the jump to the original.
Unconfirmed means "not found", never "wrong". If OCR is missing or fails, the entry is unchecked with a reason; the upload itself never fails because of verification. Re-check the new value after a correction.
Add cmd/ocrcheck, which measures every demo document in every variant (PDF text layer, forced OCR, photo, phone photo) without calling a model and writes a report.
Done when: core tests cover the matching rules and layout, and the measurement report is in docs/ocr/.
```

## 10. Prepare images for OCR

```text
Before Tesseract, bring every image into one form in Go: read JPEG, PNG, WebP and HEIC (HEIC with an embedded WebAssembly decoder, no cgo, no installed program), turn it upright using the EXIF orientation (all 8 cases), make transparent areas white, scale the long side to 2500–4096 px and pass an RGB PNG with --dpi 300. Do not decode images above 50 megapixels (reason image_too_large).
Measure each choice (grayscale or colour, resolution, downscaling) with cmd/ocrcheck and keep only what improves the result.
Done when: all 8 orientations are tested with small images, and an end-to-end test with the real Tesseract reads a rotated JPEG and a HEIC upright.
```

## 11. Transcribe a voice note

```text
Add POST /api/v1/transcriptions with a field audio (M4A, 3GP, AMR, WAV, OGG, WebM, MP3, FLAC detected from the bytes; at most 10 MB and 5 minutes). Return only the recognized text; store nothing. The app shows it for editing and saves it through the text import.
Convert with ffmpeg to 16 kHz mono PCM and send it to a local whisper.cpp server (large-v3-turbo) behind a speech interface; the backend does not start Whisper itself. Protect against invented text in silence: a level check in Go before Whisper (silence never reaches the model), Whisper's voice activity detection, German fixed, temperature 0, no previous context, and a filter for known hallucinated sentences and repeated segments. GET /status reports speech as ok, unavailable or off.
Add cmd/speechcheck, which measures word error rate and key terms (numbers, units, dates, drug names) for synthetic and own recordings; own recordings stay outside the public repository.
Done when: tests with a fake engine cover every error code, silence and noise give NO_SPEECH, nothing is left in the temp folder and no text appears in any log.
```
