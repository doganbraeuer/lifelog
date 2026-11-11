package core

import (
	"errors"
	"slices"
	"strings"
	"testing"
)

func z(s string) *string { return &s }

// gueltig ist eine synthetische Erkennung, wie Gemini sie liefern könnte.
func gueltig() Erkennung {
	return Erkennung{
		Lesbar:        true,
		Dokumentdatum: z("2026-10-14"),
		Quelle:        z("Labor Muster GmbH, Salzburg"),
		Typ:           "laborbefund",
		Volltext:      laborbefund,
		Infos: []ErkannteInfo{
			{Name: "Hämoglobin", Wert: "14,2", Einheit: z("g/dl")},
			{Name: "CRP", Wert: "<0,5", Einheit: z("mg/dl")},
			{Name: "Befundart", Wert: "Blutbild klein"},
		},
	}
}

func TestPruefeErkennung_Gueltig(t *testing.T) {
	e, ausgelassen, err := PruefeErkennung(gueltig(), "gesundheit")
	if err != nil {
		t.Fatalf("unerwarteter Fehler: %v", err)
	}
	if ausgelassen == nil || len(ausgelassen) != 0 {
		t.Errorf("ausgelassen = %#v, erwartet leere Liste", ausgelassen)
	}
	if e.Modul != "gesundheit" || e.Typ != "laborbefund" {
		t.Errorf("Modul/Typ = %q/%q", e.Modul, e.Typ)
	}
	if *e.Dokumentdatum != "2026-10-14" || *e.Quelle != "Labor Muster GmbH, Salzburg" {
		t.Errorf("Datum/Quelle = %v/%v", *e.Dokumentdatum, *e.Quelle)
	}
	if e.Volltext != laborbefund {
		t.Error("Volltext wurde verändert")
	}
	if e.Korrekturen == nil || len(e.Korrekturen) != 0 {
		t.Errorf("Korrekturen = %#v, erwartet leere Liste", e.Korrekturen)
	}

	if len(e.Infos) != 3 {
		t.Fatalf("Infos = %d", len(e.Infos))
	}
	for i, id := range []string{"i1", "i2", "i3"} {
		if e.Infos[i].ID != id {
			t.Errorf("Infos[%d].ID = %q, erwartet %q", i, e.Infos[i].ID, id)
		}
	}
	if e.Infos[1].Wert != "<0,5" {
		t.Errorf("Wert verändert: %q", e.Infos[1].Wert)
	}
	if e.Infos[2].Einheit != nil {
		t.Error("Info ohne Einheit muss nil haben")
	}
}

func TestPruefeErkennung_LeereWerteWerdenNull(t *testing.T) {
	g := gueltig()
	g.Dokumentdatum = z("  ")
	g.Quelle = z("")
	g.Infos = []ErkannteInfo{{Name: " Hämoglobin ", Wert: " 14,2 ", Einheit: z(" ")}}

	e, _, err := PruefeErkennung(g, "gesundheit")
	if err != nil {
		t.Fatal(err)
	}
	if e.Dokumentdatum != nil || e.Quelle != nil {
		t.Errorf("leeres Datum/leere Quelle müssen nil sein: %v %v", e.Dokumentdatum, e.Quelle)
	}
	i := e.Infos[0]
	if i.Name != "Hämoglobin" || i.Wert != "14,2" || i.Einheit != nil {
		t.Errorf("Info = %+v", i)
	}
}

func TestPruefeErkennung_OhneInfosLeereListe(t *testing.T) {
	g := gueltig()
	g.Infos = nil
	e, _, err := PruefeErkennung(g, "gesundheit")
	if err != nil {
		t.Fatal(err)
	}
	if e.Infos == nil {
		t.Error("Infos muss leere Liste sein, nicht nil")
	}
}

func TestPruefeErkennung_NichtLesbar(t *testing.T) {
	g := gueltig()
	g.Lesbar = false
	if _, _, err := PruefeErkennung(g, "gesundheit"); !errors.Is(err, ErrNichtLesbar) {
		t.Errorf("erwartet ErrNichtLesbar, bekommen %v", err)
	}
}

func TestPruefeErkennung_Fehler(t *testing.T) {
	faelle := []struct {
		name    string
		aendern func(*Erkennung)
		modul   string
		feld    string
	}{
		{"unbekannter Typ", func(e *Erkennung) { e.Typ = "kochrezept" }, "gesundheit", "typ"},
		{"leerer Typ", func(e *Erkennung) { e.Typ = "" }, "gesundheit", "typ"},
		{"Typ in fremdem Modul", func(e *Erkennung) {}, "sport", "typ"},
		{"leerer Volltext", func(e *Erkennung) { e.Volltext = " \n " }, "gesundheit", "volltext"},
		{"Datum deutsch", func(e *Erkennung) { e.Dokumentdatum = z("14.10.2026") }, "gesundheit", "dokumentdatum"},
		{"Datum gibt es nicht", func(e *Erkennung) { e.Dokumentdatum = z("2026-02-31") }, "gesundheit", "dokumentdatum"},
		{"Datum mit Uhrzeit", func(e *Erkennung) { e.Dokumentdatum = z("2026-10-14T00:00:00Z") }, "gesundheit", "dokumentdatum"},
	}
	for _, f := range faelle {
		t.Run(f.name, func(t *testing.T) {
			g := gueltig()
			f.aendern(&g)
			_, _, err := PruefeErkennung(g, f.modul)

			if !errors.Is(err, ErrErkennungUngueltig) {
				t.Fatalf("erwartet ErrErkennungUngueltig, bekommen %v", err)
			}
			var ef *ErkennungFehler
			if !errors.As(err, &ef) || ef.Feld != f.feld {
				t.Errorf("Feld = %v, erwartet %q", err, f.feld)
			}
		})
	}
}

func TestPruefeErkennung_FehlerEnthaeltKeineWerte(t *testing.T) {
	g := gueltig()
	g.Dokumentdatum = z("Hämoglobin 14,2 geheim")
	_, _, err := PruefeErkennung(g, "gesundheit")
	if err == nil || strings.Contains(err.Error(), "geheim") {
		t.Errorf("Fehlermeldung darf den Wert nicht enthalten: %v", err)
	}
}

func TestPruefeErkennung_UnvollstaendigeInfosWerdenWeggelassen(t *testing.T) {
	g := gueltig()
	g.Infos = []ErkannteInfo{
		{Name: "Hämoglobin", Wert: "14,2", Einheit: z("g/dl")}, // 0: bleibt → i1
		{Name: " ", Wert: "geheim-ohne-name"},                  // 1: ohne Name
		{Name: "Leukozyten", Wert: "", Einheit: z("G/l")},      // 2: ohne Wert
		{Name: "", Wert: "  "},                                 // 3: ohne beides
		{Name: "CRP", Wert: "<0,5", Einheit: z("mg/dl")},       // 4: bleibt → i2
	}

	e, ausgelassen, err := PruefeErkennung(g, "gesundheit")
	if err != nil {
		t.Fatalf("eine unvollständige Info darf den Upload nicht ablehnen: %v", err)
	}

	// Übrig bleiben die vollständigen Infos, lückenlos nummeriert.
	if len(e.Infos) != 2 {
		t.Fatalf("Infos = %+v, erwartet 2", e.Infos)
	}
	if e.Infos[0].ID != "i1" || e.Infos[0].Name != "Hämoglobin" ||
		e.Infos[1].ID != "i2" || e.Infos[1].Name != "CRP" {
		t.Errorf("Infos = %+v", e.Infos)
	}

	// Gemeldet werden nur Feldpfade mit dem Index aus der Modell-Liste.
	erwartet := []string{"infos[1].name", "infos[2].wert", "infos[3].name", "infos[3].wert"}
	if !slices.Equal(ausgelassen, erwartet) {
		t.Errorf("ausgelassen = %q, erwartet %q", ausgelassen, erwartet)
	}
	for _, a := range ausgelassen {
		if strings.Contains(a, "geheim") || strings.Contains(a, "Leukozyten") {
			t.Errorf("ausgelassen enthält einen Wert oder Namen: %q", a)
		}
	}
}

func TestPruefeErkennung_AlleInfosUnvollstaendig(t *testing.T) {
	g := gueltig()
	g.Infos = []ErkannteInfo{{Name: "", Wert: "x"}}

	e, ausgelassen, err := PruefeErkennung(g, "gesundheit")
	if err != nil {
		t.Fatal(err)
	}
	if e.Infos == nil || len(e.Infos) != 0 {
		t.Errorf("Infos = %#v, erwartet leere Liste", e.Infos)
	}
	if !slices.Equal(ausgelassen, []string{"infos[0].name"}) {
		t.Errorf("ausgelassen = %q", ausgelassen)
	}
}

func TestPruefeErkennung_AusgelassenNieNilAuchBeiFehler(t *testing.T) {
	g := gueltig()
	g.Lesbar = false
	if _, ausgelassen, _ := PruefeErkennung(g, "gesundheit"); ausgelassen == nil {
		t.Error("ausgelassen darf auch bei Fehlern nicht nil sein")
	}
}

func TestPruefeDatum(t *testing.T) {
	for _, ok := range []string{"2026-10-14", "2024-02-29", "1999-12-31", "2026-01-01"} {
		if err := PruefeDatum(ok); err != nil {
			t.Errorf("PruefeDatum(%q) = %v", ok, err)
		}
	}
	for _, falsch := range []string{
		"", "2026-1-5", "2026-01-5", "14.10.2026", "2026/10/14",
		"2025-02-29", // kein Schaltjahr
		"2026-02-30", "2026-04-31", "2026-13-01", "2026-00-10", "2026-10-00",
		"2026-10-14 ", " 2026-10-14", "2026-10-14T10:00:00Z", "２０２６-10-14",
	} {
		if err := PruefeDatum(falsch); !errors.Is(err, ErrDatumUngueltig) {
			t.Errorf("PruefeDatum(%q) = %v, erwartet ErrDatumUngueltig", falsch, err)
		}
	}

	// Format falsch und Tag unmöglich sind unterscheidbar (API.md 3.5).
	for _, format := range []string{"", "14.10.2026", "2026-1-05", "2026/10/14", "2026-10-1a", " 2026-10-14", "２０２６-10-14"} {
		if err := PruefeDatum(format); errors.Is(err, ErrKeinKalendertag) {
			t.Errorf("PruefeDatum(%q): Formatfehler als ErrKeinKalendertag gemeldet", format)
		}
	}
	for _, tag := range []string{"2025-02-29", "2026-02-30", "2026-04-31", "2026-13-01", "2026-00-10", "2026-10-00"} {
		if err := PruefeDatum(tag); !errors.Is(err, ErrKeinKalendertag) {
			t.Errorf("PruefeDatum(%q) = %v, erwartet ErrKeinKalendertag", tag, err)
		}
	}
}
