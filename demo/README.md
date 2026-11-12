# Demo data

Synthetic documents for demos, manual tests and the calibration of the
search threshold. Everything is invented and marked as TESTDOKUMENT; the
patient is always "Max Mustermann, geb. 01.01.1990", as in the seed entries
(`backend/cmd/seed`). The documents are German on purpose, because real users
upload German documents. Do not translate them: other text gives other
vectors and breaks the comparison with earlier calibration runs.

## Fill the database

Start the backend, then run from the repository root:

```sh
demo/upload.sh             # seed entries + 11 uploads
demo/upload.sh --no-seed   # uploads only
```

The script checks `GET /api/v1/status`, runs `go run ./cmd/seed` (4 entries
with embeddings) and uploads the same 11 files as in the calibration
baseline (`docs/calibration/2026-11-11-baseline.md`). On 503 (model
overloaded) it waits and retries; files that already exist answer 409 and
are skipped, so running it again is harmless. Another backend:
`LIFELOG_URL=http://192.168.0.10:8080 demo/upload.sh`.

Afterwards wait about a minute until Atlas has indexed the chunks and run
`cd backend && go run ./cmd/calibrate`. It finds the uploaded entries by file
name, so do not rename the uploaded files without changing
`backend/cmd/calibrate/questions.go` (a test checks this).

## Contents

`pdf/` and `photos/` contain the same six documents, as PDF and as a
"photographed" PNG (slightly tilted, with a shadow). `texts/` contains two
notes for the text import.

| No. | Document | Uploaded by the script |
|---|---|---|
| 01 | Lab report 2026-01-20: lipid profile (cholesterol, LDL, HDL, triglycerides, glucose) | PDF |
| 02 | Doctor's letter orthopedics 2026-03-05: right knee, physiotherapy recommended | photo |
| 03 | Referral 2026-03-05: 6 physiotherapy sessions | PDF |
| 04 | Prescription 2026-05-18: Musterol 400 mg, 1-0-1, 20 pieces | no (see `live/`) |
| 05 | Vaccination certificate 2026-06-10: TBE booster, next 06/2029 | PDF |
| 06 | Lab report 2026-07-22: lipid profile follow-up (other values than in January) | PDF |
| 07 | Note 2026-08-12: physiotherapy finished | text |
| 08 | Note 2026-09-23: blood pressure at home 122/79 | text |

`live/` contains the documents for live uploads during a presentation:

| File | Document | Uploaded by the script |
|---|---|---|
| `L1_lab_report_2026-10-28.pdf` | Lab report: vitamin D, ferritin, TSH | yes |
| `L2_doctor_letter_followup_2026-04-30.pdf` | Orthopedics follow-up, knee without complaints | yes |
| `L3_eye_test_2026-09-15_photo.png` | Eye test at the optician | no |
| `04_prescription_2026-05-18_photo.png` | Prescription Musterol, for the upload via the app | no |
| `L3_eye_test_2026-09-15_app.png` | The eye test as the app uploaded it on 2026-11-09 | yes |
| `04_prescription_2026-05-18_app.png` | The prescription as the app uploaded it on 2026-11-09 | yes |

`phone/` contains real phone photos of demo documents (for the OCR
measurement, `cmd/ocrcheck`; not uploaded by the script). `_paper` marks a
photo of the printed document, `_phone` a photo of the document **on a
screen** (not paper). The location was removed from every photo before it
was added:

| File | Document |
|---|---|
| `01_lab_report_2026-01-20_paper.jpg` | Lab report 01 printed on paper, on a table, photographed with an iPhone 14 Pro Max on 2026-11-11, upright JPEG. Location removed before it was added (no GPS in EXIF); device, time and image data unchanged. |
| `01_lab_report_2026-01-20_phone.jpg` | **Screen, not paper:** lab report 01 on a screen, photographed with an iPhone 14 Pro Max on 2026-11-11, upright JPEG. EXIF, XMP and Apple segments removed; the image data is unchanged byte for byte. |
| `01_lab_report_2026-01-20_phone.heic` | **Screen, not paper:** the same document on the screen, photographed again as HEIC (the iPhone's own format): stored lying on its side with "rotate by 90°", at an angle, with moiré. The GPS block and the maker note in EXIF were overwritten with zeros; the image data is unchanged. |

The `_app.png` files are the exact bytes stored in Atlas after the upload
from the iOS app: the image picker re-encodes the image, so they differ from
the `_photo.png` originals. The script uploads them so that a refilled
database contains exactly the same files as the baseline.

## Live upload in the app

After `demo/upload.sh`, the prescription and the eye test are already in the
database, and uploading them again gives 409. To show a live upload, delete
the entry in the app first (or skip the two `_app.png` lines in the script).

- iOS simulator: `xcrun simctl addmedia <DEVICE-ID> demo/live/04_prescription_2026-05-18_photo.png`,
  then in the app: upload → choose image.
- Android phone: copy `demo/live/04_prescription_2026-05-18_photo.png` to the
  phone.

## Good demo questions

The questions are German, because users ask in German.

| Question | Expected |
|---|---|
| Wie hat sich mein LDL-Cholesterin verändert? | two sources (January and July) |
| Wann war meine letzte FSME-Impfung? | vaccination certificate |
| Welche Physiotherapie wurde mir verordnet und habe ich sie abgeschlossen? | referral and note |
| Wie hoch war mein Blutdruck? | note and doctor's letter from the seed entries |
| Welches Medikament wurde mir im Mai verschrieben? | prescription 2026-05-18 |
| Welche Allergien habe ich? | "near" question, in no document: fixed sentence |
| Wie wird das Wetter morgen? | unrelated, fixed sentence without a model call |
