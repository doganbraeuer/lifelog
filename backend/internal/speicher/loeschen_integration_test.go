package speicher

import (
	"context"
	"errors"
	"strings"
	"testing"

	"lifelog/backend/internal/dienst"
	"lifelog/backend/internal/kern"
)

// legeAn speichert einen vollständigen Eintrag wie der Upload: Datei,
// zwei Abschnitte, Eintrag zuletzt.
func legeAn(t *testing.T, s *Speicher, ctx context.Context, hash string) kern.Eintrag {
	t.Helper()
	e := testEintrag(NeueID(), NeueID(), hash)
	if err := s.DateiSpeichern(ctx, e.Datei.ID, e.Datei.Dateiname, e.Datei.MimeTyp, []byte("%PDF-1.7 synthetisch")); err != nil {
		t.Fatal(err)
	}
	if err := s.AbschnitteSpeichern(ctx, []kern.Abschnitt{
		{EintragID: e.ID, Modul: "gesundheit", Nr: 1, Text: "Teil 1", Embedding: []float32{0.1, 0.2}},
		{EintragID: e.ID, Modul: "gesundheit", Nr: 2, Text: "Teil 2", Embedding: []float32{0.3, 0.4}},
	}); err != nil {
		t.Fatal(err)
	}
	if err := s.EintragSpeichern(ctx, e); err != nil {
		t.Fatal(err)
	}
	return e
}

// pruefeDatei: da ist true, wenn sich die Datei öffnen lässt.
func pruefeDatei(t *testing.T, s *Speicher, ctx context.Context, dateiID string, da bool) {
	t.Helper()
	inhalt, _, err := s.DateiOeffnen(ctx, dateiID)
	switch {
	case err == nil:
		inhalt.Close()
		if !da {
			t.Errorf("Datei %s noch da", dateiID)
		}
	case errors.Is(err, kern.ErrOriginalFehlt):
		if da {
			t.Errorf("Datei %s fehlt", dateiID)
		}
	default:
		t.Fatal(err)
	}
}

// Läuft nur mit LIFELOG_MONGO_TEST=1 (siehe integration_test.go). Echter
// Dienst gegen echtes MongoDB (API.md 3.7).
func TestIntegration_EintragLoeschen(t *testing.T) {
	s, ctx := testSpeicher(t)
	l := dienst.NeuLoeschen(s)

	a := legeAn(t, s, ctx, strings.Repeat("a1", 32))
	b := legeAn(t, s, ctx, strings.Repeat("b2", 32))

	if err := l.Ausfuehren(ctx, a.ID); err != nil {
		t.Fatal(err)
	}

	// A ist vollständig weg.
	if _, da, err := s.EintragLesen(ctx, a.ID); err != nil || da {
		t.Errorf("Eintrag A: da %v, %v", da, err)
	}
	pruefeAnzahlAbschnitte(t, s, ctx, a.ID, 0)
	pruefeDatei(t, s, ctx, a.Datei.ID, false)
	// Der Hash ist frei: Dieselbe Datei ließe sich wieder hochladen.
	if _, da, err := s.EintragMitHash(ctx, a.Datei.Sha256); err != nil || da {
		t.Errorf("Hash von A noch gefunden: %v, %v", da, err)
	}

	// B ist unberührt.
	if _, da, err := s.EintragLesen(ctx, b.ID); err != nil || !da {
		t.Errorf("Eintrag B: da %v, %v", da, err)
	}
	pruefeAnzahlAbschnitte(t, s, ctx, b.ID, 2)
	pruefeDatei(t, s, ctx, b.Datei.ID, true)

	// Zweites Löschen: 404.
	if err := l.Ausfuehren(ctx, a.ID); !errors.Is(err, kern.ErrEintragNichtGefunden) {
		t.Errorf("zweites Löschen: %v, erwartet ErrEintragNichtGefunden", err)
	}
}

// Wie nach einem abgebrochenen Löschen: Datei und Abschnitte fehlen schon,
// der Eintrag ist noch da. Ein erneuter Aufruf räumt ohne Fehler auf.
func TestIntegration_LoeschenNachAbbruch(t *testing.T) {
	s, ctx := testSpeicher(t)
	e := legeAn(t, s, ctx, strings.Repeat("c3", 32))
	if err := s.AbschnitteLoeschen(ctx, e.ID); err != nil {
		t.Fatal(err)
	}
	if err := s.DateiLoeschen(ctx, e.Datei.ID); err != nil {
		t.Fatal(err)
	}

	if err := dienst.NeuLoeschen(s).Ausfuehren(ctx, e.ID); err != nil {
		t.Fatalf("Löschen mit fehlender Datei: %v", err)
	}
	if _, da, err := s.EintragLesen(ctx, e.ID); err != nil || da {
		t.Errorf("Eintrag: da %v, %v", da, err)
	}
}
