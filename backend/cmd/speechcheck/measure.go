package main

import (
	"context"
	"errors"
	"fmt"
	"math"
	"math/rand/v2"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"time"

	"lifelog/backend/internal/api"
	"lifelog/backend/internal/core"
	"lifelog/backend/internal/speech"
)

// Sources of the recordings.
const (
	sourceOwn       = "iPhone"     // voice memos of a person, outside the repository
	sourceVoice     = "Anna (say)" // synthetic voice, demo/speech/*_anna.m4a
	sourceGenerated = "generated"  // made here: silence, noise, speech with noise
)

const (
	// generatedSeconds: length of generated silence and noise.
	generatedSeconds = 10
	// noiseDBFS: level of the generated noise. Well above the silence
	// threshold of core.SpeechSeconds, so Whisper is really asked and its
	// voice activity detection and the cleaning have to catch it.
	noiseDBFS = -30
	// noiseSeed: the noise is the same on every run.
	noiseSeed = 2026
	// pauseSeconds: pause between the sentences of a joined recording.
	pauseSeconds = 0.5
	// threeMinutes: the plan asks that 3 minutes take less than 30 s.
	threeMinutes      = 180
	maxSecondsFor3Min = 30
	// minTermsPercent: key terms that must be recognized.
	minTermsPercent = 95
)

// recording is one file the measurement sends through the pipeline.
type recording struct {
	Source string
	Name   string // "01", "silence", "01–08 joined"
	What   string
	Ref    string // reference text; "" = nothing is said, must give NO_SPEECH
	Terms  []term
	Joined bool // several sentences in one recording: no text in the report
	Audio  []byte
	Mime   string
}

// row is the result for one recording.
type row struct {
	Source, Name, What, Ref string
	Joined                  bool
	Seconds                 float64       // length of the recording
	Elapsed                 time.Duration // time for the whole pipeline
	Result                  string        // "text", "NO_SPEECH", "UNREADABLE", "AUDIO_TOO_LONG"
	Text                    string
	WER                     float64
	Terms                   int
	Found                   int
	Missing                 []string

	// Only for recordings without speech:
	CaughtBy     string // which check gave NO_SPEECH, from the error
	WhisperAlone string // what Whisper writes without our checks
}

// rtf is the real-time factor: processing time per second of recording.
func (r row) rtf() float64 {
	if r.Seconds == 0 {
		return 0
	}
	return r.Elapsed.Seconds() / r.Seconds
}

// expectsSpeech: the recording has something said in it.
func (r row) expectsSpeech() bool { return r.Ref != "" }

// ok: a silent recording gave NO_SPEECH; one with speech gave text.
func (r row) ok() bool {
	if r.expectsSpeech() {
		return r.Result == "text"
	}
	return r.Result == "NO_SPEECH"
}

// references reads demo/speech/NN.txt for every sample that is read aloud.
func references(dir string) (map[string]string, error) {
	refs := map[string]string{}
	for _, s := range spoken() {
		b, err := os.ReadFile(filepath.Join(dir, s.ID+".txt"))
		if err != nil {
			return nil, err
		}
		refs[s.ID] = strings.TrimSpace(string(b))
	}
	return refs, nil
}

// readAudio reads a recording and checks its type like the upload does.
func readAudio(path string) ([]byte, string, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, "", err
	}
	mime, err := core.CheckAudio(data)
	if err != nil {
		return nil, "", fmt.Errorf("%s: %w", path, err)
	}
	return data, mime, nil
}

// recordings collects everything to measure: own recordings (if ownDir is
// set), the synthetic voice, generated silence and noise, and the read
// sentences joined into one longer recording per voice.
func recordings(ctx context.Context, conv speech.Converter, speechDir, ownDir string) ([]recording, error) {
	refs, err := references(speechDir)
	if err != nil {
		return nil, fmt.Errorf("reference texts: %w", err)
	}
	var out []recording

	add := func(source, file string, s sample) ([]int16, error) {
		data, mime, err := readAudio(file)
		if err != nil {
			return nil, err
		}
		out = append(out, recording{Source: source, Name: s.ID, What: s.What, Ref: refs[s.Ref], Terms: s.Terms, Audio: data, Mime: mime})
		if s.Ref != s.ID {
			return nil, nil // not part of the joined recording
		}
		return conv.ToPCM(ctx, data, core.MaxAudioSeconds+1)
	}
	joined := func(source string, parts [][]int16) {
		var ref []string
		var terms []term
		for _, s := range spoken() {
			ref = append(ref, refs[s.ID])
			terms = append(terms, s.Terms...)
		}
		out = append(out, recording{
			Source: source, Name: "01–08 joined", What: "all read sentences in one recording, 0.5 s apart",
			Ref: strings.Join(ref, " "), Terms: terms, Joined: true,
			Audio: core.WAV(join(parts, pauseSeconds)), Mime: core.MimeAudioWAV,
		})
	}

	if ownDir != "" {
		var parts [][]int16
		for _, s := range samples {
			pcm, err := add(sourceOwn, filepath.Join(ownDir, s.ID+".m4a"), s)
			if err != nil {
				return nil, err
			}
			if pcm != nil {
				parts = append(parts, pcm)
			}
		}
		joined(sourceOwn, parts)
	}

	var parts [][]int16
	var voice03 []int16
	for _, s := range spoken() {
		pcm, err := add(sourceVoice, filepath.Join(speechDir, s.ID+"_anna.m4a"), s)
		if err != nil {
			return nil, err
		}
		parts = append(parts, pcm)
		if s.ID == "03" {
			voice03 = pcm
		}
	}
	joined(sourceVoice, parts)

	n := generatedSeconds * core.SpeechSampleRate
	out = append(out,
		recording{Source: sourceGenerated, Name: "silence", What: fmt.Sprintf("%d s digital silence", generatedSeconds),
			Audio: core.WAV(make([]int16, n)), Mime: core.MimeAudioWAV},
		recording{Source: sourceGenerated, Name: "noise", What: fmt.Sprintf("%d s white noise at %d dBFS", generatedSeconds, noiseDBFS),
			Audio: core.WAV(noise(n, noiseDBFS, noiseSeed)), Mime: core.MimeAudioWAV},
		recording{Source: sourceGenerated, Name: "03 + noise", What: fmt.Sprintf("Anna's sentence 03 with white noise at %d dBFS", noiseDBFS),
			Ref: refs["03"], Terms: termsBloodPressure,
			Audio: core.WAV(mix(voice03, noise(len(voice03), noiseDBFS, noiseSeed))), Mime: core.MimeAudioWAV},
	)
	return out, nil
}

// measure sends every recording through the pipeline, one after another,
// with the same time limit as the API. Recordings without speech also go
// to Whisper directly, without the loudness check and the cleaning, to
// show what those checks prevent.
func measure(ctx context.Context, p *speech.Pipeline, recs []recording) ([]row, error) {
	var rows []row
	for _, rec := range recs {
		pcm, err := p.Converter.ToPCM(ctx, rec.Audio, core.MaxAudioSeconds+1)
		if err != nil {
			return nil, fmt.Errorf("%s %s: %w", rec.Source, rec.Name, err)
		}
		r := row{Source: rec.Source, Name: rec.Name, What: rec.What, Ref: rec.Ref, Joined: rec.Joined, Seconds: core.PCMSeconds(pcm)}

		tctx, cancel := context.WithTimeout(ctx, api.TranscribeTimeout)
		start := time.Now()
		tr, err := p.Transcribe(tctx, rec.Audio, rec.Mime)
		r.Elapsed = time.Since(start)
		cancel()
		switch {
		case err == nil:
			r.Result, r.Text = "text", tr.Text
		case errors.Is(err, speech.ErrNoSpeech):
			r.Result = "NO_SPEECH"
			r.CaughtBy = strings.TrimSuffix(err.Error(), ": "+speech.ErrNoSpeech.Error())
		case errors.Is(err, speech.ErrUnreadable):
			r.Result = "UNREADABLE"
		case errors.Is(err, speech.ErrTooLong):
			r.Result = "AUDIO_TOO_LONG"
		default:
			return nil, fmt.Errorf("%s %s: %w", rec.Source, rec.Name, err)
		}

		if rec.Ref == "" {
			segments, err := p.Engine.Recognize(ctx, core.WAV(pcm))
			if err != nil {
				return nil, fmt.Errorf("%s %s, Whisper alone: %w", rec.Source, rec.Name, err)
			}
			var raw []string
			for _, s := range segments {
				raw = append(raw, strings.TrimSpace(s.Text))
			}
			r.WhisperAlone = strings.TrimSpace(strings.Join(raw, " "))
		}

		r.WER = core.WordErrorRate(rec.Ref, r.Text)
		r.Terms = len(rec.Terms)
		for _, tm := range rec.Terms {
			if slices.ContainsFunc(tm, func(form string) bool { return core.TextContains(r.Text, form) }) {
				r.Found++
			} else {
				r.Missing = append(r.Missing, label(tm, rec.Ref))
			}
		}
		rows = append(rows, r)
		fmt.Printf("  %-10s %-13s %5.1f s  %5.1f s  %-9s WER %3.0f %%  terms %d/%d\n",
			r.Source, r.Name, r.Seconds, r.Elapsed.Seconds(), r.Result, r.WER*100, r.Found, r.Terms)
	}
	return rows, nil
}

// label names a key term by the form the reference uses (dates have many).
func label(t term, ref string) string {
	for _, form := range t {
		if core.TextContains(ref, form) {
			return form
		}
	}
	return t[0]
}

// noise is white noise with an RMS level of dbfs; the same seed gives the
// same samples.
func noise(n int, dbfs float64, seed uint64) []int16 {
	r := rand.New(rand.NewPCG(seed, seed))
	amp := math.MaxInt16 * math.Pow(10, dbfs/20)
	out := make([]int16, n)
	for i := range out {
		out[i] = clip(r.NormFloat64() * amp)
	}
	return out
}

// mix adds b to a (as long as a), clipped to the 16-bit range.
func mix(a, b []int16) []int16 {
	out := make([]int16, len(a))
	for i := range a {
		v := float64(a[i])
		if i < len(b) {
			v += float64(b[i])
		}
		out[i] = clip(v)
	}
	return out
}

// join puts the parts one after another with pause seconds of silence
// between them.
func join(parts [][]int16, pause float64) []int16 {
	gap := make([]int16, int(pause*core.SpeechSampleRate))
	var out []int16
	for i, p := range parts {
		if i > 0 {
			out = append(out, gap...)
		}
		out = append(out, p...)
	}
	return out
}

func clip(v float64) int16 {
	return int16(max(math.MinInt16, min(math.MaxInt16, math.Round(v))))
}
