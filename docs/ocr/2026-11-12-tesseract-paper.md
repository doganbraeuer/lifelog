# OCR measurement 2026-11-12

Generated with `cd backend && go run ./cmd/ocrcheck`. Programs: pdftotext 26.11.0, tesseract 5.5.4 deu. Reference values: the current values (including corrections) of the entries in database "lifelog" that were uploaded from `demo/`; every variant of a document is checked against the same entry. No model was called. Rules: docs/API.md, section verification.

Images for Tesseract: turned upright, scaled up to a long side of 2500 pixels if smaller (at most twice), scaled down to 4096 pixels if larger, as RGB PNG; Tesseract with --dpi 300.

"Facts (value + unit)" is the strict rule of the verification; "value only" ignores the unit and shows how many facts fail only because of the unit.

## Per document

| Document | Variant | Source | Date | Issuer | Facts (value + unit) | Facts (value only) | Match | Mean OCR confidence | Time |
|---|---|---|---|---|---|---|---|---|---|
| 01_lab_report_2026-01-20 | pdf | pdf_text | ✓ | ✓ | 5/5 | 5/5 | 100 % | – | 0.0 s |
| 01_lab_report_2026-01-20 | pdf, forced OCR | ocr | ✓ | ✓ | 5/5 | 5/5 | 100 % | 91 | 0.8 s |
| 01_lab_report_2026-01-20 | photo | ocr | ✓ | ✓ | 2/5 | 5/5 | 57 % | 92 | 0.6 s |
| 01_lab_report_2026-01-20 | phone, screen JPG | ocr | ✓ | ✓ | 5/5 | 5/5 | 100 % | 91 | 1.6 s |
| 01_lab_report_2026-01-20 | phone, screen HEIC | ocr | ✓ | ✓ | 2/5 | 3/5 | 57 % | 81 | 4.1 s |
| 01_lab_report_2026-01-20 | phone, paper JPG | ocr | ✓ | ✓ | 4/5 | 5/5 | 86 % | 87 | 1.5 s |
| 02_doctor_letter_orthopedics_2026-03-05 | pdf | pdf_text | ✓ | ✓ | 3/3 | 3/3 | 100 % | – | 0.0 s |
| 02_doctor_letter_orthopedics_2026-03-05 | pdf, forced OCR | ocr | ✓ | ✓ | 3/3 | 3/3 | 100 % | 96 | 0.9 s |
| 02_doctor_letter_orthopedics_2026-03-05 | photo | ocr | ✓ | ✓ | 3/3 | 3/3 | 100 % | 96 | 0.7 s |
| 03_referral_physiotherapy_2026-03-05 | pdf | pdf_text | ✓ | ✓ | 2/2 | 2/2 | 100 % | – | 0.0 s |
| 03_referral_physiotherapy_2026-03-05 | pdf, forced OCR | ocr | ✓ | ✓ | 2/2 | 2/2 | 100 % | 96 | 0.7 s |
| 03_referral_physiotherapy_2026-03-05 | photo | ocr | ✓ | ✓ | 2/2 | 2/2 | 100 % | 96 | 0.5 s |
| 04_prescription_2026-05-18 | pdf | pdf_text | ✓ | ✓ | 3/3 | 3/3 | 100 % | – | 0.0 s |
| 04_prescription_2026-05-18 | pdf, forced OCR | ocr | ✓ | ✓ | 3/3 | 3/3 | 100 % | 95 | 0.7 s |
| 04_prescription_2026-05-18 | photo | ocr | ✓ | ✓ | 3/3 | 3/3 | 100 % | 96 | 0.5 s |
| 04_prescription_2026-05-18 | photo (live) | ocr | ✓ | ✓ | 3/3 | 3/3 | 100 % | 96 | 0.5 s |
| 04_prescription_2026-05-18 | app | ocr | ✓ | ✓ | 3/3 | 3/3 | 100 % | 96 | 0.5 s |
| 05_vaccination_tbe_2026-06-10 | pdf | pdf_text | ✓ | ✓ | 4/4 | 4/4 | 100 % | – | 0.0 s |
| 05_vaccination_tbe_2026-06-10 | pdf, forced OCR | ocr | ✓ | ✓ | 4/4 | 4/4 | 100 % | 95 | 0.7 s |
| 05_vaccination_tbe_2026-06-10 | photo | ocr | ✓ | ✓ | 4/4 | 4/4 | 100 % | 95 | 0.5 s |
| 06_lab_report_followup_2026-07-22 | pdf | pdf_text | ✓ | ✓ | 5/5 | 5/5 | 100 % | – | 0.0 s |
| 06_lab_report_followup_2026-07-22 | pdf, forced OCR | ocr | ✓ | ✓ | 5/5 | 5/5 | 100 % | 91 | 0.8 s |
| 06_lab_report_followup_2026-07-22 | photo | ocr | ✓ | ✓ | 3/5 | 5/5 | 71 % | 92 | 0.6 s |
| 07_note_physiotherapy_2026-08-12 | text | text | ✓ | ✓ | 2/2 | 2/2 | 100 % | – | 0.0 s |
| 08_note_blood_pressure_2026-09-23 | text | text | ✓ | – | 2/2 | 2/2 | 100 % | – | 0.0 s |
| L1_lab_report_2026-10-28 | pdf | pdf_text | ✓ | ✓ | 3/3 | 3/3 | 100 % | – | 0.0 s |
| L1_lab_report_2026-10-28 | pdf, forced OCR | ocr | ✓ | ✓ | 2/3 | 3/3 | 80 % | 93 | 0.7 s |
| L2_doctor_letter_followup_2026-04-30 | pdf | pdf_text | ✓ | ✓ | 2/2 | 2/2 | 100 % | – | 0.0 s |
| L2_doctor_letter_followup_2026-04-30 | pdf, forced OCR | ocr | ✓ | ✓ | 2/2 | 2/2 | 100 % | 96 | 0.9 s |
| L3_eye_test_2026-09-15 | photo (live) | ocr | ✓ | ✓ | 4/4 | 4/4 | 100 % | 95 | 0.5 s |
| L3_eye_test_2026-09-15 | app | ocr | ✓ | ✓ | 4/4 | 4/4 | 100 % | 95 | 0.5 s |

## Summary

| Variant | Documents | Match mean | Match min | Facts (value + unit) | Facts (value only) | Unchecked |
|---|---|---|---|---|---|---|
| PDF, text layer | 8 | 100 % | 100 % | 27/27 (100 %) | 27/27 (100 %) | 0 |
| PDF, OCR (forced) | 8 | 98 % | 80 % | 26/27 (96 %) | 27/27 (100 %) | 0 |
| Photo | 10 | 93 % | 57 % | 31/36 (86 %) | 36/36 (100 %) | 0 |
| Phone photo of a screen (real) | 2 | 78 % | 57 % | 7/10 (70 %) | 8/10 (80 %) | 0 |
| Phone photo of paper (real) | 1 | 86 % | 86 % | 4/5 (80 %) | 5/5 (100 %) | 0 |
| Text import | 2 | 100 % | 100 % | 4/4 (100 %) | 4/4 (100 %) | 0 |

## Not confirmed

- **01_lab_report_2026-01-20, photo**
  - f1 Gesamtcholesterin: 232 mg/dl — read: "Gesamtcholesterin 232 mgj/dl"
  - f4 Triglyceride: 165 mg/dl — read: "Triglyceride 165 mgj/dl"
  - f5 Glukose nüchtern: 98 mg/dl — read: "Glukose nüchtern 98 mgj/dl 70-99"
- **01_lab_report_2026-01-20, phone, screen HEIC**
  - f1 Gesamtcholesterin: 232 mg/dl — name not found in any row
  - f2 LDL-Cholesterin: 154 mg/dl — read: "LDL-Cholesterin"
  - f4 Triglyceride: 165 mg/dl — read: "Triglyceride 165 mog/dl <150"
- **01_lab_report_2026-01-20, phone, paper JPG**
  - f4 Triglyceride: 165 mg/dl — read: "Triglyceride 165 mgj/dl <150 |"
- **06_lab_report_followup_2026-07-22, photo**
  - f1 Gesamtcholesterin: 205 mg/dl — read: "Gesamtcholesterin 205 mgj/dl"
  - f5 Glukose nüchtern: 94 mg/dl — read: "Glukose nüchtern 94 mgj/dl 70-99"
- **L1_lab_report_2026-10-28, pdf, forced OCR**
  - f3 TSH: 1,8 mU/l — read: "TSH 1,8 mUy/l 0,4 -4,0"
