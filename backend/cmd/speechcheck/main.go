// speechcheck measures the transcription (docs/API.md, 3.9) on known
// sentences: how many words and key terms (numbers, units, dates, names,
// medication) are recognized, how fast, and whether silence and noise give
// NO_SPEECH instead of invented text. It never calls a language model and
// stores nothing. Start whisper-server first (backend/scripts/whisper-server.sh),
// then from backend/:
//
//	go run ./cmd/speechcheck                                        # synthetic voice and generated audio only
//	go run ./cmd/speechcheck -own ~/Projekte/lifelog-demo/speech    # plus own recordings 01.m4a … 10.m4a
//	go run ./cmd/speechcheck -own … -out ../docs/speech/2026-11-12-whisper.md
//	go run ./cmd/speechcheck -vocabulary=false                      # without the list of medical terms as prompt
//
// What it measures (docs/speech/README.md):
//
//   - Own recordings (-own): voice memos of the sentences in demo/speech/,
//     01–08 read aloud, 09 silence in a quiet room, 10 sentence 03 with
//     background noise. A voice is personal: these files stay outside the
//     repository.
//   - Synthetic voice: demo/speech/NN_anna.m4a (demo/speech/make-voice.sh).
//   - Generated here: digital silence, loud and quiet white noise, brown
//     noise, and sentence 03 with the loud noise.
//   - Per voice, sentences 01–08 joined into one longer recording, for the
//     speed of a spoken doctor's letter of a few minutes.
//
// Needs ffmpeg and a running whisper-server (backend/README.md, speech).
package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"lifelog/backend/internal/config"
	"lifelog/backend/internal/speech"
	"lifelog/backend/internal/speech/ffmpeg"
	"lifelog/backend/internal/speech/whisper"
)

func main() {
	demo := flag.String("demo", "../demo", "folder with the demo data (reference texts and synthetic voice in speech/)")
	own := flag.String("own", "", "folder with own recordings 01.m4a … 10.m4a (outside the repository); empty = skip them")
	out := flag.String("out", "", "also write the measurement as Markdown to this file, e.g. ../docs/speech/2026-11-12-whisper.md")
	vocabulary := flag.Bool("vocabulary", true, "send the list of medical terms (whisper.Vocabulary) as prompt, as the backend does")
	flag.Parse()

	if err := run(*demo, *own, *out, *vocabulary); err != nil {
		fmt.Fprintln(os.Stderr, "Error:", err)
		os.Exit(1)
	}
}

func run(demoDir, ownDir, out string, vocabulary bool) error {
	if err := config.LoadDotenv(); err != nil {
		return err
	}
	url := os.Getenv("WHISPER_URL")
	if url == "" {
		url = config.DefaultWhisperURL
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()
	ctx, cancel := context.WithTimeout(ctx, 20*time.Minute)
	defer cancel()

	conv, err := ffmpeg.New()
	if err != nil {
		return fmt.Errorf("%w (see backend/README.md, speech)", err)
	}
	engine := whisper.New(url)
	if !vocabulary {
		engine.Prompt = ""
	}
	if err := engine.Ping(ctx); err != nil {
		return fmt.Errorf("whisper-server does not answer at %s; start it with backend/scripts/whisper-server.sh: %w", url, err)
	}
	pipeline := &speech.Pipeline{Converter: conv, Engine: engine}

	speechDir := filepath.Join(demoDir, "speech")
	recs, err := recordings(ctx, conv, speechDir, ownDir)
	if err != nil {
		return err
	}
	refs, err := references(speechDir)
	if err != nil {
		return err
	}
	fmt.Printf("%d recordings. Engine: %s at %s, %s. No language model is called.\n", len(recs), engine.Name(), url, conv.Name())
	if ownDir == "" {
		fmt.Println("No own recordings (-own): only the synthetic voice and generated audio.")
	}

	// The first request after starting whisper-server prepares the GPU and
	// is slower; it should not count.
	if err := warmUp(ctx, pipeline, recs); err != nil {
		return err
	}

	rows, err := measure(ctx, pipeline, recs, whisper.VocabularyTerms())
	if err != nil {
		return err
	}
	rep := report{
		Date:      time.Now().Format("2006-01-02"),
		Engine:    engine.Name(),
		Converter: conv.Name(),
		Machine:   machine(),
		Prompt:    engine.Prompt,
		Vocab:     vocabularyInSentences(whisper.VocabularyTerms(), refs),
		Rows:      rows,
	}
	md := rep.Markdown()
	fmt.Println()
	fmt.Println(md)
	if out != "" {
		if err := os.WriteFile(out, []byte(md), 0o644); err != nil {
			return err
		}
		fmt.Println("Measurement written to", out)
	}
	return nil
}

// warmUp transcribes the first recording with speech once, unmeasured.
func warmUp(ctx context.Context, t speech.Transcriber, recs []recording) error {
	for _, r := range recs {
		if r.Ref != "" && !r.Joined {
			_, err := t.Transcribe(ctx, r.Audio, r.Mime)
			if err != nil && !errors.Is(err, speech.ErrNoSpeech) {
				return fmt.Errorf("warm-up: %w", err)
			}
			return nil
		}
	}
	return nil
}

// machine describes the computer for the report, e.g. "Apple M3 Pro,
// darwin/arm64".
func machine() string {
	arch := runtime.GOOS + "/" + runtime.GOARCH
	if runtime.GOOS == "darwin" {
		if b, err := exec.Command("sysctl", "-n", "machdep.cpu.brand_string").Output(); err == nil {
			return strings.TrimSpace(string(b)) + ", " + arch
		}
	}
	return arch
}

// vocabularyInSentences lists every term of the vocabulary with the test
// sentences it happens to be in (see said).
func vocabularyInSentences(vocab []string, refs map[string]string) []vocabTerm {
	out := make([]vocabTerm, len(vocab))
	for i, t := range vocab {
		out[i].Term = t
		for _, s := range spoken() {
			if said(t, refs[s.ID], s.Terms) {
				out[i].Sentences = append(out[i].Sentences, s.ID)
			}
		}
	}
	return out
}
