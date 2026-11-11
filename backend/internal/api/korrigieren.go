package api

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"slices"

	"lifelog/backend/internal/core"
)

// Fehlercodes für Korrekturen (API.md 3.5).
const (
	CodeFeldNichtKorrigierbar = "FELD_NICHT_KORRIGIERBAR"
	CodeInfoNichtGefunden     = "INFO_NICHT_GEFUNDEN"
)

// maxKorrekturBody: Grenze für den ganzen Body; eigene Längengrenzen für
// einzelne Werte gibt es noch nicht (API.md 4, Längengrenzen).
const maxKorrekturBody = 64 << 10

// Korrigierer korrigiert einen Eintrag. Umgesetzt von dienst.Korrigieren.
type Korrigierer interface {
	Ausfuehren(ctx context.Context, id string, a core.Aenderung) (core.Eintrag, error)
}

// Felder des Eintrags, die es gibt, aber nicht korrigiert werden dürfen.
// Sie bekommen FELD_NICHT_KORRIGIERBAR statt "unbekanntes Feld".
var nichtKorrigierbar = []string{"datei", "id", "importdatum", "korrekturen", "modul", "volltext"}

// korrigierenHandler beantwortet PATCH /api/v1/eintraege/{id} (API.md 3.5).
func korrigierenHandler(k Korrigierer) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id := r.PathValue("id")
		// ID vor dem Body prüfen: "/eintraege/abc" ist immer UNGUELTIGE_ID.
		if err := core.PruefeID(id); err != nil {
			schreibeFehler(w, http.StatusBadRequest, CodeUngueltigeID, "Diese Eintrag-ID ist ungültig.", nil)
			return
		}

		body, err := io.ReadAll(http.MaxBytesReader(w, r.Body, maxKorrekturBody))
		if err != nil {
			schreibeFehler(w, http.StatusBadRequest, CodeUngueltigeAnfrage,
				"Die Anfrage ist zu groß oder unvollständig.", nil)
			return
		}
		aenderung, err := leseAenderung(body)
		if err != nil {
			schreibeKorrekturFehler(w, r, err)
			return
		}

		e, err := k.Ausfuehren(r.Context(), id, aenderung)
		if err != nil {
			schreibeKorrekturFehler(w, r, err)
			return
		}
		schreibeJSON(w, http.StatusOK, eintragZuJSON(e))
	}
}

func schreibeKorrekturFehler(w http.ResponseWriter, r *http.Request, err error) {
	var kf *core.KorrekturFehler
	switch {
	case errors.As(err, &kf):
		status, code := http.StatusBadRequest, CodeUngueltigeAnfrage
		switch {
		case errors.Is(err, core.ErrFeldNichtKorrigierbar):
			code = CodeFeldNichtKorrigierbar
		case errors.Is(err, core.ErrInfoNichtGefunden):
			status, code = http.StatusNotFound, CodeInfoNichtGefunden
		}
		schreibeFehler(w, status, code, kf.Nachricht, korrekturDetails(kf))
	case errors.Is(err, core.ErrUngueltigeID):
		schreibeFehler(w, http.StatusBadRequest, CodeUngueltigeID, "Diese Eintrag-ID ist ungültig.", nil)
	case errors.Is(err, core.ErrEintragNichtGefunden):
		schreibeFehler(w, http.StatusNotFound, CodeEintragNichtGefunden, "Diesen Eintrag gibt es nicht.", nil)
	case errors.Is(err, context.Canceled):
		// Die App hat aufgegeben; niemand liest die Antwort.
	default:
		schreibeInternenFehler(w, r, err)
	}
}

// korrekturDetails: feld, wert und infoId, soweit vorhanden (API.md 3.5).
// Der Wert geht nur an die App zurück, die ihn geschickt hat, nie ins Log.
func korrekturDetails(kf *core.KorrekturFehler) map[string]any {
	d := map[string]any{}
	if kf.Feld != "" {
		d["feld"] = kf.Feld
	}
	if kf.Wert != nil {
		d["wert"] = kf.Wert
	}
	if kf.InfoID != "" {
		d["infoId"] = kf.InfoID
	}
	if len(d) == 0 {
		return nil
	}
	return d
}

// leseAenderung übersetzt den JSON-Body in eine core.Aenderung. Hier fällt
// alles auf, was am JSON selbst nicht stimmt (falscher Typ, unbekanntes
// Feld, null, wo keins erlaubt ist). Was den Eintrag braucht (Typ im Modul,
// gültiges Datum, Info vorhanden), prüft core.Korrigiere.
//
// Warum nicht einfach in ein struct dekodieren? Dann wären "feld fehlt" und
// "feld": null beide nil. Über json.RawMessage sehen wir den Unterschied.
//
// Felder werden alphabetisch geprüft, damit dieselbe Anfrage immer
// denselben Fehler liefert (eine Go-Map hat keine feste Reihenfolge).
func leseAenderung(body []byte) (core.Aenderung, error) {
	var felder map[string]json.RawMessage
	if err := json.Unmarshal(body, &felder); err != nil || felder == nil {
		return core.Aenderung{}, core.Ungueltig(
			"Bitte die Änderung als JSON-Objekt schicken, z. B. {\"quelle\": \"…\"}.", "", nil)
	}

	namen := sortierteSchluessel(felder)
	for _, name := range namen {
		switch {
		case slices.Contains(nichtKorrigierbar, name):
			return core.Aenderung{}, core.NichtKorrigierbar(name)
		case !slices.Contains([]string{"dokumentdatum", "infos", "quelle", "typ"}, name):
			return core.Aenderung{}, core.Ungueltig("Unbekanntes Feld: "+name+".", name, nil)
		}
	}

	var a core.Aenderung
	if roh, ok := felder["typ"]; ok {
		typ, ist := alsText(roh)
		if !ist {
			return core.Aenderung{}, core.Ungueltig("Der Typ muss ein Text sein.", "typ", alsWert(roh))
		}
		a.Typ = &typ
	}
	if roh, ok := felder["dokumentdatum"]; ok {
		f, ok := alsFeld(roh)
		if !ok {
			return core.Aenderung{}, core.Ungueltig("Das Dokumentdatum muss im Format JJJJ-MM-TT angegeben werden.",
				"dokumentdatum", alsWert(roh))
		}
		a.Dokumentdatum = f
	}
	if roh, ok := felder["quelle"]; ok {
		f, ok := alsFeld(roh)
		if !ok {
			return core.Aenderung{}, core.Ungueltig("Die Quelle muss ein Text oder null sein.", "quelle", alsWert(roh))
		}
		a.Quelle = f
	}
	if roh, ok := felder["infos"]; ok {
		infos, err := leseInfos(roh)
		if err != nil {
			return core.Aenderung{}, err
		}
		a.Infos = infos
	}
	return a, nil
}

// leseInfos: "infos" muss eine Liste von Objekten mit id und wert und/oder
// einheit sein. Das Ergebnis ist nie nil, auch bei "infos": [].
func leseInfos(roh json.RawMessage) ([]core.InfoAenderung, error) {
	var liste []json.RawMessage
	if err := json.Unmarshal(roh, &liste); err != nil || liste == nil {
		return nil, core.Ungueltig("\"infos\" muss eine Liste sein.", "infos", nil)
	}
	infos := make([]core.InfoAenderung, 0, len(liste))
	for _, element := range liste {
		var felder map[string]json.RawMessage
		if err := json.Unmarshal(element, &felder); err != nil || felder == nil {
			return nil, core.Ungueltig("Jede Info muss ein Objekt sein.", "infos", nil)
		}
		id, ist := alsText(felder["id"])
		if !ist || id == "" {
			return nil, core.Ungueltig("Jede Info braucht eine id.", "infos", nil)
		}
		pfad := "infos." + id + "."

		for _, name := range sortierteSchluessel(felder) {
			switch name {
			case "id", "wert", "einheit":
			case "name":
				return nil, core.NichtKorrigierbar(pfad + "name")
			default:
				return nil, core.Ungueltig("Unbekanntes Feld: "+pfad+name+".", pfad+name, nil)
			}
		}

		info := core.InfoAenderung{ID: id}
		if roh, ok := felder["wert"]; ok {
			wert, ist := alsText(roh)
			if !ist {
				return nil, core.Ungueltig("Der Wert muss ein Text sein.", pfad+"wert", alsWert(roh))
			}
			info.Wert = &wert
		}
		if roh, ok := felder["einheit"]; ok {
			f, ok := alsFeld(roh)
			if !ok {
				return nil, core.Ungueltig("Die Einheit muss ein Text oder null sein.", pfad+"einheit", alsWert(roh))
			}
			info.Einheit = f
		}
		infos = append(infos, info)
	}
	return infos, nil
}

// alsText: Ist roh ein JSON-String? null, Zahlen usw. sind es nicht.
func alsText(roh json.RawMessage) (string, bool) {
	var s string
	if len(roh) == 0 || roh[0] != '"' || json.Unmarshal(roh, &s) != nil {
		return "", false
	}
	return s, true
}

// alsFeld: null oder ein String, für Felder, die entfernt werden dürfen.
func alsFeld(roh json.RawMessage) (core.Feld, bool) {
	if bytes.Equal(bytes.TrimSpace(roh), []byte("null")) {
		return core.Feld{Gesetzt: true}, true
	}
	s, ok := alsText(roh)
	if !ok {
		return core.Feld{}, false
	}
	return core.Feld{Gesetzt: true, Wert: &s}, true
}

// alsWert dekodiert einen abgelehnten Wert für details.wert. null ergibt
// nil, und details.wert fehlt dann.
func alsWert(roh json.RawMessage) any {
	var w any
	_ = json.Unmarshal(roh, &w)
	return w
}

func sortierteSchluessel(m map[string]json.RawMessage) []string {
	namen := make([]string, 0, len(m))
	for n := range m {
		namen = append(namen, n)
	}
	slices.Sort(namen)
	return namen
}
