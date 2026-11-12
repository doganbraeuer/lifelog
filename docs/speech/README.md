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
| generated | 10 s digital silence, 10 s white noise at -30 dBFS, Anna's sentence 03 with that noise | made by `speechcheck` at run time, with a fixed seed |
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

Proposed criteria (plan for spoken notes): key terms at least 95 %, no text
without speech, 3 minutes of speech in less than 30 s on the M3 Pro.

## 2026-11-12: whisper.cpp 1.9.5, large-v3-turbo, M3 Pro

Result: [2026-11-12-whisper.md](2026-11-12-whisper.md).

| Source | Speech → text | Key terms found | WER (mean) | No speech → NO_SPEECH | Real-time factor |
|---|---|---|---|---|---|
| iPhone | 9/9 | 44/51 (86 %) | 9.6 % | 1/1 | 0.04 (joined) to 0.12 |
| Anna (say) | 8/8 | 45/47 (96 %) | 8.4 % | – | 0.05 (joined) to 0.18 |
| generated | 1/1 | 2/4 (stress test) | 15.4 % | 2/2 | – |

Criteria:

- ✅ **No text without speech** (3 of 3). Digital silence is stopped by our
  loudness check before Whisper is asked. The quiet room (iPhone 09) and the
  white noise pass the loudness check, and Whisper's voice activity
  detection finds no speech in them. Whisper alone (with its voice activity
  detection) also wrote nothing for all three; our cleaning of known
  invented sentences was not needed here and stays as a safety net.
- ✅ **Speed:** 145 s of speech in 5.5 s, so 3 minutes take about 7 s; a
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

Options (decision open):

- (a) Give Whisper a short list of expected words (`prompt` field of
  whisper-server, e.g. „Puls, Gonarthrose, Erguss, 1-0-1, mg/dl, mU/l“) and
  measure again. Usually helps with rare words; risk: words from the prompt
  appear in the text although they were not said, which the measurement
  would show.
- (b) Keep it as it is and rely on the edit step in the app.
- (c) The larger model large-v3 (about 3 GB, slower): little disk space
  left, not measured.

Limits of this measurement: one speaker, eight sentences, a quiet room, one
phone. Whisper was not measured without its voice activity detection.

## Recordings

Voice memos of iOS 27 contain, besides the audio, a time stamp, a UUID and
**Apple's own transcript** of the recording (atom `tsrp`); no location. The
backend ignores all of them (ffmpeg only reads the audio track), and the
own recordings stay outside the repository. The synthetic recordings in
`demo/speech/` carry no metadata.
