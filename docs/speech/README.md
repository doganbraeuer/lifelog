# Speech recognition measurements

Measurements of the transcription (docs/API.md, 3.9): how well whisper.cpp
recognizes spoken notes, how fast it is, and whether silence and noise give
`NO_SPEECH` instead of invented text. Each file is generated with

```sh
backend/scripts/whisper-server.sh        # in its own terminal
cd backend && go run ./cmd/speechcheck -own ~/Projekte/lifelog-demo/speech -out ../docs/speech/<date>-whisper.md
```

Without `-own`, only the synthetic voice and the generated audio are
measured; everybody can run that. The command sends every recording through
the same pipeline as `POST /api/v1/transcriptions`, calls no language model
and stores nothing.

## What is measured

The eight sentences in `demo/speech/01.txt` … `08.txt` (synthetic content in
the style of the demo documents: lab values, a prescription, blood pressure,
a referral, a vaccination, a follow-up and a spoken doctor's letter of about
40 seconds), in four kinds of recordings:

| Source | Recordings | Where |
|---|---|---|
| iPhone | 01–08 read aloud by Dogan as voice memos (AAC/M4A, 48 kHz), 09 silence in a quiet room, 10 sentence 03 with background noise | `~/Projekte/lifelog-demo/speech/`, **not in the repository**: a voice is personal |
| Anna (say) | 01–08 spoken by the macOS voice "Anna" | `demo/speech/NN_anna.m4a`, made by `demo/speech/make-voice.sh` |
| generated | 10 s each: digital silence, white noise at -30 and -40 dBFS, brown noise (low rumble) at -30 dBFS; Anna's sentence 03 with the loud white noise | made by `speechcheck` at run time, with a fixed seed |
| joined | per voice, 01–08 one after another in one recording (2.4 min for the iPhone) | made at run time, for the speed of a longer note |

Measures:

- **Key terms found**, the measure that matters: per sentence the words the
  app needs later (values with units, dates, names, medication, dose
  scheme), compared word by word (`core.TextContains`). Every accepted
  spelling is listed in `backend/cmd/speechcheck/samples.go`, e.g. „400 mg“
  for „400 Milligramm“ or „6 Einheiten“ for „sechs Einheiten“; „101“ does
  not count for „eins null eins“.
- **Word error rate** (WER, `core.WordErrorRate`): strict, so „400 mg“ for
  „400 Milligramm“ already counts as two errors. Only for comparing runs.
- **Real-time factor:** processing time per second of recording, including
  conversion and checks.
- **Recordings without speech** must give `NO_SPEECH`. For these the report
  also shows which check caught them and what Whisper writes for the same
  audio without our loudness check and cleaning.
- **Terms of the Whisper prompt** (`whisper.Vocabulary`, see below) must
  not appear in a result unless they were said, neither in recordings
  with speech nor in silence and noise (there also in what Whisper alone
  writes).

Proposed criteria (plan for spoken notes): key terms at least 95 %, no text
without speech, 3 minutes of speech in less than 30 s on the M3 Pro.

## 2026-11-12: whisper.cpp 1.9.5, large-v3-turbo, M3 Pro

Result: [2026-11-12-whisper.md](2026-11-12-whisper.md) (server with
`--max-context 0`, no prompt; generated again after the measurement got the
extra noise and the prompt check, with the same results for the recordings
it had before).

| Source | Speech → text | Key terms found | WER (mean) | No speech → NO_SPEECH | Real-time factor |
|---|---|---|---|---|---|
| iPhone | 9/9 | 44/51 (86 %) | 9.6 % | 1/1 | 0.04 (joined) to 0.12 |
| Anna (say) | 8/8 | 45/47 (96 %) | 8.4 % | – | 0.05 (joined) to 0.18 |
| generated | 1/1 | 2/4 (stress test) | 15.4 % | 4/4 | – |

Criteria:

- ✅ **No text without speech** (5 of 5). Digital silence is stopped by our
  loudness check before Whisper is asked. The quiet room (iPhone 09) and the
  three noises pass the loudness check, and Whisper's voice activity
  detection finds no speech in them. Whisper alone (with its voice activity
  detection) also wrote nothing for any of them; our cleaning of known
  invented sentences was not needed here and stays as a safety net.
- ✅ **Speed:** 145 s of speech in about 5.5 s, so 3 minutes take about 7 s; a
  short note of 12 s takes about 1.3 s. A spoken doctor's letter of a few
  minutes is far below the time limit of 120 s.
- ❌ **Key terms with a real voice: 86 %** (target 95 %); with the
  synthetic voice 96 %.

**Every number, value and date was recognized correctly**, with both voices:
232, LDL 154, HDL 48, 122 zu 79, 64, TSH 1,8, 32, 85, 400 mg, 20 Stück and
all six dates („20. Jänner 2026“ as written). The misses are words around
them:

| Said | Recognized | Recording |
|---|---|---|
| eins null eins | 101 | iPhone 02, Anna 02 |
| Puls 64 | plus 64 | iPhone 03 and 10 |
| Gonarthrose | Gonarrose | iPhone 04 |
| Milliunits pro Liter | mIunits / Millionits pro Liter | iPhone 06, Anna 06 |
| kein Erguss | keiner Guss | iPhone 08 |
| sechs Einheiten | Sexeinheiten | iPhone 08 (correct in the joined recording) |

Stress test: Anna's sentence 03 with loud white noise (-30 dBFS) still has
every number right, but „Blue-Druck“ for „Blutdruck“ and „Sitten“ for
„Sitzen“.

What this means: the text is good enough to be **corrected by the user**,
which is the plan anyway (the app shows the text for editing before it is
saved, API.md 3.9). It is not good enough to be saved unchecked. The dose
scheme is the most important miss: „101“ looks like a number and could be
read as 101 pieces.

Options:

- (a) Give Whisper a list of expected words (`prompt` field of
  whisper-server) and measure again. Usually helps with rare words; risk:
  words from the prompt appear in the text although they were not said,
  which the measurement would show.
- (b) Keep it as it is and rely on the edit step in the app.
- (c) The larger model large-v3 (about 3 GB, slower): little disk space
  left, not measured.

Decision (Dogan, 2026-11-12): (a), with a general list of terms, **not**
taken from the eight test sentences (otherwise the measurement only shows
that Whisper finds the list again); the report marks which terms happen to
be in them. See the next section.

Limits of this measurement: one speaker, eight sentences, a quiet room, one
phone. Whisper was not measured without its voice activity detection.

## 2026-11-12: list of medical terms as prompt

Results: [2026-11-12-whisper-context.md](2026-11-12-whisper-context.md)
(new server settings, no prompt) and
[2026-11-12-whisper-vocabulary.md](2026-11-12-whisper-vocabulary.md) (new
server settings with the list, as the backend now runs).

**The list** (`backend/internal/speech/whisper/vocabulary.go`, 45 terms) was
written by category from general knowledge of the domain: vital signs, lab
values and their abbreviations, units, common active substances, dose
schemes (1-0-1, 1-0-0, 0-0-1, 1-1-1, bei Bedarf), common diagnoses,
examinations. It contains no example values (a number in the prompt could
appear in a transcript; a test checks this). Whisper uses at most 224
prompt tokens and drops the beginning of a longer prompt: a first draft of
97 terms had about 414 tokens and was cut to about 205 (counted with the
model's own vocabulary).

**Terms that are also said in the test sentences: 18 of 45**, counted also
when the list has the written form of what was said: Blutdruck (03), Puls
(03), LDL, HDL, mg/dl (01), TSH, Ferritin, Vitamin D, ng/ml, mU/l (06),
Ibuprofen (07, 08), 1-0-1 (02), bei Bedarf (07), Gonarthrose (04), Erguss,
Ultraschall (08), Physiotherapie (04, 08), FSME (05). These are typical
words of the domain, which is why the sentences contain them too; but an
improvement on exactly these words is what any list containing them would
give, so it says little about words the list does not contain. The other 27
terms (mmHg, HbA1c, Kreatinin, Ramipril, Metformin, 1-1-1, Hypertonie, …)
are not tested by these sentences at all.

**Server setting.** The prompt has no effect with `--max-context 0`, not
even with `--carry-initial-prompt` (tested by hand: „101“ stays „101“).
whisper.cpp counts the prompt as text context, and `0` allows none. The
start scripts now use `--carry-initial-prompt` without `--max-context 0`:
the prompt is used for every 30-second window, and Whisper also sees the
text it recognized before. `--max-context 0` was set against repeated
sentences; the joined recordings (145 s and 98 s) show no repetition with
the new setting, and the cleaning still folds loops.

| Setting | Key terms iPhone | Key terms Anna | Stress test | WER joined iPhone / Anna | Terms of the list not said | No speech → NO_SPEECH |
|---|---|---|---|---|---|---|
| `--max-context 0`, no prompt (before) | 44/51 (86 %) | 45/47 (96 %) | 2/4 | 10 % / 9 % | 0 | 5/5 |
| context, no prompt | 44/51 (86 %) | 45/47 (96 %) | 2/4 | 7 % / 6 % | 0 | 5/5 |
| context and list (now) | **46/51 (90 %)** | 45/47 (96 %) | 3/4 | 11 % / 12 % | **0** | **5/5** |

The context alone changes no key term, and the single sentences are word
for word the same; only the long recordings differ (WER 10 → 7 % and
9 → 6 %; their texts are not in the report). The list changes these words:

| Said | Without the list | With the list | Recording | In the list |
|---|---|---|---|---|
| eins null eins | 101 | **1-0-1** | iPhone 02, Anna 02 | yes |
| Gonarthrose | Gonarrose | **Gonarthrose** | iPhone 04 | yes |
| Blutdruck (loud noise) | Blue-Druck | **Blutdruck** | stress test | yes |
| Ferritin | Ferritin | **Feritin** (worse) | Anna 06 | yes |
| Milliunits pro Liter | Millionits pro Liter | **mL pro Liter** (worse) | Anna 06 | mU/l |
| Nanogramm pro Milliliter | Nanogramm pro Milliliter | ng pro ml | iPhone 06, Anna 06 | ng/ml |

Unchanged misses: „plus 64“ for „Puls 64“ (although „Puls“ is in the list),
„keiner Guss“, „mIunits“, „Sexeinheiten“. The higher WER comes from
abbreviations („ng pro ml“, „mg pro dL“), which are right but count as
errors in the strict WER.

Criteria:

- ✅ **No term of the list without being said**, in all 25 recordings:
  silence, three noises (also what Whisper alone writes) and all sentences.
- ✅ No text without speech (5 of 5), speed unchanged (3 minutes in about
  7 s).
- ❌ Key terms with a real voice 90 % (target 95 %).

Assessment: the list fixes the dose scheme, the most important miss, with
both voices, and adds no words that were not said. It also brings one new
kind of error that is **worse than before**: „mL pro Liter“ is a real unit
with another meaning (millilitres), while „Millionits“ was obviously wrong.
A list of units makes Whisper prefer units it knows. The text therefore
still needs the user's check in the app, as planned; the list stays on
because the gain (dose scheme, diagnosis) is larger than the loss, and
`go run ./cmd/speechcheck -vocabulary=false` measures without it.

Limits as above; in addition, only 18 of the 45 terms are tested by the
sentences, three of them got better and two worse.

## Recordings

Voice memos of iOS 27 contain, besides the audio, a time stamp, a UUID and
**Apple's own transcript** of the recording (atom `tsrp`); no location. The
backend ignores all of them (ffmpeg only reads the audio track), and the
own recordings stay outside the repository. The synthetic recordings in
`demo/speech/` carry no metadata.
