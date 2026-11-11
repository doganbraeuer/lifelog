package dienst

import (
	"bytes"
	"context"
	"errors"
	"log/slog"
	"strings"
	"testing"

	"lifelog/backend/internal/core"
	"lifelog/backend/internal/llm"
	"lifelog/backend/internal/llm/fake"
)

type suchAttrappe struct {
	treffer      []core.Hit
	eintraege    map[string]core.EntrySummary
	err          error
	gefragtModul string
	gefragtK     int
}

func (s *suchAttrappe) AehnlicheAbschnitte(_ context.Context, _ []float32, modul string, k int) ([]core.Hit, error) {
	s.gefragtModul, s.gefragtK = modul, k
	return s.treffer, s.err
}

func (s *suchAttrappe) EintraegeKurzNachIDs(_ context.Context, ids []string) (map[string]core.EntrySummary, error) {
	m := map[string]core.EntrySummary{}
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
		treffer: []core.Hit{
			{EntryID: idLabor, Position: 3, Text: "Hämoglobin 14,2 g/dl", Score: 0.89},
			{EntryID: idWeg, Position: 1, Text: "verwaist", Score: 0.86},
			{EntryID: idBrief, Position: 1, Text: "Blutdruck 128/82", Score: core.MinScore},              // genau auf der Schwelle
			{EntryID: idBrief, Position: 2, Text: "Kontrolle in 6 Monaten", Score: core.MinScore - 0.01}, // darunter
		},
		eintraege: map[string]core.EntrySummary{
			idLabor: {ID: idLabor, Module: "gesundheit", Type: "laborbefund", DocumentDate: zeiger("2026-10-14"), Issuer: zeiger("Labor Muster GmbH")},
			idBrief: {ID: idBrief, Module: "gesundheit", Type: "arztbrief", DocumentDate: zeiger("2026-09-02"), Issuer: zeiger("Dr. Erika Beispiel")},
		},
	}
}

func TestFragen_Treffer(t *testing.T) {
	s := suchBestand()
	a := &fake.Answerer{Text: "Im Laborbefund steht Hämoglobin 14,2 g/dl [1]."}
	f := NeuFragen(s, &fake.Embedder{}, a)

	antwort, err := f.Ausfuehren(context.Background(), "  Wie hoch war mein Hb?  ", "")
	if err != nil {
		t.Fatal(err)
	}
	if s.gefragtModul != "gesundheit" || s.gefragtK != core.MaxHits {
		t.Errorf("Suche mit Modul %q, k %d", s.gefragtModul, s.gefragtK)
	}

	// Ans Modell gingen nur Treffer ≥ MinScore mit vorhandenem Eintrag, neu nummeriert.
	frage, fs := a.LastRequest()
	if frage != "Wie hoch war mein Hb?" {
		t.Errorf("Frage = %q, erwartet ohne Leerraum", frage)
	}
	if len(fs) != 2 || fs[0].Number != 1 || fs[0].EntryID != idLabor || fs[1].Number != 2 || fs[1].EntryID != idBrief {
		t.Fatalf("Fundstellen = %+v", fs)
	}
	if fs[0].Type != "laborbefund" || *fs[0].DocumentDate != "2026-10-14" || *fs[0].Issuer != "Labor Muster GmbH" || fs[0].Module != "gesundheit" {
		t.Errorf("Angaben aus dem Eintrag fehlen: %+v", fs[0])
	}

	if !antwort.Found || len(antwort.Sources) != 1 || antwort.Sources[0].Excerpt != "Hämoglobin 14,2 g/dl" {
		t.Errorf("Antwort = %+v", antwort)
	}
}

func TestFragen_UnterSchwelleKeinModellaufruf(t *testing.T) {
	s := &suchAttrappe{treffer: []core.Hit{{EntryID: idLabor, Score: 0.77, Text: "Wetter?"}},
		eintraege: suchBestand().eintraege}
	a := &fake.Answerer{Text: "sollte nie kommen [1]"}

	antwort, err := NeuFragen(s, &fake.Embedder{}, a).Ausfuehren(context.Background(), "Wie wird das Wetter?", "")

	if err != nil || antwort.Found || antwort.Text != core.NotFoundAnswer || antwort.Sources == nil || len(antwort.Sources) != 0 {
		t.Errorf("Antwort = %+v, %v; erwartet fester Satz", antwort, err)
	}
	if a.Calls() != 0 {
		t.Error("ohne Treffer über der Schwelle darf das Sprachmodell nicht gefragt werden")
	}
}

func TestFragen_NurVerwaisteTreffer(t *testing.T) {
	s := &suchAttrappe{treffer: []core.Hit{{EntryID: idWeg, Score: 0.95}}, eintraege: map[string]core.EntrySummary{}}
	a := &fake.Answerer{}
	antwort, _ := NeuFragen(s, &fake.Embedder{}, a).Ausfuehren(context.Background(), "x", "")
	if antwort.Found || a.Calls() != 0 {
		t.Errorf("verwaiste Abschnitte dürfen nicht ans Modell: %+v, %d Aufrufe", antwort, a.Calls())
	}
}

func TestFragen_ModellOhneVerweis(t *testing.T) {
	// Zweite Schicht: Das Modell antwortet selbst mit dem festen Satz oder
	// ohne Quelle; beides wird zu "nichts gefunden".
	for _, text := range []string{core.NotFoundAnswer, "Der Wert war 14,2."} {
		a := &fake.Answerer{Text: text}
		antwort, err := NeuFragen(suchBestand(), &fake.Embedder{}, a).Ausfuehren(context.Background(), "x", "")
		if err != nil || antwort.Found || antwort.Text != core.NotFoundAnswer || len(antwort.Sources) != 0 {
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
		einbetter *fake.Embedder
		antworter *fake.Answerer
		fehler    error
	}{
		{"Frage leer", "  ", "", suchBestand(), &fake.Embedder{}, &fake.Answerer{}, core.ErrQuestionMissing},
		{"Frage zu lang", strings.Repeat("a", 501), "", suchBestand(), &fake.Embedder{}, &fake.Answerer{}, core.ErrQuestionTooLong},
		{"Modul unbekannt", "x", "sport", suchBestand(), &fake.Embedder{}, &fake.Answerer{}, core.ErrUnknownModule},
		{"Embedding nicht erreichbar", "x", "", suchBestand(), &fake.Embedder{Err: llm.ErrUnavailable}, &fake.Answerer{}, llm.ErrUnavailable},
		{"Antworter nicht erreichbar", "x", "", suchBestand(), &fake.Embedder{}, &fake.Answerer{Err: llm.ErrUnavailable}, llm.ErrUnavailable},
		{"Vektorsuche scheitert", "x", "", &suchAttrappe{err: errors.New("Index fehlt")}, &fake.Embedder{}, &fake.Answerer{}, nil},
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

	a := &fake.Answerer{Text: "GEHEIME-ANTWORT [1]"}
	_, _ = NeuFragen(suchBestand(), &fake.Embedder{}, a).Ausfuehren(context.Background(), "GEHEIME-FRAGE", "")

	for _, geheim := range []string{"GEHEIME-FRAGE", "GEHEIME-ANTWORT", "Hämoglobin"} {
		if strings.Contains(log.String(), geheim) {
			t.Errorf("%q im Log: %s", geheim, log.String())
		}
	}
	if !strings.Contains(log.String(), "besterScore=0.89") {
		t.Errorf("Score fehlt im Log (wichtig fürs Kalibrieren): %s", log.String())
	}
}
