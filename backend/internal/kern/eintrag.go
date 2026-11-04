// Package kern enthält die Fachlogik ohne Datenbank, Netz oder HTTP:
// Typen, Modulkatalog, Sortierung und später Prüfregeln. Alles hier ist
// deterministisch und wird mit Unit-Tests abgedeckt.
package kern

import "time"

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
