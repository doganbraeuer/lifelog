package kern

import (
	"errors"
	"fmt"
	"slices"
	"strings"
	"time"
)

// Feld ist ein Wert in einer Korrektur, der fehlen, null sein oder einen
// Text enthalten kann. JSON kennt diese drei Fälle, Go-Zeiger nur zwei:
//
//	Feld{}                           fehlt in der Anfrage → unverändert
//	Feld{Gesetzt: true}              "feld": null         → Wert entfernen
//	Feld{Gesetzt: true, Wert: &s}    "feld": "…"          → Wert setzen
type Feld struct {
	Gesetzt bool
	Wert    *string
}

// InfoAenderung ändert eine Info. Wert nil heißt unverändert; null ist für
// den Wert nicht erlaubt und wird schon beim Lesen des JSON abgelehnt.
type InfoAenderung struct {
	ID      string
	Wert    *string
	Einheit Feld
}

// Aenderung ist der Inhalt von PATCH /eintraege/{id} (API.md 3.5).
type Aenderung struct {
	Typ           *string // nil: unverändert (null ist nicht erlaubt)
	Dokumentdatum Feld
	Quelle        Feld
	// Infos: nil heißt, "infos" fehlt in der Anfrage. Eine leere Liste
	// heißt "infos": [] und ändert nichts.
	Infos []InfoAenderung
}

// leer: Die Anfrage nennt kein einziges Feld, entspricht dem Body {}.
func (a Aenderung) leer() bool {
	return a.Typ == nil && !a.Dokumentdatum.Gesetzt && !a.Quelle.Gesetzt && a.Infos == nil
}

// Fehlerarten einer Korrektur. Die API macht daraus die Codes aus API.md 3.5.
var (
	ErrKorrekturUngueltig    = errors.New("Korrektur ungültig")      // 400 UNGUELTIGE_ANFRAGE
	ErrFeldNichtKorrigierbar = errors.New("Feld nicht korrigierbar") // 400 FELD_NICHT_KORRIGIERBAR
	ErrInfoNichtGefunden     = errors.New("Info nicht gefunden")     // 404 INFO_NICHT_GEFUNDEN
)

// KorrekturFehler beschreibt, was an einer Korrektur nicht passt.
//
// Nachricht ist ein deutscher Satz für die App. Feld, Wert und InfoID landen
// in details. Error() enthält den Wert bewusst nicht: Fehler können im Log
// landen, und Werte können Gesundheitsdaten sein.
type KorrekturFehler struct {
	Art       error  // ErrKorrekturUngueltig, ErrFeldNichtKorrigierbar oder ErrInfoNichtGefunden
	Nachricht string // für die App
	Feld      string // z. B. "dokumentdatum", "infos.i2.wert"; leer, wenn es um eine Info als Ganzes geht
	InfoID    string // bei Fehlern, die eine Info als Ganzes betreffen
	Wert      any    // der abgelehnte Wert, nur für details; nil, wenn er nicht hingehört
}

func (f *KorrekturFehler) Error() string {
	ziel := f.Feld
	if ziel == "" && f.InfoID != "" {
		ziel = "infos." + f.InfoID
	}
	if ziel == "" {
		return f.Art.Error()
	}
	return f.Art.Error() + ": " + ziel
}

// Is sorgt dafür, dass errors.Is(err, ErrKorrekturUngueltig) usw. zutrifft.
func (f *KorrekturFehler) Is(ziel error) bool { return ziel == f.Art }

// Ungueltig baut einen Fehler der Art ErrKorrekturUngueltig. Auch die API
// benutzt das für Fehler, die schon beim Lesen des JSON auffallen.
func Ungueltig(nachricht, feld string, wert any) *KorrekturFehler {
	return &KorrekturFehler{Art: ErrKorrekturUngueltig, Nachricht: nachricht, Feld: feld, Wert: wert}
}

// NichtKorrigierbar baut einen Fehler der Art ErrFeldNichtKorrigierbar.
func NichtKorrigierbar(feld string) *KorrekturFehler {
	return &KorrekturFehler{
		Art:       ErrFeldNichtKorrigierbar,
		Nachricht: fmt.Sprintf("Das Feld %s kann nicht korrigiert werden.", feld),
		Feld:      feld,
	}
}

// aenderung ist ein geprüfter neuer Wert für einen Feldpfad wie
// "dokumentdatum" oder "infos.i2.einheit".
type aenderung struct {
	feld string
	neu  *string
}

// Korrigiere prüft die Änderung gegen den Eintrag und wendet sie an
// (API.md 2, Regeln für Korrekturen, und 3.5).
//
// Ablauf: Zuerst wird alles geprüft. Nur wenn alles gültig ist, werden die
// Werte geschrieben. Bei einem Fehler ist das Ergebnis also nie halb
// geändert.
//
// Regeln:
//  1. Im Feld steht danach der aktuelle Wert.
//  2. Pro Feld höchstens eine Korrektur. erkannt bleibt der ursprüngliche
//     Wert, korrigiert und zeitpunkt zeigen den letzten Stand.
//  3. Zurück auf den erkannten Wert: Die Korrektur verschwindet.
//  4. Ändert sich ein Wert nicht, entsteht keine Korrektur.
//
// Leerraum am Rand wird entfernt; leere quelle oder einheit gelten als null,
// ein leerer wert ist ein Fehler.
//
// e wird nicht verändert. geaendert nennt die Felder, deren Wert sich
// wirklich geändert hat, in der Reihenfolge der Anfrage; ist die Liste
// leer, muss der Aufrufer nichts speichern. Sie enthält nur Feldnamen, nie
// Werte, darf also ins Log.
func Korrigiere(e Eintrag, a Aenderung, jetzt time.Time) (neu Eintrag, geaendert []string, err error) {
	liste, err := pruefeAenderung(e, a)
	if err != nil {
		return Eintrag{}, nil, err
	}

	neu = e
	neu.Infos = slices.Clone(e.Infos)
	neu.Korrekturen = slices.Clone(e.Korrekturen)
	if neu.Korrekturen == nil {
		neu.Korrekturen = []Korrektur{}
	}
	geaendert = []string{}
	for _, ae := range liste {
		if setze(&neu, ae.feld, ae.neu, jetzt.UTC()) {
			geaendert = append(geaendert, ae.feld)
		}
	}
	return neu, geaendert, nil
}

// pruefeAenderung prüft alles und liefert die neuen Werte als Liste von
// Feldpfaden. Reihenfolge: typ, dokumentdatum, quelle, dann die Infos in der
// Reihenfolge der Anfrage. So meldet dieselbe Anfrage immer denselben
// Fehler.
func pruefeAenderung(e Eintrag, a Aenderung) ([]aenderung, error) {
	if a.leer() {
		return nil, Ungueltig("Die Anfrage enthält keine Änderung.", "", nil)
	}
	var liste []aenderung

	if a.Typ != nil {
		typ := strings.TrimSpace(*a.Typ)
		if !TypErlaubt(e.Modul, typ) {
			return nil, Ungueltig(
				fmt.Sprintf("Unbekannter Typ. Erlaubt sind %s.", strings.Join(ErlaubteTypen(e.Modul), ", ")),
				"typ", *a.Typ)
		}
		liste = append(liste, aenderung{"typ", &typ})
	}

	if a.Dokumentdatum.Gesetzt {
		var datum *string
		if a.Dokumentdatum.Wert != nil {
			d := strings.TrimSpace(*a.Dokumentdatum.Wert)
			if err := PruefeDatum(d); err != nil {
				nachricht := "Das Dokumentdatum muss im Format JJJJ-MM-TT angegeben werden."
				if errors.Is(err, ErrKeinKalendertag) {
					nachricht = "Das Dokumentdatum ist kein gültiges Kalenderdatum."
				}
				return nil, Ungueltig(nachricht, "dokumentdatum", *a.Dokumentdatum.Wert)
			}
			datum = &d
		}
		liste = append(liste, aenderung{"dokumentdatum", datum})
	}

	if a.Quelle.Gesetzt {
		liste = append(liste, aenderung{"quelle", ohneLeer(a.Quelle.Wert)})
	}

	gesehen := map[string]bool{}
	for _, ia := range a.Infos {
		if ia.ID == "" {
			return nil, Ungueltig("Jede Info braucht eine id.", "infos", nil)
		}
		if gesehen[ia.ID] {
			return nil, &KorrekturFehler{Art: ErrKorrekturUngueltig, InfoID: ia.ID,
				Nachricht: fmt.Sprintf("Die Info %s kommt mehrfach vor.", ia.ID)}
		}
		gesehen[ia.ID] = true
		if ia.Wert == nil && !ia.Einheit.Gesetzt {
			return nil, &KorrekturFehler{Art: ErrKorrekturUngueltig, InfoID: ia.ID,
				Nachricht: fmt.Sprintf("Die Info %s enthält keine Änderung.", ia.ID)}
		}
		if infoIndex(e.Infos, ia.ID) < 0 {
			return nil, &KorrekturFehler{Art: ErrInfoNichtGefunden, InfoID: ia.ID,
				Nachricht: "Diese Info gibt es in dem Eintrag nicht."}
		}
		if ia.Wert != nil {
			wert := strings.TrimSpace(*ia.Wert)
			if wert == "" {
				return nil, Ungueltig("Der Wert darf nicht leer sein.", "infos."+ia.ID+".wert", *ia.Wert)
			}
			liste = append(liste, aenderung{"infos." + ia.ID + ".wert", &wert})
		}
		if ia.Einheit.Gesetzt {
			liste = append(liste, aenderung{"infos." + ia.ID + ".einheit", ohneLeer(ia.Einheit.Wert)})
		}
	}
	return liste, nil
}

// setze schreibt einen geprüften Wert und pflegt die Korrekturen nach den
// Regeln 2 bis 4. Ergebnis: ob sich der Wert geändert hat.
func setze(e *Eintrag, feld string, neu *string, jetzt time.Time) bool {
	alt := lies(e, feld)
	if gleich(alt, neu) {
		return false // Regel 4
	}

	i := slices.IndexFunc(e.Korrekturen, func(k Korrektur) bool { return k.Feld == feld })
	// Gibt es noch keine Korrektur, ist der aktuelle Wert der erkannte.
	erkannt := alt
	if i >= 0 {
		erkannt = e.Korrekturen[i].Erkannt
	}

	schreibe(e, feld, neu)

	switch {
	case gleich(neu, erkannt): // Regel 3: zurückgesetzt
		if i >= 0 {
			e.Korrekturen = slices.Delete(e.Korrekturen, i, i+1)
		}
	case i >= 0: // Regel 2: erkannt bleibt, letzter Stand
		e.Korrekturen[i].Korrigiert = neu
		e.Korrekturen[i].Zeitpunkt = jetzt
	default:
		e.Korrekturen = append(e.Korrekturen, Korrektur{Feld: feld, Erkannt: erkannt, Korrigiert: neu, Zeitpunkt: jetzt})
	}
	return true
}

// lies liefert den aktuellen Wert eines Feldpfads. Die Pfade sind geprüft,
// Infos existieren also.
//
// Wichtig: Für typ und wert eine Kopie zurückgeben, nie &e.Typ. setze merkt
// sich den alten Wert als "erkannt" und überschreibt danach das Feld; ein
// Zeiger ins Feld würde dann auf den neuen Wert zeigen. Die *string-Felder
// (Datum, Quelle, Einheit) sind unkritisch: schreibe ersetzt dort den
// Zeiger, ändert aber nie den Text, auf den er zeigt.
func lies(e *Eintrag, feld string) *string {
	switch feld {
	case "typ":
		typ := e.Typ
		return &typ
	case "dokumentdatum":
		return e.Dokumentdatum
	case "quelle":
		return e.Quelle
	}
	id, teil := infoPfad(feld)
	info := e.Infos[infoIndex(e.Infos, id)]
	if teil == "wert" {
		wert := info.Wert
		return &wert
	}
	return info.Einheit
}

// schreibe setzt einen Feldpfad. Infos ist eine Kopie (Korrigiere), das
// Original bleibt also unberührt. Typ und Wert sind nach der Prüfung nie nil.
func schreibe(e *Eintrag, feld string, neu *string) {
	switch feld {
	case "typ":
		e.Typ = *neu
		return
	case "dokumentdatum":
		e.Dokumentdatum = neu
		return
	case "quelle":
		e.Quelle = neu
		return
	}
	id, teil := infoPfad(feld)
	info := &e.Infos[infoIndex(e.Infos, id)]
	if teil == "wert" {
		info.Wert = *neu
	} else {
		info.Einheit = neu
	}
}

// infoPfad zerlegt "infos.i2.wert" in "i2" und "wert".
func infoPfad(feld string) (id, teil string) {
	rest := strings.TrimPrefix(feld, "infos.")
	i := strings.LastIndexByte(rest, '.')
	return rest[:i], rest[i+1:]
}

func infoIndex(infos []Info, id string) int {
	return slices.IndexFunc(infos, func(i Info) bool { return i.ID == id })
}

// gleich vergleicht zwei optionale Werte: beide nil oder beide derselbe Text.
func gleich(a, b *string) bool {
	if a == nil || b == nil {
		return a == nil && b == nil
	}
	return *a == *b
}
