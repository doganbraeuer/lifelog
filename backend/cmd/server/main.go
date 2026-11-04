// Der Lifelog-Server. Start aus dem Ordner backend/:
//
//	go run ./cmd/server
package main

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"go.mongodb.org/mongo-driver/v2/mongo"
	"go.mongodb.org/mongo-driver/v2/mongo/options"
	"go.mongodb.org/mongo-driver/v2/mongo/readpref"

	"lifelog/backend/internal/api"
	"lifelog/backend/internal/konfig"
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

	// MongoDB verbinden. Connect baut nur den Client auf, erst Ping prüft die
	// Verbindung wirklich. Die URI wird nie geloggt, weil sie das Passwort enthält.
	client, err := mongo.Connect(options.Client().ApplyURI(k.MongoURI))
	if err != nil {
		return errors.New("MongoDB-Client konnte nicht erstellt werden (MONGODB_URI prüfen)")
	}
	defer func() {
		trennCtx, abbrechen := context.WithTimeout(context.Background(), 5*time.Second)
		defer abbrechen()
		_ = client.Disconnect(trennCtx)
	}()

	db := mongoPinger{client}
	pingCtx, abbrechen := context.WithTimeout(ctx, 10*time.Second)
	err = db.Ping(pingCtx)
	abbrechen()
	if err != nil {
		return errors.New("MongoDB nicht erreichbar: Zugangsdaten und Network Access in Atlas prüfen")
	}
	slog.Info("MongoDB verbunden", "datenbank", k.MongoDB)

	server := &http.Server{
		Addr:              k.Lauschadresse(),
		Handler:           api.NeuerRouter(api.Abhaengigkeiten{DB: db}),
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
	if err := server.Shutdown(shutdownCtx); err != nil {
		return err
	}
	return nil
}

// mongoPinger passt den MongoDB-Client an die Schnittstelle api.Pinger an.
type mongoPinger struct{ client *mongo.Client }

func (m mongoPinger) Ping(ctx context.Context) error {
	return m.client.Ping(ctx, readpref.Primary())
}
