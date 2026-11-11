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

Manual upload (synthetic documents only):

```sh
curl -F file=@testdata/lab_report.pdf localhost:8080/api/v1/entries   # file
curl -F "text=<mytext.txt" localhost:8080/api/v1/entries              # text import
```

For the text import, `curl` reads the file content with `<` into the form
field `text` (not as a file attachment). It is stored as `text.txt`.

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
- Photos and PDF pages without a text layer: `pdftoppm` renders the page,
  Tesseract (`TESSERACT_LANG`, default `deu`) reads it. Images with a long
  side under 2500 pixels are scaled up first (at most twice the size),
  chosen by measurement (`docs/ocr/README.md`).

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
go run ./cmd/ocrcheck -extra ~/Desktop/01_lab_report_2026-01-20_phone.jpg   # a real photo
```

`cmd/ocrcheck` calls no model. Results and their assessment: `docs/ocr/`.

## Vector index

Search (`POST /api/v1/questions`) needs the Atlas Vector Search index
`chunks_vector` on the collection `chunks`. The definition is in
[`internal/store/chunks_vector.json`](internal/store/chunks_vector.json):
`embedding` as vector (768 dimensions, cosine), `module` as filter field.

**Option A (automatic):** the server creates the index on start if it is
missing and logs its status. Atlas builds it in the background (about 30 s in
our tests); until it is `READY`, search finds nothing. Tested on the free
tier (M0) on 2026-11-04.

**Option B (Atlas UI),** if option A fails (warning on start):

1. Open the cluster in Atlas, tab "Atlas Search" (depending on the UI
   "Search & Vector Search"), "Create Search Index".
2. Choose "Atlas Vector Search" and "JSON Editor".
3. Database = value of `MONGODB_DB`, collection `chunks`, name
   `chunks_vector`.
4. Paste the content of `internal/store/chunks_vector.json`, save.
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
upload and without the vision LLM:

```sh
go run ./cmd/seed              # 4 invented entries with chunks and embeddings (can be run again)
go run ./cmd/seed -no-chunks   # the same without Gemini; search does not find them then
go run ./cmd/seed -remove      # delete them again, with their chunks
```

The seed entries (`internal/sampledata`) have fixed IDs. Other entries stay
untouched. The chunks are created as on upload: same splitting, same
embedding model.

To try out `DELETE /entries/{id}` (e.g. from the app):

```sh
go run ./cmd/seed -throwaway                            # creates a throwaway entry with a new ID and prints the ID
curl -i -X DELETE localhost:8080/api/v1/entries/<ID>   # 204, a second time 404
```

The throwaway entry needs no Gemini: its chunk has a fixed vector instead of
a real embedding, and search does not show it. Only delete the printed ID,
not the fixed seed entries or real uploads.

## Calibrate the threshold

```sh
../demo/upload.sh                       # once: seed entries and demo uploads
go run ./cmd/calibrate                  # evaluates with core.MinScore
go run ./cmd/calibrate -threshold 0.82
```

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
| `cmd/calibrate` | Measure vector search scores for fixed questions (threshold) |
| `cmd/models` | List available Gemini models |
| `cmd/ocrcheck` | Fill in (`-write`) or redo (`-recheck -write`) the verification of stored entries, measure OCR on `demo/` |
| `internal/sampledata` | The invented seed entries, shared by seed and calibrate |
| `internal/config` | Read and check environment variables, mask secrets |
| `internal/core` | Domain logic without I/O: types, modules, sorting, file type, chunks, checking the extraction, questions, corrections, verification against the original |
| `internal/llm` | Replaceable interface to the models (`Reader`, `Embedder`, `Answerer`) |
| `internal/llm/gemini` | Implementation with Gemini, including retry and fallback model |
| `internal/llm/fake` | Fake models for tests |
| `internal/ocr` | Replaceable second reader: `Recognizer` (file), `Engine` (image), `PDF`; `Pipeline` uses the text layer where possible and OCR where needed |
| `internal/ocr/poppler`, `internal/ocr/tesseract` | Implementations with the command line programs (stdin/stdout, no temporary files) |
| `internal/ocr/fake` | Fake second reader for tests |
| `internal/service` | Use cases: upload, text import, question, correction, delete |
| `internal/store` | MongoDB: entries, chunks with embeddings, words read from the originals (`ocr`), original files (GridFS), indexes |
| `internal/api` | Router, handlers, error format, JSON types |
