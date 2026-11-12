# Speech recognition: whisper.cpp large-v3-turbo

Generated on 2026-11-12 by `go run ./cmd/speechcheck` (docs/speech/README.md). Every recording goes through the same pipeline as `POST /api/v1/transcriptions` (conversion, length and loudness check, Whisper, cleaning). No language model is called, nothing is stored.

- Engine: whisper.cpp large-v3-turbo, started with `backend/scripts/whisper-server.sh`
- Converter: ffmpeg 8.1.1
- Machine: Apple M3 Pro, darwin/arm64
- Prompt: none (`-vocabulary=false`); the terms of the list are still checked below
- Key terms are compared word by word (`core.TextContains`): case, punctuation at the edges and decimal comma do not count, word boundaries do („keiner Guss“ is not „kein Erguss“, „101“ is not „1-0-1“). Each term lists the spellings that count as right (`cmd/speechcheck/samples.go`).
- WER (word error rate) is strict: „400 mg“ for „400 Milligramm“ or „6“ for „sechs“ counts as an error. The key terms are the measure that matters.

## Summary

| Source | Recordings | Speech → text | WER (mean) | Key terms found | No speech → NO_SPEECH | Real-time factor (max) |
|---|---|---|---|---|---|---|
| iPhone | 10 | 9/9 | 9.6 % | 44/51 (86 %) | 1/1 | 0.11 |
| Anna (say) | 8 | 8/8 | 8.4 % | 45/47 (96 %) | – | 0.17 |
| generated | 5 | 1/1 | 15.4 % | 2/4 (50 %) | 4/4 | 0.17 |

## Criteria

- ❌ Key terms, iPhone: 86 % (at least 95 %)
- ✅ Key terms, Anna (say): 96 % (at least 95 %)
- ✅ No text without speech: 5 of 5 recordings without speech gave NO_SPEECH
- ✅ No term of the list written without being said, in 25 recordings (for recordings without speech also in what Whisper alone writes)
- ✅ Speed, iPhone: 145 s of speech in 5.3 s, so 3 minutes take about 7 s (at most 30 s)
- ✅ Speed, Anna (say): 98 s of speech in 4.5 s, so 3 minutes take about 8 s (at most 30 s)

## Recordings without speech

Which check gave NO_SPEECH, and what Whisper writes for the same audio without the loudness check and the cleaning (sent to whisper-server directly, with its own voice activity detection).

| Source | Recording | What | Caught by | Whisper alone |
|---|---|---|---|---|
| iPhone | 09 | silence in a quiet room | nothing left after cleaning 0 segments | – |
| generated | silence | 10 s digital silence | 0.00 s above the silence threshold | – |
| generated | noise | 10 s white noise at -30 dBFS | nothing left after cleaning 0 segments | – |
| generated | quiet noise | 10 s white noise at -40 dBFS | nothing left after cleaning 0 segments | – |
| generated | brown noise | 10 s brown noise (low rumble) at -30 dBFS | nothing left after cleaning 0 segments | – |

## Per recording

| Source | Recording | What | Length | Time | RTF | Result | WER | Key terms | Missing |
|---|---|---|---|---|---|---|---|---|---|
| iPhone | 01 | lab report, date and values | 22.4 s | 1.4 s | 0.06 | text | 6 % | 7/7 | – |
| iPhone | 02 | prescription, dose scheme | 14.0 s | 1.3 s | 0.09 | text | 31 % | 4/5 | eins null eins |
| iPhone | 03 | blood pressure note | 12.8 s | 1.3 s | 0.10 | text | 8 % | 3/4 | Puls 64 |
| iPhone | 04 | referral, diagnosis and doctor | 14.3 s | 1.3 s | 0.09 | text | 15 % | 5/6 | Gonarthrose rechts |
| iPhone | 05 | vaccination, two dates | 12.0 s | 1.3 s | 0.11 | text | 0 % | 3/3 | – |
| iPhone | 06 | lab values with decimal comma and units | 14.7 s | 1.3 s | 0.09 | text | 8 % | 4/5 | Milliunits pro Liter |
| iPhone | 07 | follow-up, date and medication | 11.7 s | 1.3 s | 0.11 | text | 0 % | 5/5 | – |
| iPhone | 08 | spoken doctor's letter (about 40 s) | 39.6 s | 2.3 s | 0.06 | text | 11 % | 10/12 | kein Erguss, sechs Einheiten |
| iPhone | 09 | silence in a quiet room | 16.2 s | 0.6 s | 0.04 | NO_SPEECH | – | – | – |
| iPhone | 10 | sentence 03 with background noise | 14.0 s | 1.3 s | 0.09 | text | 8 % | 3/4 | Puls 64 |
| iPhone | 01–08 joined | all read sentences in one recording, 0.5 s apart | 145.0 s | 5.3 s | 0.04 | text | 7 % | 42/47 | eins null eins, Puls 64, Gonarthrose rechts, Milliunits pro Liter, kein Erguss |
| Anna (say) | 01 | lab report, date and values | 14.9 s | 1.4 s | 0.09 | text | 18 % | 7/7 | – |
| Anna (say) | 02 | prescription, dose scheme | 8.1 s | 1.3 s | 0.15 | text | 31 % | 4/5 | eins null eins |
| Anna (say) | 03 | blood pressure note | 7.6 s | 1.3 s | 0.17 | text | 0 % | 4/4 | – |
| Anna (say) | 04 | referral, diagnosis and doctor | 8.8 s | 1.3 s | 0.15 | text | 8 % | 6/6 | – |
| Anna (say) | 05 | vaccination, two dates | 8.8 s | 1.3 s | 0.14 | text | 0 % | 3/3 | – |
| Anna (say) | 06 | lab values with decimal comma and units | 8.4 s | 1.3 s | 0.15 | text | 8 % | 4/5 | Milliunits pro Liter |
| Anna (say) | 07 | follow-up, date and medication | 8.6 s | 1.3 s | 0.15 | text | 0 % | 5/5 | – |
| Anna (say) | 08 | spoken doctor's letter (about 40 s) | 29.6 s | 1.7 s | 0.06 | text | 3 % | 12/12 | – |
| Anna (say) | 01–08 joined | all read sentences in one recording, 0.5 s apart | 98.3 s | 4.5 s | 0.05 | text | 6 % | 45/47 | eins null eins, Milliunits pro Liter |
| generated | silence | 10 s digital silence | 10.0 s | 0.0 s | 0.00 | NO_SPEECH | – | – | – |
| generated | noise | 10 s white noise at -30 dBFS | 10.0 s | 0.6 s | 0.06 | NO_SPEECH | – | – | – |
| generated | quiet noise | 10 s white noise at -40 dBFS | 10.0 s | 0.6 s | 0.06 | NO_SPEECH | – | – | – |
| generated | brown noise | 10 s brown noise (low rumble) at -30 dBFS | 10.0 s | 0.6 s | 0.06 | NO_SPEECH | – | – | – |
| generated | 03 + noise | Anna's sentence 03 with white noise at -30 dBFS | 7.6 s | 1.3 s | 0.17 | text | 15 % | 2/4 | Blutdruck, im Sitzen |

## Vocabulary

The terms of `whisper.Vocabulary`, written as general terms of doctor's letters, lab reports and prescriptions, not taken from the test sentences. Terms that happen to be said in a test sentence are marked with its number (also when they are the written form of what was said, e.g. „1-0-1“ for „eins null eins“, or part of a word like „FSME-Auffrischung“); a better result for those terms can come from the list itself.

- In the test sentences (18 of 45): Blutdruck (03), Puls (03), LDL (01), HDL (01), TSH (06), Ferritin (06), Vitamin D (06), mg/dl (01), ng/ml (06), mU/l (06), Ibuprofen (07, 08), 1-0-1 (02), bei Bedarf (07), Gonarthrose (04), Erguss (08), Ultraschall (08), Physiotherapie (04, 08), FSME (05)
- Not in the test sentences (27): mmHg, Hämoglobin, Kreatinin, HbA1c, Glukose, Cholesterin, Triglyceride, CRP, mmol/l, g/dl, Paracetamol, Pantoprazol, Ramipril, Metformin, L-Thyroxin, 1-0-0, 0-0-1, 1-1-1, Hypertonie, Diabetes mellitus, Hypothyreose, Arthrose, Bandscheibenvorfall, Fraktur, Röntgen, MRT, EKG

## Recognized text

Only single recordings with speech (the joined ones contain the same sentences).

**iPhone 01** (WER 6 %)

- Said: Laborbefund vom 20. Jänner 2026, Labor Muster GmbH. Gesamtcholesterin 232 Milligramm pro Deziliter, LDL 154, HDL 48.
- Recognized: Laborbefund vom 20. Jänner 2026. Labor Muster GmbH. Gesamtcholesterin 232 mg pro Deziliter. LDL 154, HDL 48.

**iPhone 02** (WER 31 %)

- Said: Rezept vom 18. Mai 2026: Musterol 400 Milligramm, eins null eins, 20 Stück.
- Recognized: Rezept vom 18. Mai 2026. Musterol 400 mg 101, 20 Stück.

**iPhone 03** (WER 8 %)

- Said: Blutdruck heute Morgen 122 zu 79, Puls 64, zu Hause im Sitzen gemessen.
- Recognized: Blutdruck heute Morgen 122 zu 79 plus 64 zu Hause im Sitzen gemessen.

**iPhone 04** (WER 15 %)

- Said: Überweisung zur Physiotherapie, sechs Einheiten, Diagnose Gonarthrose rechts, von Dr. Erika Beispiel, Allgemeinmedizin.
- Recognized: Überweisung zur Physiotherapie, 6 Einheiten, Diagnose, Gonarrose, rechts, von Dr. Erika Beispiel, Allgemeinmedizin.

**iPhone 05** (WER 0 %)

- Said: FSME-Auffrischung am 10. Juni 2026, die nächste Impfung ist im Juni 2029 fällig.
- Recognized: FSME-Auffrischung am 10. Juni 2026. Die nächste Impfung ist im Juni 2029 fällig.

**iPhone 06** (WER 8 %)

- Said: TSH 1,8 Milliunits pro Liter, Vitamin D 32 Nanogramm pro Milliliter, Ferritin 85.
- Recognized: TSH 1,8 mIunits pro Liter. Vitamin D 32 Nanogramm pro Milliliter. Ferritin 85.

**iPhone 07** (WER 0 %)

- Said: Kontrolle beim Orthopäden am 30. April 2026. Das Knie ist ohne Beschwerden, Ibuprofen nur noch bei Bedarf.
- Recognized: Kontrolle beim Orthopäden am 30. April 2026. Das Knie ist ohne Beschwerden, Ibuprofen nur noch bei Bedarf.

**iPhone 08** (WER 11 %)

- Said: Gesprochener Arztbrief nach dem Besuch bei Dr. Erika Beispiel am 12. November 2026. Seit zwei Wochen Schmerzen im rechten Knie nach einem Sturz beim Laufen. Leichte Schwellung, kein Erguss im Ultraschall. Empfehlung: Ibuprofen 400 Milligramm bis zu dreimal täglich nach dem Essen, für höchstens eine Woche, dazu Kühlen und Schonung. Kontrolle in drei Wochen, bei Verschlechterung früher. Überweisung zur Physiotherapie mit sechs Einheiten.
- Recognized: Gesprochener Arztbrief. Nach dem Besuch bei Dr. Erika Beispiel am 12. November 2026 seit zwei Wochen Schmerzen im rechten Knie nach einem Sturz beim Laufen. Leichte Schwellung, keiner Guss im Ultraschall. Empfehlung Ibuprofen 400 mg bis zu drei Mal täglich nach dem Essen. Für höchstens eine Woche. Dazu Kühlen und Schonung. Kontrolle in drei Wochen bei Verschlechterung früher. Überweisung zur Physiotherapie mit Sexeinheiten

**iPhone 10** (WER 8 %)

- Said: Blutdruck heute Morgen 122 zu 79, Puls 64, zu Hause im Sitzen gemessen.
- Recognized: Blutdruck heute Morgen 122 zu 79 plus 64 zu Hause im Sitzen gemessen.

**Anna (say) 01** (WER 18 %)

- Said: Laborbefund vom 20. Jänner 2026, Labor Muster GmbH. Gesamtcholesterin 232 Milligramm pro Deziliter, LDL 154, HDL 48.
- Recognized: Laborbefund vom 20. Jänner 2026, Labormuster GmbH, Gesamtcholesterin 232 mg pro Deziliter, LDL 154, HDL 48.

**Anna (say) 02** (WER 31 %)

- Said: Rezept vom 18. Mai 2026: Musterol 400 Milligramm, eins null eins, 20 Stück.
- Recognized: Rezept vom 18. Mai 2026, Musterol 400 mg, 101, 20 Stück.

**Anna (say) 03** (WER 0 %)

- Said: Blutdruck heute Morgen 122 zu 79, Puls 64, zu Hause im Sitzen gemessen.
- Recognized: Blutdruck heute Morgen 122 zu 79, Puls 64, zu Hause im Sitzen gemessen.

**Anna (say) 04** (WER 8 %)

- Said: Überweisung zur Physiotherapie, sechs Einheiten, Diagnose Gonarthrose rechts, von Dr. Erika Beispiel, Allgemeinmedizin.
- Recognized: Überweisung zur Physiotherapie, 6 Einheiten, Diagnose Gonarthrose rechts, von Dr. Erika Beispiel, Allgemeinmedizin.

**Anna (say) 05** (WER 0 %)

- Said: FSME-Auffrischung am 10. Juni 2026, die nächste Impfung ist im Juni 2029 fällig.
- Recognized: FSME-Auffrischung am 10. Juni 2026, die nächste Impfung ist im Juni 2029 fällig.

**Anna (say) 06** (WER 8 %)

- Said: TSH 1,8 Milliunits pro Liter, Vitamin D 32 Nanogramm pro Milliliter, Ferritin 85.
- Recognized: TSH 1,8 Millionits pro Liter, Vitamin D 32 Nanogramm pro Milliliter, Ferritin 85.

**Anna (say) 07** (WER 0 %)

- Said: Kontrolle beim Orthopäden am 30. April 2026. Das Knie ist ohne Beschwerden, Ibuprofen nur noch bei Bedarf.
- Recognized: Kontrolle beim Orthopäden am 30. April 2026. Das Knie ist ohne Beschwerden, Ibuprofen nur noch bei Bedarf.

**Anna (say) 08** (WER 3 %)

- Said: Gesprochener Arztbrief nach dem Besuch bei Dr. Erika Beispiel am 12. November 2026. Seit zwei Wochen Schmerzen im rechten Knie nach einem Sturz beim Laufen. Leichte Schwellung, kein Erguss im Ultraschall. Empfehlung: Ibuprofen 400 Milligramm bis zu dreimal täglich nach dem Essen, für höchstens eine Woche, dazu Kühlen und Schonung. Kontrolle in drei Wochen, bei Verschlechterung früher. Überweisung zur Physiotherapie mit sechs Einheiten.
- Recognized: Gesprochene Arztbrief nach dem Besuch bei Dr. Erika Beispiel am 12. November 2026. Seit zwei Wochen Schmerzen im rechten Knie nach einem Sturz beim Laufen. Leichte Schwellung, kein Erguss im Ultraschall. Empfehlung, Ibuprofen 400 mg bis zu dreimal täglich nach dem Essen, für höchstens eine Woche, dazu Kühlen und Schonung. Kontrolle in drei Wochen, bei Verschlechterung früher. Überweisung zur Physiotherapie mit sechs Einheiten.

**generated 03 + noise** (WER 15 %)

- Said: Blutdruck heute Morgen 122 zu 79, Puls 64, zu Hause im Sitzen gemessen.
- Recognized: Blue-Druck heute Morgen 122 zu 79, Puls 64, zu Hause im Sitten gemessen.

