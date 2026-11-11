package api

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"mime/multipart"
	"net/http"
	"strings"

	"lifelog/backend/internal/core"
	"lifelog/backend/internal/dienst"
	"lifelog/backend/internal/modell"
)

// Fehlercodes für den Upload (API.md 3.1).
const (
	CodeUngueltigeAnfrage         = "UNGUELTIGE_ANFRAGE"
	CodeDateiFehlt                = "DATEI_FEHLT"
	CodeDuplikat                  = "DUPLIKAT"
	CodeDateiZuGross              = "DATEI_ZU_GROSS"
	CodeDateitypNichtUnterstuetzt = "DATEITYP_NICHT_UNTERSTUETZT"
	CodeNichtLesbar               = "NICHT_LESBAR"
	CodeAuslesenFehlgeschlagen    = "AUSLESEN_FEHLGESCHLAGEN"
	CodeModellNichtErreichbar     = "MODELL_NICHT_ERREICHBAR"
)

// formularPuffer: Platz für Formular-Overhead (Grenzen, Kopfzeilen, Feld
// modul) zusätzlich zur Datei. Ist die Datei selbst über 10 MB, meldet
// core.PruefeDatei das genau; alles darüber bricht schon beim Lesen ab.
const formularPuffer = 1 << 20 // 1 MiB

// Hochlader führt den Upload bzw. Text-Import aus. Umgesetzt von
// dienst.Hochladen.
type Hochlader interface {
	Ausfuehren(ctx context.Context, u dienst.Upload) (core.Entry, error)
	TextAusfuehren(ctx context.Context, u dienst.TextUpload) (core.Entry, error)
}

// art: Datei-Upload oder Text-Import. Bestimmt nur die Fehlermeldungen.
type art int

const (
	artDatei art = iota
	artText
)

// hochladenHandler beantwortet POST /api/v1/eintraege (API.md 3.1).
func hochladenHandler(h Hochlader) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		r.Body = http.MaxBytesReader(w, r.Body, core.MaxFileBytes+formularPuffer)

		// maxMemory so groß wie die erlaubte Anfrage: Das ganze Formular
		// bleibt im Arbeitsspeicher. Sonst legt Go große Dateien als
		// temporäre Dateien auf der Festplatte ab, und Gesundheitsdokumente
		// sollen dort nicht herumliegen.
		if err := r.ParseMultipartForm(core.MaxFileBytes + formularPuffer); err != nil {
			var zuGross *http.MaxBytesError
			switch {
			case errors.As(err, &zuGross):
				schreibeFehler(w, http.StatusRequestEntityTooLarge, CodeDateiZuGross, "Die Datei ist größer als 10 MB.", nil)
			case errors.Is(err, http.ErrNotMultipart):
				schreibeFehler(w, http.StatusBadRequest, CodeUngueltigeAnfrage,
					"Bitte die Datei als multipart/form-data im Feld datei schicken.", nil)
			default:
				schreibeFehler(w, http.StatusBadRequest, CodeUngueltigeAnfrage, "Das Formular ist nicht lesbar.", nil)
			}
			return
		}
		defer func() { _ = r.MultipartForm.RemoveAll() }()

		dateien := r.MultipartForm.File["datei"]
		texte := r.MultipartForm.Value["text"]
		// Leer oder nur Leerraum gilt als fehlend (API.md 3.1).
		hatText := len(texte) > 0 && strings.TrimSpace(texte[0]) != ""
		modul := ersterWert(r.MultipartForm.Value["modul"])
		switch {
		case len(texte) > 1:
			schreibeFehler(w, http.StatusBadRequest, CodeUngueltigeAnfrage, "Bitte genau einen Text schicken.", nil)
			return
		case len(dateien) > 0 && hatText:
			schreibeFehler(w, http.StatusBadRequest, CodeUngueltigeAnfrage,
				"Bitte entweder eine Datei oder einen Text schicken, nicht beides.", nil)
			return
		case hatText:
			// Der Text geht unverändert weiter, nicht getrimmt.
			eintrag, err := h.TextAusfuehren(r.Context(), dienst.TextUpload{Text: texte[0], Modul: modul})
			schreibeErgebnis(w, r, eintrag, err, artText, modul)
			return
		case len(dateien) == 0 && len(texte) == 1:
			schreibeFehler(w, http.StatusBadRequest, CodeDateiFehlt, "Bitte einen Text eingeben.", nil)
			return
		case len(dateien) == 0:
			schreibeFehler(w, http.StatusBadRequest, CodeDateiFehlt, "Bitte eine Datei auswählen.", nil)
			return
		case len(dateien) > 1:
			schreibeFehler(w, http.StatusBadRequest, CodeUngueltigeAnfrage, "Bitte genau eine Datei schicken.", nil)
			return
		}

		daten, err := leseDatei(dateien[0])
		if err != nil {
			schreibeInternenFehler(w, r, err)
			return
		}
		eintrag, err := h.Ausfuehren(r.Context(), dienst.Upload{
			Daten: daten, Dateiname: dateien[0].Filename, Modul: modul,
		})
		schreibeErgebnis(w, r, eintrag, err, artDatei, modul)
	}
}

// schreibeErgebnis: 201 mit Location und Eintrag, oder der passende Fehler.
func schreibeErgebnis(w http.ResponseWriter, r *http.Request, eintrag core.Entry, err error, a art, modul string) {
	if err != nil {
		schreibeUploadFehler(w, r, err, a, modul)
		return
	}
	w.Header().Set("Location", "/api/v1/eintraege/"+eintrag.ID)
	schreibeJSON(w, http.StatusCreated, eintragZuJSON(eintrag))
}

// schreibeUploadFehler ordnet jedem Fehler des Dienstes den Code aus
// API.md 3.1 zu.
func schreibeUploadFehler(w http.ResponseWriter, r *http.Request, err error, a art, modul string) {
	var duplikat *dienst.DuplikatFehler
	switch {
	case a == artText && errors.Is(err, core.ErrFileEmpty):
		schreibeFehler(w, http.StatusBadRequest, CodeDateiFehlt, "Bitte einen Text eingeben.", nil)
	case errors.Is(err, core.ErrTextNotUTF8):
		schreibeFehler(w, http.StatusBadRequest, CodeUngueltigeAnfrage,
			"Der Text ist kein gültiges UTF-8.", map[string]any{"feld": "text"})
	case errors.Is(err, core.ErrTextTooLong):
		schreibeFehler(w, http.StatusRequestEntityTooLarge, CodeDateiZuGross,
			"Der Text ist länger als 100 000 Zeichen.", map[string]any{"maxZeichen": core.MaxTextChars})
	case a == artText && errors.As(err, &duplikat):
		schreibeFehler(w, http.StatusConflict, CodeDuplikat, "Diesen Text hast du schon importiert.",
			map[string]any{"eintragId": duplikat.EintragID})
	case errors.Is(err, core.ErrUnknownModule):
		schreibeFehler(w, http.StatusBadRequest, CodeUngueltigeAnfrage,
			"Unbekanntes Modul. Im Prototyp gibt es nur gesundheit.",
			map[string]any{"feld": "modul", "wert": modul})
	case errors.Is(err, core.ErrFileEmpty):
		schreibeFehler(w, http.StatusBadRequest, CodeDateiFehlt, "Die Datei ist leer.", nil)
	case errors.Is(err, core.ErrFileTooLarge):
		schreibeFehler(w, http.StatusRequestEntityTooLarge, CodeDateiZuGross, "Die Datei ist größer als 10 MB.", nil)
	case errors.Is(err, core.ErrUnsupportedFileType):
		schreibeFehler(w, http.StatusUnsupportedMediaType, CodeDateitypNichtUnterstuetzt,
			"Dieser Dateityp wird nicht unterstützt. Erlaubt sind JPEG, PNG, HEIC und PDF.", nil)
	case errors.As(err, &duplikat):
		schreibeFehler(w, http.StatusConflict, CodeDuplikat, "Diese Datei hast du schon hochgeladen.",
			map[string]any{"eintragId": duplikat.EintragID})
	case errors.Is(err, core.ErrUnreadable):
		schreibeFehler(w, http.StatusUnprocessableEntity, CodeNichtLesbar,
			"Darauf ist kein lesbares Dokument zu erkennen. Versuche es mit einem schärferen Foto.", nil)
	case errors.Is(err, core.ErrInvalidExtraction), errors.Is(err, modell.ErrAntwortUngueltig):
		slog.Warn("Auslesen fehlgeschlagen", "fehler", err)
		schreibeFehler(w, http.StatusBadGateway, CodeAuslesenFehlgeschlagen,
			"Das Dokument konnte nicht ausgelesen werden. Bitte versuche es noch einmal.", nil)
	case errors.Is(err, modell.ErrNichtErreichbar):
		slog.Warn("Modell nicht erreichbar", "fehler", err)
		schreibeFehler(w, http.StatusServiceUnavailable, CodeModellNichtErreichbar,
			"Die Texterkennung ist gerade nicht erreichbar. Bitte versuche es später noch einmal.", nil)
	case errors.Is(err, context.Canceled):
		// Die App hat die Verbindung getrennt; niemand liest die Antwort mehr.
		slog.Info("Upload abgebrochen")
	default:
		schreibeInternenFehler(w, r, err)
	}
}

func leseDatei(fh *multipart.FileHeader) ([]byte, error) {
	f, err := fh.Open()
	if err != nil {
		return nil, err
	}
	defer f.Close()
	// Ein Byte mehr als erlaubt lesen, damit core.PruefeDatei "zu groß"
	// erkennt, statt eine abgeschnittene Datei zu prüfen.
	return io.ReadAll(io.LimitReader(f, core.MaxFileBytes+1))
}

func ersterWert(werte []string) string {
	if len(werte) == 0 {
		return ""
	}
	return werte[0]
}
