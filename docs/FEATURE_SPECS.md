# Lifelog feature specs

One prompt for Claude Code per feature of the app, in build order, written so that the app can be rebuilt feature by feature. What each feature does in detail is in [FEATURES.md](FEATURES.md).

## 1. Foundation: backend connection and mock

```text
Create a Flutter app in app/ for Android and iOS (project lifelog, org at.ac.plus). All display texts are German.
Wrap every backend call from docs/API.md in an ApiClient interface with two implementations: a mock that serves the sample data and every error case from docs/API.md, and an HTTP client. If the app is started with --dart-define=API_BASE_URL, it uses the HTTP client, otherwise the mock, without any code change. In mock mode a small notice says that sample data is shown.
Use a 20 s timeout, and at least 60 s for uploads, questions and original files. Show backend errors as clear German messages. Add all Android and iOS settings the plugins need, including the texts in ios/Runner/Info.plist.
Done when: the app starts in both modes, and tests cover the models, the mock and the HTTP client.
```

## 2. Module overview

```text
Lifelog is a platform with several modules on one shared base, like the Adobe apps. The app opens on a module overview with tiles for Gesundheit (active) and Ernährung, Rezepte and Sport („Bald verfügbar“, not usable).
Keep all modules in one central list with id (the module value from docs/API.md), name, icon, colour, allowed document types and active yes/no. A new module must be nothing more than a new entry in this list, and the list should be replaceable by backend data later. Each module gets a colour scheme from its colour (Gesundheit teal, Ernährung apple green, Rezepte orange, Sport blue); the screens stay the same for every module.
The tiles show plenty of module colour, react with a slight press effect and fade in gently on start. Tapping a tile opens the module with a container transform (animations package).
Done when: a test module needs only a list entry, and widget tests cover the overview.
```

## 3. Navigation inside a module

```text
A module has three pages: „Zeitleiste“, „Hochladen“ and „Fragen“. The user switches with a NavigationBar or by swiping (PageView), kept in sync both ways, with light haptic feedback.
All pages stay alive: sort order, scroll position, the chosen file, a running upload and the last answer survive a page change. On a page change the keyboard closes, and a hidden page must not bring it back. Back leads to the module overview.
Done when: tests show that the state survives page changes and that the keyboard stays closed.
```

## 4. Timeline

```text
Show the module's entries as a timeline: month headings, a vertical line and coloured dots with the icon of the document type. The user sorts by document date or import date, ascending or descending. Group by the chosen date; entries without a date form their own group at the end. Write the grouping as a pure function.
After re-sorting, an upload or a text import the list starts at the top; after a correction it keeps its position. On first load show skeleton placeholders instead of a spinner; an empty list shows a large icon and a button to upload. Colours only mark the type and never judge a value (no red or green; prescriptions purple). Tapping an entry opens it with a container transform.
Done when: grouping tests pass for both sort fields, including entries without a date.
```

## 5. Upload a document

```text
On „Hochladen“ the user takes a photo or picks an image (including WebP) or a PDF. Scale photos down to at most 3000 px so that small print stays readable. Before sending, show a preview with an X in the top right corner to discard it, hidden during the upload. Always show the notice „Dokumente und Sprachnotizen werden zur Auswertung an Google Gemini gesendet (bezahlte Version). Mehr dazu in der Einwilligung unter Einstellungen.“ After a successful upload, open the new entry for review with a fixed bar „Verwerfen“ (deletes at once) / „Speichern“ (since 2026-11-22).
During the upload show a calm scan animation over a document icon and no invented progress; fade between picking, preview, loading and error. If the document already exists (DUPLICATE), show a message with a link to the existing entry. After success, switch to the timeline, reload it and open the new entry.
Done when: the upload, duplicate and error paths are tested against the mock.
```

## 6. Enter text

```text
Instead of a file, the user can type or paste a text on „Hochladen“, for example an own note or a copied report. Send it as text as described in docs/API.md; the backend reads it like a document, and its original is a text file.
„Speichern“ and „Abbrechen“ sit in a fixed bar above the keyboard and stay reachable with long texts. Duplicates are handled as for uploads.
Done when: a widget test at phone size with the keyboard open, not only on a large test screen, shows both buttons and the success message.
```

## 7. Dictate a voice note

```text
Add a microphone button to the text field, shown only when GET /status reports speech as ready.
- Record with the record package (AAC-LC in M4A, mono, 16 kHz, about 32 kbit/s) into the app's temporary folder. Show the running time, a stop and a cancel button. Stop automatically shortly before 5 minutes, on a phone call and when the app goes to the background.
- Ask for microphone permission on Android and iOS with a German text; if it is denied, explain this in German.
- Send the recording to POST /transcriptions (timeout at least 130 s, with a loading indicator). Insert the recognised text at the cursor, or at the end, with a space or line break as separator.
- While the text is edited, highlight numbers (with comma or point) and the unit word right after them, with the hint „Zahlen und Einheiten sind markiert. Bitte besonders genau prüfen.“ Speech recognition can write a real but wrong unit that only a person catches.
- Save only with „Speichern“. Delete the local recording afterwards, also on cancel. For errors such as no speech or a recording that is too long, show the backend's German message.
Done when: tests cover the mock transcription, the error codes, the status check and the deletion of the recording.
```

## 8. View, correct and delete an entry

```text
The entry screen shows document date, issuer, type, the extracted facts and the full text. Every field and fact can be corrected (PATCH in docs/API.md); corrected values are visibly marked as corrected. If a recognised date lies outside 1900–2100, for example a misread year 1026, the date picker starts at today instead of failing. After a date correction the entry sits in the right month group when the screen closes.
„Löschen“ asks „Eintrag löschen?“ and removes the entry with its original. The screen closes without errors, and the entry is gone from the timeline.
Done when: tests cover the corrections, the date bounds and deleting from an opened entry.
```

## 9. Checked values

```text
The backend checks whether each extracted value really appears in the original (verification in docs/API.md). Show this in the entry:
- At the top a summary such as „5 von 6 Werten im Original gefunden (83 %)“ with the source in plain German (text layer of the PDF, text recognition in the photo, typed text). Without a percentage, show only the sentence.
- Since 2026-11-22 (wish of 16.11.): no mark for found values; an unconfirmed value gets a red line „Bitte prüfen: nicht im Original gefunden“ (careful wording: not found ≠ wrong); if all are found, the summary says „Alle N Werte im Original gefunden“. Show corrected values together with the correction mark, not twice.
- Unchecked entries stay neutral, with a short German sentence per reason, for example „Die Prüfung ist auf diesem Server nicht eingerichtet.“ An entry without verification counts as unchecked.
- A short explanation behind an info icon. After a correction, show the new verification from the response.
Done when: the mock covers every source, field status and reason, and tests cover the model, the mock and the display.
```

## 10. Show the original

```text
From every entry the user can open the original file: images with zoom, PDFs inside the app (justify the PDF viewer you choose), text files as UTF-8, selectable and scrollable. Invalid bytes or unknown file types lead to a message, never a crash.
Done when: tests cover an image, a PDF, a text with invalid bytes and an unknown type.
```

## 11. Jump to the place in the original

```text
If a value in the entry (date, issuer or any fact) has a location in the verification, mark it subtly as tappable. Tapping it opens the original at exactly that place, without clashing with the correction actions:
- PDF: render only the page of the first location as an image (pdfx), in an InteractiveViewer with a layer for the rectangles (fractions 0–1 of the upright page). Mark them clearly but not glaringly and zoom so that the place is visible. From there the whole original can be opened.
- Photo: show it upright and map the rectangles to the size actually displayed after the EXIF rotation.
- Text: show the full text, highlight the range start–end (Unicode code points, not UTF-16) and scroll there.
With several locations the user steps through them („Fundstelle 1 von 2“). Back returns to the entry. Screen reader label: „Im Original zeigen“.
Done when: tests cover the rectangle mapping, including rotated photos, and text ranges with umlauts and emojis.
```

## 12. Questions with sources

```text
On „Fragen“ the user asks a question in natural language about the module's own documents. The answer appears with its sources; if nothing is found, it reads exactly „Dazu finde ich nichts in deinen Dokumenten.“
- Compact sources: number, type, date and issuer in one line; the numbers may have gaps. Opening the entry (left part of the card) and expanding („Auszug anzeigen“) are clearly separate.
- Every [n] in the answer with a matching source is a subtle link: it scrolls to that source card, expands it and highlights it briefly. [n] without a source stays plain text, and the answer stays selectable.
- While searching show placeholders; then the answer and the sources fade in one after the other, with no typewriter effect. If the service is unavailable, show a message with „Erneut versuchen“.
- Screen reader labels such as „Quelle 1 anzeigen“, and clear labels for opening and expanding.
Done when: tests cover answers with gaps in the numbering, the no-result sentence, the [n] links and the error case.
```

## 13. Accounts and login

```text
Every request except GET /status and POST /auth/login needs a token (docs/API.md, section 1 "Login and tokens", 3.10–3.15). Security comes first: the token is a secret, the password is never stored.
- Keep only the token and expiresAt, in flutter_secure_storage (exact version pinned), behind a small SessionStore interface with a fake for tests. Set android:allowBackup="false" and exclude cloud backup and device transfer. Send "Authorization: Bearer <token>" with every JSON, multipart and file request; never log tokens, passwords or the header. Add login, logout, me, updateDisplayName, changePassword and recordConsent to the ApiClient, and the error codes of 3.10–3.15 including details.retryAfterSeconds.
- A session controller above the MaterialApp shows: login without a valid token; on start GET /me (401 deletes the token, a network error keeps it and offers „Erneut versuchen“); „Eigenes Passwort festlegen“ while mustChangePassword; the consent notice while consentVersion differs from the app's constant; otherwise the modules. A 401 anywhere logs out with „Bitte melde dich erneut an.“, a 403 PASSWORD_CHANGE_REQUIRED opens the password screen, both handled in one place.
- Login: username without autocorrect or capitals, password hidden with an eye icon. Wrong data: „Benutzername oder Passwort stimmt nicht.“ Lockout: the server message plus „Bitte warte etwa 1 Minute.“ / „… 3 Minuten.“ (rounded up). No „Passwort vergessen“, only „Passwort vergessen? Melde dich bei uns.“
- Password: current, new, repeat; check in the app first (at least 10 characters, at most 72 bytes with umlauts counting double, different from the current one, both equal), then show WRONG_PASSWORD and WEAK_PASSWORD with the server message. Success: „Passwort geändert. Andere Geräte wurden abgemeldet.“
- Consent: the fixed German text with its version as a constant, scrollable, „Zustimmen“ (PUT /me/consent) and „Nicht jetzt“ (logout).
- Settings behind a large gear icon on the module overview: „Angemeldet als <Anzeigename> (<Benutzername>)“, change the display name (1–50 characters), change the password, read the privacy notice, log out. Logout calls POST /auth/logout (errors ignored), deletes the token, temporary files and the image cache and closes every page.
- The mock simulates all of it with documented test accounts, including the lockout after 5 failed attempts.
Done when: a table test shows that every ApiClient method sends the header (and /status and login do not), and widget tests cover login, lockout, start with an invalid token or without network, the password rules, consent, a 401 in the middle of use, logout and the settings icon.
```
