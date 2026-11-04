package konfig

import (
	"strings"
	"testing"
)

// umgebung baut eine getenv-Funktion aus einer Map, damit Tests keine echten
// Umgebungsvariablen brauchen.
func umgebung(werte map[string]string) func(string) string {
	return func(name string) string { return werte[name] }
}

func vollstaendig() map[string]string {
	return map[string]string{
		"MONGODB_URI":    "mongodb+srv://nutzer:GEHEIM@beispiel.invalid/",
		"MONGODB_DB":     "lifelog_test",
		"GEMINI_API_KEY": "GEHEIMER-SCHLUESSEL",
	}
}

func TestLaden_Standardwerte(t *testing.T) {
	k, err := Laden(umgebung(vollstaendig()))
	if err != nil {
		t.Fatalf("unerwarteter Fehler: %v", err)
	}
	if got := k.Lauschadresse(); got != "127.0.0.1:8080" {
		t.Errorf("Lauschadresse = %q, erwartet 127.0.0.1:8080", got)
	}
	if !k.NurLokal() {
		t.Error("Standard muss nur lokal erreichbar sein")
	}
}

func TestLaden_AdresseUndPortUeberschreiben(t *testing.T) {
	w := vollstaendig()
	w["ADRESSE"] = "0.0.0.0"
	w["PORT"] = "9090"
	k, err := Laden(umgebung(w))
	if err != nil {
		t.Fatalf("unerwarteter Fehler: %v", err)
	}
	if got := k.Lauschadresse(); got != "0.0.0.0:9090" {
		t.Errorf("Lauschadresse = %q, erwartet 0.0.0.0:9090", got)
	}
	if k.NurLokal() {
		t.Error("0.0.0.0 darf nicht als nur lokal gelten")
	}
}

func TestLaden_LeerzeichenZaehlenAlsLeer(t *testing.T) {
	w := vollstaendig()
	w["ADRESSE"] = "   "
	w["MONGODB_DB"] = "  "
	_, err := Laden(umgebung(w))
	if err == nil || !strings.Contains(err.Error(), "MONGODB_DB") {
		t.Fatalf("erwartet Fehler zu MONGODB_DB, bekommen: %v", err)
	}
}

func TestLaden_FehlendeVariablen(t *testing.T) {
	_, err := Laden(umgebung(map[string]string{}))
	if err == nil {
		t.Fatal("erwartet einen Fehler")
	}
	for _, name := range []string{"MONGODB_URI", "MONGODB_DB", "GEMINI_API_KEY"} {
		if !strings.Contains(err.Error(), name) {
			t.Errorf("Fehlermeldung nennt %s nicht: %q", name, err)
		}
	}
}

func TestLaden_GeheimnisseNieInFehlermeldung(t *testing.T) {
	w := vollstaendig()
	delete(w, "MONGODB_DB") // erzeugt einen Fehler, während Geheimnisse gesetzt sind
	w["PORT"] = "keine-zahl"
	_, err := Laden(umgebung(w))
	if err == nil {
		t.Fatal("erwartet einen Fehler")
	}
	for _, geheim := range []string{"GEHEIM", "GEHEIMER-SCHLUESSEL"} {
		if strings.Contains(err.Error(), geheim) {
			t.Errorf("Fehlermeldung enthält ein Geheimnis: %q", err)
		}
	}

	w = vollstaendig()
	w["PORT"] = "keine-zahl"
	_, err = Laden(umgebung(w))
	if err == nil || strings.Contains(err.Error(), "GEHEIM") {
		t.Errorf("Port-Fehler fehlt oder enthält Geheimnis: %v", err)
	}
}

func TestLaden_UngueltigerPort(t *testing.T) {
	for _, port := range []string{"abc", "0", "70000", "-1"} {
		w := vollstaendig()
		w["PORT"] = port
		if _, err := Laden(umgebung(w)); err == nil {
			t.Errorf("PORT=%q: erwartet Fehler", port)
		}
	}
}
