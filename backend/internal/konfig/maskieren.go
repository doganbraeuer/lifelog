package konfig

import "strings"

// Maskiere ersetzt jedes Vorkommen der Geheimnisse in text durch "***".
// Für Fehlermeldungen fremder Bibliotheken, deren Inhalt wir nicht
// kontrollieren, bevor sie ins Log oder auf den Bildschirm gehen. Leere
// Geheimnisse werden ignoriert (sonst würde jede Stelle ersetzt).
func Maskiere(text string, geheimnisse ...string) string {
	for _, g := range geheimnisse {
		if g != "" {
			text = strings.ReplaceAll(text, g, "***")
		}
	}
	return text
}
