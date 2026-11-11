// Package gemini implements the interfaces of internal/llm with Google's
// Gemini API. Only this package knows the Gemini SDK.
package gemini

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"time"

	"google.golang.org/genai"

	"lifelog/backend/internal/core"
	"lifelog/backend/internal/llm"
)

// Total limits per operation, including retries and the fallback model.
//
//	upload   (app waits 90 s): read 50 s + embed chunks 20 s
//	question (app waits 60 s): embed question 10 s + answer 45 s
const (
	readTimeout          = 50 * time.Second
	embedTimeout         = 20 * time.Second
	embedQuestionTimeout = 10 * time.Second
	answerTimeout        = 45 * time.Second

	// minMainTime: at least this much time remains for the main model, no
	// matter how large the reserve for the fallback is set.
	minMainTime = 10 * time.Second
)

// maxFallbackReserve: the reserve applies to reading and answering; it must
// fit the shorter of the two limits.
const maxFallbackReserve = min(readTimeout, answerTimeout) - minMainTime

// Settings for New.
type Settings struct {
	APIKey         string
	Model          string // main model for reading
	FallbackModel  string // optional: tried once if Model stays unavailable
	EmbeddingModel string // no fallback: embeddings of different models do not match

	// FallbackReserve: this part of the limit (reading and answering) is
	// reserved for the fallback model. The main model, including retries,
	// must finish before. Only applies if FallbackModel is set.
	FallbackReserve time.Duration
}

// The two calls that really go to the network. As functions, so tests can
// replace them and check retries and the fallback model without network.
type (
	generateFunc func(ctx context.Context, model string, contents []*genai.Content, cfg *genai.GenerateContentConfig) (*genai.GenerateContentResponse, error)
	embedFunc    func(ctx context.Context, model string, contents []*genai.Content, cfg *genai.EmbedContentConfig) (*genai.EmbedContentResponse, error)
)

// Gemini is the connection to the Gemini API.
type Gemini struct {
	s             Settings
	generate      generateFunc
	embedAPI      embedFunc
	newRetrier    func(label string) retrier
	readTimeout   time.Duration // fields instead of constants, so tests do not wait 50 s
	answerTimeout time.Duration
}

var (
	_ llm.Reader   = (*Gemini)(nil)
	_ llm.Embedder = (*Gemini)(nil)
	_ llm.Answerer = (*Gemini)(nil)
)

// New creates the client. No network call happens yet.
func New(ctx context.Context, s Settings) (*Gemini, error) {
	client, err := genai.NewClient(ctx, &genai.ClientConfig{
		APIKey:  s.APIKey,
		Backend: genai.BackendGeminiAPI,
		// No RetryOptions in the SDK: we control retries ourselves
		// (retry.go), so they only apply to 429/503 and respect the total
		// limit.
	})
	if err != nil {
		return nil, translateError(err, s.APIKey)
	}
	if s.FallbackModel == s.Model {
		s.FallbackModel = "" // the same name would only be a fourth attempt
	}
	if s.FallbackReserve < 0 || s.FallbackReserve > maxFallbackReserve {
		return nil, fmt.Errorf("GEMINI_FALLBACK_RESERVE_SECONDS must be between 0 and %d (limits: read %v, answer %v; the main model needs at least %v)",
			int(maxFallbackReserve.Seconds()), readTimeout, answerTimeout, minMainTime)
	}
	return &Gemini{
		s:             s,
		generate:      client.Models.GenerateContent,
		embedAPI:      client.Models.EmbedContent,
		newRetrier:    defaultRetrier,
		readTimeout:   readTimeout,
		answerTimeout: answerTimeout,
	}, nil
}

// ReadDocument sends the document to the vision LLM and returns the
// unchecked extraction. Retries and fallback: see generateWithFallback.
func (g *Gemini) ReadDocument(ctx context.Context, data []byte, mimeType string, allowedTypes []string) (core.Extraction, error) {
	contents, cfg := readRequest(data, mimeType, allowedTypes)
	resp, err := g.generateWithFallback(ctx, g.readTimeout, "read", contents, cfg)
	if err != nil {
		return core.Extraction{}, err
	}
	return extractionFromResponse(resp)
}

// ReadText classifies an entered text (text import, API.md 3.1): date,
// issuer, type and facts. FullText stays empty and Readable is true; the
// full text is the entered text, which the caller sets. Same retries, same
// fallback and same limit as ReadDocument.
func (g *Gemini) ReadText(ctx context.Context, text string, allowedTypes []string) (core.Extraction, error) {
	contents, cfg := textRequest(text, allowedTypes)
	resp, err := g.generateWithFallback(ctx, g.readTimeout, "read text", contents, cfg)
	if err != nil {
		return core.Extraction{}, err
	}
	answer, err := textFromResponse(resp)
	if err != nil {
		return core.Extraction{}, err
	}
	return textExtractionFromText(answer)
}

// Answer answers a question only from the sources. Same retries and same
// fallback as reading.
func (g *Gemini) Answer(ctx context.Context, question string, sources []core.Source) (string, error) {
	contents, cfg := answerRequest(question, sources)
	resp, err := g.generateWithFallback(ctx, g.answerTimeout, "answer", contents, cfg)
	if err != nil {
		return "", err
	}
	return textFromResponse(resp)
}

// generateWithFallback is the shared procedure for reading and answering:
// main model, up to two retries on 429/503. If it stays unavailable and a
// fallback model is configured, that one is tried once. Everything within
// limit.
//
// Time split with a fallback model (reading, default 30 s + 20 s):
//
//	|---- main model + retries ----|---- reserve: fallback ----|
//	0                             30 s                        50 s
//
// Without this split, a hanging main model could use up the whole limit,
// and the fallback would never get its turn.
func (g *Gemini) generateWithFallback(ctx context.Context, limit time.Duration, task string,
	contents []*genai.Content, cfg *genai.GenerateContentConfig) (*genai.GenerateContentResponse, error) {
	ctx, cancel := context.WithTimeout(ctx, limit)
	defer cancel()

	mainCtx := ctx
	if g.s.FallbackModel != "" && g.s.FallbackReserve > 0 {
		deadline, _ := ctx.Deadline() // always set by WithTimeout
		var mainCancel context.CancelFunc
		mainCtx, mainCancel = context.WithDeadline(ctx, deadline.Add(-g.s.FallbackReserve))
		defer mainCancel()
	}

	var resp *genai.GenerateContentResponse
	err := g.newRetrier(task+" "+g.s.Model).run(mainCtx, func(ctx context.Context) error {
		var err error
		resp, err = g.generate(ctx, g.s.Model, contents, cfg)
		return err
	})
	err = translateError(err, g.s.APIKey)

	// ctx.Err() == nil: the total limit is still running and the app is
	// still there. (Only mainCtx may have expired; then the reserve is left.)
	if errors.Is(err, llm.ErrUnavailable) && g.s.FallbackModel != "" && ctx.Err() == nil {
		remaining := time.Duration(0)
		if deadline, ok := ctx.Deadline(); ok {
			remaining = time.Until(deadline).Round(time.Millisecond)
		}
		// The main model's error is already translated and masked.
		slog.Warn("main model unavailable, trying the fallback model once",
			"task", task, "model", g.s.Model, "error", err, "fallback", g.s.FallbackModel, "remaining", remaining)
		resp, err = g.generate(ctx, g.s.FallbackModel, contents, cfg)
		err = translateError(err, g.s.APIKey)
	}
	return resp, err
}

// readRequest builds the content and configuration for reading a document.
// A function of its own, so the diagnostic test (diagnose_test.go) sends
// exactly the same request as the server.
func readRequest(data []byte, mimeType string, allowedTypes []string) ([]*genai.Content, *genai.GenerateContentConfig) {
	contents := []*genai.Content{genai.NewContentFromParts([]*genai.Part{
		genai.NewPartFromBytes(data, mimeType),
		genai.NewPartFromText("Übertrage dieses Dokument nach den Regeln in JSON."),
	}, genai.RoleUser)}
	cfg := &genai.GenerateContentConfig{
		SystemInstruction:  genai.NewContentFromText(readInstruction(allowedTypes), genai.RoleUser),
		ResponseMIMEType:   "application/json",
		ResponseJsonSchema: readSchema(allowedTypes),
		// Transcribing needs little "thinking". In the AI Studio playground
		// (2026-11-04, same PDF): LOW 3.4 s, MEDIUM 5.9 s.
		ThinkingConfig: &genai.ThinkingConfig{ThinkingLevel: genai.ThinkingLevelLow},
	}
	return contents, cfg
}

// textRequest builds the content and configuration for the text import. The
// text is a separate part of the message, apart from the task; the system
// instruction says that it is content and not an instruction.
func textRequest(text string, allowedTypes []string) ([]*genai.Content, *genai.GenerateContentConfig) {
	contents := []*genai.Content{genai.NewContentFromParts([]*genai.Part{
		genai.NewPartFromText("Ordne den folgenden Text nach den Regeln in JSON ein."),
		genai.NewPartFromText(text),
	}, genai.RoleUser)}
	cfg := &genai.GenerateContentConfig{
		SystemInstruction:  genai.NewContentFromText(textInstruction(allowedTypes), genai.RoleUser),
		ResponseMIMEType:   "application/json",
		ResponseJsonSchema: textSchema(allowedTypes),
		ThinkingConfig:     &genai.ThinkingConfig{ThinkingLevel: genai.ThinkingLevelLow},
	}
	return contents, cfg
}

// EmbedChunks computes one vector per chunk, in batches. The total limit
// applies to all batches together.
func (g *Gemini) EmbedChunks(ctx context.Context, texts []string) ([][]float32, error) {
	ctx, cancel := context.WithTimeout(ctx, embedTimeout)
	defer cancel()

	vectors := make([][]float32, 0, len(texts))
	for _, b := range batches(len(texts), batchSize) {
		inputs := make([]string, 0, b[1]-b[0])
		for _, t := range texts[b[0]:b[1]] {
			inputs = append(inputs, documentInput(t))
		}
		v, err := g.embed(ctx, inputs)
		if err != nil {
			return nil, err
		}
		vectors = append(vectors, v...)
	}
	return vectors, nil
}

// EmbedQuestion computes the vector for a search question. Retries on
// 429/503 yes, fallback model never (the vector spaces of different models
// do not match).
func (g *Gemini) EmbedQuestion(ctx context.Context, question string) ([]float32, error) {
	ctx, cancel := context.WithTimeout(ctx, embedQuestionTimeout)
	defer cancel()

	v, err := g.embed(ctx, []string{questionInput(question)})
	if err != nil {
		return nil, err
	}
	return v[0], nil
}

// embed: one call with retries on 429/503, without fallback model.
func (g *Gemini) embed(ctx context.Context, inputs []string) ([][]float32, error) {
	dimensions := int32(llm.EmbeddingDimensions)
	cfg := &genai.EmbedContentConfig{OutputDimensionality: &dimensions}
	contents := embeddingContents(inputs)

	var resp *genai.EmbedContentResponse
	err := g.newRetrier("embed "+g.s.EmbeddingModel).run(ctx, func(ctx context.Context) error {
		var err error
		resp, err = g.embedAPI(ctx, g.s.EmbeddingModel, contents, cfg)
		return err
	})
	if err != nil {
		return nil, translateError(err, g.s.APIKey)
	}
	return vectorsFromResponse(resp, len(inputs))
}
