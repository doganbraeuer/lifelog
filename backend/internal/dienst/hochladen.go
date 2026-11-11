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

	"lifelog/backend/internal/core"
	"lifelog/backend/internal/llm"
)

// UploadSpeicher ist, was der Upload von der Datenbank braucht. Umgesetzt
// von speicher.Speicher.
type UploadSpeicher interface {
	EintragMitHash(ctx context.Context, sha256 string) (id string, gefunden bool, err error)
	DateiSpeichern(ctx context.Context, id, dateiname, mimeTyp string, daten []byte) error
	DateiLoeschen(ctx context.Context, id string) error
	AbschnitteSpeichern(ctx context.Context, abschnitte []core.Chunk) error
	AbschnitteLoeschen(ctx context.Context, eintragID string) error
	EintragSpeichern(ctx context.Context, e core.Entry) error
}

// Upload ist eine hochgeladene Datei.
type Upload struct {
	Daten     []byte
	Dateiname string
	Modul     string // leer = Standardmodul
}

// TextUpload ist ein eingegebener Text (Text-Import, API.md 3.1).
type TextUpload struct {
	Text  string // genau wie eingegeben, nicht getrimmt
	Modul string // leer = Standardmodul
}

// eingang ist, was Datei- und Text-Upload nach der Eingangsprüfung
// gemeinsam haben. lesen ruft das passende Modell auf.
type eingang struct {
	modul     string
	daten     []byte // wird gespeichert und gehasht
	dateiname string
	mimeTyp   string
	lesen     func(ctx context.Context) (core.Extraction, error)
}

// DuplikatFehler: Genau diese Datei gibt es schon. EintragID nennt den
// vorhandenen Eintrag, damit die App direkt dorthin springen kann.
type DuplikatFehler struct{ EintragID string }

func (d *DuplikatFehler) Error() string { return "Duplikat von Eintrag " + d.EintragID }

// Is: errors.Is(err, core.ErrDuplikat) trifft auch auf DuplikatFehler zu.
func (d *DuplikatFehler) Is(ziel error) bool { return ziel == core.ErrDuplicate }

// Hochladen führt den Upload aus (docs/API.md 3.1).
type Hochladen struct {
	speicher  UploadSpeicher
	leser     llm.Leser
	einbetter llm.Einbetter
	neueID    func() string
	jetzt     func() time.Time
}

// NeuHochladen erstellt den Dienst. neueID vergibt ObjectIds
// (speicher.NeueID).
func NeuHochladen(s UploadSpeicher, l llm.Leser, e llm.Einbetter, neueID func() string) *Hochladen {
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
//  4. Erkennung prüfen (core.PruefeErkennung).
//  5. Volltext in Abschnitte zerlegen und einbetten.
//  6. Datei → Abschnitte → Eintrag speichern. Der Eintrag kommt zuletzt;
//     scheitert ein Schritt, werden die schon gespeicherten Teile wieder
//     entfernt. So bleibt kein halber Eintrag zurück.
//
// Fehler: core.ErrUnbekanntesModul, core.ErrDateiLeer, core.ErrDateiZuGross,
// core.ErrDateitypNichtUnterstuetzt, *DuplikatFehler, core.ErrNichtLesbar,
// core.ErrErkennungUngueltig, llm.ErrAntwortUngueltig,
// llm.ErrNichtErreichbar oder ein sonstiger (interner) Fehler.
func (h *Hochladen) Ausfuehren(ctx context.Context, u Upload) (core.Entry, error) {
	// 1. Prüfen, ohne Netz und ohne Datenbank.
	modul, err := core.CheckModule(u.Modul)
	if err != nil {
		return core.Entry{}, err
	}
	mimeTyp, err := core.CheckFile(u.Daten)
	if err != nil {
		return core.Entry{}, err
	}

	return h.verarbeiten(ctx, eingang{
		modul:     modul,
		daten:     u.Daten,
		dateiname: dateiname(u.Dateiname),
		mimeTyp:   mimeTyp,
		lesen: func(ctx context.Context) (core.Extraction, error) {
			return h.leser.DokumentLesen(ctx, u.Daten, mimeTyp, core.AllowedTypes(modul))
		},
	})
}

// TextAusfuehren importiert einen eingegebenen Text. Ablauf wie bei
// Ausfuehren, mit drei Unterschieden (API.md 3.1):
//
//   - Statt Größe und Typ prüft core.PruefeText (leer, UTF-8, 100 000
//     Zeichen).
//   - Gespeichert und gehasht werden die UTF-8-Bytes des Texts, als
//     text.txt mit text/plain; charset=utf-8.
//   - Das Sprachmodell (Leser.TextLesen) liefert nur Datum, Quelle, Typ
//     und Infos. volltext ist der eingegebene Text, unverändert, und ein
//     Text gilt immer als lesbar.
//
// Fehler: core.ErrUnbekanntesModul, core.ErrDateiLeer, core.ErrTextKeinUTF8,
// core.ErrTextZuLang, *DuplikatFehler, core.ErrErkennungUngueltig,
// llm.ErrAntwortUngueltig, llm.ErrNichtErreichbar oder ein interner
// Fehler.
func (h *Hochladen) TextAusfuehren(ctx context.Context, u TextUpload) (core.Entry, error) {
	modul, err := core.CheckModule(u.Modul)
	if err != nil {
		return core.Entry{}, err
	}
	if err := core.CheckText(u.Text); err != nil {
		return core.Entry{}, err
	}
	return h.verarbeiten(ctx, eingang{
		modul:     modul,
		daten:     []byte(u.Text), // Go-Strings sind UTF-8: genau diese Bytes
		dateiname: core.TextFileName,
		mimeTyp:   core.MimeText,
		lesen: func(ctx context.Context) (core.Extraction, error) {
			e, err := h.leser.TextLesen(ctx, u.Text, core.AllowedTypes(modul))
			if err != nil {
				return core.Extraction{}, err
			}
			e.Readable = true
			e.FullText = u.Text
			return e, nil
		},
	})
}

// verarbeiten führt die Schritte 2–6 aus, gleich für Datei und Text.
func (h *Hochladen) verarbeiten(ctx context.Context, ein eingang) (core.Entry, error) {
	modul := ein.modul

	// 2. Duplikat prüfen, bevor ein Modellaufruf etwas kostet.
	summe := sha256.Sum256(ein.daten)
	hash := hex.EncodeToString(summe[:])
	if id, gefunden, err := h.speicher.EintragMitHash(ctx, hash); err != nil {
		return core.Entry{}, err
	} else if gefunden {
		return core.Entry{}, &DuplikatFehler{EintragID: id}
	}

	// 3. Lesen.
	erkennung, err := ein.lesen(ctx)
	if err != nil {
		return core.Entry{}, err
	}

	// 4. Prüfen. Ins Log kommen nur Feldnamen, nie Werte (Gesundheitsdaten).
	eintrag, ausgelassen, err := core.CheckExtraction(erkennung, modul)
	if len(ausgelassen) > 0 {
		slog.Warn("unvollständige Infos weggelassen", "felder", ausgelassen)
	}
	if err != nil {
		var ef *core.ExtractionError
		if errors.As(err, &ef) {
			slog.Warn("Erkennung ungültig", "feld", ef.Field, "grund", ef.Reason)
		}
		return core.Entry{}, err
	}

	// 5. Abschnitte und Embeddings.
	texte := core.SplitIntoChunks(eintrag.FullText, core.ChunkMaxChars)
	vektoren, err := h.einbetter.AbschnitteEinbetten(ctx, texte)
	if err != nil {
		return core.Entry{}, err
	}
	if len(vektoren) != len(texte) {
		return core.Entry{}, fmt.Errorf("%d Embeddings für %d Abschnitte: %w",
			len(vektoren), len(texte), llm.ErrAntwortUngueltig)
	}

	// IDs und Metadaten vergeben. Millisekunden, weil MongoDB Zeitpunkte so
	// genau speichert: Antwort und spätere Abfrage zeigen dann dasselbe.
	eintrag.ID = h.neueID()
	eintrag.ImportedAt = h.jetzt().UTC().Truncate(time.Millisecond)
	eintrag.File = core.File{
		ID:       h.neueID(),
		FileName: ein.dateiname,
		MimeType: ein.mimeTyp,
		Size:     int64(len(ein.daten)),
		Sha256:   hash,
	}
	abschnitte := make([]core.Chunk, len(texte))
	for i, t := range texte {
		abschnitte[i] = core.Chunk{
			EntryID: eintrag.ID, Module: modul, Position: i + 1, Text: t, Embedding: vektoren[i],
		}
	}

	// 6. Speichern, Eintrag zuletzt.
	if err := h.speichern(ctx, eintrag, abschnitte, ein.daten); err != nil {
		return core.Entry{}, err
	}
	return eintrag, nil
}

func (h *Hochladen) speichern(ctx context.Context, e core.Entry, abschnitte []core.Chunk, daten []byte) error {
	if err := h.speicher.DateiSpeichern(ctx, e.File.ID, e.File.FileName, e.File.MimeType, daten); err != nil {
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

	if errors.Is(err, core.ErrDuplicate) {
		// Jemand hat dieselbe Datei gleichzeitig hochgeladen und war
		// schneller. Den vorhandenen Eintrag nennen wie bei Schritt 2.
		if id, gefunden, ferr := h.speicher.EintragMitHash(ctx, e.File.Sha256); ferr == nil && gefunden {
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
func (h *Hochladen) aufraeumen(ctx context.Context, e core.Entry, abschnitteAuch bool) {
	ctx, abbrechen := context.WithTimeout(context.WithoutCancel(ctx), aufraeumTimeout)
	defer abbrechen()

	if abschnitteAuch {
		if err := h.speicher.AbschnitteLoeschen(ctx, e.ID); err != nil {
			slog.Error("Aufräumen: Abschnitte nicht gelöscht", "eintrag", e.ID, "fehler", err)
		}
	}
	if err := h.speicher.DateiLoeschen(ctx, e.File.ID); err != nil {
		slog.Error("Aufräumen: Datei nicht gelöscht", "datei", e.File.ID, "fehler", err)
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
