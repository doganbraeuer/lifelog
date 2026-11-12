package main

import (
	"math"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"

	"lifelog/backend/internal/core"
)

const speechDir = "../../../demo/speech"

// Every key term must be in its reference text, in one of its forms;
// otherwise it could never be found and the measurement would be wrong.
func TestSamples_TermsInReference(t *testing.T) {
	refs, err := references(speechDir)
	if err != nil {
		t.Fatal(err)
	}
	ids := map[string]bool{}
	for _, s := range samples {
		if ids[s.ID] {
			t.Errorf("sample %s twice", s.ID)
		}
		ids[s.ID] = true
		if s.Ref == "" {
			if len(s.Terms) > 0 {
				t.Errorf("%s: key terms without a reference", s.ID)
			}
			continue
		}
		ref, ok := refs[s.Ref]
		if !ok {
			t.Errorf("%s: no reference text %s.txt", s.ID, s.Ref)
			continue
		}
		if len(s.Terms) == 0 {
			t.Errorf("%s: no key terms", s.ID)
		}
		for _, tm := range s.Terms {
			if !slices.ContainsFunc(tm, func(f string) bool { return core.TextContains(ref, f) }) {
				t.Errorf("%s: key term %q not in the reference %q", s.ID, tm[0], ref)
			}
		}
	}
}

// Every sentence that is read aloud has a synthetic recording, and every
// reference text belongs to a sample.
func TestSamples_Files(t *testing.T) {
	for _, s := range spoken() {
		data, err := os.ReadFile(filepath.Join(speechDir, s.ID+"_anna.m4a"))
		if err != nil {
			t.Errorf("%s: %v (run demo/speech/make-voice.sh)", s.ID, err)
			continue
		}
		if mime, err := core.CheckAudio(data); err != nil || mime != core.MimeAudioMP4 {
			t.Errorf("%s_anna.m4a: %q, %v", s.ID, mime, err)
		}
	}
	txts, _ := filepath.Glob(filepath.Join(speechDir, "*.txt"))
	if len(txts) != len(spoken()) {
		t.Errorf("%d reference texts, %d sentences read aloud", len(txts), len(spoken()))
	}
}

func TestNoise(t *testing.T) {
	n := 10 * core.SpeechSampleRate
	a, b := noise(n, noiseDBFS, noiseSeed), noise(n, noiseDBFS, noiseSeed)
	if !slices.Equal(a, b) {
		t.Error("same seed, different noise")
	}
	if slices.Equal(a, noise(n, noiseDBFS, noiseSeed+1)) {
		t.Error("other seed, same noise")
	}
	var sum float64
	for _, v := range a {
		sum += float64(v) * float64(v)
	}
	if db := 20 * math.Log10(math.Sqrt(sum/float64(n))/math.MaxInt16); math.Abs(db-noiseDBFS) > 0.5 {
		t.Errorf("level %.1f dBFS, want %d", db, noiseDBFS)
	}
	// The noise must pass the loudness check, otherwise Whisper never sees it
	// and the measurement says nothing about invented text.
	if s := core.SpeechSeconds(a); s < core.MinSpeechSeconds {
		t.Errorf("noise has only %.2f s above the silence threshold", s)
	}
}

func TestMixAndJoin(t *testing.T) {
	got := mix([]int16{100, 32000, -32000, 5}, []int16{1, 1000, -1000})
	if want := []int16{101, 32767, -32768, 5}; !slices.Equal(got, want) {
		t.Errorf("mix = %v, want %v (clipped, as long as the first)", got, want)
	}
	j := join([][]int16{{1, 2}, {3}}, 0.001) // 16 samples of pause
	if len(j) != 2+16+1 || j[0] != 1 || j[2] != 0 || j[len(j)-1] != 3 {
		t.Errorf("join = %v", j)
	}
	if len(join(nil, pauseSeconds)) != 0 {
		t.Error("join of nothing is not empty")
	}
}

func TestReport(t *testing.T) {
	rows := []row{
		{Source: sourceOwn, Name: "01", Ref: "Puls 64", Seconds: 10, Elapsed: time.Second, Result: "text", Text: "Puls 64", Terms: 2, Found: 2},
		{Source: sourceOwn, Name: "02", Ref: "Puls 64 heute", Seconds: 5, Elapsed: 2 * time.Second, Result: "text", Text: "Puls 46", WER: 0.5, Terms: 2, Found: 1, Missing: []string{"Puls 64"}},
		{Source: sourceOwn, Name: "09", Seconds: 10, Elapsed: time.Second, Result: "text", Text: "Untertitel"},
		{Source: sourceOwn, Name: "01–08 joined", Ref: "x", Joined: true, Seconds: 60, Elapsed: 6 * time.Second, Result: "text", Terms: 4, Found: 3},
		{Source: sourceGenerated, Name: "silence", Seconds: 10, Result: "NO_SPEECH", CaughtBy: "0.00 s above the silence threshold", WhisperAlone: "Untertitel im Auftrag des ZDF"},
		{Source: sourceGenerated, Name: "03 + noise", Ref: "Puls 64", Seconds: 5, Elapsed: time.Second, Result: "text", Text: "plus 64", WER: 0.5, Terms: 1},
	}
	sums := summarize(rows)
	if len(sums) != 2 {
		t.Fatalf("%d sources", len(sums))
	}
	s := sums[0]
	if s.Recordings != 3 || s.Speech != 2 || s.SpeechText != 2 || s.Silent != 1 || s.SilentOK != 0 ||
		s.Terms != 4 || s.Found != 3 || math.Abs(s.meanWER()-0.25) > 1e-9 || math.Abs(s.MaxRTF-0.4) > 1e-9 {
		t.Errorf("summary %+v (the joined recording must not count)", s)
	}

	md := report{Date: "2026-11-12", Engine: "whisper.cpp x", Converter: "ffmpeg y", Machine: "m", Rows: rows}.Markdown()
	for _, want := range []string{
		"| iPhone | 3 | 2/2 | 25.0 % | 3/4 (75 %) | 0/1 | 0.40 |",
		"❌ Key terms, iPhone: 75 %",
		"❌ No text without speech: 1 of 2",
		"iPhone 09: text „Untertitel“",
		"✅ Speed, iPhone: 60 s of speech in 6.0 s, so 3 minutes take about 18 s",
		"| iPhone | 02 |",
		"| Puls 64 |",
		"- Recognized: Puls 46",
		"| generated | silence |  | 0.00 s above the silence threshold | Untertitel im Auftrag des ZDF |",
		"| iPhone | 09 |  | nothing: text | – |",
	} {
		if !strings.Contains(md, want) {
			t.Errorf("report misses %q:\n%s", want, md)
		}
	}
	if strings.Contains(md, "Key terms, generated") {
		t.Error("criterion for the single stress test")
	}
	if strings.Contains(md, "**iPhone 01–08 joined**") {
		t.Error("joined recording printed as text")
	}
}
