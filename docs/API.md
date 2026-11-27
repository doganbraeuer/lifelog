# Lifelog REST API (prototype)

Status: 2026-11-27 · Version `v1` · Draft
(Changes: see [section 5](#5-changes))

**Routines, since 2026-11-23:** stage 1 is implemented in the backend:
routines, occurrences, logs, pauses and the user's time zone
([3.12](#312-get-the-logged-in-user), [3.13](#313-change-the-display-name),
[3.16](#316-routines-overview-and-data-model)–3.27). The app follows.
Stage 2, the routine suggestions on entries
([section 2](#routine-suggestions-proposal-stage-2), 3.28, 3.29), is still a
proposal; every part of it is marked "(proposal, stage 2)".

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
  and the responses without a body (deleting, logout, password change).
- Field names are English and follow [GLOSSARY.md](GLOSSARY.md) and the data
  model in `CLAUDE.md`.
- Entry IDs are MongoDB ObjectIds as a 24-character hex string,
  e.g. `"6727a1f3c2b4e5d6f7a8b9c0"`.
- Every request except `GET /status` and `POST /auth/login` needs a token,
  see [login and tokens](#login-and-tokens).

### Login and tokens

Since 2026-11-18. Every user has an account and logs in with username and
password ([3.10](#310-log-in)). Accounts are created by an administrator
with `backend/cmd/users`; there is no registration through the API. A new
account has a start password and `mustChangePassword: true`.

- All endpoints except `GET /status` and `POST /auth/login` need the header
  `Authorization: Bearer <token>`. The scheme `Bearer` is case-insensitive.
- Without a valid token the backend answers `401 UNAUTHORIZED`: the token
  is missing, unknown or expired, or the account is disabled. The response
  has the header `WWW-Authenticate: Bearer`. This also applies to unknown
  paths: without a token, everything except the two public endpoints is
  closed.
- While `mustChangePassword` is `true`, only `GET /me`, `POST /me/password`
  and `POST /auth/logout` work. Every other endpoint answers
  `403 PASSWORD_CHANGE_REQUIRED`.
- Order of the checks: first the token (`401`), then the password change
  (`403`), then the checks of the endpoint itself. Every endpoint except
  the two public ones can therefore also answer `401` and `403`; the error
  tables of the endpoints do not repeat this.
- The token is an opaque string of 43 characters; the app does not
  interpret it. It is valid for 30 days from the login (`expiresAt`) and is
  not extended by using it. After that, or after any `401`, the app asks
  for a new login.
- The client gets the token exactly once, in the login response. The
  backend stores only its SHA-256 hash, never the token itself. The app
  keeps the token like a password. On the server on the internet, requests
  go only over HTTPS.
- Every login creates its own token (one per device). Logout ends only the
  token of this device ([3.11](#311-log-out)); a password change ends all
  other tokens of the account ([3.14](#314-change-the-password)).
- The backend never returns a password or a password hash, and never logs
  a password, a token or the `Authorization` header.

### Per-user data

Since 2026-11-18 every entry belongs to the user who uploaded it, together
with its original file, its chunks and the words read from the original.

- Each user only sees their own entries: timeline ([3.2](#32-list-entries-for-the-timeline)),
  entry ([3.3](#33-get-an-entry)), original file ([3.4](#34-get-the-original-file)),
  correction ([3.5](#35-correct-recognized-values)) and delete
  ([3.7](#37-delete-an-entry)). An entry of another user answers
  `404 ENTRY_NOT_FOUND`, exactly like an entry that does not exist, so the
  response does not reveal that it exists.
- Questions ([3.6](#36-ask-a-question)) only search the user's own
  documents: the vector search filters by user and module, and the answer
  only cites the user's own entries.
- `409 DUPLICATE` only applies within the user's own account. The unique
  index is `(userId, file.sha256)`, so a duplicate never reveals that another
  user has the same file; another user uploading the same file gets `201`.
- Admins (role `admin`) have no additional rights through the API, also no
  access to entries of other users.
- Since 2026-11-23 routines and their logs are separated the same way
  ([3.16](#316-routines-overview-and-data-model)): a routine or log of
  another user answers `404 ROUTINE_NOT_FOUND` or
  `404 ROUTINE_LOG_NOT_FOUND`, exactly like one that does not exist.
  (Proposal, stage 2) A routine suggestion on another user's entry answers
  `404 ENTRY_NOT_FOUND`; the model only ever gets the user's own routines.

### Daily limits

Since 2026-11-18. Each user may make a limited number of costly requests
per calendar day (time zone Europe/Vienna, the day starts at 00:00 local
time):

| Limit (`details.limit`) | Requests | Default | Environment variable |
|---|---|---|---|
| `uploads` | `POST /entries` (file and text) | 30 | `LIMIT_UPLOADS_PER_DAY` |
| `questions` | `POST /questions` | 60 | `LIMIT_QUESTIONS_PER_DAY` |
| `transcriptions` | `POST /transcriptions` | 30 | `LIMIT_TRANSCRIPTIONS_PER_DAY` |

- A request counts once it is accepted: after its input has been checked
  and before the backend calls the vision LLM, the language or embedding
  model, or the speech recognition. Whether it succeeds afterwards does not
  matter; the limit protects the model budget. A request refused by its
  input checks (e.g. `400`, `413`, `415`) does not count, nor does an upload
  refused as `409 DUPLICATE`, because a duplicate costs no model call.
- When the limit is reached, the backend answers `429 LIMIT_REACHED` and
  calls no model:

  ```json
  {
    "error": {
      "code": "LIMIT_REACHED",
      "message": "Du hast dein Tageslimit erreicht. Morgen geht es weiter.",
      "details": { "limit": "questions" }
    }
  }
  ```

- The counters are stored in the database, so a restart of the backend
  does not reset them. Admins have the same limits.

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
| `documentDate` | `YYYY-MM-DD` | `"2026-10-14"` | Plain date without time and without time zone. `null` if the vision LLM recognizes no date in a file. A text import always has one: without a date in the text, the day of the import in Europe/Vienna, marked by `documentDateSource: "import"` (since 2026-11-23, see 3.1). |
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
| 400 | `INVALID_REQUEST` | JSON not readable, required field missing, wrong format, unknown field, unknown `module`, upload with both `file` and `text`, `displayName` not 1–50 characters; since 2026-11-23 also an unknown `timeZone` (3.13) and a broken rule for routines or logs (3.16–3.27) |
| 400 | `INVALID_PARAMETER` | Unknown value for a query parameter |
| 400 | `INVALID_ID` | ID is not a valid ObjectId |
| 400 | `FILE_MISSING` | Upload without `file` and without `text`, or the content is empty; transcription without `audio` |
| 400 | `FIELD_NOT_CORRECTABLE` | Correction of a field that is not (yet) correctable, e.g. `fullText` |
| 400 | `QUESTION_MISSING` | Question missing or only whitespace |
| 400 | `QUESTION_TOO_LONG` | Question longer than 500 characters |
| 400 | `WRONG_PASSWORD` | Password change: `currentPassword` is wrong (since 2026-11-18). `message`: „Das bisherige Passwort stimmt nicht.“ |
| 400 | `WEAK_PASSWORD` | Password change: the new password has fewer than 10 characters, more than 72 bytes, or equals the current one (since 2026-11-18). `message` e.g.: „Das neue Passwort muss mindestens 10 Zeichen lang sein.“ |
| 401 | `INVALID_CREDENTIALS` | Login: unknown username, wrong password or disabled account, always with the same message (since 2026-11-18). `message`: „Benutzername oder Passwort stimmt nicht.“ |
| 401 | `UNAUTHORIZED` | Token missing, unknown or expired, or account disabled (since 2026-11-18). `message`: „Bitte melde dich an.“ |
| 403 | `PASSWORD_CHANGE_REQUIRED` | `mustChangePassword` is `true` and the endpoint is not `GET /me`, `POST /me/password` or `POST /auth/logout` (since 2026-11-18). `message`: „Bitte ändere zuerst dein Startpasswort.“ |
| 404 | `ENTRY_NOT_FOUND` | No entry with this ID, or it belongs to another user (see [per-user data](#per-user-data)) |
| 404 | `FACT_NOT_FOUND` | Correction refers to an unknown fact ID |
| 404 | `ROUTINE_NOT_FOUND` | (since 2026-11-23) No routine with this ID, or it belongs to another user. `message`: „Diese Routine gibt es nicht.“ |
| 404 | `ROUTINE_LOG_NOT_FOUND` | (since 2026-11-23) No log with this ID in this routine, or it belongs to another user. `message`: „Diesen Verlaufseintrag gibt es nicht.“ |
| 404 | `ROUTINE_SUGGESTION_NOT_FOUND` | (proposal, stage 2) No suggestion with this ID on the entry, e.g. already confirmed or rejected; `details.suggestionId`. `message`: „Diesen Vorschlag gibt es nicht mehr.“ |
| 409 | `DUPLICATE` | Exactly this file (or exactly this text) was already uploaded by the same user |
| 409 | `ROUTINE_LOG_EXISTS` | (since 2026-11-23) The occurrence already has a log; `details.logId`. `message`: „Dieser Termin ist schon eingetragen.“ |
| 409 | `ROUTINE_NAME_TAKEN` | (since 2026-11-23) The user already has a routine with this name in the module; `details.routineId`. `message`: „Eine Routine mit diesem Namen gibt es schon.“ |
| 409 | `ROUTINE_LIMIT_REACHED` | (since 2026-11-23) Too many routines, pauses or logs (3.16); `details.limit` (`routines`, `pauses`, `logs`) and `details.max`. `message` e.g.: „Du hast schon 50 Routinen. Lösche eine, bevor du eine neue anlegst.“ |
| 413 | `FILE_TOO_LARGE` | File or recording larger than 10 MB, or text longer than 100,000 characters |
| 413 | `AUDIO_TOO_LONG` | Recording longer than 5 minutes (since 2026-11-12) |
| 415 | `UNSUPPORTED_FILE_TYPE` | Upload: not JPEG, PNG, HEIC, WebP or PDF. Transcription: not one of the audio formats in 3.9 |
| 422 | `UNREADABLE` | The vision LLM recognizes no document in the file, or a recording cannot be decoded |
| 422 | `NO_SPEECH` | No speech in the recording: silence, noise, or shorter than 0.5 s (since 2026-11-12) |
| 429 | `TOO_MANY_ATTEMPTS` | Too many failed attempts at login or password change; header `Retry-After` and `details.retryAfterSeconds` give the wait in seconds (since 2026-11-18). `message`: „Zu viele Fehlversuche. Bitte versuche es in einigen Minuten erneut.“ |
| 429 | `LIMIT_REACHED` | The user has reached a daily limit ([daily limits](#daily-limits)); `details.limit` is `uploads`, `questions` or `transcriptions` (since 2026-11-18). `message`: „Du hast dein Tageslimit erreicht. Morgen geht es weiter.“ |
| 500 | `INTERNAL_ERROR` | Unexpected error in the backend, e.g. database unreachable |
| 502 | `EXTRACTION_FAILED` | The vision LLM returns a response that does not match the expected JSON |
| 503 | `MODEL_UNAVAILABLE` | Language, embedding or speech recognition model does not respond or reports a limit; for questions also while the search index is being rebuilt (since 2026-11-18, see 3.6) |

### Important rules

- The app never calls a model directly. All calls to the vision LLM, the
  language model, the embedding model and the speech recognition go through
  the backend.
- Recordings are never stored: `POST /transcriptions` returns only the
  text (3.9).
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
  "documentDateSource": "document",
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
| `documentDateSource` | String | Since 2026-11-23. Where `documentDate` comes from: `document` (from the document or the entered text, or there is none) or `import` (a text import named no date, so the backend set the day of the import in Europe/Vienna, see 3.1). Always present, also in the short form. Entries stored before have `document`. The app shows an `import` date with the time of the import, e.g. „23.11.2026 (Importdatum)“. |
| `importedAt` | String | Time of the upload |
| `issuer` | String or `null` | Issuing body. `null` if not recognized |
| `fullText` | String | Text of the document as read by the vision LLM. For a text import, the entered text, unchanged. |
| `facts` | List | Extracted facts, see below |
| `file` | Object | Reference to the original file |
| `corrections` | List | Corrected fields with the originally recognized value |
| `verification` | Object | Check of the recognized values by a second reader (PDF text layer or OCR), see [verification](#verification). Always present. |
| `routineSuggestions` | List | (proposal, stage 2, 2026-11-23) Intakes or activities of the user's routines that the model found in the document, not yet confirmed, see [routine suggestions](#routine-suggestions-proposal-stage-2). Always present, `[]` if there are none. |

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
- Not `documentDate` if `documentDateSource` is `import` (since
  2026-11-23): the backend set the day of the import because the text named
  no date, so it cannot be found in the text. Like a `null` date, it has no
  item in `fields` and does not count in `total`, so the app never shows it
  as "not found".
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
   zeros. For a text import also as the word for it, seen from the day of
   the import in Europe/Vienna (since 2026-11-23): „heute“ for the import
   day, „gestern“ one day before, „vorgestern“ two days before, and the
   weekday („Montag“) for one to seven days before. A note says „gestern“,
   and the language model turns it into the date (3.1); without this rule
   that date would never be found. A file is never searched for these
   words: a document on paper has its own date.
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

### Routine suggestions (proposal, stage 2)

> **Proposal of 2026-11-23 (Dylan), to be agreed with Dogan. Not
> implemented. Stage 2:** comes after routines
> ([3.16](#316-routines-overview-and-data-model), stage 1, implemented
> since 2026-11-23).
> It changes the read prompt for every document (run `cmd/calibrate`
> again) and sends routine names to Gemini (update the consent text
> first).

If the user has active routines in the module, the model that reads the
upload also looks for intakes or activities of these routines, **in the same
call** (vision LLM for a file, language model for a text import or a voice
note). There is no extra model call, and the upload counts once against the
daily limit `uploads` as before. What it finds is stored on the entry as
suggestions. A suggestion counts for nothing until the user confirms it in
the review ([3.28](#328-confirm-a-routine-suggestion-proposal-stage-2)); only then
does the backend create a log with `source: entry`. Rejecting
([3.29](#329-reject-a-routine-suggestion-proposal-stage-2)) removes it.

```json
{
  "routineSuggestions": [
    {
      "id": "s1",
      "routineId": "6742bf10a1b2c3d4e5f6a7b8",
      "amount": 2,
      "takenAt": "2026-11-23T06:30:00Z",
      "timeStated": true,
      "quote": "heute um 7:30 zwei Vitamin-D-Tabletten genommen"
    }
  ]
}
```

| Field | Type | Note |
|---|---|---|
| `id` | String | Stable ID within the entry (`s1`, `s2`, …), assigned on upload without gaps. Confirming or rejecting removes the suggestion; the IDs of the others stay. |
| `routineId` | String | ID of one of the user's routines |
| `amount` | Number or `null` | Amount in the routine's `unit` as stated in the text. `null` if the text names none or the routine has no `amount`; confirming then uses the routine's `amount`. |
| `takenAt` | String | When it was taken or done (RFC 3339, UTC), see "How the backend builds `takenAt`" |
| `timeStated` | Boolean | `true` if the text names the time of day. `false` if the backend completed it; the app then asks for the time before confirming. |
| `quote` | String | The passage of `fullText` the suggestion rests on, verbatim, at most 200 characters |

**What the model gets and returns**

The read and text prompts get, for each active routine of the module
(status `active`, at most 50): a short key (`r1`, `r2`, …), its `name`,
`amount` and `unit`. They also get the current date, weekday and time in
the user's time zone (`timeZone`, [3.12](#312-get-the-logged-in-user)).
IDs, schedules and logs are not sent. The short keys spare the model from
copying 24-digit IDs. If the user has no active routine, the prompt part is
left out and nothing changes.

The model answers per suggestion with the key, `amount`, `takenDate`
(`YYYY-MM-DD` or `null`), `takenTime` (`HH:MM` or `null`) and `quote`.

**Rules for the model**

1. Only a concrete intake or activity that has happened: „genommen“,
   „eingenommen“, „gemessen“, „getrunken“, „war eisbaden“. Not prescribed
   or recommended („verordnet“, „verschrieben“, a prescription, „empfohlen“,
   „soll … nehmen“), not planned or in the future („ab morgen“, „werde“),
   not a general statement („nimmt seit drei Wochen“, „regelmäßig“). Wish
   of Prof. Pree (2026-11-16): taken is not prescribed.
2. One suggestion per completed occurrence in the text: „morgens und abends
   gemessen“ gives two.
3. **The time must be determinable.** A suggestion only arises if
   1. the text names the date (with or without time), or
   2. the text uses a relative expression („gerade“, „heute früh“, „gestern
      Abend“). In a text import or a voice note it refers to the current
      date and time; in a photo or PDF to the document's date
      (`documentDate`), or
   3. the entry is a text import or a voice note: it is written now, so
      without any statement the upload moment applies.

   A photo or PDF without a date for the intake and without `documentDate`
   gives no suggestion.
4. Only a routine the text clearly means. When in doubt, no suggestion.
5. `amount` only if the text names it in the routine's unit or an
   equivalent one („zwei Tabletten“ for `Stück`), otherwise `null`.

Examples (synthetic). The routines are „Vitamin D“ (2 Stück, daily 08:00),
„Blutdruck messen“ (daily 08:00 and 20:00), „Eisbaden“ (weekly, no amount)
and „Wasser“ (250 ml, hourly 08:00–20:00), all started on 2026-11-18. The
upload is on 2026-11-23 at 18:42 in Europe/Vienna.

| Input | Suggestion |
|---|---|
| Voice note: „Heute um 7:30 zwei Vitamin-D-Tabletten genommen.“ | Vitamin D, `amount` 2, `takenAt` 2026-11-23 07:30, `timeStated: true` |
| Text import: „Vitamin D genommen.“ | Vitamin D, `amount` `null`, `takenAt` = upload moment (18:42), `timeStated: false` (rule 3.3) |
| Voice note: „Habe gerade ein Glas Wasser getrunken.“ | Wasser, `amount` `null` („ein Glas“ is no amount in ml), `takenAt` 18:42, `timeStated: true` („gerade“) |
| Voice note: „Gestern Abend Blutdruck gemessen, 128/82.“ | Blutdruck messen, `takenAt` 2026-11-22 12:00, `timeStated: false` („Abend“ is no time of day; the app asks) |
| Text import: „Eisbaden heute, drei Minuten.“ | Eisbaden, `amount` `null` (the routine has no amount), `takenAt` = upload moment, `timeStated: false` |
| Photo of a lab report of 14.10.2026: „Vitamin D3 1000 IE täglich verordnet“ | none: prescribed, not taken |
| PDF prescription „Vitamin D3 …, 1-0-0“ | none: a prescription |
| Text import: „Ab morgen zweimal täglich Blutdruck messen.“ | none: a plan |
| Doctor's letter: „Die Patientin nimmt seit drei Wochen Vitamin D.“ | none: a general statement |
| Old doctor's letter without a date: „Blutdruck heute gemessen: 135/85.“ | none: photo or PDF without a determinable date |
| Belatedly uploaded doctor's letter of 12.06.2025: „Blutdruck in der Praxis gemessen: 130/80.“ | none: even if the model suggests one, the backend drops it (before `startDate`, and more than 7 days before the upload) |

**How the backend builds `takenAt`**

In the user's time zone, then converted to UTC. Times that do not exist or
exist twice are handled as for occurrences
([3.16](#time-zone-and-points-in-time)).

1. Date: `takenDate`. If it is `null`: for a text import the day of the
   upload; for a photo or PDF the suggestion is dropped (rule 3).
2. With `takenTime`: that date and time, `timeStated: true`.
3. Without `takenTime`: if the date is the day of the upload, `takenAt` =
   `importedAt`; otherwise 12:00 on that date. `timeStated: false` in both
   cases.

**Checks in the backend**

The backend drops a suggestion, and never fails the upload because of one,
if:

- the key is not one of the routines sent;
- `amount` is not a number greater than 0 (a missing amount becomes
  `null`), or date or time have the wrong format;
- `takenAt` is later than `importedAt`;
- `takenAt` is more than 7 days before `importedAt` (value to be agreed,
  see [open questions](#open-questions-on-routines-2026-11-23));
- the day of `takenAt` is before the routine's `startDate` or after its
  `endDate`;
- `quote` is empty, longer than 200 characters, or not in `fullText` (same
  normalization as the verification, whitespace ignored). This ties every
  suggestion to the document;
- it repeats another suggestion (same routine, same `takenAt`).

At most 10 suggestions per entry, the first 10 in the model's order. As for
facts, the log only names the field and the reason, never a value, a name
or a quote.

**Lifecycle**

- Pending suggestions stay on the entry until they are confirmed or
  rejected, also if the review is left with „Zurück“; the entry shows them
  again.
- Correcting the entry ([3.5](#35-correct-recognized-values)) does not
  change them.
- Deleting the entry ([3.7](#37-delete-an-entry)) deletes its pending
  suggestions with it, and **also the logs confirmed from it**
  (`source: entry` with this `entryId`). Who discards an upload in the
  review („Verwerfen“) expects nothing of it to stay.
- Deleting a routine ([3.23](#323-delete-a-routine)) removes its
  pending suggestions from all entries of the user.
- Entries stored before have `"routineSuggestions": []`.

### Short form for the timeline

The list only returns what the timeline needs. `fullText`, `facts`, `file`,
`corrections` and `verification` are missing so the response stays small.

```json
{
  "id": "6727a1f3c2b4e5d6f7a8b9c0",
  "module": "health",
  "type": "lab_report",
  "documentDate": "2026-10-14",
  "documentDateSource": "document",
  "importedAt": "2026-11-03T18:42:07Z",
  "issuer": "Labor Muster GmbH, Salzburg"
}
```

`documentDateSource` is in the short form too (since 2026-11-23), so the
timeline can show a date from the import day with the time of the import
(„23.11.2026, 10:46“). A spoken note without a date, imported on
2026-11-23 at 10:46 in Vienna:

```json
{
  "id": "6742c0a1b2c3d4e5f6a7b8c9",
  "module": "health",
  "type": "other",
  "documentDate": "2026-11-23",
  "documentDateSource": "import",
  "importedAt": "2026-11-23T09:46:12Z",
  "issuer": null
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
| `GET` | `/api/v1/status` | Health check: is the backend running, is the database reachable? No token needed. |
| `POST` | `/api/v1/transcriptions` | Turn a recording into text; nothing is stored (since 2026-11-12) |
| `POST` | `/api/v1/auth/login` | Log in and get a token; no token needed (since 2026-11-18) |
| `POST` | `/api/v1/auth/logout` | Log out this device (since 2026-11-18) |
| `GET` | `/api/v1/me` | Get the logged-in user (since 2026-11-18) |
| `PATCH` | `/api/v1/me` | Change the display name (since 2026-11-18) and the time zone (since 2026-11-23) |
| `POST` | `/api/v1/me/password` | Change the password (since 2026-11-18) |
| `PUT` | `/api/v1/me/consent` | Record the consent (since 2026-11-18) |
| `GET` | `/api/v1/routines` | (since 2026-11-23) All routines of a module with status and progress |
| `POST` | `/api/v1/routines` | (since 2026-11-23) Create a routine |
| `GET` | `/api/v1/routines/{id}` | (since 2026-11-23) Get one routine |
| `PATCH` | `/api/v1/routines/{id}` | (since 2026-11-23) Change a routine |
| `POST` | `/api/v1/routines/{id}/pause` | (since 2026-11-23) Pause a routine |
| `POST` | `/api/v1/routines/{id}/resume` | (since 2026-11-23) Resume a routine |
| `DELETE` | `/api/v1/routines/{id}` | (since 2026-11-23) Delete a routine with its logs |
| `GET` | `/api/v1/routines/occurrences` | (since 2026-11-23) Occurrences of all routines in a date range, with status |
| `GET` | `/api/v1/routines/{id}/logs` | (since 2026-11-23) Logs of a routine |
| `POST` | `/api/v1/routines/{id}/logs` | (since 2026-11-23) Add a log: done or skipped |
| `DELETE` | `/api/v1/routines/{id}/logs/{logId}` | (since 2026-11-23) Delete a log |
| `POST` | `/api/v1/entries/{id}/routine-suggestions/{suggestionId}/confirm` | (proposal, stage 2) Confirm a routine suggestion; creates a log |
| `DELETE` | `/api/v1/entries/{id}/routine-suggestions/{suggestionId}` | (proposal, stage 2) Reject a routine suggestion |

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
- A text always gets a `documentDate` (since 2026-11-23): one named in the
  text, also a relative one such as „gestern“, or else the day of the
  import (steps 4 and 5). This also applies to voice notes, which the app
  sends as text.
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
   text) and look up `file.sha256` among the user's own entries. If the hash
   already exists there, the backend stops with `409 DUPLICATE`. This check
   comes **before** the model call, so a duplicate costs no model call. A
   unique index on `(userId, file.sha256)` also catches the case where the
   same user uploads the same file twice at the same time. The check applies
   across all modules of the user, never across users (see
   [per-user data](#per-user-data)).
3. Count the upload against the user's daily limit `uploads`. At the limit
   the backend stops with `429 LIMIT_REACHED` (see [daily limits](#daily-limits)).
4. Send the file to the vision LLM. It returns `documentDate`, `issuer`,
   `type`, `fullText` and `facts` as JSON. For a text import the language
   model gets the text and returns the same fields except `fullText`;
   `fullText` is the entered text. Since 2026-11-23 the instruction for a
   text import names today's date with the weekday in Europe/Vienna
   („Today is Monday, 2026-11-23“): a note is written now, and relative
   dates such as „heute“, „gestern“ or „letzten Montag“ refer to that day.
   If the text names no date at all, the model answers `null`. The
   instruction for files is unchanged (a document on paper has its own
   date, and the file prompt is calibrated).
5. Check the JSON (required fields, date format, allowed `type` of the
   module) and set `module`. The backend **drops** a single fact without
   `name` or without `value` instead of rejecting the whole upload. It only
   logs which field was missing (e.g. `facts[2].value`), **never** a value.
   The remaining facts are numbered without gaps (`f1`, `f2`, …).
   `documentDateSource` is `document`. Only for a text import without a
   date: `documentDate` becomes the day of `importedAt` in Europe/Vienna
   and `documentDateSource` becomes `import` (e.g. imported at
   2026-11-23T23:30:00Z → `2026-11-24`). A file without a date keeps
   `null`.
6. Check the recognized values (see [verification](#verification)). For a
   file, reading the original runs **in parallel** to step 4, so the upload
   does not take longer: a PDF page with a text layer is read directly, a
   page without one and a photo go through OCR. A photo (JPEG, PNG, HEIC,
   WebP) is first prepared: turned upright and scaled to a long side
   between 2500 and 4096 pixels. For a text import the
   values are compared against the entered text; a date from the import
   day is not checked.
7. Split the full text into chunks and compute embeddings.
8. Store the original file, the chunks with embeddings, the words read
   from the original (with positions) and the entry, each marked with the
   user.

(Proposal, stage 2, 2026-11-23: if the user has active routines in the module,
step 4 also returns routine suggestions in the same call, and step 5 checks
them; a suggestion never fails the upload. See
[routine suggestions](#routine-suggestions-proposal-stage-2).)

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
  "documentDateSource": "document",
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
| 409 | `DUPLICATE` | The same user already has a file or text with the same SHA-256 hash. `details.entryId` names the existing entry. |
| 413 | `FILE_TOO_LARGE` | File over 10 MB or text over 100,000 characters (then `details.maxChars`: `100000`) |
| 415 | `UNSUPPORTED_FILE_TYPE` | Not JPEG, PNG, HEIC, WebP or PDF |
| 422 | `UNREADABLE` | No document recognizable, e.g. blurry photo (only for `file`) |
| 429 | `LIMIT_REACHED` | Daily limit reached, `details.limit`: `uploads` (see [daily limits](#daily-limits)) |
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

All parameters are optional. The list only contains the user's own entries
of the chosen module, never those of other users (see
[per-user data](#per-user-data)). Without parameters the newest document
from `health` comes first.

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
      "documentDateSource": "document",
      "importedAt": "2026-11-03T18:42:07Z",
      "issuer": "Labor Muster GmbH, Salzburg"
    },
    {
      "id": "6727a20bc2b4e5d6f7a8b9d4",
      "module": "health",
      "type": "doctor_letter",
      "documentDate": "2026-09-02",
      "documentDateSource": "document",
      "importedAt": "2026-11-03T18:50:12Z",
      "issuer": "Dr. Erika Beispiel, Allgemeinmedizin"
    },
    {
      "id": "6727a2a9c2b4e5d6f7a8b9e7",
      "module": "health",
      "type": "other",
      "documentDate": null,
      "documentDateSource": "document",
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
| 404 | `ENTRY_NOT_FOUND` | No entry with this ID, or it belongs to another user |
| 500 | `INTERNAL_ERROR` | |

### 3.4 Get the original file

```http
GET /api/v1/entries/6727a1f3c2b4e5d6f7a8b9c0/file
```

The app uses this to show the original next to the recognized values. This
makes it possible to check whether the vision LLM read correctly.

The file is only returned if the entry belongs to the user; the backend
also checks the owner stored with the file itself.

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
| 404 | `ENTRY_NOT_FOUND` | No entry with this ID, or it belongs to another user |
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
- **A date from the import day** (`documentDateSource: "import"`, since
  2026-11-23): the model recognized no date, so a request that contains
  `documentDate` first sets the date back to `null` and
  `documentDateSource` to `document`; then the normal rules apply. A date,
  also the import day itself, becomes a correction with
  `"recognized": null`. `null` removes the date without a correction, and
  the entry is stored (it no longer has the import day). A request without
  `documentDate` keeps the mark.

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

Corrections only change `type`, `documentDate` (and with it
`documentDateSource`), `issuer` and `facts`. The chunks and embeddings for
search stay unchanged because they are derived from `fullText`.

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
  "documentDateSource": "document",
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
| 404 | `ENTRY_NOT_FOUND` | No entry with this ID, or it belongs to another user; nothing is changed |
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
| `module` | String | Optional, default `health`. Only the user's own documents of this module are searched. |

```json
{
  "question": "Welchen Hämoglobinwert hatte ich im Oktober?",
  "module": "health"
}
```

**Processing in the backend**

1. Check the question and the module.
2. If the search index is being rebuilt (e.g. after an update of its
   definition), stop with `503 MODEL_UNAVAILABLE` (see below). This happens
   before any model call and does not count against the daily limit.
3. Count the question against the user's daily limit `questions`. At the
   limit the backend stops with `429 LIMIT_REACHED` (see
   [daily limits](#daily-limits)).
4. Compute the embedding of the question.
5. Find the best-matching chunks of the user's own entries in the module
   via Atlas Vector Search (at most 5). The search itself filters by user
   and module, so chunks of other users are never candidates.
6. First layer: chunks with a score below 0.82 are dropped (coarse filter,
   calibrated on 2026-11-04 with `backend/cmd/calibrate`, see the comment on
   `core.MinScore`), as are chunks whose entry no longer exists or does not
   belong to the user. If none remains, the backend answers with the fixed
   sentence (see below) without calling the language model.
7. Otherwise the language model gets the question and only these chunks. It
   answers only from these chunks and sets references like `[1]`. If the
   chunks do not answer the question, it answers with the fixed sentence.
8. Second layer: if the answer contains no valid reference, it counts as
   "nothing found" (fixed sentence, `found: false`). An answer without
   sources never reaches the app.

**While the search index is being rebuilt**, questions do not work. The
backend answers `503 MODEL_UNAVAILABLE` with the message „Die Suche wird
gerade aktualisiert. Bitte versuche es in ein paar Minuten erneut.“ instead
of an answer, never „nothing found“. Usually the backend knows this before
step 3; then the question costs nothing and does not count. If it only
notices in step 5, the embedding of step 4 has already been computed and
the question counts. Upload, timeline and everything else work normally
meanwhile.

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
| 429 | `LIMIT_REACHED` | Daily limit reached, `details.limit`: `questions` (see [daily limits](#daily-limits)) |
| 503 | `MODEL_UNAVAILABLE` | Embedding or language model unreachable, or the language model returns no usable answer (e.g. empty or blocked). Also while the search index is being rebuilt, then with the message „Die Suche wird gerade aktualisiert. Bitte versuche es in ein paar Minuten erneut.“ |
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

(Proposal, stage 2, 2026-11-23: before step 4 the backend also deletes the routine
logs confirmed from this entry (`source: entry`, this `entryId`). Pending
routine suggestions are part of the entry and go with it. See
[routine suggestions](#routine-suggestions-proposal-stage-2).)

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
| 404 | `ENTRY_NOT_FOUND` | Also when deleting the same entry a second time, and for an entry of another user (nothing is deleted) |
| 500 | `INTERNAL_ERROR` | |

### 3.8 Status (health check)

```http
GET /api/v1/status
```

Checks whether the backend is running and MongoDB is reachable. Calls no
model and therefore costs nothing. The path is deliberately not `/health`,
so it is not confused with the module `health`.

One of the two endpoints that work without a token (see
[login and tokens](#login-and-tokens)), so that monitoring and the app can
check the server before a login. It returns nothing about users or entries.

**Response `200 OK`**

```json
{
  "status": "ok",
  "database": "ok",
  "speech": "ok"
}
```

**Response `503 Service Unavailable`** if MongoDB does not respond:

```json
{
  "status": "error",
  "database": "unreachable",
  "speech": "ok"
}
```

`speech` (since 2026-11-12) tells whether transcription (3.9) works right
now:

| `speech` | Meaning |
|---|---|
| `ok` | The speech recognition answers |
| `unavailable` | Switched on, but the speech recognition or `ffmpeg` does not answer; `POST /transcriptions` gives `503` |
| `off` | Switched off on this server (`SPEECH_ENGINE=none`) |

The HTTP status depends only on the database. The app can use `speech` to
hide or disable the microphone button.

This endpoint deliberately does not use the error format from section 1, so
monitoring tools can read both cases the same way.

### 3.9 Transcribe a recording

Since 2026-11-12. Turns a spoken note (e.g. a "spoken doctor's letter"
after a visit) into text. The backend **only returns the text**: no entry
is created, the recording is **not stored**, and there is no duplicate
check. The flow in the app:

1. Record, then send the recording here.
2. Show the returned `text` to the user for checking and editing. It is
   unchecked machine output.
3. Only when the user confirms: send the (edited) text with
   `POST /api/v1/entries` and the form field `text` (3.1, text import,
   unchanged). From there on it is an ordinary text import.
4. Delete the local recording on the phone.

```http
POST /api/v1/transcriptions
Content-Type: multipart/form-data; boundary=----lifelog
```

**Request**

| Form field | Note |
|---|---|
| `audio` | Exactly one recording |

```http
------lifelog
Content-Disposition: form-data; name="audio"; filename="notiz.m4a"
Content-Type: audio/mp4

<binary data>
------lifelog--
```

Allowed formats. The backend recognizes the format from the content, not
from the file name or the `Content-Type`:

| MIME type | Format | Typical source |
|---|---|---|
| `audio/mp4` | M4A / MP4 with AAC | iOS (Voice Memos, AVAudioRecorder), Android, Flutter package `record` (default) |
| `audio/3gpp` | 3GP | older Android recorders |
| `audio/amr` | AMR | older Android recorders |
| `audio/wav` | WAV | `record` with `AudioEncoder.wav` |
| `audio/ogg` | OGG (Opus, Vorbis) | messengers, Android |
| `audio/webm` | WebM (Opus) | Android, browsers |
| `audio/mpeg` | MP3 | other apps |
| `audio/flac` | FLAC | other apps |

Limits:

- At most **10 MB** and at most **5 minutes**. Recordings shorter than
  0.5 seconds count as "no speech".
- Recommended for the app: package `record` with `AudioEncoder.aacLc`
  (M4A), mono, 16 kHz, about 32 kbit/s. A minute is then about 240 KB.
- Language: always German. Whisper is not asked to detect the language and
  does not translate.

The call is **synchronous** and can take up to about 120 seconds (time
limit of the backend). The app sets its HTTP timeout for this call to at
least 130 seconds and shows a loading indicator.

**Response `200 OK`**

```json
{
  "text": "Arztbrief vom 12. November 2026. Kniegelenk rechts, leichter Erguss. Ibuprofen 400 Milligramm dreimal täglich für eine Woche.",
  "durationSeconds": 21.4,
  "language": "de",
  "engine": "whisper.cpp large-v3-turbo"
}
```

| Field | Type | Note |
|---|---|---|
| `text` | String | Recognized text, never empty (no speech → `422 NO_SPEECH`). Sentences separated by a space; how numbers and units are written ("400 Milligramm" or "400 mg") depends on the model. |
| `durationSeconds` | Number | Length of the recording in seconds, one decimal |
| `language` | String | Always `de` |
| `engine` | String | Program and model, for display and support |

**Errors**

| HTTP | Code | Note | `message` |
|---|---|---|---|
| 400 | `FILE_MISSING` | No field `audio`, or it is empty | Bitte eine Aufnahme mitschicken. |
| 400 | `INVALID_REQUEST` | More than one `audio` field (`details.field`: `audio`) | Bitte genau eine Aufnahme mitschicken. |
| 413 | `FILE_TOO_LARGE` | Over 10 MB | Die Aufnahme ist größer als 10 MB. |
| 413 | `AUDIO_TOO_LONG` | Over 5 minutes; `details.maxSeconds`: `300` | Die Aufnahme ist länger als 5 Minuten. Bitte kürzer sprechen oder aufteilen. |
| 415 | `UNSUPPORTED_FILE_TYPE` | Not one of the formats above | Dieses Audioformat wird nicht unterstützt. Erlaubt sind M4A, 3GP, AMR, WAV, OGG, WebM, MP3 und FLAC. |
| 422 | `UNREADABLE` | Damaged, or no audio track | Die Aufnahme lässt sich nicht abspielen. |
| 422 | `NO_SPEECH` | Silence, noise only, or shorter than 0.5 s | In der Aufnahme ist keine Sprache zu hören. |
| 429 | `LIMIT_REACHED` | Daily limit reached, `details.limit`: `transcriptions` (since 2026-11-18, see [daily limits](#daily-limits)). Counted after the checks of size and format, before ffmpeg and Whisper. | Du hast dein Tageslimit erreicht. Morgen geht es weiter. |
| 503 | `MODEL_UNAVAILABLE` | Speech recognition or `ffmpeg` not reachable, or the time limit was exceeded | Die Spracherkennung ist gerade nicht erreichbar. Bitte später noch einmal versuchen. |
| 500 | `INTERNAL_ERROR` | Unexpected error | |

**How the backend does it** (for understanding; not part of the contract):

1. Check size and format.
2. Convert with `ffmpeg` to 16 kHz mono PCM, the format Whisper needs. The
   recording lies in a temporary file for these few seconds (an M4A from
   iOS cannot be read through a pipe); the file is deleted right after,
   also on errors. Nothing else is written.
3. Check the length (5 minutes) and the loudness: if there is less than
   0.3 seconds of sound above the noise floor, the answer is `NO_SPEECH`
   without asking Whisper at all. Whisper is known to invent sentences in
   silence ("Untertitel im Auftrag des ZDF").
4. Whisper (whisper.cpp, model large-v3-turbo, running locally as its own
   server) with voice activity detection (only parts with speech are
   decoded), language fixed to German, temperature 0, and a list of
   general medical terms as prompt (`whisper.Vocabulary`, since
   2026-11-12; see docs/speech/README.md).
   With `SPEECH_ENGINE=gemini` (since 2026-11-20, for servers without a
   GPU, where Whisper needs about 40 s per 30-second window) the read
   model (`GEMINI_MODEL`, paid tier) writes the text instead, word for
   word, with the same list as a hint for spellings. Steps 1–3 and 5 stay
   the same, so silence still costs no model call. `engine` is then e.g.
   `"Gemini gemini-3.8-flash"`.
5. Remove known phantom sentences and repeated identical segments; if
   nothing is left, `NO_SPEECH`.

The server log contains only length, size and result codes, never the
text.

### 3.10 Log in

Since 2026-11-18. Besides `/status` the only endpoint that works without a
token.

```http
POST /api/v1/auth/login
Content-Type: application/json
```

**Request**

| Field | Type | Note |
|---|---|---|
| `username` | String | Required. The backend removes whitespace at the edges and converts it to lowercase, so `Tester01 ` logs in as `tester01` (phone keyboards often start with a capital letter). |
| `password` | String | Required. Taken exactly as sent, not trimmed. |

The body is at most 4 KB. Unknown fields are an error.

```json
{
  "username": "tester01",
  "password": "h4Pq8sZt2kXw"
}
```

**Response `200 OK`**

```http
Cache-Control: no-store
```

```json
{
  "token": "q3Xv9LmT0bR7sYk2WcN8pZ4hJ6dF1gA5eU0iO3rK7tM",
  "expiresAt": "2026-12-18T09:15:42Z",
  "mustChangePassword": true,
  "user": {
    "id": "673b0c1fa2d4e5f6a7b8c9d0",
    "username": "tester01",
    "displayName": "tester01",
    "consentVersion": null
  }
}
```

| Field | Type | Note |
|---|---|---|
| `token` | String | Opaque, 43 characters. Sent as `Authorization: Bearer <token>` with every further request. Only in this response; the backend cannot show it again. |
| `expiresAt` | String | RFC 3339, UTC. 30 days after the login. |
| `mustChangePassword` | Boolean | `true` for a start password. The app then asks for a new password ([3.14](#314-change-the-password)) before anything else. |
| `user` | Object | `id` (ObjectId), `username`, `displayName`, `consentVersion` (since 2026-11-18), as in [3.12](#312-get-the-logged-in-user) |

`Cache-Control: no-store`: the response contains the token and must not end
up in any cache.

**Failed attempts**

A failed attempt is a login with an unknown username, a wrong password or
a disabled account, and a wrong `currentPassword` when changing the
password ([3.14](#314-change-the-password)). The backend counts them in
two ways:

1. **Per username and IP address:** 5 failed attempts for the same username
   from the same IP address within 15 minutes lock this combination for
   1 minute. Every further lock doubles the time (2, 4, 8 minutes, at most
   15 minutes). After one hour without a failed attempt, or after a
   successful login, the next lock starts at 1 minute again. Someone
   guessing from another IP address therefore cannot lock out an account.
2. **Per IP address,** whatever the username: 30 failed attempts within
   15 minutes lock this address for 15 minutes. IPv6 addresses count per
   /64 network.

During a lock the backend answers `429 TOO_MANY_ATTEMPTS` without checking
the password, even if it is correct. The header `Retry-After` and
`details.retryAfterSeconds` give the remaining wait in seconds, rounded up;
both have the same value. The `message` names no fixed time, because the
wait grows.

The IP address is the address of the connection. Behind a reverse proxy
(e.g. Caddy) the backend takes the client address from `X-Forwarded-For`,
but only if the proxy is configured as trusted (backend/README.md). The
counters live in the backend's memory; a restart resets them.

An unknown username takes as long as a known one (the backend compares
against a dummy hash), so neither the response nor its timing reveals
whether an account exists.

**Errors**

| HTTP | Code | Note | `message` |
|---|---|---|---|
| 400 | `INVALID_REQUEST` | Body is not a JSON object, larger than 4 KB, or has an unknown field; `username` or `password` missing, empty or not a string | Bitte Benutzername und Passwort schicken. |
| 401 | `INVALID_CREDENTIALS` | Unknown username, wrong password, disabled account, or a password longer than 72 bytes. Always the same message, so the response does not reveal which. | Benutzername oder Passwort stimmt nicht. |
| 429 | `TOO_MANY_ATTEMPTS` | Too many failed attempts, see above. Header `Retry-After`, `details.retryAfterSeconds` | Zu viele Fehlversuche. Bitte versuche es in einigen Minuten erneut. |
| 500 | `INTERNAL_ERROR` | | |

```http
HTTP/1.1 429 Too Many Requests
Retry-After: 60
```

```json
{
  "error": {
    "code": "TOO_MANY_ATTEMPTS",
    "message": "Zu viele Fehlversuche. Bitte versuche es in einigen Minuten erneut.",
    "details": { "retryAfterSeconds": 60 }
  }
}
```

### 3.11 Log out

Since 2026-11-18.

```http
POST /api/v1/auth/logout
Authorization: Bearer q3Xv9LmT0bR7sYk2WcN8pZ4hJ6dF1gA5eU0iO3rK7tM
```

No body.

**Response `204 No Content`**

The token of this device is invalid afterwards; a further request with it
gets `401 UNAUTHORIZED`. Other devices of the same account stay logged in.
Logout also works while `mustChangePassword` is `true`. The app deletes its
stored token in any case, also if this request fails.

**Errors**

| HTTP | Code | Note |
|---|---|---|
| 401 | `UNAUTHORIZED` | Token missing or no longer valid (section 1) |
| 500 | `INTERNAL_ERROR` | |

### 3.12 Get the logged-in user

Since 2026-11-18. Also works while `mustChangePassword` is `true`.

```http
GET /api/v1/me
Authorization: Bearer q3Xv9LmT0bR7sYk2WcN8pZ4hJ6dF1gA5eU0iO3rK7tM
```

**Response `200 OK`**

```json
{
  "id": "673b0c1fa2d4e5f6a7b8c9d0",
  "username": "tester01",
  "displayName": "Erika",
  "mustChangePassword": false,
  "consentVersion": "2026-11-18",
  "timeZone": "Europe/Vienna"
}
```

| Field | Type | Note |
|---|---|---|
| `id` | String | ObjectId of the user |
| `username` | String | 3–32 characters, only `a-z` and `0-9`. Set when the account is created; cannot be changed through the API. |
| `displayName` | String | Name the app shows, e.g. in a greeting. Equal to the username unless the administrator gave one or the user changed it ([3.13](#313-change-the-display-name)). |
| `mustChangePassword` | Boolean | `true` until the start password has been changed ([3.14](#314-change-the-password)) |
| `consentVersion` | String or `null` | Version of the consent text the user last agreed to ([3.15](#315-record-the-consent)); `null` if they have not agreed yet. The app compares it with the version of its own text and shows the notice again if they differ. Since 2026-11-18. |
| `timeZone` | String | Since 2026-11-23. IANA name of the user's time zone, e.g. `"Europe/Vienna"`. `"Europe/Vienna"` until the app sets another one ([3.13](#313-change-the-display-name)). Routines compute in it ([3.16](#time-zone-and-points-in-time)). |

**Errors**

| HTTP | Code | Note |
|---|---|---|
| 401 | `UNAUTHORIZED` | Section 1 |
| 500 | `INTERNAL_ERROR` | |

### 3.13 Change the display name

Since 2026-11-18.

```http
PATCH /api/v1/me
Content-Type: application/json
```

**Request**

| Field | Type | Note |
|---|---|---|
| `displayName` | String | Whitespace at the edges is removed; then 1 to 50 characters (Unicode characters, not bytes) without control characters such as line breaks or tabs. Required until 2026-11-23; since then `displayName` or `timeZone` (below). |

Only `displayName` and `timeZone` can be changed. Any other field is an error.

```json
{
  "displayName": "Erika"
}
```

**Response `200 OK`**

The user as in [3.12](#312-get-the-logged-in-user), with the stored
(trimmed) name.

**Errors**

| HTTP | Code | Note | `message` |
|---|---|---|---|
| 400 | `INVALID_REQUEST` | `displayName` `null`, not a string, empty after trimming, longer than 50 characters, or with control characters (`details.field`: `displayName`) | Der Anzeigename muss 1 bis 50 Zeichen lang sein. |
| 400 | `INVALID_REQUEST` | Body is not a JSON object, or has an unknown field (`details.field` names it) | Unbekanntes Feld: username. |
| 500 | `INTERNAL_ERROR` | | |

```json
{
  "error": {
    "code": "INVALID_REQUEST",
    "message": "Der Anzeigename muss 1 bis 50 Zeichen lang sein.",
    "details": { "field": "displayName" }
  }
}
```

**Time zone (since 2026-11-23).** `PATCH /me` also accepts
`timeZone`:

| Field | Type | Note |
|---|---|---|
| `timeZone` | String | IANA name of a place, e.g. `"Europe/Lisbon"`, stored exactly as sent. Accepted: `UTC`, `Etc/UTC` (some Android phones report it), or `Area/Place` or `Area/Place/Place` with an area of `Africa`, `America`, `Antarctica`, `Arctic`, `Asia`, `Atlantic`, `Australia`, `Europe`, `Indian`, `Pacific`; every part starts with an upper-case letter and has only `A–Z`, `a–z`, `0–9`, `_` and `-`; at most 64 characters; the zone database must know it. Legacy names of places such as `Asia/Calcutta` are fine. Refused: abbreviations and old names outside the areas (`CET`, `EST5EDT`, `Japan`, `US/Eastern`, `GMT`, `Etc/GMT+1`), offsets (`+01:00`), `""`, `"Local"`, other trees of the database (`posix/Europe/Vienna`) and other spellings (`europe/vienna`). |

With it, `displayName` and `timeZone` are both optional, but at least one
must be sent. Everything is checked first; then both are stored together,
so a request with one invalid field changes nothing. If a request has
several errors, the first in this order is reported: unknown field
(alphabetical), `displayName`, `timeZone`, neither of both. The response is
the user as in 3.12, including `timeZone`. The login response (3.10) has
no `timeZone`; the app reads it from `GET /me`.

```json
{
  "timeZone": "Europe/Lisbon"
}
```

| HTTP | Code | Note | `message` |
|---|---|---|---|
| 400 | `INVALID_REQUEST` | `timeZone` `null`, not a string, or not accepted (above) (`details.field`: `timeZone`, `details.value`: the value sent, missing for `null`) | Unbekannte Zeitzone. |
| 400 | `INVALID_REQUEST` | Neither `displayName` nor `timeZone` (`{}`); no `details` | Bitte einen Anzeigenamen oder eine Zeitzone schicken. |

What the time zone changes and what it does not:

- **Routines compute in it:** the wall-clock times of occurrences, "today",
  the week and the period of the progress, pauses, start and end date, the
  assignment of suggestions to occurrences, and the date and time the model
  gets ([3.16](#time-zone-and-points-in-time)).
- **Points in time stay UTC** (`importedAt`, `takenAt`, `dueAt`,
  `createdAt`) and always come from the server clock, never from the phone.
- **Daily limits** stay on Europe/Vienna for now (open question in
  [section 4](#open-questions-on-routines-2026-11-23)).
- After a change, "08:00" means 08:00 at the new place: occurrences move.
  Existing logs stay unchanged and keep their occurrence
  ([logs](#logs)).

App: it reads the phone's IANA name with a package such as
`flutter_timezone`; `DateTime.timeZoneName` only gives abbreviations such
as `CET`. At start it compares the name with `timeZone` from `GET /me`,
sends `PATCH /me` if they differ, and then plans its reminders again.

Backend (`core.CheckTimeZone`): the rule above, then `time.LoadLocation`
with the zone database embedded (`import _ "time/tzdata"`).
`time.LoadLocation` alone would accept `""` (UTC), `"Local"` (the server's
zone) and `CET`, and it reads the operating system's files first: on
Linux `posix/Europe/Vienna` would load, on macOS `Europe/vienna`. With the
rule the result does not depend on the server's operating system. A user
without the field counts as `Europe/Vienna`.

### 3.14 Change the password

Since 2026-11-18. Also works while `mustChangePassword` is `true`; that is
how a start password is replaced.

```http
POST /api/v1/me/password
Content-Type: application/json
```

**Request**

| Field | Type | Note |
|---|---|---|
| `currentPassword` | String | Required. The password used so far (for a new account: the start password). |
| `newPassword` | String | Required. At least 10 characters (Unicode characters), at most 72 bytes in UTF-8 (bcrypt's limit; an umlaut counts as 2 bytes), not equal to `currentPassword`. Taken exactly as sent, not trimmed. There are no further rules such as required special characters. |

```json
{
  "currentPassword": "h4Pq8sZt2kXw",
  "newPassword": "Sonnenblume am Mönchsberg"
}
```

**Processing in the backend**

1. If the username together with this IP address, or the IP address
   alone, is locked after failed attempts ([3.10](#310-log-in)), answer
   `429 TOO_MANY_ATTEMPTS` without checking anything.
2. Check `currentPassword`. A wrong one is `400 WRONG_PASSWORD` and counts
   as a failed attempt, with the same counters as the login.
3. Check `newPassword` (`400 WEAK_PASSWORD`).
4. Store the new password and set `mustChangePassword` to `false`.
5. End all other tokens of the account. The token of this request stays
   valid, so the app stays logged in.

If a request has several errors, the first in this order is reported.

**Response `204 No Content`**

No body.

**Errors**

| HTTP | Code | Note | `message` |
|---|---|---|---|
| 400 | `INVALID_REQUEST` | Body is not a JSON object, or has an unknown field; a field missing or not a string | Bitte das bisherige und das neue Passwort schicken. |
| 400 | `WRONG_PASSWORD` | `currentPassword` is wrong; counts as a failed attempt. Deliberately `400` and not `401`: the token is still valid, and the app must not log out. | Das bisherige Passwort stimmt nicht. |
| 400 | `WEAK_PASSWORD` | `newPassword` has fewer than 10 characters | Das neue Passwort muss mindestens 10 Zeichen lang sein. |
| 400 | `WEAK_PASSWORD` | `newPassword` has more than 72 bytes | Das neue Passwort ist zu lang. Erlaubt sind höchstens 72 Byte, Umlaute zählen doppelt. |
| 400 | `WEAK_PASSWORD` | `newPassword` equals `currentPassword` | Das neue Passwort muss sich vom bisherigen unterscheiden. |
| 429 | `TOO_MANY_ATTEMPTS` | Too many failed attempts, as in [3.10](#310-log-in). Header `Retry-After`, `details.retryAfterSeconds` | Zu viele Fehlversuche. Bitte versuche es in einigen Minuten erneut. |
| 500 | `INTERNAL_ERROR` | | |

### 3.15 Record the consent

Since 2026-11-18. The app shows its consent notice and sends the version
of that text once the user agrees. Like every endpoint except the three
named in section 1, it answers `403 PASSWORD_CHANGE_REQUIRED` while the
start password is unchanged; the app asks for the consent after the
password change.

```http
PUT /api/v1/me/consent
Content-Type: application/json
```

**Request**

| Field | Type | Note |
|---|---|---|
| `version` | String | Required. Version of the consent text the user agreed to, e.g. `"2026-11-23"`. Whitespace at the edges is removed; then 1 to 100 characters without control characters. |

```json
{
  "version": "2026-11-23"
}
```

**Response `204 No Content`**

The backend stores `consentVersion` (the trimmed `version`) and `consentAt`
(the server time of the request, UTC) with the user. Sending the same
version again only sets `consentAt` anew. `GET /me` and the login response
show `consentVersion` from then on.

**Errors**

| HTTP | Code | Note | `message` |
|---|---|---|---|
| 400 | `INVALID_REQUEST` | `version` missing, `null`, not a string, empty after trimming, longer than 100 characters, or with control characters (`details.field`: `version`) | Bitte die Version des Hinweises schicken. |
| 400 | `INVALID_REQUEST` | Body is not a JSON object, or has an unknown field (`details.field` names it) | Unbekanntes Feld: … |
| 500 | `INTERNAL_ERROR` | | |

### 3.16 Routines: overview and data model

> **Stage 1 implemented in the backend on 2026-11-23** (Dylan, with
> Dogan's approval of the proposal; Dogan reviews the code). The app
> follows. Wish of Prof. Pree of 2026-11-16
> ([meeting](meetings/2026-11-16.md)), concept by Dylan, agreed in the team
> on 2026-11-23. Open questions: [section 4](#open-questions-on-routines-2026-11-23).
>
> **Two stages** (Dogan's review, 2026-11-23):
>
> - **Stage 1, implemented:** routines, occurrences, logs, pauses,
>   reminders and the time zone (3.16–3.27, 3.12/3.13).
> - **Stage 2, still a proposal:** the routine suggestions (section 2,
>   3.28, 3.29), marked "(proposal, stage 2)".
>   They change the read prompt for every document, so `cmd/calibrate` must
>   run again, and they send routine names to Gemini, so the consent text
>   must change first.
>
> Parts of 3.16–3.27 that only matter with suggestions (`source: entry`,
> removing suggestions when a routine is deleted) are marked "stage 2".

A routine is like an alarm clock for something the user wants to do
regularly. It is a view of its own next to the timeline in module `health`
(button „Neue Routine“). Examples:

| Routine | Schedule |
|---|---|
| „Vitamin D“, 2 Stück | daily at 08:00 |
| „Blutdruck messen“ | daily at 08:00 and 20:00, for 14 days |
| „Eisbaden“ | weekly on Tuesday and Friday at 07:00 |
| „Wasser“, 250 ml | hourly from 08:00 to 20:00 |
| „Eiweiß“, 30 g | daily at 20:00, logged in portions |

Terms:

- **Routine:** name, optional target amount per occurrence, schedule,
  start date and optional end date.
- **Occurrence** („Termin“): a point in time the schedule plans, e.g.
  2026-11-23 at 08:00.
- **Log** („Verlaufseintrag“): the user did it (`done`) or skipped it
  (`skipped`), for an occurrence or without one. It comes from the app
  (`manual`), from a reminder (`notification`) or from a confirmed
  suggestion on an entry (`entry`).
- **Progress:** e.g. „heute 1 von 2“, „Woche 5 von 7“, „Tag 6 von 14“,
  „heute 1.250 von 3.250 ml“.

Who does what: the backend expands the schedule into occurrences, keeps the
logs, and computes status and progress. The app shows them and plans the
reminders locally from the occurrences ([reminders](#reminders-in-the-app)).
There are no push messages from the server and no Firebase.

#### The routine

This is how every routine endpoint returns a routine:

```json
{
  "id": "6742c0a1b2c3d4e5f6a7b8c9",
  "module": "health",
  "name": "Blutdruck messen",
  "amount": null,
  "unit": null,
  "schedule": {
    "frequency": "daily",
    "interval": 1,
    "times": ["08:00", "20:00"]
  },
  "startDate": "2026-11-18",
  "endDate": "2026-12-01",
  "status": "active",
  "pauses": [],
  "createdAt": "2026-11-18T07:02:11Z",
  "progress": {
    "today": { "done": 1, "skipped": 0, "due": 2, "amountDone": null, "amountTarget": null },
    "week": { "done": 1, "skipped": 0, "due": 14, "amountDone": null, "amountTarget": null },
    "period": { "day": 6, "days": 14, "done": 9, "skipped": 1, "due": 28, "amountDone": null, "amountTarget": null }
  }
}
```

| Field | Type | Note |
|---|---|---|
| `id` | String | ObjectId of the routine |
| `module` | String | See [modules](#modules); default `health`. Cannot be changed. |
| `name` | String | Name the user gave, e.g. `"Vitamin D"`. Unique per user and module. |
| `amount` | Number or `null` | Target amount per occurrence, e.g. `2` or `0.5`. `null`: no amount. |
| `unit` | String or `null` | Unit of `amount`, e.g. `"Stück"`, `"ml"`, `"g"`. Only with an `amount`; an amount without a unit is allowed (20 push-ups). |
| `schedule` | Object | When the occurrences are, see [schedule](#schedule) |
| `startDate` | String | First day, `YYYY-MM-DD`, a plain date like `documentDate` |
| `endDate` | String or `null` | Last day, inclusive. `null`: no end. |
| `status` | String | `upcoming`, `active`, `paused` or `ended`, see [routine status](#routine-status) |
| `pauses` | List | All pauses, oldest first, see [pauses](#pauses) |
| `createdAt` | String | Time of creation (RFC 3339, UTC) |
| `progress` | Object | Computed at the time of the request, see [progress](#routine-progress) |

#### Schedule

Three kinds, each with a whole number `interval` N:

| `frequency` | Fields | Occurrences |
|---|---|---|
| `hourly` | `interval` (1–23), `windowStart`, `windowEnd` | Every day from `windowStart` every N hours up to `windowEnd`, which counts if it is hit. The window does not cross midnight: `windowStart` < `windowEnd`. |
| `daily` | `interval` (1–365), `times` | At every time in `times` on every N-th day, counted from `startDate` (day 0). |
| `weekly` | `interval` (1–52), `weekdays`, `times` | At every time in `times` on the given weekdays of every N-th week. Weeks run from Monday to Sunday; the week containing `startDate` is week 0. |

- Times are `"HH:MM"`, 24 hours, `00:00` to `23:59`. `times` has 1 to 24
  different values; the backend stores them sorted.
- `weekdays` has 1 to 7 different values of `mon`, `tue`, `wed`, `thu`,
  `fri`, `sat`, `sun`, stored in this order.
- A field that does not belong to the `frequency` is an error (e.g.
  `times` with `hourly`).
- At most 24 occurrences per day. For `hourly` this follows from the rules
  (at least one hour apart, within one day); for `daily` and `weekly` from
  the at most 24 `times`.
- No occurrences before `startDate` or after `endDate`.

`{ "frequency": "hourly", "interval": 1, "windowStart": "08:00", "windowEnd": "20:00" }`
gives 13 occurrences a day: 08:00, 09:00, …, 20:00. With `"interval": 3`:
08:00, 11:00, 14:00, 17:00, 20:00.

`{ "frequency": "daily", "interval": 2, "times": ["08:00"] }` with
`startDate` 2026-11-23: 23, 25, 27 November, … at 08:00.

`{ "frequency": "weekly", "interval": 2, "weekdays": ["tue", "fri"], "times": ["07:00"] }`
with `startDate` 2026-11-23 (a Monday): 24 and 27 November, then 8 and
11 December, at 07:00.

This is deliberately a simple model of its own, not an iCalendar `RRULE`.
It covers the wishes, is easy to check, and the app can explain it in one
line („jeden 2. Tag um 08:00“).

#### Time zone and points in time

- Every wall-clock time (`times`, `windowStart`, `windowEnd`) and every
  date (`startDate`, `endDate`, pauses, "today", the week) refers to the
  user's time zone `timeZone` ([3.12](#312-get-the-logged-in-user),
  default `Europe/Vienna`).
- Points in time (`dueAt`, `takenAt`, `createdAt`) are RFC 3339 in UTC,
  like `importedAt`. "Now" is always the server clock, never the phone's.
- **Daylight saving time:** a time that does not exist on the day the
  clocks go forward (02:30 on the last Sunday of March in Vienna) moves
  forward by one hour (03:30). A time that exists twice on the day the
  clocks go back takes the earlier one (still summer time). If two
  occurrences fall on the same point in time this way, they become one.
  In general a time in a gap moves forward by the length of the gap (30
  minutes on Lord Howe Island). These rules are covered by tests; the
  cases are in `backend/internal/core/testdata/routine_occurrences.json`,
  which the app's mock can read too.
- `date` of an occurrence is always the local date of its `dueAt`. A time
  moved past midnight by a gap (only Pacific/Apia on 2011-12-30 so far)
  therefore counts on the next day; `startDate` and `endDate` refer to the
  planned day.
- A local day is the range from its first moment (midnight, or the end of
  a gap at midnight, as in America/Santiago) to the first moment of the
  next day; it can have 23 or 25 hours. "The day of `takenAt`" (3.25,
  limit per day) means this range.
- After a change of `timeZone`, "08:00" means 08:00 at the new place.
  Occurrences are computed in the new zone from then on, also for past
  days. Logs keep their occurrence (see [logs](#logs), rule 2).

#### Occurrences

The backend computes occurrences on request; they are not stored.
`GET /routines/occurrences` ([3.24](#324-list-occurrences))
returns them:

```json
{
  "routineId": "6742c0a1b2c3d4e5f6a7b8c9",
  "dueAt": "2026-11-23T19:00:00Z",
  "date": "2026-11-23",
  "time": "20:00",
  "status": "open",
  "logId": null
}
```

| Field | Type | Note |
|---|---|---|
| `routineId` | String | |
| `dueAt` | String | Point in time (RFC 3339, UTC) |
| `date`, `time` | String | Local date and time of `dueAt` in the user's time zone, so the app does not convert. After a forward shift `time` is the shifted time (`03:30`). |
| `status` | String | `done` or `skipped`: there is a log. `paused`: the day is in a pause and there is no log. `open`: otherwise, before or after `dueAt`; the app sees from `dueAt` whether it is overdue. |
| `logId` | String or `null` | The log for `done` and `skipped`, otherwise `null` |

#### Pauses

A routine can be paused, e.g. for a week, and resumed
([3.21](#321-pause-a-routine), [3.22](#322-resume-a-routine)).
Pauses are whole days in the user's time zone:

```json
{
  "pauses": [
    { "from": "2026-11-02", "until": "2026-11-08" },
    { "from": "2026-11-23", "until": null }
  ]
}
```

- `from`, `until`: first and last paused day, inclusive. `until: null`:
  paused until resumed.
- Occurrences on paused days are not due: no reminder, and they do not
  count in `due`. An occurrence with a log counts anyway, also on a paused
  day.
- A pause does not move `endDate`; the user can change it
  ([3.20](#320-change-a-routine)).
- At most 100 pauses per routine.
- Pauses count as the union of their days. After a change of the time
  zone two pauses can overlap; that does no harm. Pause and resume
  (3.21, 3.22) then act on every pause that includes today.

#### Routine status

Computed by the backend for today in the user's time zone:

| `status` | When |
|---|---|
| `ended` | `endDate` is before today |
| `upcoming` | `startDate` is after today |
| `paused` | a pause includes today |
| `active` | none of the above |

The first row that applies wins.

#### Routine progress

Example „Wasser“, 250 ml, hourly 08:00–20:00, on a Wednesday evening:

```json
{
  "progress": {
    "today": { "done": 5, "skipped": 0, "due": 13, "amountDone": 1250, "amountTarget": 3250 },
    "week": { "done": 28, "skipped": 1, "due": 91, "amountDone": 7500, "amountTarget": 22750 },
    "period": null
  }
}
```

The app shows „heute 1.250 von 3.250 ml“. On Tuesday two extra glasses
were logged without an occurrence: they count in `amountDone` of the week
(28 × 250 ml + 500 ml = 7,500 ml), but not in `done`.

| Field | Note |
|---|---|
| `today` | Today in the user's time zone |
| `week` | The current week, Monday to Sunday |
| `period` | From `startDate` to `endDate`; `null` if the routine has no `endDate` |
| `done`, `skipped` | Occurrences of the range with a log `done` or `skipped` |
| `due` | Occurrences of the range, including those later today, without paused ones (unless they have a log). „heute 1 von 2“ is `done` of `due`. |
| `amountDone` | Sum of `amount` of all `done` logs of the range, **also of logs without an occurrence**. A log counts for the day of its occurrence; a log without one (or whose occurrence no longer exists) for the day of its `takenAt`. A `done` log without `amount` counts with the routine's current `amount`. `null` if the routine has no `amount`. |
| `amountTarget` | `amount` × `due`; `null` if the routine has no `amount` |

`amountDone` and `amountTarget` are rounded to 6 decimal places, so
3 × 0.1 gives `0.3` and not `0.30000000000000004`. An `amount` on a log of
a routine without `amount` is stored, but counts nowhere (both sums stay
`null`).
| `day`, `days` | Only in `period`: day of the period (1 = `startDate`; 0 before the start; `days` after the end) and the number of days. „Tag 6 von 14“. |

Logs without an occurrence count for the amount, but not for `done` and
`due`.

Example „Eiweiß“, 30 g, daily at 20:00, logged in portions: the user logs
10 g at noon and 15 g in the afternoon, each without an occurrence
(`dueAt: null`). `today` is then `done` 0 of `due` 1, `amountDone` 25 of
`amountTarget` 30 („heute 25 von 30 g“). The reminder at 20:00 still comes.
If the last portion of 5 g is logged for the occurrence, `today` is `done`
1 of 1 and 30 of 30 g.

**Decision: the backend computes occurrences, status and progress.**

- The backend has to expand the schedule anyway, to check the `dueAt` of a
  log and (stage 2) to assign suggestions to occurrences. If the app expanded it a
  second time (in Dart, with daylight saving time and intervals), sooner or
  later the two would differ: the app would remind of an occurrence that
  the backend refuses with `400`.
- It follows the rest of the contract: the app does not sort or check
  itself (timeline, verification), it shows what the backend returns.
- Statistics and questions about routines (phase C,
  [roadmap](meetings/ROADMAP.md)) need the same numbers on the server.

*Alternative:* the app computes occurrences and progress from the routine
and its logs. For it: it works offline, updates at midnight without a
request, and endpoint 3.24 is not needed. Against it: the rules exist twice
(Go and Dart) and must stay equal, and the backend still needs them for its
checks. Either way the app's mock (`MockApiClient`) needs the expansion,
because it must follow this contract.

#### Logs

```json
{
  "id": "6742d3e4f5a6b7c8d9e0f1a2",
  "routineId": "6742c0a1b2c3d4e5f6a7b8c9",
  "dueAt": "2026-11-23T07:00:00Z",
  "takenAt": "2026-11-23T07:04:12Z",
  "status": "done",
  "amount": null,
  "source": "notification",
  "entryId": null,
  "timeZone": "Europe/Vienna",
  "createdAt": "2026-11-23T07:04:12Z"
}
```

| Field | Type | Note |
|---|---|---|
| `id` | String | ObjectId of the log |
| `routineId` | String | |
| `dueAt` | String or `null` | The occurrence this log is for. `null`: without an occurrence, e.g. an extra glass of water or a portion. |
| `takenAt` | String | When it was done or skipped (RFC 3339, UTC). The server time of the request, unless the user entered a time. |
| `status` | String | `done` or `skipped` |
| `amount` | Number or `null` | Amount actually taken, in the routine's `unit`. For `done` without an amount in the request: the routine's `amount`. Always `null` for `skipped`. |
| `source` | String | `manual` (in the app), `notification` (from a reminder: a button, or the screen it opened), `entry` (stage 2: a confirmed suggestion, [3.28](#328-confirm-a-routine-suggestion-proposal-stage-2)). Not the `sources` of an answer, not the `source` of the verification. |
| `entryId` | String or `null` | The entry for `source: entry`, otherwise `null` |
| `timeZone` | String | The user's time zone when the log was created |
| `createdAt` | String | Server time of the request |

Rules:

1. At most one log per occurrence. A second one is refused with
   `409 ROUTINE_LOG_EXISTS` and `details.logId`. So a reminder button sent
   twice over a bad network cannot create two logs. A unique index on the
   occurrence's local date and time secures this.
2. A log belongs to the occurrence whose local date and time equal those
   of its `dueAt` in its `timeZone`. Usually that is exactly `dueAt`. If
   the user's time zone changes, the occurrence gets a new `dueAt`, but the
   log stays with it, unchanged.
3. If the schedule changes ([3.20](#320-change-a-routine)) and a
   log's occurrence no longer exists, the log stays in the history; it then
   counts only for `amountDone`, like a log without an occurrence.
4. Logs are not changed. To correct one, delete it
   ([3.27](#327-delete-a-log)) and add a new one.

#### Limits and validation

| What | Rule | Error |
|---|---|---|
| Routines per user | at most 50, over all modules and statuses | `409 ROUTINE_LIMIT_REACHED`, `details.limit`: `routines` |
| `name` | whitespace at the edges removed; 1–60 characters without control characters; unique per user and module, ignoring upper and lower case (the name in lower case, `ß` and `ss` stay different) | `400 INVALID_REQUEST`; `409 ROUTINE_NAME_TAKEN` |
| `amount` | number greater than 0 and at most 100,000, or `null` | `400 INVALID_REQUEST` |
| `unit` | whitespace at the edges removed; 1–20 characters without control characters; only with an `amount`; empty counts as `null` | `400 INVALID_REQUEST` |
| `interval` | required; whole number (`2.0` is fine, `1.5` is not); `hourly` 1–23, `daily` 1–365, `weekly` 1–52 | `400 INVALID_REQUEST` |
| Times | `HH:MM` from `00:00` to `23:59`; `windowStart` < `windowEnd`; `times` 1–24 different values | `400 INVALID_REQUEST` |
| `weekdays` | 1–7 different values | `400 INVALID_REQUEST` |
| Schedule fields | only those of the `frequency`; a field of another frequency is an error, also with `null` | `400 INVALID_REQUEST` |
| `startDate`, `endDate`, `until` (3.21) | valid calendar dates `YYYY-MM-DD` from 2000-01-01 to 2099-12-31; `endDate` ≥ `startDate` | `400 INVALID_REQUEST` |
| Pauses per routine | at most 100 | `409 ROUTINE_LIMIT_REACHED`, `details.limit`: `pauses` |
| Logs per routine and day | at most 100, local day of `takenAt` | `409 ROUTINE_LIMIT_REACHED`, `details.limit`: `logs` |
| Body | a JSON object of at most 16 KB | `400 INVALID_REQUEST` |
| Range of occurrences (3.24) | at most 31 days | `400 INVALID_PARAMETER` |
| Range of logs (3.25) | at most 366 days | `400 INVALID_PARAMETER` |

The limits protect the database and the phone (iOS keeps at most 64
planned notifications per app). The dates are limited to 2000–2099 so the
progress of a period stays cheap to compute. There is no daily limit for
routines (decided 2026-11-23), because no routine endpoint calls a model;
suggestions come with the upload, which already counts.

Names and "one log per occurrence" are secured by unique indexes, also for
two requests at the same moment. The limits of 50 routines and 100 logs a
day are counted before writing; two requests at the same moment can
exceed them by one.

IDs of routines and logs are ObjectIds like entry IDs. An invalid one is
`400 INVALID_ID` before anything else is checked: „Diese Routinen-ID ist
ungültig.“ or „Diese Verlaufseintrag-ID ist ungültig.“ For every endpoint
with a routine in the path, the routine of the user is checked next
(`404 ROUTINE_NOT_FOUND`), then the body and the parameters; a routine of
another user is `404` whatever the request contains. A body over 16 KB or
an interrupted one: `400 INVALID_REQUEST`, „Die Anfrage ist zu groß oder
unvollständig.“ Lists are always an array, here too: `routines`,
`pauses`, `times`, `weekdays`, `occurrences`, `logs`,
`routineSuggestions`.

#### Per-user data and logging

- Every routine and every log belongs to one user (`userId` in the
  collections `routines` and `routineLogs`). Every endpoint only sees the
  user's own. A routine or log of another user answers
  `404 ROUTINE_NOT_FOUND` or `404 ROUTINE_LOG_NOT_FOUND`, exactly like one
  that does not exist. Admins have no additional rights.
- The model only gets the user's own active routines, and of them only
  the short key, name, amount and unit.
- Routines and logs are health data. The backend never logs a name, an
  amount, a unit, a quote or the time of a log, only method, path, status,
  durations and counts.
- Deleting an account (`cmd/users delete`) also deletes its routines and
  logs.
- Backend: every new route goes into `routes()` and is classified in
  `TestSeparation_EveryRoutePassesTheUser`
  (`backend/internal/api/separation_test.go`): user B calls it on user A's
  routine and gets `404`, and every store call carries B's user ID.

#### Reminders in the app

The app plans reminders as local notifications on the phone, e.g. with
`flutter_local_notifications`. The server sends no push messages.

1. The app loads `GET /routines/occurrences` for today and the next 6 days
   and plans a notification for every occurrence with `status: open` and
   `dueAt` in the future. It never reminds of `done`, `skipped` or
   `paused`.
2. It plans again at start, when it comes back to the foreground, after
   every log, pause, resume or change of a routine, and after a change of
   the time zone.
3. **After the phone restarts**, Android has forgotten all planned
   notifications. The app plans them again: with the permission
   `RECEIVE_BOOT_COMPLETED` and the boot receiver of the notification
   package from the last planned list, and in any case at the next app
   start from `GET /routines/occurrences`.
4. iOS keeps at most 64 planned notifications per app, so the app plans
   the nearest ones first. Android 13 and later needs the permission
   `POST_NOTIFICATIONS`. Battery savers (e.g. on the OnePlus test phone)
   can delay notifications.
5. **Approximate reminders are enough** (a deliberate decision, open
   question in [section 4](#open-questions-on-routines-2026-11-23)).
   Since Android 14, `SCHEDULE_EXACT_ALARM` is no longer granted by
   default, and `USE_EXACT_ALARM` is only allowed for alarm clock and
   calendar apps. The app therefore plans inexact notifications
   (`AndroidScheduleMode.inexactAllowWhileIdle`). They usually come a few
   minutes late, the later the further ahead they were planned: in the
   worst case about an hour, and in Doze (phone idle on battery) more. On
   the test phone a reminder planned 4 to 5 minutes ahead came 3 to 4
   minutes late. For a routine that is fine; the occurrence keeps its
   `dueAt`.
6. A notification has three buttons:

   | Button | Request |
   |---|---|
   | „Ja“ | `POST /routines/{id}/logs` with the occurrence's `dueAt`, `status: done`, `source: notification`, without `amount` (= the target amount) |
   | „Nein“ | the same with `status: skipped` |
   | „Andere Menge“ | opens the app at the occurrence; the user enters the amount, then the same request as „Ja“ with `amount` |

   `409 ROUTINE_LOG_EXISTS` means the occurrence is already done, e.g. on
   another device. The app treats it as success, not as an error, and
   removes the reminder. There is no button „Schon erledigt“. If the request
   fails otherwise (e.g. no network), the occurrence stays open and the app
   offers it again when it is opened.
7. **Lock screen:** a routine's name is health data. Android:
   `visibility: private`. The notification package
   (flutter_local_notifications 23) has no `publicVersion`, so there is no
   neutral text of our own: if the phone is set to hide sensitive content
   on the lock screen, Android shows the app's name and a placeholder of
   the system instead of the content; otherwise the name is visible there.
   The consent notice (version 2026-11-26) says that a reminder can show a
   routine's name on the phone. iOS shows previews on the lock screen
   depending on the user's setting, so there the name is in neither title
   nor text („Zeit für deine Routine · 08:00“).

### 3.17 List routines

```http
GET /api/v1/routines?module=health
```

**Request**

| Parameter | Values | Default |
|---|---|---|
| `module` | see [modules](#modules) | `health` |

All of the user's routines of the module, in every status, sorted by
`createdAt` ascending, then `id`. The app groups them by `status`.

**Response `200 OK`**

```json
{
  "routines": [
    {
      "id": "6742bf10a1b2c3d4e5f6a7b8",
      "module": "health",
      "name": "Vitamin D",
      "amount": 2,
      "unit": "Stück",
      "schedule": { "frequency": "daily", "interval": 1, "times": ["08:00"] },
      "startDate": "2026-11-18",
      "endDate": null,
      "status": "active",
      "pauses": [],
      "createdAt": "2026-11-18T06:55:40Z",
      "progress": {
        "today": { "done": 1, "skipped": 0, "due": 1, "amountDone": 2, "amountTarget": 2 },
        "week": { "done": 1, "skipped": 0, "due": 7, "amountDone": 2, "amountTarget": 14 },
        "period": null
      }
    }
  ],
  "count": 1
}
```

No routines: `"routines": []` and `"count": 0`. No pagination; at most 50
routines exist.

**Errors**

| HTTP | Code | Note |
|---|---|---|
| 400 | `INVALID_PARAMETER` | Unknown `module` (`details.parameter`, `details.value`) |
| 500 | `INTERNAL_ERROR` | |

### 3.18 Create a routine

```http
POST /api/v1/routines
Content-Type: application/json
```

**Request**

| Field | Type | Note |
|---|---|---|
| `module` | String | Optional, default `health` |
| `name` | String | Required |
| `amount` | Number or `null` | Optional |
| `unit` | String or `null` | Optional, only with `amount` |
| `schedule` | Object | Required, see [schedule](#schedule) |
| `startDate` | String | Required, `YYYY-MM-DD`; may be in the past |
| `endDate` | String or `null` | Optional |

Unknown fields are an error. The rules are in
[limits and validation](#limits-and-validation).

```json
{
  "name": "Wasser",
  "amount": 250,
  "unit": "ml",
  "schedule": { "frequency": "hourly", "interval": 1, "windowStart": "08:00", "windowEnd": "20:00" },
  "startDate": "2026-11-23"
}
```

**Processing in the backend**

1. Check the body. If any part is invalid, nothing is stored.
2. Check the number of the user's routines (50) and whether the name is
   taken in the module (unique index on user, module and the name in lower
   case).
3. Store the routine with `userId`, `createdAt` and `pauses: []`.

**Response `201 Created`**

```http
Location: /api/v1/routines/6742c5d7e8f9a0b1c2d3e4f5
```

The full routine as in [3.16](#the-routine). For the example, created on
Monday 2026-11-23 at 09:15:

```json
{
  "id": "6742c5d7e8f9a0b1c2d3e4f5",
  "module": "health",
  "name": "Wasser",
  "amount": 250,
  "unit": "ml",
  "schedule": { "frequency": "hourly", "interval": 1, "windowStart": "08:00", "windowEnd": "20:00" },
  "startDate": "2026-11-23",
  "endDate": null,
  "status": "active",
  "pauses": [],
  "createdAt": "2026-11-23T08:15:03Z",
  "progress": {
    "today": { "done": 0, "skipped": 0, "due": 13, "amountDone": 0, "amountTarget": 3250 },
    "week": { "done": 0, "skipped": 0, "due": 91, "amountDone": 0, "amountTarget": 22750 },
    "period": null
  }
}
```

**Errors**

| HTTP | Code | Note | `message` |
|---|---|---|---|
| 400 | `INVALID_REQUEST` | Body not a JSON object, unknown field, required field missing, wrong type, a rule broken, unknown `module`. `details.field` names the field (e.g. `schedule.times`), `details.value` the rejected value. | see below |
| 409 | `ROUTINE_NAME_TAKEN` | `details.routineId` names the existing routine | Eine Routine mit diesem Namen gibt es schon. |
| 409 | `ROUTINE_LIMIT_REACHED` | `details.limit`: `routines`, `details.max`: `50` | Du hast schon 50 Routinen. Lösche eine, bevor du eine neue anlegst. |
| 500 | `INTERNAL_ERROR` | | |

`message` for `INVALID_REQUEST`, by field:

| `details.field` | `message` |
|---|---|
| none (body not a JSON object) | Bitte die Routine als JSON-Objekt schicken. |
| the unknown field, e.g. `color` or `schedule.every` | Unbekanntes Feld: color. |
| `module` | Unbekanntes Modul. Im Prototyp gibt es nur health. (`null` counts as missing: `health`) |
| `name` | Der Name muss 1 bis 60 Zeichen lang sein. (also missing, `null` or not a string) |
| `amount` | Die Menge muss größer als 0 und höchstens 100.000 sein. (also not a number) |
| `unit` | Eine Einheit braucht eine Menge. / Die Einheit darf höchstens 20 Zeichen lang sein. (also with control characters) / Die Einheit muss ein Text sein. |
| `schedule` | Bitte einen Rhythmus angeben. (missing, `null` or not an object) |
| `schedule.frequency` | Unbekannter Rhythmus. |
| a field of another frequency, e.g. `schedule.times` with `hourly` | Dieses Feld passt nicht zum gewählten Rhythmus. |
| `schedule.interval` | Das Intervall muss zwischen 1 und 23 liegen. (the maximum of the `frequency`; also missing) |
| `schedule.times`, `schedule.windowStart`, `schedule.windowEnd` | Uhrzeiten bitte im Format HH:MM angeben, z. B. 08:00. / Höchstens 24 Uhrzeiten pro Tag. / Jede Uhrzeit nur einmal. |
| `schedule.times` | Bitte mindestens eine Uhrzeit angeben. (missing, `null` or `[]`) |
| `schedule.windowEnd` | Das Ende des Zeitfensters muss nach dem Beginn liegen. |
| `schedule.weekdays` | Bitte mindestens einen Wochentag wählen. (also not a list) / Unbekannter Wochentag. / Jeden Wochentag nur einmal. |
| `startDate`, `endDate` | Das Datum muss im Format JJJJ-MM-TT angegeben werden. / Das Datum muss zwischen 2000 und 2099 liegen. / Das Enddatum darf nicht vor dem Startdatum liegen. |

`details.value` is the rejected value as sent (a single element for a
list, e.g. `"8 Uhr"`); it is missing for a missing field, `null`, an
unknown field and a count (more than 24 times). If a request has several
errors, the first in this order is reported: unknown field
(alphabetical), `module`, `name`, `amount`, `unit`, `schedule` (object,
unknown field, `frequency`, fields of another frequency, `interval`, then
`windowStart`, `windowEnd`, the window's order, or `weekdays`, `times`;
more than 24 times before their format), `startDate`, `endDate`, `endDate`
before `startDate`.

```json
{
  "error": {
    "code": "INVALID_REQUEST",
    "message": "Uhrzeiten bitte im Format HH:MM angeben, z. B. 08:00.",
    "details": { "field": "schedule.times", "value": "8 Uhr" }
  }
}
```

### 3.19 Get a routine

```http
GET /api/v1/routines/6742c0a1b2c3d4e5f6a7b8c9
```

No body. **Response `200 OK`:** the full routine as in
[3.16](#the-routine).

**Errors**

| HTTP | Code | Note |
|---|---|---|
| 400 | `INVALID_ID` | `{id}` is not a valid ObjectId |
| 404 | `ROUTINE_NOT_FOUND` | No routine with this ID, or it belongs to another user |
| 500 | `INTERNAL_ERROR` | |

### 3.20 Change a routine

```http
PATCH /api/v1/routines/6742c0a1b2c3d4e5f6a7b8c9
Content-Type: application/json
```

**Request**

The body contains only the fields that should change. A missing field
stays unchanged; `null` clears it.

| Field | Type | Note |
|---|---|---|
| `name` | String | `null` is not allowed |
| `amount` | Number or `null` | `null` removes the amount, and with it the `unit` |
| `unit` | String or `null` | `null` removes the unit |
| `schedule` | Object | Replaced as a whole, never merged; `null` is not allowed |
| `startDate` | String | `null` is not allowed |
| `endDate` | String or `null` | `null` removes the end |

`id`, `module`, `status`, `pauses`, `createdAt`, `progress` and unknown
fields are an error (`400 INVALID_REQUEST`, `details.field`), as for
`PATCH /me`. Pausing has endpoints of its own (3.21, 3.22). An empty body
`{}` is an error. A request whose values already apply is valid: `200`
with the unchanged routine. After the change the whole routine must follow
the rules of [3.16](#limits-and-validation).

```json
{
  "schedule": { "frequency": "daily", "interval": 1, "times": ["07:30", "20:00"] },
  "endDate": "2026-12-07"
}
```

**Processing in the backend**

1. Check the request. If any part is invalid, nothing changes.
2. Store the fields whose value changes, and only those (`$set`); a
   request that only repeats values writes nothing. If two requests
   change the same field at the same moment, the last one wins (as for
   corrections). The schedule applies to the whole plan, also to past
   days; there are no plan versions in the prototype (open question).
3. Logs are not changed. A log whose occurrence no longer exists counts
   only for `amountDone` ([logs](#logs), rule 3). Pending suggestions are
   not changed either.

**Response `200 OK`:** the updated routine. The app plans its reminders
again.

**Errors**

| HTTP | Code | Note |
|---|---|---|
| 400 | `INVALID_ID` | |
| 400 | `INVALID_REQUEST` | As in 3.18; also a field that cannot be changed, `null` where it is not allowed, an empty body |
| 404 | `ROUTINE_NOT_FOUND` | Also for a routine of another user; nothing is changed |
| 409 | `ROUTINE_NAME_TAKEN` | The new name is taken; `details.routineId` |
| 500 | `INTERNAL_ERROR` | |

`message` for `INVALID_REQUEST`, in addition to 3.18 (`null` for `name`,
`schedule` or `startDate` gives the message of that field):

| Case | `details.field` | `message` |
|---|---|---|
| Body not a JSON object | – | Bitte die Änderung als JSON-Objekt schicken. |
| Empty body `{}` | – | Bitte mindestens ein Feld zum Ändern schicken. |
| A field that cannot be changed | the field, e.g. `status` | Das Feld status kann nicht geändert werden. |
| New `startDate` after the stored `endDate` | `startDate` | Das Startdatum darf nicht nach dem Enddatum liegen. |
| New `endDate` before the (new or stored) `startDate` | `endDate` | Das Enddatum darf nicht vor dem Startdatum liegen. |
| A `unit` while the `amount` is or becomes `null` | `unit` | Eine Einheit braucht eine Menge. |

Order: invalid ID, routine of the user (otherwise `404`, also with an
invalid body), body not an object, empty body, field names
(alphabetical), the values in the order of 3.18, then the rules over the
changed routine.

### 3.21 Pause a routine

```http
POST /api/v1/routines/6742c0a1b2c3d4e5f6a7b8c9/pause
Content-Type: application/json
```

**Request**

| Field | Type | Note |
|---|---|---|
| `until` | String or `null` | Last paused day, inclusive, `YYYY-MM-DD`, not before today, at most 2099-12-31. `null` or missing: paused until resumed. |

A request without a body counts as `{}`: paused until resumed.

```json
{
  "until": "2026-11-30"
}
```

**Processing in the backend** ("today" in the user's time zone)

1. If no pause includes today, append `{ "from": today, "until": until }`.
2. If a pause includes today, only its `until` changes (longer or
   shorter); `from` stays. If several do (overlapping after a change of
   the time zone), all of them.
3. If nothing changes (the same request sent twice), nothing is written.
   The backend reads, changes and writes the pauses with a condition on
   the old ones; if another request changed them at the same moment, it
   reads again (at most 3 times).

Also allowed for an `upcoming` or `ended` routine; the pause then only
matters on days within the routine's period.

**Response `200 OK`:** the full routine, for a running routine with
`status: paused`. The app removes the reminders of the paused days.

**Why an endpoint of its own and not a field for `PATCH`?**

- Pausing is an action relative to today (server clock, user's time zone)
  that adds to a history; it is not a value of the plan. The history is
  needed so that paused days count neither in the progress nor as missed
  in the occurrences, also after resuming.
- `PATCH` stays "change the plan", and its rule "missing stays, `null`
  clears" stays unambiguous. A field `pausedUntil` would need `null` for
  "resume" and another value for "paused without an end".
- The buttons „Pausieren“ and „Fortsetzen“ map to one request each.
  Sending one twice changes nothing more.

*Alternative:* a field `pausedUntil` for `PATCH`: one endpoint fewer, but
the history would have to be derived, and "without an end" needs a value
of its own.

**Errors**

| HTTP | Code | Note | `message` |
|---|---|---|---|
| 400 | `INVALID_ID` | | |
| 400 | `INVALID_REQUEST` | `until` before today (`details.field`: `until`, `details.value`) | Das Ende der Pause darf nicht vor heute liegen. |
| 400 | `INVALID_REQUEST` | `until` not a valid date / not from 2000 to 2099 (`details.field`: `until`) | Das Datum muss im Format JJJJ-MM-TT angegeben werden. / Das Datum muss zwischen 2000 und 2099 liegen. |
| 400 | `INVALID_REQUEST` | Unknown field; body not a JSON object | Unbekanntes Feld: from. / Bitte die Pause als JSON-Objekt schicken. |
| 404 | `ROUTINE_NOT_FOUND` | Checked before the body | Diese Routine gibt es nicht. |
| 409 | `ROUTINE_LIMIT_REACHED` | `details.limit`: `pauses`, `details.max`: `100` | Diese Routine wurde schon zu oft pausiert. |
| 500 | `INTERNAL_ERROR` | | |

### 3.22 Resume a routine

```http
POST /api/v1/routines/6742c0a1b2c3d4e5f6a7b8c9/resume
```

No body; a body is ignored. Like pausing, with a condition on the old
pauses and at most 3 attempts.

**Processing in the backend**

1. If a pause includes today: if it began today, it is removed; otherwise
   its `until` becomes yesterday. Today's occurrences are due again, also
   the earlier ones of today without a log.
2. If no pause includes today, nothing changes.

**Response `200 OK`:** the full routine. The app plans its reminders
again.

**Errors**

| HTTP | Code | Note |
|---|---|---|
| 400 | `INVALID_ID` | |
| 404 | `ROUTINE_NOT_FOUND` | |
| 500 | `INTERNAL_ERROR` | |

### 3.23 Delete a routine

```http
DELETE /api/v1/routines/6742c0a1b2c3d4e5f6a7b8c9
```

No body.

**Processing in the backend**

1. (Stage 2) Remove the routine's pending suggestions from the user's
   entries.
2. Delete the routine's logs, including those confirmed from entries
   (stage 2); the entries themselves stay.
3. Delete the routine.
4. Delete the routine's logs once more: a log added while steps 2 and 3
   ran would otherwise stay without its routine.

The routine comes **last** (before the second pass over the logs). If a step fails, the backend answers
`500 INTERNAL_ERROR` and the routine stays visible; calling again cleans up
the rest. If the app drops the connection after deleting has started, it
still runs to completion.

**Response `204 No Content`.** The app cancels the routine's reminders.

**Errors**

| HTTP | Code | Note |
|---|---|---|
| 400 | `INVALID_ID` | |
| 404 | `ROUTINE_NOT_FOUND` | Also when deleting the same routine a second time, and for a routine of another user (nothing is deleted) |
| 500 | `INTERNAL_ERROR` | |

### 3.24 List occurrences

The basis for the reminders and for the view of a day.

```http
GET /api/v1/routines/occurrences?from=2026-11-23&to=2026-11-29&module=health
```

**Request**

| Parameter | Values | Default |
|---|---|---|
| `from` | `YYYY-MM-DD` | today (user's time zone) |
| `to` | `YYYY-MM-DD`, not before `from`, at most 31 days including both | `from` + 6 days |
| `module` | see [modules](#modules) | `health` |

The list holds the occurrences of all the user's routines of the module on
these days, in every status, sorted by `dueAt` ascending, then
`routineId`. Logs without an occurrence are not in it (see 3.25).

**Response `200 OK`** (for `from` = `to` = 2026-11-23)

```json
{
  "from": "2026-11-23",
  "to": "2026-11-23",
  "occurrences": [
    {
      "routineId": "6742bf10a1b2c3d4e5f6a7b8",
      "dueAt": "2026-11-23T07:00:00Z",
      "date": "2026-11-23",
      "time": "08:00",
      "status": "done",
      "logId": "6742d2b1c3d4e5f6a7b8c9d0"
    },
    {
      "routineId": "6742c0a1b2c3d4e5f6a7b8c9",
      "dueAt": "2026-11-23T07:00:00Z",
      "date": "2026-11-23",
      "time": "08:00",
      "status": "done",
      "logId": "6742d3e4f5a6b7c8d9e0f1a2"
    },
    {
      "routineId": "6742c0a1b2c3d4e5f6a7b8c9",
      "dueAt": "2026-11-23T19:00:00Z",
      "date": "2026-11-23",
      "time": "20:00",
      "status": "open",
      "logId": null
    }
  ],
  "count": 3
}
```

**Processing in the backend**

1. Load the user's routines of the module and their logs on these days.
2. Expand each schedule in the user's time zone ([3.16](#schedule)),
   connect the logs ([logs](#logs), rule 2) and mark paused days.

The path is fixed; Go's `ServeMux` gives `/routines/occurrences` precedence
over `/routines/{id}` (`occurrences` is never a valid ObjectId).

**Errors**

| HTTP | Code | Note | `message` |
|---|---|---|---|
| 400 | `INVALID_PARAMETER` | Unknown `module` (checked first) | Unbekanntes Modul. Im Prototyp gibt es nur health. |
| 400 | `INVALID_PARAMETER` | `from` or `to` not a valid date | Das Datum muss im Format JJJJ-MM-TT angegeben werden. |
| 400 | `INVALID_PARAMETER` | `to` before `from` | Das Ende des Zeitraums darf nicht vor dem Anfang liegen. |
| 400 | `INVALID_PARAMETER` | More than 31 days | Der Zeitraum darf höchstens 31 Tage umfassen. |
| 500 | `INTERNAL_ERROR` | | |

`details.parameter` and `details.value` name the parameter as sent. For a
wrong range that is `to`, or `from` if `to` was missing (then `to` is
its default).

### 3.25 List the logs of a routine

```http
GET /api/v1/routines/6742c0a1b2c3d4e5f6a7b8c9/logs?from=2026-10-25&to=2026-11-23
```

**Request**

| Parameter | Values | Default |
|---|---|---|
| `from` | `YYYY-MM-DD` | `to` − 29 days |
| `to` | `YYYY-MM-DD`, not before `from`, at most 366 days including both | today |

The day of a log is the day of its `takenAt` in the user's time zone. The
list is sorted by `takenAt` descending, then `id` in the same order.

**Response `200 OK`**

```json
{
  "logs": [
    {
      "id": "6742d3e4f5a6b7c8d9e0f1a2",
      "routineId": "6742c0a1b2c3d4e5f6a7b8c9",
      "dueAt": "2026-11-23T07:00:00Z",
      "takenAt": "2026-11-23T07:04:12Z",
      "status": "done",
      "amount": null,
      "source": "notification",
      "entryId": null,
      "timeZone": "Europe/Vienna",
      "createdAt": "2026-11-23T07:04:12Z"
    }
  ],
  "count": 1
}
```

No logs: `"logs": []` and `"count": 0`.

**Errors**

| HTTP | Code | Note |
|---|---|---|
| 400 | `INVALID_ID` | |
| 400 | `INVALID_PARAMETER` | `from` or `to` not a valid date, `to` before `from`, more than 366 days; messages and `details` as in 3.24 (checked after the routine: an unknown routine is `404` first) |
| 404 | `ROUTINE_NOT_FOUND` | |
| 500 | `INTERNAL_ERROR` | |

### 3.26 Add a log

```http
POST /api/v1/routines/6742c0a1b2c3d4e5f6a7b8c9/logs
Content-Type: application/json
```

**Request**

| Field | Type | Note |
|---|---|---|
| `status` | String | Required: `done` or `skipped` |
| `source` | String | Required: `manual` or `notification`. `entry` only arises through 3.28. |
| `dueAt` | String or `null` | The occurrence, as `dueAt` from 3.24. Missing or `null`: without an occurrence. |
| `takenAt` | String or `null` | RFC 3339; stored as UTC, to the millisecond (sent back in seconds like every point in time). Missing or `null`: the server time. The app only sends it if the user entered a time (logging afterwards), never the phone's clock as "now". |
| `amount` | Number or `null` | Only for `done`; missing or `null`: the routine's `amount`. Must be missing or `null` for `skipped`. Also allowed for a routine without `amount`: stored, but counts nowhere. |

```json
{
  "dueAt": "2026-11-23T07:00:00Z",
  "status": "done",
  "source": "notification"
}
```

**Processing in the backend**

1. Check the body:
   - `dueAt` must be an occurrence of the routine, and its day must not be
     after today. Occurrences on paused days may be logged.
   - `takenAt` must not be later than the server time plus 5 minutes. The
     5 minutes allow for a time picker that starts from the phone's clock.
   - `amount` must follow the rules of 3.16.
2. If the occurrence already has a log: `409 ROUTINE_LOG_EXISTS`.
3. At most 100 logs per routine and local day of `takenAt`.
4. Store the log with `userId`, the user's `timeZone` and `createdAt`.
   The unique index on the occurrence also catches two requests at the
   same moment (then `409` as in step 2).

If a request has several errors, the first in this order is reported:
invalid ID, routine of the user (`404`), body not an object, unknown field
(alphabetical), `status`, `source`, format of `dueAt`, format of
`takenAt`, type of `amount`, `dueAt` is an occurrence, its day not after
today, `takenAt` not in the future, `amount` with `skipped`, range of
`amount`, `409`, limit per day.

Logging works in every status of the routine (also paused, upcoming or
ended), e.g. to add a log afterwards.

**Response `201 Created`:** the log as in [3.16](#logs). There is no `GET`
for a single log and therefore no `Location` header.

**Errors**

| HTTP | Code | Note | `message` |
|---|---|---|---|
| 400 | `INVALID_ID` | | |
| 400 | `INVALID_REQUEST` | `dueAt` is no occurrence of the routine (`details.field`: `dueAt`) | Zu diesem Zeitpunkt ist kein Termin der Routine geplant. |
| 400 | `INVALID_REQUEST` | `dueAt` after today | Termine in der Zukunft können noch nicht eingetragen werden. |
| 400 | `INVALID_REQUEST` | `takenAt` in the future | Der Zeitpunkt darf nicht in der Zukunft liegen. |
| 400 | `INVALID_REQUEST` | `amount` with `skipped` | Ausgelassene Termine haben keine Menge. |
| 400 | `INVALID_REQUEST` | `status` missing or not `done`/`skipped` (`details.field`: `status`) | Unbekannter Status. |
| 400 | `INVALID_REQUEST` | `source` missing or not `manual`/`notification`, also `entry` (`details.field`: `source`) | Unbekannte Herkunft. |
| 400 | `INVALID_REQUEST` | `dueAt` or `takenAt` not an RFC 3339 point in time | Bitte einen gültigen Zeitpunkt angeben. |
| 400 | `INVALID_REQUEST` | `amount` not a number, not greater than 0 or above 100,000 | Die Menge muss größer als 0 und höchstens 100.000 sein. |
| 400 | `INVALID_REQUEST` | Unknown field (e.g. `entryId`); body not a JSON object | Unbekanntes Feld: entryId. / Bitte den Verlaufseintrag als JSON-Objekt schicken. |
| 404 | `ROUTINE_NOT_FOUND` | | Diese Routine gibt es nicht. |
| 409 | `ROUTINE_LOG_EXISTS` | The occurrence already has a log; `details.logId` names it. The app treats it as "already done" ([reminders](#reminders-in-the-app)). | Dieser Termin ist schon eingetragen. |
| 409 | `ROUTINE_LIMIT_REACHED` | `details.limit`: `logs`, `details.max`: `100` | Für heute sind bei dieser Routine keine weiteren Einträge möglich. |
| 500 | `INTERNAL_ERROR` | | |

```json
{
  "error": {
    "code": "ROUTINE_LOG_EXISTS",
    "message": "Dieser Termin ist schon eingetragen.",
    "details": { "logId": "6742d3e4f5a6b7c8d9e0f1a2" }
  }
}
```

### 3.27 Delete a log

```http
DELETE /api/v1/routines/6742c0a1b2c3d4e5f6a7b8c9/logs/6742d3e4f5a6b7c8d9e0f1a2
```

No body. Afterwards the occurrence is open again; the app plans its
reminder again if it is still ahead. For a log from a suggestion, the
suggestion does not come back.

**Response `204 No Content`.**

**Errors**

| HTTP | Code | Note |
|---|---|---|
| 400 | `INVALID_ID` | `{id}` or `{logId}` is not a valid ObjectId |
| 404 | `ROUTINE_NOT_FOUND` | |
| 404 | `ROUTINE_LOG_NOT_FOUND` | No log with this ID in this routine; also when deleting it a second time |
| 500 | `INTERNAL_ERROR` | |

### 3.28 Confirm a routine suggestion (proposal, stage 2)

In the review after an upload, or later in the entry, the user confirms a
[routine suggestion](#routine-suggestions-proposal-stage-2). Only then does it
count.

```http
POST /api/v1/entries/6727a1f3c2b4e5d6f7a8b9c0/routine-suggestions/s1/confirm
Content-Type: application/json
```

**Request**

The body is optional. Without a body (or with `{}`) the suggestion is
taken as it is.

| Field | Type | Note |
|---|---|---|
| `amount` | Number or `null` | Missing: the suggestion's `amount`. `null` (or the suggestion has none): the routine's `amount`. |
| `takenAt` | String | Missing: the suggestion's `takenAt`. The app sends it if the user changed the time; for `timeStated: false` it always asks. |
| `dueAt` | String or `null` | Missing: the backend assigns the occurrence (below). `null`: without an occurrence. A value: rules as in 3.26. |

```json
{
  "takenAt": "2026-11-22T19:30:00Z"
}
```

**Assignment to an occurrence:** the occurrence of the routine on the same
day as `takenAt` (user's time zone) that has no log yet and is nearest to
`takenAt`, also on a paused day. On a tie, the earlier one. If there is
none, the log has no occurrence (`dueAt: null`).

**Processing in the backend**

1. The entry must belong to the user (`404 ENTRY_NOT_FOUND`), the
   suggestion must be on it (`404 ROUTINE_SUGGESTION_NOT_FOUND`).
2. Check the body as in 3.26 (`takenAt`, `amount`, an explicit `dueAt`).
3. Create the log with `status: done`, `source: entry`, `entryId`, then
   remove the suggestion from the entry. If removing fails, the backend
   deletes the log again and answers `500`; nothing is half done.

**Response `201 Created`:** the log as in [3.16](#logs). The app removes
the suggestion from the entry it shows (or loads `GET /entries/{id}`
again) and plans its reminders again.

```json
{
  "id": "6742e0a1b2c3d4e5f6a7b8c9",
  "routineId": "6742c0a1b2c3d4e5f6a7b8c9",
  "dueAt": "2026-11-22T19:00:00Z",
  "takenAt": "2026-11-22T19:30:00Z",
  "status": "done",
  "amount": null,
  "source": "entry",
  "entryId": "6727a1f3c2b4e5d6f7a8b9c0",
  "timeZone": "Europe/Vienna",
  "createdAt": "2026-11-23T17:44:51Z"
}
```

**Errors**

| HTTP | Code | Note |
|---|---|---|
| 400 | `INVALID_ID` | `{id}` is not a valid ObjectId |
| 400 | `INVALID_REQUEST` | As in 3.26 |
| 404 | `ENTRY_NOT_FOUND` | No entry with this ID, or it belongs to another user |
| 404 | `ROUTINE_SUGGESTION_NOT_FOUND` | No such suggestion on the entry, also when it was already confirmed or rejected; `details.suggestionId` |
| 409 | `ROUTINE_LOG_EXISTS` | Only with an explicit `dueAt` that already has a log; `details.logId` |
| 409 | `ROUTINE_LIMIT_REACHED` | `details.limit`: `logs` |
| 500 | `INTERNAL_ERROR` | |

### 3.29 Reject a routine suggestion (proposal, stage 2)

```http
DELETE /api/v1/entries/6727a1f3c2b4e5d6f7a8b9c0/routine-suggestions/s1
```

No body. Removes the suggestion from the entry; nothing else changes.

**Response `204 No Content`.**

**Errors**

| HTTP | Code | Note |
|---|---|---|
| 400 | `INVALID_ID` | `{id}` is not a valid ObjectId |
| 404 | `ENTRY_NOT_FOUND` | |
| 404 | `ROUTINE_SUGGESTION_NOT_FOUND` | Also when rejecting it a second time; `details.suggestionId` |
| 500 | `INTERNAL_ERROR` | |

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

- **Login and multiple users:** done on 2026-11-18: login and tokens
  ([login and tokens](#login-and-tokens), 3.10–3.14), per-user data
  ([per-user data](#per-user-data)), daily limits
  ([daily limits](#daily-limits)) and the consent ([3.15](#315-record-the-consent)).
  Still open: limits per month, and a way for users to see how much of
  their daily limit is left.
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
  lost. Each entry belongs to one user and is corrected in one dialog at a
  time, so this is hardly reachable in the prototype. Later: a version counter in the entry, and
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
  (2026-11-12): every value read, but `mg/dl` read as `mgj/dl`, the same
  confusion as on the synthetic photos.
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

### Further points on transcription

- **Mark spoken entries:** an entry made from a transcription looks like
  any text import (`file.mimeType` `text/plain`). If the timeline should
  show a microphone, the text import could accept an optional field
  (e.g. `origin=speech`).
- **Words with timestamps:** Whisper knows when each word was spoken. If
  answers should later jump to a place in a recording, the recording
  would have to be stored after all; not planned.
- **Longer recordings:** over 5 minutes would need an asynchronous call
  (start, then poll), because a synchronous request would take too long.
- **Speed on Windows:** without a graphics card large-v3-turbo is much
  slower; the quantized model `ggml-large-v3-turbo-q5_0.bin` is an option
  (backend/README.md), not measured yet.

### Open questions on routines (2026-11-23)

Collected with the proposal
([3.16](#316-routines-overview-and-data-model)–3.29). Stage 1 is
implemented since 2026-11-23; the questions below stay open unless marked
as decided.

1. **Categories (phase B):** Should a routine get a category (e.g.
   „Medikamente“, „Bewegung“), so that it can be filtered like the timeline
   and grouped in statistics? Should logs also appear on the timeline?
2. **Daily limit for routines:** **decided 2026-11-23:** none, because no
   routine endpoint calls a model; the count limits of 3.16 protect the
   database.
3. **Window of 7 days for suggestions:** a suggestion whose `takenAt` is
   more than 7 days before the upload is dropped. Agree the value with
   Prof. Pree.
4. **Time zone:** solved by `timeZone` per user (3.12, 3.13, implemented).
   Still open: the daily limits count in Europe/Vienna, and so does the
   import day of a text import (3.1). Should they follow `timeZone` too?
5. **Statistics and time series (phase C):** routine amounts and logs as a
   time series, together with facts such as blood pressure, and questions
   about routines („Wie oft habe ich letzte Woche Vitamin D genommen?“).
   Questions (3.6) only search text chunks; logs need a path of their own,
   and the rule "relations, never causes" applies.
6. **Measured value on a log:** „Blutdruck messen“ could store the value
   (128/82) on the log, not only „erledigt“. How does that connect to
   facts and time series?
7. **Logs without an occurrence:** they count for `amountDone`, not for
   `done` and `due`. Should they also count for `done`, e.g. for flexible
   goals (8)?
8. **Flexible weekly goals:** „2× pro Woche, egal an welchem Tag“, without
   fixed weekdays.
9. **Plan versions:** a change of the schedule also applies to past days.
   Keep old occurrences with a field such as `effectiveFrom`?
10. **Archive instead of delete:** deleting removes the history; an archive
    would keep it for statistics.
11. **Pauses that start in the future** („Urlaub ab Samstag“).
12. **Motivation:** a second reminder if an occurrence stays open; streaks.
13. **Consent (stage 2):** with suggestions, the names, amounts and units
    of the active routines go to Google Gemini with every upload. Update
    the consent text (new `consentVersion`) before stage 2 is switched on.
14. **Changing a log:** today delete and add again; a `PATCH` would keep
    `createdAt`.
15. **Suggestions never reviewed** stay on the entry. Show an overview
    („3 offene Vorschläge“) or let them expire?
16. **Quote in the original:** mark the quote of a suggestion in the
    original, like a verification location.
17. **Prompt change (stage 2):** the read prompt determines `fullText`
    and with it the chunks. After adding the suggestions, run
    `cmd/calibrate` and the live tests again; `fullText` must not change.
18. **Approximate reminders:** proposed deliberately, because exact alarms
    are restricted since Android 14 (3.16, reminders). Is a delay of a few
    minutes acceptable, also for medication? If not: ask for
    `SCHEDULE_EXACT_ALARM` in the app settings, with a hint to the user.

---

## 5. Changes

### 2026-11-27: Reminders in the app, points 5 and 7 (Dylan, Dogan reviews)

The app (PR D) implements the reminders of 3.16. Two points follow what
the platform allows; no endpoint changes.

1. **Point 5:** how late approximate reminders come: usually minutes, in
   the worst case about an hour, more in Doze; measured on the test phone.
2. **Point 7:** no neutral `publicVersion`, because
   flutter_local_notifications 23 has none. `visibility: private` hides
   the content on the lock screen only if the phone is set to hide
   sensitive content. The consent notice 2026-11-26 names it.

### 2026-11-23: Routines stage 1 implemented (Dylan, Dogan reviews)

The backend implements stage 1 of the proposal below: routines,
occurrences, logs, pauses, status, progress and the user's time zone
(3.12, 3.13, 3.16–3.27). The marks "(proposal)" of stage 1 are removed;
stage 2 (routine suggestions, 3.28, 3.29) stays a proposal. Additive for
existing endpoints, except `PATCH /me` with an empty body (point 2).

Clarifications of the proposal, to be confirmed by Dogan in the review:

1. **Time zone names** (3.13): `UTC`, `Etc/UTC` and `Area/Place[/Place]`
   with one of ten areas, every part starting with an upper-case letter,
   at most 64 characters, known to the zone database. `CET`, `EST5EDT`,
   `Japan`, `US/Eastern`, `GMT`, `posix/…` and other spellings are refused.
2. **`PATCH /me`** (3.13): `{}` now answers „Bitte einen Anzeigenamen oder
   eine Zeitzone schicken.“ without `details` (before: the display name
   message with `details.field: displayName`). Order of the errors:
   unknown field, `displayName`, `timeZone`. `timeZone` wrong or `null`:
   „Unbekannte Zeitzone.“ with `details.value`. The login response has no
   `timeZone`.
3. **Dates from 2000 to 2099** for `startDate`, `endDate` and a pause's
   `until`: „Das Datum muss zwischen 2000 und 2099 liegen.“
4. **`interval` is required.**
5. **An amount on a log of a routine without amount** is allowed and
   stored; it counts nowhere.
6. **Pause without a body** counts as `{}`; resume ignores a body.
   Overlapping pauses: pause and resume act on every pause that includes
   today.
7. **New messages** (3.18, 3.20, 3.21, 3.26): body not a JSON object (four
   texts), empty `PATCH`, field that cannot be changed, field of another
   frequency, missing schedule, missing times, unknown or repeated
   weekday, unit not a text (a unit also refuses control characters),
   `startDate` after the stored `endDate`, unknown status and source,
   invalid point in time, and the texts for `from`/`to` and invalid IDs.
   The order of the errors is fixed per endpoint.
8. **Order of the checks** for a routine in the path: `INVALID_ID`, then
   `404`, then body and parameters. A routine of another user is always
   `404`.
9. **Rounding:** `amountDone` and `amountTarget` have at most 6 decimal
   places. Name uniqueness: the name in lower case, `ß` ≠ `ss`. `takenAt`
   `null` counts as missing; points in time are stored to the millisecond.
10. **Deleting a routine** deletes its logs once more after the routine.
11. **Daily limits** stay on Europe/Vienna (open question 4); "no daily
    limit for routines" (open question 2) is decided.
12. **Index `user_entry`** on `routineLogs` comes with stage 2; stage 1
    never uses it.
13. **Consent:** the backend accepts any version, unchanged. Whether the
    app's notice has to name routines (stored in Atlas, never sent to
    Gemini in stage 1) is decided with the app.

Backend: collections `routines` and `routineLogs` with the indexes of the
glossary; `cmd/users delete` also deletes and counts routines and logs;
every new route is classified in the separation test; the cases of the
occurrences are in `backend/internal/core/testdata/routine_occurrences.json`.

### 2026-11-23: Date for text imports and voice notes; consent v2

Implemented (Dylan, App and backend with Dogan's approval; Dogan reviews).
Additive: one new field, no new error code. An older app ignores the new
field and keeps working.

1. **Today's date in the text prompt** ([3.1](#31-upload-and-extract-a-file-or-text),
   step 4): the instruction for a text import (also a voice note) names
   today with the weekday in Europe/Vienna; „heute“, „gestern“ or „letzten
   Montag“ become a date. The prompt for photos and PDFs is unchanged, so
   no new calibration is needed; a test keeps its SHA-256.
2. **Import day instead of null** (3.1, step 5): a text import without any
   date gets the day of `importedAt` in Europe/Vienna. Files keep `null`.
3. **New field `documentDateSource`** ([section 2](#fields)): `document` or
   `import`, in the full entry and the short form; entries stored before
   count as `document`.
4. **Verification** ([section 2](#verification)): a date from the import
   day is not checked (no item in `fields`, never "not found"). In a text
   import, „heute“, „gestern“, „vorgestern“ and the weekday (one to seven
   days before) also confirm the matching date.
5. **Correction** ([3.5](#35-correct-recognized-values)): a `documentDate`
   in the request removes the mark; the import day counts as not
   recognized (`recognized: null`).
6. **Consent** ([3.15](#315-record-the-consent)): the app's notice has
   version `2026-11-23` (netcup in Germany, MongoDB Atlas in Frankfurt,
   recordings of voice notes go to Google Gemini). The backend accepts any
   version, so nothing changes in the API; users who agreed to
   `2026-11-18` see the notice once more.
7. **App:** shows a date from the import day as „23.11.2026 (Importdatum)“
   with „Im Text stand kein Datum. Importiert um 10:46 Uhr.“, and in the
   timeline with the time („23.11.2026, 10:46“).

### 2026-11-23: Routines (proposal, Dylan, agreed with Dogan)

**Proposal.** Stage 1 is implemented since the same day (entry above);
stage 2 is still a proposal. It was the basis for Dogan's review before
anybody wrote backend or app code. Additive, except for two existing
endpoints (upload and delete of an entry, stage 2) and `PATCH /me`.

Two stages (Dogan's review): **stage 1** = points 1, 2 (without the
suggestion endpoints), 4 and 5–8 as far as they concern routines and logs;
**stage 2, later** = the routine suggestions (point 3, endpoints 3.28 and
3.29, `ROUTINE_SUGGESTION_NOT_FOUND`), because they change the read prompt
for every document (`cmd/calibrate` again) and send routine names to Gemini
(consent text first).

1. **Routines** ([3.16](#316-routines-overview-and-data-model)):
   a schedule model of their own (`hourly`, `daily`, `weekly` with
   `interval`, start and end date, optional target amount), pauses as whole
   days, `status`, `progress` (including `amountDone` and `amountTarget`),
   and logs with `dueAt`, `takenAt`, `status`, `amount`, `source`,
   `entryId` and `timeZone`. The backend computes occurrences and
   progress; the app plans local reminders from them.
2. **New endpoints** 3.17–3.29: routines (list, create, get, change,
   pause, resume, delete), occurrences, logs (list, add, delete), confirm
   and reject a routine suggestion.
3. **Entry** ([section 2](#routine-suggestions-proposal-stage-2)): new field
   `routineSuggestions`, always a list. Upload (3.1): with active routines
   the existing read call also returns suggestions; there is no extra model
   call. Delete (3.7): also deletes the logs confirmed from the entry.
4. **User** (3.12, 3.13): new field `timeZone` (IANA name, default
   `Europe/Vienna`), changeable with `PATCH /me`. Routines compute in it;
   points in time stay UTC from the server clock.
5. **New error codes:** `404 ROUTINE_NOT_FOUND`,
   `404 ROUTINE_LOG_NOT_FOUND`, `404 ROUTINE_SUGGESTION_NOT_FOUND`,
   `409 ROUTINE_LOG_EXISTS`, `409 ROUTINE_NAME_TAKEN`,
   `409 ROUTINE_LIMIT_REACHED`. New `details` keys: `routineId`, `logId`,
   `suggestionId`, `max`; new values of `details.limit`: `routines`,
   `pauses`, `logs`.
6. **Per-user data:** routines and logs carry `userId`; IDs of other users
   answer `404`. Nothing of them goes into the log.
7. **App:** a routines view next to the timeline, local reminders with
   three buttons and no name on the lock screen (planned again after a
   restart of the phone, approximate rather than exact), sending the time
   zone; in stage 2 the suggestions in the review.
8. **Backend:** collections `routines` and `routineLogs`; every new route
   in the separation test; `cmd/users delete` also deletes routines and
   logs; embed `time/tzdata`; calibrate again after the prompt change.

### 2026-11-18: Per-user data, daily limits, consent (Dylan, agreed with Dogan)

Builds on "Accounts and login" below; implements what was announced there.

1. **Per-user data** ([per-user data](#per-user-data)): timeline, entry,
   original file, correction, delete and questions only see the user's own
   entries. An entry of another user answers `404 ENTRY_NOT_FOUND`. The
   vector search filters by user and module. `409 DUPLICATE` only within the
   own account (unique index `(userId, file.sha256)`); another user gets
   `201` for the same file. Admins have no additional rights.
2. **Daily limits** ([daily limits](#daily-limits)): per user and calendar
   day in Europe/Vienna, 30 uploads, 60 questions and 30 transcriptions by
   default (`LIMIT_UPLOADS_PER_DAY`, `LIMIT_QUESTIONS_PER_DAY`,
   `LIMIT_TRANSCRIPTIONS_PER_DAY`). Counted after the input checks and
   before any model call. `429 LIMIT_REACHED` with `details.limit`.
3. **Questions while the search index is rebuilt** ([3.6](#36-ask-a-question)):
   `503 MODEL_UNAVAILABLE` with its own message instead of an answer.
4. **Consent** ([3.15](#315-record-the-consent)): `PUT /me/consent` is
   implemented. `GET /me`, `PATCH /me` and `user` in the login response have
   the new field `consentVersion` (string or `null`).
5. **Operation:** entries stored before this change have no owner and are
   visible to nobody until an administrator assigns them with
   `go run ./cmd/users assign-orphans --username <name>` (backend/README.md).
   On its first start the backend updates the vector index; questions only
   work again once it is `READY`.
6. **App:** handle `429 LIMIT_REACHED` (show `message`), read
   `consentVersion` and send `PUT /me/consent` after the password change.

### 2026-11-18: Accounts and login (Dylan, agreed with Dogan)

**Breaking:** from this change on, every endpoint except `GET /status` and
`POST /auth/login` needs a token. An app without login gets
`401 UNAUTHORIZED` everywhere else.

1. **Login and tokens** ([section 1](#login-and-tokens)): header
   `Authorization: Bearer <token>`; `401 UNAUTHORIZED` (with
   `WWW-Authenticate: Bearer`) if the token is missing, unknown or expired
   or the account is disabled, also for unknown paths;
   `403 PASSWORD_CHANGE_REQUIRED` while `mustChangePassword` is `true`,
   except for `GET /me`, `POST /me/password` and `POST /auth/logout`. A
   token is valid for 30 days; the backend stores only its hash.
2. **New endpoints:** `POST /auth/login` ([3.10](#310-log-in)),
   `POST /auth/logout` ([3.11](#311-log-out)), `GET /me`
   ([3.12](#312-get-the-logged-in-user)), `PATCH /me`
   ([3.13](#313-change-the-display-name)), `POST /me/password`
   ([3.14](#314-change-the-password)). `PUT /me/consent`
   ([3.15](#315-record-the-consent)) is described but implemented in a
   later PR.
3. **Failed attempts** ([3.10](#310-log-in)): per username and IP address a
   lock that grows from 1 to 15 minutes, per IP address a lock of
   15 minutes; a wrong `currentPassword` counts too.
   `429 TOO_MANY_ATTEMPTS` with the header `Retry-After` and
   `details.retryAfterSeconds`.
4. **New error codes:** `400 WRONG_PASSWORD`, `400 WEAK_PASSWORD`,
   `401 INVALID_CREDENTIALS`, `401 UNAUTHORIZED`,
   `403 PASSWORD_CHANGE_REQUIRED`, `429 TOO_MANY_ATTEMPTS`; announced:
   `429 LIMIT_REACHED`. New `details` key: `retryAfterSeconds`.
5. **Announced for a later PR:** entries of other users answer
   `404 ENTRY_NOT_FOUND`; `DUPLICATE` only within the own account (unique
   index `(userId, file.sha256)`); `429 LIMIT_REACHED` for a daily limit
   per user.
6. **App:** log in, keep the token like a password, send it with every
   request, ask for a new password while `mustChangePassword` is `true`, and
   show the login again after a `401`.

### 2026-11-12: Transcription (spoken notes)

Additive: no existing field changes.

1. **New endpoint** `POST /api/v1/transcriptions`
   ([3.9](#39-transcribe-a-recording)): form field `audio`, returns `text`,
   `durationSeconds`, `language`, `engine`. Nothing is stored; the app
   sends the checked text through the existing text import.
2. **New error codes:** `413 AUDIO_TOO_LONG`, `422 NO_SPEECH`. Reused:
   `FILE_MISSING`, `INVALID_REQUEST`, `FILE_TOO_LARGE`,
   `UNSUPPORTED_FILE_TYPE`, `UNREADABLE`, `MODEL_UNAVAILABLE`.
3. **Status** ([3.8](#38-status-health-check)): new field `speech`
   (`ok`, `unavailable`, `off`).
4. **App:** microphone permission (`NSMicrophoneUsageDescription` on iOS,
   `RECORD_AUDIO` on Android), recording with the package `record`
   (M4A/AAC, mono, 16 kHz), HTTP timeout at least 130 s, delete the local
   recording afterwards.

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
