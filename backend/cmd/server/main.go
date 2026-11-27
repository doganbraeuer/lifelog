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
	"sync/atomic"
	"syscall"
	"time"

	"lifelog/backend/internal/api"
	"lifelog/backend/internal/auth"
	"lifelog/backend/internal/config"
	"lifelog/backend/internal/llm"
	"lifelog/backend/internal/llm/gemini"
	"lifelog/backend/internal/ocr"
	"lifelog/backend/internal/ocr/poppler"
	"lifelog/backend/internal/ocr/tesseract"
	"lifelog/backend/internal/service"
	"lifelog/backend/internal/speech"
	"lifelog/backend/internal/speech/ffmpeg"
	"lifelog/backend/internal/speech/llmengine"
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
	// fatal: without the index, upload and timeline work, only questions do
	// not. Then create or edit the index in the Atlas UI.
	searchReady := watchVectorIndex(ctx, db)

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
	transcriber := newTranscriber(ctx, c, g)

	// Login and tokens (docs/API.md, section 1). Failed attempts are
	// counted in memory; a restart resets the counters. Behind a reverse
	// proxy (Caddy) TRUSTED_PROXIES must name it, otherwise every client
	// counts as 127.0.0.1 and one IP lock would hit everybody.
	accounts := service.NewAccounts(db, auth.NewLimiter(time.Now), store.NewID)
	slog.Info("client IP for failed-attempt limits", "x_forwarded_for", len(c.TrustedProxies) > 0)

	// Daily limits per user, counted in MongoDB (docs/API.md, daily limits).
	usage := service.NewUsage(db, service.Limits{
		Uploads:        c.LimitUploadsPerDay,
		Questions:      c.LimitQuestionsPerDay,
		Transcriptions: c.LimitTranscriptionsPerDay,
	})
	slog.Info("daily limits per user", "uploads", c.LimitUploadsPerDay, "questions", c.LimitQuestionsPerDay,
		"transcriptions", c.LimitTranscriptionsPerDay)

	server := &http.Server{
		Addr: c.ListenAddress(),
		Handler: api.NewRouter(api.Dependencies{
			DB:        db,
			Entries:   db,
			Uploader:  service.NewUploader(db, g, g, recognizer, usage, store.NewID),
			Asker:     service.NewAsker(db, g, g, usage, searchReady.Load),
			Corrector: service.NewCorrector(db),
			Deleter:   service.NewDeleter(db),

			Transcriber: transcriber,
			Routines:    service.NewRoutines(db, store.NewID, time.Now),

			Accounts:       accounts,
			Usage:          usage,
			TrustedProxies: c.TrustedProxies,
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

// vectorIndexPoll: how often the server asks Atlas whether a new or updated
// vector index is READY.
const vectorIndexPoll = 15 * time.Second

// watchVectorIndex makes sure the vector index exists with the current
// definition (store.CreateVectorIndex) and returns a flag that tells
// whether questions can be answered (POST /questions, docs/API.md 3.6).
//
//   - READY at start: the flag is set at once.
//   - Created, updated or still building: questions answer 503 until Atlas
//     reports READY. A goroutine asks every 15 s, logs once when it is, and
//     sets the flag. Upload and timeline work meanwhile.
//   - Atlas cannot be asked (e.g. no permission): the flag is set anyway,
//     so questions are never blocked for good; if the index really has the
//     old definition, the search itself fails and answers 503 too
//     (store.SimilarChunks).
func watchVectorIndex(ctx context.Context, db *store.Store) *atomic.Bool {
	ready := &atomic.Bool{}
	status, err := db.CreateVectorIndex(ctx)
	switch {
	case err != nil:
		slog.Warn("vector index could not be checked or changed; questions only work once it has the definition of internal/store/chunks_vector.json (backend/README.md, section vector index)",
			"index", store.VectorIndexName, "error", err)
		ready.Store(true)
		return ready
	case status == store.VectorIndexReady:
		slog.Info("vector index", "index", store.VectorIndexName, "status", status)
		ready.Store(true)
		return ready
	case status == store.VectorIndexUpdated:
		slog.Warn("vector index updated (filter userId added); questions work again once the vector index is READY (until then POST /questions answers 503)",
			"index", store.VectorIndexName)
	default:
		slog.Warn("vector index not ready; questions work again once the vector index is READY (until then POST /questions answers 503)",
			"index", store.VectorIndexName, "status", status)
	}

	go func() {
		t := time.NewTicker(vectorIndexPoll)
		defer t.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-t.C:
			}
			status, found, err := db.VectorIndexStatus(ctx)
			if err != nil {
				slog.Warn("vector index status unknown", "error", err)
				continue
			}
			if found && status == store.VectorIndexReady {
				ready.Store(true)
				slog.Info("vector index READY, questions work again", "index", store.VectorIndexName)
				return
			}
		}
	}()
	return ready
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
// SPEECH_ENGINE=none. With SPEECH_ENGINE=gemini the read model writes the
// text instead of whisper (servers without a GPU, docs/speech/README.md).
func newTranscriber(ctx context.Context, c config.Config, model llm.AudioTranscriber) api.Transcriber {
	if c.SpeechEngine == config.SpeechEngineNone {
		slog.Info("speech recognition switched off (SPEECH_ENGINE=none)")
		return nil
	}
	var engine speech.Engine
	if c.SpeechEngine == config.SpeechEngineGemini {
		engine = llmengine.New(model, "Gemini "+c.GeminiModel)
	} else {
		engine = whisper.New(c.WhisperURL)
	}
	p := &speech.Pipeline{Engine: engine}
	if conv, err := ffmpeg.New(); err != nil {
		slog.Warn("ffmpeg missing, transcription unavailable (backend/README.md, section speech)", "error", err)
	} else {
		p.Converter = conv
		slog.Info("speech recognition", "converter", conv.Name(), "engine", engine.Name())
	}
	if c.SpeechEngine == config.SpeechEngineWhisper {
		if err := engine.Ping(ctx); err != nil {
			slog.Warn("whisper-server not reachable yet; start it with backend/scripts/whisper-server.sh", "url", c.WhisperURL)
		}
	}
	return p
}
