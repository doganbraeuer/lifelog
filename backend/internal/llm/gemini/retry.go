package gemini

import (
	"context"
	"log/slog"
	"time"

	"google.golang.org/genai"
)

// wiederholer führt einen Aufruf aus und wiederholt ihn, wenn Gemini
// überlastet ist (503) oder ein Limit meldet (429): höchstens weitere Male,
// mit exponentiell wachsender Wartezeit (basis, 2·basis, …).
//
// Alles bleibt innerhalb der Frist des Kontexts (das Gesamtlimit, z. B. 50 s
// fürs Lesen): Reicht die Restzeit nicht mehr für Warten plus einen
// sinnvollen neuen Versuch, gibt der wiederholer auf und liefert den letzten
// Fehler.
//
// Andere Fehler (400, 404, 500, 504, Netzwerk) werden nicht wiederholt.
type wiederholer struct {
	weitere      int           // weitere Versuche nach dem ersten
	basis        time.Duration // erste Wartezeit
	mindestRest  time.Duration // so viel Zeit muss nach dem Warten noch bleiben
	jetzt        func() time.Time
	schlafen     func(context.Context, time.Duration) error
	beschreibung string // fürs Log, z. B. "Lesen gemini-3.8-flash"
}

func standardWiederholer(beschreibung string) wiederholer {
	return wiederholer{
		weitere:      2,
		basis:        time.Second,
		mindestRest:  5 * time.Second,
		jetzt:        time.Now,
		schlafen:     schlafenMitKontext,
		beschreibung: beschreibung,
	}
}

func (w wiederholer) ausfuehren(ctx context.Context, aufruf func(context.Context) error) error {
	warte := w.basis
	for versuch := 1; ; versuch++ {
		err := aufruf(ctx)
		if err == nil || !wiederholbar(err) || versuch > w.weitere {
			return err
		}
		if !w.reichtZeit(ctx, warte) {
			slog.Warn("Gemini überlastet, keine Zeit mehr für weiteren Versuch",
				"aufruf", w.beschreibung, "versuch", versuch)
			return err
		}
		slog.Warn("Gemini überlastet, neuer Versuch",
			"aufruf", w.beschreibung, "versuch", versuch, "warte", warte, "http", httpCode(err))
		if serr := w.schlafen(ctx, warte); serr != nil {
			return serr // Frist abgelaufen oder Anfrage abgebrochen
		}
		warte *= 2
	}
}

// reichtZeit: Bleibt nach dem Warten noch mindestRest bis zur Frist?
func (w wiederholer) reichtZeit(ctx context.Context, warte time.Duration) bool {
	frist, ok := ctx.Deadline()
	if !ok {
		return true
	}
	return frist.Sub(w.jetzt()) >= warte+w.mindestRest
}

// wiederholbar: nur 429 (Limit) und 503 (überlastet).
func wiederholbar(err error) bool {
	code := httpCode(err)
	return code == 429 || code == 503
}

// httpCode liefert den HTTP-Status eines Gemini-Fehlers oder 0.
func httpCode(err error) int {
	var api genai.APIError
	if asFehler(err, &api) {
		return api.Code
	}
	return 0
}

func schlafenMitKontext(ctx context.Context, d time.Duration) error {
	t := time.NewTimer(d)
	defer t.Stop()
	select {
	case <-t.C:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}
