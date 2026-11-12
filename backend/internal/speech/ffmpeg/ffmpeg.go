// Package ffmpeg implements speech.Converter with the ffmpeg command line
// program: any allowed recording becomes 16 kHz mono 16-bit PCM.
//
// The recording goes through one temporary file: an M4A from iOS has its
// index ("moov atom") at the end, and ffmpeg cannot read such a file from
// stdin. The file is only readable by the user (0600) and is deleted right
// after, also on errors. The samples come back via stdout.
//
// Install: macOS `brew install ffmpeg`, Debian/Ubuntu `apt install ffmpeg`,
// Windows `winget install Gyan.FFmpeg`.
package ffmpeg

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"os/exec"
	"strconv"
	"strings"

	"lifelog/backend/internal/core"
	"lifelog/backend/internal/speech"
)

// FFmpeg implements speech.Converter.
type FFmpeg struct {
	path, version string
}

// New finds ffmpeg in PATH. If it is missing, the error is
// speech.ErrUnavailable.
func New() (*FFmpeg, error) {
	path, err := exec.LookPath("ffmpeg")
	if err != nil {
		return nil, fmt.Errorf("ffmpeg not found: %w", speech.ErrUnavailable)
	}
	out, err := exec.Command(path, "-hide_banner", "-version").Output()
	if err != nil {
		return nil, fmt.Errorf("ffmpeg -version: %v: %w", err, speech.ErrUnavailable)
	}
	return &FFmpeg{path: path, version: parseVersion(string(out))}, nil
}

// parseVersion takes "8.1.1" from "ffmpeg version 8.1.1 Copyright …".
func parseVersion(out string) string {
	f := strings.Fields(out)
	if len(f) >= 3 && f[0] == "ffmpeg" && f[1] == "version" {
		return f[2]
	}
	return "unknown"
}

// Name is e.g. "ffmpeg 8.1.1".
func (f *FFmpeg) Name() string { return "ffmpeg " + f.version }

// ToPCM implements speech.Converter.
func (f *FFmpeg) ToPCM(ctx context.Context, audio []byte, limitSeconds float64) ([]int16, error) {
	tmp, err := os.CreateTemp("", "lifelog-audio-*") // mode 0600
	if err != nil {
		return nil, fmt.Errorf("temporary file: %w", err)
	}
	defer os.Remove(tmp.Name())
	_, err = tmp.Write(audio)
	if cerr := tmp.Close(); err == nil {
		err = cerr
	}
	if err != nil {
		return nil, fmt.Errorf("temporary file: %w", err)
	}

	cmd := exec.CommandContext(ctx, f.path, args(tmp.Name(), limitSeconds)...)
	var stdout, stderr bytes.Buffer
	cmd.Stdout, cmd.Stderr = &stdout, &stderr
	if err := cmd.Run(); err != nil {
		if ctx.Err() != nil {
			return nil, ctx.Err()
		}
		// stderr holds ffmpeg's reason ("Invalid data found when
		// processing input", "does not contain any stream"), no content
		// of the recording.
		msg := strings.TrimSpace(stderr.String())
		msg = strings.ReplaceAll(msg, tmp.Name(), "<recording>")
		if len(msg) > 200 {
			msg = msg[:200]
		}
		return nil, fmt.Errorf("ffmpeg: %v: %s: %w", err, msg, speech.ErrUnreadable)
	}
	return core.PCM16(stdout.Bytes()), nil
}

// args: decode the file, drop video, mix down to mono, resample to 16 kHz,
// stop after limitSeconds, write raw signed 16-bit little-endian samples.
func args(input string, limitSeconds float64) []string {
	return []string{
		"-nostdin", "-hide_banner", "-loglevel", "error",
		"-i", input,
		"-vn", "-ac", "1", "-ar", strconv.Itoa(core.SpeechSampleRate),
		"-t", strconv.FormatFloat(limitSeconds, 'f', -1, 64),
		"-f", "s16le", "-c:a", "pcm_s16le",
		"pipe:1",
	}
}
