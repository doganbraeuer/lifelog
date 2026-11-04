// Package dienst enthält die Abläufe des Backends (Upload, später Fragen,
// Löschen, Korrigieren). Ein Dienst verbindet Fachlogik (kern), Modelle
// (modell) und Speicher, kennt aber weder HTTP noch MongoDB oder Gemini
// direkt, nur deren Schnittstellen. Deshalb lässt er sich vollständig mit
// Attrappen testen.
package dienst

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"lifelog/backend/internal/kern"
	"lifelog/backend/internal/modell"
)

// UploadSpeicher ist, was der Upload von der Datenbank braucht. Umgesetzt
// von speicher.Speicher.
type UploadSpeicher interface {
	EintragMitHash(ctx context.Context, sha256 string) (id string, gefunden bool, err error)
	DateiSpeichern(ctx context.Context, id, dateiname, mimeTyp string, daten []byte) error
	DateiLoeschen(ctx context.Context, id string) error
	AbschnitteSpeichern(ctx context.Context, abschnitte []kern.Abschnitt) error
	AbschnitteLoeschen(ctx context.Context, eintragID string) error
	EintragSpeichern(ctx context.Context, e kern.Eintrag) error
}

// Upload ist eine hochgeladene Datei.
type Upload struct {
	Daten     []byte
	Dateiname string
	Modul     string // leer = Standardmodul
}

// DuplikatFehler: Genau diese Datei gibt es schon. EintragID nennt den
// vorhandenen Eintrag, damit die App direkt dorthin springen kann.
type DuplikatFehler struct{ EintragID string }

func (d *DuplikatFehler) Error() string { return "Duplikat von Eintrag " + d.EintragID }

// Is: errors.Is(err, kern.ErrDuplikat) trifft auch auf DuplikatFehler zu.
func (d *DuplikatFehler) Is(ziel error) bool { return ziel == kern.ErrDuplikat }

// Hochladen führt den Upload aus (docs/API.md 3.1).
type Hochladen struct {
	speicher  UploadSpeicher
	leser     modell.Leser
	einbetter modell.Einbetter
	neueID    func() string
	jetzt     func() time.Time
}

// NeuHochladen erstellt den Dienst. neueID vergibt ObjectIds
// (speicher.NeueID).
func NeuHochladen(s UploadSpeicher, l modell.Leser, e modell.Einbetter, neueID func() string) *Hochladen {
	return &Hochladen{speicher: s, leser: l, einbetter: e, neueID: neueID, jetzt: time.Now}
}

// aufraeumTimeout: Zeit fürs Entfernen halb gespeicherter Teile. Läuft mit
// eigenem Kontext, weil der Kontext der Anfrage dann oft schon abgelaufen
// oder abgebrochen ist.
const aufraeumTimeout = 10 * time.Second

// Ausfuehren lädt die Datei hoch, lässt sie auslesen und speichert alles.
//
//  1. Modul, Größe und Typ prüfen.
//  2. Hash berechnen, Duplikat prüfen, VOR dem Modellaufruf.
//  3. Vision-LLM lesen lassen.
//  4. Erkennung prüfen (kern.PruefeErkennung).
//  5. Volltext in Abschnitte zerlegen und einbetten.
//  6. Datei → Abschnitte → Eintrag speichern. Der Eintrag kommt zuletzt;
//     scheitert ein Schritt, werden die schon gespeicherten Teile wieder
//     entfernt. So bleibt kein halber Eintrag zurück.
//
// Fehler: kern.ErrUnbekanntesModul, kern.ErrDateiLeer, kern.ErrDateiZuGross,
// kern.ErrDateitypNichtUnterstuetzt, *DuplikatFehler, kern.ErrNichtLesbar,
// kern.ErrErkennungUngueltig, modell.ErrAntwortUngueltig,
// modell.ErrNichtErreichbar oder ein sonstiger (interner) Fehler.
func (h *Hochladen) Ausfuehren(ctx context.Context, u Upload) (kern.Eintrag, error) {
	// 1. Prüfen, ohne Netz und ohne Datenbank.
	modul, err := kern.PruefeModul(u.Modul)
	if err != nil {
		return kern.Eintrag{}, err
	}
	mimeTyp, err := kern.PruefeDatei(u.Daten)
	if err != nil {
		return kern.Eintrag{}, err
	}

	// 2. Duplikat prüfen, bevor ein Modellaufruf etwas kostet.
	summe := sha256.Sum256(u.Daten)
	hash := hex.EncodeToString(summe[:])
	if id, gefunden, err := h.speicher.EintragMitHash(ctx, hash); err != nil {
		return kern.Eintrag{}, err
	} else if gefunden {
		return kern.Eintrag{}, &DuplikatFehler{EintragID: id}
	}

	// 3. Lesen.
	erkennung, err := h.leser.DokumentLesen(ctx, u.Daten, mimeTyp, kern.ErlaubteTypen(modul))
	if err != nil {
		return kern.Eintrag{}, err
	}

	// 4. Prüfen. Ins Log kommen nur Feldnamen, nie Werte (Gesundheitsdaten).
	eintrag, ausgelassen, err := kern.PruefeErkennung(erkennung, modul)
	if len(ausgelassen) > 0 {
		slog.Warn("unvollständige Infos weggelassen", "felder", ausgelassen)
	}
	if err != nil {
		var ef *kern.ErkennungFehler
		if errors.As(err, &ef) {
			slog.Warn("Erkennung ungültig", "feld", ef.Feld, "grund", ef.Grund)
		}
		return kern.Eintrag{}, err
	}

	// 5. Abschnitte und Embeddings.
	texte := kern.ZerlegeInAbschnitte(eintrag.Volltext, kern.AbschnittMaxZeichen)
	vektoren, err := h.einbetter.AbschnitteEinbetten(ctx, texte)
	if err != nil {
		return kern.Eintrag{}, err
	}
	if len(vektoren) != len(texte) {
		return kern.Eintrag{}, fmt.Errorf("%d Embeddings für %d Abschnitte: %w",
			len(vektoren), len(texte), modell.ErrAntwortUngueltig)
	}

	// IDs und Metadaten vergeben. Millisekunden, weil MongoDB Zeitpunkte so
	// genau speichert: Antwort und spätere Abfrage zeigen dann dasselbe.
	eintrag.ID = h.neueID()
	eintrag.Importdatum = h.jetzt().UTC().Truncate(time.Millisecond)
	eintrag.Datei = kern.Datei{
		ID:        h.neueID(),
		Dateiname: dateiname(u.Dateiname),
		MimeTyp:   mimeTyp,
		Groesse:   int64(len(u.Daten)),
		Sha256:    hash,
	}
	abschnitte := make([]kern.Abschnitt, len(texte))
	for i, t := range texte {
		abschnitte[i] = kern.Abschnitt{
			EintragID: eintrag.ID, Modul: modul, Nr: i + 1, Text: t, Embedding: vektoren[i],
		}
	}

	// 6. Speichern, Eintrag zuletzt.
	if err := h.speichern(ctx, eintrag, abschnitte, u.Daten); err != nil {
		return kern.Eintrag{}, err
	}
	return eintrag, nil
}

func (h *Hochladen) speichern(ctx context.Context, e kern.Eintrag, abschnitte []kern.Abschnitt, daten []byte) error {
	if err := h.speicher.DateiSpeichern(ctx, e.Datei.ID, e.Datei.Dateiname, e.Datei.MimeTyp, daten); err != nil {
		h.aufraeumen(ctx, e, false)
		return err
	}
	if err := h.speicher.AbschnitteSpeichern(ctx, abschnitte); err != nil {
		h.aufraeumen(ctx, e, true)
		return err
	}
	err := h.speicher.EintragSpeichern(ctx, e)
	if err == nil {
		return nil
	}
	h.aufraeumen(ctx, e, true)

	if errors.Is(err, kern.ErrDuplikat) {
		// Jemand hat dieselbe Datei gleichzeitig hochgeladen und war
		// schneller. Den vorhandenen Eintrag nennen wie bei Schritt 2.
		if id, gefunden, ferr := h.speicher.EintragMitHash(ctx, e.Datei.Sha256); ferr == nil && gefunden {
			return &DuplikatFehler{EintragID: id}
		}
	}
	return err
}

// aufraeumen entfernt Datei und (falls schon gespeichert) Abschnitte eines
// gescheiterten Uploads. Auch die Datei wird bei einem Fehler in
// DateiSpeichern gelöscht: GridFS kann schon einzelne Stücke geschrieben
// haben. Scheitert das Aufräumen, steht es im Log; der Eintrag selbst
// existiert ja nicht, die App sieht also nichts Halbes.
func (h *Hochladen) aufraeumen(ctx context.Context, e kern.Eintrag, abschnitteAuch bool) {
	ctx, abbrechen := context.WithTimeout(context.WithoutCancel(ctx), aufraeumTimeout)
	defer abbrechen()

	if abschnitteAuch {
		if err := h.speicher.AbschnitteLoeschen(ctx, e.ID); err != nil {
			slog.Error("Aufräumen: Abschnitte nicht gelöscht", "eintrag", e.ID, "fehler", err)
		}
	}
	if err := h.speicher.DateiLoeschen(ctx, e.Datei.ID); err != nil {
		slog.Error("Aufräumen: Datei nicht gelöscht", "datei", e.Datei.ID, "fehler", err)
	}
}

// dateiname macht aus dem Namen beim Upload einen sicheren Anzeigenamen:
// ohne Pfad und Steuerzeichen, höchstens 200 Zeichen, nie leer.
func dateiname(name string) string {
	if i := strings.LastIndexAny(name, `/\`); i >= 0 {
		name = name[i+1:]
	}
	name = strings.Map(func(r rune) rune {
		if r < 0x20 || r == 0x7f {
			return -1
		}
		return r
	}, name)
	name = strings.TrimSpace(name)
	if r := []rune(name); len(r) > 200 {
		name = string(r[:200])
	}
	if name == "" {
		return "dokument"
	}
	return name
}
