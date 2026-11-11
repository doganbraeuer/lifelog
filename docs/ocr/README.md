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
and triglycerides included, except one `mgj/dl`. The eye test, however,
lost its numbers completely when scaled. The app uploads photos at their
original size (`demo/live/*_app.png` have the same size as the originals),
so real phone photos (3,000–4,000 pixels) are likely read better than these
synthetic ones. Real photos bring other problems instead (perspective,
shadows, blur), which the demo photos only imitate.

Options, not decided yet:

- **(a) Scale up small images before Tesseract** (e.g. to a long side of
  2,500–3,500 pixels). Cheap, no new program; measure with this command
  first, because it can also make things worse (eye test).
- **(b) Tolerate typical OCR confusions in units** (`l`/`i`/`1`, `/`/`j`).
  Raises the rate at once, but a tolerant comparison can also confirm a
  value that is wrong. Only for units, never for values.
- **(c) PaddleOCR** as a second engine (`internal/ocr` is prepared for it).
  Needs Python and a model download on the server; worth measuring if (a)
  does not reach about 90 % on photos.
