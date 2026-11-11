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

const correctionID = "6727a1f3c2b4e5d6f7a8b9c0"

type fakeCorrector struct {
	entry  core.Entry
	err    error
	called bool
	id     string
	change core.Change
}

func (c *fakeCorrector) Correct(_ context.Context, id string, ch core.Change) (core.Entry, error) {
	c.called, c.id, c.change = true, id, ch
	return c.entry, c.err
}

func correct(t *testing.T, c *fakeCorrector, id, body string) *httptest.ResponseRecorder {
	t.Helper()
	router := NewRouter(Dependencies{DB: fakePinger{}, Corrector: c})
	r := httptest.NewRequest(http.MethodPatch, "/api/v1/eintraege/"+id, strings.NewReader(body))
	r.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, r)
	return rec
}

func ptr(s string) *string { return &s }

func TestCorrect_SuccessExactlyAsAPIExample(t *testing.T) {
	at := time.Date(2026, 11, 3, 19, 5, 31, 0, time.UTC)
	c := &fakeCorrector{entry: core.Entry{
		ID: correctionID, Module: "gesundheit", Type: "laborbefund",
		DocumentDate: ptr("2026-10-15"), ImportedAt: time.Date(2026, 11, 3, 18, 42, 7, 0, time.UTC),
		Issuer: ptr("Labor Muster GmbH, Salzburg"), FullText: "Leukozyten 6,3 G/l",
		Facts: []core.Fact{{ID: "i2", Name: "Leukozyten", Value: "6,8", Unit: ptr("G/l")}},
		File:  core.File{ID: "6727a1f3c2b4e5d6f7a8b9c1", FileName: "befund_oktober.pdf", MimeType: "application/pdf", Size: 184320, Sha256: "3f8a"},
		Corrections: []core.Correction{
			{Field: "dokumentdatum", Recognized: ptr("2026-10-14"), Corrected: ptr("2026-10-15"), CorrectedAt: at},
			{Field: "infos.i2.wert", Recognized: ptr("6,3"), Corrected: ptr("6,8"), CorrectedAt: at},
		},
	}}
	rec := correct(t, c, correctionID, `{"dokumentdatum": "2026-10-15", "infos": [{"id": "i2", "wert": "6,8"}]}`)

	if rec.Code != http.StatusOK {
		t.Fatalf("status %d: %s", rec.Code, rec.Body)
	}
	if c.id != correctionID {
		t.Errorf("ID to the service: %q", c.id)
	}
	want := core.Change{
		DocumentDate: core.Nullable{Set: true, Value: ptr("2026-10-15")},
		Facts:        []core.FactChange{{ID: "i2", Value: ptr("6,8")}},
	}
	if !reflect.DeepEqual(c.change, want) {
		t.Errorf("change = %+v, want %+v", c.change, want)
	}
	wantBody := `{"id":"6727a1f3c2b4e5d6f7a8b9c0","modul":"gesundheit","typ":"laborbefund","dokumentdatum":"2026-10-15",` +
		`"importdatum":"2026-11-03T18:42:07Z","quelle":"Labor Muster GmbH, Salzburg","volltext":"Leukozyten 6,3 G/l",` +
		`"infos":[{"id":"i2","name":"Leukozyten","wert":"6,8","einheit":"G/l"}],` +
		`"datei":{"id":"6727a1f3c2b4e5d6f7a8b9c1","dateiname":"befund_oktober.pdf","mimeTyp":"application/pdf","groesse":184320,"sha256":"3f8a"},` +
		`"korrekturen":[{"feld":"dokumentdatum","erkannt":"2026-10-14","korrigiert":"2026-10-15","zeitpunkt":"2026-11-03T19:05:31Z"},` +
		`{"feld":"infos.i2.wert","erkannt":"6,3","korrigiert":"6,8","zeitpunkt":"2026-11-03T19:05:31Z"}]}`
	if got := strings.TrimSpace(rec.Body.String()); got != wantBody {
		t.Errorf("body =\n%s\nwant\n%s", got, wantBody)
	}
}

func TestParseChange_ThreeCases(t *testing.T) {
	// missing / null / value, for all fields.
	cases := map[string]core.Change{
		`{"typ": "rezept"}`:       {Type: ptr("rezept")},
		`{"dokumentdatum": null}`: {DocumentDate: core.Nullable{Set: true}},
		`{"quelle": "  "}`:        {Issuer: core.Nullable{Set: true, Value: ptr("  ")}}, // core removes the whitespace
		`{"quelle": null, "dokumentdatum": "2026-10-15"}`: {
			Issuer: core.Nullable{Set: true}, DocumentDate: core.Nullable{Set: true, Value: ptr("2026-10-15")}},
		`{"infos": []}`: {Facts: []core.FactChange{}},
		`{"infos": [{"id": "i1", "einheit": null}, {"id": "i2", "wert": "1", "einheit": "mg"}]}`: {Facts: []core.FactChange{
			{ID: "i1", Unit: core.Nullable{Set: true}},
			{ID: "i2", Value: ptr("1"), Unit: core.Nullable{Set: true, Value: ptr("mg")}},
		}},
		`{}`: {},
	}
	for body, want := range cases {
		got, err := parseChange([]byte(body))
		if err != nil {
			t.Errorf("%s: %v", body, err)
			continue
		}
		if !reflect.DeepEqual(got, want) {
			t.Errorf("%s:\n%+v\nwant\n%+v", body, got, want)
		}
	}
	// "infos" missing → nil, not an empty list (core tells them apart).
	if c, _ := parseChange([]byte(`{"typ": "rezept"}`)); c.Facts != nil {
		t.Error("facts without the field are not nil")
	}
}

func TestCorrect_JSONErrors(t *testing.T) {
	cases := []struct {
		body    string
		status  int
		code    string
		details string // fmt of details, "" for none
	}{
		{``, 400, CodeInvalidRequest, ""},
		{`kein json`, 400, CodeInvalidRequest, ""},
		{`null`, 400, CodeInvalidRequest, ""},
		{`["typ"]`, 400, CodeInvalidRequest, ""},
		{`{"typ": "rezept"} {}`, 400, CodeInvalidRequest, ""},
		{`{"volltext": "neu"}`, 400, CodeFieldNotCorrectable, "map[feld:volltext]"},
		{`{"modul": "sport"}`, 400, CodeFieldNotCorrectable, "map[feld:modul]"},
		{`{"korrekturen": []}`, 400, CodeFieldNotCorrectable, "map[feld:korrekturen]"},
		{`{"datum": "2026-10-15"}`, 400, CodeInvalidRequest, "map[feld:datum]"},
		{`{"typ": null}`, 400, CodeInvalidRequest, "map[feld:typ]"},
		{`{"typ": 3}`, 400, CodeInvalidRequest, "map[feld:typ wert:3]"},
		{`{"dokumentdatum": 20261015}`, 400, CodeInvalidRequest, "map[feld:dokumentdatum wert:2.0261015e+07]"},
		{`{"quelle": true}`, 400, CodeInvalidRequest, "map[feld:quelle wert:true]"},
		{`{"infos": null}`, 400, CodeInvalidRequest, "map[feld:infos]"},
		{`{"infos": {"id": "i1"}}`, 400, CodeInvalidRequest, "map[feld:infos]"},
		{`{"infos": ["i1"]}`, 400, CodeInvalidRequest, "map[feld:infos]"},
		{`{"infos": [{"wert": "1"}]}`, 400, CodeInvalidRequest, "map[feld:infos]"},
		{`{"infos": [{"id": 1, "wert": "1"}]}`, 400, CodeInvalidRequest, "map[feld:infos]"},
		{`{"infos": [{"id": "i1", "name": "Hb"}]}`, 400, CodeFieldNotCorrectable, "map[feld:infos.i1.name]"},
		{`{"infos": [{"id": "i1", "wret": "1"}]}`, 400, CodeInvalidRequest, "map[feld:infos.i1.wret]"},
		{`{"infos": [{"id": "i1", "wert": null}]}`, 400, CodeInvalidRequest, "map[feld:infos.i1.wert]"},
		{`{"infos": [{"id": "i1", "wert": 14.2}]}`, 400, CodeInvalidRequest, "map[feld:infos.i1.wert wert:14.2]"},
		{`{"infos": [{"id": "i1", "einheit": 5}]}`, 400, CodeInvalidRequest, "map[feld:infos.i1.einheit wert:5]"},
	}
	for _, c := range cases {
		t.Run(c.body, func(t *testing.T) {
			fc := &fakeCorrector{}
			a := checkError(t, correct(t, fc, correctionID, c.body), c.status, c.code)
			if got := details(a); got != c.details {
				t.Errorf("details = %s, want %s", got, c.details)
			}
			if fc.called {
				t.Error("service called despite a JSON error")
			}
		})
	}
}

func details(a errorResponse) string {
	if len(a.Error.Details) == 0 {
		return ""
	}
	return fmt.Sprint(a.Error.Details) // fmt sorts map keys
}

func TestCorrect_FixedOrderOfErrors(t *testing.T) {
	// Two errors: always the same one is reported (alphabetically), however
	// Go iterates the map.
	for range 20 {
		a := checkError(t, correct(t, &fakeCorrector{}, correctionID,
			`{"volltext": "x", "modul": "y", "datei": {}}`), 400, CodeFieldNotCorrectable)
		if details(a) != "map[feld:datei]" {
			t.Fatalf("details = %s", details(a))
		}
	}
}

func TestCorrect_InvalidIDBeforeBody(t *testing.T) {
	c := &fakeCorrector{}
	checkError(t, correct(t, c, "abc", `kein json`), 400, CodeInvalidID)
	if c.called {
		t.Error("service called")
	}
}

func TestCorrect_BodyTooLarge(t *testing.T) {
	c := &fakeCorrector{}
	body := `{"quelle": "` + strings.Repeat("a", maxCorrectionBody) + `"}`
	checkError(t, correct(t, c, correctionID, body), 400, CodeInvalidRequest)
	if c.called {
		t.Error("service called")
	}
}

func TestCorrect_ServiceErrors(t *testing.T) {
	cases := []struct {
		name    string
		err     error
		status  int
		code    string
		details string
		message string
	}{
		{"date", core.Invalid("Das Dokumentdatum ist kein gültiges Kalenderdatum.", "dokumentdatum", "2026-02-31"),
			400, CodeInvalidRequest, "map[feld:dokumentdatum wert:2026-02-31]", "kein gültiges Kalenderdatum"},
		{"fact twice", &core.CorrectionError{Kind: core.ErrInvalidCorrection, FactID: "i1", Message: "Die Info i1 kommt mehrfach vor."},
			400, CodeInvalidRequest, "map[infoId:i1]", "mehrfach"},
		{"empty", core.Invalid("Die Anfrage enthält keine Änderung.", "", nil), 400, CodeInvalidRequest, "", "keine Änderung"},
		{"unknown fact", &core.CorrectionError{Kind: core.ErrFactNotFound, FactID: "i9", Message: "Diese Info gibt es in dem Eintrag nicht."},
			404, CodeFactNotFound, "map[infoId:i9]", "gibt es in dem Eintrag nicht"},
		{"not correctable", core.NotCorrectable("volltext"), 400, CodeFieldNotCorrectable, "map[feld:volltext]", "volltext"},
		{"entry missing", core.ErrEntryNotFound, 404, CodeEntryNotFound, "", "gibt es nicht"},
		{"id", core.ErrInvalidID, 400, CodeInvalidID, "", "ungültig"},
		{"internal", errors.New("Mongo: connection gone"), 500, CodeInternalError, "", "schiefgegangen"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			rec := correct(t, &fakeCorrector{err: c.err}, correctionID, `{"typ": "rezept"}`)
			a := checkError(t, rec, c.status, c.code)
			if details(a) != c.details {
				t.Errorf("details = %s, want %s", details(a), c.details)
			}
			if !strings.Contains(a.Error.Message, c.message) {
				t.Errorf("message %q does not contain %q", a.Error.Message, c.message)
			}
			if strings.Contains(rec.Body.String(), "Mongo") {
				t.Errorf("internal cause in the response: %s", rec.Body)
			}
		})
	}
}

func TestCorrect_CanceledWritesNothing(t *testing.T) {
	rec := correct(t, &fakeCorrector{err: context.Canceled}, correctionID, `{"typ": "rezept"}`)
	if rec.Body.Len() != 0 {
		t.Errorf("body on cancel: %s", rec.Body)
	}
}
