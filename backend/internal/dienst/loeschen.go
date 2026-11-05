package dienst

import (
	"context"
	"log/slog"
	"time"

	"lifelog/backend/internal/kern"
)

// LoeschSpeicher ist, was das Löschen von der Datenbank braucht. Umgesetzt
// von speicher.Speicher. DateiLoeschen und AbschnitteLoeschen dürfen keinen
// Fehler liefern, wenn es nichts (mehr) zu löschen gibt.
type LoeschSpeicher interface {
	EintragLesen(ctx context.Context, id string) (e kern.Eintrag, gefunden bool, err error)
	AbschnitteLoeschen(ctx context.Context, eintragID string) error
	DateiLoeschen(ctx context.Context, id string) error
	EintragLoeschen(ctx context.Context, id string) (gefunden bool, err error)
}

// loeschTimeout: Zeit für die drei Löschschritte. Sie laufen ohne den
// Kontext der Anfrage weiter (siehe Ausfuehren), brauchen also eine eigene
// Grenze.
const loeschTimeout = 30 * time.Second

// Loeschen löscht einen Eintrag mit Abschnitten und Originaldatei
// (docs/API.md 3.7).
type Loeschen struct {
	speicher LoeschSpeicher
}

func NeuLoeschen(s LoeschSpeicher) *Loeschen {
	return &Loeschen{speicher: s}
}

// Ausfuehren:
//
//  1. ID prüfen, Eintrag laden (er nennt die ID der Originaldatei).
//  2. Abschnitte löschen: Ab jetzt findet die Suche den Eintrag nicht mehr.
//  3. Originaldatei löschen. Fehlt sie schon, ist das kein Fehler.
//  4. Eintrag zuletzt löschen.
//
// Scheitert ein Schritt, bricht Ausfuehren ab. Der Eintrag ist dann noch
// sichtbar, und ein erneuter Aufruf räumt den Rest auf.
//
// Ab Schritt 2 läuft das Löschen auch dann zu Ende, wenn die App die
// Verbindung abbricht (context.WithoutCancel). Sonst könnte ein Eintrag
// ohne Datei und Abschnitte übrig bleiben, den die Zeitleiste zwar zeigt,
// dessen Original sich aber nicht mehr öffnen lässt.
//
// Fehler: kern.ErrUngueltigeID, kern.ErrEintragNichtGefunden (auch beim
// zweiten Löschen) oder ein interner Fehler.
func (l *Loeschen) Ausfuehren(ctx context.Context, id string) error {
	if err := kern.PruefeID(id); err != nil {
		return err
	}
	e, gefunden, err := l.speicher.EintragLesen(ctx, id)
	if err != nil {
		return err
	}
	if !gefunden {
		return kern.ErrEintragNichtGefunden
	}

	ctx, abbrechen := context.WithTimeout(context.WithoutCancel(ctx), loeschTimeout)
	defer abbrechen()

	if err := l.speicher.AbschnitteLoeschen(ctx, id); err != nil {
		return err
	}
	if err := l.speicher.DateiLoeschen(ctx, e.Datei.ID); err != nil {
		return err
	}
	gefunden, err = l.speicher.EintragLoeschen(ctx, id)
	if err != nil {
		return err
	}
	if !gefunden {
		// Ein gleichzeitiger Aufruf war schneller.
		return kern.ErrEintragNichtGefunden
	}
	// Nur die ID, nie Inhalte.
	slog.Info("Eintrag gelöscht", "eintrag", id)
	return nil
}
