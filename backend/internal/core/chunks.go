package core

import (
	"regexp"
	"strings"
	"unicode/utf8"
)

// AbschnittMaxZeichen ist die Obergrenze für einen Abschnitt, gezählt in
// Zeichen (Runes), nicht in Bytes. "ä" ist ein Zeichen, aber zwei Bytes.
//
// Warum 500? Kleinere Abschnitte machen die Quellenangabe genauer (der
// Auszug zeigt die passende Stelle statt des ganzen Dokuments). Größere
// geben dem Sprachmodell mehr Zusammenhang. 500 ist ein Startwert für den
// Prototyp, den wir mit echten (synthetischen) Dokumenten nachjustieren.
const AbschnittMaxZeichen = 500

// leerzeilen trennt Absätze: ein Zeilenumbruch, dann mindestens eine Zeile,
// die leer ist oder nur Leerraum enthält.
var leerzeilen = regexp.MustCompile(`\n[ \t]*\n\s*`)

// einheit ist ein Stück Text plus der Trenner, der im Original davor stand.
type einheit struct {
	trenner string // "\n\n" zwischen Absätzen, "\n" zwischen Zeilen, " " zwischen Wörtern, "" bei hartem Schnitt
	text    string
}

// ZerlegeInAbschnitte teilt einen Volltext in Abschnitte für die Suche.
//
//  1. Absätze (getrennt durch Leerzeilen) sind die natürlichen Einheiten.
//  2. Ein zu langer Absatz wird an Zeilen geteilt, eine zu lange Zeile an
//     Wörtern, ein zu langes Wort hart nach max Zeichen.
//  3. Die Einheiten werden der Reihe nach zusammengefasst, solange der
//     Abschnitt höchstens max Zeichen hat.
//
// Kein Abschnitt ist leer oder länger als max. Leerer Text ergibt eine leere
// Liste (nicht nil). Gleicher Text ergibt immer dieselben Abschnitte.
func ZerlegeInAbschnitte(volltext string, max int) []string {
	if max < 1 {
		max = AbschnittMaxZeichen
	}
	text := strings.ReplaceAll(volltext, "\r\n", "\n")
	text = strings.ReplaceAll(text, "\r", "\n")

	var einheiten []einheit
	for _, absatz := range leerzeilen.Split(text, -1) {
		absatz = strings.TrimSpace(absatz)
		if absatz == "" {
			continue
		}
		einheiten = append(einheiten, teile(absatz, "\n\n", max)...)
	}
	return fasseZusammen(einheiten, max)
}

// teile zerlegt text so weit, dass jedes Stück höchstens max Zeichen hat.
// trenner ist der Trenner vor dem ersten Stück.
func teile(text, trenner string, max int) []einheit {
	if laenge(text) <= max {
		return []einheit{{trenner, text}}
	}

	var stuecke []string
	var innen string
	switch {
	case strings.Contains(text, "\n"):
		stuecke, innen = nichtLeer(strings.Split(text, "\n")), "\n"
	case len(strings.Fields(text)) > 1:
		stuecke, innen = strings.Fields(text), " "
	default:
		return hartSchneiden(text, trenner, max)
	}

	var out []einheit
	for i, s := range stuecke {
		t := innen
		if i == 0 {
			t = trenner
		}
		out = append(out, teile(s, t, max)...)
	}
	return out
}

// hartSchneiden teilt ein einzelnes überlanges Wort (z. B. eine lange
// Zeichenkette ohne Leerzeichen) nach max Zeichen. Geschnitten wird an
// Zeichengrenzen, nie mitten in einem Umlaut.
func hartSchneiden(text, trenner string, max int) []einheit {
	var out []einheit
	runes := []rune(text)
	for start := 0; start < len(runes); start += max {
		ende := min(start+max, len(runes))
		t := ""
		if start == 0 {
			t = trenner
		}
		out = append(out, einheit{t, string(runes[start:ende])})
	}
	return out
}

func fasseZusammen(einheiten []einheit, max int) []string {
	abschnitte := []string{}
	var aktuell strings.Builder
	aktuellLaenge := 0

	for _, e := range einheiten {
		textLaenge := laenge(e.text)
		if aktuellLaenge > 0 && aktuellLaenge+laenge(e.trenner)+textLaenge <= max {
			aktuell.WriteString(e.trenner)
			aktuell.WriteString(e.text)
			aktuellLaenge += laenge(e.trenner) + textLaenge
			continue
		}
		if aktuellLaenge > 0 {
			abschnitte = append(abschnitte, aktuell.String())
			aktuell.Reset()
		}
		aktuell.WriteString(e.text) // Trenner am Anfang eines Abschnitts entfällt
		aktuellLaenge = textLaenge
	}
	if aktuellLaenge > 0 {
		abschnitte = append(abschnitte, aktuell.String())
	}
	return abschnitte
}

func nichtLeer(zeilen []string) []string {
	out := zeilen[:0:0]
	for _, z := range zeilen {
		if z = strings.TrimSpace(z); z != "" {
			out = append(out, z)
		}
	}
	return out
}

func laenge(s string) int { return utf8.RuneCountInString(s) }
