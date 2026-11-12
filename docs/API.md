# Lifelog REST API (prototype)

Status: 2026-11-11 · Version `v1` · Draft
(Changes: see [section 5](#5-changes))

This document defines the interface between the Flutter app (`app/`) and the
Go backend (`backend/`). Both sides build against this contract. Changes to
the API are made here first.

All examples contain synthetic data only.

---

## 1. Principles

### Basics

- Base path: `/api/v1`
- Requests and responses are JSON in UTF-8
  (`Content-Type: application/json; charset=utf-8`). Exceptions are the
  upload (`multipart/form-data`), fetching the original file (binary data)
  and deleting (response without a body).
- Field names are English and follow [GLOSSARY.md](GLOSSARY.md) and the data
  model in `CLAUDE.md`.
- Entry IDs are MongoDB ObjectIds as a 24-character hex string,
  e.g. `"6727a1f3c2b4e5d6f7a8b9c0"`.
- The prototype has no login. The backend knows exactly one user
  (see [open points](#4-open-points)).

### Modules

Every entry belongs to exactly one module (field `module`), e.g. `health`.
Upload, list and question accept an optional `module`. If it is missing,
`health` applies.

In the prototype the backend only knows `health`. Any other value is an
error (`400`, see the individual endpoints). Further modules (planned in the
app: `nutrition`, `recipes`, `sport`) only become valid once their types are
defined.

### Dates and times

| Field | Format | Example | Note |
|---|---|---|---|
| `documentDate` | `YYYY-MM-DD` | `"2026-10-14"` | Plain date without time and without time zone. `null` if the vision LLM recognizes no date. |
| `importedAt` | RFC 3339, UTC | `"2026-11-03T18:42:07Z"` | Set by the backend on upload. Always present. |
| `correctedAt` (correction) | RFC 3339, UTC | `"2026-11-03T19:05:31Z"` | Set by the backend when correcting. |

Why a plain date for `documentDate`? A report from 14 October should always
appear on 14 October. A timestamp could shift by one day depending on the
device's time zone.

### Error format

Every error has the same structure. The app evaluates `code`. `message` is a
German text that the app may show directly. `details` is optional and
depends on the code.

```json
{
  "error": {
    "code": "ENTRY_NOT_FOUND",
    "message": "Diesen Eintrag gibt es nicht.",
    "details": {}
  }
}
```

### All error codes

| HTTP | Code | When |
|---|---|---|
| 400 | `INVALID_REQUEST` | JSON not readable, required field missing, wrong format, unknown field, unknown `module`, upload with both `file` and `text` |
| 400 | `INVALID_PARAMETER` | Unknown value for a query parameter |
| 400 | `INVALID_ID` | ID is not a valid ObjectId |
| 400 | `FILE_MISSING` | Upload without `file` and without `text`, or the content is empty |
| 400 | `FIELD_NOT_CORRECTABLE` | Correction of a field that is not (yet) correctable, e.g. `fullText` |
| 400 | `QUESTION_MISSING` | Question missing or only whitespace |
| 400 | `QUESTION_TOO_LONG` | Question longer than 500 characters |
| 404 | `ENTRY_NOT_FOUND` | No entry with this ID |
| 404 | `FACT_NOT_FOUND` | Correction refers to an unknown fact ID |
| 409 | `DUPLICATE` | Exactly this file (or exactly this text) was already uploaded |
| 413 | `FILE_TOO_LARGE` | File larger than 10 MB or text longer than 100,000 characters |
| 415 | `UNSUPPORTED_FILE_TYPE` | Not JPEG, PNG, HEIC, WebP or PDF |
| 422 | `UNREADABLE` | The vision LLM recognizes no document in the file |
| 500 | `INTERNAL_ERROR` | Unexpected error in the backend, e.g. database unreachable |
| 502 | `EXTRACTION_FAILED` | The vision LLM returns a response that does not match the expected JSON |
| 503 | `MODEL_UNAVAILABLE` | Language or embedding model does not respond or reports a limit |

### Important rules

- The app never calls a model directly. All calls to the vision LLM, the
  language model and the embedding model go through the backend.
- The API gives no medical assessments or recommendations. It only returns
  what is in the documents.
- Lists are always an array. An empty list is `[]`, never `null` and never a
  missing field. This applies to `entries`, `facts`, `corrections` and
  `sources`.

---

## 2. Data model: entry

### Full entry

This is how `POST /entries`, `GET /entries/{id}` and `PATCH /entries/{id}`
return an entry:

```json
{
  "id": "6727a1f3c2b4e5d6f7a8b9c0",
  "module": "health",
  "type": "lab_report",
  "documentDate": "2026-10-14",
  "importedAt": "2026-11-03T18:42:07Z",
  "issuer": "Labor Muster GmbH, Salzburg",
  "fullText": "Labor Muster GmbH\nMusterstraße 1, 5020 Salzburg\n\nLaborbefund vom 14.10.2026\nPatient: Max Mustermann, geb. 01.01.1990\n\nHämoglobin 14,2 g/dl (Ref. 13,5-17,5)\nLeukozyten 6,8 G/l (Ref. 4,0-10,0)\nCRP <0,5 mg/dl (Ref. <0,5)\nBefundart: Blutbild klein",
  "facts": [
    { "id": "f1", "name": "Hämoglobin", "value": "14,2", "unit": "g/dl" },
    { "id": "f2", "name": "Leukozyten", "value": "6,8", "unit": "G/l" },
    { "id": "f3", "name": "CRP", "value": "<0,5", "unit": "mg/dl" },
    { "id": "f4", "name": "Befundart", "value": "Blutbild klein" }
  ],
  "file": {
    "id": "6727a1f3c2b4e5d6f7a8b9c1",
    "fileName": "befund_oktober.pdf",
    "mimeType": "application/pdf",
    "size": 184320,
    "sha256": "3f8a1c0e9b7d2f4a6c5e8b1d0a9f7e3c2b4d6a8f0e1c3b5d7a9f2e4c6b8d0a1f"
  },
  "corrections": [
    {
      "field": "facts.f2.value",
      "recognized": "6,3",
      "corrected": "6,8",
      "correctedAt": "2026-11-03T19:05:31Z"
    }
  ],
  "verification": {
    "status": "checked",
    "reason": null,
    "source": "pdf_text",
    "engine": "pdftotext 26.11.0",
    "checkedAt": "2026-11-03T19:05:31Z",
    "confirmed": 5,
    "corrected": 1,
    "total": 6,
    "matchPercent": 83,
    "fields": [
      { "field": "documentDate", "status": "confirmed", "source": "pdf_text", "confidence": null,
        "locations": [{ "page": 1, "x": 0.2803, "y": 0.1821, "width": 0.1105, "height": 0.0142 }] },
      { "field": "issuer", "status": "confirmed", "source": "pdf_text", "confidence": null,
        "locations": [{ "page": 1, "x": 0.1210, "y": 0.0702, "width": 0.2104, "height": 0.0160 },
                      { "page": 1, "x": 0.3391, "y": 0.0911, "width": 0.0802, "height": 0.0139 }] },
      { "field": "facts.f1", "status": "confirmed", "source": "pdf_text", "confidence": null,
        "locations": [{ "page": 1, "x": 0.3012, "y": 0.2950, "width": 0.1120, "height": 0.0145 }] },
      { "field": "facts.f2", "status": "corrected_confirmed", "source": "pdf_text", "confidence": null,
        "locations": [{ "page": 1, "x": 0.3018, "y": 0.3157, "width": 0.0988, "height": 0.0143 }] },
      { "field": "facts.f3", "status": "unconfirmed", "source": null, "confidence": null, "locations": [] },
      { "field": "facts.f4", "status": "confirmed", "source": "pdf_text", "confidence": null,
        "locations": [{ "page": 1, "x": 0.2590, "y": 0.3571, "width": 0.1702, "height": 0.0151 }] }
    ]
  }
}
```

### Fields

| Field | Type | Note |
|---|---|---|
| `id` | String | ObjectId of the entry |
| `module` | String | Required. Area the entry belongs to, see [modules](#modules). Comes from the upload (field `module`, default `health`), not from the vision LLM. |
| `type` | String | Document type within the module. For `health` one of `doctor_letter`, `lab_report`, `prescription`, `other` |
| `documentDate` | String or `null` | Date on the document, see section 1 |
| `importedAt` | String | Time of the upload |
| `issuer` | String or `null` | Issuing body. `null` if not recognized |
| `fullText` | String | Text of the document as read by the vision LLM. For a text import, the entered text, unchanged. |
| `facts` | List | Extracted facts, see below |
| `file` | Object | Reference to the original file |
| `corrections` | List | Corrected fields with the originally recognized value |
| `verification` | Object | Check of the recognized values by a second reader (PDF text layer or OCR), see [verification](#verification). Always present. |

**`facts`**

| Field | Type | Note |
|---|---|---|
| `id` | String | Stable ID within the entry (`f1`, `f2`, …). Assigned by the backend on upload, without gaps even if it drops incomplete facts (see 3.1). Facts are corrected via this ID. |
| `name` | String | Label, e.g. `"Hämoglobin"` |
| `value` | String | Always a string, exactly as on the document (`"14,2"`, `"<0,5"`, `"positiv"`). Nothing is lost by converting to numbers. |
| `unit` | String, optional | Missing if the document names no unit |

**`file`**

| Field | Type | Note |
|---|---|---|
| `id` | String | ID of the stored original file |
| `fileName` | String | Name at upload. For a text import `text.txt`. |
| `mimeType` | String | Detected by the backend from the content, not from the file extension. For a text import `text/plain; charset=utf-8`. |
| `size` | Number | Size in bytes |
| `sha256` | String | SHA-256 hash of the file content as a hex string (64 characters). Used for duplicate detection. |

The file itself is not part of the JSON. The app loads it via
`GET /entries/{id}/file`.

**`corrections`**

| Field | Type | Note |
|---|---|---|
| `field` | String | Path of the corrected field: `type`, `documentDate`, `issuer`, `facts.<id>.value` or `facts.<id>.unit` |
| `recognized` | String or `null` | Value originally recognized by the vision LLM |
| `corrected` | String or `null` | Current, corrected value |
| `correctedAt` | String | Time of the last correction of this field |

Rules for corrections:

1. The fields of the entry (`documentDate`, `facts[].value` etc.) always
   contain the **current** value. The app does not have to combine anything
   for display.
2. There is at most one item in `corrections` per field. `recognized` is
   always the original value from the vision LLM, even if a field is
   corrected several times. `corrected` and `correctedAt` show the latest
   state.
3. If someone sets a field back to the recognized value, the backend removes
   the item from `corrections`. The field then counts as not corrected.
4. The app sees whether a fact was corrected by checking whether
   `corrections` contains a `field` starting with `facts.<id>.`.

These rules are deterministic and are covered by tests in the backend.

### Verification

The vision LLM can misread or invent a value. A second, independent reader
therefore checks the recognized values. It runs locally in the backend
(open source, no extra model call) and reads the original in one of three
ways:

| Source | Used for | How |
|---|---|---|
| `pdf_text` | PDF pages with a text layer (digitally created PDFs) | Words and positions are read from the PDF itself (`pdftotext -bbox-layout`). Exact, no recognition errors. |
| `ocr` | Photos (JPEG, PNG) and PDF pages without a text layer (scans) | The page is rendered as an image (`pdftoppm`) and read with OCR (Tesseract). |
| `text` | Text import | The entered text itself |

A PDF page counts as having a text layer if its text layer contains at
least 20 characters (without whitespace). This is decided per page: a PDF
with a digital first page and a scanned second page uses both sources.

For each field the backend looks for the value in the text read this way.
The app can show which values are confirmed, by which source, and jump to
the place in the original.

**`verification`**

| Field | Type | Note |
|---|---|---|
| `status` | String | `checked` or `unchecked` |
| `reason` | String or `null` | Only for `unchecked`, see below. `null` for `checked`. |
| `source` | String or `null` | What was compared against: `pdf_text`, `ocr`, `text`, or `mixed` if the pages of a PDF used both `pdf_text` and `ocr`. `null` for `unchecked`. |
| `engine` | String or `null` | Programs with version (and language for OCR), e.g. `"pdftotext 26.11.0"`, `"tesseract 5.5.4 deu"`, for `mixed` both separated by a comma. `null` for a text import and for `unchecked`. |
| `checkedAt` | String or `null` | Time of the last check (RFC 3339, UTC). `null` for `unchecked`. |
| `confirmed` | Number | Number of fields whose current value was found in the original (`confirmed` and `corrected_confirmed`) |
| `corrected` | Number | Number of fields the user corrected (`corrected_confirmed` and `corrected_unconfirmed`) |
| `total` | Number | Number of checked fields |
| `matchPercent` | Number or `null` | `confirmed / total` in percent, rounded to a whole number. `null` for `unchecked` and when `total` is 0. |
| `fields` | List | One item per checked field. `[]` for `unchecked`. |

**`fields[]`**

| Field | Type | Note |
|---|---|---|
| `field` | String | `documentDate`, `issuer` or `facts.<id>`. Order: date, issuer, facts in the order of `facts`. |
| `status` | String | `confirmed`: found in the original. `unconfirmed`: not found. `corrected_confirmed`: the user corrected the field, and the corrected value was found in the original. `corrected_unconfirmed`: the user corrected the field, and the corrected value was not found. |
| `source` | String or `null` | How the field was confirmed: `pdf_text`, `ocr` or `text` (the source of the page of the first location). `null` if the value was not found. |
| `confidence` | Number or `null` | Lowest OCR confidence (0–100) of the matched words. Only for `source: ocr`; `null` for `pdf_text`, `text` and if the value was not found. |
| `locations` | List | Where the value is in the original, best match first, at most 3. `[]` if the value was not found. |

A location in a **file** (`pdf_text` or `ocr`) is a rectangle on a page:

| Field | Type | Note |
|---|---|---|
| `page` | Number | Page, starting at 1. A photo has one page. |
| `x`, `y` | Number | Top left corner as a fraction of the page width and height (0–1, origin top left) |
| `width`, `height` | Number | Size as a fraction of the page width and height |

Fractions instead of pixels or PDF points, so the location does not depend
on the resolution: the app multiplies them by the size at which it shows the
page or photo. Text layer and OCR give the same kind of rectangle. A value
that spans several lines has one location per line.

For a **photo**, the page is the photo **as it is shown upright**: the
backend applies the rotation stored in the file before OCR (EXIF
orientation in a JPEG, the rotation in a HEIC file), like image viewers do.
An iPhone stores a portrait photo as a landscape image plus "rotate by
90°"; the locations refer to the portrait view. The app must therefore
show the photo with this rotation applied (Flutter's `Image` does this for
JPEG; HEIC is decoded by the platform, which also applies it). The stored
original file is not changed.

A location in a **text import** (`source: text`) is a range in `fullText`:
`{ "start": 37, "end": 47 }`. `start` is inclusive, `end` exclusive, both
counted in Unicode code points (not bytes; in Dart `fullText.runes`).
Punctuation at the edges is not part of the range.

A field confirmed on a photo looks like this:

```json
{ "field": "facts.f1", "status": "confirmed", "source": "ocr", "confidence": 93,
  "locations": [{ "page": 1, "x": 0.3012, "y": 0.2950, "width": 0.1120, "height": 0.0145 }] }
```

Which fields are checked:

- `documentDate` and `issuer`, unless they are `null`.
- Every fact: its `value`, and its `unit` if it has one. A fact with a unit
  only counts as `confirmed` if value and unit are both found.

How the backend compares (deterministic, covered by tests; the same for
all three sources):

1. Both sides are normalized: Unicode NFKC, lowercase, uniform dashes and
   quotation marks, punctuation at the edges of a word removed (`<`, `>`,
   `%`, `/`, `+` and `-` stay), a decimal comma between digits counts as a
   point (`14,2` = `14.2`). Whitespace does not count: `14,2 g/dl` matches
   `14,2g/dl`. A match must start and end at word boundaries, so `4` does
   not match inside `14`.
2. `documentDate` is searched in common German forms: `14.10.2026`,
   `14.10.26`, `2026-10-14`, `14/10/2026`, `14. Oktober 2026` (also
   Austrian `Jänner`, abbreviations like `Okt.`), with or without leading
   zeros.
3. `issuer` is confirmed if it appears as a whole (also across line breaks),
   or if each of its comma-separated parts appears somewhere in the document
   (`Labor Muster GmbH, Salzburg`: the name in the letterhead, the city in
   the address).
4. A fact's `value` must appear as a whole; its `unit` must follow in the
   same line within the next three words. "Line" means a visual row,
   decided from the positions: lines that overlap vertically and have a
   similar height form one row, read from left to right. Text layer and OCR
   often read a table column by column; this way `Hämoglobin`, `14,2` and
   `g/dl` in one table row still count as one line. A short value without a unit (at
   most 3 characters, e.g. `98`) only counts if the first word of the fact's
   `name` is in the same line, so that an unrelated number elsewhere does
   not confirm it. If a value appears several times, the match in the line
   of the name comes first.

`unchecked` means: the entry could not be checked. The upload still
succeeds; a failed check is never an HTTP error. `reason`:

| `reason` | When |
|---|---|
| `ocr_unavailable` | Switched off, or a needed program (`pdftotext`, `pdftoppm`, `tesseract`) is not installed on the server |
| `ocr_failed` | Reading returned an error (e.g. a damaged PDF) or took too long (30 s) |
| `unsupported_file_type` | The file type cannot be read by OCR. Every allowed type can be read, so this does not happen at the moment. |
| `too_many_pages` | PDF with more than 10 pages |
| `image_too_large` | Image with more than 50 megapixels; it is not decoded, to protect the server's memory |
| `no_text` | Neither text layer nor OCR found any text, e.g. a very blurry photo |
| `not_run` | Entry from before 2026-11-11, never checked |

`verification` describes the current values: after a correction
(`PATCH`, section 3.5) the backend checks again against the words stored at
upload. A field counts as corrected while `corrections` holds an item for
it (`documentDate`, `issuer`, or for a fact `facts.<id>.value` or
`facts.<id>.unit`). Its status then says whether the corrected value is in
the original: `corrected_confirmed` or `corrected_unconfirmed`. Setting a
field back to the recognized value removes the correction (section 2,
rule 3), and the field is `confirmed` or `unconfirmed` again. A field set to
`null` is no longer checked.

### Short form for the timeline

The list only returns what the timeline needs. `fullText`, `facts`, `file`,
`corrections` and `verification` are missing so the response stays small.

```json
{
  "id": "6727a1f3c2b4e5d6f7a8b9c0",
  "module": "health",
  "type": "lab_report",
  "documentDate": "2026-10-14",
  "importedAt": "2026-11-03T18:42:07Z",
  "issuer": "Labor Muster GmbH, Salzburg"
}
```

---

## 3. Endpoints

| Method | Path | Purpose |
|---|---|---|
| `POST` | `/api/v1/entries` | Upload a file or text and have it extracted |
| `GET` | `/api/v1/entries` | All entries of a module for the timeline |
| `GET` | `/api/v1/entries/{id}` | Get one entry |
| `GET` | `/api/v1/entries/{id}/file` | Get the original file |
| `PATCH` | `/api/v1/entries/{id}` | Correct recognized values |
| `DELETE` | `/api/v1/entries/{id}` | Delete an entry with its original file and chunks |
| `POST` | `/api/v1/questions` | Ask a question, answer with sources |
| `GET` | `/api/v1/status` | Health check: is the backend running, is the database reachable? |

### 3.1 Upload and extract a file or text

```http
POST /api/v1/entries
Content-Type: multipart/form-data; boundary=----lifelog
```

**Request**

| Form field | Note |
|---|---|
| `file` | Exactly one file. Either `file` or `text`, not both. |
| `text` | Document content as text (UTF-8), e.g. from a text field in the app. Either `file` or `text`, not both. |
| `module` | Optional, default `health`. See [modules](#modules). |

For `file`:

- Allowed types: `image/jpeg`, `image/png`, `image/heic`, `image/webp`,
  `application/pdf`. (`image/webp` since 2026-11-11.)
  The backend checks the type from the file content, not just the extension.
- Maximum size: 10 MB.

```http
------lifelog
Content-Disposition: form-data; name="file"; filename="befund_oktober.pdf"
Content-Type: application/pdf

<binary data>
------lifelog
Content-Disposition: form-data; name="module"

health
------lifelog--
```

For `text`:

- At most 100,000 characters (Unicode characters, not bytes). Empty or only
  whitespace counts as missing. If `file` comes together with such an empty
  `text`, the request counts as a file upload.
- Exactly one field `text`, valid UTF-8.
- The text is not changed, not even trimmed: whitespace at the edges and
  line endings such as `\r\n` stay as they arrive. `fullText` is exactly
  this text.
- There is no `UNREADABLE` for text: entered text always counts as
  readable. If no type fits, it becomes `other`.
- The backend stores the text as the original file: `fileName` `text.txt`,
  `mimeType` `text/plain; charset=utf-8`, `size` and `sha256` over the UTF-8
  bytes. Fetching (`GET …/file`), duplicate detection and deleting therefore
  work exactly as for a file.

```http
------lifelog
Content-Disposition: form-data; name="text"

Hausarztpraxis Beispiel
Überweisung vom 21.10.2026
...
------lifelog--
```

**Processing in the backend**

1. Check size and type, or check the text; check `module`.
2. Compute the SHA-256 hash over the file content (or the UTF-8 bytes of the
   text) and look up `file.sha256` in MongoDB. If the hash already exists,
   the backend stops with `409 DUPLICATE`. This check comes **before** the
   model call, so a duplicate costs no model call. A unique index on
   `file.sha256` also catches the case where the same file is uploaded twice
   at the same time. The check applies across all modules.
3. Send the file to the vision LLM. It returns `documentDate`, `issuer`,
   `type`, `fullText` and `facts` as JSON. For a text import the language
   model gets the text and returns the same fields except `fullText`;
   `fullText` is the entered text.
4. Check the JSON (required fields, date format, allowed `type` of the
   module) and set `module`. The backend **drops** a single fact without
   `name` or without `value` instead of rejecting the whole upload. It only
   logs which field was missing (e.g. `facts[2].value`), **never** a value.
   The remaining facts are numbered without gaps (`f1`, `f2`, …).
5. Check the recognized values (see [verification](#verification)). For a
   file, reading the original runs **in parallel** to step 3, so the upload
   does not take longer: a PDF page with a text layer is read directly, a
   page without one and a photo go through OCR. A photo (JPEG, PNG, HEIC,
   WebP) is first prepared: turned upright and scaled to a long side
   between 2500 and 4096 pixels. For a text import the
   values are compared against the entered text.
6. Split the full text into chunks and compute embeddings.
7. Store the original file, the chunks with embeddings, the words read
   from the original (with positions) and the entry.

If a step fails, the backend stores nothing. No half entry is left behind.
Exception: if reading the original fails, the upload still succeeds and
the entry gets `"status": "unchecked"` with a `reason`. The verification
never causes an error code.

The call is **synchronous** and can take up to about 60 seconds. The app
shows a loading indicator meanwhile and sets its HTTP timeout for this call
to at least 90 seconds.

**Response `201 Created`**

```http
Location: /api/v1/entries/6727a1f3c2b4e5d6f7a8b9c0
```

The body is the full entry as in section 2, with an empty `corrections`
list (`verification` shortened here, see section 2):

```json
{
  "id": "6727a1f3c2b4e5d6f7a8b9c0",
  "module": "health",
  "type": "lab_report",
  "documentDate": "2026-10-14",
  "importedAt": "2026-11-03T18:42:07Z",
  "issuer": "Labor Muster GmbH, Salzburg",
  "fullText": "Labor Muster GmbH\nMusterstraße 1, 5020 Salzburg\n\nLaborbefund vom 14.10.2026\nPatient: Max Mustermann, geb. 01.01.1990\n\nHämoglobin 14,2 g/dl (Ref. 13,5-17,5)\nLeukozyten 6,3 G/l (Ref. 4,0-10,0)\nCRP <0,5 mg/dl (Ref. <0,5)\nBefundart: Blutbild klein",
  "facts": [
    { "id": "f1", "name": "Hämoglobin", "value": "14,2", "unit": "g/dl" },
    { "id": "f2", "name": "Leukozyten", "value": "6,3", "unit": "G/l" },
    { "id": "f3", "name": "CRP", "value": "<0,5", "unit": "mg/dl" },
    { "id": "f4", "name": "Befundart", "value": "Blutbild klein" }
  ],
  "file": {
    "id": "6727a1f3c2b4e5d6f7a8b9c1",
    "fileName": "befund_oktober.pdf",
    "mimeType": "application/pdf",
    "size": 184320,
    "sha256": "3f8a1c0e9b7d2f4a6c5e8b1d0a9f7e3c2b4d6a8f0e1c3b5d7a9f2e4c6b8d0a1f"
  },
  "corrections": [],
  "verification": {
    "status": "checked",
    "reason": null,
    "source": "pdf_text",
    "engine": "pdftotext 26.11.0",
    "checkedAt": "2026-11-03T18:42:07Z",
    "confirmed": 5,
    "corrected": 0,
    "total": 6,
    "matchPercent": 83,
    "fields": [ "…" ]
  }
}
```

If the original could not be read, `verification` looks like this:

```json
"verification": {
  "status": "unchecked",
  "reason": "ocr_failed",
  "source": null,
  "engine": null,
  "checkedAt": null,
  "confirmed": 0,
  "corrected": 0,
  "total": 0,
  "matchPercent": null,
  "fields": []
}
```

**Errors**

| HTTP | Code | Note |
|---|---|---|
| 400 | `FILE_MISSING` | Neither `file` nor `text`, or the content is empty or only whitespace |
| 400 | `INVALID_REQUEST` | `file` and `text` together, more than one file or more than one `text` field, `text` not valid UTF-8 (`details.field`: `text`), or unknown `module` (`details.field`, `details.value`) |
| 409 | `DUPLICATE` | A file or text with the same SHA-256 hash exists. `details.entryId` names the existing entry. |
| 413 | `FILE_TOO_LARGE` | File over 10 MB or text over 100,000 characters (then `details.maxChars`: `100000`) |
| 415 | `UNSUPPORTED_FILE_TYPE` | Not JPEG, PNG, HEIC, WebP or PDF |
| 422 | `UNREADABLE` | No document recognizable, e.g. blurry photo (only for `file`) |
| 502 | `EXTRACTION_FAILED` | Response of the vision LLM does not match the expected JSON |
| 503 | `MODEL_UNAVAILABLE` | Vision LLM or embedding model unreachable |
| 500 | `INTERNAL_ERROR` | e.g. database unreachable |

Example of a duplicate. With `details.entryId` the app can jump straight to
the existing entry:

```json
{
  "error": {
    "code": "DUPLICATE",
    "message": "Diese Datei hast du schon hochgeladen.",
    "details": {
      "entryId": "6727a1f3c2b4e5d6f7a8b9c0"
    }
  }
}
```

### 3.2 List entries for the timeline

```http
GET /api/v1/entries?module=health&sort=documentDate&order=desc
```

**Request**

| Parameter | Values | Default |
|---|---|---|
| `module` | see [modules](#modules) | `health` |
| `sort` | `documentDate`, `importedAt` | `documentDate` |
| `order` | `desc`, `asc` | `desc` |

All parameters are optional. The list only contains entries of the chosen
module. Without parameters the newest document from `health` comes first.

**Sorting rules**

1. Sort by the chosen field in the chosen order.
2. Entries without `documentDate` (`null`) are always **at the end** with
   `sort=documentDate`, regardless of the order. Among themselves they are
   sorted by `importedAt` descending.
3. For equal `documentDate`, `importedAt` decides in the same order, then
   `id`. This makes the order always unique.

These rules are covered by tests in the backend.

**Response `200 OK`**

```json
{
  "entries": [
    {
      "id": "6727a1f3c2b4e5d6f7a8b9c0",
      "module": "health",
      "type": "lab_report",
      "documentDate": "2026-10-14",
      "importedAt": "2026-11-03T18:42:07Z",
      "issuer": "Labor Muster GmbH, Salzburg"
    },
    {
      "id": "6727a20bc2b4e5d6f7a8b9d4",
      "module": "health",
      "type": "doctor_letter",
      "documentDate": "2026-09-02",
      "importedAt": "2026-11-03T18:50:12Z",
      "issuer": "Dr. Erika Beispiel, Allgemeinmedizin"
    },
    {
      "id": "6727a2a9c2b4e5d6f7a8b9e7",
      "module": "health",
      "type": "other",
      "documentDate": null,
      "importedAt": "2026-11-03T19:01:44Z",
      "issuer": null
    }
  ],
  "count": 3
}
```

No entries: `200 OK` with `"entries": []` and `"count": 0`.

The prototype has no pagination. The list always contains all entries of
the module.

**Errors**

| HTTP | Code | Note |
|---|---|---|
| 400 | `INVALID_PARAMETER` | e.g. `sort=issuer` or an unknown `module` |
| 500 | `INTERNAL_ERROR` | |

```json
{
  "error": {
    "code": "INVALID_PARAMETER",
    "message": "Unbekannte Sortierung. Erlaubt sind documentDate und importedAt.",
    "details": { "parameter": "sort", "value": "issuer" }
  }
}
```

### 3.3 Get an entry

```http
GET /api/v1/entries/6727a1f3c2b4e5d6f7a8b9c0
```

**Request**

No body. `{id}` is the ID of the entry.

**Response `200 OK`**

The full entry including `fullText` and `corrections`, exactly like the
example in [section 2](#full-entry).

**Errors**

| HTTP | Code | Note |
|---|---|---|
| 400 | `INVALID_ID` | `{id}` is not a valid ObjectId |
| 404 | `ENTRY_NOT_FOUND` | |
| 500 | `INTERNAL_ERROR` | |

### 3.4 Get the original file

```http
GET /api/v1/entries/6727a1f3c2b4e5d6f7a8b9c0/file
```

The app uses this to show the original next to the recognized values. This
makes it possible to check whether the vision LLM read correctly.

**Response `200 OK`**

The file as binary data, not JSON.

```http
Content-Type: application/pdf
Content-Length: 184320
Content-Disposition: inline; filename=befund_oktober.pdf
X-Content-Type-Options: nosniff
Cache-Control: private, no-store
```

For a text import it is the text with
`Content-Type: text/plain; charset=utf-8` and `filename=text.txt`.

- `Content-Disposition` follows RFC 6266: quotes only if the name needs
  them; names with umlauts come as `filename*=utf-8''…` (percent-encoded).
  The app therefore reads the header with an RFC-compliant parser instead of
  expecting the example literally.
- `X-Content-Type-Options: nosniff`: the content is only treated as the
  given type, never guessed.
- `Cache-Control: private, no-store`: the original file must not end up in
  any cache (proxy, HTTP cache). If the app wants to show the document
  offline, it stores it deliberately itself.

**Errors** (as JSON as in section 1)

| HTTP | Code | Note |
|---|---|---|
| 400 | `INVALID_ID` | |
| 404 | `ENTRY_NOT_FOUND` | |
| 500 | `INTERNAL_ERROR` | |

### 3.5 Correct recognized values

```http
PATCH /api/v1/entries/6727a1f3c2b4e5d6f7a8b9c0
Content-Type: application/json
```

**Request**

The body contains only the fields that should change. Missing fields stay
unchanged.

| Field | Type | Note |
|---|---|---|
| `type` | String | One of the allowed types of the entry's module (section 2). `null` is not allowed. |
| `documentDate` | String or `null` | Format `YYYY-MM-DD`, must be a valid calendar date. `null` removes the date. |
| `issuer` | String or `null` | `null` removes the issuer. |
| `facts` | List | Per item `id` (required) plus `value` and/or `unit`. `"unit": null` removes the unit. `"value": null` is not allowed; a fact always has a value. |

`fullText` is not correctable in the prototype and leads to
`400 FIELD_NOT_CORRECTABLE` (extension see [open points](#4-open-points)).
The same applies to `id`, `module`, `importedAt`, `file`, `corrections` and
the `name` of a fact (`facts.<id>.name`). Adding or deleting facts is not
possible in the prototype either.

Further rules:

- **Whitespace** at the start and end is removed, as on upload. If nothing
  remains for `issuer` or `unit`, this counts as `null`. An empty `value` is
  an error.
- **Each fact `id`** may appear only once. An item without `value` and
  without `unit` is an error.
- **An empty body** (`{}`) is an error. A request whose values already
  apply, however, is valid: the response is `200` with the unchanged entry,
  and no correction is created.

```json
{
  "documentDate": "2026-10-15",
  "facts": [
    { "id": "f2", "value": "6,8" }
  ]
}
```

**Processing in the backend**

1. Check the request. If any part is invalid, the backend changes nothing.
2. Write the new values into the fields.
3. Maintain `corrections` according to the rules in section 2. A field
   whose value does not change creates no correction. All corrections of
   one request get the same `correctedAt`.

Corrections only change `type`, `documentDate`, `issuer` and `facts`. The
chunks and embeddings for search stay unchanged because they are derived
from `fullText`.

If at least one value changed, the backend checks the entry again
([verification](#verification)) against the words stored at upload (or
against `fullText` for a text import). This needs no new OCR run. A
corrected field gets `corrected_confirmed` if the corrected value appears
in the original, otherwise `corrected_unconfirmed`. An `unchecked` entry
stays `unchecked`.

**Response `200 OK`**

The updated full entry. Excerpt for the example above:

```json
{
  "id": "6727a1f3c2b4e5d6f7a8b9c0",
  "module": "health",
  "type": "lab_report",
  "documentDate": "2026-10-15",
  "importedAt": "2026-11-03T18:42:07Z",
  "issuer": "Labor Muster GmbH, Salzburg",
  "fullText": "Labor Muster GmbH\nMusterstraße 1, 5020 Salzburg\n\nLaborbefund vom 14.10.2026\nPatient: Max Mustermann, geb. 01.01.1990\n\nHämoglobin 14,2 g/dl (Ref. 13,5-17,5)\nLeukozyten 6,3 G/l (Ref. 4,0-10,0)\nCRP <0,5 mg/dl (Ref. <0,5)\nBefundart: Blutbild klein",
  "facts": [
    { "id": "f1", "name": "Hämoglobin", "value": "14,2", "unit": "g/dl" },
    { "id": "f2", "name": "Leukozyten", "value": "6,8", "unit": "G/l" },
    { "id": "f3", "name": "CRP", "value": "<0,5", "unit": "mg/dl" },
    { "id": "f4", "name": "Befundart", "value": "Blutbild klein" }
  ],
  "file": {
    "id": "6727a1f3c2b4e5d6f7a8b9c1",
    "fileName": "befund_oktober.pdf",
    "mimeType": "application/pdf",
    "size": 184320,
    "sha256": "3f8a1c0e9b7d2f4a6c5e8b1d0a9f7e3c2b4d6a8f0e1c3b5d7a9f2e4c6b8d0a1f"
  },
  "corrections": [
    {
      "field": "documentDate",
      "recognized": "2026-10-14",
      "corrected": "2026-10-15",
      "correctedAt": "2026-11-03T19:05:31Z"
    },
    {
      "field": "facts.f2.value",
      "recognized": "6,3",
      "corrected": "6,8",
      "correctedAt": "2026-11-03T19:05:31Z"
    }
  ]
}
```

**Errors**

| HTTP | Code | Note |
|---|---|---|
| 400 | `INVALID_ID` | |
| 400 | `INVALID_REQUEST` | e.g. `"documentDate": "15.10.2026"`, unknown field, unknown `type`, wrong JSON type, `null` for `type` or `value`, empty `value`, fact without `id`, duplicate fact `id`, fact without change, empty body. `details.field` names the field (e.g. `facts.f2.value`), `details.value` the rejected value, for fact errors without a field `details.factId` the ID. |
| 400 | `FIELD_NOT_CORRECTABLE` | e.g. `fullText`, `module`, `importedAt`, `file`, `facts.<id>.name`. `details.field` names the field. |
| 404 | `ENTRY_NOT_FOUND` | |
| 404 | `FACT_NOT_FOUND` | `details.factId` names the unknown ID |
| 500 | `INTERNAL_ERROR` | |

If a date has the right format but does not exist (`"2026-02-31"`), the
message is „Das Dokumentdatum ist kein gültiges Kalenderdatum.“ If a request
contains several errors, the backend reports only one of them.

```json
{
  "error": {
    "code": "INVALID_REQUEST",
    "message": "Das Dokumentdatum muss im Format JJJJ-MM-TT angegeben werden.",
    "details": { "field": "documentDate", "value": "15.10.2026" }
  }
}
```

### 3.6 Ask a question

```http
POST /api/v1/questions
Content-Type: application/json
```

**Request**

| Field | Type | Note |
|---|---|---|
| `question` | String | Required, at most 500 characters |
| `module` | String | Optional, default `health`. Only documents of this module are searched. |

```json
{
  "question": "Welchen Hämoglobinwert hatte ich im Oktober?",
  "module": "health"
}
```

**Processing in the backend**

1. Compute the embedding of the question.
2. Find the best-matching chunks of the module via Atlas Vector Search (at
   most 5).
3. First layer: chunks with a score below 0.82 are dropped (coarse filter,
   calibrated on 2026-11-04 with `backend/cmd/calibrate`, see the comment on
   `core.MinScore`), as are chunks whose entry no longer exists. If none
   remains, the backend answers with the fixed sentence (see below) without
   calling the language model.
4. Otherwise the language model gets the question and only these chunks. It
   answers only from these chunks and sets references like `[1]`. If the
   chunks do not answer the question, it answers with the fixed sentence.
5. Second layer: if the answer contains no valid reference, it counts as
   "nothing found" (fixed sentence, `found: false`). An answer without
   sources never reaches the app.

`sources` only contains the chunks the answer actually refers to. The
numbers `number` are not necessarily without gaps (e.g. only `1` and `3`);
they always match the references in the text.

The answer is based on `fullText`. Corrections to `facts` do not change it.

**Response `200 OK` with a hit**

```json
{
  "answer": "Im Laborbefund der Labor Muster GmbH vom 14.10.2026 steht: Hämoglobin 14,2 g/dl [1].",
  "found": true,
  "sources": [
    {
      "number": 1,
      "entryId": "6727a1f3c2b4e5d6f7a8b9c0",
      "module": "health",
      "type": "lab_report",
      "issuer": "Labor Muster GmbH, Salzburg",
      "documentDate": "2026-10-14",
      "excerpt": "Hämoglobin 14,2 g/dl (Ref. 13,5-17,5)"
    }
  ]
}
```

| Field | Type | Note |
|---|---|---|
| `answer` | String | Answer text (German). References `[n]` point to `sources[].number`. |
| `found` | Boolean | `false` if no matching chunk was found |
| `sources` | List | Chunks used. The app opens the entry via `entryId`. |
| `sources[].module` | String | Module of the entry. A `type` is only unique together with the module (e.g. `prescription`). |
| `sources[].excerpt` | String | The chunk from the full text the answer is based on |

`documentDate` and `issuer` in `sources` show the current values of the
entry, including corrected ones.

**Response `200 OK` without a hit**

No hit is not an error. The answer is always literally:

```json
{
  "answer": "Dazu finde ich nichts in deinen Dokumenten.",
  "found": false,
  "sources": []
}
```

**Errors**

| HTTP | Code | Note |
|---|---|---|
| 400 | `INVALID_REQUEST` | Body is not valid JSON, unknown field, or unknown `module` (`details.field`, `details.value`) |
| 400 | `QUESTION_MISSING` | Field missing, empty or only whitespace |
| 400 | `QUESTION_TOO_LONG` | Over 500 characters |
| 503 | `MODEL_UNAVAILABLE` | Embedding or language model unreachable, or the language model returns no usable answer (e.g. empty or blocked) |
| 500 | `INTERNAL_ERROR` | |

### 3.7 Delete an entry

```http
DELETE /api/v1/entries/6727a1f3c2b4e5d6f7a8b9c0
```

**Request**

No body. `{id}` is the ID of the entry.

**Processing in the backend**

1. Delete the entry's chunks with their embeddings.
2. Delete the stored words of the original (collection `ocr`).
3. Delete the original file.
4. Delete the entry.

The entry comes **last**. If a step fails, the backend responds with
`500 INTERNAL_ERROR` and the entry stays visible. Calling again cleans up the
rest. If the app drops the connection after deleting has started, it still
runs to completion.

Afterwards the same file can be uploaded again, because its hash is no
longer stored.

**Response `204 No Content`**

No body.

**Errors**

| HTTP | Code | Note |
|---|---|---|
| 400 | `INVALID_ID` | |
| 404 | `ENTRY_NOT_FOUND` | Also when deleting the same entry a second time |
| 500 | `INTERNAL_ERROR` | |

### 3.8 Status (health check)

```http
GET /api/v1/status
```

Checks whether the backend is running and MongoDB is reachable. Calls no
model and therefore costs nothing. The path is deliberately not `/health`,
so it is not confused with the module `health`.

**Response `200 OK`**

```json
{
  "status": "ok",
  "database": "ok"
}
```

**Response `503 Service Unavailable`** if MongoDB does not respond:

```json
{
  "status": "error",
  "database": "unreachable"
}
```

This endpoint deliberately does not use the error format from section 1, so
monitoring tools can read both cases the same way.

---

## 4. Open points

These points are deliberately left out of the prototype.

### Correcting the full text (later extension)

If the vision LLM reads the text wrongly, search works on the wrong text.
Later, `PATCH /api/v1/entries/{id}` should therefore also accept `fullText`:

```json
{
  "fullText": "Labor Muster GmbH\nMusterstraße 1, 5020 Salzburg\n\nLaborbefund vom 14.10.2026\nPatient: Max Mustermann, geb. 01.01.1990\n\nHämoglobin 14,2 g/dl (Ref. 13,5-17,5)\nLeukozyten 6,8 G/l (Ref. 4,0-10,0)\nCRP <0,5 mg/dl (Ref. <0,5)\nBefundart: Blutbild klein"
}
```

Planned behaviour:

1. The originally recognized full text is kept in `corrections`
   (`"field": "fullText"`), following the same rules as the other fields.
2. The backend splits the new text into chunks and recomputes the
   embeddings.
3. Only once all new embeddings exist does it replace the entry's old
   chunks. Search then no longer answers from the old text, and there is no
   state in which part is missing.
4. If the embedding model is unreachable, the backend responds with
   `503 MODEL_UNAVAILABLE` and the old state stays completely intact.
5. The call becomes slower than other corrections. The app shows a loading
   indicator.

### Further points

- **Login and multiple users:** the prototype has one user without login.
  Later every call needs a login, and every entry belongs to a user.
  Duplicate detection then applies per user.
- **Asynchronous upload:** instead of waiting up to 60 seconds,
  `POST /entries` could return `202 Accepted` immediately. The app then
  polls the status.
- **Pagination** of the timeline once there are many entries.
- **Adding and deleting facts**, in case the vision LLM misses a value.
- **Filter by `type`** in the timeline.
- **Further modules** (nutrition, recipes, sport): upload, list and question
  already accept `module` (see [modules](#modules)). A `type` only ever
  applies within its module. So `prescription` in `health` stays a medical
  prescription, and a later module for cooking recipes can have its own
  types. Still open: the IDs and types of further modules, and whether
  `module` becomes correctable.
- **Duplicates only for identical bytes:** a new photo of the same document
  has a different hash and is not detected as a duplicate. For a text
  import, one extra space is enough for a new hash.
- **Recalibrate the threshold:** re-measured on 2026-11-08 with the
  synthetic demo data (45 questions, 12 entries): relevant 0.830–0.902 (for
  two entries 0.850–0.866), near 0.823–0.861, unrelated 0.758–0.808.
  Separating thresholds lie in (0.808; 0.830]; 0.82 has a margin of 0.012
  up and down. Closest relevant question: „Wie oft pro Woche soll ich zur
  Physiotherapie?“ (0.830, the physio note scores higher). Closest unrelated
  question: „Hast du ein Rezept für Apfelkuchen?“ (0.808). Decision (Dogan,
  2026-11-08): 0.82 stays. Keep watching with more documents. Measure again
  after the switch to English prompts (2026-11): the read prompt determines
  the full text of uploaded files and therefore their chunks and vectors.
  Baseline before the switch (15 entries, same ranges, near up to 0.865):
  [docs/calibration/2026-11-11-baseline.md](calibration/2026-11-11-baseline.md).
  Measured again on 2026-11-11 after refilling with `demo/upload.sh`
  ([result](calibration/2026-11-11-after-english.md)): seed entries
  identical, 4 of 45 questions differ by at most 0.002, 0.82 stays. Since
  then „Wie hoch war mein Vitamin-D-Wert?“ counts as relevant (30 relevant,
  7 near).
- **Filter letterheads and addresses before embedding:** unrelated questions
  mentioning „Salzburg“ (weather, train ride) scored 0.79–0.80, close to the
  threshold, because the address is in the letterhead. If letterheads and
  addresses are not embedded, such scores drop. `fullText` and the excerpts
  in `sources` stay unchanged; only what goes into the embedding changes.
  Recalibrate afterwards.
- **Concurrent corrections:** `PATCH /entries/{id}` reads the entry,
  changes it and writes it back without a lock. If two corrections of the
  same entry arrive at the same time, the second wins and the first can be
  lost. With one user and one dialog per correction this is hardly
  reachable in the prototype. Later: a version counter in the entry, and
  `409` on a conflict.
- **Length limits for corrections:** `issuer`, `value` and `unit` have no
  maximum length of their own, only the limit for the whole body (64 KiB).
  Define later, e.g. matching what the app can display.

### References `[n]` → place in the original (proposal, 2026-11-11)

Answers should later link directly to the place in the original. Today a
reference `[n]` points to `sources[n]`, i.e. to a chunk of `fullText`. Since
upload stores the words of the original with their positions (text layer
or OCR, collection `ocr`), two
levels are possible, both without a new OCR run and without a model call:

1. **Chunk level:** at upload, align each chunk's text with the OCR words
   (same normalization as for the verification, longest runs of matching
   words) and store the rectangles with the chunk. `sources[].locations`
   then names the area the excerpt comes from (page and one rectangle per
   line). Existing entries can be filled in afterwards from the stored OCR
   words.
2. **Value level:** for each `[n]`, take the sentence of the answer that
   contains `[n]`. Facts of the source entry whose value (and unit) appear
   both in that sentence and in the excerpt contribute their
   `verification` locations as `sources[].highlights`. The app opens the
   file at `page` and draws exactly this rectangle.

Level 1 always gives something to show; level 2 is more precise but only
works for answers that quote a fact. For a text import both levels work the
same way with `start`/`end` in `fullText`.

### Further points on verification

- **Text layers made by OCR:** some scanners put an invisible OCR text
  layer into the PDF. It counts as `pdf_text`, although its quality is
  that of the scanner's OCR. Not distinguishable from a digital PDF in the
  prototype.
- **Pages with little text:** a scanned page with a small digital stamp
  (at least 20 characters) counts as a text layer, and its scanned content
  is not read. Raise the limit or OCR such pages additionally if this
  happens.
- **Rotated PDF pages** (`/Rotate`): positions are not tested with them
  yet.
- **Tesseract or PaddleOCR:** measured on 2026-11-11 on `demo/`
  ([docs/ocr/](ocr/README.md), `cmd/ocrcheck`): PDFs 100 % via the text
  layer, photos 75 % (58 % of the facts with unit, 78 % by value only).
  Most failures were units and decimal commas on low-resolution photos.
  Since then small images are scaled up before Tesseract (long side 2500
  pixels, at most twice the size): photos 93 %, every value read, 86 % of
  the facts with unit (the rest are misread units, which stay
  unconfirmed). The comparison stays strict (no tolerance for OCR
  confusions). Two real iPhone photos of lab report 01 (`demo/phone/`,
  both taken of a screen, not of paper): the JPEG reached 100 %, the HEIC
  (at an angle, with moiré) 57 %, with every value but two units read
  wrongly; Apple's own decoder gives the same text, so it is the photo.
  Decision: Tesseract is enough for now; PaddleOCR only if further real
  photos (of paper) do clearly worse. The first photo of paper
  (2026-11-12): every value read, but `mg/dl` read as `mgj/dl` twice, the
  same confusion as on the synthetic photos.
- **Values with a misread unit:** a fact whose value is found but whose
  unit is misread (`mgj/dl`) is `unconfirmed`. Two proposals, neither
  implemented: a status "value confirmed, unit uncertain", or tolerating
  this one known confusion when comparing units. Details:
  [docs/ocr/README.md](ocr/README.md), "Open point: values with a misread
  unit".
- **Old entries:** entries stored before 2026-11-11 are `not_run`.
  `go run ./cmd/ocrcheck -write` fills in their verification (without
  `-write` it only shows what it would store). After a change to OCR or
  the rules, `-recheck -write` checks every entry again.
  `internal/ocr` is an interface like `internal/llm`; another engine is a
  new subpackage.
- **Badge in the timeline:** the short form could carry `matchPercent`.
  Only if the app needs it.
- **Remove location and other metadata from uploaded photos:** a phone
  photo carries EXIF data with the GPS position (latitude, longitude,
  altitude, direction), the camera model and the time it was taken. Gemini
  gets the file as it is, and `GET /entries/{id}/file` returns it as it
  is, so real users would store where they took the photo. Proposal: the
  backend removes these segments before it stores the file (JPEG: APP1
  EXIF/XMP, without decoding the image; HEIC: the Exif item; PNG and WebP:
  `eXIf`/`EXIF` chunks), keeps only the orientation (or applies it), and
  computes `sha256` from the cleaned file. To be decided: whether the app
  should already remove it before the upload (image_picker on iOS keeps
  EXIF by default).
- **Orientation in PNG and WebP:** only JPEG (EXIF) and HEIC (rotation in
  the file) are turned upright. PNG and WebP can carry EXIF too, but
  phones do not write it; these images are read as they are stored.

---

## 5. Changes

### 2026-11-11: WebP upload, HEIC and photo orientation in the verification

1. **Upload** ([3.1](#31-upload-and-extract-a-file-or-text)): `image/webp`
   is allowed. The app can allow WebP in its file picker and in its own
   type check; the 415 message names WebP.
2. **HEIC is checked** like every other photo, no longer `unchecked` with
   `unsupported_file_type`.
3. **Orientation** ([verification](#verification)): photos are turned
   upright before OCR (EXIF in JPEG, rotation in HEIC). The locations
   refer to the upright photo, as shown by the app.
4. **New `reason`:** `image_too_large` (more than 50 megapixels).

### 2026-11-11: Verification by text layer and OCR

New field `verification` in the full entry (section 2,
[verification](#verification)). Additive: no existing field changes, no new
error code.

1. **Upload** ([3.1](#31-upload-and-extract-a-file-or-text)): reading the
   original runs in parallel to the vision LLM and checks date, issuer and
   facts: PDF pages with a text layer directly (`pdf_text`), scans and
   photos with OCR (`ocr`). A failure never fails the upload; the entry is
   then `unchecked` with a `reason`. Each field names its `source`.
2. **Text import:** the same check against the entered text
   (`source: text`, locations as `start`/`end`).
3. **Correcting** ([3.5](#35-correct-recognized-values)): `verification` is
   recomputed after a change against the stored words; corrected fields get
   `corrected_confirmed` or `corrected_unconfirmed`.
4. **Delete** ([3.7](#37-delete-an-entry)): also deletes the stored OCR
   words.
5. **Old entries:** `unchecked` with `reason: not_run`.

### 2026-11-09: English names

All names in the API are now English (Prof. Pree's requirement). The full
mapping from the old German names is in [GLOSSARY.md](GLOSSARY.md). The
behaviour is unchanged. App and backend switch together; the base path stays
`/api/v1`.

1. **Paths:** `/eintraege` → `/entries`, `/eintraege/{id}/datei` →
   `/entries/{id}/file`, `/fragen` → `/questions`. `/status` is unchanged.
2. **Fields:** e.g. `modul` → `module`, `typ` → `type`, `dokumentdatum` →
   `documentDate`, `importdatum` → `importedAt`, `quelle` → `issuer`,
   `volltext` → `fullText`, `infos` → `facts` (`wert` → `value`, `einheit` →
   `unit`), `datei` → `file`, `korrekturen` → `corrections`. Fact IDs are
   `f1`, `f2`, … instead of `i1`, `i2`, ….
3. **Upload form fields:** `datei` → `file`, `modul` → `module`; `text` is
   unchanged.
4. **List query:** `sortierung` → `sort` (`documentDate`, `importedAt`),
   `richtung` → `order` (`desc`, `asc`); response `eintraege` → `entries`,
   `anzahl` → `count`.
5. **Questions:** `frage` → `question`; response `antwort` → `answer`,
   `gefunden` → `found`, `quellen` → `sources` (`nr` → `number`, `eintragId`
   → `entryId`, `auszug` → `excerpt`).
6. **Error format:** `fehler` → `error`, `nachricht` → `message` (text stays
   German); `details` keys `field`, `value`, `parameter`, `factId`,
   `entryId`, `maxChars`. All error codes are English, see
   [all error codes](#all-error-codes).
7. **Codes:** module `gesundheit` → `health`; types `arztbrief` →
   `doctor_letter`, `laborbefund` → `lab_report`, `rezept` → `prescription`,
   `sonstiges` → `other`.
8. **Status:** `"status": "error"` instead of `"fehler"`, `"database"`
   instead of `"datenbank"`, `"unreachable"` instead of
   `"nicht erreichbar"`.

### Since 2026-11-03

Overview for the backend implementation. All changes use existing error
codes. (Written with the old German names; listed here with the current
names.)

1. **`module`** ([modules](#modules)): optional for `POST /entries` (form
   field), `GET /entries` (query) and `POST /questions` (JSON), default
   `health`. List and search apply only to that module. `sources[]` also has
   `module`.
2. **Text import** ([3.1](#31-upload-and-extract-a-file-or-text)): form field
   `text` instead of `file` on the same endpoint. The text is stored as
   `text.txt` (`text/plain; charset=utf-8`); `file` therefore stays a
   required field. At most 100,000 characters.
3. **Delete** ([3.7](#37-delete-an-entry)): `DELETE /entries/{id}` with
   `204`. Deletes chunks, file and finally the entry. Runs to completion even
   if the app drops the connection.
4. **Rule** ([important rules](#important-rules)): lists are always `[]`,
   never `null`.
5. **Status** ([3.8](#38-status-health-check)): `GET /status` as a health
   check, `200` or `503`.
6. **Incomplete facts** ([3.1](#31-upload-and-extract-a-file-or-text),
   step 4): facts without `name` or `value` are dropped on upload, not
   rejected. The log only names the field, never the value.
7. **Original file** ([3.4](#34-get-the-original-file)): additionally
   `X-Content-Type-Options: nosniff` and `Cache-Control: private, no-store`;
   `Content-Disposition` per RFC 6266 (quotes only when needed, umlauts as
   `filename*`).
8. **Questions** ([3.6](#36-ask-a-question)): threshold 0.82 as the first
   filter; answers without a valid reference become the fixed sentence;
   `sources` only with the cited chunks; 503 also for an unusable model
   answer.
9. **Correcting** ([3.5](#35-correct-recognized-values)): whitespace is
   removed, empty `issuer`/`unit` count as `null`, an empty `value` is an
   error; `null` for `type` and `value` is an error; duplicate fact IDs and
   facts without a change are errors; `facts.<id>.name` is not correctable;
   `details.field` or `details.factId` on errors; a request without an
   actual change returns `200` and the unchanged entry.
10. **Text import** ([3.1](#31-upload-and-extract-a-file-or-text)):
    characters count as Unicode characters; the text is not trimmed; no
    `UNREADABLE` for text; `INVALID_REQUEST` also for several `text` fields
    or invalid UTF-8; `413` with `details.maxChars`.
