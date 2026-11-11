package dienst

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"log/slog"
	"slices"
	"strings"
	"testing"
	"time"

	"lifelog/backend/internal/core"
	"lifelog/backend/internal/llm"
	"lifelog/backend/internal/llm/fake"
)

// speicherAttrappe protokolliert alle Aufrufe in Reihenfolge und kann bei
// einzelnen Methoden scheitern.
type speicherAttrappe struct {
	hashes     map[string]string // sha256 → Eintrag-ID (vorhandene Einträge)
	fehler     map[string]error  // Methodenname → Fehler
	protokoll  []string
	eintraege  []core.Entry
	abschnitte []core.Chunk
	dateien    map[string][]byte
}

func neuerSpeicher() *speicherAttrappe {
	return &speicherAttrappe{hashes: map[string]string{}, fehler: map[string]error{}, dateien: map[string][]byte{}}
}

func (s *speicherAttrappe) EintragMitHash(_ context.Context, sha string) (string, bool, error) {
	s.protokoll = append(s.protokoll, "EintragMitHash")
	if err := s.fehler["EintragMitHash"]; err != nil {
		return "", false, err
	}
	id, ok := s.hashes[sha]
	return id, ok, nil
}

func (s *speicherAttrappe) DateiSpeichern(_ context.Context, id, _, _ string, daten []byte) error {
	s.protokoll = append(s.protokoll, "DateiSpeichern")
	if err := s.fehler["DateiSpeichern"]; err != nil {
		return err
	}
	s.dateien[id] = daten
	return nil
}

func (s *speicherAttrappe) DateiLoeschen(_ context.Context, id string) error {
	s.protokoll = append(s.protokoll, "DateiLoeschen")
	delete(s.dateien, id)
	return nil
}

func (s *speicherAttrappe) AbschnitteSpeichern(_ context.Context, a []core.Chunk) error {
	s.protokoll = append(s.protokoll, "AbschnitteSpeichern")
	if err := s.fehler["AbschnitteSpeichern"]; err != nil {
		return err
	}
	s.abschnitte = append(s.abschnitte, a...)
	return nil
}

func (s *speicherAttrappe) AbschnitteLoeschen(_ context.Context, eintragID string) error {
	s.protokoll = append(s.protokoll, "AbschnitteLoeschen")
	s.abschnitte = slices.DeleteFunc(s.abschnitte, func(a core.Chunk) bool { return a.EntryID == eintragID })
	return nil
}

func (s *speicherAttrappe) EintragSpeichern(_ context.Context, e core.Entry) error {
	s.protokoll = append(s.protokoll, "EintragSpeichern")
	if err := s.fehler["EintragSpeichern"]; err != nil {
		if errors.Is(err, core.ErrDuplicate) {
			// Wie bei einem gleichzeitigen Upload: Jetzt existiert der andere.
			s.hashes[e.File.Sha256] = "aaaaaaaaaaaaaaaaaaaaaaaa"
		}
		return err
	}
	s.eintraege = append(s.eintraege, e)
	return nil
}

// leer: Nach einem gescheiterten Upload darf nichts übrig bleiben.
func (s *speicherAttrappe) leer() bool {
	return len(s.eintraege) == 0 && len(s.abschnitte) == 0 && len(s.dateien) == 0
}

func zeiger(s string) *string { return &s }

const pdf = "%PDF-1.7\nsynthetischer Befund"

func erkennung() core.Extraction {
	return core.Extraction{
		Readable:     true,
		DocumentDate: zeiger("2026-10-14"),
		Issuer:       zeiger("Labor Muster GmbH"),
		Type:         "laborbefund",
		FullText:     "Labor Muster GmbH\n\nHämoglobin 14,2 g/dl\nCRP <0,5 mg/dl",
		Facts: []core.ExtractedFact{
			{Name: "Hämoglobin", Value: "14,2", Unit: zeiger("g/dl")},
			{Name: "CRP", Value: "<0,5", Unit: zeiger("mg/dl")},
		},
	}
}

type aufbau struct {
	speicher  *speicherAttrappe
	leser     *fake.Leser
	einbetter *fake.Einbetter
	dienst    *Hochladen
}

func neuerAufbau() aufbau {
	a := aufbau{
		speicher:  neuerSpeicher(),
		leser:     &fake.Leser{Erkennung: erkennung()},
		einbetter: &fake.Einbetter{},
	}
	n := 0
	a.dienst = NeuHochladen(a.speicher, a.leser, a.einbetter, func() string {
		n++
		return fmt.Sprintf("%024d", n) // 000…001, 000…002: gültige ObjectId-Form
	})
	a.dienst.jetzt = func() time.Time { return time.Date(2026, 11, 4, 13, 0, 0, 123456789, time.UTC) }
	return a
}

func TestHochladen_Erfolg(t *testing.T) {
	a := neuerAufbau()
	e, err := a.dienst.Ausfuehren(context.Background(), Upload{Daten: []byte(pdf), Dateiname: "befund.pdf"})
	if err != nil {
		t.Fatal(err)
	}

	// Reihenfolge: erst prüfen, dann Datei → Abschnitte → Eintrag zuletzt.
	erwartet := []string{"EintragMitHash", "DateiSpeichern", "AbschnitteSpeichern", "EintragSpeichern"}
	if !slices.Equal(a.speicher.protokoll, erwartet) {
		t.Errorf("Reihenfolge = %v, erwartet %v", a.speicher.protokoll, erwartet)
	}

	if e.ID == "" || e.File.ID == "" || e.ID == e.File.ID {
		t.Errorf("IDs: Eintrag %q, Datei %q", e.ID, e.File.ID)
	}
	if e.Module != "gesundheit" || e.Type != "laborbefund" {
		t.Errorf("Modul/Typ = %q/%q", e.Module, e.Type)
	}
	if !e.ImportedAt.Equal(time.Date(2026, 11, 4, 13, 0, 0, 123000000, time.UTC)) {
		t.Errorf("Importdatum = %v, erwartet auf Millisekunden gekürzt", e.ImportedAt)
	}
	d := e.File
	if d.MimeType != core.MimePDF || d.Size != int64(len(pdf)) || d.FileName != "befund.pdf" || len(d.Sha256) != 64 {
		t.Errorf("Datei = %+v", d)
	}
	if e.Corrections == nil || len(e.Corrections) != 0 {
		t.Errorf("Korrekturen = %#v", e.Corrections)
	}
	if len(e.Facts) != 2 || e.Facts[0].ID != "i1" {
		t.Errorf("Infos = %+v", e.Facts)
	}

	// Abschnitte gehören zum Eintrag, tragen das Modul und sind nummeriert.
	ab := a.speicher.abschnitte
	if len(ab) == 0 {
		t.Fatal("keine Abschnitte gespeichert")
	}
	for i, x := range ab {
		if x.EntryID != e.ID || x.Module != "gesundheit" || x.Position != i+1 || len(x.Embedding) != llm.EmbeddingDimensionen {
			t.Errorf("Abschnitt %d = %+v", i, x)
		}
	}
	if !bytes.Equal(a.speicher.dateien[d.ID], []byte(pdf)) {
		t.Error("Datei nicht unverändert gespeichert")
	}
	if len(a.speicher.eintraege) != 1 {
		t.Errorf("%d Einträge gespeichert", len(a.speicher.eintraege))
	}
}

func TestHochladen_DuplikatVorModellaufruf(t *testing.T) {
	a := neuerAufbau()
	if _, err := a.dienst.Ausfuehren(context.Background(), Upload{Daten: []byte(pdf)}); err != nil {
		t.Fatal(err)
	}
	vorhanden := a.speicher.eintraege[0]
	a.speicher.hashes[vorhanden.File.Sha256] = vorhanden.ID
	leserVorher, einbetterVorher := a.leser.Aufrufe(), a.einbetter.Aufrufe()

	_, err := a.dienst.Ausfuehren(context.Background(), Upload{Daten: []byte(pdf)})

	var df *DuplikatFehler
	if !errors.As(err, &df) || df.EintragID != vorhanden.ID {
		t.Fatalf("erwartet DuplikatFehler mit %q, bekommen %v", vorhanden.ID, err)
	}
	if !errors.Is(err, core.ErrDuplicate) {
		t.Error("DuplikatFehler muss auch core.ErrDuplikat sein")
	}
	if a.leser.Aufrufe() != leserVorher || a.einbetter.Aufrufe() != einbetterVorher {
		t.Error("bei einem Duplikat darf kein Modell aufgerufen werden")
	}
	if len(a.speicher.eintraege) != 1 {
		t.Error("Duplikat wurde gespeichert")
	}
}

func TestHochladen_PruefungenVorJedemAufruf(t *testing.T) {
	faelle := []struct {
		name   string
		upload Upload
		fehler error
	}{
		{"leere Datei", Upload{Daten: nil}, core.ErrFileEmpty},
		{"zu groß", Upload{Daten: append([]byte("%PDF-"), make([]byte, core.MaxFileBytes)...)}, core.ErrFileTooLarge},
		{"falscher Typ", Upload{Daten: []byte("nur Text")}, core.ErrUnsupportedFileType},
		{"unbekanntes Modul", Upload{Daten: []byte(pdf), Modul: "sport"}, core.ErrUnknownModule},
	}
	for _, f := range faelle {
		t.Run(f.name, func(t *testing.T) {
			a := neuerAufbau()
			_, err := a.dienst.Ausfuehren(context.Background(), f.upload)
			if !errors.Is(err, f.fehler) {
				t.Errorf("erwartet %v, bekommen %v", f.fehler, err)
			}
			if len(a.speicher.protokoll) != 0 || a.leser.Aufrufe() != 0 {
				t.Errorf("vor bestandener Prüfung darf nichts aufgerufen werden: %v", a.speicher.protokoll)
			}
		})
	}
}

func TestHochladen_FehlerVorDemSpeichern(t *testing.T) {
	nichtLesbar := erkennung()
	nichtLesbar.Readable = false
	falscherTyp := erkennung()
	falscherTyp.Type = "kochrezept"

	faelle := []struct {
		name      string
		leser     *fake.Leser
		einbetter *fake.Einbetter
		fehler    error
	}{
		{"Vision-LLM nicht erreichbar", &fake.Leser{Err: llm.ErrNichtErreichbar}, &fake.Einbetter{}, llm.ErrNichtErreichbar},
		{"Antwort ungültig", &fake.Leser{Err: llm.ErrAntwortUngueltig}, &fake.Einbetter{}, llm.ErrAntwortUngueltig},
		{"nicht lesbar", &fake.Leser{Erkennung: nichtLesbar}, &fake.Einbetter{}, core.ErrUnreadable},
		{"Erkennung ungültig", &fake.Leser{Erkennung: falscherTyp}, &fake.Einbetter{}, core.ErrInvalidExtraction},
		{"Embedding nicht erreichbar", &fake.Leser{Erkennung: erkennung()}, &fake.Einbetter{Err: llm.ErrNichtErreichbar}, llm.ErrNichtErreichbar},
	}
	for _, f := range faelle {
		t.Run(f.name, func(t *testing.T) {
			a := neuerAufbau()
			a.dienst.leser, a.dienst.einbetter = f.leser, f.einbetter

			_, err := a.dienst.Ausfuehren(context.Background(), Upload{Daten: []byte(pdf)})

			if !errors.Is(err, f.fehler) {
				t.Errorf("erwartet %v, bekommen %v", f.fehler, err)
			}
			if !slices.Equal(a.speicher.protokoll, []string{"EintragMitHash"}) {
				t.Errorf("nach einem Modellfehler darf nichts gespeichert werden: %v", a.speicher.protokoll)
			}
		})
	}
}

func TestHochladen_AufraeumenBeiSpeicherfehlern(t *testing.T) {
	dbFehler := errors.New("Verbindung verloren")
	faelle := []struct {
		name      string
		scheitert string
		protokoll []string
	}{
		{"Datei scheitert", "DateiSpeichern",
			[]string{"EintragMitHash", "DateiSpeichern", "DateiLoeschen"}},
		{"Abschnitte scheitern", "AbschnitteSpeichern",
			[]string{"EintragMitHash", "DateiSpeichern", "AbschnitteSpeichern", "AbschnitteLoeschen", "DateiLoeschen"}},
		{"Eintrag scheitert", "EintragSpeichern",
			[]string{"EintragMitHash", "DateiSpeichern", "AbschnitteSpeichern", "EintragSpeichern", "AbschnitteLoeschen", "DateiLoeschen"}},
	}
	for _, f := range faelle {
		t.Run(f.name, func(t *testing.T) {
			a := neuerAufbau()
			a.speicher.fehler[f.scheitert] = dbFehler

			_, err := a.dienst.Ausfuehren(context.Background(), Upload{Daten: []byte(pdf)})

			if !errors.Is(err, dbFehler) {
				t.Errorf("err = %v", err)
			}
			if !slices.Equal(a.speicher.protokoll, f.protokoll) {
				t.Errorf("Protokoll = %v\nerwartet   %v", a.speicher.protokoll, f.protokoll)
			}
			if !a.speicher.leer() {
				t.Error("nach dem Aufräumen ist noch etwas gespeichert")
			}
		})
	}
}

func TestHochladen_GleichzeitigesDuplikat(t *testing.T) {
	// Beide Uploads kommen an der Prüfung in Schritt 2 vorbei; erst der
	// eindeutige Index beim Speichern merkt es.
	a := neuerAufbau()
	a.speicher.fehler["EintragSpeichern"] = core.ErrDuplicate

	_, err := a.dienst.Ausfuehren(context.Background(), Upload{Daten: []byte(pdf)})

	var df *DuplikatFehler
	if !errors.As(err, &df) || df.EintragID != "aaaaaaaaaaaaaaaaaaaaaaaa" {
		t.Fatalf("erwartet DuplikatFehler mit vorhandener ID, bekommen %v", err)
	}
	if !a.speicher.leer() {
		t.Error("eigene Datei und Abschnitte müssen wieder entfernt sein")
	}
}

func TestHochladen_AufraeumenAuchNachAbbruch(t *testing.T) {
	// Die App bricht ab, während gespeichert wird. Das Aufräumen braucht
	// einen eigenen Kontext, sonst scheitert es selbst am Abbruch.
	a := neuerAufbau()
	ctx, abbrechen := context.WithCancel(context.Background())
	a.speicher.fehler["EintragSpeichern"] = context.Canceled
	abbrechen()

	// Die Attrappen ignorieren den Kontext; entscheidend ist, dass
	// aufgeräumt wird und der Abbruch erkennbar bleibt.
	_, err := a.dienst.Ausfuehren(ctx, Upload{Daten: []byte(pdf)})
	if !errors.Is(err, context.Canceled) {
		t.Errorf("err = %v", err)
	}
	if !a.speicher.leer() {
		t.Error("nach Abbruch nicht aufgeräumt")
	}
}

func TestHochladen_AusgelasseneInfosNurAlsFeldImLog(t *testing.T) {
	var log bytes.Buffer
	alt := slog.Default()
	slog.SetDefault(slog.New(slog.NewTextHandler(&log, nil)))
	t.Cleanup(func() { slog.SetDefault(alt) })

	e := erkennung()
	e.Facts = append(e.Facts, core.ExtractedFact{Name: "", Value: "GEHEIMER-WERT-42"})
	a := neuerAufbau()
	a.dienst.leser = &fake.Leser{Erkennung: e}

	eintrag, err := a.dienst.Ausfuehren(context.Background(), Upload{Daten: []byte(pdf)})
	if err != nil {
		t.Fatal(err)
	}
	if len(eintrag.Facts) != 2 {
		t.Errorf("Infos = %+v, die unvollständige muss fehlen", eintrag.Facts)
	}
	if !strings.Contains(log.String(), "infos[2].name") {
		t.Errorf("Feld fehlt im Log: %s", log.String())
	}
	if strings.Contains(log.String(), "GEHEIMER-WERT-42") {
		t.Errorf("Wert im Log: %s", log.String())
	}
}

func TestDateiname(t *testing.T) {
	faelle := map[string]string{
		"befund.pdf":              "befund.pdf",
		"  befund.pdf  ":          "befund.pdf",
		"../../etc/passwd":        "passwd",
		`C:\Users\x\Befund Ä.pdf`: "Befund Ä.pdf",
		"zeile\nzwei\x00.pdf":     "zeilezwei.pdf",
		"":                        "dokument",
		"   ":                     "dokument",
		strings.Repeat("ä", 250):  strings.Repeat("ä", 200),
	}
	for ein, erwartet := range faelle {
		if got := dateiname(ein); got != erwartet {
			t.Errorf("dateiname(%q) = %q, erwartet %q", ein, got, erwartet)
		}
	}
}
