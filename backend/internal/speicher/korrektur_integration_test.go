package speicher

import (
	"reflect"
	"strings"
	"testing"
	"time"

	"lifelog/backend/internal/core"
)

// Läuft nur mit LIFELOG_MONGO_TEST=1 (siehe integration_test.go).
func TestIntegration_KorrekturSpeichern(t *testing.T) {
	s, ctx := testSpeicher(t)

	e := testEintrag(NeueID(), NeueID(), strings.Repeat("cd", 32))
	e.Infos = append(e.Infos, core.Info{ID: "i2", Name: "Befundart", Wert: "Blutbild klein"})
	if err := s.EintragSpeichern(ctx, e); err != nil {
		t.Fatal(err)
	}

	// Alle Arten von Korrektur auf einmal, auch null-Werte: Datum und
	// Einheit entfernt, Einheit neu, Typ und Wert geändert.
	jetzt := time.Date(2026, 11, 4, 19, 5, 31, 123_000_000, time.UTC) // Millisekunden wie im Dienst
	q := "Labor B"
	neu, geaendert, err := core.Korrigiere(e, core.Aenderung{
		Typ:           &[]string{"arztbrief"}[0],
		Dokumentdatum: core.Feld{Gesetzt: true},
		Quelle:        core.Feld{Gesetzt: true, Wert: &q},
		Infos: []core.InfoAenderung{
			{ID: "i1", Wert: &[]string{"14,0"}[0], Einheit: core.Feld{Gesetzt: true}},
			{ID: "i2", Einheit: core.Feld{Gesetzt: true, Wert: &[]string{"Stück"}[0]}},
		},
	}, jetzt)
	if err != nil || len(geaendert) != 6 {
		t.Fatalf("Korrigiere: %v, geaendert %v", err, geaendert)
	}

	gefunden, err := s.KorrekturSpeichern(ctx, neu)
	if err != nil || !gefunden {
		t.Fatalf("KorrekturSpeichern: gefunden %v, %v", gefunden, err)
	}

	gelesen, _, err := s.EintragLesen(ctx, e.ID)
	if err != nil {
		t.Fatal(err)
	}
	// Hin und zurück durch MongoDB: genau der korrigierte Eintrag, inklusive
	// null-Werten und Zeitpunkt in UTC.
	if !reflect.DeepEqual(gelesen, neu) {
		t.Errorf("gelesen\n%+v\nerwartet\n%+v", gelesen, neu)
	}
	// Nicht korrigierbare Felder sind unverändert.
	if gelesen.Volltext != e.Volltext || gelesen.Datei != e.Datei || !gelesen.Importdatum.Equal(e.Importdatum) || gelesen.Modul != e.Modul {
		t.Errorf("nicht korrigierbare Felder geändert: %+v", gelesen)
	}

	// Gibt es den Eintrag nicht, meldet KorrekturSpeichern das, statt einen
	// neuen anzulegen.
	fremd := neu
	fremd.ID = NeueID()
	gefunden, err = s.KorrekturSpeichern(ctx, fremd)
	if err != nil || gefunden {
		t.Errorf("unbekannter Eintrag: gefunden %v, %v", gefunden, err)
	}
	if _, da, _ := s.EintragLesen(ctx, fremd.ID); da {
		t.Error("KorrekturSpeichern hat einen neuen Eintrag angelegt")
	}
}
