package api

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strings"
	"testing"
	"time"

	"lifelog/backend/internal/core"
)

const idKorrektur = "6727a1f3c2b4e5d6f7a8b9c0"

type korrigiererAttrappe struct {
	eintrag    core.Entry
	err        error
	aufgerufen bool
	id         string
	aenderung  core.Change
}

func (k *korrigiererAttrappe) Correct(_ context.Context, id string, a core.Change) (core.Entry, error) {
	k.aufgerufen, k.id, k.aenderung = true, id, a
	return k.eintrag, k.err
}

func korrigieren(t *testing.T, k *korrigiererAttrappe, id, body string) *httptest.ResponseRecorder {
	t.Helper()
	router := NeuerRouter(Abhaengigkeiten{DB: pingerAttrappe{}, Korrigieren: k})
	r := httptest.NewRequest(http.MethodPatch, "/api/v1/eintraege/"+id, strings.NewReader(body))
	r.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, r)
	return rec
}

func zt(s string) *string { return &s }

func TestKorrigieren_ErfolgGenauWieAPIBeispiel(t *testing.T) {
	zeit := time.Date(2026, 11, 3, 19, 5, 31, 0, time.UTC)
	k := &korrigiererAttrappe{eintrag: core.Entry{
		ID: idKorrektur, Module: "gesundheit", Type: "laborbefund",
		DocumentDate: zt("2026-10-15"), ImportedAt: time.Date(2026, 11, 3, 18, 42, 7, 0, time.UTC),
		Issuer: zt("Labor Muster GmbH, Salzburg"), FullText: "Leukozyten 6,3 G/l",
		Facts: []core.Fact{{ID: "i2", Name: "Leukozyten", Value: "6,8", Unit: zt("G/l")}},
		File:  core.File{ID: "6727a1f3c2b4e5d6f7a8b9c1", FileName: "befund_oktober.pdf", MimeType: "application/pdf", Size: 184320, Sha256: "3f8a"},
		Corrections: []core.Correction{
			{Field: "dokumentdatum", Recognized: zt("2026-10-14"), Corrected: zt("2026-10-15"), CorrectedAt: zeit},
			{Field: "infos.i2.wert", Recognized: zt("6,3"), Corrected: zt("6,8"), CorrectedAt: zeit},
		},
	}}
	rec := korrigieren(t, k, idKorrektur, `{"dokumentdatum": "2026-10-15", "infos": [{"id": "i2", "wert": "6,8"}]}`)

	if rec.Code != http.StatusOK {
		t.Fatalf("Status %d: %s", rec.Code, rec.Body)
	}
	if k.id != idKorrektur {
		t.Errorf("ID an den Dienst: %q", k.id)
	}
	want := core.Change{
		DocumentDate: core.Nullable{Set: true, Value: zt("2026-10-15")},
		Facts:        []core.FactChange{{ID: "i2", Value: zt("6,8")}},
	}
	if !reflect.DeepEqual(k.aenderung, want) {
		t.Errorf("Änderung = %+v, erwartet %+v", k.aenderung, want)
	}
	soll := `{"id":"6727a1f3c2b4e5d6f7a8b9c0","modul":"gesundheit","typ":"laborbefund","dokumentdatum":"2026-10-15",` +
		`"importdatum":"2026-11-03T18:42:07Z","quelle":"Labor Muster GmbH, Salzburg","volltext":"Leukozyten 6,3 G/l",` +
		`"infos":[{"id":"i2","name":"Leukozyten","wert":"6,8","einheit":"G/l"}],` +
		`"datei":{"id":"6727a1f3c2b4e5d6f7a8b9c1","dateiname":"befund_oktober.pdf","mimeTyp":"application/pdf","groesse":184320,"sha256":"3f8a"},` +
		`"korrekturen":[{"feld":"dokumentdatum","erkannt":"2026-10-14","korrigiert":"2026-10-15","zeitpunkt":"2026-11-03T19:05:31Z"},` +
		`{"feld":"infos.i2.wert","erkannt":"6,3","korrigiert":"6,8","zeitpunkt":"2026-11-03T19:05:31Z"}]}`
	if got := strings.TrimSpace(rec.Body.String()); got != soll {
		t.Errorf("Body =\n%s\nerwartet\n%s", got, soll)
	}
}

func TestLeseAenderung_DreiFaelle(t *testing.T) {
	// fehlt / null / Wert, für alle Felder.
	faelle := map[string]core.Change{
		`{"typ": "rezept"}`:       {Type: zt("rezept")},
		`{"dokumentdatum": null}`: {DocumentDate: core.Nullable{Set: true}},
		`{"quelle": "  "}`:        {Issuer: core.Nullable{Set: true, Value: zt("  ")}}, // Leerraum entfernt erst kern
		`{"quelle": null, "dokumentdatum": "2026-10-15"}`: {
			Issuer: core.Nullable{Set: true}, DocumentDate: core.Nullable{Set: true, Value: zt("2026-10-15")}},
		`{"infos": []}`: {Facts: []core.FactChange{}},
		`{"infos": [{"id": "i1", "einheit": null}, {"id": "i2", "wert": "1", "einheit": "mg"}]}`: {Facts: []core.FactChange{
			{ID: "i1", Unit: core.Nullable{Set: true}},
			{ID: "i2", Value: zt("1"), Unit: core.Nullable{Set: true, Value: zt("mg")}},
		}},
		`{}`: {},
	}
	for body, want := range faelle {
		got, err := leseAenderung([]byte(body))
		if err != nil {
			t.Errorf("%s: %v", body, err)
			continue
		}
		if !reflect.DeepEqual(got, want) {
			t.Errorf("%s:\n%+v\nerwartet\n%+v", body, got, want)
		}
	}
	// "infos" fehlt → nil, nicht leere Liste (kern unterscheidet das).
	if a, _ := leseAenderung([]byte(`{"typ": "rezept"}`)); a.Facts != nil {
		t.Error("Infos ohne Feld nicht nil")
	}
}

func TestKorrigieren_JSONFehler(t *testing.T) {
	faelle := []struct {
		body    string
		status  int
		code    string
		details string // fmt von details, "" für keine
	}{
		{``, 400, CodeUngueltigeAnfrage, ""},
		{`kein json`, 400, CodeUngueltigeAnfrage, ""},
		{`null`, 400, CodeUngueltigeAnfrage, ""},
		{`["typ"]`, 400, CodeUngueltigeAnfrage, ""},
		{`{"typ": "rezept"} {}`, 400, CodeUngueltigeAnfrage, ""},
		{`{"volltext": "neu"}`, 400, CodeFeldNichtKorrigierbar, "map[feld:volltext]"},
		{`{"modul": "sport"}`, 400, CodeFeldNichtKorrigierbar, "map[feld:modul]"},
		{`{"korrekturen": []}`, 400, CodeFeldNichtKorrigierbar, "map[feld:korrekturen]"},
		{`{"datum": "2026-10-15"}`, 400, CodeUngueltigeAnfrage, "map[feld:datum]"},
		{`{"typ": null}`, 400, CodeUngueltigeAnfrage, "map[feld:typ]"},
		{`{"typ": 3}`, 400, CodeUngueltigeAnfrage, "map[feld:typ wert:3]"},
		{`{"dokumentdatum": 20261015}`, 400, CodeUngueltigeAnfrage, "map[feld:dokumentdatum wert:2.0261015e+07]"},
		{`{"quelle": true}`, 400, CodeUngueltigeAnfrage, "map[feld:quelle wert:true]"},
		{`{"infos": null}`, 400, CodeUngueltigeAnfrage, "map[feld:infos]"},
		{`{"infos": {"id": "i1"}}`, 400, CodeUngueltigeAnfrage, "map[feld:infos]"},
		{`{"infos": ["i1"]}`, 400, CodeUngueltigeAnfrage, "map[feld:infos]"},
		{`{"infos": [{"wert": "1"}]}`, 400, CodeUngueltigeAnfrage, "map[feld:infos]"},
		{`{"infos": [{"id": 1, "wert": "1"}]}`, 400, CodeUngueltigeAnfrage, "map[feld:infos]"},
		{`{"infos": [{"id": "i1", "name": "Hb"}]}`, 400, CodeFeldNichtKorrigierbar, "map[feld:infos.i1.name]"},
		{`{"infos": [{"id": "i1", "wret": "1"}]}`, 400, CodeUngueltigeAnfrage, "map[feld:infos.i1.wret]"},
		{`{"infos": [{"id": "i1", "wert": null}]}`, 400, CodeUngueltigeAnfrage, "map[feld:infos.i1.wert]"},
		{`{"infos": [{"id": "i1", "wert": 14.2}]}`, 400, CodeUngueltigeAnfrage, "map[feld:infos.i1.wert wert:14.2]"},
		{`{"infos": [{"id": "i1", "einheit": 5}]}`, 400, CodeUngueltigeAnfrage, "map[feld:infos.i1.einheit wert:5]"},
	}
	for _, f := range faelle {
		t.Run(f.body, func(t *testing.T) {
			k := &korrigiererAttrappe{}
			a := pruefeFehler(t, korrigieren(t, k, idKorrektur, f.body), f.status, f.code)
			if got := details(a); got != f.details {
				t.Errorf("details = %s, erwartet %s", got, f.details)
			}
			if k.aufgerufen {
				t.Error("Dienst trotz JSON-Fehler aufgerufen")
			}
		})
	}
}

func details(a fehlerAntwort) string {
	if len(a.Fehler.Details) == 0 {
		return ""
	}
	return fmt.Sprint(a.Fehler.Details) // fmt sortiert Map-Schlüssel
}

func TestKorrigieren_FesteReihenfolgeDerFehler(t *testing.T) {
	// Zwei Fehler: immer derselbe wird gemeldet (alphabetisch), egal wie
	// Go die Map durchläuft.
	for range 20 {
		a := pruefeFehler(t, korrigieren(t, &korrigiererAttrappe{}, idKorrektur,
			`{"volltext": "x", "modul": "y", "datei": {}}`), 400, CodeFeldNichtKorrigierbar)
		if details(a) != "map[feld:datei]" {
			t.Fatalf("details = %s", details(a))
		}
	}
}

func TestKorrigieren_UngueltigeIDVorDemBody(t *testing.T) {
	k := &korrigiererAttrappe{}
	pruefeFehler(t, korrigieren(t, k, "abc", `kein json`), 400, CodeUngueltigeID)
	if k.aufgerufen {
		t.Error("Dienst aufgerufen")
	}
}

func TestKorrigieren_ZuGrosserBody(t *testing.T) {
	k := &korrigiererAttrappe{}
	body := `{"quelle": "` + strings.Repeat("a", maxKorrekturBody) + `"}`
	pruefeFehler(t, korrigieren(t, k, idKorrektur, body), 400, CodeUngueltigeAnfrage)
	if k.aufgerufen {
		t.Error("Dienst aufgerufen")
	}
}

func TestKorrigieren_FehlerDesDienstes(t *testing.T) {
	faelle := []struct {
		name    string
		err     error
		status  int
		code    string
		details string
		meldung string
	}{
		{"datum", core.Invalid("Das Dokumentdatum ist kein gültiges Kalenderdatum.", "dokumentdatum", "2026-02-31"),
			400, CodeUngueltigeAnfrage, "map[feld:dokumentdatum wert:2026-02-31]", "kein gültiges Kalenderdatum"},
		{"info doppelt", &core.CorrectionError{Kind: core.ErrInvalidCorrection, FactID: "i1", Message: "Die Info i1 kommt mehrfach vor."},
			400, CodeUngueltigeAnfrage, "map[infoId:i1]", "mehrfach"},
		{"leer", core.Invalid("Die Anfrage enthält keine Änderung.", "", nil), 400, CodeUngueltigeAnfrage, "", "keine Änderung"},
		{"info unbekannt", &core.CorrectionError{Kind: core.ErrFactNotFound, FactID: "i9", Message: "Diese Info gibt es in dem Eintrag nicht."},
			404, CodeInfoNichtGefunden, "map[infoId:i9]", "gibt es in dem Eintrag nicht"},
		{"nicht korrigierbar", core.NotCorrectable("volltext"), 400, CodeFeldNichtKorrigierbar, "map[feld:volltext]", "volltext"},
		{"eintrag fehlt", core.ErrEntryNotFound, 404, CodeEintragNichtGefunden, "", "gibt es nicht"},
		{"id", core.ErrInvalidID, 400, CodeUngueltigeID, "", "ungültig"},
		{"intern", errors.New("Mongo: Verbindung weg"), 500, CodeInternerFehler, "", "schiefgegangen"},
	}
	for _, f := range faelle {
		t.Run(f.name, func(t *testing.T) {
			rec := korrigieren(t, &korrigiererAttrappe{err: f.err}, idKorrektur, `{"typ": "rezept"}`)
			a := pruefeFehler(t, rec, f.status, f.code)
			if details(a) != f.details {
				t.Errorf("details = %s, erwartet %s", details(a), f.details)
			}
			if !strings.Contains(a.Fehler.Nachricht, f.meldung) {
				t.Errorf("Nachricht %q enthält nicht %q", a.Fehler.Nachricht, f.meldung)
			}
			if strings.Contains(rec.Body.String(), "Mongo") {
				t.Errorf("interne Ursache in der Antwort: %s", rec.Body)
			}
		})
	}
}

func TestKorrigieren_AbgebrochenSchreibtNichts(t *testing.T) {
	rec := korrigieren(t, &korrigiererAttrappe{err: context.Canceled}, idKorrektur, `{"typ": "rezept"}`)
	if rec.Body.Len() != 0 {
		t.Errorf("Body bei Abbruch: %s", rec.Body)
	}
}
