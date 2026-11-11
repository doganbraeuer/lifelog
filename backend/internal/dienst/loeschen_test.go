package dienst

import (
	"bytes"
	"context"
	"errors"
	"log/slog"
	"reflect"
	"strings"
	"testing"

	"lifelog/backend/internal/core"
)

const dateiWegwerf = "bbbbbbbbbbbbbbbbbbbbbbbb"

// loeschAttrappe hält einen Eintrag mit Abschnitten und Datei, protokolliert
// jeden Aufruf mit Argument und kann in einem Schritt scheitern.
type loeschAttrappe struct {
	eintrag    *core.Entry // nil: Eintrag gibt es nicht (mehr)
	abschnitte bool
	datei      bool

	scheitertBei string // Name der Methode, die einen Fehler liefert
	fehler       error  // dieser Fehler, sonst errDB
	nachLesen    func() // läuft nach EintragLesen, z. B. um ctx abzubrechen
	ctxFehler    []error
	protokoll    []string
}

var errDB = errors.New("Datenbank weg")

func (s *loeschAttrappe) schritt(ctx context.Context, name, arg string) error {
	s.protokoll = append(s.protokoll, name+"("+arg+")")
	s.ctxFehler = append(s.ctxFehler, ctx.Err())
	if s.scheitertBei == name {
		if s.fehler != nil {
			return s.fehler
		}
		return errDB
	}
	return nil
}

func (s *loeschAttrappe) GetEntry(ctx context.Context, id string) (core.Entry, bool, error) {
	if err := s.schritt(ctx, "EintragLesen", id); err != nil {
		return core.Entry{}, false, err
	}
	if s.eintrag == nil || s.eintrag.ID != id {
		return core.Entry{}, false, nil
	}
	e := *s.eintrag
	if s.nachLesen != nil {
		s.nachLesen()
	}
	return e, true, nil
}

func (s *loeschAttrappe) DeleteChunks(ctx context.Context, id string) error {
	if err := s.schritt(ctx, "AbschnitteLoeschen", id); err != nil {
		return err
	}
	s.abschnitte = false
	return nil
}

// DateiLoeschen verhält sich wie speicher: Fehlt die Datei, kein Fehler.
func (s *loeschAttrappe) DeleteFile(ctx context.Context, id string) error {
	if err := s.schritt(ctx, "DateiLoeschen", id); err != nil {
		return err
	}
	s.datei = false
	return nil
}

func (s *loeschAttrappe) DeleteEntry(ctx context.Context, id string) (bool, error) {
	if err := s.schritt(ctx, "EintragLoeschen", id); err != nil {
		return false, err
	}
	if s.eintrag == nil || s.eintrag.ID != id {
		return false, nil
	}
	s.eintrag = nil
	return true, nil
}

func loeschBestand() *loeschAttrappe {
	return &loeschAttrappe{
		eintrag: &core.Entry{
			ID: idLabor, Module: "gesundheit", Type: "laborbefund",
			Issuer: zd("Labor Geheim GmbH"), FullText: "Hämoglobin 14,2 g/dl",
			Facts: []core.Fact{{ID: "i1", Name: "Hämoglobin", Value: "14,2", Unit: zd("g/dl")}},
			File:  core.File{ID: dateiWegwerf, FileName: "befund_geheim.pdf", MimeType: "application/pdf"},
		},
		abschnitte: true,
		datei:      true,
	}
}

var alleSchritte = []string{
	"EintragLesen(" + idLabor + ")",
	"AbschnitteLoeschen(" + idLabor + ")",
	"DateiLoeschen(" + dateiWegwerf + ")",
	"EintragLoeschen(" + idLabor + ")",
}

func TestLoeschen_ErfolgInDieserReihenfolge(t *testing.T) {
	s := loeschBestand()
	if err := NeuLoeschen(s).Ausfuehren(context.Background(), idLabor); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(s.protokoll, alleSchritte) {
		t.Errorf("Aufrufe\n%v\nerwartet\n%v", s.protokoll, alleSchritte)
	}
	if s.eintrag != nil || s.abschnitte || s.datei {
		t.Errorf("nicht alles gelöscht: eintrag %v, abschnitte %v, datei %v", s.eintrag != nil, s.abschnitte, s.datei)
	}
}

// Scheitert ein Schritt, kommen die späteren nicht mehr dran, und der
// Eintrag bleibt sichtbar (API.md 3.7). Ein zweiter Aufruf räumt auf, ein
// dritter liefert 404.
func TestLoeschen_ScheitertInEinemSchritt(t *testing.T) {
	for i, schritt := range []string{"EintragLesen", "AbschnitteLoeschen", "DateiLoeschen", "EintragLoeschen"} {
		t.Run(schritt, func(t *testing.T) {
			s := loeschBestand()
			s.scheitertBei = schritt
			l := NeuLoeschen(s)

			err := l.Ausfuehren(context.Background(), idLabor)
			if !errors.Is(err, errDB) {
				t.Fatalf("Fehler %v, erwartet %v", err, errDB)
			}
			if want := alleSchritte[:i+1]; !reflect.DeepEqual(s.protokoll, want) {
				t.Errorf("Aufrufe %v, erwartet %v", s.protokoll, want)
			}
			if s.eintrag == nil {
				t.Error("Eintrag gelöscht, obwohl ein früherer Schritt gescheitert ist")
			}

			// Wiederholen räumt den Rest auf.
			s.scheitertBei, s.protokoll = "", nil
			if err := l.Ausfuehren(context.Background(), idLabor); err != nil {
				t.Fatalf("zweiter Aufruf: %v", err)
			}
			if s.eintrag != nil || s.abschnitte || s.datei {
				t.Error("zweiter Aufruf hat nicht alles gelöscht")
			}
			if err := l.Ausfuehren(context.Background(), idLabor); !errors.Is(err, core.ErrEntryNotFound) {
				t.Errorf("dritter Aufruf: %v, erwartet ErrEintragNichtGefunden", err)
			}
		})
	}
}

func TestLoeschen_ZweimalLiefertNichtGefunden(t *testing.T) {
	s := loeschBestand()
	l := NeuLoeschen(s)
	if err := l.Ausfuehren(context.Background(), idLabor); err != nil {
		t.Fatal(err)
	}
	s.protokoll = nil
	if err := l.Ausfuehren(context.Background(), idLabor); !errors.Is(err, core.ErrEntryNotFound) {
		t.Fatalf("zweiter Aufruf: %v", err)
	}
	// Nur gelesen, nichts gelöscht.
	if want := alleSchritte[:1]; !reflect.DeepEqual(s.protokoll, want) {
		t.Errorf("Aufrufe %v, erwartet %v", s.protokoll, want)
	}
}

func TestLoeschen_UngueltigeIDOhneSpeicher(t *testing.T) {
	s := loeschBestand()
	for _, id := range []string{"", "abc", strings.Repeat("z", 24)} {
		if err := NeuLoeschen(s).Ausfuehren(context.Background(), id); !errors.Is(err, core.ErrInvalidID) {
			t.Errorf("%q: %v, erwartet ErrUngueltigeID", id, err)
		}
	}
	if len(s.protokoll) != 0 {
		t.Errorf("Speicher aufgerufen: %v", s.protokoll)
	}
}

// Datei schon weg (z. B. nach einem abgebrochenen Löschen): kein Fehler.
func TestLoeschen_DateiFehltSchon(t *testing.T) {
	s := loeschBestand()
	s.datei, s.abschnitte = false, false
	if err := NeuLoeschen(s).Ausfuehren(context.Background(), idLabor); err != nil {
		t.Fatal(err)
	}
	if s.eintrag != nil {
		t.Error("Eintrag nicht gelöscht")
	}
}

// Ein gleichzeitiger Aufruf löscht den Eintrag zwischen Lesen und Löschen.
func TestLoeschen_GleichzeitigGeloescht(t *testing.T) {
	s := loeschBestand()
	s.nachLesen = func() { s.eintrag = nil }
	if err := NeuLoeschen(s).Ausfuehren(context.Background(), idLabor); !errors.Is(err, core.ErrEntryNotFound) {
		t.Errorf("%v, erwartet ErrEintragNichtGefunden", err)
	}
	if !reflect.DeepEqual(s.protokoll, alleSchritte) {
		t.Errorf("Aufrufe %v, erwartet alle vier", s.protokoll)
	}
}

// Bricht die App nach dem Lesen ab, laufen die Löschschritte trotzdem mit
// einem gültigen Kontext zu Ende.
func TestLoeschen_AbbruchNachBeginnLaeuftZuEnde(t *testing.T) {
	ctx, abbrechen := context.WithCancel(context.Background())
	s := loeschBestand()
	s.nachLesen = abbrechen

	if err := NeuLoeschen(s).Ausfuehren(ctx, idLabor); err != nil {
		t.Fatal(err)
	}
	if s.eintrag != nil {
		t.Fatal("Eintrag nicht gelöscht")
	}
	for i, err := range s.ctxFehler[1:] {
		if err != nil {
			t.Errorf("%s mit abgebrochenem Kontext: %v", s.protokoll[i+1], err)
		}
	}
}

// Schon vor Beginn abgebrochen: Das Lesen scheitert, gelöscht wird nichts.
func TestLoeschen_AbbruchVorBeginn(t *testing.T) {
	s := loeschBestand()
	s.scheitertBei, s.fehler = "EintragLesen", context.Canceled

	err := NeuLoeschen(s).Ausfuehren(context.Background(), idLabor)
	if !errors.Is(err, context.Canceled) || s.eintrag == nil {
		t.Errorf("Fehler %v, eintrag noch da: %v", err, s.eintrag != nil)
	}
}

func TestLoeschen_LogNurID(t *testing.T) {
	var log bytes.Buffer
	alt := slog.Default()
	slog.SetDefault(slog.New(slog.NewTextHandler(&log, nil)))
	t.Cleanup(func() { slog.SetDefault(alt) })

	if err := NeuLoeschen(loeschBestand()).Ausfuehren(context.Background(), idLabor); err != nil {
		t.Fatal(err)
	}
	text := log.String()
	if !strings.Contains(text, "Eintrag gelöscht") || !strings.Contains(text, idLabor) {
		t.Errorf("Log ohne Meldung oder ID: %s", text)
	}
	for _, inhalt := range []string{"Geheim", "geheim", "Hämoglobin", "14,2", "laborbefund", dateiWegwerf} {
		if strings.Contains(text, inhalt) {
			t.Errorf("Log enthält %q: %s", inhalt, text)
		}
	}
}
