// Package speicher kapselt den Zugriff auf MongoDB. Andere Pakete sehen nur
// kern-Typen, keine BSON-Dokumente.
package speicher

import (
	"context"
	"errors"
	"fmt"
	"time"

	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"
	"go.mongodb.org/mongo-driver/v2/mongo/options"
	"go.mongodb.org/mongo-driver/v2/mongo/readpref"

	"lifelog/backend/internal/kern"
)

// Namen der Collections.
const (
	collEintraege = "eintraege"
)

// Speicher ist die Verbindung zu einer Lifelog-Datenbank.
type Speicher struct {
	client    *mongo.Client
	eintraege *mongo.Collection
}

// Verbinden baut die Verbindung auf und prüft sie mit einem Ping.
//
// Fehlermeldungen enthalten bewusst nicht die Ursache des Treibers, weil darin
// Teile der URI stehen können. Die URI enthält das Passwort.
func Verbinden(ctx context.Context, uri, datenbank string) (*Speicher, error) {
	client, err := mongo.Connect(options.Client().ApplyURI(uri))
	if err != nil {
		return nil, errors.New("MongoDB-Client konnte nicht erstellt werden (MONGODB_URI prüfen)")
	}
	s := &Speicher{
		client:    client,
		eintraege: client.Database(datenbank).Collection(collEintraege),
	}

	pingCtx, abbrechen := context.WithTimeout(ctx, 10*time.Second)
	defer abbrechen()
	if err := s.Ping(pingCtx); err != nil {
		_ = s.Trennen(context.Background())
		return nil, errors.New("MongoDB nicht erreichbar: Zugangsdaten und Network Access in Atlas prüfen")
	}
	return s, nil
}

// Trennen schließt die Verbindung.
func (s *Speicher) Trennen(ctx context.Context) error {
	ctx, abbrechen := context.WithTimeout(ctx, 5*time.Second)
	defer abbrechen()
	return s.client.Disconnect(ctx)
}

// Ping prüft, ob die Datenbank antwortet (für GET /status).
func (s *Speicher) Ping(ctx context.Context) error {
	return s.client.Ping(ctx, readpref.Primary())
}

// IndexeAnlegen legt die normalen Indexe an. Gibt es einen Index schon mit
// derselben Definition, passiert nichts. Man kann das bei jedem Start aufrufen.
func (s *Speicher) IndexeAnlegen(ctx context.Context) error {
	_, err := s.eintraege.Indexes().CreateMany(ctx, []mongo.IndexModel{
		{
			// Duplikat-Erkennung (API.md 3.1): Der eindeutige Index fängt auch
			// zwei gleichzeitige Uploads derselben Datei ab.
			Keys:    bson.D{{Key: "datei.sha256", Value: 1}},
			Options: options.Index().SetName("datei_sha256_eindeutig").SetUnique(true),
		},
		{
			// Die Zeitleiste filtert immer nach Modul.
			Keys:    bson.D{{Key: "modul", Value: 1}},
			Options: options.Index().SetName("modul"),
		},
	})
	if err != nil {
		return fmt.Errorf("Indexe anlegen: %w", err)
	}
	return nil
}

// ListeKurz liefert alle Einträge eines Moduls in Kurzform, unsortiert.
// Sortiert wird in kern.Sortiere, weil die Regeln (null am Ende usw.) dort
// getestet sind.
func (s *Speicher) ListeKurz(ctx context.Context, modul string) ([]kern.EintragKurz, error) {
	cur, err := s.eintraege.Find(ctx,
		bson.D{{Key: "modul", Value: modul}},
		options.Find().SetProjection(kurzProjektion))
	if err != nil {
		return nil, fmt.Errorf("Einträge lesen: %w", err)
	}
	var doks []eintragKurzDok
	if err := cur.All(ctx, &doks); err != nil {
		return nil, fmt.Errorf("Einträge lesen: %w", err)
	}

	liste := make([]kern.EintragKurz, 0, len(doks))
	for _, d := range doks {
		liste = append(liste, d.zuKern())
	}
	return liste, nil
}

// EintragSpeichern fügt einen neuen Eintrag ein. ID und Datei.ID müssen
// gültige ObjectIds sein.
func (s *Speicher) EintragSpeichern(ctx context.Context, e kern.Eintrag) error {
	dok, err := zuEintragDok(e)
	if err != nil {
		return err
	}
	if _, err := s.eintraege.InsertOne(ctx, dok); err != nil {
		return fmt.Errorf("Eintrag speichern: %w", err)
	}
	return nil
}

// EintragLoeschen löscht nur das Eintrag-Dokument. gefunden ist false, wenn
// es keinen Eintrag mit dieser ID gab. Abschnitte und Datei folgen in
// Schritt 8 (DELETE).
func (s *Speicher) EintragLoeschen(ctx context.Context, id string) (gefunden bool, err error) {
	oid, err := bson.ObjectIDFromHex(id)
	if err != nil {
		return false, fmt.Errorf("ungültige Eintrag-ID: %w", err)
	}
	res, err := s.eintraege.DeleteOne(ctx, bson.D{{Key: "_id", Value: oid}})
	if err != nil {
		return false, fmt.Errorf("Eintrag löschen: %w", err)
	}
	return res.DeletedCount > 0, nil
}
