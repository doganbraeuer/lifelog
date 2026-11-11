package core

import (
	"errors"
	"slices"
	"strings"
)

// Sortierung ist das Feld, nach dem die Zeitleiste sortiert wird.
type Sortierung string

const (
	NachDokumentdatum Sortierung = "dokumentdatum"
	NachImportdatum   Sortierung = "importdatum"
)

// Richtung der Sortierung.
type Richtung string

const (
	Absteigend  Richtung = "absteigend"
	Aufsteigend Richtung = "aufsteigend"
)

// ErrUnbekannterWert: Ein Parameter hat einen nicht erlaubten Wert.
var ErrUnbekannterWert = errors.New("unbekannter Wert")

// SortierungAus liest den Query-Parameter "sortierung". Leer ergibt den
// Standard dokumentdatum.
func SortierungAus(wert string) (Sortierung, error) {
	switch Sortierung(wert) {
	case "", NachDokumentdatum:
		return NachDokumentdatum, nil
	case NachImportdatum:
		return NachImportdatum, nil
	}
	return "", ErrUnbekannterWert
}

// RichtungAus liest den Query-Parameter "richtung". Leer ergibt den Standard
// absteigend (neuestes Dokument zuerst).
func RichtungAus(wert string) (Richtung, error) {
	switch Richtung(wert) {
	case "", Absteigend:
		return Absteigend, nil
	case Aufsteigend:
		return Aufsteigend, nil
	}
	return "", ErrUnbekannterWert
}

// Sortiere ordnet die Einträge nach den Regeln aus docs/API.md, 3.2:
//
//  1. Nach dem gewählten Feld in der gewählten Richtung.
//  2. Bei sortierung=dokumentdatum stehen Einträge ohne Datum immer am Ende,
//     egal in welcher Richtung. Untereinander nach importdatum absteigend.
//  3. Gleichstand: importdatum in derselben Richtung, danach id.
//
// Die Reihenfolge ist damit immer eindeutig, auch bei gleichen Daten.
func Sortiere(eintraege []EintragKurz, s Sortierung, r Richtung) {
	slices.SortFunc(eintraege, func(a, b EintragKurz) int {
		return vergleiche(a, b, s, r)
	})
}

// vergleiche liefert <0, wenn a vor b stehen soll, >0 wenn danach.
func vergleiche(a, b EintragKurz, s Sortierung, r Richtung) int {
	if s == NachDokumentdatum {
		switch {
		case a.Dokumentdatum == nil && b.Dokumentdatum == nil:
			// Regel 2: untereinander immer absteigend, unabhängig von r.
			return -vergleicheImportUndID(a, b)
		case a.Dokumentdatum == nil:
			return 1 // a ohne Datum → ans Ende
		case b.Dokumentdatum == nil:
			return -1
		}
		// "YYYY-MM-DD" lässt sich als Text vergleichen: Die Reihenfolge der
		// Zeichen entspricht der zeitlichen Reihenfolge.
		if c := strings.Compare(*a.Dokumentdatum, *b.Dokumentdatum); c != 0 {
			return inRichtung(c, r)
		}
	}
	return inRichtung(vergleicheImportUndID(a, b), r)
}

// vergleicheImportUndID vergleicht aufsteigend nach importdatum, dann id.
func vergleicheImportUndID(a, b EintragKurz) int {
	if c := a.Importdatum.Compare(b.Importdatum); c != 0 {
		return c
	}
	return strings.Compare(a.ID, b.ID)
}

func inRichtung(c int, r Richtung) int {
	if r == Absteigend {
		return -c
	}
	return c
}
