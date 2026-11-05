// Der Lifelog-Server. Start aus dem Ordner backend/:
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
	"lifelog/backend/internal/dienst"
	"lifelog/backend/internal/konfig"
	"lifelog/backend/internal/modell/gemini"
	"lifelog/backend/internal/speicher"
)

func main() {
	if err := starten(); err != nil {
		slog.Error("Server beendet mit Fehler", "fehler", err)
		os.Exit(1)
	}
}

func starten() error {
	if err := konfig.DotenvLaden(); err != nil {
		return err
	}
	k, err := konfig.Laden(os.Getenv)
	if err != nil {
		return err
	}

	// Strg+C (SIGINT) oder ein Stopp-Signal (SIGTERM) beenden den Server sauber.
	ctx, stopp := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stopp()

	db, err := speicher.Verbinden(ctx, k.MongoURI, k.MongoDB)
	if err != nil {
		return err
	}
	defer db.Trennen(context.Background())
	slog.Info("MongoDB verbunden", "datenbank", k.MongoDB)

	if err := db.IndexeAnlegen(ctx); err != nil {
		return err
	}
	// Nach IndexeAnlegen, weil die Collection "abschnitte" existieren muss.
	// Nicht fatal: Ohne Index funktionieren Upload und Zeitleiste, nur die
	// Suche nicht. Dann den Index in der Atlas-Oberfläche anlegen.
	if status, err := db.VektorindexAnlegen(ctx); err != nil {
		slog.Warn("Vektorindex fehlt, POST /fragen funktioniert erst nach dem Anlegen in Atlas (backend/README.md, Abschnitt Vektorindex)",
			"index", speicher.VektorindexName, "fehler", err)
	} else {
		slog.Info("Vektorindex", "index", speicher.VektorindexName, "status", status)
	}

	// Hier wird der Modell-Anbieter gewählt. Ein anderer Anbieter = ein
	// anderes Paket unter internal/modell, das Leser, Einbetter und
	// Antworter umsetzt.
	g, err := gemini.Neu(ctx, gemini.Einstellungen{
		Schluessel:      k.GeminiKey,
		Modell:          k.GeminiModell,
		ErsatzModell:    k.GeminiModellErsatz,
		ErsatzReserve:   k.GeminiErsatzReserve,
		EmbeddingModell: k.GeminiEmbeddingModell,
	})
	if err != nil {
		return err
	}
	slog.Info("Modelle", "lesen", k.GeminiModell, "ersatz", k.GeminiModellErsatz,
		"reserve", k.GeminiErsatzReserve, "embedding", k.GeminiEmbeddingModell)

	server := &http.Server{
		Addr: k.Lauschadresse(),
		Handler: api.NeuerRouter(api.Abhaengigkeiten{
			DB:          db,
			Eintraege:   db,
			Hochladen:   dienst.NeuHochladen(db, g, g, speicher.NeueID),
			Fragen:      dienst.NeuFragen(db, g, g),
			Korrigieren: dienst.NeuKorrigieren(db),
			Loeschen:    dienst.NeuLoeschen(db),
		}),
		ReadHeaderTimeout: 10 * time.Second,
		ReadTimeout:       60 * time.Second,  // Upload bis 10 MB, auch über langsames Mobilfunknetz
		WriteTimeout:      120 * time.Second, // Upload mit Vision-LLM dauert bis ~60 s
		IdleTimeout:       120 * time.Second,
	}

	if !k.NurLokal() {
		slog.Warn("Backend ist im ganzen Netz erreichbar (ADRESSE ist nicht lokal). Nur zum Testen verwenden.",
			"adresse", k.Adresse)
	}

	serverFehler := make(chan error, 1)
	go func() {
		slog.Info("Server läuft", "adresse", "http://"+k.Lauschadresse())
		serverFehler <- server.ListenAndServe()
	}()

	select {
	case err := <-serverFehler:
		// z. B. Port schon belegt
		return err
	case <-ctx.Done():
		slog.Info("Server wird beendet …")
	}

	// Laufende Anfragen dürfen noch bis zu 10 s fertig werden.
	shutdownCtx, abbrechen := context.WithTimeout(context.Background(), 10*time.Second)
	defer abbrechen()
	return server.Shutdown(shutdownCtx)
}
