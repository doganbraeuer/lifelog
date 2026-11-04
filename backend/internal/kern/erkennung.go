package kern

import (
	"errors"
	"fmt"
	"strings"
	"time"
)

// Erkennung ist das, was ein Modell aus einem Dokument gelesen hat, noch
// ungeprüft. Die Modell-Schnittstelle (Paket modell) liefert diesen Typ,
// egal welcher Anbieter dahintersteckt.
type Erkennung struct {
	Lesbar        bool    // false: kein Dokument erkennbar (z. B. unscharfes Foto)
	Dokumentdatum *string // soll "YYYY-MM-DD" sein
	Quelle        *string
	Typ           string
	Volltext      string
	Infos         []ErkannteInfo
}

// ErkannteInfo ist eine Angabe, wie das Modell sie liefert (noch ohne ID).
type ErkannteInfo struct {
	Name    string
	Wert    string
	Einheit *string
}

// ErrNichtLesbar: Das Modell erkennt kein Dokument (→ 422 NICHT_LESBAR).
var ErrNichtLesbar = errors.New("kein Dokument erkennbar")

// ErrErkennungUngueltig: Die Antwort des Modells passt nicht zu den Regeln
// (→ 502 AUSLESEN_FEHLGESCHLAGEN).
var ErrErkennungUngueltig = errors.New("Erkennung ungültig")

// ErkennungFehler sagt, welches Feld nicht passt. Er enthält bewusst nie den
// Wert selbst, weil er im Log landet und Werte Gesundheitsdaten sein können.
type ErkennungFehler struct {
	Feld  string // z. B. "dokumentdatum" oder "infos[2].name"
	Grund string
}

func (f *ErkennungFehler) Error() string {
	return fmt.Sprintf("Erkennung ungültig: %s: %s", f.Feld, f.Grund)
}

// Is sorgt dafür, dass errors.Is(err, ErrErkennungUngueltig) zutrifft.
func (f *ErkennungFehler) Is(ziel error) bool { return ziel == ErrErkennungUngueltig }

// ErrDatumUngueltig: Ein Datum ist nicht im Format YYYY-MM-DD oder kein
// echtes Kalenderdatum.
var ErrDatumUngueltig = errors.New("Datum muss im Format JJJJ-MM-TT ein gültiges Kalenderdatum sein")

// PruefeDatum prüft ein Dokumentdatum: genau "YYYY-MM-DD" mit führenden
// Nullen, und das Datum muss es geben (kein 31.02.). Wird auch für PATCH
// gebraucht (API.md 3.5).
func PruefeDatum(datum string) error {
	if len(datum) != len("2006-01-02") {
		return ErrDatumUngueltig
	}
	// time.Parse lehnt Tage außerhalb des Monats ab, z. B. 2026-02-30.
	if _, err := time.Parse("2006-01-02", datum); err != nil {
		return ErrDatumUngueltig
	}
	return nil
}

// PruefeErkennung prüft die Antwort des Modells und macht daraus einen
// Eintrag für das Modul. Gesetzt werden Modul, Typ, Dokumentdatum, Quelle,
// Volltext, Infos (mit IDs i1, i2, …) und eine leere Korrekturliste.
// ID, Importdatum und Datei setzt danach der Aufrufer.
//
// Regeln:
//   - Lesbar=false ergibt ErrNichtLesbar.
//   - typ muss im Modul erlaubt sein.
//   - volltext darf nicht leer sein. Er wird nicht verändert.
//   - dokumentdatum: leer heißt „nicht erkannt“ (nil), sonst PruefeDatum.
//   - quelle und einheit: leer heißt nil.
//   - Eine Info ohne name oder wert wird weggelassen, nicht abgelehnt
//     (API.md 3.1, Schritt 4). Die übrigen bekommen lückenlose IDs.
//     Leerraum am Rand wird entfernt, der Wert bleibt sonst genau wie auf
//     dem Dokument ("<0,5" bleibt "<0,5").
//
// ausgelassen nennt die fehlenden Felder der weggelassenen Infos, z. B.
// "infos[2].wert". Der Index bezieht sich auf die Liste des Modells. Die
// Liste enthält nie Werte, der Aufrufer darf sie also ins Log schreiben.
// Sie ist nie nil.
func PruefeErkennung(e Erkennung, modul string) (eintrag Eintrag, ausgelassen []string, err error) {
	ausgelassen = []string{}
	if !e.Lesbar {
		return Eintrag{}, ausgelassen, ErrNichtLesbar
	}
	if !TypErlaubt(modul, e.Typ) {
		return Eintrag{}, ausgelassen, &ErkennungFehler{Feld: "typ", Grund: "im Modul " + modul + " nicht erlaubt"}
	}
	if strings.TrimSpace(e.Volltext) == "" {
		return Eintrag{}, ausgelassen, &ErkennungFehler{Feld: "volltext", Grund: "leer"}
	}

	datum := ohneLeer(e.Dokumentdatum)
	if datum != nil {
		if err := PruefeDatum(*datum); err != nil {
			return Eintrag{}, ausgelassen, &ErkennungFehler{Feld: "dokumentdatum", Grund: "kein gültiges Datum im Format JJJJ-MM-TT"}
		}
	}

	infos := make([]Info, 0, len(e.Infos))
	for i, ei := range e.Infos {
		name := strings.TrimSpace(ei.Name)
		wert := strings.TrimSpace(ei.Wert)
		if name == "" || wert == "" {
			if name == "" {
				ausgelassen = append(ausgelassen, fmt.Sprintf("infos[%d].name", i))
			}
			if wert == "" {
				ausgelassen = append(ausgelassen, fmt.Sprintf("infos[%d].wert", i))
			}
			continue
		}
		infos = append(infos, Info{
			ID:      fmt.Sprintf("i%d", len(infos)+1), // lückenlos, auch nach Auslassungen
			Name:    name,
			Wert:    wert,
			Einheit: ohneLeer(ei.Einheit),
		})
	}

	return Eintrag{
		Modul:         modul,
		Typ:           e.Typ,
		Dokumentdatum: datum,
		Quelle:        ohneLeer(e.Quelle),
		Volltext:      e.Volltext,
		Infos:         infos,
		Korrekturen:   []Korrektur{},
	}, ausgelassen, nil
}

// ohneLeer entfernt Leerraum am Rand. Bleibt nichts übrig, ergibt das nil.
func ohneLeer(s *string) *string {
	if s == nil {
		return nil
	}
	t := strings.TrimSpace(*s)
	if t == "" {
		return nil
	}
	return &t
}
