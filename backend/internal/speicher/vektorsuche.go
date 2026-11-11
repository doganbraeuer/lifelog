package speicher

import (
	"context"
	_ "embed"
	"fmt"

	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"
	"go.mongodb.org/mongo-driver/v2/mongo/options"

	"lifelog/backend/internal/core"
)

// VektorindexName ist der Name des Atlas-Vector-Search-Index auf der
// Collection "abschnitte".
const VektorindexName = "abschnitte_vektor"

// vektorindexDefinition: "embedding" als Vektor (768 Dimensionen, cosine),
// "modul" als Filterfeld, damit die Suche nur im gewählten Modul sucht.
// Dieselbe Datei wird beim Anlegen in der Atlas-Oberfläche eingefügt
// (backend/README.md, „Vektorindex“).
//
//go:embed abschnitte_vektor.json
var vektorindexDefinition []byte

// numCandidates: So viele Kandidaten betrachtet Atlas, bevor es die besten
// AnzahlTreffer auswählt. Mehr Kandidaten = genauer, aber langsamer;
// 100 ist für wenige tausend Abschnitte reichlich.
const numCandidates = 100

// VektorindexAnlegen legt den Vektorindex an, falls es ihn noch nicht gibt,
// und liefert seinen Status (z. B. "PENDING", "BUILDING", "READY").
//
// Atlas baut den Index im Hintergrund; bis er "READY" ist, findet die Suche
// nichts. Scheitert das Anlegen (z. B. fehlende Rechte), liefert die
// Funktion den Fehler; der Server startet trotzdem, und der Index kann in
// der Atlas-Oberfläche angelegt werden.
//
// Voraussetzung: Die Collection "abschnitte" existiert. IndexeAnlegen legt
// sie mit dem Index auf eintragId an, darum vorher aufrufen.
func (s *Speicher) VektorindexAnlegen(ctx context.Context) (status string, err error) {
	status, gefunden, err := s.VektorindexStatus(ctx)
	if err != nil || gefunden {
		return status, err
	}

	var definition bson.D
	if err := bson.UnmarshalExtJSON(vektorindexDefinition, false, &definition); err != nil {
		return "", fmt.Errorf("Vektorindex-Definition nicht lesbar: %w", err)
	}
	_, err = s.abschnitte.SearchIndexes().CreateOne(ctx, mongo.SearchIndexModel{
		Definition: definition,
		Options:    options.SearchIndexes().SetName(VektorindexName).SetType("vectorSearch"),
	})
	if err != nil {
		return "", fmt.Errorf("Vektorindex anlegen: %w", err)
	}
	return "ANGELEGT", nil
}

// VektorindexStatus liefert den Status des Vektorindex, falls es ihn gibt.
func (s *Speicher) VektorindexStatus(ctx context.Context) (status string, gefunden bool, err error) {
	cur, err := s.abschnitte.SearchIndexes().List(ctx, options.SearchIndexes().SetName(VektorindexName))
	if err != nil {
		return "", false, fmt.Errorf("Vektorindex prüfen: %w", err)
	}
	var indexe []struct {
		Status string `bson:"status"`
	}
	if err := cur.All(ctx, &indexe); err != nil {
		return "", false, fmt.Errorf("Vektorindex prüfen: %w", err)
	}
	if len(indexe) == 0 {
		return "", false, nil
	}
	return indexe[0].Status, true, nil
}

// trefferDok ist ein Ergebnis von $vectorSearch.
type trefferDok struct {
	EintragID bson.ObjectID `bson:"eintragId"`
	Nr        int           `bson:"nr"`
	Text      string        `bson:"text"`
	Score     float64       `bson:"score"`
}

// AehnlicheAbschnitte sucht die k Abschnitte des Moduls, deren Embedding dem
// Vektor am ähnlichsten ist, absteigend nach Score.
//
// Der Score ist der vectorSearchScore von Atlas, bei cosine (1 + cos) / 2.
// Gefiltert nach Schwelle wird nicht hier, sondern im Dienst (core.MinScore).
func (s *Speicher) AehnlicheAbschnitte(ctx context.Context, vektor []float32, modul string, k int) ([]core.Treffer, error) {
	pipeline := mongo.Pipeline{
		{{Key: "$vectorSearch", Value: bson.D{
			{Key: "index", Value: VektorindexName},
			{Key: "path", Value: "embedding"},
			{Key: "queryVector", Value: vektor},
			{Key: "numCandidates", Value: numCandidates},
			{Key: "limit", Value: k},
			// Nur möglich, weil "modul" im Index als Filterfeld eingetragen ist.
			{Key: "filter", Value: bson.D{{Key: "modul", Value: bson.D{{Key: "$eq", Value: modul}}}}},
		}}},
		{{Key: "$project", Value: bson.D{
			{Key: "_id", Value: 0},
			{Key: "eintragId", Value: 1},
			{Key: "nr", Value: 1},
			{Key: "text", Value: 1},
			{Key: "score", Value: bson.D{{Key: "$meta", Value: "vectorSearchScore"}}},
		}}},
	}
	cur, err := s.abschnitte.Aggregate(ctx, pipeline)
	if err != nil {
		return nil, fmt.Errorf("Vektorsuche: %w", err)
	}
	var doks []trefferDok
	if err := cur.All(ctx, &doks); err != nil {
		return nil, fmt.Errorf("Vektorsuche: %w", err)
	}
	treffer := make([]core.Treffer, 0, len(doks))
	for _, d := range doks {
		treffer = append(treffer, core.Treffer{EintragID: d.EintragID.Hex(), Nr: d.Nr, Text: d.Text, Score: d.Score})
	}
	return treffer, nil
}

// EintraegeKurzNachIDs lädt die Kurzform mehrerer Einträge auf einmal.
// Einträge, die es nicht (mehr) gibt, fehlen einfach in der Map; der Dienst
// verwirft dann die zugehörigen Abschnitte.
func (s *Speicher) EintraegeKurzNachIDs(ctx context.Context, ids []string) (map[string]core.EintragKurz, error) {
	oids := make([]bson.ObjectID, 0, len(ids))
	for _, id := range ids {
		oid, err := bson.ObjectIDFromHex(id)
		if err != nil {
			return nil, core.ErrUngueltigeID
		}
		oids = append(oids, oid)
	}
	ergebnis := map[string]core.EintragKurz{}
	if len(oids) == 0 {
		return ergebnis, nil
	}
	cur, err := s.eintraege.Find(ctx,
		bson.D{{Key: "_id", Value: bson.D{{Key: "$in", Value: oids}}}},
		options.Find().SetProjection(kurzProjektion))
	if err != nil {
		return nil, fmt.Errorf("Einträge lesen: %w", err)
	}
	var doks []eintragKurzDok
	if err := cur.All(ctx, &doks); err != nil {
		return nil, fmt.Errorf("Einträge lesen: %w", err)
	}
	for _, d := range doks {
		ergebnis[d.ID.Hex()] = d.zuKern()
	}
	return ergebnis, nil
}
