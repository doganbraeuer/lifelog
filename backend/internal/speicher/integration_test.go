package speicher

// Integrationstests gegen ein echtes MongoDB (Atlas). Sie laufen nur mit
//
//	LIFELOG_MONGO_TEST=1 go test ./internal/speicher/ -run Integration -v
//
// Sie benutzen NICHT die Datenbank aus MONGODB_DB, sondern eine eigene
// Datenbank "<MONGODB_DB>_integrationstest", und löschen sie am Ende wieder.
// Liegen dort Collections, die nicht von diesem Test stammen, bricht der
// Test ab, statt etwas zu löschen.

import (
	"bytes"
	"context"
	"errors"
	"io"
	"os"
	"slices"
	"strings"
	"testing"
	"time"

	"go.mongodb.org/mongo-driver/v2/bson"

	"lifelog/backend/internal/kern"
	"lifelog/backend/internal/konfig"
)

var eigeneCollections = []string{collEintraege, collAbschnitte, bucketDateien + ".files", bucketDateien + ".chunks"}

func testSpeicher(t *testing.T) (*Speicher, context.Context) {
	t.Helper()
	if os.Getenv("LIFELOG_MONGO_TEST") != "1" {
		t.Skip("Integrationstest übersprungen (LIFELOG_MONGO_TEST=1 setzen)")
	}
	if err := konfig.DotenvSuchen(); err != nil {
		t.Fatal(err)
	}
	k, err := konfig.Laden(os.Getenv)
	if err != nil {
		t.Fatal(err)
	}

	ctx, abbrechen := context.WithTimeout(context.Background(), 60*time.Second)
	t.Cleanup(abbrechen)

	name := k.MongoDB + "_integrationstest"
	s, err := Verbinden(ctx, k.MongoURI, name)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = s.Trennen(context.Background()) })

	vorhanden, err := s.db.ListCollectionNames(ctx, bson.D{})
	if err != nil {
		t.Fatal(err)
	}
	for _, c := range vorhanden {
		if !slices.Contains(eigeneCollections, c) {
			t.Fatalf("Datenbank %q enthält fremde Collection %q – Test bricht ab, um nichts zu löschen", name, c)
		}
	}

	// Sauber starten und am Ende aufräumen.
	if err := s.db.Drop(ctx); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		ctx, abbrechen := context.WithTimeout(context.Background(), 30*time.Second)
		defer abbrechen()
		if err := s.db.Drop(ctx); err != nil {
			t.Errorf("Testdatenbank %q nicht gelöscht: %v", name, err)
		}
	})
	t.Logf("Testdatenbank: %s", name)

	if err := s.IndexeAnlegen(ctx); err != nil {
		t.Fatal(err)
	}
	return s, ctx
}

// testEintrag ist synthetisch. hash bestimmt datei.sha256.
func testEintrag(id, dateiID, hash string) kern.Eintrag {
	return kern.Eintrag{
		ID: id, Modul: "gesundheit", Typ: "laborbefund",
		Dokumentdatum: zeiger("2026-10-14"),
		Importdatum:   time.Date(2026, 11, 4, 12, 0, 0, 0, time.UTC),
		Quelle:        zeiger("Labor Muster GmbH"),
		Volltext:      "Hämoglobin 14,2 g/dl",
		Infos:         []kern.Info{{ID: "i1", Name: "Hämoglobin", Wert: "14,2", Einheit: zeiger("g/dl")}},
		Datei: kern.Datei{ID: dateiID, Dateiname: "befund.pdf", MimeTyp: kern.MimePDF,
			Groesse: 9, Sha256: hash},
		Korrekturen: []kern.Korrektur{},
	}
}

func TestIntegration_SpeichernFindenLoeschen(t *testing.T) {
	s, ctx := testSpeicher(t)

	// Indexe ein zweites Mal anlegen darf nichts kaputt machen.
	if err := s.IndexeAnlegen(ctx); err != nil {
		t.Fatalf("IndexeAnlegen ist nicht wiederholbar: %v", err)
	}

	eintragID, dateiID := NeueID(), NeueID()
	hash := strings.Repeat("ab", 32)
	daten := []byte("%PDF-1.7 synthetisch")

	// 1. Datei in GridFS, mit Metadaten.
	if err := s.DateiSpeichern(ctx, dateiID, "befund.pdf", kern.MimePDF, daten); err != nil {
		t.Fatal(err)
	}
	oid, _ := bson.ObjectIDFromHex(dateiID)
	strom, err := s.dateien.OpenDownloadStream(ctx, oid)
	if err != nil {
		t.Fatal(err)
	}
	gelesen, _ := io.ReadAll(strom)
	_ = strom.Close()
	if !bytes.Equal(gelesen, daten) {
		t.Errorf("Datei verändert: %q", gelesen)
	}
	if f := strom.GetFile(); f.Name != "befund.pdf" || !strings.Contains(f.Metadata.String(), kern.MimePDF) {
		t.Errorf("Datei-Metadaten: %s / %s", f.Name, f.Metadata)
	}

	// 2. Abschnitte.
	abschnitte := []kern.Abschnitt{
		{EintragID: eintragID, Modul: "gesundheit", Nr: 1, Text: "Teil 1", Embedding: []float32{0.1, 0.2}},
		{EintragID: eintragID, Modul: "gesundheit", Nr: 2, Text: "Teil 2", Embedding: []float32{0.3, 0.4}},
	}
	if err := s.AbschnitteSpeichern(ctx, abschnitte); err != nil {
		t.Fatal(err)
	}
	if err := s.AbschnitteSpeichern(ctx, nil); err != nil {
		t.Errorf("leere Liste: %v", err)
	}
	pruefeAnzahlAbschnitte(t, s, ctx, eintragID, 2)

	// 3. Eintrag zuletzt, dann über den Hash und in der Liste auffindbar.
	if _, gefunden, err := s.EintragMitHash(ctx, hash); err != nil || gefunden {
		t.Fatalf("vor dem Speichern: gefunden=%v, err=%v", gefunden, err)
	}
	if err := s.EintragSpeichern(ctx, testEintrag(eintragID, dateiID, hash)); err != nil {
		t.Fatal(err)
	}
	id, gefunden, err := s.EintragMitHash(ctx, hash)
	if err != nil || !gefunden || id != eintragID {
		t.Fatalf("EintragMitHash = %q, %v, %v", id, gefunden, err)
	}
	liste, err := s.ListeKurz(ctx, "gesundheit")
	if err != nil || len(liste) != 1 || liste[0].ID != eintragID {
		t.Fatalf("ListeKurz = %+v, %v", liste, err)
	}

	// 4. Löschen in der Reihenfolge aus API.md 3.7, jeder Schritt wiederholbar.
	for durchgang := 1; durchgang <= 2; durchgang++ {
		if err := s.AbschnitteLoeschen(ctx, eintragID); err != nil {
			t.Fatalf("Durchgang %d, Abschnitte: %v", durchgang, err)
		}
		if err := s.DateiLoeschen(ctx, dateiID); err != nil {
			t.Fatalf("Durchgang %d, Datei: %v", durchgang, err)
		}
		gefunden, err := s.EintragLoeschen(ctx, eintragID)
		if err != nil || gefunden != (durchgang == 1) {
			t.Fatalf("Durchgang %d, Eintrag: gefunden=%v, err=%v", durchgang, gefunden, err)
		}
	}
	pruefeAnzahlAbschnitte(t, s, ctx, eintragID, 0)
	if _, err := s.dateien.OpenDownloadStream(ctx, oid); err == nil {
		t.Error("Datei ist nach dem Löschen noch abrufbar")
	}
	if _, gefunden, _ := s.EintragMitHash(ctx, hash); gefunden {
		t.Error("Hash nach dem Löschen noch vorhanden: dieselbe Datei ließe sich nicht neu hochladen")
	}
}

func TestIntegration_DuplikatUeberIndex(t *testing.T) {
	s, ctx := testSpeicher(t)
	hash := strings.Repeat("cd", 32)

	if err := s.EintragSpeichern(ctx, testEintrag(NeueID(), NeueID(), hash)); err != nil {
		t.Fatal(err)
	}
	// Zweiter Eintrag mit anderer ID, aber gleichem Hash: Der eindeutige
	// Index muss das ablehnen, auch ohne vorherige Prüfung.
	err := s.EintragSpeichern(ctx, testEintrag(NeueID(), NeueID(), hash))
	if !errors.Is(err, ErrDuplikat) {
		t.Fatalf("erwartet ErrDuplikat, bekommen %v", err)
	}

	// Anderer Hash geht.
	if err := s.EintragSpeichern(ctx, testEintrag(NeueID(), NeueID(), strings.Repeat("ef", 32))); err != nil {
		t.Errorf("anderer Hash: %v", err)
	}
}

func pruefeAnzahlAbschnitte(t *testing.T, s *Speicher, ctx context.Context, eintragID string, erwartet int64) {
	t.Helper()
	oid, _ := bson.ObjectIDFromHex(eintragID)
	n, err := s.abschnitte.CountDocuments(ctx, bson.D{{Key: "eintragId", Value: oid}})
	if err != nil {
		t.Fatal(err)
	}
	if n != erwartet {
		t.Errorf("%d Abschnitte, erwartet %d", n, erwartet)
	}
}
