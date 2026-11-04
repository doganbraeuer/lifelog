package dienst

import (
	"context"
	"log/slog"
	"time"

	"lifelog/backend/internal/kern"
)

// KorrekturSpeicher ist, was das Korrigieren von der Datenbank braucht.
// Umgesetzt von speicher.Speicher.
type KorrekturSpeicher interface {
	EintragLesen(ctx context.Context, id string) (e kern.Eintrag, gefunden bool, err error)
	KorrekturSpeichern(ctx context.Context, e kern.Eintrag) (gefunden bool, err error)
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
//  2. kern.Korrigiere: alles prüfen, dann Regeln 1–4 anwenden.
//  3. Hat sich nichts geändert: nichts schreiben, Eintrag unverändert
//     zurückgeben (API.md 3.5).
//  4. Sonst die korrigierbaren Felder speichern.
//
// Ohne Sperre: Zwei gleichzeitige Korrekturen am selben Eintrag können sich
// überschreiben (API.md 4, Gleichzeitige Korrekturen).
//
// Fehler: kern.ErrUngueltigeID, kern.ErrEintragNichtGefunden, die Fehler
// aus kern.Korrigiere (*kern.KorrekturFehler) oder ein interner Fehler.
func (k *Korrigieren) Ausfuehren(ctx context.Context, id string, a kern.Aenderung) (kern.Eintrag, error) {
	if err := kern.PruefeID(id); err != nil {
		return kern.Eintrag{}, err
	}
	e, gefunden, err := k.speicher.EintragLesen(ctx, id)
	if err != nil {
		return kern.Eintrag{}, err
	}
	if !gefunden {
		return kern.Eintrag{}, kern.ErrEintragNichtGefunden
	}

	// Millisekunden, weil MongoDB Zeitpunkte so genau speichert: Antwort und
	// spätere Abfrage zeigen dann dasselbe (wie beim Upload).
	jetzt := k.jetzt().UTC().Truncate(time.Millisecond)
	neu, geaendert, err := kern.Korrigiere(e, a, jetzt)
	if err != nil {
		return kern.Eintrag{}, err
	}
	if len(geaendert) == 0 {
		slog.Info("Korrektur ohne Änderung", "eintrag", id)
		return neu, nil
	}

	gefunden, err = k.speicher.KorrekturSpeichern(ctx, neu)
	if err != nil {
		return kern.Eintrag{}, err
	}
	if !gefunden {
		// Zwischen Lesen und Schreiben gelöscht.
		return kern.Eintrag{}, kern.ErrEintragNichtGefunden
	}
	// Nur Feldnamen, nie Werte.
	slog.Info("Korrektur gespeichert", "eintrag", id, "felder", geaendert)
	return neu, nil
}
