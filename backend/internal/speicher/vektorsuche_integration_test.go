package speicher

import (
	"context"
	"math"
	"strings"
	"testing"
	"time"

	"lifelog/backend/internal/core"
	"lifelog/backend/internal/modell"
)

// einheitsvektor hat eine 1 an Stelle i, sonst 0. Zwei verschiedene
// Einheitsvektoren haben cos 0, also Atlas-Score 0,5.
func einheitsvektor(i int) []float32 {
	v := make([]float32, modell.EmbeddingDimensionen)
	v[i] = 1
	return v
}

// Läuft in der Testdatenbank (siehe testSpeicher). Atlas baut den Index im
// Hintergrund; der Test wartet darauf, das kann eine Minute dauern.
func TestIntegration_Vektorsuche(t *testing.T) {
	s, _ := testSpeicher(t)
	ctx, abbrechen := context.WithTimeout(context.Background(), 4*time.Minute)
	defer abbrechen()

	eintragA, eintragB, eintragSport := NeueID(), NeueID(), NeueID()
	abschnitte := []core.Chunk{
		{EntryID: eintragA, Module: "gesundheit", Position: 1, Text: "Hämoglobin 14,2 g/dl", Embedding: einheitsvektor(0)},
		{EntryID: eintragB, Module: "gesundheit", Position: 1, Text: "Blutdruck 128/82", Embedding: einheitsvektor(1)},
		// Gleicher Vektor wie A, aber anderes Modul: Der Filter muss ihn ausschließen.
		{EntryID: eintragSport, Module: "sport", Position: 1, Text: "Lauf 10 km", Embedding: einheitsvektor(0)},
	}
	if err := s.AbschnitteSpeichern(ctx, abschnitte); err != nil {
		t.Fatal(err)
	}

	start := time.Now()
	status, err := s.VektorindexAnlegen(ctx)
	if err != nil {
		t.Fatalf("Anlegen per Code gescheitert (dann Weg B in der Atlas-Oberfläche): %v", err)
	}
	t.Logf("Vektorindex: %s", status)
	if status2, err := s.VektorindexAnlegen(ctx); err != nil || status2 == "ANGELEGT" {
		t.Errorf("zweites Anlegen muss den vorhandenen Index erkennen: %q, %v", status2, err)
	}

	// Warten, bis der Index fertig ist und die Abschnitte gefunden werden.
	var treffer []core.Hit
	for {
		status, _, err := s.VektorindexStatus(ctx)
		if err != nil {
			t.Fatal(err)
		}
		if status == "READY" {
			treffer, err = s.AehnlicheAbschnitte(ctx, einheitsvektor(0), "gesundheit", core.MaxHits)
			if err != nil {
				t.Fatalf("Vektorsuche: %v", err)
			}
			if len(treffer) >= 2 {
				break
			}
		}
		if ctx.Err() != nil {
			t.Fatalf("Index nach %v nicht bereit (Status %q, %d Treffer)", time.Since(start).Round(time.Second), status, len(treffer))
		}
		time.Sleep(3 * time.Second)
	}
	t.Logf("Index bereit und durchsuchbar nach %v", time.Since(start).Round(time.Second))

	for _, tr := range treffer {
		t.Logf("  %s Score %.4f %q", tr.EntryID[18:], tr.Score, tr.Text)
		if tr.EntryID == eintragSport {
			t.Error("Abschnitt aus Modul sport gefunden: Filter wirkt nicht")
		}
	}
	if treffer[0].EntryID != eintragA || treffer[0].Position != 1 || treffer[0].Text != "Hämoglobin 14,2 g/dl" {
		t.Errorf("bester Treffer = %+v, erwartet Abschnitt von A", treffer[0])
	}
	// Gleicher Vektor: cos 1 → Score 1. Senkrecht: cos 0 → Score 0,5.
	// Das bestätigt die Umrechnung (1 + cos) / 2, auf der core.MinScore beruht.
	if math.Abs(treffer[0].Score-1) > 1e-3 || math.Abs(treffer[1].Score-0.5) > 1e-3 {
		t.Errorf("Scores = %.4f / %.4f, erwartet 1 / 0,5", treffer[0].Score, treffer[1].Score)
	}

	// Im Modul sport gibt es nur den einen Abschnitt.
	sport, err := s.AehnlicheAbschnitte(ctx, einheitsvektor(0), "sport", core.MaxHits)
	if err != nil || len(sport) != 1 || sport[0].EntryID != eintragSport {
		t.Errorf("Modul sport: %+v, %v", sport, err)
	}
}

func TestIntegration_EintraegeKurzNachIDs(t *testing.T) {
	s, ctx := testSpeicher(t)
	a := testEintrag(NeueID(), NeueID(), strings.Repeat("12", 32))
	if err := s.EintragSpeichern(ctx, a); err != nil {
		t.Fatal(err)
	}
	fehlt := NeueID()

	m, err := s.EintraegeKurzNachIDs(ctx, []string{a.ID, fehlt})
	if err != nil {
		t.Fatal(err)
	}
	if len(m) != 1 || m[a.ID].Type != "laborbefund" || m[a.ID].Issuer == nil {
		t.Errorf("Map = %+v", m)
	}
	if _, ok := m[fehlt]; ok {
		t.Error("nicht vorhandener Eintrag in der Map")
	}
	if leer, err := s.EintraegeKurzNachIDs(ctx, nil); err != nil || leer == nil || len(leer) != 0 {
		t.Errorf("leere Liste: %#v, %v", leer, err)
	}
}
