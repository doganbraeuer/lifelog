package kern

import (
	"errors"
	"slices"
	"testing"
)

func TestPruefeModul(t *testing.T) {
	for wert, erwartet := range map[string]string{"": "gesundheit", "gesundheit": "gesundheit"} {
		got, err := PruefeModul(wert)
		if err != nil || got != erwartet {
			t.Errorf("PruefeModul(%q) = %q, %v", wert, got, err)
		}
	}
	// Module aus der App, die im Backend noch nicht gelten, sind Fehler.
	for _, wert := range []string{"ernaehrung", "sport", "Gesundheit", " gesundheit"} {
		if _, err := PruefeModul(wert); !errors.Is(err, ErrUnbekanntesModul) {
			t.Errorf("PruefeModul(%q): erwartet ErrUnbekanntesModul, bekommen %v", wert, err)
		}
	}
}

func TestTypen(t *testing.T) {
	if !TypErlaubt("gesundheit", "laborbefund") {
		t.Error("laborbefund muss in gesundheit erlaubt sein")
	}
	if TypErlaubt("gesundheit", "kochrezept") || TypErlaubt("sport", "rezept") {
		t.Error("unbekannter Typ oder unbekanntes Modul darf nicht erlaubt sein")
	}
	if len(ErlaubteTypen("sport")) != 0 {
		t.Error("unbekanntes Modul muss leere Typenliste liefern")
	}

	// ErlaubteTypen liefert eine Kopie, der Katalog bleibt unverändert.
	typen := ErlaubteTypen("gesundheit")
	typen[0] = "kaputt"
	if TypErlaubt("gesundheit", "kaputt") {
		t.Error("Katalog wurde über die zurückgegebene Liste verändert")
	}

	if got := Module(); !slices.Equal(got, []string{"gesundheit"}) {
		t.Errorf("Module() = %v", got)
	}
}
