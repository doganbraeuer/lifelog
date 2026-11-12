// The Lifelog server. Start from the backend/ directory:
//
//	go run ./cmd/server
package main

import (
	"context"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"lifelog/backend/internal/api"
	"lifelog/backend/internal/config"
	"lifelog/backend/internal/llm/gemini"
	"lifelog/backend/internal/ocr"
	"lifelog/backend/internal/ocr/poppler"
	"lifelog/backend/internal/ocr/tesseract"
	"lifelog/backend/internal/service"
	"lifelog/backend/internal/speech"
	"lifelog/backend/internal/speech/ffmpeg"
	"lifelog/backend/internal/speech/whisper"
	"lifelog/backend/internal/store"
)

func main() {
	if err := run(); err != nil {
		slog.Error("server stopped with an error", "error", err)
		os.Exit(1)
	}
}

func run() error {
	if err := config.LoadDotenv(); err != nil {
		return err
	}
	c, err := config.Load(os.Getenv)
	if err != nil {
		return err
	}

	// Ctrl+C (SIGINT) or a stop signal (SIGTERM) shut the server down cleanly.
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	db, err := store.Connect(ctx, c.MongoURI, c.MongoDB)
	if err != nil {
		return err
	}
	defer db.Disconnect(context.Background())
	slog.Info("MongoDB connected", "database", c.MongoDB)

	if err := db.CreateIndexes(ctx); err != nil {
		return err
	}
	// After CreateIndexes, because the chunk collection must exist. Not
	// fatal: without the index, upload and timeline work, only search does
	// not. Then create the index in the Atlas UI.
	if status, err := db.CreateVectorIndex(ctx); err != nil {
		slog.Warn("vector index missing, POST /questions only works after creating it in Atlas (backend/README.md, section vector index)",
			"index", store.VectorIndexName, "error", err)
	} else {
		slog.Info("vector index", "index", store.VectorIndexName, "status", status)
	}

	// The model provider is chosen here. Another provider = another package
	// under internal/llm that implements Reader, Embedder and Answerer.
	g, err := gemini.New(ctx, gemini.Settings{
		APIKey:          c.GeminiKey,
		Model:           c.GeminiModel,
		FallbackModel:   c.GeminiFallbackModel,
		FallbackReserve: c.GeminiFallbackReserve,
		EmbeddingModel:  c.GeminiEmbeddingModel,
	})
	if err != nil {
		return err
	}
	slog.Info("models", "read", c.GeminiModel, "fallback", c.GeminiFallbackModel,
		"reserve", c.GeminiFallbackReserve, "embedding", c.GeminiEmbeddingModel)

	recognizer := newRecognizer(c)
	transcriber := newTranscriber(ctx, c)

	server := &http.Server{
		Addr: c.ListenAddress(),
		Handler: api.NewRouter(api.Dependencies{
			DB:        db,
			Entries:   db,
			Uploader:  service.NewUploader(db, g, g, recognizer, store.NewID),
			Asker:     service.NewAsker(db, g, g),
			Corrector: service.NewCorrector(db),
			Deleter:   service.NewDeleter(db),

			Transcriber: transcriber,
		}),
		ReadHeaderTimeout: 10 * time.Second,
		ReadTimeout:       60 * time.Second,  // upload up to 10 MB, also over a slow mobile network
		WriteTimeout:      150 * time.Second, // upload with the vision LLM up to ~60 s, transcription up to 120 s
		IdleTimeout:       120 * time.Second,
	}

	if !c.LocalOnly() {
		slog.Warn("backend is reachable from the whole network (ADDRESS is not local). Use for testing only.",
			"address", c.Address)
	}

	serverErr := make(chan error, 1)
	go func() {
		slog.Info("server running", "address", "http://"+c.ListenAddress())
		serverErr <- server.ListenAndServe()
	}()

	select {
	case err := <-serverErr:
		// e.g. port already in use
		return err
	case <-ctx.Done():
		slog.Info("server shutting down …")
	}

	// Running requests may still finish within 10 s.
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	return server.Shutdown(shutdownCtx)
}

// newRecognizer builds the second reader for the verification (docs/API.md,
// verification). A missing program is never fatal: the server starts, and
// files it cannot read are stored unchecked (ocr_unavailable). Without
// Poppler that concerns PDFs, without Tesseract photos and scanned pages.
// nil if OCR_ENGINE=none.
func newRecognizer(c config.Config) ocr.Recognizer {
	if c.OCREngine == config.OCREngineNone {
		slog.Info("verification switched off (OCR_ENGINE=none), files stay unchecked")
		return nil
	}
	p := &ocr.Pipeline{Image: ocr.DefaultImageOptions()}
	if pdf, err := poppler.New(); err != nil {
		slog.Warn("Poppler missing, PDFs stay unchecked (backend/README.md, section verification)", "error", err)
	} else {
		p.PDF = pdf
	}
	if engine, err := tesseract.New(c.TesseractLang); err != nil {
		slog.Warn("Tesseract missing, photos and scans stay unchecked (backend/README.md, section verification)", "error", err)
	} else {
		p.Engine = engine
	}
	var names []string
	if p.PDF != nil {
		names = append(names, p.PDF.Name())
	}
	if p.Engine != nil {
		names = append(names, p.Engine.Name())
	}
	slog.Info("verification", "programs", names)
	return p
}

// newTranscriber builds the speech recognition (docs/API.md, 3.9). Like OCR,
// nothing is fatal: without ffmpeg or with whisper-server not running, the
// server starts, /status says speech "unavailable" and transcriptions
// answer 503. whisper-server may also be started later. nil if
// SPEECH_ENGINE=none.
func newTranscriber(ctx context.Context, c config.Config) api.Transcriber {
	if c.SpeechEngine == config.SpeechEngineNone {
		slog.Info("speech recognition switched off (SPEECH_ENGINE=none)")
		return nil
	}
	p := &speech.Pipeline{Engine: whisper.New(c.WhisperURL)}
	if conv, err := ffmpeg.New(); err != nil {
		slog.Warn("ffmpeg missing, transcription unavailable (backend/README.md, section speech)", "error", err)
	} else {
		p.Converter = conv
		slog.Info("speech recognition", "converter", conv.Name(), "engine", p.Engine.Name(), "url", c.WhisperURL)
	}
	if err := p.Engine.Ping(ctx); err != nil {
		slog.Warn("whisper-server not reachable yet; start it with backend/scripts/whisper-server.sh", "url", c.WhisperURL)
	}
	return p
}
