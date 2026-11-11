package core

import (
	"errors"
	"slices"
	"testing"
)

func TestCheckModule(t *testing.T) {
	for value, want := range map[string]string{"": "gesundheit", "gesundheit": "gesundheit"} {
		got, err := CheckModule(value)
		if err != nil || got != want {
			t.Errorf("CheckModule(%q) = %q, %v", value, got, err)
		}
	}
	// Modules from the app that are not yet valid in the backend are errors.
	for _, value := range []string{"ernaehrung", "sport", "Gesundheit", " gesundheit"} {
		if _, err := CheckModule(value); !errors.Is(err, ErrUnknownModule) {
			t.Errorf("CheckModule(%q): want ErrUnknownModule, got %v", value, err)
		}
	}
}

func TestTypes(t *testing.T) {
	if !TypeAllowed("gesundheit", "laborbefund") {
		t.Error("laborbefund must be allowed in gesundheit")
	}
	if TypeAllowed("gesundheit", "kochrezept") || TypeAllowed("sport", "rezept") {
		t.Error("unknown type or unknown module must not be allowed")
	}
	if len(AllowedTypes("sport")) != 0 {
		t.Error("unknown module must return an empty list of types")
	}

	// AllowedTypes returns a copy; the catalog stays unchanged.
	types := AllowedTypes("gesundheit")
	types[0] = "broken"
	if TypeAllowed("gesundheit", "broken") {
		t.Error("catalog was changed through the returned list")
	}

	if got := Modules(); !slices.Equal(got, []string{"gesundheit"}) {
		t.Errorf("Modules() = %v", got)
	}
}
