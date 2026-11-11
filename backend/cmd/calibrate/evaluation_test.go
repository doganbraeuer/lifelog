package main

import (
	"strings"
	"testing"
	"time"

	"lifelog/backend/internal/core"
	"lifelog/backend/internal/sampledata"
)

func TestQuestions_Structure(t *testing.T) {
	ids := map[string]bool{}
	for _, e := range sampledata.Entries() {
		ids[e.ID] = true
	}
	count := map[group]int{}
	texts := map[string]bool{}
	multiple := 0
	for _, q := range questions() {
		count[q.Group]++
		if texts[q.Text] {
			t.Errorf("question twice: %q", q.Text)
		}
		texts[q.Text] = true
		if _, err := core.CheckQuestion(q.Text); err != nil {
			t.Errorf("%q: %v", q.Text, err)
		}
		if q.Group != relevant {
			if len(q.Expected) != 0 {
				t.Errorf("%q: group %v must not have an expected entry", q.Text, q.Group)
			}
			continue
		}
		if len(q.Expected) == 0 {
			t.Errorf("%q: relevant question without an expected entry", q.Text)
		}
		if len(q.Expected) > 1 {
			multiple++
		}
		seen := map[target]bool{}
		for _, tg := range q.Expected {
			if seen[tg] {
				t.Errorf("%q: target %v twice", q.Text, tg)
			}
			seen[tg] = true
			switch {
			case tg.ID != "" && (tg.Type != "" || tg.Date != "" || tg.Issuer != ""):
				t.Errorf("%q: target %+v has ID and type/date/issuer", q.Text, tg)
			case tg.ID != "" && !ids[tg.ID]:
				t.Errorf("%q: expected entry %q is not a seed entry", q.Text, tg.ID)
			case tg.ID == "":
				if tg.Type == "" && tg.Issuer == "" {
					t.Errorf("%q: target %+v without type and without issuer", q.Text, tg)
				}
				if _, err := time.Parse(time.DateOnly, tg.Date); err != nil {
					t.Errorf("%q: target %+v: invalid date: %v", q.Text, tg, err)
				}
			}
		}
	}
	if count[relevant] < 10 || count[near] < 5 || count[unrelated] < 5 || multiple < 3 {
		t.Errorf("questions per group: %v, with several entries %d; want ≥ 10 relevant, ≥ 5 near, ≥ 5 unrelated, ≥ 3 with several",
			count, multiple)
	}
}

func date(s string) *string { return &s }

func TestResolve(t *testing.T) {
	list := []core.EntrySummary{
		{ID: "T1", Type: "doctor_letter", DocumentDate: date("2026-09-02")},
		{ID: "D1", Type: "lab_report", DocumentDate: date("2026-01-20")},
		{ID: "D2", Type: "lab_report", DocumentDate: date("2026-07-22")},
		{ID: "D3", Type: "lab_report", DocumentDate: date("2026-10-14")},
		{ID: "D4", Type: "lab_report", DocumentDate: date("2026-10-14")},
		{ID: "N1", Type: "other", DocumentDate: nil},
		{ID: "O1", Type: "doctor_letter", DocumentDate: date("2026-03-05"), Issuer: date("Orthopädie, Dr. Probe, Facharzt")},
		{ID: "O2", Type: "other", DocumentDate: date("2026-03-05"), Issuer: date("Orthopädie, Dr. Probe")},
	}
	jan := target{Type: "lab_report", Date: "2026-01-20"}
	july := target{Type: "lab_report", Date: "2026-07-22"}
	ref := target{Date: "2026-03-05", Issuer: "Orthopädie, Dr. Probe"}
	q := func(targets ...target) question {
		return question{Text: "x", Group: relevant, Expected: targets}
	}

	ids, err := resolve([]question{
		q(target{ID: "T1"}, july),
		q(jan, july), // july a second time: no double check, no error
		q(ref),       // by issuer, independent of the type; the longer issuer on the same day does not match
		{Text: "y", Group: unrelated},
	}, list)
	if err != nil {
		t.Fatal(err)
	}
	want := map[target]string{{ID: "T1"}: "T1", jan: "D1", july: "D2", ref: "O2"}
	if len(ids) != len(want) {
		t.Errorf("ids = %v", ids)
	}
	for tg, id := range want {
		if ids[tg] != id {
			t.Errorf("%v → %q, want %q", tg, ids[tg], id)
		}
	}

	for name, c := range map[string]struct {
		tg   target
		want string
	}{
		"ID missing":         {target{ID: "T9"}, "no entry for ID T9"},
		"date missing":       {target{Type: "lab_report", Date: "2026-08-12"}, "no entry for lab_report 2026-08-12"},
		"wrong type":         {target{Type: "prescription", Date: "2026-01-20"}, "no entry for prescription 2026-01-20"},
		"twice":              {target{Type: "lab_report", Date: "2026-10-14"}, "2 entries for lab_report 2026-10-14"},
		"never without date": {target{Type: "other", Date: ""}, "no entry for other "},
		"issuer missing":     {target{Date: "2026-03-05", Issuer: "Physiotherapie Beispiel"}, "no entry for 2026-03-05 (Physiotherapie Beispiel)"},
		"only date":          {target{Date: "2026-03-05"}, "2 entries for 2026-03-05"},
	} {
		_, err := resolve([]question{q(jan, c.tg)}, list)
		if err == nil || !strings.Contains(err.Error(), c.want) {
			t.Errorf("%s: error %v, want %q", name, err, c.want)
		}
	}

	// Several problems come together, each only once.
	missing := target{Type: "doctor_letter", Date: "2026-04-01"}
	_, err = resolve([]question{q(missing), q(missing, target{ID: "T9"})}, list)
	if err == nil || strings.Count(err.Error(), "doctor_letter 2026-04-01") != 1 || !strings.Contains(err.Error(), "ID T9") {
		t.Errorf("several errors: %v", err)
	}
}

func TestMeasure(t *testing.T) {
	q := question{Text: "x", Group: relevant, Expected: []target{{ID: "B"}}}
	// Deliberately unsorted; entry B appears with two chunks.
	m := measure(q, []string{"B"}, []core.Hit{
		{EntryID: "B", Score: 0.81}, {EntryID: "A", Score: 0.90}, {EntryID: "B", Score: 0.85},
	})
	if m.Best != 0.90 || m.BestEntry != "A" || m.Expected[0] != 0.85 || m.RightOnTop() || m.decisive() != 0.85 {
		t.Errorf("= %+v, RightOnTop %v", m, m.RightOnTop())
	}

	m = measure(q, []string{"B"}, []core.Hit{{EntryID: "B", Score: 0.88}})
	if !m.RightOnTop() || m.Expected[0] != 0.88 || !m.AllFound() {
		t.Errorf("right on top: %+v", m)
	}

	m = measure(q, []string{"B"}, nil)
	if m.Best != 0 || m.BestEntry != "" || m.Expected[0] != 0 || m.RightOnTop() || m.AllFound() {
		t.Errorf("without hits: %+v", m)
	}

	// Two entries: the weaker one is decisive; one of them on top is enough.
	two := question{Text: "z", Group: relevant, Expected: []target{{ID: "B"}, {ID: "C"}}}
	m = measure(two, []string{"B", "C"}, []core.Hit{
		{EntryID: "C", Score: 0.89}, {EntryID: "A", Score: 0.86}, {EntryID: "B", Score: 0.84},
	})
	if !m.RightOnTop() || !m.AllFound() || m.decisive() != 0.84 {
		t.Errorf("two: %+v", m)
	}
	m = measure(two, []string{"B", "C"}, []core.Hit{{EntryID: "C", Score: 0.89}})
	if !m.RightOnTop() || m.AllFound() || m.decisive() != 0 {
		t.Errorf("two, B missing: %+v", m)
	}

	// Unrelated questions have no expected entry, even if a hit happens to
	// have an empty ID.
	m = measure(question{Group: unrelated}, nil, []core.Hit{{EntryID: "", Score: 0.7}})
	if len(m.Expected) != 0 || m.RightOnTop() || m.decisive() != 0.7 {
		t.Errorf("unrelated: %+v", m)
	}
}

func meas(g group, best float64, expected []float64, rightOnTop bool) measurement {
	m := measurement{Question: question{Group: g}, Best: best, BestEntry: "X"}
	if g == relevant {
		m.Expected = expected
		for i := range expected {
			id := string(rune('Y' + i))
			m.IDs = append(m.IDs, id)
			m.Question.Expected = append(m.Question.Expected, target{ID: id})
		}
		if rightOnTop {
			m.BestEntry = "Y"
		}
	}
	return m
}

func TestEvaluate(t *testing.T) {
	measurements := []measurement{
		meas(relevant, 0.90, []float64{0.90}, true),
		meas(relevant, 0.88, []float64{0.80}, false),      // wrong entry on top; the right one counts: exactly on the threshold
		meas(relevant, 0.86, []float64{0}, false),         // right entry missing → decisive 0
		meas(relevant, 0.89, []float64{0.89, 0.83}, true), // two entries: decisive 0.83
		meas(near, 0.83, nil, false),
		meas(unrelated, 0.77, nil, false),
		meas(unrelated, 0.81, nil, false),
	}
	ev := evaluate(measurements, 0.80)

	if ev.Relevant != (span{Count: 4, Passed: 3, Min: 0, Max: 0.90}) {
		t.Errorf("relevant = %+v", ev.Relevant)
	}
	if ev.Multiple != (span{Count: 1, Passed: 1, Min: 0.83, Max: 0.83}) {
		t.Errorf("multiple = %+v", ev.Multiple)
	}
	if ev.Near != (span{Count: 1, Passed: 1, Min: 0.83, Max: 0.83}) {
		t.Errorf("near = %+v", ev.Near)
	}
	if ev.Unrelated != (span{Count: 2, Passed: 1, Min: 0.77, Max: 0.81}) {
		t.Errorf("unrelated = %+v", ev.Unrelated)
	}
	if ev.WrongOnTop != 1 || ev.NotFound != 1 {
		t.Errorf("WrongOnTop %d, NotFound %d", ev.WrongOnTop, ev.NotFound)
	}
	if ev.Separable() {
		t.Error("separable although relevant starts at 0")
	}
}

func TestSeparableAndSummary(t *testing.T) {
	ev := evaluate([]measurement{
		meas(relevant, 0.90, []float64{0.90}, true),
		meas(relevant, 0.87, []float64{0.87}, true),
		meas(unrelated, 0.78, nil, false),
		meas(unrelated, 0.74, nil, false),
	}, 0.80)
	if !ev.Separable() {
		t.Fatalf("not separable: %+v", ev)
	}
	text := ev.Summary()
	for _, want := range []string{
		"Threshold 0.800 (cos 0.600)",
		"2 questions, score 0.870 … 0.900,  2/2 sources get through",
		"2 questions, score 0.740 … 0.780,  0/2 go to the language model (should: none)",
		"every threshold above 0.780 up to and including 0.870",
	} {
		if !strings.Contains(text, want) {
			t.Errorf("missing %q in\n%s", want, text)
		}
	}
	if strings.Contains(text, "near") || strings.Contains(text, "multiple") {
		t.Errorf("empty group appears:\n%s", text)
	}

	// A tie is not separable: a threshold ≥ 0.85 lets both through or none.
	tie := evaluate([]measurement{meas(relevant, 0.85, []float64{0.85}, true), meas(unrelated, 0.85, nil, false)}, 0.80)
	if tie.Separable() || !strings.Contains(tie.Summary(), "Not separable") {
		t.Errorf("tie: %s", tie.Summary())
	}
}

func TestScore(t *testing.T) {
	for f, want := range map[float64]string{0.8: "0.800", 0.8916: "0.892", 0.8914: "0.891", 0: "0.000", 1: "1.000"} {
		if got := score(f); got != want {
			t.Errorf("score(%v) = %q, want %q", f, got, want)
		}
	}
}
