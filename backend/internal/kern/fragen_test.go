package kern

import (
	"errors"
	"math"
	"slices"
	"strings"
	"testing"
)

func TestPruefeFrage(t *testing.T) {
	if f, err := PruefeFrage("  Wie hoch war mein Hb?\n"); err != nil || f != "Wie hoch war mein Hb?" {
		t.Errorf("= %q, %v", f, err)
	}
	for _, leer := range []string{"", "   ", "\n\t"} {
		if _, err := PruefeFrage(leer); !errors.Is(err, ErrFrageFehlt) {
			t.Errorf("%q: %v", leer, err)
		}
	}
	// Grenze in Zeichen, nicht Bytes: 500 × "ä" (1000 Bytes) ist erlaubt.
	if _, err := PruefeFrage(strings.Repeat("ä", 500)); err != nil {
		t.Errorf("500 Zeichen: %v", err)
	}
	if _, err := PruefeFrage(strings.Repeat("ä", 501)); !errors.Is(err, ErrFrageZuLang) {
		t.Errorf("501 Zeichen: %v", err)
	}
	// Leerraum am Rand zählt nicht mit.
	if _, err := PruefeFrage("  " + strings.Repeat("a", 500) + "  "); err != nil {
		t.Errorf("500 Zeichen mit Rand: %v", err)
	}
}

func TestScoreUmrechnung(t *testing.T) {
	faelle := map[float64]float64{-1: 0, 0: 0.5, 0.6: 0.8, 1: 1}
	for cos, score := range faelle {
		if got := ScoreAusKosinus(cos); math.Abs(got-score) > 1e-12 {
			t.Errorf("ScoreAusKosinus(%v) = %v, erwartet %v", cos, got, score)
		}
		if got := KosinusAusScore(score); math.Abs(got-cos) > 1e-12 {
			t.Errorf("KosinusAusScore(%v) = %v, erwartet %v", score, got, cos)
		}
	}
	// Die Schwelle 0,80 entspricht cos 0,60.
	if got := KosinusAusScore(MinScore); math.Abs(got-0.6) > 1e-12 {
		t.Errorf("MinScore entspricht cos %v", got)
	}
}

func TestUeberSchwelle(t *testing.T) {
	treffer := []Treffer{{Nr: 1, Score: 0.90}, {Nr: 2, Score: 0.80}, {Nr: 3, Score: 0.7999}, {Nr: 4, Score: 0.85}}
	got := UeberSchwelle(treffer, MinScore)
	var nrs []int
	for _, t := range got {
		nrs = append(nrs, t.Nr)
	}
	// Genau auf der Schwelle zählt als Treffer; Reihenfolge bleibt.
	if !slices.Equal(nrs, []int{1, 2, 4}) {
		t.Errorf("= %v", nrs)
	}
	if leer := UeberSchwelle(nil, MinScore); leer == nil || len(leer) != 0 {
		t.Errorf("nil → %#v, erwartet leere Liste", leer)
	}
}

func TestWerteZitateAus(t *testing.T) {
	faelle := []struct {
		text     string
		anzahl   int
		erwartet []int
	}{
		{"Hämoglobin 14,2 g/dl [1].", 3, []int{1}},
		{"A [2]. B [1][3].", 3, []int{2, 1, 3}},
		{"A [1, 3] und [2,1].", 3, []int{1, 3, 2}},
		{"Doppelt [1] [1] [1].", 3, []int{1}},
		{"Unbekannte Nummer [7] und [0].", 3, []int{}},
		{"Teils gültig [2] [9].", 3, []int{2}},
		{"Kein Verweis.", 3, []int{}},
		{FesterSatz, 3, []int{}},
		{"Wert [1a] oder [ 1 ] oder (1).", 3, []int{}},
		{"[12] bei zwölf Fundstellen", 12, []int{12}},
	}
	for _, f := range faelle {
		got := WerteZitateAus(f.text, f.anzahl)
		if got == nil || !slices.Equal(got, f.erwartet) {
			t.Errorf("WerteZitateAus(%q, %d) = %#v, erwartet %v", f.text, f.anzahl, got, f.erwartet)
		}
	}
}

func TestBaueAntwort(t *testing.T) {
	fundstellen := []Fundstelle{
		{Nr: 1, EintragID: "a", Auszug: "Hämoglobin 14,2 g/dl"},
		{Nr: 2, EintragID: "b", Auszug: "Blutdruck 128/82"},
		{Nr: 3, EintragID: "a", Auszug: "CRP <0,5"},
	}

	a := BaueAntwort("  Laut Befund: Hämoglobin 14,2 g/dl [1], CRP <0,5 [3].  ", fundstellen)
	if !a.Gefunden || a.Text != "Laut Befund: Hämoglobin 14,2 g/dl [1], CRP <0,5 [3]." {
		t.Errorf("Antwort = %+v", a)
	}
	var nrs []int
	for _, q := range a.Quellen {
		nrs = append(nrs, q.Nr)
	}
	if !slices.Equal(nrs, []int{1, 3}) {
		t.Errorf("Quellen = %v, erwartet nur die zitierten [1 3]", nrs)
	}

	// Ohne gültigen Verweis: fester Satz, auch wenn das Modell etwas anderes schrieb.
	for _, text := range []string{FesterSatz, "Der Wert ist 14,2.", "Siehe [9].", ""} {
		a := BaueAntwort(text, fundstellen)
		if a.Gefunden || a.Text != FesterSatz || a.Quellen == nil || len(a.Quellen) != 0 {
			t.Errorf("%q → %+v, erwartet KeinTreffer", text, a)
		}
	}
}
