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
	"lifelog/backend/internal/service"
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

	server := &http.Server{
		Addr: c.ListenAddress(),
		Handler: api.NewRouter(api.Dependencies{
			DB:        db,
			Entries:   db,
			Uploader:  service.NewUploader(db, g, g, store.NewID),
			Asker:     service.NewAsker(db, g, g),
			Corrector: service.NewCorrector(db),
			Deleter:   service.NewDeleter(db),
		}),
		ReadHeaderTimeout: 10 * time.Second,
		ReadTimeout:       60 * time.Second,  // upload up to 10 MB, also over a slow mobile network
		WriteTimeout:      120 * time.Second, // upload with the vision LLM takes up to ~60 s
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
