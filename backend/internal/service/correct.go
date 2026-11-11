package service

import (
	"context"
	"log/slog"
	"time"

	"lifelog/backend/internal/core"
)

// KorrekturSpeicher ist, was das Korrigieren von der Datenbank braucht.
// Umgesetzt von speicher.Speicher.
type KorrekturSpeicher interface {
	GetEntry(ctx context.Context, id string) (e core.Entry, gefunden bool, err error)
	SaveCorrection(ctx context.Context, e core.Entry) (gefunden bool, err error)
}

// Korrigieren korrigiert erkannte Werte eines Eintrags (docs/API.md 3.5).
type Korrigieren struct {
	speicher KorrekturSpeicher
	jetzt    func() time.Time
}

func NeuKorrigieren(s KorrekturSpeicher) *Korrigieren {
	return &Korrigieren{speicher: s, jetzt: time.Now}
}

// Ausfuehren:
//
//  1. ID prüfen, Eintrag laden.
//  2. core.Korrigiere: alles prüfen, dann Regeln 1–4 anwenden.
//  3. Hat sich nichts geändert: nichts schreiben, Eintrag unverändert
//     zurückgeben (API.md 3.5).
//  4. Sonst die korrigierbaren Felder speichern.
//
// Ohne Sperre: Zwei gleichzeitige Korrekturen am selben Eintrag können sich
// überschreiben (API.md 4, Gleichzeitige Korrekturen).
//
// Fehler: core.ErrUngueltigeID, core.ErrEintragNichtGefunden, die Fehler
// aus core.Korrigiere (*core.KorrekturFehler) oder ein interner Fehler.
func (k *Korrigieren) Ausfuehren(ctx context.Context, id string, a core.Change) (core.Entry, error) {
	if err := core.CheckID(id); err != nil {
		return core.Entry{}, err
	}
	e, gefunden, err := k.speicher.GetEntry(ctx, id)
	if err != nil {
		return core.Entry{}, err
	}
	if !gefunden {
		return core.Entry{}, core.ErrEntryNotFound
	}

	// Millisekunden, weil MongoDB Zeitpunkte so genau speichert: Antwort und
	// spätere Abfrage zeigen dann dasselbe (wie beim Upload).
	jetzt := k.jetzt().UTC().Truncate(time.Millisecond)
	neu, geaendert, err := core.Correct(e, a, jetzt)
	if err != nil {
		return core.Entry{}, err
	}
	if len(geaendert) == 0 {
		slog.Info("Korrektur ohne Änderung", "eintrag", id)
		return neu, nil
	}

	gefunden, err = k.speicher.SaveCorrection(ctx, neu)
	if err != nil {
		return core.Entry{}, err
	}
	if !gefunden {
		// Zwischen Lesen und Schreiben gelöscht.
		return core.Entry{}, core.ErrEntryNotFound
	}
	// Nur Feldnamen, nie Werte.
	slog.Info("Korrektur gespeichert", "eintrag", id, "felder", geaendert)
	return neu, nil
}
