# Roadmap

Every wish from the meetings and what the test server needs, with a rough
effort. Updated 2026-11-22. ☐ open, ☑ done.

## Test week (from 2026-11-23)

| | Task | Effort |
|---|---|---|
| ☑ | Test server with HTTPS, accounts, own database, paid Gemini ([SERVER.md](../SERVER.md)) | – |
| ☑ | Speech via Gemini on servers without a GPU (PR #29) | – |
| ☑ | Review after upload: „Speichern“ / „Verwerfen“ (PR #30) | – |
| ☑ | Red „Bitte prüfen: nicht im Original gefunden“, „Alle N Werte im Original gefunden“ (PR #30) | – |
| ☑ | Scroll bug in long voice notes (PR #30) | – |
| ☐ | APK with the server address, shared through Google Drive | 1–2 h |
| ☐ | Security check by Dylan before access data goes out | 1–2 h |

## Phase B: until the next meeting

| | Wish | What belongs to it | Effort |
|---|---|---|---|
| ☐ | Categories with parent and child items | API first; backend: field on the entry, model suggests, filter; app: check boxes in the review, filter in the timeline | 1.5–2.5 days |
| ☐ | Voice note as a type of its own | title „Sprachnachricht vom …“ plus a summary | 0.5–1 day |
| ☐ | Edit the text of a voice note later | | 0.5 day |
| ☐ | Own verification display for voice notes | the original is the text | 2 h |

## Phase C: afterwards

| | Wish | Effort |
|---|---|---|
| ☐ | Routines: period, frequency, daily goal, button „neue Routine“ | 3–5 days |
| ☐ | Reminders (push notifications) | 1–2 days |
| ☐ | Pause a routine | 0.5 day |
| ☐ | Motivation (progress, streaks) | 1–2 days |
| ☐ | Store values as time series (basis for statistics) | 1–2 days |
| ☐ | Charts over time | 1–2 days |
| ☐ | Questions about the statistics (relations only, never causes) | 2–3 days |
| ☐ | „taken“ versus „prescribed“ | 1 day |
| ☐ | Settings with a large icon | 2 h |
| ☐ | Encrypt data | 2–4 days |

## Operations

| | Task | Why | Effort |
|---|---|---|---|
| ☐ | Vertex AI instead of the Gemini API, region Frankfurt | 2026-11-22 the Flash models were overloaded and only Flash-Lite answered; own capacity, processing in the EU | 0.5–1 day |
| ☐ | Process uploads in the background with automatic retries | the app does not wait up to 50 s; overload is absorbed | 2–3 days |
| ☐ | Main model: Flash-Lite (stopgap) or 3.8-flash, measured with `cmd/calibrate` | reading quality | 1 h |
| ☐ | Remove EXIF/GPS from photos | location data | 2 h |
| ☐ | Monitoring: mail when the server is down | notice before the testers do | 30 min |
| ☐ | Fixed release key for the APK | updates without uninstalling | 1 h |
| ☐ | iPhone: TestFlight and an Apple developer account, if needed | an APK only runs on Android | 1 day |
| ☐ | Consent text: name MongoDB Atlas (Frankfurt) | transparency | 15 min |
