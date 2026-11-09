# Lifelog REST API (prototype)

Status: 2026-11-09 · Version `v1` · Draft
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
| 415 | `UNSUPPORTED_FILE_TYPE` | Not JPEG, PNG, HEIC or PDF |
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
  ]
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

### Short form for the timeline

The list only returns what the timeline needs. `fullText`, `facts`, `file`
and `corrections` are missing so the response stays small.

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

- Allowed types: `image/jpeg`, `image/png`, `image/heic`, `application/pdf`.
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
5. Split the full text into chunks and compute embeddings.
6. Store the entry, the chunks with embeddings and the original file.

If a step fails, the backend stores nothing. No half entry is left behind.

The call is **synchronous** and can take up to about 60 seconds. The app
shows a loading indicator meanwhile and sets its HTTP timeout for this call
to at least 90 seconds.

**Response `201 Created`**

```http
Location: /api/v1/entries/6727a1f3c2b4e5d6f7a8b9c0
```

The body is the full entry as in section 2, with an empty `corrections`
list:

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
  "corrections": []
}
```

**Errors**

| HTTP | Code | Note |
|---|---|---|
| 400 | `FILE_MISSING` | Neither `file` nor `text`, or the content is empty or only whitespace |
| 400 | `INVALID_REQUEST` | `file` and `text` together, more than one file or more than one `text` field, `text` not valid UTF-8 (`details.field`: `text`), or unknown `module` (`details.field`, `details.value`) |
| 409 | `DUPLICATE` | A file or text with the same SHA-256 hash exists. `details.entryId` names the existing entry. |
| 413 | `FILE_TOO_LARGE` | File over 10 MB or text over 100,000 characters (then `details.maxChars`: `100000`) |
| 415 | `UNSUPPORTED_FILE_TYPE` | Not JPEG, PNG, HEIC or PDF |
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
2. Delete the original file.
3. Delete the entry.

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

---

## 5. Changes

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
