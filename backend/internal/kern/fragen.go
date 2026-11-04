package kern

import (
	"errors"
	"regexp"
	"strconv"
	"strings"
	"unicode/utf8"
)

// FesterSatz ist die Antwort, wenn die Dokumente nichts zur Frage enthalten
// (CLAUDE.md, API.md 3.6). Wörtlich, die App darf darauf vergleichen.
const FesterSatz = "Dazu finde ich nichts in deinen Dokumenten."

// MaxFrageZeichen: längste erlaubte Frage (API.md 3.6), in Zeichen.
const MaxFrageZeichen = 500

// AnzahlTreffer: so viele Abschnitte holt die Vektorsuche höchstens.
const AnzahlTreffer = 5

// MinScore ist die Schwelle für Treffer der Vektorsuche.
//
// Atlas meldet bei similarity "cosine" nicht die Kosinus-Ähnlichkeit selbst,
// sondern score = (1 + cos) / 2, also einen Wert zwischen 0 und 1.
// 0,80 entspricht cos = 0,60 (siehe ScoreAusKosinus/KosinusAusScore).
//
// Die Schwelle ist bewusst nur ein grober Filter. Die zweite Schicht ist das
// Sprachmodell: Beantworten die Abschnitte die Frage nicht, antwortet es mit
// dem FesterSatz. Messwerte vom 04.11.2026 (gemini-embedding-2, 768 Dim.):
// passender Abschnitt 0,88–0,90, andere Gesundheitsabschnitte 0,81–0,84,
// fachfremde Frage („Wetter“) ~0,77. Wird mit mehr Fragen kalibriert.
const MinScore = 0.80

var (
	ErrFrageFehlt  = errors.New("Frage fehlt")
	ErrFrageZuLang = errors.New("Frage zu lang")
)

// PruefeFrage entfernt Leerraum am Rand und prüft die Länge.
func PruefeFrage(frage string) (string, error) {
	frage = strings.TrimSpace(frage)
	if frage == "" {
		return "", ErrFrageFehlt
	}
	if utf8.RuneCountInString(frage) > MaxFrageZeichen {
		return "", ErrFrageZuLang
	}
	return frage, nil
}

// ScoreAusKosinus rechnet eine Kosinus-Ähnlichkeit (−1 … 1) in den Score
// um, den Atlas Vector Search meldet (0 … 1).
func ScoreAusKosinus(cos float64) float64 { return (1 + cos) / 2 }

// KosinusAusScore ist die Umkehrung von ScoreAusKosinus.
func KosinusAusScore(score float64) float64 { return 2*score - 1 }

// Treffer ist ein Abschnitt aus der Vektorsuche.
type Treffer struct {
	EintragID string
	Nr        int // Nummer des Abschnitts im Volltext
	Text      string
	Score     float64 // Atlas-Score, 0 … 1
}

// UeberSchwelle behält die Treffer mit Score ≥ min, in derselben Reihenfolge.
func UeberSchwelle(treffer []Treffer, min float64) []Treffer {
	out := make([]Treffer, 0, len(treffer))
	for _, t := range treffer {
		if t.Score >= min {
			out = append(out, t)
		}
	}
	return out
}

// Fundstelle ist ein Abschnitt, wie er dem Sprachmodell vorgelegt und in
// der Antwort als Quelle genannt wird. Nr ist die Nummer, auf die sich
// Verweise wie [1] in der Antwort beziehen.
type Fundstelle struct {
	Nr            int
	EintragID     string
	Modul         string
	Typ           string
	Quelle        *string
	Dokumentdatum *string
	Auszug        string
}

// Antwort auf eine Frage (API.md 3.6).
type Antwort struct {
	Text     string
	Gefunden bool
	Quellen  []Fundstelle
}

// KeinTreffer ist die Antwort, wenn nichts Passendes gefunden wurde.
func KeinTreffer() Antwort {
	return Antwort{Text: FesterSatz, Gefunden: false, Quellen: []Fundstelle{}}
}

// zitat findet Verweise wie [1], [12] und auch [1, 3] oder [1,3].
var zitat = regexp.MustCompile(`\[(\d+(?:\s*,\s*\d+)*)\]`)

// WerteZitateAus liefert die Nummern, auf die die Antwort verweist, in der
// Reihenfolge des ersten Auftretens und ohne Doppelte. Nur Nummern von 1 bis
// anzahl zählen; [7] bei 5 Fundstellen ist ein Fehler des Modells und wird
// ignoriert. Das Ergebnis ist nie nil.
func WerteZitateAus(text string, anzahl int) []int {
	nummern := []int{}
	gesehen := map[int]bool{}
	for _, m := range zitat.FindAllStringSubmatch(text, -1) {
		for _, teil := range strings.Split(m[1], ",") {
			n, err := strconv.Atoi(strings.TrimSpace(teil))
			if err != nil || n < 1 || n > anzahl || gesehen[n] {
				continue
			}
			gesehen[n] = true
			nummern = append(nummern, n)
		}
	}
	return nummern
}

// BaueAntwort macht aus dem Text des Sprachmodells die Antwort für die App.
//
// Regel (CLAUDE.md: „immer mit Quellenangabe“): Verweist der Text auf keine
// der Fundstellen, gilt er als „nichts gefunden“, und die App bekommt den
// FesterSatz. Das fängt auch den Fall ab, dass das Modell selbst den festen
// Satz schreibt. Sonst enthält quellen genau die zitierten Fundstellen, in
// der Reihenfolge ihrer Nummern.
func BaueAntwort(text string, fundstellen []Fundstelle) Antwort {
	zitiert := WerteZitateAus(text, len(fundstellen))
	if len(zitiert) == 0 {
		return KeinTreffer()
	}
	istZitiert := map[int]bool{}
	for _, n := range zitiert {
		istZitiert[n] = true
	}
	quellen := []Fundstelle{}
	for _, f := range fundstellen {
		if istZitiert[f.Nr] {
			quellen = append(quellen, f)
		}
	}
	return Antwort{Text: strings.TrimSpace(text), Gefunden: true, Quellen: quellen}
}
