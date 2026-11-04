package dienst

import (
	"bytes"
	"context"
	"errors"
	"log/slog"
	"strings"
	"testing"

	"lifelog/backend/internal/kern"
	"lifelog/backend/internal/modell"
	"lifelog/backend/internal/modell/attrappe"
)

type suchAttrappe struct {
	treffer      []kern.Treffer
	eintraege    map[string]kern.EintragKurz
	err          error
	gefragtModul string
	gefragtK     int
}

func (s *suchAttrappe) AehnlicheAbschnitte(_ context.Context, _ []float32, modul string, k int) ([]kern.Treffer, error) {
	s.gefragtModul, s.gefragtK = modul, k
	return s.treffer, s.err
}

func (s *suchAttrappe) EintraegeKurzNachIDs(_ context.Context, ids []string) (map[string]kern.EintragKurz, error) {
	m := map[string]kern.EintragKurz{}
	for _, id := range ids {
		if e, ok := s.eintraege[id]; ok {
			m[id] = e
		}
	}
	return m, nil
}

const (
	idLabor = "aaaaaaaaaaaaaaaaaaaaaaaa"
	idBrief = "bbbbbbbbbbbbbbbbbbbbbbbb"
	idWeg   = "cccccccccccccccccccccccc" // Eintrag gelöscht, Abschnitt noch da
)

func suchBestand() *suchAttrappe {
	return &suchAttrappe{
		treffer: []kern.Treffer{
			{EintragID: idLabor, Nr: 3, Text: "Hämoglobin 14,2 g/dl", Score: 0.89},
			{EintragID: idWeg, Nr: 1, Text: "verwaist", Score: 0.86},
			{EintragID: idBrief, Nr: 1, Text: "Blutdruck 128/82", Score: kern.MinScore},              // genau auf der Schwelle
			{EintragID: idBrief, Nr: 2, Text: "Kontrolle in 6 Monaten", Score: kern.MinScore - 0.01}, // darunter
		},
		eintraege: map[string]kern.EintragKurz{
			idLabor: {ID: idLabor, Modul: "gesundheit", Typ: "laborbefund", Dokumentdatum: zeiger("2026-10-14"), Quelle: zeiger("Labor Muster GmbH")},
			idBrief: {ID: idBrief, Modul: "gesundheit", Typ: "arztbrief", Dokumentdatum: zeiger("2026-09-02"), Quelle: zeiger("Dr. Erika Beispiel")},
		},
	}
}

func TestFragen_Treffer(t *testing.T) {
	s := suchBestand()
	a := &attrappe.Antworter{Text: "Im Laborbefund steht Hämoglobin 14,2 g/dl [1]."}
	f := NeuFragen(s, &attrappe.Einbetter{}, a)

	antwort, err := f.Ausfuehren(context.Background(), "  Wie hoch war mein Hb?  ", "")
	if err != nil {
		t.Fatal(err)
	}
	if s.gefragtModul != "gesundheit" || s.gefragtK != kern.AnzahlTreffer {
		t.Errorf("Suche mit Modul %q, k %d", s.gefragtModul, s.gefragtK)
	}

	// Ans Modell gingen nur Treffer ≥ MinScore mit vorhandenem Eintrag, neu nummeriert.
	frage, fs := a.LetzteAnfrage()
	if frage != "Wie hoch war mein Hb?" {
		t.Errorf("Frage = %q, erwartet ohne Leerraum", frage)
	}
	if len(fs) != 2 || fs[0].Nr != 1 || fs[0].EintragID != idLabor || fs[1].Nr != 2 || fs[1].EintragID != idBrief {
		t.Fatalf("Fundstellen = %+v", fs)
	}
	if fs[0].Typ != "laborbefund" || *fs[0].Dokumentdatum != "2026-10-14" || *fs[0].Quelle != "Labor Muster GmbH" || fs[0].Modul != "gesundheit" {
		t.Errorf("Angaben aus dem Eintrag fehlen: %+v", fs[0])
	}

	if !antwort.Gefunden || len(antwort.Quellen) != 1 || antwort.Quellen[0].Auszug != "Hämoglobin 14,2 g/dl" {
		t.Errorf("Antwort = %+v", antwort)
	}
}

func TestFragen_UnterSchwelleKeinModellaufruf(t *testing.T) {
	s := &suchAttrappe{treffer: []kern.Treffer{{EintragID: idLabor, Score: 0.77, Text: "Wetter?"}},
		eintraege: suchBestand().eintraege}
	a := &attrappe.Antworter{Text: "sollte nie kommen [1]"}

	antwort, err := NeuFragen(s, &attrappe.Einbetter{}, a).Ausfuehren(context.Background(), "Wie wird das Wetter?", "")

	if err != nil || antwort.Gefunden || antwort.Text != kern.FesterSatz || antwort.Quellen == nil || len(antwort.Quellen) != 0 {
		t.Errorf("Antwort = %+v, %v; erwartet fester Satz", antwort, err)
	}
	if a.Aufrufe() != 0 {
		t.Error("ohne Treffer über der Schwelle darf das Sprachmodell nicht gefragt werden")
	}
}

func TestFragen_NurVerwaisteTreffer(t *testing.T) {
	s := &suchAttrappe{treffer: []kern.Treffer{{EintragID: idWeg, Score: 0.95}}, eintraege: map[string]kern.EintragKurz{}}
	a := &attrappe.Antworter{}
	antwort, _ := NeuFragen(s, &attrappe.Einbetter{}, a).Ausfuehren(context.Background(), "x", "")
	if antwort.Gefunden || a.Aufrufe() != 0 {
		t.Errorf("verwaiste Abschnitte dürfen nicht ans Modell: %+v, %d Aufrufe", antwort, a.Aufrufe())
	}
}

func TestFragen_ModellOhneVerweis(t *testing.T) {
	// Zweite Schicht: Das Modell antwortet selbst mit dem festen Satz oder
	// ohne Quelle; beides wird zu "nichts gefunden".
	for _, text := range []string{kern.FesterSatz, "Der Wert war 14,2."} {
		a := &attrappe.Antworter{Text: text}
		antwort, err := NeuFragen(suchBestand(), &attrappe.Einbetter{}, a).Ausfuehren(context.Background(), "x", "")
		if err != nil || antwort.Gefunden || antwort.Text != kern.FesterSatz || len(antwort.Quellen) != 0 {
			t.Errorf("%q → %+v, %v", text, antwort, err)
		}
	}
}

func TestFragen_Fehler(t *testing.T) {
	faelle := []struct {
		name      string
		frage     string
		modul     string
		suche     *suchAttrappe
		einbetter *attrappe.Einbetter
		antworter *attrappe.Antworter
		fehler    error
	}{
		{"Frage leer", "  ", "", suchBestand(), &attrappe.Einbetter{}, &attrappe.Antworter{}, kern.ErrFrageFehlt},
		{"Frage zu lang", strings.Repeat("a", 501), "", suchBestand(), &attrappe.Einbetter{}, &attrappe.Antworter{}, kern.ErrFrageZuLang},
		{"Modul unbekannt", "x", "sport", suchBestand(), &attrappe.Einbetter{}, &attrappe.Antworter{}, kern.ErrUnbekanntesModul},
		{"Embedding nicht erreichbar", "x", "", suchBestand(), &attrappe.Einbetter{Err: modell.ErrNichtErreichbar}, &attrappe.Antworter{}, modell.ErrNichtErreichbar},
		{"Antworter nicht erreichbar", "x", "", suchBestand(), &attrappe.Einbetter{}, &attrappe.Antworter{Err: modell.ErrNichtErreichbar}, modell.ErrNichtErreichbar},
		{"Vektorsuche scheitert", "x", "", &suchAttrappe{err: errors.New("Index fehlt")}, &attrappe.Einbetter{}, &attrappe.Antworter{}, nil},
	}
	for _, f := range faelle {
		t.Run(f.name, func(t *testing.T) {
			_, err := NeuFragen(f.suche, f.einbetter, f.antworter).Ausfuehren(context.Background(), f.frage, f.modul)
			if err == nil {
				t.Fatal("erwartet Fehler")
			}
			if f.fehler != nil && !errors.Is(err, f.fehler) {
				t.Errorf("err = %v, erwartet %v", err, f.fehler)
			}
		})
	}
}

func TestFragen_FrageUndTexteNichtImLog(t *testing.T) {
	var log bytes.Buffer
	alt := slog.Default()
	slog.SetDefault(slog.New(slog.NewTextHandler(&log, nil)))
	t.Cleanup(func() { slog.SetDefault(alt) })

	a := &attrappe.Antworter{Text: "GEHEIME-ANTWORT [1]"}
	_, _ = NeuFragen(suchBestand(), &attrappe.Einbetter{}, a).Ausfuehren(context.Background(), "GEHEIME-FRAGE", "")

	for _, geheim := range []string{"GEHEIME-FRAGE", "GEHEIME-ANTWORT", "Hämoglobin"} {
		if strings.Contains(log.String(), geheim) {
			t.Errorf("%q im Log: %s", geheim, log.String())
		}
	}
	if !strings.Contains(log.String(), "besterScore=0.89") {
		t.Errorf("Score fehlt im Log (wichtig fürs Kalibrieren): %s", log.String())
	}
}
