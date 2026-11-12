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
photos (3,000–4,000 pixels) are therefore read as they are.

## 2026-11-11: a real phone photo (of a screen, not paper)

Result: [2026-11-11-tesseract-phone.md](2026-11-11-tesseract-phone.md).

Lab report 01 was photographed with an iPhone 14 Pro Max (2268 × 4032
pixels, JPEG): `demo/phone/01_lab_report_2026-01-20_phone.jpg`. As it turned
out later, the document was shown **on a screen**, not printed on paper.
Before it went into the repository, its metadata (EXIF with the GPS
position, XMP) was removed; the image data is unchanged byte for byte.

| Variant | Match | Facts (value + unit) | Facts (value only) | Mean OCR confidence | Time |
|---|---|---|---|---|---|
| Phone photo (real) | **100 %** | 5/5 | 5/5 | 91 | 0.7 s |
| Demo photo of the same document (upscaled) | 57 % | 2/5 | 5/5 | 92 | 0.6 s |

The real photo has enough pixels, so it is not scaled, and every unit is
read as `mg/dl`; on the synthetic photo of the same document three of them
are `mgj/dl`. The measurement now always includes `demo/phone/` (variant
`phone`, its own row "Phone photo (real)" in the summary).

**Decision (Dogan, 2026-11-11): Tesseract is enough for now.** PaddleOCR
only if further real photos do clearly worse. One photo is a small sample
(a screen, held straight); worth adding next: photos of printed paper, in
poor light, a crumpled page, one taken at an angle.

The same day, `go run ./cmd/ocrcheck -recheck -write` checked every stored
entry again with the upscaling: only the eye test uploaded by the app (L3)
changed, from 3 of 6 to 6 of 6 confirmed.

## 2026-11-11: every image prepared the same way (HEIC, orientation, size)

Result: [2026-11-11-tesseract-prepare.md](2026-11-11-tesseract-prepare.md).

Before, Tesseract got the uploaded bytes (only small images were scaled).
HEIC, the iPhone's default format, could not be read at all, and a photo
stored lying on its side with an EXIF orientation was read sideways. Now
`ocr.PrepareImage` turns every photo (JPEG, PNG, HEIC, WebP) into one
format before OCR: decoded in Go, turned upright, put on white where it is
transparent, scaled to a long side between 2500 and 4096 pixels, and
written as an RGB PNG. Tesseract gets `--dpi 300`. PDF pages rendered by
`pdftoppm` are already such PNGs and are not prepared again.

What the measurement showed on the way:

1. **The resolution must be given.** The first version (grey PNG) read the
   real phone JPEG worse: 86 % instead of 100 %. In colour as well. Cause:
   a PNG from Go carries no resolution, the JPEG said 300 dpi. Without one,
   Tesseract estimates it ("Estimating resolution as 441"), misread
   `mgj/dl` and dropped the reference column. With `--dpi` 200, 300 or 400
   it reads everything; 300 was chosen (exact for an A4 page at 3508
   pixels).
2. **Colour, not grey.** With `--dpi 300`, a grey image read the demo
   photos worse (90 % instead of 93 %, 29/36 instead of 31/36 facts) and the
   HEIC photo much worse (29 % instead of 57 %).
3. **Do not scale 12-megapixel photos down.** On the phone JPEG
   (4032 pixels): limit 2500 → 86 %, 3000 → 57 %, 3508 → 71 %, 4096 (not
   scaled) → 100 %. So the limit is 4096: a usual 12-megapixel photo is
   read as it is, only larger ones (48 megapixels, 8064 pixels) are
   halved. Larger ones are not measured yet.
4. **The HEIC photo** (`demo/phone/01_lab_report_2026-01-20_phone.heic`, a
   second photo of the same screen, slightly at an angle, with moiré):
   upright and read, 57 % (date and issuer, 2 of 5 facts). Not
   confirmed: the row „Gesamtcholesterin“ is not read at all, the value of
   LDL is missing from its row, and one unit is `mog/dl`. The same file
   decoded by Apple (`sips`) and turned upright gives the same kind of
   mistakes (`mg/dil`, `mg/di`, the name „Gesamtcholesterin“ missing): the
   photo is the cause, not the decoder.
5. **Orientation:** a demo photo stored lying on its side with EXIF
   orientation 6 is confirmed completely; without turning it, not a single
   field (test `TestPipeline_JPEG_EXIFOrientation`).

| Variant | Before | Now |
|---|---|---|
| PDF, text layer | 100 % | 100 % |
| PDF, OCR (forced) | 98 % | 98 % |
| Demo photos | 93 % (31/36) | 93 % (31/36) |
| Phone JPEG | 100 % | 100 % |
| Phone HEIC | not checked (`unsupported_file_type`) | 57 % |

Time per photo: phone JPEG 1.6 s (before 0.7 s: decoding and the PNG cost
time), HEIC 4.1 s (decoding with WebAssembly takes about 1.1 s), demo
photos 0.5 s. All within the time of the vision LLM and far below the
limit of 30 s.

HEIC is decoded by `github.com/gen2brain/heic` v0.7.2 (MIT): a HEVC
decoder compiled to WebAssembly and run by `wazero`, so it needs no cgo and
no program on the server, and works the same on macOS, Linux and Windows.
It would prefer a libheif installed on the machine; the backend switches
that off (`heic.ForceWasmMode`), so all machines decode the same way.

## 2026-11-12: a real phone photo of paper

Result: [2026-11-12-tesseract-paper.md](2026-11-12-tesseract-paper.md).

The two earlier phone photos (`demo/phone/*_phone.jpg`, `*_phone.heic`)
**show the document on a screen, not on paper**; they are labelled
"phone, screen" in the measurement and counted as "Phone photo of a screen
(real)". The first photo of real paper: lab report 01, printed,
photographed upright with an iPhone 14 Pro Max on a table:
`demo/phone/01_lab_report_2026-01-20_paper.jpg` (variant `_paper`, "Phone
photo of paper (real)"). The location had been removed before (checked: no
GPS in EXIF or XMP). The photo was then **cropped to the sheet**
(2268 × 4032 → 2188 × 3104 pixels), because the edges showed the room.
The crop is lossless (`jpegtran -crop`, the remaining pixels are identical
to the original) and drops all metadata (`-copy none`).

| Variant | Match | Facts (value + unit) | Facts (value only) | Mean OCR confidence | Time |
|---|---|---|---|---|---|
| Phone photo of paper, cropped to the sheet | 86 % | 4/5 | **5/5** | 87 | 1.5 s |
| the same photo before cropping | 71 % | 3/5 | **5/5** | 92 | 1.8 s |
| Phone photo of a screen, JPEG | 100 % | 5/5 | 5/5 | 91 | 1.6 s |
| Phone photo of a screen, HEIC | 57 % | 2/5 | 3/5 | 81 | 4.1 s |
| Demo photo (synthetic) | 57 % | 2/5 | 5/5 | 92 | 0.6 s |

**On paper every value is read correctly**, and so are date and issuer.
The only thing that fails is the unit: **`mg/dl` is read as `mgj/dl`**
(Triglyceride; before cropping also HDL-Cholesterin). Cropping does not
change a single pixel of the sheet, yet one confusion fewer: Tesseract
adapts its layout analysis and thresholds to the whole image, so which
`mg/dl` turns into `mgj/dl` varies. That it happens is systematic, not
chance: the
same confusion causes all five unconfirmed facts of the two synthetic lab
report photos (`demo/photos/01…`, `06…`). Presumably Tesseract reads the
descender of the `g` together with the `/` as a `j`. Apart from it, the
current measurement has only two other misread units: `mog/dl` on the
screen HEIC and `mUy/l` on a rendered PDF page.

Because the check is strict (value **and** unit, decision of 2026-11-11),
such a fact is `unconfirmed` although its value is right, and the entry
shows 86 % instead of 100 %.

### Open point: values with a misread unit

Two ways out, **neither is implemented**; to be decided together:

1. **Own status "value confirmed, unit uncertain"** (e.g.
   `value_confirmed`, and `corrected_value_confirmed` after a correction):
   the value was found in the row of its name, the unit next to it was
   not. Honest about what the second reader saw, and shows the user which
   part to look at. Costs: a new status in the API contract (docs/API.md,
   verification) that the app has to show, and a rule for how it counts in
   `confirmed` and `matchPercent` (probably not as confirmed, or as half).
2. **Tolerate this one confusion on purpose:** when comparing units, treat
   `gj` like `g/` (so `mgj/dl` matches `mg/dl`), as a fixed list of known
   OCR confusions in the normalization, not as general fuzzy matching.
   Fixes exactly the measured failures without an API change. Costs: the
   check becomes less strict (it would also accept a unit that really
   reads `mgj/dl`, which does not exist), and every further confusion
   needs a new list entry backed by measurement.

The measurement already shows how many facts fail only because of the
unit: column "Facts (value only)" in every report.

## Adding a real photo

Print a demo document, photograph it with a phone and save the photo
(JPEG, PNG, HEIC or WebP) named after the document with `_paper`, e.g.
`01_lab_report_2026-01-20_paper.jpg` (`_phone` is used for photos of the
document on a screen). **Check for the location before it
goes into `demo/phone/`** (`mdls <file> | grep -i -E 'latitude|gps'`):
on 2026-11-11 a HEIC sent by AirDrop still contained it although Location
was switched off in the share options.
To measure it once without adding it:

```sh
cd backend && go run ./cmd/ocrcheck -values -extra ~/Desktop/01_lab_report_2026-01-20_paper.jpg
```

It is checked against the same entry as the PDF and the demo photo of that
document.
