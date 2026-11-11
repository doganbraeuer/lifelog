package core

import (
	"errors"
	"slices"
	"testing"
)

func TestCheckModule(t *testing.T) {
	for value, want := range map[string]string{"": "health", "health": "health"} {
		got, err := CheckModule(value)
		if err != nil || got != want {
			t.Errorf("CheckModule(%q) = %q, %v", value, got, err)
		}
	}
	// Modules from the app that are not yet valid in the backend are errors.
	for _, value := range []string{"nutrition", "sport", "Health", " health", "gesundheit"} {
		if _, err := CheckModule(value); !errors.Is(err, ErrUnknownModule) {
			t.Errorf("CheckModule(%q): want ErrUnknownModule, got %v", value, err)
		}
	}
}

func TestTypes(t *testing.T) {
	if !TypeAllowed("health", "lab_report") {
		t.Error("lab_report must be allowed in health")
	}
	if TypeAllowed("health", "kochrezept") || TypeAllowed("sport", "prescription") {
		t.Error("unknown type or unknown module must not be allowed")
	}
	if len(AllowedTypes("sport")) != 0 {
		t.Error("unknown module must return an empty list of types")
	}

	// AllowedTypes returns a copy; the catalog stays unchanged.
	types := AllowedTypes("health")
	types[0] = "broken"
	if TypeAllowed("health", "broken") {
		t.Error("catalog was changed through the returned list")
	}

	if got := Modules(); !slices.Equal(got, []string{"health"}) {
		t.Errorf("Modules() = %v", got)
	}
}

func TestTypeName_EveryTypeHasAName(t *testing.T) {
	for _, module := range Modules() {
		for _, typ := range AllowedTypes(module) {
			if name := TypeName(module, typ); name == typ || name == "" {
				t.Errorf("type %s in %s has no display name", typ, module)
			}
		}
	}
	if TypeName("health", "lab_report") != "Laborbefund" {
		t.Error("lab_report must be called Laborbefund, as in the app")
	}
	if TypeName("health", "x_ray") != "x_ray" {
		t.Error("unknown types keep their code")
	}
}
