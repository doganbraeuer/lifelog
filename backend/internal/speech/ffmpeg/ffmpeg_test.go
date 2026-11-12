package ffmpeg

import (
	"context"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"testing"
	"time"

	"lifelog/backend/internal/core"
	"lifelog/backend/internal/speech"
)

func TestParseVersionAndArgs(t *testing.T) {
	if v := parseVersion("ffmpeg version 8.1.1 Copyright (c) 2000-2026 the FFmpeg developers\n"); v != "8.1.1" {
		t.Errorf("version %q", v)
	}
	if v := parseVersion("something else"); v != "unknown" {
		t.Errorf("version %q", v)
	}
	a := args("/tmp/x", 301)
	for _, want := range [][]string{{"-i", "/tmp/x"}, {"-ac", "1"}, {"-ar", "16000"}, {"-t", "301"}, {"-f", "s16le"}} {
		i := slices.Index(a, want[0])
		if i < 0 || i+1 >= len(a) || a[i+1] != want[1] {
			t.Errorf("args %v: missing %v", a, want)
		}
	}
	if a[len(a)-1] != "pipe:1" || !slices.Contains(a, "-nostdin") || !slices.Contains(a, "-vn") {
		t.Errorf("args %v", a)
	}
}

// generate makes a test file with the installed ffmpeg (lavfi sources).
func generate(t *testing.T, name string, ffmpegArgs ...string) []byte {
	t.Helper()
	out := filepath.Join(t.TempDir(), name)
	cmd := exec.Command("ffmpeg", append(append([]string{"-nostdin", "-loglevel", "error", "-y"}, ffmpegArgs...), out)...)
	if b, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("generate %s: %v: %s", name, err, b)
	}
	data, err := os.ReadFile(out)
	if err != nil {
		t.Fatal(err)
	}
	return data
}

func leftovers(t *testing.T) []string {
	t.Helper()
	m, err := filepath.Glob(filepath.Join(os.TempDir(), "lifelog-audio-*"))
	if err != nil {
		t.Fatal(err)
	}
	return m
}

// Integration with the installed ffmpeg. Skipped if it is missing.
func TestToPCM(t *testing.T) {
	f, err := New()
	if err != nil {
		t.Skipf("ffmpeg not installed: %v", err)
	}
	before := len(leftovers(t))
	ctx := context.Background()

	// AAC in M4A, 3 s, stereo 44.1 kHz, index at the end like an iPhone
	// recording (no "faststart").
	m4a := generate(t, "tone.m4a", "-f", "lavfi", "-i", "sine=frequency=440:sample_rate=44100:duration=3", "-ac", "2", "-c:a", "aac", "-b:a", "64k")
	if mime, err := core.DetectAudioType(m4a); err != nil || mime != core.MimeAudioMP4 {
		t.Fatalf("generated m4a detected as %q, %v", mime, err)
	}
	samples, err := f.ToPCM(ctx, m4a, 301)
	if err != nil {
		t.Fatal(err)
	}
	if s := core.PCMSeconds(samples); s < 2.9 || s > 3.1 {
		t.Errorf("m4a: %.2f s, want about 3", s)
	}
	if core.SpeechSeconds(samples) < 2.5 {
		t.Errorf("m4a: the tone is too quiet after converting")
	}

	// The limit cuts the decoding.
	samples, err = f.ToPCM(ctx, m4a, 1.5)
	if err != nil || core.PCMSeconds(samples) != 1.5 {
		t.Errorf("limit 1.5 s: %.2f s, %v", core.PCMSeconds(samples), err)
	}

	// Silence as WAV stays silence.
	wav := generate(t, "silence.wav", "-f", "lavfi", "-i", "anullsrc=r=48000:cl=mono", "-t", "2")
	samples, err = f.ToPCM(ctx, wav, 301)
	if err != nil || core.PCMSeconds(samples) != 2 || core.SpeechSeconds(samples) != 0 {
		t.Errorf("silence: %.2f s, sound %.2f s, %v", core.PCMSeconds(samples), core.SpeechSeconds(samples), err)
	}

	// A video without audio track and random bytes cannot be decoded.
	video := generate(t, "video.mp4", "-f", "lavfi", "-i", "testsrc=size=64x64:rate=5:duration=1", "-pix_fmt", "yuv420p")
	for name, data := range map[string][]byte{"video without audio": video, "garbage": []byte("ftypM4A not really an m4a")} {
		if _, err := f.ToPCM(ctx, data, 301); !errors.Is(err, speech.ErrUnreadable) {
			t.Errorf("%s: %v, want ErrUnreadable", name, err)
		}
	}

	// Canceled: the context error, not "unreadable".
	canceled, cancel := context.WithCancel(ctx)
	cancel()
	if _, err := f.ToPCM(canceled, m4a, 301); !errors.Is(err, context.Canceled) {
		t.Errorf("canceled: %v", err)
	}
	timeout, cancel2 := context.WithTimeout(ctx, time.Nanosecond)
	defer cancel2()
	time.Sleep(time.Millisecond)
	if _, err := f.ToPCM(timeout, m4a, 301); !errors.Is(err, context.DeadlineExceeded) {
		t.Errorf("timeout: %v", err)
	}

	// No temporary file is left behind.
	if after := leftovers(t); len(after) != before {
		t.Errorf("temporary files left: %v", after)
	}
}
