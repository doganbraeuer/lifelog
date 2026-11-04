// Package speicher kapselt den Zugriff auf MongoDB. Andere Pakete sehen nur
// kern-Typen, keine BSON-Dokumente.
package speicher

import (
	"bytes"
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

// Namen der Collections. GridFS legt für den Bucket "dateien" selbst die
// Collections dateien.files (Metadaten) und dateien.chunks (Daten in
// Stücken zu 255 KB) an.
const (
	collEintraege  = "eintraege"
	collAbschnitte = "abschnitte"
	bucketDateien  = "dateien"
)

// Speicher ist die Verbindung zu einer Lifelog-Datenbank.
type Speicher struct {
	client     *mongo.Client
	db         *mongo.Database
	eintraege  *mongo.Collection
	abschnitte *mongo.Collection
	dateien    *mongo.GridFSBucket
}

// NeueID erzeugt eine neue ObjectId als Hex-String. Der Upload vergibt die
// IDs vorab, weil Datei und Abschnitte auf den Eintrag verweisen, der erst
// zuletzt gespeichert wird.
func NeueID() string {
	return bson.NewObjectID().Hex()
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
	db := client.Database(datenbank)
	s := &Speicher{
		client:     client,
		db:         db,
		eintraege:  db.Collection(collEintraege),
		abschnitte: db.Collection(collAbschnitte),
		dateien:    db.GridFSBucket(options.GridFSBucket().SetName(bucketDateien)),
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

	// Löschen eines Eintrags entfernt seine Abschnitte über eintragId.
	// Ohne Index müsste MongoDB dafür alle Abschnitte durchsuchen.
	_, err = s.abschnitte.Indexes().CreateOne(ctx, mongo.IndexModel{
		Keys:    bson.D{{Key: "eintragId", Value: 1}},
		Options: options.Index().SetName("eintragId"),
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
//
// Gibt es schon einen Eintrag mit demselben datei.sha256, meldet der
// eindeutige Index einen Fehler, und EintragSpeichern liefert kern.ErrDuplikat.
// Das fängt den Fall ab, dass zwei gleiche Uploads gleichzeitig an der
// Duplikat-Prüfung vorbeikommen.
func (s *Speicher) EintragSpeichern(ctx context.Context, e kern.Eintrag) error {
	dok, err := zuEintragDok(e)
	if err != nil {
		return err
	}
	if _, err := s.eintraege.InsertOne(ctx, dok); err != nil {
		if mongo.IsDuplicateKeyError(err) {
			return kern.ErrDuplikat
		}
		return fmt.Errorf("Eintrag speichern: %w", err)
	}
	return nil
}

// EintragMitHash sucht den Eintrag, dessen Datei diesen SHA-256-Hash hat.
// Die Suche gilt über alle Module (API.md 3.1).
func (s *Speicher) EintragMitHash(ctx context.Context, sha256 string) (id string, gefunden bool, err error) {
	var treffer struct {
		ID bson.ObjectID `bson:"_id"`
	}
	err = s.eintraege.FindOne(ctx,
		bson.D{{Key: "datei.sha256", Value: sha256}},
		options.FindOne().SetProjection(bson.D{{Key: "_id", Value: 1}}),
	).Decode(&treffer)
	if errors.Is(err, mongo.ErrNoDocuments) {
		return "", false, nil
	}
	if err != nil {
		return "", false, fmt.Errorf("Duplikat prüfen: %w", err)
	}
	return treffer.ID.Hex(), true, nil
}

// DateiSpeichern legt die Originaldatei in GridFS ab. GridFS teilt Dateien
// in Stücke, weil ein einzelnes MongoDB-Dokument höchstens 16 MB groß sein
// darf. Unsere Dateien sind zwar ≤ 10 MB, aber so bleibt das Eintrag-Dokument
// klein und die Datei wird nur geladen, wenn jemand sie wirklich ansieht.
func (s *Speicher) DateiSpeichern(ctx context.Context, id, dateiname, mimeTyp string, daten []byte) error {
	oid, err := bson.ObjectIDFromHex(id)
	if err != nil {
		return fmt.Errorf("ungültige Datei-ID: %w", err)
	}
	err = s.dateien.UploadFromStreamWithID(ctx, oid, dateiname, bytes.NewReader(daten),
		options.GridFSUpload().SetMetadata(bson.D{{Key: "mimeTyp", Value: mimeTyp}}))
	if err != nil {
		return fmt.Errorf("Datei speichern: %w", err)
	}
	return nil
}

// DateiLoeschen entfernt die Datei aus GridFS. Gibt es sie nicht (mehr), ist
// das kein Fehler: So kann ein abgebrochenes Löschen einfach wiederholt
// werden (API.md 3.7).
func (s *Speicher) DateiLoeschen(ctx context.Context, id string) error {
	oid, err := bson.ObjectIDFromHex(id)
	if err != nil {
		return fmt.Errorf("ungültige Datei-ID: %w", err)
	}
	if err := s.dateien.Delete(ctx, oid); err != nil && !errors.Is(err, mongo.ErrFileNotFound) {
		return fmt.Errorf("Datei löschen: %w", err)
	}
	return nil
}

// AbschnitteSpeichern fügt alle Abschnitte eines Eintrags ein. Eine leere
// Liste ist erlaubt und bewirkt nichts.
func (s *Speicher) AbschnitteSpeichern(ctx context.Context, abschnitte []kern.Abschnitt) error {
	if len(abschnitte) == 0 {
		return nil
	}
	doks := make([]any, 0, len(abschnitte))
	for _, a := range abschnitte {
		dok, err := zuAbschnittDok(a)
		if err != nil {
			return err
		}
		doks = append(doks, dok)
	}
	if _, err := s.abschnitte.InsertMany(ctx, doks); err != nil {
		return fmt.Errorf("Abschnitte speichern: %w", err)
	}
	return nil
}

// AbschnitteLoeschen entfernt alle Abschnitte eines Eintrags. Gibt es keine,
// ist das kein Fehler.
func (s *Speicher) AbschnitteLoeschen(ctx context.Context, eintragID string) error {
	oid, err := bson.ObjectIDFromHex(eintragID)
	if err != nil {
		return fmt.Errorf("ungültige Eintrag-ID: %w", err)
	}
	if _, err := s.abschnitte.DeleteMany(ctx, bson.D{{Key: "eintragId", Value: oid}}); err != nil {
		return fmt.Errorf("Abschnitte löschen: %w", err)
	}
	return nil
}

// EintragLoeschen löscht nur das Eintrag-Dokument. gefunden ist false, wenn
// es keinen Eintrag mit dieser ID gab. Abschnitte und Datei löscht der
// Aufrufer vorher mit AbschnitteLoeschen und DateiLoeschen (API.md 3.7).
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
