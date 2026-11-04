package kern

import (
	"errors"
	"slices"
	"sort"
)

// StandardModul gilt, wenn eine Anfrage kein Modul nennt.
const StandardModul = "gesundheit"

// modulTypen ist der Modulkatalog: welche Module es gibt und welche Typen
// innerhalb jedes Moduls erlaubt sind. Ein Typ gilt nur in seinem Modul, ein
// späteres Kochrezept-Modul darf also wieder einen Typ "rezept" haben.
//
// Weitere Module (ernaehrung, rezepte, sport) kommen erst dazu, wenn ihre
// Typen feststehen (docs/API.md, Abschnitt „Module“).
var modulTypen = map[string][]string{
	"gesundheit": {"arztbrief", "laborbefund", "rezept", "sonstiges"},
}

// ErrUnbekanntesModul: Der Wert ist kein bekanntes Modul.
var ErrUnbekanntesModul = errors.New("unbekanntes Modul")

// PruefeModul gibt das Modul für eine Anfrage zurück. Ein leerer Wert ergibt
// das Standardmodul. Groß- und Kleinschreibung zählen: "Gesundheit" ist
// unbekannt.
func PruefeModul(wert string) (string, error) {
	if wert == "" {
		return StandardModul, nil
	}
	if _, ok := modulTypen[wert]; !ok {
		return "", ErrUnbekanntesModul
	}
	return wert, nil
}

// ErlaubteTypen liefert die Typen eines Moduls (Kopie, damit niemand den
// Katalog verändert). Für ein unbekanntes Modul ist das Ergebnis leer.
func ErlaubteTypen(modul string) []string {
	return slices.Clone(modulTypen[modul])
}

// TypErlaubt prüft, ob typ im Modul vorkommt.
func TypErlaubt(modul, typ string) bool {
	return slices.Contains(modulTypen[modul], typ)
}

// Module liefert alle bekannten Module, alphabetisch sortiert.
func Module() []string {
	m := make([]string, 0, len(modulTypen))
	for name := range modulTypen {
		m = append(m, name)
	}
	sort.Strings(m)
	return m
}
