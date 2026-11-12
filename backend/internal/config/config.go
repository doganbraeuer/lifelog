// Package config reads the backend settings from environment variables.
//
// Secrets (MongoDB credentials, API key) live only in .env and are never
// logged. Error messages therefore name only variable names, never values.
package config

import (
	"errors"
	"fmt"
	"net"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/joho/godotenv"
)

// Defaults for optional variables.
const (
	DefaultAddress = "127.0.0.1"
	DefaultPort    = "8080"

	// Models, checked on 2026-11-04 with go run ./cmd/models and the pricing
	// page (aistudio.google.com/docs/pricing). Both can be used in the free
	// tier; there, inputs may be used to improve the products, so only
	// synthetic documents. The exact free-tier limits (requests per
	// minute/day) are only shown by AI Studio in the own project.
	//
	// gemini-3.8-flash: stable, image (including HEIC) and PDF as input,
	// JSON output following a schema. (gemini-2.5-flash is deprecated and
	// will be redirected from 2026-12-01.)
	DefaultGeminiModel = "gemini-3.8-flash"

	// gemini-embedding-2: up to 8192 tokens per text, 768 dimensions are
	// normalized automatically. CAUTION: embeddings of different models are
	// not comparable. Whoever switches the model must re-embed all chunks,
	// otherwise search finds nothing useful anymore.
	DefaultGeminiEmbeddingModel = "gemini-embedding-2"

	// DefaultFallbackReserveSeconds: this many seconds of the time limit are
	// reserved for the fallback model, when reading (50 s → main model 30 s)
	// and when answering (45 s → main model 25 s). Allowed are 0 to 35
	// (checked by gemini.Neu). Without a fallback model there is no reserve.
	// Reason: if the main model hangs until the deadline, there would be no
	// time left for the fallback (observed on 2026-11-04).
	DefaultFallbackReserveSeconds = 20

	// OCR (docs/API.md, verification): "tesseract" reads PDF text layers
	// with Poppler and photos and scans with Tesseract; "none" switches the
	// check off (every file entry is then unchecked/ocr_unavailable).
	OCREngineTesseract   = "tesseract"
	OCREngineNone        = "none"
	DefaultOCREngine     = OCREngineTesseract
	DefaultTesseractLang = "deu"

	// Speech recognition (docs/API.md, 3.9): "whisper" converts recordings
	// with ffmpeg and asks the local whisper-server at WHISPER_URL; "none"
	// switches transcription off (/status says "off").
	SpeechEngineWhisper = "whisper"
	SpeechEngineNone    = "none"
	DefaultSpeechEngine = SpeechEngineWhisper
	DefaultWhisperURL   = "http://127.0.0.1:8178"
)

// renamedVariables maps the German variable names used until 2026-11-09 to
// their English replacements (docs/GLOSSARY.md). Most of them are optional:
// without this check, an old name left in .env would be ignored silently and
// the default would apply. Remove once nobody uses the old names anymore.
var renamedVariables = []struct{ old, new string }{
	{"ADRESSE", "ADDRESS"},
	{"GEMINI_MODELL", "GEMINI_MODEL"},
	{"GEMINI_EMBEDDING_MODELL", "GEMINI_EMBEDDING_MODEL"},
	{"GEMINI_MODELL_ERSATZ", "GEMINI_FALLBACK_MODEL"},
	{"GEMINI_ERSATZ_RESERVE_SEKUNDEN", "GEMINI_FALLBACK_RESERVE_SECONDS"},
}

// Config holds all settings the backend needs at startup.
type Config struct {
	MongoURI              string
	MongoDB               string
	GeminiKey             string
	GeminiModel           string
	GeminiFallbackModel   string        // optional, empty = no fallback model
	GeminiFallbackReserve time.Duration // part of the read time limit reserved for the fallback
	GeminiEmbeddingModel  string
	Address               string
	Port                  string
	OCREngine             string // OCREngineTesseract or OCREngineNone
	TesseractLang         string // language pack, e.g. "deu" or "deu+eng"
	SpeechEngine          string // SpeechEngineWhisper or SpeechEngineNone
	WhisperURL            string // base URL of the local whisper-server
}

// ListenAddress returns e.g. "127.0.0.1:8080". net.JoinHostPort puts IPv6
// addresses in square brackets correctly.
func (c Config) ListenAddress() string {
	return net.JoinHostPort(c.Address, c.Port)
}

// LocalOnly is true if the server can only be reached from this machine.
func (c Config) LocalOnly() bool {
	switch c.Address {
	case "127.0.0.1", "localhost", "::1":
		return true
	}
	return false
}

// LoadDotenv loads .env files into the environment. It looks in backend/.env
// and in the repository root (../.env when the server is started from
// backend/). A missing file is not an error, because in operation the
// variables can also be set directly. Variables that are already set are not
// overwritten; they take precedence.
func LoadDotenv() error {
	for _, path := range []string{".env", "../.env"} {
		err := godotenv.Load(path)
		if err != nil && !errors.Is(err, os.ErrNotExist) {
			// Deliberately without the file content, only path and kind of error.
			return fmt.Errorf("could not read %s: %w", path, err)
		}
	}
	return nil
}

// FindDotenv loads the first .env found from the current directory upwards.
// For tests: go test runs in the package directory (e.g.
// backend/internal/store), the .env lives in the repository root. Not
// finding a .env is not an error.
func FindDotenv() error {
	dir, err := os.Getwd()
	if err != nil {
		return err
	}
	for {
		path := filepath.Join(dir, ".env")
		if _, err := os.Stat(path); err == nil {
			if err := godotenv.Load(path); err != nil {
				return fmt.Errorf("could not read %s: %w", path, err)
			}
			return nil
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return nil
		}
		dir = parent
	}
}

// Load reads the configuration via getenv (os.Getenv in the program, a
// custom function in tests). If required variables are missing, the error
// names all missing names at once.
func Load(getenv func(string) string) (Config, error) {
	if err := checkRenamed(getenv); err != nil {
		return Config{}, err
	}

	c := Config{
		MongoURI:    strings.TrimSpace(getenv("MONGODB_URI")),
		MongoDB:     strings.TrimSpace(getenv("MONGODB_DB")),
		GeminiKey:   strings.TrimSpace(getenv("GEMINI_API_KEY")),
		GeminiModel: withDefault(getenv("GEMINI_MODEL"), DefaultGeminiModel),
		// Fallback only for reading and answering. There is deliberately none
		// for embeddings: vectors of two models are not comparable.
		GeminiFallbackModel: strings.TrimSpace(getenv("GEMINI_FALLBACK_MODEL")),
		GeminiEmbeddingModel: withDefault(getenv("GEMINI_EMBEDDING_MODEL"),
			DefaultGeminiEmbeddingModel),
		Address:       withDefault(getenv("ADDRESS"), DefaultAddress),
		Port:          withDefault(getenv("PORT"), DefaultPort),
		OCREngine:     withDefault(getenv("OCR_ENGINE"), DefaultOCREngine),
		TesseractLang: withDefault(getenv("TESSERACT_LANG"), DefaultTesseractLang),
		SpeechEngine:  withDefault(getenv("SPEECH_ENGINE"), DefaultSpeechEngine),
		WhisperURL:    withDefault(getenv("WHISPER_URL"), DefaultWhisperURL),
	}

	var missing []string
	if c.MongoURI == "" {
		missing = append(missing, "MONGODB_URI")
	}
	if c.MongoDB == "" {
		missing = append(missing, "MONGODB_DB")
	}
	if c.GeminiKey == "" {
		missing = append(missing, "GEMINI_API_KEY")
	}
	if len(missing) > 0 {
		return Config{}, fmt.Errorf("required variables missing: %s (see .env.example)",
			strings.Join(missing, ", "))
	}

	if p, err := strconv.Atoi(c.Port); err != nil || p < 1 || p > 65535 {
		return Config{}, errors.New("PORT must be a number between 1 and 65535")
	}
	if c.OCREngine != OCREngineTesseract && c.OCREngine != OCREngineNone {
		return Config{}, errors.New("OCR_ENGINE must be tesseract or none")
	}
	if c.SpeechEngine != SpeechEngineWhisper && c.SpeechEngine != SpeechEngineNone {
		return Config{}, errors.New("SPEECH_ENGINE must be whisper or none")
	}
	if u, err := url.Parse(c.WhisperURL); err != nil || (u.Scheme != "http" && u.Scheme != "https") || u.Host == "" {
		return Config{}, errors.New("WHISPER_URL must be a URL like http://127.0.0.1:8178")
	}

	// Whether the reserve fits the read time limit is checked by gemini.Neu
	// (the limit is known there). Here only: whole number, not negative.
	reserve := withDefault(getenv("GEMINI_FALLBACK_RESERVE_SECONDS"), strconv.Itoa(DefaultFallbackReserveSeconds))
	sec, err := strconv.Atoi(reserve)
	if err != nil || sec < 0 {
		return Config{}, errors.New("GEMINI_FALLBACK_RESERVE_SECONDS must be a whole number ≥ 0")
	}
	c.GeminiFallbackReserve = time.Duration(sec) * time.Second

	return c, nil
}

// checkRenamed fails if a variable is still set under its old German name.
// The error names all of them at once, with their new names, but no values.
func checkRenamed(getenv func(string) string) error {
	var found []string
	for _, r := range renamedVariables {
		if strings.TrimSpace(getenv(r.old)) != "" {
			found = append(found, r.old+" → "+r.new)
		}
	}
	if len(found) > 0 {
		return fmt.Errorf("environment variables were renamed, please update .env (see .env.example): %s",
			strings.Join(found, ", "))
	}
	return nil
}

func withDefault(value, def string) string {
	if v := strings.TrimSpace(value); v != "" {
		return v
	}
	return def
}
