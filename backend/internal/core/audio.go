package core

import (
	"bytes"
	"encoding/binary"
	"errors"
	"math"
	"slices"
)

// Audio formats allowed for a transcription (docs/API.md, 3.9). The type is
// recognized from the content, like CheckFile does for documents.
const (
	MimeAudioMP4  = "audio/mp4"  // M4A/MP4 with AAC: iOS, Android, Flutter "record"
	MimeAudio3GPP = "audio/3gpp" // older Android recorders
	MimeAudioAMR  = "audio/amr"
	MimeAudioWAV  = "audio/wav"
	MimeAudioOGG  = "audio/ogg"  // Opus or Vorbis
	MimeAudioWebM = "audio/webm" // Opus
	MimeAudioMPEG = "audio/mpeg" // MP3
	MimeAudioFLAC = "audio/flac"
)

// Limits of a recording (docs/API.md, 3.9).
const (
	MaxAudioSeconds = 300 // 5 minutes; a spoken doctor's letter takes 1–3
	MinAudioSeconds = 0.5 // shorter counts as "no speech"
)

// Whisper needs 16 kHz mono PCM with 16 bit per sample.
const (
	SpeechSampleRate = 16000
	bytesPerSample   = 2
)

// ErrUnsupportedAudioType: not one of the allowed audio formats (→ 415).
var ErrUnsupportedAudioType = errors.New("audio type is not supported")

// CheckAudio checks size and type of a recording and returns the MIME type.
// Errors: ErrFileEmpty (→ 400 FILE_MISSING), ErrFileTooLarge (→ 413),
// ErrUnsupportedAudioType (→ 415). Whether the recording can be decoded and
// how long it is, only ffmpeg finds out.
func CheckAudio(data []byte) (string, error) {
	if len(data) == 0 {
		return "", ErrFileEmpty
	}
	if len(data) > MaxFileBytes {
		return "", ErrFileTooLarge
	}
	return DetectAudioType(data)
}

// MP4 brands of audio files. "isom", "mp41" and "mp42" are generic MP4 (a
// video has them too; ffmpeg then takes its audio track or reports that
// there is none). HEIC/AVIF brands are images and not on the list.
var (
	mp4AudioBrands = []string{"M4A ", "M4B ", "M4P ", "mp41", "mp42", "isom", "iso2", "iso4", "iso5", "iso6", "dash", "MSNV", "f4a "}
	threeGPBrands  = []string{"3gp4", "3gp5", "3gp6", "3gp7", "3ge6", "3ge7", "3gg6", "3g2a", "3g2b", "3g2c"}
)

// DetectAudioType recognizes the allowed audio formats from their first
// bytes ("magic bytes").
func DetectAudioType(d []byte) (string, error) {
	switch {
	case len(d) >= 12 && string(d[4:8]) == "ftyp":
		major := string(d[8:12])
		switch {
		case slices.Contains(threeGPBrands, major):
			return MimeAudio3GPP, nil
		case slices.Contains(mp4AudioBrands, major):
			return MimeAudioMP4, nil
		}
	case bytes.HasPrefix(d, []byte("#!AMR")):
		return MimeAudioAMR, nil
	case len(d) >= 12 && string(d[0:4]) == "RIFF" && string(d[8:12]) == "WAVE":
		return MimeAudioWAV, nil
	case bytes.HasPrefix(d, []byte("OggS")):
		return MimeAudioOGG, nil
	case bytes.HasPrefix(d, []byte{0x1A, 0x45, 0xDF, 0xA3}):
		// EBML: WebM or Matroska. The document type is near the start.
		if bytes.Contains(d[:min(len(d), 64)], []byte("webm")) {
			return MimeAudioWebM, nil
		}
	case bytes.HasPrefix(d, []byte("fLaC")):
		return MimeAudioFLAC, nil
	case bytes.HasPrefix(d, []byte("ID3")), isMP3Frame(d):
		return MimeAudioMPEG, nil
	}
	return "", ErrUnsupportedAudioType
}

// isMP3Frame: an MPEG audio frame header without ID3 tag: 11 bits sync, a
// valid version and a layer that is not "reserved". AAC in ADTS also starts
// with the sync but has layer 0, and a JPEG starts with FF D8; both are no
// MP3.
func isMP3Frame(d []byte) bool {
	if len(d) < 4 || d[0] != 0xFF || d[1]&0xE0 != 0xE0 {
		return false
	}
	version := (d[1] >> 3) & 0x03
	layer := (d[1] >> 1) & 0x03
	bitrate := d[2] >> 4
	return version != 1 && layer != 0 && bitrate != 0x0F
}

// PCM16 reads raw 16-bit little-endian samples (ffmpeg -f s16le). An odd
// last byte is ignored.
func PCM16(raw []byte) []int16 {
	out := make([]int16, len(raw)/bytesPerSample)
	for i := range out {
		out[i] = int16(binary.LittleEndian.Uint16(raw[2*i:]))
	}
	return out
}

// PCMSeconds is the length of the samples at SpeechSampleRate.
func PCMSeconds(samples []int16) float64 {
	return float64(len(samples)) / SpeechSampleRate
}

// WAV wraps 16 kHz mono samples into a WAV file (44-byte header), the
// format whisper.cpp reads.
func WAV(samples []int16) []byte {
	dataLen := len(samples) * bytesPerSample
	b := make([]byte, 44+dataLen)
	copy(b[0:], "RIFF")
	binary.LittleEndian.PutUint32(b[4:], uint32(36+dataLen))
	copy(b[8:], "WAVEfmt ")
	binary.LittleEndian.PutUint32(b[16:], 16) // size of the fmt chunk
	binary.LittleEndian.PutUint16(b[20:], 1)  // PCM
	binary.LittleEndian.PutUint16(b[22:], 1)  // mono
	binary.LittleEndian.PutUint32(b[24:], SpeechSampleRate)
	binary.LittleEndian.PutUint32(b[28:], SpeechSampleRate*bytesPerSample) // bytes per second
	binary.LittleEndian.PutUint16(b[32:], bytesPerSample)                  // block align
	binary.LittleEndian.PutUint16(b[34:], 16)                              // bits per sample
	copy(b[36:], "data")
	binary.LittleEndian.PutUint32(b[40:], uint32(dataLen))
	for i, s := range samples {
		binary.LittleEndian.PutUint16(b[44+2*i:], uint16(s))
	}
	return b
}

// Loudness check before Whisper (docs/API.md, 3.9): Whisper invents
// sentences in silence, so a recording with too little sound is answered
// with "no speech" without asking it.
const (
	// SilenceThresholdDBFS: a 30 ms frame counts as sound if its RMS level
	// is above this, in dB below full scale. A quiet room recorded with a
	// phone is around -60 to -50 dBFS, speech around -35 to -15 dBFS. To be
	// confirmed by the measurement (cmd/speechcheck).
	SilenceThresholdDBFS = -45.0

	// MinSpeechSeconds: less sound than this in total counts as no speech.
	MinSpeechSeconds = 0.3

	frameSeconds = 0.03
)

// SpeechSeconds adds up the 30 ms frames whose RMS level is above
// SilenceThresholdDBFS. It is no speech detection (whisper.cpp's VAD does
// that); it only recognizes recordings that are (nearly) silent.
func SpeechSeconds(samples []int16) float64 {
	frame := int(SpeechSampleRate * frameSeconds) // 480 samples
	threshold := 32768 * math.Pow(10, SilenceThresholdDBFS/20)
	loud := 0
	for start := 0; start+frame <= len(samples); start += frame {
		sum := 0.0
		for _, s := range samples[start : start+frame] {
			sum += float64(s) * float64(s)
		}
		if math.Sqrt(sum/float64(frame)) > threshold {
			loud++
		}
	}
	return float64(loud) * frameSeconds
}
