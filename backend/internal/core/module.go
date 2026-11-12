package core

import (
	"errors"
	"slices"
	"sort"
)

// DefaultModule applies when a request names no module.
const DefaultModule = "health"

// moduleTypes is the module catalog: which modules exist and which types are
// allowed within each module. A type is only valid in its module, so another
// module may use the same type code with a different meaning (e.g. "other").
//
// Further modules (nutrition, recipes, sport) are added once their types are
// defined (docs/API.md, section "Modules").
var moduleTypes = map[string][]string{
	"health": {"doctor_letter", "lab_report", "prescription", "other"},
}

// typeNames are the German display names of the types, exactly as the app
// shows them (app/lib/module/modul_katalog.dart). They are user-facing and
// therefore German. The answer prompt uses them, so that answers name a
// document the same way as the app ("Laborbefund", not "Laborbericht").
var typeNames = map[string]map[string]string{
	"health": {
		"doctor_letter": "Arztbrief",
		"lab_report":    "Laborbefund",
		"prescription":  "Rezept",
		"other":         "Sonstiges",
	},
}

// TypeName returns the German display name of a type in its module. For an
// unknown type it returns the code itself, so nothing is lost.
func TypeName(module, typ string) string {
	if name, ok := typeNames[module][typ]; ok {
		return name
	}
	return typ
}

// ErrUnknownModule: the value is not a known module.
var ErrUnknownModule = errors.New("unknown module")

// CheckModule returns the module for a request. An empty value gives the
// default module. Case matters: "Health" is unknown.
func CheckModule(value string) (string, error) {
	if value == "" {
		return DefaultModule, nil
	}
	if _, ok := moduleTypes[value]; !ok {
		return "", ErrUnknownModule
	}
	return value, nil
}

// AllowedTypes returns the types of a module (a copy, so nobody changes the
// catalog). For an unknown module the result is empty.
func AllowedTypes(module string) []string {
	return slices.Clone(moduleTypes[module])
}

// TypeAllowed checks whether typ exists in the module.
func TypeAllowed(module, typ string) bool {
	return slices.Contains(moduleTypes[module], typ)
}

// Modules returns all known modules, sorted alphabetically.
func Modules() []string {
	m := make([]string, 0, len(moduleTypes))
	for name := range moduleTypes {
		m = append(m, name)
	}
	sort.Strings(m)
	return m
}
