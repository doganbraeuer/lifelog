// models lists the Gemini models the own API key can see, with their
// capabilities. Helps with choosing GEMINI_MODEL and GEMINI_EMBEDDING_MODEL.
//
//	go run ./cmd/models
//
// The key comes from .env (as for the server), so it is never in the
// command, and it is never printed: error messages go through config.Mask.
//
// Caution: the list says which models exist, not whether they can be used
// in the free tier or with which limits. That is in AI Studio or on
// ai.google.dev/gemini-api/docs/rate-limits.
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
	if err := config.LoadDotenv(); err != nil {
		fmt.Fprintln(os.Stderr, "Error:", err)
		os.Exit(1)
	}
	apiKey := strings.TrimSpace(os.Getenv("GEMINI_API_KEY"))
	if apiKey == "" {
		fmt.Fprintln(os.Stderr, "Error: GEMINI_API_KEY missing (see .env.example)")
		os.Exit(1)
	}

	if err := list(apiKey); err != nil {
		fmt.Fprintln(os.Stderr, "Error:", config.Mask(err.Error(), apiKey))
		os.Exit(1)
	}
}

func list(apiKey string) error {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	client, err := genai.NewClient(ctx, &genai.ClientConfig{
		APIKey:  apiKey,
		Backend: genai.BackendGeminiAPI,
	})
	if err != nil {
		return err
	}

	var lines []string
	for m, err := range client.Models.All(ctx) {
		if err != nil {
			return err
		}
		text := slices.Contains(m.SupportedActions, "generateContent")
		embed := slices.Contains(m.SupportedActions, "embedContent")
		if !text && !embed {
			continue
		}
		kind := "Text/Vision"
		if embed {
			kind = "Embedding"
		}
		lines = append(lines, fmt.Sprintf("%-11s  %-45s  in %8d / out %6d tokens  %s",
			kind, strings.TrimPrefix(m.Name, "models/"), m.InputTokenLimit, m.OutputTokenLimit,
			strings.Join(m.SupportedActions, ",")))
	}

	slices.Sort(lines)
	for _, l := range lines {
		fmt.Println(l)
	}
	fmt.Printf("\n%d models with generateContent or embedContent.\n", len(lines))
	return nil
}
