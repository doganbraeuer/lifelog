package gemini

// Diagnosis: a single read call, without retries and without fallback, with
// duration, finish reason and token usage.
//
//	LIFELOG_DIAGNOSE=1 LIFELOG_DIAGNOSE_VARIANT=full go test ./internal/llm/gemini/ -run Diagnose -v
//
// Variants (each builds on the previous one):
//
//	full       exactly the server's request (readRequest)
//	no-schema  without JSON schema and without JSON mode ("no structured output")
//	no-system  additionally without system instruction (like the playground test)
//
// More switches to compare other models:
//
//	LIFELOG_DIAGNOSE_MODEL=gemini-3.6-flash   instead of GEMINI_MODEL
//	LIFELOG_DIAGNOSE_FILE=lab_report.png      instead of lab_report.pdf (or an
//	                                          absolute path, synthetic only!)
//	LIFELOG_DIAGNOSE_THINKING=off             without ThinkingConfig (for models
//	                                          without thinkingLevel)
//
// Sends only synthetic files from testdata/.

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
	"unicode/utf8"

	"lifelog/backend/internal/config"
	"lifelog/backend/internal/core"
)

const diagnoseLimit = 120 * time.Second

func TestDiagnose_Read(t *testing.T) {
	if os.Getenv("LIFELOG_DIAGNOSE") != "1" {
		t.Skip("diagnosis skipped (set LIFELOG_DIAGNOSE=1)")
	}
	variant := os.Getenv("LIFELOG_DIAGNOSE_VARIANT")
	if variant == "" {
		variant = "full"
	}

	if err := config.FindDotenv(); err != nil {
		t.Fatal(err)
	}
	c, err := config.Load(os.Getenv)
	if err != nil {
		t.Fatal(err)
	}
	g, err := New(context.Background(), Settings{
		APIKey: c.GeminiKey, Model: c.GeminiModel, EmbeddingModel: c.GeminiEmbeddingModel,
	})
	if err != nil {
		t.Fatal(err)
	}

	model := c.GeminiModel
	if m := os.Getenv("LIFELOG_DIAGNOSE_MODEL"); m != "" {
		model = m
	}
	fileName := os.Getenv("LIFELOG_DIAGNOSE_FILE")
	if fileName == "" {
		fileName = "lab_report.pdf"
	}
	// An absolute path allows converted variants outside the repository
	// (e.g. the same page as JPEG); otherwise a file from testdata/.
	var data []byte
	if filepath.IsAbs(fileName) {
		if data, err = os.ReadFile(fileName); err != nil {
			t.Fatal(err)
		}
	} else {
		data = testFile(t, fileName)
	}
	mimeType, err := core.DetectFileType(data)
	if err != nil {
		t.Fatal(err)
	}

	contents, cfg := readRequest(data, mimeType, core.AllowedTypes("health"))
	thinking := string(cfg.ThinkingConfig.ThinkingLevel)
	if os.Getenv("LIFELOG_DIAGNOSE_THINKING") == "off" {
		cfg.ThinkingConfig, thinking = nil, "off"
	}
	switch variant {
	case "full":
	case "no-schema":
		cfg.ResponseJsonSchema, cfg.ResponseMIMEType = nil, ""
	case "no-system":
		cfg.ResponseJsonSchema, cfg.ResponseMIMEType = nil, ""
		cfg.SystemInstruction = nil
	default:
		t.Fatalf("unknown variant %q", variant)
	}
	t.Logf("model %s, variant %s, thinking %s, limit %v, %s (%s, %d bytes)",
		model, variant, thinking, diagnoseLimit, fileName, mimeType, len(data))

	ctx, cancel := context.WithTimeout(context.Background(), diagnoseLimit)
	defer cancel()

	start := time.Now()
	resp, err := g.generate(ctx, model, contents, cfg)
	duration := time.Since(start).Round(time.Millisecond)

	if err != nil {
		t.Fatalf("after %v: %v", duration, translateError(err, c.GeminiKey))
	}

	t.Logf("duration: %v", duration)
	t.Logf("model version: %s", resp.ModelVersion)
	if len(resp.Candidates) > 0 && resp.Candidates[0] != nil {
		t.Logf("finish reason: %q", resp.Candidates[0].FinishReason)
	}
	if fb := resp.PromptFeedback; fb != nil && fb.BlockReason != "" {
		t.Logf("blocked: %s", fb.BlockReason)
	}
	if u := resp.UsageMetadata; u != nil {
		t.Logf("tokens: input %d, thinking %d, output %d, total %d",
			u.PromptTokenCount, u.ThoughtsTokenCount, u.CandidatesTokenCount, u.TotalTokenCount)
	}

	text := resp.Text()
	t.Logf("response: %d characters", utf8.RuneCountInString(text))
	t.Logf("start:\n%s", shorten(text, 400, false))
	if utf8.RuneCountInString(text) > 800 {
		t.Logf("end:\n%s", shorten(text, 300, true))
	}

	if variant == "full" {
		x, err := extractionFromResponse(resp)
		if err != nil {
			t.Errorf("response not usable: %v", err)
			return
		}
		entry, dropped, err := core.CheckExtraction(x, "health")
		if err != nil {
			t.Errorf("check: %v", err)
			return
		}
		t.Logf("checked: type=%s date=%s issuer=%s, %d facts, dropped %v",
			entry.Type, deref(entry.DocumentDate), deref(entry.Issuer), len(entry.Facts), dropped)
		// The values are synthetic (testdata/), so they may go into the test log.
		for _, f := range entry.Facts {
			t.Logf("  %s = %q %s", f.Name, f.Value, deref(f.Unit))
		}
	}
}

// shorten returns the first (or last) n characters.
func shorten(s string, n int, end bool) string {
	r := []rune(strings.TrimSpace(s))
	if len(r) <= n {
		return string(r)
	}
	if end {
		return "…" + string(r[len(r)-n:])
	}
	return string(r[:n]) + "…"
}
