// Package kern enthält die Fachlogik ohne Datenbank, Netz oder HTTP:
// Typen, Modulkatalog, Sortierung und später Prüfregeln. Alles hier ist
// deterministisch und wird mit Unit-Tests abgedeckt.
package kern

import (
	"errors"
	"time"
)

// ErrDuplikat: Es gibt schon einen Eintrag mit genau dieser Datei (gleicher
// SHA-256-Hash). Der Speicher meldet ihn, wenn der eindeutige Index
// anschlägt.
var ErrDuplikat = errors.New("Duplikat: Datei existiert bereits")

// ErrEintragNichtGefunden: Es gibt keinen Eintrag mit dieser ID
// (→ 404 EINTRAG_NICHT_GEFUNDEN).
var ErrEintragNichtGefunden = errors.New("Eintrag nicht gefunden")

// ErrUngueltigeID: Die ID ist keine ObjectId (24 Hex-Zeichen).
var ErrUngueltigeID = errors.New("ungültige ID")

// ErrOriginalFehlt: Der Eintrag existiert, seine Originaldatei aber nicht.
// Darf nicht vorkommen (Datei wird vor dem Eintrag gespeichert und nach
// ihm gelöscht); wenn doch, ist das ein interner Fehler.
var ErrOriginalFehlt = errors.New("Originaldatei fehlt")

// PruefeID prüft, ob id wie eine ObjectId aussieht: genau 24 Hex-Zeichen.
// So antwortet die API auf "/eintraege/abc" mit 400 statt mit 404, und die
// Datenbank wird gar nicht erst gefragt.
func PruefeID(id string) error {
	if len(id) != 24 {
		return ErrUngueltigeID
	}
	for _, r := range id {
		if !('0' <= r && r <= '9' || 'a' <= r && r <= 'f' || 'A' <= r && r <= 'F') {
			return ErrUngueltigeID
		}
	}
	return nil
}

// Eintrag ist ein vollständiger Eintrag nach docs/API.md, Abschnitt 2.
type Eintrag struct {
	ID            string    // ObjectId als Hex-String (24 Zeichen)
	Modul         string    // z. B. "gesundheit"
	Typ           string    // z. B. "laborbefund", gilt innerhalb des Moduls
	Dokumentdatum *string   // "YYYY-MM-DD" oder nil, wenn nicht erkannt
	Importdatum   time.Time // UTC
	Quelle        *string   // ausstellende Stelle oder nil
	Volltext      string
	Infos         []Info
	Datei         Datei
	Korrekturen   []Korrektur
}

// Info ist eine extrahierte Angabe. Wert bleibt immer ein String, genau wie
// auf dem Dokument ("14,2", "<0,5"), damit beim Umwandeln nichts verloren geht.
type Info struct {
	ID      string // "i1", "i2", …
	Name    string
	Wert    string
	Einheit *string // nil, wenn das Dokument keine Einheit nennt
}

// Datei verweist auf die gespeicherte Originaldatei.
type Datei struct {
	ID        string
	Dateiname string
	MimeTyp   string
	Groesse   int64
	Sha256    string
}

// Korrektur hält den ursprünglich erkannten und den korrigierten Wert fest.
type Korrektur struct {
	Feld       string
	Erkannt    *string
	Korrigiert *string
	Zeitpunkt  time.Time
}

// EintragKurz ist die Kurzform für die Zeitleiste (ohne Volltext, Infos usw.).
type EintragKurz struct {
	ID            string
	Modul         string
	Typ           string
	Dokumentdatum *string
	Importdatum   time.Time
	Quelle        *string
}

// Abschnitt ist ein Stück Volltext mit seinem Embedding, die Grundlage der
// Suche (POST /fragen). Modul steht im Abschnitt selbst, damit die
// Vektorsuche direkt danach filtern kann.
type Abschnitt struct {
	EintragID string
	Modul     string
	Nr        int // Reihenfolge im Volltext, ab 1
	Text      string
	Embedding []float32
}
