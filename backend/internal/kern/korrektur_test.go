package kern

import (
	"errors"
	"reflect"
	"slices"
	"strings"
	"testing"
	"time"
)

func zk(s string) *string { return &s }

var (
	t1 = time.Date(2026, 11, 4, 19, 5, 31, 0, time.UTC)
	t2 = time.Date(2026, 11, 4, 19, 7, 0, 0, time.UTC)
)

// korrekturEintrag: synthetischer Laborbefund ohne Korrekturen.
func korrekturEintrag() Eintrag {
	return Eintrag{
		ID: "6727a1f3c2b4e5d6f7a8b9c0", Modul: "gesundheit", Typ: "laborbefund",
		Dokumentdatum: zk("2026-10-14"), Quelle: zk("Labor Muster GmbH"),
		Volltext: "Hämoglobin 14,2 g/dl …",
		Infos: []Info{
			{ID: "i1", Name: "Hämoglobin", Wert: "14,2", Einheit: zk("g/dl")},
			{ID: "i2", Name: "Leukozyten", Wert: "6,3", Einheit: zk("G/l")},
			{ID: "i3", Name: "Befundart", Wert: "Blutbild klein"},
		},
		Korrekturen: []Korrektur{},
	}
}

func setzen(s string) Feld { return Feld{Gesetzt: true, Wert: &s} }

var entfernen = Feld{Gesetzt: true}

// muss ruft Korrigiere auf und bricht bei einem Fehler ab.
func muss(t *testing.T, e Eintrag, a Aenderung, jetzt time.Time) (Eintrag, []string) {
	t.Helper()
	neu, geaendert, err := Korrigiere(e, a, jetzt)
	if err != nil {
		t.Fatalf("Korrigiere: %v", err)
	}
	return neu, geaendert
}

func text(s *string) string {
	if s == nil {
		return "<null>"
	}
	return *s
}

func pruefeKorrekturen(t *testing.T, got []Korrektur, want ...Korrektur) {
	t.Helper()
	if got == nil {
		t.Fatal("Korrekturen ist nil, erwartet eine Liste")
	}
	if len(got) != len(want) {
		t.Fatalf("%d Korrekturen, erwartet %d: %+v", len(got), len(want), got)
	}
	for i := range want {
		g, w := got[i], want[i]
		if g.Feld != w.Feld || text(g.Erkannt) != text(w.Erkannt) || text(g.Korrigiert) != text(w.Korrigiert) || !g.Zeitpunkt.Equal(w.Zeitpunkt) {
			t.Errorf("Korrektur %d = {%s %s→%s %v}, erwartet {%s %s→%s %v}", i,
				g.Feld, text(g.Erkannt), text(g.Korrigiert), g.Zeitpunkt,
				w.Feld, text(w.Erkannt), text(w.Korrigiert), w.Zeitpunkt)
		}
	}
}

func TestKorrigiere_ErsteKorrekturWieImAPIBeispiel(t *testing.T) {
	// API.md 3.5: Datum und Leukozyten in einer Anfrage.
	neu, geaendert := muss(t, korrekturEintrag(), Aenderung{
		Dokumentdatum: setzen("2026-10-15"),
		Infos:         []InfoAenderung{{ID: "i2", Wert: zk("6,8")}},
	}, t1)

	// Regel 1: Die Felder enthalten den aktuellen Wert.
	if text(neu.Dokumentdatum) != "2026-10-15" || neu.Infos[1].Wert != "6,8" {
		t.Errorf("Datum %s, i2 %s", text(neu.Dokumentdatum), neu.Infos[1].Wert)
	}
	pruefeKorrekturen(t, neu.Korrekturen,
		Korrektur{Feld: "dokumentdatum", Erkannt: zk("2026-10-14"), Korrigiert: zk("2026-10-15"), Zeitpunkt: t1},
		Korrektur{Feld: "infos.i2.wert", Erkannt: zk("6,3"), Korrigiert: zk("6,8"), Zeitpunkt: t1},
	)
	if !slices.Equal(geaendert, []string{"dokumentdatum", "infos.i2.wert"}) {
		t.Errorf("geaendert = %v", geaendert)
	}
	// Alles andere bleibt, wie es war.
	alt := korrekturEintrag()
	if neu.Typ != alt.Typ || text(neu.Quelle) != text(alt.Quelle) || neu.Volltext != alt.Volltext ||
		!reflect.DeepEqual(neu.Infos[0], alt.Infos[0]) || !reflect.DeepEqual(neu.Infos[2], alt.Infos[2]) {
		t.Errorf("unbeteiligte Felder geändert: %+v", neu)
	}
}

func TestKorrigiere_ZweimalKorrigiertBehaeltErkannt(t *testing.T) {
	// Regel 2: Ein Eintrag pro Feld, erkannt bleibt der Originalwert.
	e, _ := muss(t, korrekturEintrag(), Aenderung{Infos: []InfoAenderung{{ID: "i2", Wert: zk("6,8")}}}, t1)
	e, _ = muss(t, e, Aenderung{Infos: []InfoAenderung{{ID: "i2", Wert: zk("6,9")}}}, t2)

	pruefeKorrekturen(t, e.Korrekturen,
		Korrektur{Feld: "infos.i2.wert", Erkannt: zk("6,3"), Korrigiert: zk("6,9"), Zeitpunkt: t2})
}

func TestKorrigiere_TypZweimal(t *testing.T) {
	// Fängt den Zeiger-Fehler: lies darf für typ keinen Zeiger ins Feld
	// liefern, sonst wäre "erkannt" nach dem Schreiben der neue Wert und die
	// Korrektur würde als "zurückgesetzt" verschwinden.
	e, _ := muss(t, korrekturEintrag(), Aenderung{Typ: zk("arztbrief")}, t1)
	pruefeKorrekturen(t, e.Korrekturen,
		Korrektur{Feld: "typ", Erkannt: zk("laborbefund"), Korrigiert: zk("arztbrief"), Zeitpunkt: t1})

	e, _ = muss(t, e, Aenderung{Typ: zk("rezept")}, t2)
	if e.Typ != "rezept" {
		t.Errorf("Typ = %s", e.Typ)
	}
	pruefeKorrekturen(t, e.Korrekturen,
		Korrektur{Feld: "typ", Erkannt: zk("laborbefund"), Korrigiert: zk("rezept"), Zeitpunkt: t2})
}

func TestKorrigiere_ZuruecksetzenEntferntKorrektur(t *testing.T) {
	// Regel 3, für jede Art von Feld.
	faelle := []struct {
		name        string
		hin, zurück Aenderung
	}{
		{"typ", Aenderung{Typ: zk("rezept")}, Aenderung{Typ: zk("laborbefund")}},
		{"datum", Aenderung{Dokumentdatum: setzen("2026-10-15")}, Aenderung{Dokumentdatum: setzen("2026-10-14")}},
		{"datum entfernt und wieder da", Aenderung{Dokumentdatum: entfernen}, Aenderung{Dokumentdatum: setzen("2026-10-14")}},
		{"quelle", Aenderung{Quelle: setzen("Labor B")}, Aenderung{Quelle: setzen("Labor Muster GmbH")}},
		{"wert", Aenderung{Infos: []InfoAenderung{{ID: "i2", Wert: zk("6,8")}}}, Aenderung{Infos: []InfoAenderung{{ID: "i2", Wert: zk("6,3")}}}},
		{"einheit entfernt", Aenderung{Infos: []InfoAenderung{{ID: "i1", Einheit: entfernen}}}, Aenderung{Infos: []InfoAenderung{{ID: "i1", Einheit: setzen("g/dl")}}}},
		{"einheit neu", Aenderung{Infos: []InfoAenderung{{ID: "i3", Einheit: setzen("x")}}}, Aenderung{Infos: []InfoAenderung{{ID: "i3", Einheit: entfernen}}}},
	}
	for _, f := range faelle {
		t.Run(f.name, func(t *testing.T) {
			e, _ := muss(t, korrekturEintrag(), f.hin, t1)
			if len(e.Korrekturen) != 1 {
				t.Fatalf("nach dem Korrigieren %d Korrekturen", len(e.Korrekturen))
			}
			e, geaendert := muss(t, e, f.zurück, t2)
			pruefeKorrekturen(t, e.Korrekturen) // leer
			if !reflect.DeepEqual(e, korrekturEintrag()) {
				t.Errorf("nach dem Zurücksetzen nicht wie vorher:\n%+v", e)
			}
			if len(geaendert) != 1 {
				t.Errorf("geaendert = %v", geaendert)
			}
		})
	}
}

func TestKorrigiere_NullWerte(t *testing.T) {
	// Datum war nicht erkannt (null) und wird gesetzt: erkannt ist null.
	ohneDatum := korrekturEintrag()
	ohneDatum.Dokumentdatum = nil
	e, _ := muss(t, ohneDatum, Aenderung{Dokumentdatum: setzen("2026-10-14")}, t1)
	pruefeKorrekturen(t, e.Korrekturen,
		Korrektur{Feld: "dokumentdatum", Erkannt: nil, Korrigiert: zk("2026-10-14"), Zeitpunkt: t1})

	// Quelle entfernen: korrigiert ist null.
	e, _ = muss(t, korrekturEintrag(), Aenderung{Quelle: entfernen}, t1)
	if e.Quelle != nil {
		t.Errorf("Quelle = %s", text(e.Quelle))
	}
	pruefeKorrekturen(t, e.Korrekturen,
		Korrektur{Feld: "quelle", Erkannt: zk("Labor Muster GmbH"), Korrigiert: nil, Zeitpunkt: t1})

	// Einheit hinzufügen, wo keine war, und eine entfernen.
	e, _ = muss(t, korrekturEintrag(), Aenderung{Infos: []InfoAenderung{
		{ID: "i3", Einheit: setzen("Stück")}, {ID: "i1", Einheit: entfernen},
	}}, t1)
	if text(e.Infos[2].Einheit) != "Stück" || e.Infos[0].Einheit != nil {
		t.Errorf("Einheiten: i3 %s, i1 %s", text(e.Infos[2].Einheit), text(e.Infos[0].Einheit))
	}
	pruefeKorrekturen(t, e.Korrekturen,
		Korrektur{Feld: "infos.i3.einheit", Erkannt: nil, Korrigiert: zk("Stück"), Zeitpunkt: t1},
		Korrektur{Feld: "infos.i1.einheit", Erkannt: zk("g/dl"), Korrigiert: nil, Zeitpunkt: t1},
	)
}

func TestKorrigiere_OhneAenderungKeineKorrektur(t *testing.T) {
	// Regel 4, auch mit Leerraum und mit null, wo schon null ist.
	ohneDatum := korrekturEintrag()
	ohneDatum.Dokumentdatum = nil
	for name, a := range map[string]Aenderung{
		"gleicher typ":      {Typ: zk(" laborbefund ")},
		"gleiches datum":    {Dokumentdatum: setzen("2026-10-14")},
		"gleiche quelle":    {Quelle: setzen("  Labor Muster GmbH ")},
		"gleicher wert":     {Infos: []InfoAenderung{{ID: "i1", Wert: zk("14,2 ")}}},
		"keine einheit":     {Infos: []InfoAenderung{{ID: "i3", Einheit: entfernen}}},
		"leere einheit":     {Infos: []InfoAenderung{{ID: "i3", Einheit: setzen("  ")}}},
		"leere infos-liste": {Infos: []InfoAenderung{}},
	} {
		t.Run(name, func(t *testing.T) {
			neu, geaendert := muss(t, korrekturEintrag(), a, t1)
			if len(geaendert) != 0 || !reflect.DeepEqual(neu, korrekturEintrag()) {
				t.Errorf("geaendert %v, Eintrag %+v", geaendert, neu)
			}
		})
	}
	neu, geaendert := muss(t, ohneDatum, Aenderung{Dokumentdatum: entfernen}, t1)
	if len(geaendert) != 0 || len(neu.Korrekturen) != 0 {
		t.Errorf("null auf null: geaendert %v, %+v", geaendert, neu.Korrekturen)
	}
}

func TestKorrigiere_Leerraum(t *testing.T) {
	neu, _ := muss(t, korrekturEintrag(), Aenderung{
		Typ:           zk(" arztbrief\n"),
		Dokumentdatum: setzen(" 2026-10-15 "),
		Quelle:        setzen("   "), // leer → null
		Infos:         []InfoAenderung{{ID: "i2", Wert: zk(" 6,8 "), Einheit: setzen("")}},
	}, t1)
	if neu.Typ != "arztbrief" || text(neu.Dokumentdatum) != "2026-10-15" || neu.Quelle != nil ||
		neu.Infos[1].Wert != "6,8" || neu.Infos[1].Einheit != nil {
		t.Errorf("= typ %q datum %s quelle %s wert %q einheit %s", neu.Typ, text(neu.Dokumentdatum),
			text(neu.Quelle), neu.Infos[1].Wert, text(neu.Infos[1].Einheit))
	}
}

func TestKorrigiere_EingabeBleibtUnveraendert(t *testing.T) {
	// Eintrag mit einer vorhandenen Korrektur, die geändert wird (Regel 2),
	// und einer, die entfernt wird (Regel 3).
	e, _ := muss(t, korrekturEintrag(), Aenderung{
		Quelle: setzen("Labor B"),
		Infos:  []InfoAenderung{{ID: "i2", Wert: zk("6,8")}, {ID: "i1", Einheit: entfernen}},
	}, t1)
	vorher := kopie(e)

	_, _ = muss(t, e, Aenderung{
		Quelle: setzen("Labor C"),
		Infos:  []InfoAenderung{{ID: "i2", Wert: zk("6,3")}, {ID: "i1", Wert: zk("14,0")}},
	}, t2)

	if !reflect.DeepEqual(e, vorher) {
		t.Errorf("Eingabe verändert:\nvorher  %+v\nnachher %+v", vorher, e)
	}
}

// kopie: tiefe Kopie, damit der Vergleich auch geteilte Zeiger erfasst.
func kopie(e Eintrag) Eintrag {
	k := e
	k.Infos = nil
	for _, i := range e.Infos {
		if i.Einheit != nil {
			i.Einheit = zk(*i.Einheit)
		}
		k.Infos = append(k.Infos, i)
	}
	k.Korrekturen = nil
	for _, ko := range e.Korrekturen {
		if ko.Erkannt != nil {
			ko.Erkannt = zk(*ko.Erkannt)
		}
		if ko.Korrigiert != nil {
			ko.Korrigiert = zk(*ko.Korrigiert)
		}
		k.Korrekturen = append(k.Korrekturen, ko)
	}
	if e.Quelle != nil {
		k.Quelle = zk(*e.Quelle)
	}
	if e.Dokumentdatum != nil {
		k.Dokumentdatum = zk(*e.Dokumentdatum)
	}
	return k
}

func TestKorrigiere_KorrekturenNilWirdListe(t *testing.T) {
	e := korrekturEintrag()
	e.Korrekturen = nil
	neu, _ := muss(t, e, Aenderung{Typ: zk("laborbefund")}, t1)
	if neu.Korrekturen == nil {
		t.Error("Korrekturen nil")
	}
}

func TestKorrigiere_ZeitpunktInUTC(t *testing.T) {
	wien := time.FixedZone("MEZ", 3600)
	neu, _ := muss(t, korrekturEintrag(), Aenderung{Typ: zk("rezept")}, t1.In(wien))
	if neu.Korrekturen[0].Zeitpunkt.Location() != time.UTC || !neu.Korrekturen[0].Zeitpunkt.Equal(t1) {
		t.Errorf("Zeitpunkt = %v", neu.Korrekturen[0].Zeitpunkt)
	}
}

func TestKorrigiere_Fehler(t *testing.T) {
	faelle := []struct {
		name      string
		a         Aenderung
		art       error
		feld      string
		infoID    string
		wert      any
		nachricht string
	}{
		{"leer", Aenderung{}, ErrKorrekturUngueltig, "", "", nil, "keine Änderung"},
		{"typ unbekannt", Aenderung{Typ: zk("befund")}, ErrKorrekturUngueltig, "typ", "", "befund",
			"Erlaubt sind arztbrief, laborbefund, rezept, sonstiges."},
		{"datum format", Aenderung{Dokumentdatum: setzen("15.10.2026")}, ErrKorrekturUngueltig, "dokumentdatum", "", "15.10.2026",
			"Das Dokumentdatum muss im Format JJJJ-MM-TT angegeben werden."},
		{"datum leer", Aenderung{Dokumentdatum: setzen(" ")}, ErrKorrekturUngueltig, "dokumentdatum", "", " ", "Format JJJJ-MM-TT"},
		{"kein kalendertag", Aenderung{Dokumentdatum: setzen("2026-02-31")}, ErrKorrekturUngueltig, "dokumentdatum", "", "2026-02-31",
			"Das Dokumentdatum ist kein gültiges Kalenderdatum."},
		{"info ohne id", Aenderung{Infos: []InfoAenderung{{Wert: zk("1")}}}, ErrKorrekturUngueltig, "infos", "", nil, "braucht eine id"},
		{"info doppelt", Aenderung{Infos: []InfoAenderung{{ID: "i1", Wert: zk("1")}, {ID: "i1", Einheit: entfernen}}},
			ErrKorrekturUngueltig, "", "i1", nil, "mehrfach"},
		{"info ohne änderung", Aenderung{Infos: []InfoAenderung{{ID: "i1"}}}, ErrKorrekturUngueltig, "", "i1", nil, "keine Änderung"},
		{"info unbekannt", Aenderung{Infos: []InfoAenderung{{ID: "i9", Wert: zk("1")}}}, ErrInfoNichtGefunden, "", "i9", nil, "gibt es in dem Eintrag nicht"},
		{"wert leer", Aenderung{Infos: []InfoAenderung{{ID: "i2", Wert: zk("  ")}}}, ErrKorrekturUngueltig, "infos.i2.wert", "", "  ", "nicht leer"},
		// Gültiges zuerst, ungültige Info danach: Fehler, nichts geändert.
		{"teilweise gültig", Aenderung{Quelle: setzen("Labor B"), Infos: []InfoAenderung{{ID: "i2", Wert: zk("6,8")}, {ID: "i7", Wert: zk("1")}}},
			ErrInfoNichtGefunden, "", "i7", nil, ""},
	}
	for _, f := range faelle {
		t.Run(f.name, func(t *testing.T) {
			e := korrekturEintrag()
			neu, geaendert, err := Korrigiere(e, f.a, t1)
			if !errors.Is(err, f.art) {
				t.Fatalf("err = %v, erwartet %v", err, f.art)
			}
			var kf *KorrekturFehler
			if !errors.As(err, &kf) {
				t.Fatalf("kein KorrekturFehler: %T", err)
			}
			if kf.Feld != f.feld || kf.InfoID != f.infoID || !reflect.DeepEqual(kf.Wert, f.wert) {
				t.Errorf("Feld %q InfoID %q Wert %#v, erwartet %q %q %#v", kf.Feld, kf.InfoID, kf.Wert, f.feld, f.infoID, f.wert)
			}
			if !strings.Contains(kf.Nachricht, f.nachricht) {
				t.Errorf("Nachricht %q enthält nicht %q", kf.Nachricht, f.nachricht)
			}
			if neu.ID != "" || geaendert != nil {
				t.Errorf("bei Fehler Ergebnis %+v, geaendert %v", neu, geaendert)
			}
			if !reflect.DeepEqual(e, korrekturEintrag()) {
				t.Error("Eingabe bei Fehler verändert")
			}
			// Error() ist fürs Log: kein Wert, aber das Ziel.
			if s, ok := f.wert.(string); ok && strings.TrimSpace(s) != "" && strings.Contains(err.Error(), s) {
				t.Errorf("Error() enthält den Wert: %q", err.Error())
			}
		})
	}
}

func TestKorrekturFehler_ErrorUndKonstruktoren(t *testing.T) {
	faelle := map[string]error{
		"Korrektur ungültig":                     Ungueltig("x", "", nil),
		"Korrektur ungültig: quelle":             Ungueltig("x", "quelle", 42),
		"Korrektur ungültig: infos.i1":           &KorrekturFehler{Art: ErrKorrekturUngueltig, InfoID: "i1"},
		"Feld nicht korrigierbar: volltext":      NichtKorrigierbar("volltext"),
		"Feld nicht korrigierbar: infos.i1.name": NichtKorrigierbar("infos.i1.name"),
	}
	for soll, err := range faelle {
		if err.Error() != soll {
			t.Errorf("Error() = %q, erwartet %q", err.Error(), soll)
		}
	}
	nk := NichtKorrigierbar("volltext")
	if !errors.Is(nk, ErrFeldNichtKorrigierbar) || errors.Is(nk, ErrKorrekturUngueltig) {
		t.Error("NichtKorrigierbar hat die falsche Art")
	}
	if nk.Nachricht != "Das Feld volltext kann nicht korrigiert werden." {
		t.Errorf("Nachricht = %q", nk.Nachricht)
	}
}
