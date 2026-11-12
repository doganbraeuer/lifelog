package main

import (
	"fmt"
	"strings"
)

// report is the measurement as Markdown (docs/speech/).
type report struct {
	Date      string
	Engine    string // "whisper.cpp large-v3-turbo"
	Converter string // "ffmpeg 8.1.1"
	Machine   string // "Apple M3 Pro, darwin/arm64"
	Rows      []row
}

// sourceSummary sums up the single recordings of one source (not the
// joined one, so no sentence counts twice).
type sourceSummary struct {
	Source             string
	Recordings         int
	Speech, SpeechText int // with speech / of those gave text
	Silent, SilentOK   int // without speech / of those gave NO_SPEECH
	WERSum             float64
	Terms, Found       int
	MaxRTF             float64
}

func (s sourceSummary) termsPercent() float64 {
	if s.Terms == 0 {
		return 0
	}
	return 100 * float64(s.Found) / float64(s.Terms)
}

func (s sourceSummary) meanWER() float64 {
	if s.Speech == 0 {
		return 0
	}
	return s.WERSum / float64(s.Speech)
}

func summarize(rows []row) []sourceSummary {
	var out []sourceSummary
	index := map[string]int{}
	for _, r := range rows {
		if r.Joined {
			continue
		}
		i, ok := index[r.Source]
		if !ok {
			i = len(out)
			index[r.Source] = i
			out = append(out, sourceSummary{Source: r.Source})
		}
		s := &out[i]
		s.Recordings++
		if r.expectsSpeech() {
			s.Speech++
			if r.ok() {
				s.SpeechText++
			}
			s.WERSum += r.WER
			s.Terms += r.Terms
			s.Found += r.Found
		} else {
			s.Silent++
			if r.ok() {
				s.SilentOK++
			}
		}
		s.MaxRTF = max(s.MaxRTF, r.rtf())
	}
	return out
}

// Markdown renders the report.
func (rep report) Markdown() string {
	var b strings.Builder
	p := func(format string, a ...any) { fmt.Fprintf(&b, format, a...) }

	p("# Speech recognition: %s\n\n", rep.Engine)
	p("Generated on %s by `go run ./cmd/speechcheck` (docs/speech/README.md). ", rep.Date)
	p("Every recording goes through the same pipeline as `POST /api/v1/transcriptions` ")
	p("(conversion, length and loudness check, Whisper, cleaning). No language model is called, nothing is stored.\n\n")
	p("- Engine: %s, started with `backend/scripts/whisper-server.sh`\n", rep.Engine)
	p("- Converter: %s\n", rep.Converter)
	p("- Machine: %s\n", rep.Machine)
	p("- Key terms are compared word by word (`core.TextContains`): case, punctuation at the edges and decimal comma ")
	p("do not count, word boundaries do („keiner Guss“ is not „kein Erguss“, „101“ is not „1-0-1“). ")
	p("Each term lists the spellings that count as right (`cmd/speechcheck/samples.go`).\n")
	p("- WER (word error rate) is strict: „400 mg“ for „400 Milligramm“ or „6“ for „sechs“ counts as an error. ")
	p("The key terms are the measure that matters.\n\n")

	p("## Summary\n\n")
	p("| Source | Recordings | Speech → text | WER (mean) | Key terms found | No speech → NO_SPEECH | Real-time factor (max) |\n")
	p("|---|---|---|---|---|---|---|\n")
	sums := summarize(rep.Rows)
	for _, s := range sums {
		speech, terms, wer, silent := "–", "–", "–", "–"
		if s.Speech > 0 {
			speech = fmt.Sprintf("%d/%d", s.SpeechText, s.Speech)
			wer = fmt.Sprintf("%.1f %%", 100*s.meanWER())
			terms = fmt.Sprintf("%d/%d (%.0f %%)", s.Found, s.Terms, s.termsPercent())
		}
		if s.Silent > 0 {
			silent = fmt.Sprintf("%d/%d", s.SilentOK, s.Silent)
		}
		p("| %s | %d | %s | %s | %s | %s | %.2f |\n", s.Source, s.Recordings, speech, wer, terms, silent, s.MaxRTF)
	}
	p("\n")

	p("## Criteria\n\n")
	for _, s := range sums {
		if s.Speech == 0 || s.Source == sourceGenerated {
			continue // "03 + noise" is a single stress test, see below
		}
		p("- %s Key terms, %s: %.0f %% (at least %d %%)\n", mark(s.termsPercent() >= minTermsPercent), s.Source, s.termsPercent(), minTermsPercent)
	}
	var invented []string
	silent := 0
	for _, r := range rep.Rows {
		if !r.expectsSpeech() {
			silent++
			if !r.ok() {
				invented = append(invented, fmt.Sprintf("%s %s: %s „%s“", r.Source, r.Name, r.Result, r.Text))
			}
		}
	}
	p("- %s No text without speech: %d of %d recordings without speech gave NO_SPEECH", mark(len(invented) == 0), silent-len(invented), silent)
	if len(invented) > 0 {
		p(" (%s)", strings.Join(invented, "; "))
	}
	p("\n")
	for _, r := range rep.Rows {
		if r.Joined {
			est := r.rtf() * threeMinutes
			p("- %s Speed, %s: %.0f s of speech in %.1f s, so 3 minutes take about %.0f s (at most %d s)\n",
				mark(est <= maxSecondsFor3Min), r.Source, r.Seconds, r.Elapsed.Seconds(), est, maxSecondsFor3Min)
		}
	}
	p("\n")

	p("## Recordings without speech\n\n")
	p("Which check gave NO_SPEECH, and what Whisper writes for the same audio without the loudness check and the cleaning ")
	p("(sent to whisper-server directly, with its own voice activity detection).\n\n")
	p("| Source | Recording | What | Caught by | Whisper alone |\n|---|---|---|---|---|\n")
	for _, r := range rep.Rows {
		if !r.expectsSpeech() {
			caught := r.CaughtBy
			if r.Result != "NO_SPEECH" {
				caught = "nothing: " + r.Result
			}
			p("| %s | %s | %s | %s | %s |\n", r.Source, r.Name, r.What, cell(caught), cell(r.WhisperAlone))
		}
	}
	p("\n")

	p("## Per recording\n\n")
	p("| Source | Recording | What | Length | Time | RTF | Result | WER | Key terms | Missing |\n")
	p("|---|---|---|---|---|---|---|---|---|---|\n")
	for _, r := range rep.Rows {
		wer, terms := "–", "–"
		if r.expectsSpeech() {
			wer = fmt.Sprintf("%.0f %%", 100*r.WER)
			terms = fmt.Sprintf("%d/%d", r.Found, r.Terms)
		}
		p("| %s | %s | %s | %.1f s | %.1f s | %.2f | %s | %s | %s | %s |\n",
			r.Source, r.Name, r.What, r.Seconds, r.Elapsed.Seconds(), r.rtf(), r.Result, wer, terms, cell(strings.Join(r.Missing, ", ")))
	}
	p("\n")

	p("## Recognized text\n\n")
	p("Only single recordings with speech (the joined ones contain the same sentences).\n\n")
	for _, r := range rep.Rows {
		if r.Joined || !r.expectsSpeech() {
			continue
		}
		p("**%s %s** (WER %.0f %%)\n\n", r.Source, r.Name, 100*r.WER)
		p("- Said: %s\n", r.Ref)
		text := r.Text
		if r.Result != "text" {
			text = "(" + r.Result + ")"
		}
		p("- Recognized: %s\n\n", text)
	}
	return b.String()
}

func mark(ok bool) string {
	if ok {
		return "✅"
	}
	return "❌"
}

// cell escapes text for a Markdown table cell.
func cell(s string) string {
	if s == "" {
		return "–"
	}
	return strings.ReplaceAll(s, "|", `\|`)
}
