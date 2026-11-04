package konfig

import "testing"

func TestMaskiere(t *testing.T) {
	faelle := []struct {
		text, erwartet string
		geheim         []string
	}{
		{"Fehler mit Schlüssel ABC123 drin", "Fehler mit Schlüssel *** drin", []string{"ABC123"}},
		{"ABC123/ABC123", "***/***", []string{"ABC123"}},
		{"nichts Geheimes", "nichts Geheimes", []string{"ABC123"}},
		{"uri pw und key", "uri *** und ***", []string{"pw", "key"}},
		{"leeres Geheimnis ändert nichts", "leeres Geheimnis ändert nichts", []string{""}},
		{"ohne Geheimnisse", "ohne Geheimnisse", nil},
	}
	for _, f := range faelle {
		if got := Maskiere(f.text, f.geheim...); got != f.erwartet {
			t.Errorf("Maskiere(%q, %q) = %q, erwartet %q", f.text, f.geheim, got, f.erwartet)
		}
	}
}
