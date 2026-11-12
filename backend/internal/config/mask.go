package config

import "strings"

// Mask replaces every occurrence of the secrets in text with "***". For
// error messages of third-party libraries whose content we do not control,
// before they go to the log or the screen. Empty secrets are ignored
// (otherwise every position would be replaced).
func Mask(text string, secrets ...string) string {
	for _, s := range secrets {
		if s != "" {
			text = strings.ReplaceAll(text, s, "***")
		}
	}
	return text
}
