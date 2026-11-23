package store

import (
	"encoding/json"
	"reflect"
	"strings"
	"testing"
	"time"

	"go.mongodb.org/mongo-driver/v2/bson"

	"lifelog/backend/internal/core"
)

// parsedRoutine builds a routine as the API would, through core.
func parsedRoutine(t *testing.T, body string) core.Routine {
	t.Helper()
	var m map[string]any
	if err := json.Unmarshal([]byte(body), &m); err != nil {
		t.Fatal(err)
	}
	r, err := core.ParseNewRoutine(m)
	if err != nil {
		t.Fatal(err)
	}
	r.ID, r.UserID = NewID(), testOwner
	r.CreatedAt = time.Date(2026, 11, 23, 8, 15, 3, 0, time.UTC)
	return r
}

func TestRoutineDoc_RoundTrip(t *testing.T) {
	for _, body := range []string{
		`{"name": "Wasser", "amount": 250, "unit": "ml", "startDate": "2026-11-23",
			"schedule": {"frequency": "hourly", "interval": 1, "windowStart": "00:00", "windowEnd": "20:00"}}`,
		`{"name": "Blutdruck messen", "startDate": "2026-11-18", "endDate": "2026-12-01",
			"schedule": {"frequency": "daily", "interval": 1, "times": ["20:00", "08:00"]}}`,
		`{"name": "Eisbaden", "startDate": "2026-11-23",
			"schedule": {"frequency": "weekly", "interval": 2, "weekdays": ["fri", "tue"], "times": ["07:00"]}}`,
	} {
		r := parsedRoutine(t, body)
		until := core.DateFromYMD(2026, 11, 30)
		r.Pauses = []core.Pause{{From: core.DateFromYMD(2026, 11, 2), Until: &until}, {From: core.DateFromYMD(2026, 12, 1)}}
		doc, err := toRoutineDoc(r)
		if err != nil {
			t.Fatal(err)
		}
		doc.UserID = mustOID(t, testOwner)
		if doc.NameKey != strings.ToLower(r.Name) {
			t.Errorf("nameKey %q", doc.NameKey)
		}
		back, err := doc.toCore()
		if err != nil {
			t.Fatal(err)
		}
		if !reflect.DeepEqual(back, r) {
			t.Errorf("round trip:\n%+v\n%+v", back, r)
		}
	}
}

// The schedule holds only the fields of its frequency.
func TestScheduleDoc_OnlyOwnFields(t *testing.T) {
	r := parsedRoutine(t, `{"name": "x", "startDate": "2026-11-23",
		"schedule": {"frequency": "hourly", "interval": 3, "windowStart": "08:00", "windowEnd": "20:00"}}`)
	raw, err := bson.MarshalExtJSON(toScheduleDoc(r.Schedule), false, false)
	if err != nil {
		t.Fatal(err)
	}
	if want := `{"frequency":"hourly","interval":3,"windowStart":"08:00","windowEnd":"20:00"}`; string(raw) != want {
		t.Errorf("hourly = %s, want %s", raw, want)
	}
	r = parsedRoutine(t, `{"name": "x", "startDate": "2026-11-23",
		"schedule": {"frequency": "weekly", "interval": 1, "weekdays": ["sun", "mon"], "times": ["07:00"]}}`)
	raw, _ = bson.MarshalExtJSON(toScheduleDoc(r.Schedule), false, false)
	if want := `{"frequency":"weekly","interval":1,"weekdays":["mon","sun"],"times":["07:00"]}`; string(raw) != want {
		t.Errorf("weekly = %s, want %s", raw, want)
	}
}

// A value that cannot be read back is an error that names the field, but
// not the value.
func TestRoutineDoc_InvalidStoredValue(t *testing.T) {
	r := parsedRoutine(t, `{"name": "x", "startDate": "2026-11-23",
		"schedule": {"frequency": "daily", "interval": 1, "times": ["08:00"]}}`)
	good, _ := toRoutineDoc(r)
	cases := map[string]func(d *routineDoc){
		"startDate":         func(d *routineDoc) { d.StartDate = "Geheim-1" },
		"schedule":          func(d *routineDoc) { d.Schedule.Frequency = "monthly" },
		"schedule.times":    func(d *routineDoc) { d.Schedule.Times = []string{"Geheim-2"} },
		"schedule.weekdays": func(d *routineDoc) { d.Schedule.Weekdays = []string{"Geheim-3"} },
		"pauses":            func(d *routineDoc) { d.Pauses = []pauseDoc{{From: "Geheim-4"}} },
	}
	for field, spoil := range cases {
		d := good
		spoil(&d)
		_, err := d.toCore()
		if err == nil || !strings.Contains(err.Error(), field) || strings.Contains(err.Error(), "Geheim") {
			t.Errorf("%s: err = %v", field, err)
		}
	}
}

func TestRoutineLogDoc_RoundTrip(t *testing.T) {
	due := time.Date(2026, 11, 23, 7, 0, 0, 0, time.UTC)
	amount := 2.5
	withOccurrence := core.RoutineLog{
		ID: NewID(), UserID: testOwner, RoutineID: NewID(), DueAt: &due, DueLocal: "2026-11-23T08:00",
		TakenAt: due.Add(4*time.Minute + 12*time.Second), Status: core.LogDone, Amount: &amount,
		Source: core.LogSourceNotification, TimeZone: "Europe/Vienna", CreatedAt: due.Add(5 * time.Minute),
	}
	without := withOccurrence
	without.ID, without.DueAt, without.DueLocal, without.Amount, without.Status = NewID(), nil, "", nil, core.LogSkipped

	for _, l := range []core.RoutineLog{withOccurrence, without} {
		doc, err := toRoutineLogDoc(l)
		if err != nil {
			t.Fatal(err)
		}
		doc.UserID = mustOID(t, testOwner)
		if back := doc.toCore(); !reflect.DeepEqual(back, l) {
			t.Errorf("round trip:\n%+v\n%+v", back, l)
		}
	}

	// Without an occurrence dueLocal is missing, not null: the partial
	// unique index only covers strings.
	doc, _ := toRoutineLogDoc(without)
	raw, err := bson.Marshal(doc)
	if err != nil {
		t.Fatal(err)
	}
	var m bson.M
	if err := bson.Unmarshal(raw, &m); err != nil {
		t.Fatal(err)
	}
	if _, ok := m["dueLocal"]; ok {
		t.Error("dueLocal stored for a log without an occurrence")
	}
	if v, ok := m["dueAt"]; !ok || v != nil {
		t.Errorf("dueAt = %v, %v; want null", v, ok)
	}
	if v, ok := m["entryId"]; !ok || v != nil {
		t.Errorf("entryId = %v, %v; want null", v, ok)
	}
}

func TestRoutineStore_RefusesOtherOwner(t *testing.T) {
	s := &Store{}
	r := core.Routine{ID: NewID(), UserID: NewID()}
	if err := s.CreateRoutine(t.Context(), NewID(), r); err == nil {
		t.Error("a routine of one user must not be saved for another")
	}
	l := core.RoutineLog{ID: NewID(), RoutineID: NewID(), UserID: NewID()}
	if err := s.CreateRoutineLog(t.Context(), NewID(), l); err == nil {
		t.Error("a log of one user must not be saved for another")
	}
}
