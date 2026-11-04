package dienst

import (
	"bytes"
	"context"
	"errors"
	"log/slog"
	"strings"
	"testing"
	"time"

	"lifelog/backend/internal/kern"
)

// korrekturAttrappe hält einen Eintrag im Speicher und merkt sich, was
// geschrieben wurde.
type korrekturAttrappe struct {
	eintrag      *kern.Eintrag // nil: Eintrag gibt es nicht
	lesenErr     error
	speichernErr error
	weg          bool // Eintrag verschwindet zwischen Lesen und Schreiben
	gelesen      int
	gespeichert  []kern.Eintrag
}

func (s *korrekturAttrappe) EintragLesen(_ context.Context, id string) (kern.Eintrag, bool, error) {
	s.gelesen++
	if s.lesenErr != nil {
		return kern.Eintrag{}, false, s.lesenErr
	}
	if s.eintrag == nil || s.eintrag.ID != id {
		return kern.Eintrag{}, false, nil
	}
	return *s.eintrag, true, nil
}

func (s *korrekturAttrappe) KorrekturSpeichern(_ context.Context, e kern.Eintrag) (bool, error) {
	if s.speichernErr != nil {
		return false, s.speichernErr
	}
	if s.weg {
		return false, nil
	}
	s.gespeichert = append(s.gespeichert, e)
	return true, nil
}

func zd(s string) *string { return &s }

func korrekturBestand() *korrekturAttrappe {
	return &korrekturAttrappe{eintrag: &kern.Eintrag{
		ID: idLabor, Modul: "gesundheit", Typ: "laborbefund",
		Dokumentdatum: zd("2026-10-14"), Quelle: zd("Labor Muster GmbH"),
		Infos:       []kern.Info{{ID: "i2", Name: "Leukozyten", Wert: "6,3", Einheit: zd("G/l")}},
		Korrekturen: []kern.Korrektur{},
	}}
}

func neuKorrigierenFest(s KorrekturSpeicher) *Korrigieren {
	k := NeuKorrigieren(s)
	// Mit Nanosekunden und in einer anderen Zeitzone, um das Kürzen auf
	// Millisekunden und die Umrechnung nach UTC zu prüfen.
	k.jetzt = func() time.Time {
		return time.Date(2026, 11, 4, 20, 5, 31, 123_456_789, time.FixedZone("MEZ", 3600))
	}
	return k
}

func TestKorrigieren_Erfolg(t *testing.T) {
	s := korrekturBestand()
	e, err := neuKorrigierenFest(s).Ausfuehren(context.Background(), idLabor,
		kern.Aenderung{Infos: []kern.InfoAenderung{{ID: "i2", Wert: zd("6,8")}}})
	if err != nil {
		t.Fatal(err)
	}
	if len(s.gespeichert) != 1 {
		t.Fatalf("%d mal gespeichert", len(s.gespeichert))
	}
	if e.Infos[0].Wert != "6,8" || s.gespeichert[0].Infos[0].Wert != "6,8" {
		t.Errorf("Antwort %q, gespeichert %q", e.Infos[0].Wert, s.gespeichert[0].Infos[0].Wert)
	}
	soll := time.Date(2026, 11, 4, 19, 5, 31, 123_000_000, time.UTC)
	if k := e.Korrekturen; len(k) != 1 || !k[0].Zeitpunkt.Equal(soll) || k[0].Zeitpunkt.Location() != time.UTC {
		t.Errorf("Korrekturen = %+v, erwartet Zeitpunkt %v", k, soll)
	}
}

func TestKorrigieren_OhneAenderungKeinSchreiben(t *testing.T) {
	s := korrekturBestand()
	e, err := neuKorrigierenFest(s).Ausfuehren(context.Background(), idLabor,
		kern.Aenderung{Quelle: kern.Feld{Gesetzt: true, Wert: zd("Labor Muster GmbH")}})
	if err != nil {
		t.Fatal(err)
	}
	if len(s.gespeichert) != 0 {
		t.Error("ohne Änderung gespeichert")
	}
	if e.ID != idLabor || len(e.Korrekturen) != 0 {
		t.Errorf("Antwort = %+v, erwartet den unveränderten Eintrag", e)
	}
}

func TestKorrigieren_Fehler(t *testing.T) {
	intern := errors.New("Datenbank weg")
	gueltig := kern.Aenderung{Typ: zd("arztbrief")}
	faelle := []struct {
		name    string
		s       *korrekturAttrappe
		id      string
		a       kern.Aenderung
		err     error
		gelesen int
	}{
		{"ungültige ID", korrekturBestand(), "abc", gueltig, kern.ErrUngueltigeID, 0},
		{"nicht gefunden", korrekturBestand(), idBrief, gueltig, kern.ErrEintragNichtGefunden, 1},
		{"lesen scheitert", &korrekturAttrappe{lesenErr: intern}, idLabor, gueltig, intern, 1},
		{"korrektur ungültig", korrekturBestand(), idLabor, kern.Aenderung{Typ: zd("befund")}, kern.ErrKorrekturUngueltig, 1},
		{"info unbekannt", korrekturBestand(), idLabor,
			kern.Aenderung{Infos: []kern.InfoAenderung{{ID: "i9", Wert: zd("1")}}}, kern.ErrInfoNichtGefunden, 1},
		{"speichern scheitert", func() *korrekturAttrappe { s := korrekturBestand(); s.speichernErr = intern; return s }(),
			idLabor, gueltig, intern, 1},
		{"zwischendurch gelöscht", func() *korrekturAttrappe { s := korrekturBestand(); s.weg = true; return s }(),
			idLabor, gueltig, kern.ErrEintragNichtGefunden, 1},
	}
	for _, f := range faelle {
		t.Run(f.name, func(t *testing.T) {
			_, err := neuKorrigierenFest(f.s).Ausfuehren(context.Background(), f.id, f.a)
			if !errors.Is(err, f.err) {
				t.Errorf("err = %v, erwartet %v", err, f.err)
			}
			if f.s.gelesen != f.gelesen {
				t.Errorf("%d mal gelesen, erwartet %d", f.s.gelesen, f.gelesen)
			}
			if len(f.s.gespeichert) != 0 {
				t.Error("trotz Fehler gespeichert")
			}
		})
	}
}

func TestKorrigieren_LogOhneWerte(t *testing.T) {
	var log bytes.Buffer
	alt := slog.Default()
	slog.SetDefault(slog.New(slog.NewTextHandler(&log, nil)))
	t.Cleanup(func() { slog.SetDefault(alt) })

	_, err := neuKorrigierenFest(korrekturBestand()).Ausfuehren(context.Background(), idLabor, kern.Aenderung{
		Quelle: kern.Feld{Gesetzt: true, Wert: zd("Dr. Geheim")},
		Infos:  []kern.InfoAenderung{{ID: "i2", Wert: zd("7,7")}},
	})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(log.String(), "quelle") || !strings.Contains(log.String(), "infos.i2.wert") {
		t.Errorf("Feldnamen fehlen im Log: %s", log.String())
	}
	for _, wert := range []string{"Dr. Geheim", "7,7", "6,3", "Labor Muster"} {
		if strings.Contains(log.String(), wert) {
			t.Errorf("Wert %q im Log: %s", wert, log.String())
		}
	}
}
