package dienst

import (
	"context"
	"log/slog"

	"lifelog/backend/internal/core"
	"lifelog/backend/internal/llm"
)

// FragenSpeicher ist, was die Suche von der Datenbank braucht. Umgesetzt
// von speicher.Speicher.
type FragenSpeicher interface {
	AehnlicheAbschnitte(ctx context.Context, vektor []float32, modul string, k int) ([]core.Hit, error)
	EintraegeKurzNachIDs(ctx context.Context, ids []string) (map[string]core.EntrySummary, error)
}

// Fragen beantwortet Fragen zu den Dokumenten (docs/API.md 3.6).
type Fragen struct {
	speicher  FragenSpeicher
	einbetter llm.Einbetter
	antworter llm.Antworter
}

func NeuFragen(s FragenSpeicher, e llm.Einbetter, a llm.Antworter) *Fragen {
	return &Fragen{speicher: s, einbetter: e, antworter: a}
}

// Ausfuehren:
//
//  1. Frage und Modul prüfen.
//  2. Frage einbetten, ähnlichste Abschnitte des Moduls suchen.
//  3. Erste Schicht: Treffer unter core.MinScore verwerfen, ebenso
//     Abschnitte, deren Eintrag es nicht (mehr) gibt.
//  4. Bleibt nichts übrig: fester Satz, OHNE Aufruf des Sprachmodells.
//  5. Sonst: Sprachmodell mit Frage und nummerierten Fundstellen.
//  6. Zweite Schicht: core.BaueAntwort. Ohne gültigen Verweis gilt die
//     Antwort als „nichts gefunden“.
//
// Ins Log kommen weder Frage noch Abschnittstexte, nur Zahlen.
//
// Fehler: core.ErrUnbekanntesModul, core.ErrFrageFehlt, core.ErrFrageZuLang,
// llm.ErrNichtErreichbar, llm.ErrAntwortUngueltig oder intern.
func (f *Fragen) Ausfuehren(ctx context.Context, frage, modul string) (core.Answer, error) {
	modul, err := core.CheckModule(modul)
	if err != nil {
		return core.Answer{}, err
	}
	frage, err = core.CheckQuestion(frage)
	if err != nil {
		return core.Answer{}, err
	}

	vektor, err := f.einbetter.FrageEinbetten(ctx, frage)
	if err != nil {
		return core.Answer{}, err
	}
	alle, err := f.speicher.AehnlicheAbschnitte(ctx, vektor, modul, core.MaxHits)
	if err != nil {
		return core.Answer{}, err
	}
	treffer := core.AboveThreshold(alle, core.MinScore)

	fundstellen, err := f.fundstellen(ctx, treffer, modul)
	if err != nil {
		return core.Answer{}, err
	}
	slog.Info("Frage", "modul", modul, "treffer", len(alle), "ueberSchwelle", len(treffer),
		"fundstellen", len(fundstellen), "besterScore", besterScore(alle))
	if len(fundstellen) == 0 {
		return core.NoHit(), nil
	}

	text, err := f.antworter.Antworten(ctx, frage, fundstellen)
	if err != nil {
		return core.Answer{}, err
	}
	antwort := core.BuildAnswer(text, fundstellen)
	if !antwort.Found {
		slog.Info("Antwort ohne gültigen Verweis, fester Satz", "fundstellen", len(fundstellen))
	}
	return antwort, nil
}

// fundstellen nummeriert die Treffer (1, 2, …) und ergänzt die aktuellen
// Angaben des Eintrags: Typ, Datum und Quelle auch dann, wenn sie
// korrigiert wurden (API.md 3.6). Treffer ohne Eintrag fallen weg; das kann
// kurz während eines Uploads oder Löschens vorkommen.
func (f *Fragen) fundstellen(ctx context.Context, treffer []core.Hit, modul string) ([]core.Source, error) {
	if len(treffer) == 0 {
		return []core.Source{}, nil
	}
	ids := make([]string, 0, len(treffer))
	for _, t := range treffer {
		ids = append(ids, t.EntryID)
	}
	eintraege, err := f.speicher.EintraegeKurzNachIDs(ctx, ids)
	if err != nil {
		return nil, err
	}

	fundstellen := make([]core.Source, 0, len(treffer))
	for _, t := range treffer {
		e, ok := eintraege[t.EntryID]
		if !ok {
			continue // verwaister Abschnitt
		}
		fundstellen = append(fundstellen, core.Source{
			Number:       len(fundstellen) + 1,
			EntryID:      t.EntryID,
			Module:       modul,
			Type:         e.Type,
			Issuer:       e.Issuer,
			DocumentDate: e.DocumentDate,
			Excerpt:      t.Text,
		})
	}
	return fundstellen, nil
}

func besterScore(treffer []core.Hit) float64 {
	if len(treffer) == 0 {
		return 0
	}
	return treffer[0].Score
}
