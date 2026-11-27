package core

import (
	"encoding/json"
	"errors"
	"fmt"
	"reflect"
	"slices"
	"strings"
	"testing"
	"time"
)

// jsonBody decodes a request body as the API does: encoding/json into a
// map[string]any. A key given twice keeps its last value, which the tables
// below use to override a field of a valid body.
func jsonBody(t *testing.T, s string) map[string]any {
	t.Helper()
	var m map[string]any
	if err := json.Unmarshal([]byte(s), &m); err != nil {
		t.Fatalf("test body is not JSON: %v\n%s", err, s)
	}
	return m
}

// checkRoutineError checks field, message and value of a *RoutineError.
func checkRoutineError(t *testing.T, err error, field, message string, value any) {
	t.Helper()
	var re *RoutineError
	if !errors.As(err, &re) || !errors.Is(err, ErrInvalidRoutine) {
		t.Fatalf("err = %v, want a *RoutineError", err)
	}
	if re.Field != field || re.Message != message {
		t.Errorf("error = %q / %q, want %q / %q", re.Field, re.Message, field, message)
	}
	if !reflect.DeepEqual(re.Value, value) {
		t.Errorf("value = %#v, want %#v", re.Value, value)
	}
	// The value never appears in Error(), which may end up in the log.
	if s, ok := value.(string); ok && len(strings.TrimSpace(s)) >= 3 && strings.Contains(err.Error(), s) {
		t.Errorf("Error() %q contains the value %q", err.Error(), s)
	}
}

func date(t *testing.T, s string) Date {
	t.Helper()
	d, ok := ParseDate(s)
	if !ok {
		t.Fatalf("bad test date %q", s)
	}
	return d
}

func clocks(t *testing.T, list ...string) []ClockTime {
	t.Helper()
	out := make([]ClockTime, 0, len(list))
	for _, s := range list {
		c, ok := ParseClockTime(s)
		if !ok {
			t.Fatalf("bad test time %q", s)
		}
		out = append(out, c)
	}
	return out
}

func float(f float64) *float64 { return &f }

func TestParseNewRoutine_Example(t *testing.T) {
	// The request of API.md 3.18.
	r, err := ParseNewRoutine(jsonBody(t, `{
		"name": "Wasser",
		"amount": 250,
		"unit": "ml",
		"schedule": { "frequency": "hourly", "interval": 1, "windowStart": "08:00", "windowEnd": "20:00" },
		"startDate": "2026-11-23"
	}`))
	if err != nil {
		t.Fatal(err)
	}
	want := Routine{
		Module: "health",
		Name:   "Wasser",
		Amount: float(250),
		Unit:   ptr("ml"),
		Schedule: Schedule{Frequency: FrequencyHourly, Interval: 1,
			WindowStart: clocks(t, "08:00")[0], WindowEnd: clocks(t, "20:00")[0]},
		StartDate: date(t, "2026-11-23"),
		Pauses:    []Pause{},
	}
	if !reflect.DeepEqual(r, want) {
		t.Errorf("routine =\n%+v\nwant\n%+v", r, want)
	}
	if got := len(r.Schedule.WallTimes()); got != 13 {
		t.Errorf("%d wall times, want 13 (08:00 to 20:00)", got)
	}
}

func TestParseNewRoutine_Valid(t *testing.T) {
	daily := `"schedule": {"frequency": "daily", "interval": 1, "times": ["08:00"]}, "startDate": "2026-11-23"`
	cases := []struct {
		name  string
		body  string
		check func(t *testing.T, r Routine)
	}{
		{"name trimmed", `{"name": "  Vitamin D  ", ` + daily + `}`, func(t *testing.T, r Routine) {
			if r.Name != "Vitamin D" {
				t.Errorf("name %q", r.Name)
			}
		}},
		{"60 characters", `{"name": "` + strings.Repeat("ä", 60) + `", ` + daily + `}`, nil},
		{"amount without unit", `{"name": "Liegestütze", "amount": 20, ` + daily + `}`, func(t *testing.T, r Routine) {
			if r.Amount == nil || *r.Amount != 20 || r.Unit != nil {
				t.Errorf("amount %v, unit %v", r.Amount, r.Unit)
			}
		}},
		{"amount 0.5", `{"name": "x", "amount": 0.5, ` + daily + `}`, nil},
		{"amount 100000", `{"name": "x", "amount": 100000, ` + daily + `}`, nil},
		{"empty unit is null", `{"name": "x", "amount": 2, "unit": "  ", ` + daily + `}`, func(t *testing.T, r Routine) {
			if r.Unit != nil {
				t.Errorf("unit %q", *r.Unit)
			}
		}},
		{"empty unit without amount", `{"name": "x", "unit": "", ` + daily + `}`, nil},
		{"unit trimmed", `{"name": "x", "amount": 2, "unit": " Stück ", ` + daily + `}`, func(t *testing.T, r Routine) {
			if r.Unit == nil || *r.Unit != "Stück" {
				t.Errorf("unit %v", r.Unit)
			}
		}},
		{"20-character unit", `{"name": "x", "amount": 2, "unit": "` + strings.Repeat("u", 20) + `", ` + daily + `}`, nil},
		{"nulls", `{"name": "x", "amount": null, "unit": null, "endDate": null, "module": null, ` + daily + `}`, func(t *testing.T, r Routine) {
			if r.Amount != nil || r.Unit != nil || r.EndDate != nil || r.Module != "health" {
				t.Errorf("%+v", r)
			}
		}},
		{"module health", `{"name": "x", "module": "health", ` + daily + `}`, nil},
		{"times sorted", `{"name": "x", "schedule": {"frequency": "daily", "interval": 2, "times": ["20:00", "00:00", "23:59", "08:00"]}, "startDate": "2026-11-23"}`,
			func(t *testing.T, r Routine) {
				if !slices.Equal(r.Schedule.Times, clocks(t, "00:00", "08:00", "20:00", "23:59")) || r.Schedule.Interval != 2 {
					t.Errorf("schedule %+v", r.Schedule)
				}
			}},
		{"24 times", `{"name": "x", "schedule": {"frequency": "daily", "interval": 1, "times": [` + hourList(24) + `]}, "startDate": "2026-11-23"}`, nil},
		{"weekdays Monday first", `{"name": "x", "schedule": {"frequency": "weekly", "interval": 2, "weekdays": ["sun", "fri", "mon"], "times": ["07:00"]}, "startDate": "2026-11-23"}`,
			func(t *testing.T, r Routine) {
				want := []time.Weekday{time.Monday, time.Friday, time.Sunday}
				if !slices.Equal(r.Schedule.Weekdays, want) {
					t.Errorf("weekdays %v", r.Schedule.Weekdays)
				}
			}},
		{"all 7 weekdays", `{"name": "x", "schedule": {"frequency": "weekly", "interval": 1, "weekdays": ["mon", "tue", "wed", "thu", "fri", "sat", "sun"], "times": ["07:00"]}, "startDate": "2026-11-23"}`, nil},
		{"hourly 23", `{"name": "x", "schedule": {"frequency": "hourly", "interval": 23, "windowStart": "00:00", "windowEnd": "23:59"}, "startDate": "2026-11-23"}`, nil},
		{"daily 365", `{"name": "x", "schedule": {"frequency": "daily", "interval": 365, "times": ["08:00"]}, "startDate": "2026-11-23"}`, nil},
		{"weekly 52", `{"name": "x", "schedule": {"frequency": "weekly", "interval": 52, "weekdays": ["mon"], "times": ["08:00"]}, "startDate": "2026-11-23"}`, nil},
		{"interval 2.0", `{"name": "x", "schedule": {"frequency": "daily", "interval": 2.0, "times": ["08:00"]}, "startDate": "2026-11-23"}`, nil},
		{"end equals start", `{"name": "x", ` + daily + `, "endDate": "2026-11-23"}`, func(t *testing.T, r Routine) {
			if r.EndDate == nil || *r.EndDate != r.StartDate {
				t.Errorf("endDate %v", r.EndDate)
			}
		}},
		{"start in the past, first and last allowed day", `{"name": "x", "schedule": {"frequency": "daily", "interval": 1, "times": ["08:00"]}, "startDate": "2000-01-01", "endDate": "2099-12-31"}`, nil},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			r, err := ParseNewRoutine(jsonBody(t, c.body))
			if err != nil {
				t.Fatalf("err = %v", err)
			}
			if c.check != nil {
				c.check(t, r)
			}
		})
	}
}

// hourList gives n different times "00:00", "01:00", …, as JSON strings.
func hourList(n int) string {
	parts := make([]string, n)
	for i := range n {
		parts[i] = fmt.Sprintf(`"%02d:%02d"`, i%24, i/24)
	}
	return strings.Join(parts, ", ")
}

func TestParseNewRoutine_Errors(t *testing.T) {
	// base is a valid body; a case appends fields, and a repeated key
	// replaces the one in base.
	const base = `{"name": "Wasser", "amount": 250, "unit": "ml", "startDate": "2026-11-23",
		"schedule": {"frequency": "daily", "interval": 1, "times": ["08:00"]}`
	hourly := func(fields string) string {
		return base + `, "schedule": {"frequency": "hourly", "interval": 1` + fields + `}}`
	}
	daily := func(fields string) string {
		return base + `, "schedule": {"frequency": "daily", "interval": 1` + fields + `}}`
	}
	weekly := func(fields string) string {
		return base + `, "schedule": {"frequency": "weekly", "interval": 1, "times": ["07:00"]` + fields + `}}`
	}
	interval := func(freq, value string) string {
		extra := map[string]string{
			"hourly": `, "windowStart": "08:00", "windowEnd": "20:00"`,
			"daily":  `, "times": ["08:00"]`,
			"weekly": `, "weekdays": ["mon"], "times": ["08:00"]`,
		}[freq]
		return base + `, "schedule": {"frequency": "` + freq + `", "interval": ` + value + extra + `}}`
	}

	cases := []struct {
		name, body, field, message string
		value                      any
	}{
		// Unknown fields come first, in alphabetical order.
		{"unknown field", base + `, "color": "blue"}`, "color", "Unbekanntes Feld: color.", nil},
		{"id is unknown here", base + `, "id": "6742c0a1b2c3d4e5f6a7b8c9"}`, "id", "Unbekanntes Feld: id.", nil},
		{"status is unknown here", base + `, "status": "active"}`, "status", "Unbekanntes Feld: status.", nil},
		{"unknown before invalid", base + `, "name": "", "zz": 1}`, "zz", "Unbekanntes Feld: zz.", nil},

		{"module unknown", base + `, "module": "nutrition"}`, "module", msgModule, "nutrition"},
		{"module case", base + `, "module": "Health"}`, "module", msgModule, "Health"},
		{"module number", base + `, "module": 1}`, "module", msgModule, float64(1)},

		{"name missing", `{"startDate": "2026-11-23", "schedule": {"frequency": "daily", "interval": 1, "times": ["08:00"]}}`, "name", msgName, nil},
		{"name null", base + `, "name": null}`, "name", msgName, nil},
		{"name empty", base + `, "name": ""}`, "name", msgName, ""},
		{"name blank", base + `, "name": "   "}`, "name", msgName, "   "},
		{"name 61 characters", base + `, "name": "` + strings.Repeat("ä", 61) + `"}`, "name", msgName, strings.Repeat("ä", 61)},
		{"name with tab", base + `, "name": "Was\tser"}`, "name", msgName, "Was\tser"},
		{"name with line break", base + `, "name": "Was\nser"}`, "name", msgName, "Was\nser"},
		{"name number", base + `, "name": 5}`, "name", msgName, float64(5)},
		{"name before amount", base + `, "name": "", "amount": 0}`, "name", msgName, ""},

		{"amount 0", base + `, "amount": 0}`, "amount", msgAmount, float64(0)},
		{"amount negative", base + `, "amount": -1}`, "amount", msgAmount, float64(-1)},
		{"amount too large", base + `, "amount": 100000.01}`, "amount", msgAmount, 100000.01},
		{"amount string", base + `, "amount": "250"}`, "amount", msgAmount, "250"},
		{"amount bool", base + `, "amount": true}`, "amount", msgAmount, true},
		{"amount before unit", base + `, "amount": 0, "unit": 5}`, "amount", msgAmount, float64(0)},

		{"unit without amount", base + `, "amount": null, "unit": "ml"}`, "unit", msgUnitNeedsAmount, "ml"},
		{"unit 21 characters", base + `, "unit": "` + strings.Repeat("u", 21) + `"}`, "unit", msgUnitLength, strings.Repeat("u", 21)},
		{"unit with line break", base + `, "unit": "m\nl"}`, "unit", msgUnitLength, "m\nl"},
		{"unit number", base + `, "unit": 5}`, "unit", msgUnitType, float64(5)},

		{"schedule missing", `{"name": "x", "startDate": "2026-11-23"}`, "schedule", msgSchedule, nil},
		{"schedule null", base + `, "schedule": null}`, "schedule", msgSchedule, nil},
		{"schedule string", base + `, "schedule": "daily"}`, "schedule", msgSchedule, "daily"},
		{"schedule list", base + `, "schedule": []}`, "schedule", msgSchedule, []any{}},
		{"schedule unknown field", daily(`, "times": ["08:00"], "every": 2`), "schedule.every", "Unbekanntes Feld: schedule.every.", nil},
		{"frequency missing", base + `, "schedule": {"interval": 1, "times": ["08:00"]}}`, "schedule.frequency", msgFrequency, nil},
		{"frequency unknown", base + `, "schedule": {"frequency": "monthly", "interval": 1}}`, "schedule.frequency", msgFrequency, "monthly"},
		{"frequency case", base + `, "schedule": {"frequency": "Daily", "interval": 1, "times": ["08:00"]}}`, "schedule.frequency", msgFrequency, "Daily"},

		{"times with hourly", hourly(`, "windowStart": "08:00", "windowEnd": "20:00", "times": ["08:00"]`), "schedule.times", msgForeignField, nil},
		{"times null with hourly", hourly(`, "windowStart": "08:00", "windowEnd": "20:00", "times": null`), "schedule.times", msgForeignField, nil},
		{"windowStart with daily", daily(`, "times": ["08:00"], "windowStart": "08:00"`), "schedule.windowStart", msgForeignField, nil},
		{"weekdays with daily", daily(`, "times": ["08:00"], "weekdays": ["mon"]`), "schedule.weekdays", msgForeignField, nil},
		{"foreign before interval", base + `, "schedule": {"frequency": "daily", "interval": 0, "times": ["08:00"], "windowEnd": "09:00"}}`, "schedule.windowEnd", msgForeignField, nil},

		{"hourly interval 0", interval("hourly", "0"), "schedule.interval", "Das Intervall muss zwischen 1 und 23 liegen.", float64(0)},
		{"hourly interval 24", interval("hourly", "24"), "schedule.interval", "Das Intervall muss zwischen 1 und 23 liegen.", float64(24)},
		{"daily interval 366", interval("daily", "366"), "schedule.interval", "Das Intervall muss zwischen 1 und 365 liegen.", float64(366)},
		{"weekly interval 53", interval("weekly", "53"), "schedule.interval", "Das Intervall muss zwischen 1 und 52 liegen.", float64(53)},
		{"interval 1.5", interval("daily", "1.5"), "schedule.interval", "Das Intervall muss zwischen 1 und 365 liegen.", 1.5},
		{"interval string", interval("daily", `"1"`), "schedule.interval", "Das Intervall muss zwischen 1 und 365 liegen.", "1"},
		{"interval huge", interval("daily", "1e300"), "schedule.interval", "Das Intervall muss zwischen 1 und 365 liegen.", 1e300},
		{"interval missing", base + `, "schedule": {"frequency": "daily", "times": ["08:00"]}}`, "schedule.interval", "Das Intervall muss zwischen 1 und 365 liegen.", nil},

		{"time 8:00", daily(`, "times": ["8:00"]`), "schedule.times", msgTimeFormat, "8:00"},
		{"time 24:00", daily(`, "times": ["08:00", "24:00"]`), "schedule.times", msgTimeFormat, "24:00"},
		{"time number", daily(`, "times": [8]`), "schedule.times", msgTimeFormat, float64(8)},
		{"times string", daily(`, "times": "08:00"`), "schedule.times", msgTimeFormat, "08:00"},
		{"times empty", daily(`, "times": []`), "schedule.times", msgTimesEmpty, nil},
		{"times missing", daily(``), "schedule.times", msgTimesEmpty, nil},
		{"times null", daily(`, "times": null`), "schedule.times", msgTimesEmpty, nil},
		{"25 times", daily(`, "times": [` + hourList(25) + `]`), "schedule.times", msgTimesMax, nil},
		{"time twice", daily(`, "times": ["08:00", "20:00", "08:00"]`), "schedule.times", msgTimesDuplicate, "08:00"},

		{"windowStart 8:00", hourly(`, "windowStart": "8:00", "windowEnd": "20:00"`), "schedule.windowStart", msgTimeFormat, "8:00"},
		{"windowStart missing", hourly(`, "windowEnd": "20:00"`), "schedule.windowStart", msgTimeFormat, nil},
		{"windowEnd missing", hourly(`, "windowStart": "08:00"`), "schedule.windowEnd", msgTimeFormat, nil},
		{"window empty", hourly(`, "windowStart": "08:00", "windowEnd": "08:00"`), "schedule.windowEnd", msgWindowOrder, "08:00"},
		{"window reversed", hourly(`, "windowStart": "20:00", "windowEnd": "08:00"`), "schedule.windowEnd", msgWindowOrder, "08:00"},

		{"weekdays missing", weekly(``), "schedule.weekdays", msgWeekdaysEmpty, nil},
		{"weekdays empty", weekly(`, "weekdays": []`), "schedule.weekdays", msgWeekdaysEmpty, []any{}},
		{"weekdays string", weekly(`, "weekdays": "tue"`), "schedule.weekdays", msgWeekdaysEmpty, "tue"},
		{"weekday unknown", weekly(`, "weekdays": ["mon", "mo"]`), "schedule.weekdays", msgWeekdayUnknown, "mo"},
		{"weekday case", weekly(`, "weekdays": ["Tue"]`), "schedule.weekdays", msgWeekdayUnknown, "Tue"},
		{"weekday number", weekly(`, "weekdays": [1]`), "schedule.weekdays", msgWeekdayUnknown, float64(1)},
		{"weekday twice", weekly(`, "weekdays": ["tue", "fri", "tue"]`), "schedule.weekdays", msgWeekdayDuplicate, "tue"},
		{"weekdays before times", base + `, "schedule": {"frequency": "weekly", "interval": 1, "weekdays": [], "times": []}}`, "schedule.weekdays", msgWeekdaysEmpty, []any{}},

		{"startDate missing", `{"name": "x", "schedule": {"frequency": "daily", "interval": 1, "times": ["08:00"]}}`, "startDate", msgDateFormat, nil},
		{"startDate null", base + `, "startDate": null}`, "startDate", msgDateFormat, nil},
		{"startDate 30 February", base + `, "startDate": "2026-02-30"}`, "startDate", msgDateFormat, "2026-02-30"},
		{"startDate short", base + `, "startDate": "2026-2-3"}`, "startDate", msgDateFormat, "2026-2-3"},
		{"startDate German", base + `, "startDate": "23.11.2026"}`, "startDate", msgDateFormat, "23.11.2026"},
		{"startDate number", base + `, "startDate": 20261123}`, "startDate", msgDateFormat, float64(20261123)},
		{"startDate 1999", base + `, "startDate": "1999-12-31"}`, "startDate", msgDateRange, "1999-12-31"},
		{"startDate 2100", base + `, "startDate": "2100-01-01"}`, "startDate", msgDateRange, "2100-01-01"},
		{"endDate invalid", base + `, "endDate": "2026-11-31"}`, "endDate", msgDateFormat, "2026-11-31"},
		{"endDate 2100", base + `, "endDate": "2100-01-01"}`, "endDate", msgDateRange, "2100-01-01"},
		{"endDate before startDate", base + `, "endDate": "2026-11-22"}`, "endDate", msgEndBeforeStart, "2026-11-22"},
		{"schedule before startDate", base + `, "schedule": null, "startDate": "x"}`, "schedule", msgSchedule, nil},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			_, err := ParseNewRoutine(jsonBody(t, c.body))
			checkRoutineError(t, err, c.field, c.message, c.value)
		})
	}

	t.Run("not an object", func(t *testing.T) {
		_, err := ParseNewRoutine(nil)
		checkRoutineError(t, err, "", msgRoutineBody, nil)
	})
}

func TestScheduleWallTimes(t *testing.T) {
	cases := []struct {
		schedule Schedule
		want     []ClockTime
	}{
		// API.md 3.16: 08:00 to 20:00, every 3 hours.
		{Schedule{Frequency: FrequencyHourly, Interval: 3, WindowStart: 480, WindowEnd: 1200}, clocks(t, "08:00", "11:00", "14:00", "17:00", "20:00")},
		// windowEnd is not hit: the last occurrence is 17:00.
		{Schedule{Frequency: FrequencyHourly, Interval: 3, WindowStart: 480, WindowEnd: 1140}, clocks(t, "08:00", "11:00", "14:00", "17:00")},
		{Schedule{Frequency: FrequencyHourly, Interval: 23, WindowStart: 0, WindowEnd: 1439}, clocks(t, "00:00", "23:00")},
		{Schedule{Frequency: FrequencyDaily, Interval: 2, Times: clocks(t, "08:00", "20:00")}, clocks(t, "08:00", "20:00")},
	}
	for _, c := range cases {
		if got := c.schedule.WallTimes(); !slices.Equal(got, c.want) {
			t.Errorf("WallTimes(%+v) = %v, want %v", c.schedule, got, c.want)
		}
	}
}

// patchBase is the routine the PATCH tests start from: „Blutdruck messen“
// with 2 Stück (the amount only so the unit rules can be tested).
func patchBase(t *testing.T) Routine {
	t.Helper()
	r, err := ParseNewRoutine(jsonBody(t, `{"name": "Blutdruck messen", "amount": 2, "unit": "Stück",
		"schedule": {"frequency": "daily", "interval": 1, "times": ["08:00", "20:00"]},
		"startDate": "2026-11-18", "endDate": "2026-12-01"}`))
	if err != nil {
		t.Fatal(err)
	}
	r.ID, r.UserID = "6742c0a1b2c3d4e5f6a7b8c9", "673b0c1fa2d4e5f6a7b8c9d0"
	r.CreatedAt = time.Date(2026, 11, 18, 7, 2, 11, 0, time.UTC)
	r.Pauses = []Pause{{From: date(t, "2026-11-20"), Until: nil}}
	return r
}

func TestApplyRoutinePatch(t *testing.T) {
	cases := []struct {
		name    string
		body    string
		changed []string
		check   func(t *testing.T, old, n Routine)
	}{
		{"name", `{"name": " Blutdruck "}`, []string{"name"}, func(t *testing.T, _, n Routine) {
			if n.Name != "Blutdruck" {
				t.Errorf("name %q", n.Name)
			}
		}},
		{"name only in case", `{"name": "blutdruck messen"}`, []string{"name"}, nil},
		{"amount null removes the unit", `{"amount": null}`, []string{"amount", "unit"}, func(t *testing.T, _, n Routine) {
			if n.Amount != nil || n.Unit != nil {
				t.Errorf("amount %v, unit %v", n.Amount, n.Unit)
			}
		}},
		{"amount and unit null", `{"amount": null, "unit": null}`, []string{"amount", "unit"}, nil},
		{"unit null", `{"unit": null}`, []string{"unit"}, func(t *testing.T, _, n Routine) {
			if n.Unit != nil || n.Amount == nil {
				t.Errorf("amount %v, unit %v", n.Amount, n.Unit)
			}
		}},
		{"empty unit", `{"unit": ""}`, []string{"unit"}, nil},
		{"new amount keeps the unit", `{"amount": 3}`, []string{"amount"}, func(t *testing.T, _, n Routine) {
			if *n.Amount != 3 || *n.Unit != "Stück" {
				t.Errorf("amount %v, unit %v", *n.Amount, *n.Unit)
			}
		}},
		{"schedule replaced as a whole", `{"schedule": {"frequency": "hourly", "interval": 2, "windowStart": "08:00", "windowEnd": "12:00"}}`,
			[]string{"schedule"}, func(t *testing.T, _, n Routine) {
				if n.Schedule.Frequency != FrequencyHourly || n.Schedule.Times != nil {
					t.Errorf("schedule %+v", n.Schedule)
				}
			}},
		{"example of 3.20", `{"schedule": {"frequency": "daily", "interval": 1, "times": ["07:30", "20:00"]}, "endDate": "2026-12-07"}`,
			[]string{"schedule", "endDate"}, func(t *testing.T, _, n Routine) {
				if !slices.Equal(n.Schedule.Times, clocks(t, "07:30", "20:00")) || n.EndDate.String() != "2026-12-07" {
					t.Errorf("%+v", n)
				}
			}},
		{"endDate null", `{"endDate": null}`, []string{"endDate"}, func(t *testing.T, _, n Routine) {
			if n.EndDate != nil {
				t.Errorf("endDate %v", n.EndDate)
			}
		}},
		{"startDate", `{"startDate": "2026-11-01"}`, []string{"startDate"}, nil},
		{"start and end moved together", `{"startDate": "2026-12-05", "endDate": "2026-12-10"}`, []string{"startDate", "endDate"}, nil},
		{"values that already apply", `{"name": "Blutdruck messen", "amount": 2, "unit": "Stück", "startDate": "2026-11-18", "endDate": "2026-12-01",
			"schedule": {"frequency": "daily", "interval": 1, "times": ["20:00", "08:00"]}}`, []string{}, func(t *testing.T, old, n Routine) {
			if !reflect.DeepEqual(old, n) {
				t.Errorf("routine changed:\n%+v\n%+v", old, n)
			}
		}},
		{"everything else stays", `{"name": "Puls"}`, []string{"name"}, func(t *testing.T, old, n Routine) {
			n.Name = old.Name
			if !reflect.DeepEqual(old, n) {
				t.Errorf("other fields changed:\n%+v\n%+v", old, n)
			}
		}},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			old := patchBase(t)
			snapshot := patchBase(t)
			n, changed, err := ApplyRoutinePatch(old, jsonBody(t, c.body))
			if err != nil {
				t.Fatalf("err = %v", err)
			}
			if !slices.Equal(changed, c.changed) || changed == nil {
				t.Errorf("changed = %#v, want %#v", changed, c.changed)
			}
			if c.check != nil {
				c.check(t, old, n)
			}
			if !reflect.DeepEqual(old, snapshot) {
				t.Error("the routine passed in was changed")
			}
		})
	}
}

func TestApplyRoutinePatch_Errors(t *testing.T) {
	cases := []struct {
		name, body, field, message string
		value                      any
	}{
		{"empty body", `{}`, "", msgEmptyPatch, nil},
		{"unknown field", `{"color": "red"}`, "color", "Unbekanntes Feld: color.", nil},
		{"id", `{"id": "6742c0a1b2c3d4e5f6a7b8c9"}`, "id", "Das Feld id kann nicht geändert werden.", nil},
		{"module", `{"module": "health"}`, "module", "Das Feld module kann nicht geändert werden.", nil},
		{"status", `{"status": "paused"}`, "status", "Das Feld status kann nicht geändert werden.", nil},
		{"pauses", `{"pauses": []}`, "pauses", "Das Feld pauses kann nicht geändert werden.", nil},
		{"createdAt", `{"createdAt": "2026-11-18T07:02:11Z"}`, "createdAt", "Das Feld createdAt kann nicht geändert werden.", nil},
		{"progress", `{"progress": {}}`, "progress", "Das Feld progress kann nicht geändert werden.", nil},
		{"field names before values", `{"name": "", "status": "paused"}`, "status", "Das Feld status kann nicht geändert werden.", nil},

		{"name null", `{"name": null}`, "name", msgName, nil},
		{"name empty", `{"name": " "}`, "name", msgName, " "},
		{"schedule null", `{"schedule": null}`, "schedule", msgSchedule, nil},
		{"schedule invalid", `{"schedule": {"frequency": "daily", "interval": 1, "times": []}}`, "schedule.times", msgTimesEmpty, nil},
		{"startDate null", `{"startDate": null}`, "startDate", msgDateFormat, nil},
		{"amount 0", `{"amount": 0}`, "amount", msgAmount, float64(0)},
		{"unit too long", `{"unit": "` + strings.Repeat("u", 21) + `"}`, "unit", msgUnitLength, strings.Repeat("u", 21)},

		// Rules over the merged routine.
		{"amount null with a unit", `{"amount": null, "unit": "ml"}`, "unit", msgUnitNeedsAmount, "ml"},
		{"startDate after the old endDate", `{"startDate": "2026-12-02"}`, "startDate", msgStartAfterEnd, "2026-12-02"},
		{"endDate before the old startDate", `{"endDate": "2026-11-17"}`, "endDate", msgEndBeforeStart, "2026-11-17"},
		{"both sent, end before start", `{"startDate": "2026-12-10", "endDate": "2026-12-05"}`, "endDate", msgEndBeforeStart, "2026-12-05"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			_, _, err := ApplyRoutinePatch(patchBase(t), jsonBody(t, c.body))
			checkRoutineError(t, err, c.field, c.message, c.value)
		})
	}

	t.Run("unit on a routine without amount", func(t *testing.T) {
		r := patchBase(t)
		r.Amount, r.Unit = nil, nil
		_, _, err := ApplyRoutinePatch(r, jsonBody(t, `{"unit": "ml"}`))
		checkRoutineError(t, err, "unit", msgUnitNeedsAmount, "ml")
	})
	t.Run("not an object", func(t *testing.T) {
		_, _, err := ApplyRoutinePatch(patchBase(t), nil)
		checkRoutineError(t, err, "", msgPatchBody, nil)
	})
}

func TestRoutineNameKey(t *testing.T) {
	if RoutineNameKey("Vitamin D") != RoutineNameKey("vitamin d") || RoutineNameKey("VITAMIN D") != "vitamin d" {
		t.Error("upper and lower case must give the same key")
	}
	// No Unicode folding beyond lower case: ß and ss stay different.
	if RoutineNameKey("Fußbad") == RoutineNameKey("Fussbad") {
		t.Error("ß and ss give the same key")
	}
	if RoutineNameKey("ÄPFEL") != "äpfel" {
		t.Errorf("umlauts: %q", RoutineNameKey("ÄPFEL"))
	}
}

func TestRoutineErrors(t *testing.T) {
	var err error = &RoutineNameTakenError{RoutineID: "abc"}
	if !errors.Is(err, ErrRoutineNameTaken) {
		t.Error("RoutineNameTakenError is not ErrRoutineNameTaken")
	}
	err = &RoutineLogExistsError{LogID: "abc"}
	if !errors.Is(err, ErrRoutineLogExists) {
		t.Error("RoutineLogExistsError is not ErrRoutineLogExists")
	}
	err = fmt.Errorf("wrapped: %w", &RoutineError{Message: "x", Field: "name", Value: "Geheim"})
	if !errors.Is(err, ErrInvalidRoutine) || strings.Contains(err.Error(), "Geheim") {
		t.Errorf("RoutineError: %v", err)
	}
}
