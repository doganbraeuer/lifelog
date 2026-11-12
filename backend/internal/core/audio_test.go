package core

import (
	"bytes"
	"errors"
	"math"
	"math/rand"
	"testing"
)

// mp4 builds the start of an MP4/3GP/HEIC file with the given major brand.
func mp4(brand string) []byte {
	return append([]byte("\x00\x00\x00\x1cftyp"+brand+"\x00\x00\x00\x00isommp42"), bytes.Repeat([]byte{0}, 16)...)
}

func TestDetectAudioType(t *testing.T) {
	tests := []struct {
		name string
		data []byte
		want string // "" = not allowed
	}{
		{"M4A from iOS", mp4("M4A "), MimeAudioMP4},
		{"MP4 with mp42 (Android)", mp4("mp42"), MimeAudioMP4},
		{"MP4 with isom", mp4("isom"), MimeAudioMP4},
		{"3GP", mp4("3gp4"), MimeAudio3GPP},
		{"AMR", []byte("#!AMR\n\x3c\x91\x17"), MimeAudioAMR},
		{"AMR-WB", []byte("#!AMR-WB\n\x00"), MimeAudioAMR},
		{"WAV", append([]byte("RIFF\x24\x00\x00\x00WAVEfmt "), make([]byte, 20)...), MimeAudioWAV},
		{"OGG", []byte("OggS\x00\x02\x00\x00\x00\x00"), MimeAudioOGG},
		{"WebM", append([]byte{0x1A, 0x45, 0xDF, 0xA3, 0x9F, 0x42, 0x86, 0x81, 0x01, 0x42, 0x82, 0x84}, []byte("webm\x42\x87")...), MimeAudioWebM},
		{"FLAC", []byte("fLaC\x00\x00\x00\x22"), MimeAudioFLAC},
		{"MP3 with ID3", []byte("ID3\x04\x00\x00\x00\x00\x00\x00"), MimeAudioMPEG},
		{"MP3 frame without tag", []byte{0xFF, 0xFB, 0x90, 0x64, 0x00}, MimeAudioMPEG},

		{"HEIC is an image", mp4("heic"), ""},
		{"AVIF is an image", mp4("avif"), ""},
		{"generic HEIF", mp4("mif1"), ""},
		{"Matroska video is no WebM", append([]byte{0x1A, 0x45, 0xDF, 0xA3, 0x9F, 0x42, 0x82, 0x88}, []byte("matroska")...), ""},
		{"RIFF that is WebP", []byte("RIFF\x24\x00\x00\x00WEBPVP8 "), ""},
		{"AAC in ADTS (layer 0)", []byte{0xFF, 0xF1, 0x50, 0x80, 0x00}, ""},
		{"JPEG", []byte{0xFF, 0xD8, 0xFF, 0xE0, 0x00}, ""},
		{"PDF", []byte("%PDF-1.7"), ""},
		{"text", []byte("Arztbrief vom 12. November"), ""},
		{"too short", []byte("Og"), ""},
	}
	for _, tt := range tests {
		got, err := DetectAudioType(tt.data)
		if tt.want == "" {
			if !errors.Is(err, ErrUnsupportedAudioType) {
				t.Errorf("%s: %q, %v; want ErrUnsupportedAudioType", tt.name, got, err)
			}
			continue
		}
		if err != nil || got != tt.want {
			t.Errorf("%s: %q, %v; want %q", tt.name, got, err, tt.want)
		}
	}
}

func TestCheckAudio(t *testing.T) {
	if _, err := CheckAudio(nil); !errors.Is(err, ErrFileEmpty) {
		t.Errorf("empty: %v", err)
	}
	big := append(mp4("M4A "), make([]byte, MaxFileBytes)...)
	if _, err := CheckAudio(big); !errors.Is(err, ErrFileTooLarge) {
		t.Errorf("too large: %v", err)
	}
	if got, err := CheckAudio(mp4("M4A ")); err != nil || got != MimeAudioMP4 {
		t.Errorf("m4a: %q, %v", got, err)
	}
	if _, err := CheckAudio([]byte("hello")); !errors.Is(err, ErrUnsupportedAudioType) {
		t.Errorf("text: %v", err)
	}
}

func TestPCM16AndWAV(t *testing.T) {
	samples := []int16{0, 1, -1, 32767, -32768, 1000}
	raw := make([]byte, 0, 2*len(samples)+1)
	for _, s := range samples {
		raw = append(raw, byte(uint16(s)), byte(uint16(s)>>8))
	}
	got := PCM16(append(raw, 0x7F)) // odd trailing byte is ignored
	if len(got) != len(samples) {
		t.Fatalf("%d samples", len(got))
	}
	for i := range samples {
		if got[i] != samples[i] {
			t.Errorf("sample %d: %d, want %d", i, got[i], samples[i])
		}
	}

	w := WAV(samples)
	if len(w) != 44+len(raw) || string(w[0:4]) != "RIFF" || string(w[8:16]) != "WAVEfmt " || string(w[36:40]) != "data" {
		t.Fatalf("header %q", w[:44])
	}
	if !bytes.Equal(w[44:], raw) {
		t.Error("data differs from the samples")
	}
	if mime, err := DetectAudioType(w); err != nil || mime != MimeAudioWAV {
		t.Errorf("own WAV detected as %q, %v", mime, err)
	}
	if s := PCMSeconds(make([]int16, 24000)); s != 1.5 {
		t.Errorf("seconds %v", s)
	}
}

// tone: a sine at the given level in dBFS.
func tone(seconds, dbfs float64) []int16 {
	n := int(seconds * SpeechSampleRate)
	amp := 32767 * math.Pow(10, dbfs/20) * math.Sqrt2 // RMS of a sine = amplitude / √2
	out := make([]int16, n)
	for i := range out {
		out[i] = int16(amp * math.Sin(2*math.Pi*220*float64(i)/SpeechSampleRate))
	}
	return out
}

// noise: random samples at the given RMS level in dBFS (fixed seed).
func noise(seconds, dbfs float64) []int16 {
	r := rand.New(rand.NewSource(1))
	n := int(seconds * SpeechSampleRate)
	sd := 32767 * math.Pow(10, dbfs/20)
	out := make([]int16, n)
	for i := range out {
		out[i] = int16(max(-32768, min(32767, r.NormFloat64()*sd)))
	}
	return out
}

func TestSpeechSeconds(t *testing.T) {
	near := func(a, b float64) bool { return math.Abs(a-b) < 0.031 }
	if got := SpeechSeconds(make([]int16, 10*SpeechSampleRate)); got != 0 {
		t.Errorf("silence: %v", got)
	}
	if got := SpeechSeconds(noise(10, -60)); got != 0 {
		t.Errorf("quiet room (-60 dBFS): %v", got)
	}
	if got := SpeechSeconds(tone(2, -20)); !near(got, 1.98) {
		t.Errorf("loud tone 2 s: %v", got)
	}
	// 1 s of sound inside 9 s of silence.
	mixed := append(append(make([]int16, 4*SpeechSampleRate), tone(1, -25)...), make([]int16, 4*SpeechSampleRate)...)
	if got := SpeechSeconds(mixed); !near(got, 1.0) {
		t.Errorf("1 s sound in silence: %v", got)
	}
	// Just below and above the threshold.
	if got := SpeechSeconds(tone(1, SilenceThresholdDBFS-3)); got != 0 {
		t.Errorf("3 dB below threshold: %v", got)
	}
	if got := SpeechSeconds(tone(1, SilenceThresholdDBFS+3)); got < 0.9 {
		t.Errorf("3 dB above threshold: %v", got)
	}
	if got := SpeechSeconds(nil); got != 0 {
		t.Errorf("empty: %v", got)
	}
}
