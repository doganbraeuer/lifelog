package gemini

import (
	"fmt"

	"google.golang.org/genai"

	"lifelog/backend/internal/modell"
)

// gemini-embedding-2 kennt kein task_type. Stattdessen sagt ein Präfix im
// Text, ob es sich um ein Dokument oder um eine Suchanfrage handelt
// (Formate aus aistudio.google.com/docs/embeddings, geprüft 04.11.2026).
// Unsere Abschnitte haben keinen eigenen Titel, laut Doku dann "none".
func dokumentEingabe(text string) string { return "title: none | text: " + text }
func frageEingabe(frage string) string   { return "task: search result | query: " + frage }

// stapelGroesse: so viele Texte gehen höchstens in einen Aufruf. Ein langer
// Volltext (100 000 Zeichen / 500) hat ~200 Abschnitte, das sind dann
// mehrere Aufrufe statt eines riesigen.
const stapelGroesse = 100

// stapel teilt n Elemente in Bereiche [von, bis) mit höchstens groesse
// Elementen.
func stapel(n, groesse int) [][2]int {
	bereiche := [][2]int{}
	for von := 0; von < n; von += groesse {
		bereiche = append(bereiche, [2]int{von, min(von+groesse, n)})
	}
	return bereiche
}

// embeddingInhalte macht aus jedem Text ein eigenes Content-Objekt.
// Wichtig: gemini-embedding-2 fasst mehrere Teile EINES Contents zu einem
// einzigen Vektor zusammen. Nur getrennte Contents ergeben einen Vektor pro
// Text.
func embeddingInhalte(eingaben []string) []*genai.Content {
	inhalte := make([]*genai.Content, len(eingaben))
	for i, e := range eingaben {
		inhalte[i] = genai.NewContentFromText(e, genai.RoleUser)
	}
	return inhalte
}

// vektorenAusAntwort prüft, dass genau anzahl Vektoren mit der richtigen
// Länge zurückkamen. Ein falsch langer Vektor würde später beim Speichern
// oder im Vektorindex scheitern, das fangen wir hier früh ab.
func vektorenAusAntwort(resp *genai.EmbedContentResponse, anzahl int) ([][]float32, error) {
	if resp == nil || len(resp.Embeddings) != anzahl {
		erhalten := 0
		if resp != nil {
			erhalten = len(resp.Embeddings)
		}
		return nil, fmt.Errorf("%d Embeddings erwartet, %d erhalten: %w", anzahl, erhalten, modell.ErrAntwortUngueltig)
	}
	vektoren := make([][]float32, anzahl)
	for i, e := range resp.Embeddings {
		if e == nil || len(e.Values) != modell.EmbeddingDimensionen {
			laenge := 0
			if e != nil {
				laenge = len(e.Values)
			}
			return nil, fmt.Errorf("Embedding %d hat %d statt %d Dimensionen: %w",
				i, laenge, modell.EmbeddingDimensionen, modell.ErrAntwortUngueltig)
		}
		vektoren[i] = e.Values
	}
	return vektoren, nil
}
