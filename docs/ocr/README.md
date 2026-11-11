# OCR measurements

Measurements of the second reader that checks extracted values
(docs/API.md, section 2, verification). Each file is generated with

```sh
cd backend && go run ./cmd/ocrcheck -values -out ../docs/ocr/<date>-<engine>.md
```

The command reads every file in `demo/` and checks it against the entry
that was uploaded from the same document (its current values). It calls no
model and writes nothing to the database without `-write`.

## 2026-11-11: Tesseract 5.5.4 (deu), Poppler 26.11.0

Result: [2026-11-11-tesseract.md](2026-11-11-tesseract.md).

| Variant | Match mean | Facts (value + unit) | Facts (value only) |
|---|---|---|---|
| PDF, text layer | 100 % | 27/27 | 27/27 |
| PDF, OCR (forced) | 98 % | 26/27 | 27/27 |
| Photo | 75 % (min 29 %) | 21/36 (58 %) | 28/36 (78 %) |
| Text import | 100 % | 4/4 | 4/4 |

What fails on photos:

1. **Units** in the two lab reports: `mg/dl` is read as `mg/di`, `mgjdl` or
   `mgjal`. The value is right, so these facts fail only because of the
   unit (the difference between the two columns).
2. **Missing values in tables:** the triglyceride value is missing in both
   lab report photos (`Triglyceride mg/dl`).
3. **Decimal commas** in the eye test: `1,0` is read as `10`, `1,2` as `12`.
4. **Letterhead:** in the referral photo, „Orthopädie am Fluss“ is not read.

PDFs are not a problem: the text layer confirms everything, and even OCR on
a rendered PDF page reaches 96 % (one `mUy/l` instead of `mU/l`).

**Resolution is a main cause.** The demo photos are about 1,000 × 1,400
pixels, roughly 120 dpi for an A4 page; Tesseract is made for about 300 dpi.
A quick test outside the backend (photos scaled to 2,800 pixels on the long
side with `sips`, then `tesseract`) read both lab reports correctly, units
and triglycerides included, except one `mgj/dl`. The eye test seemed to
lose its numbers when scaled; the measurement below shows that this was
only how plain `tesseract` prints table cells. The app uploads photos at their
original size (`demo/live/*_app.png` have the same size as the originals),
so real phone photos (3,000–4,000 pixels) are likely read better than these
synthetic ones. Real photos bring other problems instead (perspective,
shadows, blur), which the demo photos only imitate.

Options considered: (a) scale up small images before Tesseract, (b)
tolerate typical OCR confusions in units, (c) PaddleOCR. Decision (Dogan,
2026-11-11): try (a); (b) not for now, the check stays strict.

## 2026-11-11: scaling small images up before Tesseract

Result: [2026-11-11-tesseract-upscale.md](2026-11-11-tesseract-upscale.md).

The Tesseract adapter now scales an image whose long side is shorter than
`tesseract.DefaultMinLongSide` up to that size (Catmull-Rom), at most to
twice its size. Series on the demo photos (`go run ./cmd/ocrcheck -upscale
<n>`):

| Long side | Photo match mean (min) | Facts (value + unit) | Facts (value only) | PDF, forced OCR |
|---|---|---|---|---|
| not scaled | 75 % (29 %) | 21/36 (58 %) | 28/36 (78 %) | 96 % |
| 2000 | 93 % (57 %) | 31/36 (86 %) | 36/36 (100 %) | 96 % |
| **2500** | **93 % (57 %)** | **31/36 (86 %)** | **36/36 (100 %)** | 96 % |
| 3000 | 94 % (71 %) | 32/36 (89 %) | 34/36 (94 %) | 96 % |
| 3508 | 89 % (29 %) | 28/36 (78 %) | 36/36 (100 %) | 96 % |
| 4000 | 89 % (43 %) | 28/36 (78 %) | 36/36 (100 %) | 85 % |

Chosen: **2500**.

- At 2000 and 2500 every value of every photo is read. What still fails is
  the unit of five lab values, nearly always `mgj/dl` for `mg/dl`; the
  strict rule leaves them unconfirmed, as intended.
- At 3000 one more fact is confirmed, but the triglyceride row disappears
  from both lab report photos: losing a value is worse than misreading a
  unit. From 3508 on, results get worse again; at 4000 even rendered PDF
  pages (3508 pixels) suffer.
- **The eye test** is at 100 % from 2000 on (it was 50 %). Its numbers
  („1,0“, „1,2“) were not lost by scaling, as plain `tesseract` output
  suggested: Tesseract puts the table cells in blocks of their own, and the
  rows built from positions (core) join them again.
- **At most twice the size:** the 612 × 792 test image
  (`backend/testdata/lab_report.png`, 72 dpi) scaled to 2500 read the date
  as „14.18.2076“. Enlarging that much invents letter shapes. The demo
  photos need a factor of 1.8 and are not affected by the limit.
- Time per photo: about 0.5 s instead of 0.3 s, still well inside the time
  of the vision LLM.

Images with a long side of at least 2500 pixels are not touched; real phone
photos (3,000–4,000 pixels) are therefore read as they are. Next: measure a
real phone photo of a printed demo document (`-extra`, see below).

## Measuring a real photo

Print a demo document, photograph it with a phone and save the photo (JPEG
or PNG; HEIC is not supported, see API.md) named after the document with
`_phone`, e.g. `01_lab_report_2026-01-20_phone.jpg`. Then:

```sh
cd backend && go run ./cmd/ocrcheck -values -extra ~/Desktop/01_lab_report_2026-01-20_phone.jpg
```

It is checked against the same entry as the PDF and the demo photo of that
document.
