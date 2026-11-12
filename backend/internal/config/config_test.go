package config

import (
	"strings"
	"testing"
	"time"
)

// env builds a getenv function from a map, so tests need no real
// environment variables.
func env(values map[string]string) func(string) string {
	return func(name string) string { return values[name] }
}

func complete() map[string]string {
	return map[string]string{
		"MONGODB_URI":    "mongodb+srv://user:SECRET@example.invalid/",
		"MONGODB_DB":     "lifelog_test",
		"GEMINI_API_KEY": "SECRET-KEY",
	}
}

func TestLoad_Defaults(t *testing.T) {
	c, err := Load(env(complete()))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got := c.ListenAddress(); got != "127.0.0.1:8080" {
		t.Errorf("ListenAddress = %q, want 127.0.0.1:8080", got)
	}
	if !c.LocalOnly() {
		t.Error("default must be reachable only locally")
	}
	if c.GeminiModel != DefaultGeminiModel || c.GeminiEmbeddingModel != DefaultGeminiEmbeddingModel {
		t.Errorf("models = %q / %q", c.GeminiModel, c.GeminiEmbeddingModel)
	}
	if c.GeminiFallbackModel != "" {
		t.Errorf("fallback model is optional and empty by default, not %q", c.GeminiFallbackModel)
	}
	if c.GeminiFallbackReserve != 20*time.Second {
		t.Errorf("reserve = %v, want 20s", c.GeminiFallbackReserve)
	}
}

func TestLoad_FallbackReserve(t *testing.T) {
	for value, want := range map[string]time.Duration{"15": 15 * time.Second, "0": 0, " 25 ": 25 * time.Second} {
		e := complete()
		e["GEMINI_FALLBACK_RESERVE_SECONDS"] = value
		c, err := Load(env(e))
		if err != nil || c.GeminiFallbackReserve != want {
			t.Errorf("%q: %v, %v; want %v", value, c.GeminiFallbackReserve, err, want)
		}
	}
	for _, invalid := range []string{"-1", "abc", "2.5", "20s"} {
		e := complete()
		e["GEMINI_FALLBACK_RESERVE_SECONDS"] = invalid
		if _, err := Load(env(e)); err == nil {
			t.Errorf("%q: want error", invalid)
		}
	}
}

func TestLoad_OverrideModels(t *testing.T) {
	e := complete()
	e["GEMINI_MODEL"] = " other-model "
	e["GEMINI_EMBEDDING_MODEL"] = "other-embedding"
	e["GEMINI_FALLBACK_MODEL"] = " fallback-model "
	c, err := Load(env(e))
	if err != nil {
		t.Fatal(err)
	}
	if c.GeminiModel != "other-model" || c.GeminiEmbeddingModel != "other-embedding" {
		t.Errorf("models = %q / %q", c.GeminiModel, c.GeminiEmbeddingModel)
	}
	if c.GeminiFallbackModel != "fallback-model" {
		t.Errorf("fallback = %q", c.GeminiFallbackModel)
	}
}

func TestLoad_OverrideAddressAndPort(t *testing.T) {
	e := complete()
	e["ADDRESS"] = "0.0.0.0"
	e["PORT"] = "9090"
	c, err := Load(env(e))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got := c.ListenAddress(); got != "0.0.0.0:9090" {
		t.Errorf("ListenAddress = %q, want 0.0.0.0:9090", got)
	}
	if c.LocalOnly() {
		t.Error("0.0.0.0 must not count as local only")
	}
}

func TestLoad_WhitespaceCountsAsEmpty(t *testing.T) {
	e := complete()
	e["ADDRESS"] = "   "
	e["MONGODB_DB"] = "  "
	_, err := Load(env(e))
	if err == nil || !strings.Contains(err.Error(), "MONGODB_DB") {
		t.Fatalf("want error about MONGODB_DB, got: %v", err)
	}
}

func TestLoad_MissingVariables(t *testing.T) {
	_, err := Load(env(map[string]string{}))
	if err == nil {
		t.Fatal("want an error")
	}
	for _, name := range []string{"MONGODB_URI", "MONGODB_DB", "GEMINI_API_KEY"} {
		if !strings.Contains(err.Error(), name) {
			t.Errorf("error message does not name %s: %q", name, err)
		}
	}
}

func TestLoad_SecretsNeverInErrorMessage(t *testing.T) {
	e := complete()
	delete(e, "MONGODB_DB") // causes an error while secrets are set
	e["PORT"] = "not-a-number"
	_, err := Load(env(e))
	if err == nil {
		t.Fatal("want an error")
	}
	for _, secret := range []string{"SECRET", "SECRET-KEY"} {
		if strings.Contains(err.Error(), secret) {
			t.Errorf("error message contains a secret: %q", err)
		}
	}

	e = complete()
	e["PORT"] = "not-a-number"
	_, err = Load(env(e))
	if err == nil || strings.Contains(err.Error(), "SECRET") {
		t.Errorf("port error missing or contains a secret: %v", err)
	}
}

func TestLoad_InvalidPort(t *testing.T) {
	for _, port := range []string{"abc", "0", "70000", "-1"} {
		e := complete()
		e["PORT"] = port
		if _, err := Load(env(e)); err == nil {
			t.Errorf("PORT=%q: want error", port)
		}
	}
}

// An old German name left in .env must not be ignored silently: most of
// these variables are optional, so the default would apply without notice.
func TestLoad_RenamedVariablesFail(t *testing.T) {
	for _, r := range renamedVariables {
		e := complete()
		e[r.old] = "secret-value"
		_, err := Load(env(e))
		if err == nil {
			t.Errorf("%s set: want error", r.old)
			continue
		}
		msg := err.Error()
		if !strings.Contains(msg, r.old) || !strings.Contains(msg, r.new) {
			t.Errorf("%s: error should name old and new name: %q", r.old, msg)
		}
		if strings.Contains(msg, "secret-value") {
			t.Errorf("%s: error message contains the value: %q", r.old, msg)
		}
	}
}

func TestLoad_RenamedVariablesAllNamed(t *testing.T) {
	e := complete()
	e["ADRESSE"] = "0.0.0.0"
	e["GEMINI_MODELL"] = "x"
	_, err := Load(env(e))
	if err == nil || !strings.Contains(err.Error(), "ADRESSE") || !strings.Contains(err.Error(), "GEMINI_MODELL") {
		t.Fatalf("want one error naming both old variables, got: %v", err)
	}
}

func TestLoad_RenamedVariableEmptyIsIgnored(t *testing.T) {
	// An empty old line like "GEMINI_MODELL=" from the old .env.example is harmless.
	e := complete()
	e["GEMINI_MODELL"] = "  "
	if _, err := Load(env(e)); err != nil {
		t.Fatalf("empty old variable must not fail: %v", err)
	}
}

func TestLoad_OCR(t *testing.T) {
	c, err := Load(env(complete()))
	if err != nil || c.OCREngine != "tesseract" || c.TesseractLang != "deu" {
		t.Errorf("defaults: %q, %q, %v", c.OCREngine, c.TesseractLang, err)
	}
	e := complete()
	e["OCR_ENGINE"], e["TESSERACT_LANG"] = " none ", "deu+eng"
	c, err = Load(env(e))
	if err != nil || c.OCREngine != "none" || c.TesseractLang != "deu+eng" {
		t.Errorf("set: %q, %q, %v", c.OCREngine, c.TesseractLang, err)
	}
	e["OCR_ENGINE"] = "paddle"
	if _, err := Load(env(e)); err == nil || !strings.Contains(err.Error(), "OCR_ENGINE") {
		t.Errorf("unknown engine: %v", err)
	}
}

func TestLoad_Speech(t *testing.T) {
	c, err := Load(env(complete()))
	if err != nil || c.SpeechEngine != "whisper" || c.WhisperURL != "http://127.0.0.1:8178" {
		t.Errorf("defaults: %q, %q, %v", c.SpeechEngine, c.WhisperURL, err)
	}
	e := complete()
	e["SPEECH_ENGINE"], e["WHISPER_URL"] = " none ", "http://192.168.0.20:9000"
	c, err = Load(env(e))
	if err != nil || c.SpeechEngine != "none" || c.WhisperURL != "http://192.168.0.20:9000" {
		t.Errorf("set: %q, %q, %v", c.SpeechEngine, c.WhisperURL, err)
	}
	for name, v := range map[string][2]string{
		"unknown engine":   {"vosk", ""},
		"no scheme":        {"", "127.0.0.1:8178"},
		"other scheme":     {"", "ftp://127.0.0.1:8178"},
		"not a URL at all": {"", "http://"},
	} {
		e := complete()
		e["SPEECH_ENGINE"], e["WHISPER_URL"] = v[0], v[1]
		if _, err := Load(env(e)); err == nil {
			t.Errorf("%s: no error", name)
		}
	}
}
