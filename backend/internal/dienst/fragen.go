package dienst

import (
	"context"
	"log/slog"

	"lifelog/backend/internal/core"
	"lifelog/backend/internal/modell"
)

// FragenSpeicher ist, was die Suche von der Datenbank braucht. Umgesetzt
// von speicher.Speicher.
type FragenSpeicher interface {
	AehnlicheAbschnitte(ctx context.Context, vektor []float32, modul string, k int) ([]core.Treffer, error)
	EintraegeKurzNachIDs(ctx context.Context, ids []string) (map[string]core.EintragKurz, error)
}

// Fragen beantwortet Fragen zu den Dokumenten (docs/API.md 3.6).
type Fragen struct {
	speicher  FragenSpeicher
	einbetter modell.Einbetter
	antworter modell.Antworter
}

func NeuFragen(s FragenSpeicher, e modell.Einbetter, a modell.Antworter) *Fragen {
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
// modell.ErrNichtErreichbar, modell.ErrAntwortUngueltig oder intern.
func (f *Fragen) Ausfuehren(ctx context.Context, frage, modul string) (core.Antwort, error) {
	modul, err := core.PruefeModul(modul)
	if err != nil {
		return core.Antwort{}, err
	}
	frage, err = core.PruefeFrage(frage)
	if err != nil {
		return core.Antwort{}, err
	}

	vektor, err := f.einbetter.FrageEinbetten(ctx, frage)
	if err != nil {
		return core.Antwort{}, err
	}
	alle, err := f.speicher.AehnlicheAbschnitte(ctx, vektor, modul, core.AnzahlTreffer)
	if err != nil {
		return core.Antwort{}, err
	}
	treffer := core.UeberSchwelle(alle, core.MinScore)

	fundstellen, err := f.fundstellen(ctx, treffer, modul)
	if err != nil {
		return core.Antwort{}, err
	}
	slog.Info("Frage", "modul", modul, "treffer", len(alle), "ueberSchwelle", len(treffer),
		"fundstellen", len(fundstellen), "besterScore", besterScore(alle))
	if len(fundstellen) == 0 {
		return core.KeinTreffer(), nil
	}

	text, err := f.antworter.Antworten(ctx, frage, fundstellen)
	if err != nil {
		return core.Antwort{}, err
	}
	antwort := core.BaueAntwort(text, fundstellen)
	if !antwort.Gefunden {
		slog.Info("Antwort ohne gültigen Verweis, fester Satz", "fundstellen", len(fundstellen))
	}
	return antwort, nil
}

// fundstellen nummeriert die Treffer (1, 2, …) und ergänzt die aktuellen
// Angaben des Eintrags: Typ, Datum und Quelle auch dann, wenn sie
// korrigiert wurden (API.md 3.6). Treffer ohne Eintrag fallen weg; das kann
// kurz während eines Uploads oder Löschens vorkommen.
func (f *Fragen) fundstellen(ctx context.Context, treffer []core.Treffer, modul string) ([]core.Fundstelle, error) {
	if len(treffer) == 0 {
		return []core.Fundstelle{}, nil
	}
	ids := make([]string, 0, len(treffer))
	for _, t := range treffer {
		ids = append(ids, t.EintragID)
	}
	eintraege, err := f.speicher.EintraegeKurzNachIDs(ctx, ids)
	if err != nil {
		return nil, err
	}

	fundstellen := make([]core.Fundstelle, 0, len(treffer))
	for _, t := range treffer {
		e, ok := eintraege[t.EintragID]
		if !ok {
			continue // verwaister Abschnitt
		}
		fundstellen = append(fundstellen, core.Fundstelle{
			Nr:            len(fundstellen) + 1,
			EintragID:     t.EintragID,
			Modul:         modul,
			Typ:           e.Typ,
			Quelle:        e.Quelle,
			Dokumentdatum: e.Dokumentdatum,
			Auszug:        t.Text,
		})
	}
	return fundstellen, nil
}

func besterScore(treffer []core.Treffer) float64 {
	if len(treffer) == 0 {
		return 0
	}
	return treffer[0].Score
}
