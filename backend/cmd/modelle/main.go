// modelle listet die Gemini-Modelle, die der eigene API-Schlüssel sehen kann,
// mit ihren Fähigkeiten. Hilft beim Auswählen von GEMINI_MODELL und
// GEMINI_EMBEDDING_MODELL.
//
//	go run ./cmd/modelle
//
// Der Schlüssel kommt aus .env (wie beim Server), steht also nie im Befehl,
// und wird nie ausgegeben: Fehlermeldungen laufen durch config.Maskiere.
//
// Achtung: Die Liste sagt, welche Modelle es gibt, nicht, ob sie in der
// kostenlosen Stufe nutzbar sind oder mit welchen Limits. Das steht in AI
// Studio bzw. auf ai.google.dev/gemini-api/docs/rate-limits.
package main

import (
	"context"
	"fmt"
	"os"
	"slices"
	"strings"
	"time"

	"google.golang.org/genai"

	"lifelog/backend/internal/config"
)

func main() {
	if err := config.DotenvLaden(); err != nil {
		fmt.Fprintln(os.Stderr, "Fehler:", err)
		os.Exit(1)
	}
	schluessel := strings.TrimSpace(os.Getenv("GEMINI_API_KEY"))
	if schluessel == "" {
		fmt.Fprintln(os.Stderr, "Fehler: GEMINI_API_KEY fehlt (siehe .env.example)")
		os.Exit(1)
	}

	if err := auflisten(schluessel); err != nil {
		fmt.Fprintln(os.Stderr, "Fehler:", config.Maskiere(err.Error(), schluessel))
		os.Exit(1)
	}
}

func auflisten(schluessel string) error {
	ctx, abbrechen := context.WithTimeout(context.Background(), 30*time.Second)
	defer abbrechen()

	client, err := genai.NewClient(ctx, &genai.ClientConfig{
		APIKey:  schluessel,
		Backend: genai.BackendGeminiAPI,
	})
	if err != nil {
		return err
	}

	var zeilen []string
	for m, err := range client.Models.All(ctx) {
		if err != nil {
			return err
		}
		text := slices.Contains(m.SupportedActions, "generateContent")
		embed := slices.Contains(m.SupportedActions, "embedContent")
		if !text && !embed {
			continue
		}
		art := "Text/Vision"
		if embed {
			art = "Embedding"
		}
		zeilen = append(zeilen, fmt.Sprintf("%-11s  %-45s  ein %8d / aus %6d Tokens  %s",
			art, strings.TrimPrefix(m.Name, "models/"), m.InputTokenLimit, m.OutputTokenLimit,
			strings.Join(m.SupportedActions, ",")))
	}

	slices.Sort(zeilen)
	for _, z := range zeilen {
		fmt.Println(z)
	}
	fmt.Printf("\n%d Modelle mit generateContent oder embedContent.\n", len(zeilen))
	return nil
}
