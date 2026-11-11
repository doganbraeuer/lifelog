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
	r := httptest.NewRequest(http.MethodPatch, "/api/v1/entries/"+id, strings.NewReader(body))
	r.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, r)
	return rec
}

func ptr(s string) *string { return &s }

func TestCorrect_SuccessExactlyAsAPIExample(t *testing.T) {
	at := time.Date(2026, 11, 3, 19, 5, 31, 0, time.UTC)
	c := &fakeCorrector{entry: core.Entry{
		ID: correctionID, Module: "health", Type: "lab_report",
		DocumentDate: ptr("2026-10-15"), ImportedAt: time.Date(2026, 11, 3, 18, 42, 7, 0, time.UTC),
		Issuer: ptr("Labor Muster GmbH, Salzburg"), FullText: "Leukozyten 6,3 G/l",
		Facts: []core.Fact{{ID: "f2", Name: "Leukozyten", Value: "6,8", Unit: ptr("G/l")}},
		File:  core.File{ID: "6727a1f3c2b4e5d6f7a8b9c1", FileName: "befund_oktober.pdf", MimeType: "application/pdf", Size: 184320, Sha256: "3f8a"},
		Corrections: []core.Correction{
			{Field: "documentDate", Recognized: ptr("2026-10-14"), Corrected: ptr("2026-10-15"), CorrectedAt: at},
			{Field: "facts.f2.value", Recognized: ptr("6,3"), Corrected: ptr("6,8"), CorrectedAt: at},
		},
	}}
	rec := correct(t, c, correctionID, `{"documentDate": "2026-10-15", "facts": [{"id": "f2", "value": "6,8"}]}`)

	if rec.Code != http.StatusOK {
		t.Fatalf("status %d: %s", rec.Code, rec.Body)
	}
	if c.id != correctionID {
		t.Errorf("ID to the service: %q", c.id)
	}
	want := core.Change{
		DocumentDate: core.Nullable{Set: true, Value: ptr("2026-10-15")},
		Facts:        []core.FactChange{{ID: "f2", Value: ptr("6,8")}},
	}
	if !reflect.DeepEqual(c.change, want) {
		t.Errorf("change = %+v, want %+v", c.change, want)
	}
	wantBody := `{"id":"6727a1f3c2b4e5d6f7a8b9c0","module":"health","type":"lab_report","documentDate":"2026-10-15",` +
		`"importedAt":"2026-11-03T18:42:07Z","issuer":"Labor Muster GmbH, Salzburg","fullText":"Leukozyten 6,3 G/l",` +
		`"facts":[{"id":"f2","name":"Leukozyten","value":"6,8","unit":"G/l"}],` +
		`"file":{"id":"6727a1f3c2b4e5d6f7a8b9c1","fileName":"befund_oktober.pdf","mimeType":"application/pdf","size":184320,"sha256":"3f8a"},` +
		`"corrections":[{"field":"documentDate","recognized":"2026-10-14","corrected":"2026-10-15","correctedAt":"2026-11-03T19:05:31Z"},` +
		`{"field":"facts.f2.value","recognized":"6,3","corrected":"6,8","correctedAt":"2026-11-03T19:05:31Z"}]}`
	if got := strings.TrimSpace(rec.Body.String()); got != wantBody {
		t.Errorf("body =\n%s\nwant\n%s", got, wantBody)
	}
}

func TestParseChange_ThreeCases(t *testing.T) {
	// missing / null / value, for all fields.
	cases := map[string]core.Change{
		`{"type": "prescription"}`: {Type: ptr("prescription")},
		`{"documentDate": null}`:   {DocumentDate: core.Nullable{Set: true}},
		`{"issuer": "  "}`:         {Issuer: core.Nullable{Set: true, Value: ptr("  ")}}, // core removes the whitespace
		`{"issuer": null, "documentDate": "2026-10-15"}`: {
			Issuer: core.Nullable{Set: true}, DocumentDate: core.Nullable{Set: true, Value: ptr("2026-10-15")}},
		`{"facts": []}`: {Facts: []core.FactChange{}},
		`{"facts": [{"id": "f1", "unit": null}, {"id": "f2", "value": "1", "unit": "mg"}]}`: {Facts: []core.FactChange{
			{ID: "f1", Unit: core.Nullable{Set: true}},
			{ID: "f2", Value: ptr("1"), Unit: core.Nullable{Set: true, Value: ptr("mg")}},
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
	// "facts" missing → nil, not an empty list (core tells them apart).
	if c, _ := parseChange([]byte(`{"type": "prescription"}`)); c.Facts != nil {
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
		{`["type"]`, 400, CodeInvalidRequest, ""},
		{`{"type": "prescription"} {}`, 400, CodeInvalidRequest, ""},
		{`{"fullText": "neu"}`, 400, CodeFieldNotCorrectable, "map[field:fullText]"},
		{`{"module": "sport"}`, 400, CodeFieldNotCorrectable, "map[field:module]"},
		{`{"corrections": []}`, 400, CodeFieldNotCorrectable, "map[field:corrections]"},
		{`{"datum": "2026-10-15"}`, 400, CodeInvalidRequest, "map[field:datum]"},
		{`{"type": null}`, 400, CodeInvalidRequest, "map[field:type]"},
		{`{"type": 3}`, 400, CodeInvalidRequest, "map[field:type value:3]"},
		{`{"documentDate": 20261015}`, 400, CodeInvalidRequest, "map[field:documentDate value:2.0261015e+07]"},
		{`{"issuer": true}`, 400, CodeInvalidRequest, "map[field:issuer value:true]"},
		{`{"facts": null}`, 400, CodeInvalidRequest, "map[field:facts]"},
		{`{"facts": {"id": "f1"}}`, 400, CodeInvalidRequest, "map[field:facts]"},
		{`{"facts": ["f1"]}`, 400, CodeInvalidRequest, "map[field:facts]"},
		{`{"facts": [{"value": "1"}]}`, 400, CodeInvalidRequest, "map[field:facts]"},
		{`{"facts": [{"id": 1, "value": "1"}]}`, 400, CodeInvalidRequest, "map[field:facts]"},
		{`{"facts": [{"id": "f1", "name": "Hb"}]}`, 400, CodeFieldNotCorrectable, "map[field:facts.f1.name]"},
		{`{"facts": [{"id": "f1", "wret": "1"}]}`, 400, CodeInvalidRequest, "map[field:facts.f1.wret]"},
		{`{"facts": [{"id": "f1", "value": null}]}`, 400, CodeInvalidRequest, "map[field:facts.f1.value]"},
		{`{"facts": [{"id": "f1", "value": 14.2}]}`, 400, CodeInvalidRequest, "map[field:facts.f1.value value:14.2]"},
		{`{"facts": [{"id": "f1", "unit": 5}]}`, 400, CodeInvalidRequest, "map[field:facts.f1.unit value:5]"},
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
			`{"fullText": "x", "module": "y", "file": {}}`), 400, CodeFieldNotCorrectable)
		if details(a) != "map[field:file]" {
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
	body := `{"issuer": "` + strings.Repeat("a", maxCorrectionBody) + `"}`
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
		{"date", core.Invalid("Das Dokumentdatum ist kein gültiges Kalenderdatum.", "documentDate", "2026-02-31"),
			400, CodeInvalidRequest, "map[field:documentDate value:2026-02-31]", "kein gültiges Kalenderdatum"},
		{"fact twice", &core.CorrectionError{Kind: core.ErrInvalidCorrection, FactID: "f1", Message: "Die Info f1 kommt mehrfach vor."},
			400, CodeInvalidRequest, "map[factId:f1]", "mehrfach"},
		{"empty", core.Invalid("Die Anfrage enthält keine Änderung.", "", nil), 400, CodeInvalidRequest, "", "keine Änderung"},
		{"unknown fact", &core.CorrectionError{Kind: core.ErrFactNotFound, FactID: "f9", Message: "Diese Info gibt es in dem Eintrag nicht."},
			404, CodeFactNotFound, "map[factId:f9]", "gibt es in dem Eintrag nicht"},
		{"not correctable", core.NotCorrectable("fullText"), 400, CodeFieldNotCorrectable, "map[field:fullText]", "fullText"},
		{"entry missing", core.ErrEntryNotFound, 404, CodeEntryNotFound, "", "gibt es nicht"},
		{"id", core.ErrInvalidID, 400, CodeInvalidID, "", "ungültig"},
		{"internal", errors.New("Mongo: connection gone"), 500, CodeInternalError, "", "schiefgegangen"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			rec := correct(t, &fakeCorrector{err: c.err}, correctionID, `{"type": "prescription"}`)
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
	rec := correct(t, &fakeCorrector{err: context.Canceled}, correctionID, `{"type": "prescription"}`)
	if rec.Body.Len() != 0 {
		t.Errorf("body on cancel: %s", rec.Body)
	}
}
