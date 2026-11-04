// Package konfig liest die Einstellungen des Backends aus Umgebungsvariablen.
//
// Geheimnisse (MongoDB-Zugang, API-Schlüssel) stehen nur in .env und werden
// nie geloggt. Fehlermeldungen nennen deshalb nur Variablennamen, nie Werte.
package konfig

import (
	"errors"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/joho/godotenv"
)

// Standardwerte für optionale Variablen.
const (
	StandardAdresse = "127.0.0.1"
	StandardPort    = "8080"

	// Modelle, geprüft am 04.11.2026 mit go run ./cmd/modelle und der
	// Preisseite (aistudio.google.com/docs/pricing). Beide sind in der
	// kostenlosen Stufe nutzbar; dort dürfen Eingaben zur Verbesserung der
	// Produkte verwendet werden, daher nur synthetische Dokumente.
	// Die genauen Free-Tier-Limits (Anfragen pro Minute/Tag) zeigt nur AI
	// Studio im eigenen Projekt an.
	//
	// gemini-3.8-flash: stabil, Bild (inkl. HEIC) und PDF als Eingabe,
	// JSON-Ausgabe nach Schema. (gemini-2.5-flash ist veraltet und wird ab
	// 01.12.2026 umgeleitet.)
	StandardGeminiModell = "gemini-3.8-flash"

	// gemini-embedding-2: bis 8192 Tokens pro Text, 768 Dimensionen werden
	// automatisch normalisiert. ACHTUNG: Embeddings verschiedener Modelle
	// sind nicht vergleichbar. Wer das Modell wechselt, muss alle Abschnitte
	// neu einbetten, sonst findet die Suche nichts Sinnvolles mehr.
	StandardGeminiEmbeddingModell = "gemini-embedding-2"

	// StandardErsatzReserveSekunden: So viele Sekunden des Limits bleiben für
	// das Ersatzmodell reserviert, beim Lesen (50 s → Hauptmodell 30 s) und
	// beim Antworten (45 s → Hauptmodell 25 s). Erlaubt sind 0 bis 35 (prüft
	// gemini.Neu). Ohne Ersatzmodell gibt es keine Reserve. Grund: Hängt das
	// Hauptmodell bis zur Frist, bliebe für den Ersatz sonst keine Zeit
	// (beobachtet am 04.11.2026).
	StandardErsatzReserveSekunden = 20
)

// Konfig enthält alle Einstellungen, die das Backend beim Start braucht.
type Konfig struct {
	MongoURI              string
	MongoDB               string
	GeminiKey             string
	GeminiModell          string
	GeminiModellErsatz    string        // optional, leer = kein Ersatzmodell
	GeminiErsatzReserve   time.Duration // Teil des Lese-Limits, der dem Ersatz vorbehalten ist
	GeminiEmbeddingModell string
	Adresse               string
	Port                  string
}

// Lauschadresse ergibt z. B. "127.0.0.1:8080". net.JoinHostPort setzt
// IPv6-Adressen korrekt in eckige Klammern.
func (k Konfig) Lauschadresse() string {
	return net.JoinHostPort(k.Adresse, k.Port)
}

// NurLokal ist true, wenn der Server nur vom eigenen Rechner aus erreichbar ist.
func (k Konfig) NurLokal() bool {
	switch k.Adresse {
	case "127.0.0.1", "localhost", "::1":
		return true
	}
	return false
}

// DotenvLaden lädt .env-Dateien in die Umgebung. Gesucht wird in backend/.env
// und im Repo-Wurzelverzeichnis (../.env, wenn der Server aus backend/
// gestartet wird). Eine fehlende Datei ist kein Fehler, denn im Betrieb
// können die Variablen auch direkt gesetzt sein. Bereits gesetzte
// Umgebungsvariablen werden nicht überschrieben, sie haben Vorrang.
func DotenvLaden() error {
	for _, pfad := range []string{".env", "../.env"} {
		err := godotenv.Load(pfad)
		if err != nil && !errors.Is(err, os.ErrNotExist) {
			// Bewusst ohne err-Inhalt der Datei, nur Pfad und Art des Fehlers.
			return fmt.Errorf("%s konnte nicht gelesen werden: %w", pfad, err)
		}
	}
	return nil
}

// DotenvSuchen lädt die erste .env, die es vom aktuellen Ordner aus nach
// oben findet. Für Tests: go test läuft im Paketordner (z. B.
// backend/internal/speicher), die .env liegt im Repo-Wurzelverzeichnis.
// Keine .env zu finden ist kein Fehler.
func DotenvSuchen() error {
	dir, err := os.Getwd()
	if err != nil {
		return err
	}
	for {
		pfad := filepath.Join(dir, ".env")
		if _, err := os.Stat(pfad); err == nil {
			if err := godotenv.Load(pfad); err != nil {
				return fmt.Errorf("%s konnte nicht gelesen werden: %w", pfad, err)
			}
			return nil
		}
		eltern := filepath.Dir(dir)
		if eltern == dir {
			return nil
		}
		dir = eltern
	}
}

// Laden liest die Konfiguration über getenv (im Programm os.Getenv, in Tests
// eine eigene Funktion). Fehlen Pflichtvariablen, nennt der Fehler alle
// fehlenden Namen auf einmal.
func Laden(getenv func(string) string) (Konfig, error) {
	k := Konfig{
		MongoURI:     strings.TrimSpace(getenv("MONGODB_URI")),
		MongoDB:      strings.TrimSpace(getenv("MONGODB_DB")),
		GeminiKey:    strings.TrimSpace(getenv("GEMINI_API_KEY")),
		GeminiModell: mitStandard(getenv("GEMINI_MODELL"), StandardGeminiModell),
		// Ersatz nur fürs Lesen. Für Embeddings gibt es bewusst keinen: Vektoren
		// zweier Modelle sind nicht vergleichbar.
		GeminiModellErsatz: strings.TrimSpace(getenv("GEMINI_MODELL_ERSATZ")),
		GeminiEmbeddingModell: mitStandard(getenv("GEMINI_EMBEDDING_MODELL"),
			StandardGeminiEmbeddingModell),
		Adresse: mitStandard(getenv("ADRESSE"), StandardAdresse),
		Port:    mitStandard(getenv("PORT"), StandardPort),
	}

	var fehlend []string
	if k.MongoURI == "" {
		fehlend = append(fehlend, "MONGODB_URI")
	}
	if k.MongoDB == "" {
		fehlend = append(fehlend, "MONGODB_DB")
	}
	if k.GeminiKey == "" {
		fehlend = append(fehlend, "GEMINI_API_KEY")
	}
	if len(fehlend) > 0 {
		return Konfig{}, fmt.Errorf("Pflichtvariablen fehlen: %s (siehe .env.example)",
			strings.Join(fehlend, ", "))
	}

	if p, err := strconv.Atoi(k.Port); err != nil || p < 1 || p > 65535 {
		return Konfig{}, errors.New("PORT muss eine Zahl zwischen 1 und 65535 sein")
	}

	// Ob die Reserve zum Lese-Limit passt, prüft gemini.Neu (dort ist das
	// Limit bekannt). Hier nur: ganze Zahl, nicht negativ.
	reserve := mitStandard(getenv("GEMINI_ERSATZ_RESERVE_SEKUNDEN"), strconv.Itoa(StandardErsatzReserveSekunden))
	sek, err := strconv.Atoi(reserve)
	if err != nil || sek < 0 {
		return Konfig{}, errors.New("GEMINI_ERSATZ_RESERVE_SEKUNDEN muss eine ganze Zahl ≥ 0 sein")
	}
	k.GeminiErsatzReserve = time.Duration(sek) * time.Second

	return k, nil
}

func mitStandard(wert, standard string) string {
	if w := strings.TrimSpace(wert); w != "" {
		return w
	}
	return standard
}
