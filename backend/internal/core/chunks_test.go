package core

import (
	"slices"
	"strings"
	"testing"
	"unicode/utf8"
)

// Synthetischer Laborbefund wie in docs/API.md.
const laborbefund = "Labor Muster GmbH\nMusterstraße 1, 5020 Salzburg\n\n" +
	"Laborbefund vom 14.10.2026\nPatient: Max Mustermann, geb. 01.01.1990\n\n" +
	"Hämoglobin 14,2 g/dl (Ref. 13,5-17,5)\nLeukozyten 6,8 G/l (Ref. 4,0-10,0)\n" +
	"CRP <0,5 mg/dl (Ref. <0,5)\nBefundart: Blutbild klein"

func TestZerlege_KurzerTextIstEinAbschnitt(t *testing.T) {
	got := ZerlegeInAbschnitte(laborbefund, AbschnittMaxZeichen)
	if len(got) != 1 || got[0] != laborbefund {
		t.Errorf("erwartet den ganzen Text als einen Abschnitt, bekommen %q", got)
	}
}

func TestZerlege_AbsaetzeUndZeilen(t *testing.T) {
	// max 60. Längen in Zeichen:
	//   Absatz 1: 17 + 1 + 29 = 47      → passt, bleibt ganz
	//   Absatz 2: 26 + 1 + 40 = 67      → zu lang, wird an der Zeile geteilt
	//   Absatz 3: 37 | 34 | 26 | 25     → Zeilen werden zusammengefasst,
	//             solange es passt: 37+1+34 = 72 nein, 34+1+26 = 61 nein,
	//             26+1+25 = 52 ja
	// Absätze 1 und 2 zusammen (47 + 2 + 26) wären auch zu lang.
	got := ZerlegeInAbschnitte(laborbefund, 60)
	erwartet := []string{
		"Labor Muster GmbH\nMusterstraße 1, 5020 Salzburg",
		"Laborbefund vom 14.10.2026",
		"Patient: Max Mustermann, geb. 01.01.1990",
		"Hämoglobin 14,2 g/dl (Ref. 13,5-17,5)",
		"Leukozyten 6,8 G/l (Ref. 4,0-10,0)",
		"CRP <0,5 mg/dl (Ref. <0,5)\nBefundart: Blutbild klein",
	}
	if !slices.Equal(got, erwartet) {
		t.Errorf("Abschnitte:\n%q\nerwartet:\n%q", got, erwartet)
	}
}

func TestZerlege_KleineAbsaetzeWerdenZusammengefasst(t *testing.T) {
	got := ZerlegeInAbschnitte("A\n\nB\n\nC", 4)
	// "A\n\nB" hat genau 4 Zeichen, C passt nicht mehr dazu.
	if !slices.Equal(got, []string{"A\n\nB", "C"}) {
		t.Errorf("= %q", got)
	}
}

func TestZerlege_ZuLangeZeileWirdAnWoertenGeteilt(t *testing.T) {
	got := ZerlegeInAbschnitte("eins zwei drei vier", 9)
	if !slices.Equal(got, []string{"eins zwei", "drei vier"}) {
		t.Errorf("= %q", got)
	}
}

func TestZerlege_UeberlangesWortWirdHartGeschnitten(t *testing.T) {
	got := ZerlegeInAbschnitte(strings.Repeat("ä", 7), 3)
	if !slices.Equal(got, []string{"äää", "äää", "ä"}) {
		t.Errorf("= %q", got)
	}
	for _, a := range got {
		if !utf8.ValidString(a) {
			t.Errorf("Umlaut zerschnitten: %q", a)
		}
	}
}

func TestZerlege_UmlauteZaehlenAlsEinZeichen(t *testing.T) {
	// 5 Zeichen, aber 10 Bytes: passt in max 5.
	if got := ZerlegeInAbschnitte("äöüßé", 5); !slices.Equal(got, []string{"äöüßé"}) {
		t.Errorf("= %q", got)
	}
}

func TestZerlege_Leerraum(t *testing.T) {
	for _, text := range []string{"", "   ", "\n\n\n", " \t\n \n"} {
		got := ZerlegeInAbschnitte(text, 10)
		if got == nil || len(got) != 0 {
			t.Errorf("ZerlegeInAbschnitte(%q) = %#v, erwartet leere Liste", text, got)
		}
	}
}

func TestZerlege_ZeilenendenUndLeerzeilenMitLeerzeichen(t *testing.T) {
	// Windows-Zeilenenden werden zu "\n", und eine "Leerzeile" mit
	// Leerzeichen trennt Absätze. max 30 ist absichtlich groß genug, dass
	// alles in EINEN Abschnitt passt (12 + 2 + 5 = 19). So entscheidet nicht
	// die Länge, sondern nur das Zerlegen, wie das Ergebnis aussieht:
	//   - Leerzeichen-Zeile nicht als Trenner erkannt → "Kopf\nZeile 2\n  \nRumpf"
	//   - "\r\n" nicht umgewandelt                    → Ergebnis enthält "\r"
	got := ZerlegeInAbschnitte("Kopf\r\nZeile 2\r\n  \r\nRumpf", 30)
	if !slices.Equal(got, []string{"Kopf\nZeile 2\n\nRumpf"}) {
		t.Errorf("= %q", got)
	}
}

func TestZerlege_Invarianten(t *testing.T) {
	// Längerer synthetischer Text mit allen Fällen gemischt.
	text := strings.Repeat(laborbefund+"\n\n", 6) +
		strings.Repeat("Wort ", 300) + "\n\n" + strings.Repeat("x", 1200)

	for _, max := range []int{1, 7, 50, 120, AbschnittMaxZeichen} {
		got := ZerlegeInAbschnitte(text, max)
		if len(got) == 0 {
			t.Fatalf("max %d: keine Abschnitte", max)
		}
		for i, a := range got {
			if strings.TrimSpace(a) == "" {
				t.Errorf("max %d: Abschnitt %d ist leer", max, i)
			}
			if n := utf8.RuneCountInString(a); n > max {
				t.Errorf("max %d: Abschnitt %d hat %d Zeichen", max, i, n)
			}
		}
		// Nichts geht verloren: Alle Nicht-Leerzeichen bleiben in Reihenfolge.
		if ohneLeerraum(strings.Join(got, "")) != ohneLeerraum(text) {
			t.Errorf("max %d: Inhalt verändert", max)
		}
		// Deterministisch.
		if !slices.Equal(got, ZerlegeInAbschnitte(text, max)) {
			t.Errorf("max %d: zweiter Aufruf liefert anderes Ergebnis", max)
		}
	}
}

func TestZerlege_UngueltigesMaxNimmtStandard(t *testing.T) {
	if got := ZerlegeInAbschnitte(laborbefund, 0); len(got) != 1 {
		t.Errorf("max 0 soll AbschnittMaxZeichen verwenden, bekommen %d Abschnitte", len(got))
	}
}

func ohneLeerraum(s string) string {
	return strings.Join(strings.Fields(s), "")
}
