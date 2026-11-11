# OCR measurement 2026-11-11

Generated with `cd backend && go run ./cmd/ocrcheck`. Programs: pdftotext 26.11.0, tesseract 5.5.4 deu. Reference values: the current values (including corrections) of the entries in database "lifelog" that were uploaded from `demo/`; every variant of a document is checked against the same entry. No model was called. Rules: docs/API.md, section verification.

"Facts (value + unit)" is the strict rule of the verification; "value only" ignores the unit and shows how many facts fail only because of the unit.

## Per document

| Document | Variant | Source | Date | Issuer | Facts (value + unit) | Facts (value only) | Match | Mean OCR confidence | Time |
|---|---|---|---|---|---|---|---|---|---|
| 01_lab_report_2026-01-20 | pdf | pdf_text | ✓ | ✓ | 5/5 | 5/5 | 100 % | – | 0.0 s |
| 01_lab_report_2026-01-20 | pdf, forced OCR | ocr | ✓ | ✓ | 5/5 | 5/5 | 100 % | 91 | 0.8 s |
| 01_lab_report_2026-01-20 | photo | ocr | ✓ | ✓ | 0/5 | 4/5 | 29 % | 88 | 0.3 s |
| 02_doctor_letter_orthopedics_2026-03-05 | pdf | pdf_text | ✓ | ✓ | 3/3 | 3/3 | 100 % | – | 0.0 s |
| 02_doctor_letter_orthopedics_2026-03-05 | pdf, forced OCR | ocr | ✓ | ✓ | 3/3 | 3/3 | 100 % | 96 | 0.9 s |
| 02_doctor_letter_orthopedics_2026-03-05 | photo | ocr | ✓ | ✓ | 3/3 | 3/3 | 100 % | 94 | 0.3 s |
| 03_referral_physiotherapy_2026-03-05 | pdf | pdf_text | ✓ | ✓ | 2/2 | 2/2 | 100 % | – | 0.0 s |
| 03_referral_physiotherapy_2026-03-05 | pdf, forced OCR | ocr | ✓ | ✓ | 2/2 | 2/2 | 100 % | 96 | 0.7 s |
| 03_referral_physiotherapy_2026-03-05 | photo | ocr | ✓ | ✗ | 2/2 | 2/2 | 75 % | 91 | 0.2 s |
| 04_prescription_2026-05-18 | pdf | pdf_text | ✓ | ✓ | 3/3 | 3/3 | 100 % | – | 0.0 s |
| 04_prescription_2026-05-18 | pdf, forced OCR | ocr | ✓ | ✓ | 3/3 | 3/3 | 100 % | 95 | 0.6 s |
| 04_prescription_2026-05-18 | photo | ocr | ✓ | ✓ | 3/3 | 3/3 | 100 % | 92 | 0.2 s |
| 04_prescription_2026-05-18 | photo (live) | ocr | ✓ | ✓ | 3/3 | 3/3 | 100 % | 94 | 0.2 s |
| 04_prescription_2026-05-18 | app | ocr | ✓ | ✓ | 3/3 | 3/3 | 100 % | 94 | 0.2 s |
| 05_vaccination_tbe_2026-06-10 | pdf | pdf_text | ✓ | ✓ | 4/4 | 4/4 | 100 % | – | 0.0 s |
| 05_vaccination_tbe_2026-06-10 | pdf, forced OCR | ocr | ✓ | ✓ | 4/4 | 4/4 | 100 % | 95 | 0.7 s |
| 05_vaccination_tbe_2026-06-10 | photo | ocr | ✓ | ✓ | 4/4 | 4/4 | 100 % | 92 | 0.2 s |
| 06_lab_report_followup_2026-07-22 | pdf | pdf_text | ✓ | ✓ | 5/5 | 5/5 | 100 % | – | 0.0 s |
| 06_lab_report_followup_2026-07-22 | pdf, forced OCR | ocr | ✓ | ✓ | 5/5 | 5/5 | 100 % | 91 | 0.8 s |
| 06_lab_report_followup_2026-07-22 | photo | ocr | ✓ | ✓ | 1/5 | 4/5 | 43 % | 85 | 0.3 s |
| 07_note_physiotherapy_2026-08-12 | text | text | ✓ | ✓ | 2/2 | 2/2 | 100 % | – | 0.0 s |
| 08_note_blood_pressure_2026-09-23 | text | text | ✓ | – | 2/2 | 2/2 | 100 % | – | 0.0 s |
| L1_lab_report_2026-10-28 | pdf | pdf_text | ✓ | ✓ | 3/3 | 3/3 | 100 % | – | 0.0 s |
| L1_lab_report_2026-10-28 | pdf, forced OCR | ocr | ✓ | ✓ | 2/3 | 3/3 | 80 % | 93 | 0.7 s |
| L2_doctor_letter_followup_2026-04-30 | pdf | pdf_text | ✓ | ✓ | 2/2 | 2/2 | 100 % | – | 0.0 s |
| L2_doctor_letter_followup_2026-04-30 | pdf, forced OCR | ocr | ✓ | ✓ | 2/2 | 2/2 | 100 % | 96 | 0.9 s |
| L3_eye_test_2026-09-15 | photo (live) | ocr | ✓ | ✓ | 1/4 | 1/4 | 50 % | 90 | 0.2 s |
| L3_eye_test_2026-09-15 | app | ocr | ✓ | ✓ | 1/4 | 1/4 | 50 % | 90 | 0.2 s |

## Summary

| Variant | Documents | Match mean | Match min | Facts (value + unit) | Facts (value only) | Unchecked |
|---|---|---|---|---|---|---|
| PDF, text layer | 8 | 100 % | 100 % | 27/27 (100 %) | 27/27 (100 %) | 0 |
| PDF, OCR (forced) | 8 | 98 % | 80 % | 26/27 (96 %) | 27/27 (100 %) | 0 |
| Photo | 10 | 75 % | 29 % | 21/36 (58 %) | 28/36 (78 %) | 0 |
| Text import | 2 | 100 % | 100 % | 4/4 (100 %) | 4/4 (100 %) | 0 |

## Not confirmed

- **01_lab_report_2026-01-20, photo**
  - f1 Gesamtcholesterin: 232 mg/dl — read: "Gesamtcholesterin 232 mgjal"
  - f2 LDL-Cholesterin: 154 mg/dl — read: "LDL-Cholesterin 154 mgjdl"
  - f3 HDL-Cholesterin: 48 mg/dl — read: "HDL-Cholesterin 48 mgjdl"
  - f4 Triglyceride: 165 mg/dl — read: "Triglyceride mgjdl"
  - f5 Glukose nüchtern: 98 mg/dl — read: "Glukose nüchtern 98 mg/di 70-99"
- **03_referral_physiotherapy_2026-03-05, photo**
  - issuer: Orthopädie am Fluss, Dr. Thomas Probe — "Orthopädie am Fluss" read: "Dr. Thomas Probe, Facharzt für Orthopädie" — "Dr. Thomas Probe" read: "Dr. Thomas Probe, Facharzt für Orthopädie"
- **06_lab_report_followup_2026-07-22, photo**
  - f1 Gesamtcholesterin: 205 mg/dl — read: "Gesamtcholesterin 205 mg/di"
  - f3 HDL-Cholesterin: 52 mg/dl — read: "HDL-Cholesterin 52 mg/di"
  - f4 Triglyceride: 140 mg/dl — read: "Triglyceride mg/di"
  - f5 Glukose nüchtern: 94 mg/dl — read: "Glukose nüchtern 94 mg/di 70-99"
- **L1_lab_report_2026-10-28, pdf, forced OCR**
  - f3 TSH: 1,8 mU/l — read: "TSH 1,8 mUy/l 0,4 -4,0"
- **L3_eye_test_2026-09-15, photo (live)**
  - f1 Sehschärfe ohne Brille Rechts: 1,0 — read: "Sehschärfe ohne Brille 10 0,8"
  - f3 Sehschärfe mit Brille Rechts: 1,2 — read: "Sehschärfe mit Brille rl 12"
  - f4 Sehschärfe mit Brille Links: 1,2 — read: "Sehschärfe mit Brille rl 12"
- **L3_eye_test_2026-09-15, app**
  - f1 Sehschärfe ohne Brille Rechts: 1,0 — read: "Sehschärfe ohne Brille 10 0,8"
  - f3 Sehschärfe mit Brille Rechts: 1,2 — read: "Sehschärfe mit Brille rl 12"
  - f4 Sehschärfe mit Brille Links: 1,2 — read: "Sehschärfe mit Brille rl 12"
