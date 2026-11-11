// Package gemini setzt die Schnittstellen aus internal/modell mit Googles
// Gemini-API um. Nur dieses Paket kennt das Gemini-SDK.
package gemini

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"time"

	"google.golang.org/genai"

	"lifelog/backend/internal/core"
	"lifelog/backend/internal/llm"
)

// Gesamtlimits je Vorgang, inklusive Wiederholungen und Ersatzmodell.
//
//	Upload (App wartet 90 s): Lesen 50 s + Abschnitte einbetten 20 s
//	Frage  (App wartet 60 s): Frage einbetten 10 s + Antworten 45 s
const (
	leseTimeout           = 50 * time.Second
	einbettenTimeout      = 20 * time.Second
	frageEinbettenTimeout = 10 * time.Second
	antwortTimeout        = 45 * time.Second

	// mindestHauptzeit: So viel Zeit bleibt dem Hauptmodell mindestens,
	// egal wie groß die Reserve für den Ersatz eingestellt ist.
	mindestHauptzeit = 10 * time.Second
)

// maxErsatzReserve: Die Reserve gilt fürs Lesen und fürs Antworten; sie muss
// zum kürzeren der beiden Limits passen.
const maxErsatzReserve = min(leseTimeout, antwortTimeout) - mindestHauptzeit

// Einstellungen für Neu.
type Einstellungen struct {
	Schluessel      string
	Modell          string // Hauptmodell zum Lesen
	ErsatzModell    string // optional: wird einmal versucht, wenn Modell nicht erreichbar bleibt
	EmbeddingModell string // ohne Ersatz: Embeddings verschiedener Modelle passen nicht zusammen

	// ErsatzReserve: Dieser Teil des Limits (Lesen und Antworten) ist dem
	// Ersatzmodell vorbehalten. Das Hauptmodell muss samt Wiederholungen
	// vorher fertig sein. Gilt nur, wenn ErsatzModell gesetzt ist.
	ErsatzReserve time.Duration
}

// Die beiden Aufrufe, die wirklich ins Netz gehen. Als Funktionen, damit
// Tests sie ersetzen und Wiederholung und Ersatzmodell ohne Netz prüfen
// können.
type (
	erzeugenFunk  func(ctx context.Context, modell string, inhalte []*genai.Content, cfg *genai.GenerateContentConfig) (*genai.GenerateContentResponse, error)
	einbettenFunk func(ctx context.Context, modell string, inhalte []*genai.Content, cfg *genai.EmbedContentConfig) (*genai.EmbedContentResponse, error)
)

// Gemini ist die Verbindung zur Gemini-API.
type Gemini struct {
	e              Einstellungen
	erzeugen       erzeugenFunk
	einbettenAPI   einbettenFunk
	neuWiederholer func(beschreibung string) wiederholer
	leseTimeout    time.Duration // Felder statt Konstanten, damit Tests nicht 50 s warten
	antwortTimeout time.Duration
}

var (
	_ llm.Leser     = (*Gemini)(nil)
	_ llm.Einbetter = (*Gemini)(nil)
	_ llm.Antworter = (*Gemini)(nil)
)

// Neu erstellt den Client. Es findet noch kein Netzwerkaufruf statt.
func Neu(ctx context.Context, e Einstellungen) (*Gemini, error) {
	client, err := genai.NewClient(ctx, &genai.ClientConfig{
		APIKey:  e.Schluessel,
		Backend: genai.BackendGeminiAPI,
		// Keine RetryOptions im SDK: Wiederholungen steuern wir selbst
		// (wiederholung.go), damit sie nur bei 429/503 greifen und das
		// Gesamtlimit einhalten.
	})
	if err != nil {
		return nil, uebersetzeFehler(err, e.Schluessel)
	}
	if e.ErsatzModell == e.Modell {
		e.ErsatzModell = "" // derselbe Name wäre nur ein vierter Versuch
	}
	if e.ErsatzReserve < 0 || e.ErsatzReserve > maxErsatzReserve {
		return nil, fmt.Errorf("GEMINI_FALLBACK_RESERVE_SECONDS muss zwischen 0 und %d liegen (Limits: Lesen %v, Antworten %v; Hauptmodell braucht mindestens %v)",
			int(maxErsatzReserve.Seconds()), leseTimeout, antwortTimeout, mindestHauptzeit)
	}
	return &Gemini{
		e:              e,
		erzeugen:       client.Models.GenerateContent,
		einbettenAPI:   client.Models.EmbedContent,
		neuWiederholer: standardWiederholer,
		leseTimeout:    leseTimeout,
		antwortTimeout: antwortTimeout,
	}, nil
}

// DokumentLesen schickt das Dokument an das Vision-LLM und liefert die
// ungeprüfte Erkennung. Wiederholung und Ersatz: siehe erzeugenMitErsatz.
func (g *Gemini) DokumentLesen(ctx context.Context, daten []byte, mimeTyp string, erlaubteTypen []string) (core.Extraction, error) {
	inhalte, cfg := leseAnfrage(daten, mimeTyp, erlaubteTypen)
	resp, err := g.erzeugenMitErsatz(ctx, g.leseTimeout, "Lesen", inhalte, cfg)
	if err != nil {
		return core.Extraction{}, err
	}
	return erkennungAusAntwort(resp)
}

// TextLesen ordnet einen eingegebenen Text ein (Text-Import, API.md 3.1):
// Datum, Quelle, Typ und Infos. Volltext bleibt leer und Lesbar ist true;
// der Volltext ist der eingegebene Text, den setzt der Aufrufer. Gleiche
// Wiederholung, gleicher Ersatz und gleiches Limit wie DokumentLesen.
func (g *Gemini) TextLesen(ctx context.Context, text string, erlaubteTypen []string) (core.Extraction, error) {
	inhalte, cfg := textAnfrage(text, erlaubteTypen)
	resp, err := g.erzeugenMitErsatz(ctx, g.leseTimeout, "Text lesen", inhalte, cfg)
	if err != nil {
		return core.Extraction{}, err
	}
	antwort, err := textAusAntwort(resp)
	if err != nil {
		return core.Extraction{}, err
	}
	return textErkennungAusText(antwort)
}

// Antworten beantwortet eine Frage nur aus den Fundstellen (Schritt 5).
// Gleiche Wiederholung und gleicher Ersatz wie beim Lesen.
func (g *Gemini) Antworten(ctx context.Context, frage string, fundstellen []core.Source) (string, error) {
	inhalte, cfg := antwortAnfrage(frage, fundstellen)
	resp, err := g.erzeugenMitErsatz(ctx, g.antwortTimeout, "Antworten", inhalte, cfg)
	if err != nil {
		return "", err
	}
	return textAusAntwort(resp)
}

// erzeugenMitErsatz ist der gemeinsame Ablauf fürs Lesen und Antworten:
// Hauptmodell, bei 429/503 bis zu zwei Wiederholungen. Bleibt es nicht
// erreichbar und ist ein Ersatzmodell eingestellt, wird dieses einmal
// versucht. Alles innerhalb von limit.
//
// Zeitaufteilung mit Ersatzmodell (Lesen, Standard 30 s + 20 s):
//
//	|---- Hauptmodell + Wiederholungen ----|---- Reserve: Ersatz ----|
//	0                                     30 s                     50 s
//
// Ohne diese Aufteilung könnte ein hängendes Hauptmodell das ganze Limit
// verbrauchen, und der Ersatz käme nie zum Zug.
func (g *Gemini) erzeugenMitErsatz(ctx context.Context, limit time.Duration, aufgabe string,
	inhalte []*genai.Content, cfg *genai.GenerateContentConfig) (*genai.GenerateContentResponse, error) {
	ctx, abbrechen := context.WithTimeout(ctx, limit)
	defer abbrechen()

	hauptCtx := ctx
	if g.e.ErsatzModell != "" && g.e.ErsatzReserve > 0 {
		frist, _ := ctx.Deadline() // durch WithTimeout immer gesetzt
		var hauptAbbrechen context.CancelFunc
		hauptCtx, hauptAbbrechen = context.WithDeadline(ctx, frist.Add(-g.e.ErsatzReserve))
		defer hauptAbbrechen()
	}

	var resp *genai.GenerateContentResponse
	err := g.neuWiederholer(aufgabe+" "+g.e.Modell).ausfuehren(hauptCtx, func(ctx context.Context) error {
		var err error
		resp, err = g.erzeugen(ctx, g.e.Modell, inhalte, cfg)
		return err
	})
	err = uebersetzeFehler(err, g.e.Schluessel)

	// ctx.Err() == nil: Das Gesamtlimit läuft noch und die App ist noch da.
	// (Abgelaufen sein darf nur hauptCtx, dann ist die Reserve übrig.)
	if errors.Is(err, llm.ErrNichtErreichbar) && g.e.ErsatzModell != "" && ctx.Err() == nil {
		rest := time.Duration(0)
		if frist, ok := ctx.Deadline(); ok {
			rest = time.Until(frist).Round(time.Millisecond)
		}
		// Der Fehler des Hauptmodells ist schon übersetzt und maskiert.
		slog.Warn("Hauptmodell nicht erreichbar, Ersatzmodell wird einmal versucht",
			"aufgabe", aufgabe, "modell", g.e.Modell, "fehler", err, "ersatz", g.e.ErsatzModell, "restzeit", rest)
		resp, err = g.erzeugen(ctx, g.e.ErsatzModell, inhalte, cfg)
		err = uebersetzeFehler(err, g.e.Schluessel)
	}
	return resp, err
}

// leseAnfrage baut Inhalt und Konfiguration für das Lesen eines Dokuments.
// Eigene Funktion, damit der Diagnose-Test (diagnose_test.go) genau
// dieselbe Anfrage schickt wie der Server.
func leseAnfrage(daten []byte, mimeTyp string, erlaubteTypen []string) ([]*genai.Content, *genai.GenerateContentConfig) {
	inhalte := []*genai.Content{genai.NewContentFromParts([]*genai.Part{
		genai.NewPartFromBytes(daten, mimeTyp),
		genai.NewPartFromText("Übertrage dieses Dokument nach den Regeln in JSON."),
	}, genai.RoleUser)}
	cfg := &genai.GenerateContentConfig{
		SystemInstruction:  genai.NewContentFromText(leseAnweisung(erlaubteTypen), genai.RoleUser),
		ResponseMIMEType:   "application/json",
		ResponseJsonSchema: leseSchema(erlaubteTypen),
		// Abschreiben braucht wenig „Nachdenken“. Im AI Studio Playground
		// (04.11.2026, gleiches PDF): LOW 3,4 s, MEDIUM 5,9 s.
		ThinkingConfig: &genai.ThinkingConfig{ThinkingLevel: genai.ThinkingLevelLow},
	}
	return inhalte, cfg
}

// textAnfrage baut Inhalt und Konfiguration für den Text-Import. Der Text
// ist ein eigener Teil der Nachricht, getrennt vom Auftrag; die
// Systemanweisung sagt, dass er Inhalt ist und keine Anweisung.
func textAnfrage(text string, erlaubteTypen []string) ([]*genai.Content, *genai.GenerateContentConfig) {
	inhalte := []*genai.Content{genai.NewContentFromParts([]*genai.Part{
		genai.NewPartFromText("Ordne den folgenden Text nach den Regeln in JSON ein."),
		genai.NewPartFromText(text),
	}, genai.RoleUser)}
	cfg := &genai.GenerateContentConfig{
		SystemInstruction:  genai.NewContentFromText(textAnweisung(erlaubteTypen), genai.RoleUser),
		ResponseMIMEType:   "application/json",
		ResponseJsonSchema: textSchema(erlaubteTypen),
		ThinkingConfig:     &genai.ThinkingConfig{ThinkingLevel: genai.ThinkingLevelLow},
	}
	return inhalte, cfg
}

// AbschnitteEinbetten berechnet einen Vektor pro Abschnitt, in Stapeln.
// Das Gesamtlimit gilt für alle Stapel zusammen.
func (g *Gemini) AbschnitteEinbetten(ctx context.Context, texte []string) ([][]float32, error) {
	ctx, abbrechen := context.WithTimeout(ctx, einbettenTimeout)
	defer abbrechen()

	vektoren := make([][]float32, 0, len(texte))
	for _, b := range stapel(len(texte), stapelGroesse) {
		eingaben := make([]string, 0, b[1]-b[0])
		for _, t := range texte[b[0]:b[1]] {
			eingaben = append(eingaben, dokumentEingabe(t))
		}
		v, err := g.einbetten(ctx, eingaben)
		if err != nil {
			return nil, err
		}
		vektoren = append(vektoren, v...)
	}
	return vektoren, nil
}

// FrageEinbetten berechnet den Vektor für eine Suchfrage. Wiederholung bei
// 429/503 ja, Ersatzmodell nie (Vektorräume verschiedener Modelle passen
// nicht zusammen).
func (g *Gemini) FrageEinbetten(ctx context.Context, frage string) ([]float32, error) {
	ctx, abbrechen := context.WithTimeout(ctx, frageEinbettenTimeout)
	defer abbrechen()

	v, err := g.einbetten(ctx, []string{frageEingabe(frage)})
	if err != nil {
		return nil, err
	}
	return v[0], nil
}

// einbetten: ein Aufruf mit Wiederholung bei 429/503, ohne Ersatzmodell.
func (g *Gemini) einbetten(ctx context.Context, eingaben []string) ([][]float32, error) {
	dimensionen := int32(llm.EmbeddingDimensionen)
	cfg := &genai.EmbedContentConfig{OutputDimensionality: &dimensionen}
	inhalte := embeddingInhalte(eingaben)

	var resp *genai.EmbedContentResponse
	err := g.neuWiederholer("Einbetten "+g.e.EmbeddingModell).ausfuehren(ctx, func(ctx context.Context) error {
		var err error
		resp, err = g.einbettenAPI(ctx, g.e.EmbeddingModell, inhalte, cfg)
		return err
	})
	if err != nil {
		return nil, uebersetzeFehler(err, g.e.Schluessel)
	}
	return vektorenAusAntwort(resp, len(eingaben))
}
