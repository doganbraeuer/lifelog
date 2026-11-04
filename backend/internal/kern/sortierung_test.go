package kern

import (
	"errors"
	"slices"
	"testing"
	"time"
)

// e baut einen Kurz-Eintrag. datum "" bedeutet: kein Dokumentdatum (null).
// importMinute ist die Minute des Imports am 03.11.2026, das hält die Fälle kurz.
func e(id, datum string, importMinute int) EintragKurz {
	k := EintragKurz{
		ID:          id,
		Importdatum: time.Date(2026, 11, 3, 18, importMinute, 0, 0, time.UTC),
	}
	if datum != "" {
		k.Dokumentdatum = &datum
	}
	return k
}

func ids(eintraege []EintragKurz) []string {
	out := make([]string, len(eintraege))
	for i, x := range eintraege {
		out[i] = x.ID
	}
	return out
}

func TestSortiere(t *testing.T) {
	// Gemischter Bestand für mehrere Fälle:
	// a: 14.10., Import :10 | b: 02.09., Import :20 | c: ohne Datum, Import :05
	// d: ohne Datum, Import :30 | e: 14.10., Import :40 (gleiches Datum wie a)
	bestand := func() []EintragKurz {
		return []EintragKurz{
			e("a", "2026-10-14", 10),
			e("c", "", 5),
			e("b", "2026-09-02", 20),
			e("d", "", 30),
			e("e", "2026-10-14", 40),
		}
	}

	faelle := []struct {
		name       string
		eingabe    []EintragKurz
		sortierung Sortierung
		richtung   Richtung
		erwartet   []string
	}{
		{
			name:    "Regel 1+2+3: Dokumentdatum absteigend, null am Ende, Gleichstand nach Import absteigend",
			eingabe: bestand(), sortierung: NachDokumentdatum, richtung: Absteigend,
			erwartet: []string{"e", "a", "b", "d", "c"},
		},
		{
			name:    "Regel 2: null auch aufsteigend am Ende und untereinander absteigend",
			eingabe: bestand(), sortierung: NachDokumentdatum, richtung: Aufsteigend,
			erwartet: []string{"b", "a", "e", "d", "c"},
		},
		{
			name:    "Importdatum absteigend, Dokumentdatum spielt keine Rolle",
			eingabe: bestand(), sortierung: NachImportdatum, richtung: Absteigend,
			erwartet: []string{"e", "d", "b", "a", "c"},
		},
		{
			name:    "Importdatum aufsteigend",
			eingabe: bestand(), sortierung: NachImportdatum, richtung: Aufsteigend,
			erwartet: []string{"c", "a", "b", "d", "e"},
		},
		{
			name: "Regel 3: gleiches Datum und gleicher Import, dann id absteigend",
			eingabe: []EintragKurz{
				e("x1", "2026-10-14", 10), e("x3", "2026-10-14", 10), e("x2", "2026-10-14", 10),
			},
			sortierung: NachDokumentdatum, richtung: Absteigend,
			erwartet: []string{"x3", "x2", "x1"},
		},
		{
			name: "Regel 3: gleiches Datum und gleicher Import, dann id aufsteigend",
			eingabe: []EintragKurz{
				e("x1", "2026-10-14", 10), e("x3", "2026-10-14", 10), e("x2", "2026-10-14", 10),
			},
			sortierung: NachDokumentdatum, richtung: Aufsteigend,
			erwartet: []string{"x1", "x2", "x3"},
		},
		{
			name: "Zwei Einträge ohne Datum mit gleichem Import: id absteigend",
			eingabe: []EintragKurz{
				e("n1", "", 10), e("n2", "", 10),
			},
			sortierung: NachDokumentdatum, richtung: Aufsteigend,
			erwartet: []string{"n2", "n1"},
		},
		{
			name: "Gleicher Import bei Sortierung nach Import: id in derselben Richtung",
			eingabe: []EintragKurz{
				e("m1", "2026-01-01", 10), e("m2", "", 10),
			},
			sortierung: NachImportdatum, richtung: Absteigend,
			erwartet: []string{"m2", "m1"},
		},
		{
			name: "Jahreswechsel: Text-Vergleich der Daten stimmt zeitlich",
			eingabe: []EintragKurz{
				e("alt", "2025-12-31", 10), e("neu", "2026-01-01", 10),
			},
			sortierung: NachDokumentdatum, richtung: Absteigend,
			erwartet: []string{"neu", "alt"},
		},
		{
			name: "Leere Liste", eingabe: []EintragKurz{},
			sortierung: NachDokumentdatum, richtung: Absteigend, erwartet: []string{},
		},
	}

	for _, f := range faelle {
		t.Run(f.name, func(t *testing.T) {
			Sortiere(f.eingabe, f.sortierung, f.richtung)
			if got := ids(f.eingabe); !slices.Equal(got, f.erwartet) {
				t.Errorf("Reihenfolge = %v, erwartet %v", got, f.erwartet)
			}
		})
	}
}

func TestSortiere_UnabhaengigVonEingabereihenfolge(t *testing.T) {
	// Eindeutige Reihenfolge heißt: Jede Permutation ergibt dasselbe Ergebnis.
	basis := []EintragKurz{
		e("a", "2026-10-14", 10), e("b", "2026-10-14", 10), e("c", "", 10),
		e("d", "", 10), e("e", "2026-09-02", 10),
	}
	var referenz []string
	for i := range 20 {
		kopie := slices.Clone(basis)
		// einfache, deterministische Durchmischung
		for j := range kopie {
			k := (j*7 + i*3) % len(kopie)
			kopie[j], kopie[k] = kopie[k], kopie[j]
		}
		Sortiere(kopie, NachDokumentdatum, Absteigend)
		if referenz == nil {
			referenz = ids(kopie)
			continue
		}
		if got := ids(kopie); !slices.Equal(got, referenz) {
			t.Fatalf("Durchlauf %d: %v, erwartet %v", i, got, referenz)
		}
	}
}

func TestSortierungAus(t *testing.T) {
	faelle := map[string]Sortierung{"": NachDokumentdatum, "dokumentdatum": NachDokumentdatum, "importdatum": NachImportdatum}
	for wert, erwartet := range faelle {
		got, err := SortierungAus(wert)
		if err != nil || got != erwartet {
			t.Errorf("SortierungAus(%q) = %q, %v", wert, got, err)
		}
	}
	for _, wert := range []string{"quelle", "Dokumentdatum", " importdatum"} {
		if _, err := SortierungAus(wert); !errors.Is(err, ErrUnbekannterWert) {
			t.Errorf("SortierungAus(%q): erwartet ErrUnbekannterWert, bekommen %v", wert, err)
		}
	}
}

func TestRichtungAus(t *testing.T) {
	faelle := map[string]Richtung{"": Absteigend, "absteigend": Absteigend, "aufsteigend": Aufsteigend}
	for wert, erwartet := range faelle {
		got, err := RichtungAus(wert)
		if err != nil || got != erwartet {
			t.Errorf("RichtungAus(%q) = %q, %v", wert, got, err)
		}
	}
	for _, wert := range []string{"desc", "asc", "Absteigend"} {
		if _, err := RichtungAus(wert); !errors.Is(err, ErrUnbekannterWert) {
			t.Errorf("RichtungAus(%q): erwartet ErrUnbekannterWert, bekommen %v", wert, err)
		}
	}
}
