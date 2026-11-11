package gemini

import (
	"context"
	"errors"
	"fmt"
	"net"

	"google.golang.org/genai"

	"lifelog/backend/internal/config"
	"lifelog/backend/internal/llm"
)

// translateError turns an error of the Gemini SDK into an error the rest of
// the backend understands (llm.ErrUnavailable etc.).
//
// The API key is removed from every message. The SDK sends it in a header
// and not in the URL, but we do not control the error messages of
// third-party libraries.
func translateError(err error, apiKey string) error {
	if err == nil {
		return nil
	}
	text := config.Mask(err.Error(), apiKey)

	var api genai.APIError
	if asError(err, &api) {
		switch {
		case api.Code == 408, api.Code == 429, api.Code >= 500:
			// Timeout, limit reached or outage at Google: trying again later
			// can help.
			return fmt.Errorf("Gemini HTTP %d: %s: %w", api.Code, text, llm.ErrUnavailable)
		default:
			// 400, 401, 403, 404: our request or configuration is wrong (e.g.
			// invalid key, unknown model). This is an internal error, not a
			// problem of the document.
			return fmt.Errorf("Gemini HTTP %d: %s", api.Code, text)
		}
	}

	if errors.Is(err, context.Canceled) {
		// The app dropped the connection. Not a model problem.
		return fmt.Errorf("Gemini call canceled: %w", context.Canceled)
	}
	var netErr net.Error
	if errors.Is(err, context.DeadlineExceeded) || asError(err, &netErr) {
		return fmt.Errorf("Gemini: %s: %w", text, llm.ErrUnavailable)
	}
	return fmt.Errorf("Gemini: %s", text)
}

// asError is errors.As, but as a function with its own name, so the calls
// above stay readable.
func asError[T any](err error, target *T) bool {
	return errors.As(err, target)
}
