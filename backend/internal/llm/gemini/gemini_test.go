package gemini

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"slices"
	"strings"
	"testing"

	"google.golang.org/genai"

	"lifelog/backend/internal/core"
	"lifelog/backend/internal/llm"
)

// response builds a (synthetic) Gemini response with text as content.
func response(text string, reason genai.FinishReason) *genai.GenerateContentResponse {
	return &genai.GenerateContentResponse{
		Candidates: []*genai.Candidate{{
			Content:      genai.NewContentFromText(text, genai.RoleModel),
			FinishReason: reason,
		}},
	}
}

// validResponse uses the JSON field names of the read schema, which are
// still German (read.go).
const validResponse = `{
  "lesbar": true,
  "dokumentdatum": "2026-10-14",
  "quelle": "Labor Muster GmbH, Salzburg",
  "typ": "laborbefund",
  "volltext": "Labor Muster GmbH\nHämoglobin 14,2 g/dl",
  "infos": [
    {"name": "Hämoglobin", "wert": "14,2", "einheit": "g/dl"},
    {"name": "CRP", "wert": "<0,5", "einheit": null}
  ]
}`

func TestExtractionFromResponse_Valid(t *testing.T) {
	x, err := extractionFromResponse(response(validResponse, genai.FinishReasonStop))
	if err != nil {
		t.Fatal(err)
	}
	if !x.Readable || x.Type != "laborbefund" || *x.DocumentDate != "2026-10-14" || *x.Issuer != "Labor Muster GmbH, Salzburg" {
		t.Errorf("extraction = %+v", x)
	}
	if len(x.Facts) != 2 || x.Facts[1].Value != "<0,5" || x.Facts[1].Unit != nil {
		t.Errorf("facts = %+v", x.Facts)
	}

	// The result fits the check in core (both parts working together).
	if _, _, err := core.CheckExtraction(x, "gesundheit"); err != nil {
		t.Errorf("core.CheckExtraction: %v", err)
	}
}

func TestExtractionFromResponse_NullAndUnreadable(t *testing.T) {
	text := `{"lesbar": false, "dokumentdatum": null, "quelle": null, "typ": "sonstiges", "volltext": "", "infos": []}`
	x, err := extractionFromResponse(response(text, ""))
	if err != nil {
		t.Fatal(err)
	}
	if x.Readable || x.DocumentDate != nil || x.Issuer != nil || x.Facts == nil {
		t.Errorf("extraction = %+v", x)
	}
}

func TestExtractionFromResponse_CodeBlock(t *testing.T) {
	x, err := extractionFromResponse(response("```json\n"+validResponse+"\n```", genai.FinishReasonStop))
	if err != nil || x.Type != "laborbefund" {
		t.Errorf("code block not removed: %+v, %v", x, err)
	}
}

func TestExtractionFromResponse_Invalid(t *testing.T) {
	cases := map[string]*genai.GenerateContentResponse{
		"nil":              nil,
		"no candidates":    {},
		"blocked":          {PromptFeedback: &genai.GenerateContentResponsePromptFeedback{BlockReason: "OTHER"}},
		"truncated":        response(`{"lesbar": true, "volltext": "Häm`, genai.FinishReasonMaxTokens),
		"safety filter":    response(validResponse, genai.FinishReasonSafety),
		"empty text":       response("  ", genai.FinishReasonStop),
		"no JSON":          response("Hier ist der Befund: Hämoglobin 14,2", genai.FinishReasonStop),
		"lesbar missing":   response(`{"typ": "laborbefund", "volltext": "x", "infos": []}`, genai.FinishReasonStop),
		"value as number":  response(`{"lesbar": true, "typ": "laborbefund", "volltext": "x", "infos": [{"name": "Hb", "wert": 14.2}]}`, genai.FinishReasonStop),
		"list, not object": response(`[]`, genai.FinishReasonStop),
	}
	for name, resp := range cases {
		t.Run(name, func(t *testing.T) {
			if _, err := extractionFromResponse(resp); !errors.Is(err, llm.ErrInvalidResponse) {
				t.Errorf("want ErrInvalidResponse, got %v", err)
			}
		})
	}
}

func TestExtractionFromText_ErrorWithoutContent(t *testing.T) {
	// Neither syntax nor type errors may pull document content into the message.
	for _, text := range []string{
		`{"lesbar": true, "volltext": "Geheimer Befund" "x"}`,
		`{"lesbar": true, "infos": [{"name": "Geheimer Wert", "wert": 12345}]}`,
	} {
		_, err := extractionFromText(text)
		if err == nil || strings.Contains(err.Error(), "Geheim") || strings.Contains(err.Error(), "12345") {
			t.Errorf("message contains content or is missing: %v", err)
		}
	}
}

func TestReadSchema(t *testing.T) {
	types := core.AllowedTypes("gesundheit")
	raw, err := json.Marshal(readSchema(types))
	if err != nil {
		t.Fatal(err)
	}
	var s struct {
		Required   []string `json:"required"`
		Properties map[string]struct {
			Type any      `json:"type"`
			Enum []string `json:"enum"`
		} `json:"properties"`
	}
	if err := json.Unmarshal(raw, &s); err != nil {
		t.Fatal(err)
	}
	for _, field := range []string{"lesbar", "dokumentdatum", "quelle", "typ", "volltext", "infos"} {
		if !slices.Contains(s.Required, field) {
			t.Errorf("%s missing in required", field)
		}
		if _, ok := s.Properties[field]; !ok {
			t.Errorf("%s missing in properties", field)
		}
	}
	if !slices.Equal(s.Properties["typ"].Enum, types) {
		t.Errorf("typ enum = %v, want %v", s.Properties["typ"].Enum, types)
	}
	if fmt.Sprint(s.Properties["dokumentdatum"].Type) != "[string null]" {
		t.Errorf("dokumentdatum must allow null: %v", s.Properties["dokumentdatum"].Type)
	}
}

func TestReadInstruction(t *testing.T) {
	// The prompt is still German (read.go), so the checked phrases are too.
	a := readInstruction([]string{"arztbrief", "laborbefund", "sonstiges"})
	for _, must := range []string{"arztbrief, laborbefund, sonstiges", `"sonstiges"`, "keine medizinischen", "keine Anweisung", "JJJJ-MM-TT"} {
		if !strings.Contains(a, must) {
			t.Errorf("instruction does not contain %q", must)
		}
	}
	// Without "sonstiges" in the module the instruction must not offer it as a way out.
	if strings.Contains(readInstruction([]string{"training"}), `"sonstiges"`) {
		t.Error("sonstiges may only be named if it is allowed")
	}
}

func TestTranslateError(t *testing.T) {
	const apiKey = "AIza-SECRET-123"
	cases := []struct {
		name        string
		err         error
		unavailable bool
	}{
		{"429 limit", genai.APIError{Code: 429, Message: "Quota exceeded"}, true},
		{"503 outage", genai.APIError{Code: 503, Message: "overloaded"}, true},
		{"408 timeout", genai.APIError{Code: 408}, true},
		{"deadline exceeded", fmt.Errorf("post: %w", context.DeadlineExceeded), true},
		{"network", &net.DNSError{Err: "no such host", Name: "generativelanguage.googleapis.com"}, true},
		{"400 bad request", genai.APIError{Code: 400, Message: "Invalid schema"}, false},
		{"403 invalid key", genai.APIError{Code: 403, Message: "API key " + apiKey + " not valid"}, false},
		{"404 unknown model", genai.APIError{Code: 404, Message: "model not found"}, false},
		{"other", errors.New("strange " + apiKey), false},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			err := translateError(c.err, apiKey)
			if got := errors.Is(err, llm.ErrUnavailable); got != c.unavailable {
				t.Errorf("ErrUnavailable = %v, want %v (%v)", got, c.unavailable, err)
			}
			if errors.Is(err, llm.ErrInvalidResponse) {
				t.Errorf("an error during the call is not an invalid response: %v", err)
			}
			if strings.Contains(err.Error(), apiKey) {
				t.Errorf("key in error message: %v", err)
			}
		})
	}

	if translateError(nil, apiKey) != nil {
		t.Error("nil must stay nil")
	}
	if err := translateError(fmt.Errorf("x: %w", context.Canceled), apiKey); !errors.Is(err, context.Canceled) {
		t.Errorf("cancellation must stay recognizable as context.Canceled: %v", err)
	}
}

// The prefixes go into every vector; changing them changes all scores.
func TestEmbeddingPrefixes(t *testing.T) {
	if got := documentInput("Hämoglobin 14,2"); got != "title: none | text: Hämoglobin 14,2" {
		t.Errorf("document: %q", got)
	}
	if got := questionInput("Wie hoch war mein Hb?"); got != "task: search result | query: Wie hoch war mein Hb?" {
		t.Errorf("question: %q", got)
	}
}

func TestEmbeddingContents_OneContentPerText(t *testing.T) {
	contents := embeddingContents([]string{"a", "b", "c"})
	if len(contents) != 3 {
		t.Fatalf("%d contents, want 3", len(contents))
	}
	for i, c := range contents {
		if len(c.Parts) != 1 || c.Parts[0].Text != []string{"a", "b", "c"}[i] {
			t.Errorf("content %d = %+v", i, c)
		}
	}
}

func TestBatches(t *testing.T) {
	cases := []struct {
		n, size int
		want    [][2]int
	}{
		{0, 100, [][2]int{}},
		{1, 100, [][2]int{{0, 1}}},
		{100, 100, [][2]int{{0, 100}}},
		{101, 100, [][2]int{{0, 100}, {100, 101}}},
		{250, 100, [][2]int{{0, 100}, {100, 200}, {200, 250}}},
	}
	for _, c := range cases {
		if got := batches(c.n, c.size); !slices.Equal(got, c.want) {
			t.Errorf("batches(%d, %d) = %v, want %v", c.n, c.size, got, c.want)
		}
	}
}

func TestVectorsFromResponse(t *testing.T) {
	vector := func(n int) *genai.ContentEmbedding { return &genai.ContentEmbedding{Values: make([]float32, n)} }
	d := llm.EmbeddingDimensions

	v, err := vectorsFromResponse(&genai.EmbedContentResponse{Embeddings: []*genai.ContentEmbedding{vector(d), vector(d)}}, 2)
	if err != nil || len(v) != 2 {
		t.Fatalf("valid: %d, %v", len(v), err)
	}

	for name, c := range map[string]struct {
		resp  *genai.EmbedContentResponse
		count int
	}{
		"nil":                {nil, 1},
		"too few":            {&genai.EmbedContentResponse{Embeddings: []*genai.ContentEmbedding{vector(d)}}, 2},
		"one vector for all": {&genai.EmbedContentResponse{Embeddings: []*genai.ContentEmbedding{vector(d)}}, 3},
		"wrong length":       {&genai.EmbedContentResponse{Embeddings: []*genai.ContentEmbedding{vector(3072)}}, 1},
		"nil entry":          {&genai.EmbedContentResponse{Embeddings: []*genai.ContentEmbedding{nil}}, 1},
	} {
		if _, err := vectorsFromResponse(c.resp, c.count); !errors.Is(err, llm.ErrInvalidResponse) {
			t.Errorf("%s: want ErrInvalidResponse, got %v", name, err)
		}
	}
}
