// Package llm ist die einzige Stelle, über die das Backend Sprach-,
// Vision- und Embedding-Modelle aufruft (CLAUDE.md: „Anbieter austauschbar“).
//
// Der Rest des Backends kennt nur diese Schnittstellen. Ein anderer Anbieter
// bedeutet: ein neues Unterpaket neben gemini/, das sie umsetzt, und eine
// Zeile in cmd/server/main.go.
package llm

import (
	"context"
	"errors"

	"lifelog/backend/internal/core"
)

// EmbeddingDimensionen ist die Länge jedes Embedding-Vektors. Der
// Vektorindex in Atlas (Schritt 5) ist auf genau diese Zahl festgelegt.
// Ändert sie sich, müssen alle Abschnitte neu eingebettet werden.
const EmbeddingDimensionen = 768

// Leser liest ein Dokument (Bild oder PDF) und liefert, was darin steht.
// Das Ergebnis ist ungeprüft, prüfen muss core.PruefeErkennung.
type Leser interface {
	DokumentLesen(ctx context.Context, daten []byte, mimeTyp string, erlaubteTypen []string) (core.Extraction, error)

	// TextLesen ordnet einen eingegebenen Text ein (Text-Import): Datum,
	// Quelle, Typ und Infos. Lesbar ist immer true, Volltext bleibt leer;
	// der Volltext ist der eingegebene Text, den setzt der Aufrufer.
	TextLesen(ctx context.Context, text string, erlaubteTypen []string) (core.Extraction, error)
}

// Einbetter berechnet Embeddings: Vektoren, deren Abstand zueinander
// ausdrückt, wie ähnlich sich zwei Texte inhaltlich sind.
//
// Abschnitte und Fragen werden getrennt eingebettet, weil manche Modelle
// beide unterschiedlich behandeln („Dokument“ vs. „Suchanfrage“). Beide
// Methoden liefern Vektoren mit EmbeddingDimensionen Einträgen.
type Einbetter interface {
	// AbschnitteEinbetten liefert genau einen Vektor pro Text, in derselben
	// Reihenfolge.
	AbschnitteEinbetten(ctx context.Context, texte []string) ([][]float32, error)
	FrageEinbetten(ctx context.Context, frage string) ([]float32, error)
}

// Antworter beantwortet eine Frage nur aus den übergebenen Fundstellen. Er
// setzt Verweise wie [1], die sich auf Fundstelle.Nr beziehen, und antwortet
// mit core.FesterSatz, wenn die Fundstellen die Frage nicht beantworten.
// Ob die Antwort wirklich Verweise enthält, prüft danach core.BaueAntwort.
type Antworter interface {
	Antworten(ctx context.Context, frage string, fundstellen []core.Source) (string, error)
}

// Fehlerarten. Jede Umsetzung übersetzt ihre eigenen Fehler in diese, damit
// die HTTP-Schicht den richtigen Code wählen kann, egal welcher Anbieter.
var (
	// ErrNichtErreichbar: Netzwerk, Zeitüberschreitung, Limit (429) oder
	// Serverfehler des Anbieters (→ 503 MODELL_NICHT_ERREICHBAR).
	ErrNichtErreichbar = errors.New("Modell nicht erreichbar")

	// ErrAntwortUngueltig: Das Modell hat geantwortet, aber nicht in der
	// erwarteten Form (→ 502 AUSLESEN_FEHLGESCHLAGEN).
	ErrAntwortUngueltig = errors.New("Antwort des Modells ungültig")
)
