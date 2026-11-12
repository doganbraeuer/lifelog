package main

import (
	"errors"
	"fmt"
	"math"
	"strings"

	"lifelog/backend/internal/core"
)

// candidate is what resolve needs to know about an entry.
type candidate struct {
	ID       string
	FileName string // file name of the original, "text.txt" for text imports
	Date     string // document date, "" if the entry has none
}

// resolve turns the targets of all questions into entry IDs. An ID target
// must be in the list, a target by file name (plus date) must match exactly
// one entry. Otherwise there is an error (all problems at once), so a
// missing or duplicate upload does not silently give wrong numbers. Without
// I/O, so it is testable.
func resolve(questions []question, list []candidate) (map[target]string, error) {
	ids := map[target]string{}
	var errs []error
	for _, q := range questions {
		for _, t := range q.Expected {
			if _, done := ids[t]; done {
				continue
			}
			var matching []string
			for _, c := range list {
				if matches(t, c) {
					matching = append(matching, c.ID)
				}
			}
			switch len(matching) {
			case 1:
				ids[t] = matching[0]
			case 0:
				errs = append(errs, fmt.Errorf("no entry for %v (question %q)", t, q.Text))
				ids[t] = "" // report only once
			default:
				errs = append(errs, fmt.Errorf("%d entries for %v, want exactly one (question %q)", len(matching), t, q.Text))
				ids[t] = ""
			}
		}
	}
	if len(errs) > 0 {
		return nil, errors.Join(errs...)
	}
	return ids, nil
}

// matches: the same ID, or the same file name and, if given, the same
// document date. A target without ID and file name matches nothing.
func matches(t target, c candidate) bool {
	if t.ID != "" {
		return c.ID == t.ID
	}
	if t.FileName == "" || c.FileName != t.FileName {
		return false
	}
	return t.Date == "" || c.Date == t.Date
}

// measurement is the result of the vector search for one question. Without
// I/O, so the evaluation is testable.
type measurement struct {
	Question  question
	IDs       []string  // resolved targets, same order as Question.Expected
	Best      float64   // highest score of all hits, 0 without hits
	BestEntry string    // entry of the best hit, "" without hits
	Expected  []float64 // best score per expected entry, 0 if not among the hits
}

// measure summarizes the hits of a question. Atlas returns the hits sorted
// descending, but we do not rely on that.
func measure(q question, ids []string, hits []core.Hit) measurement {
	m := measurement{Question: q, IDs: ids, Expected: make([]float64, len(ids))}
	for _, h := range hits {
		if h.Score > m.Best {
			m.Best, m.BestEntry = h.Score, h.EntryID
		}
		for i, id := range ids {
			if h.EntryID == id && h.Score > m.Expected[i] {
				m.Expected[i] = h.Score
			}
		}
	}
	return m
}

// RightOnTop: the best hit belongs to one of the expected entries.
func (m measurement) RightOnTop() bool {
	if m.Question.Group != relevant || m.BestEntry == "" {
		return false
	}
	for _, id := range m.IDs {
		if id == m.BestEntry {
			return true
		}
	}
	return false
}

// AllFound: every expected entry is among the hits.
func (m measurement) AllFound() bool {
	for _, s := range m.Expected {
		if s == 0 {
			return false
		}
	}
	return len(m.Expected) > 0
}

// decisive is the score that decides about the threshold: for relevant
// questions the weakest expected entry (do all needed sources reach the
// language model?), otherwise the best of all (is the language model
// asked?).
func (m measurement) decisive() float64 {
	if m.Question.Group != relevant {
		return m.Best
	}
	if len(m.Expected) == 0 {
		return 0
	}
	min := m.Expected[0]
	for _, s := range m.Expected[1:] {
		min = math.Min(min, s)
	}
	return min
}

// span: smallest and largest decisive score of a group and how many
// questions reach the threshold (≥, like core.AboveThreshold).
type span struct {
	Count, Passed int
	Min, Max      float64
}

func (s *span) add(value, threshold float64) {
	if s.Count == 0 {
		s.Min, s.Max = value, value
	}
	s.Min, s.Max = math.Min(s.Min, value), math.Max(s.Max, value)
	s.Count++
	if value >= threshold {
		s.Passed++
	}
}

type evaluation struct {
	Threshold                 float64
	Relevant, Near, Unrelated span
	Multiple                  span // subset of Relevant: the answer needs several entries
	WrongOnTop, NotFound      int  // only relevant
}

func evaluate(measurements []measurement, threshold float64) evaluation {
	ev := evaluation{Threshold: threshold}
	for _, m := range measurements {
		value := m.decisive()
		switch m.Question.Group {
		case relevant:
			ev.Relevant.add(value, threshold)
			if len(m.Expected) > 1 {
				ev.Multiple.add(value, threshold)
			}
			if !m.AllFound() {
				ev.NotFound++
			} else if !m.RightOnTop() {
				ev.WrongOnTop++
			}
		case near:
			ev.Near.add(value, threshold)
		default:
			ev.Unrelated.add(value, threshold)
		}
	}
	return ev
}

// Separable: is there a threshold that lets all relevant questions through
// and catches all unrelated ones? Then it lies in (Unrelated.Max,
// Relevant.Min].
func (ev evaluation) Separable() bool {
	return ev.Relevant.Count > 0 && ev.Unrelated.Count > 0 && ev.Relevant.Min > ev.Unrelated.Max
}

// score writes a score with three decimals.
func score(f float64) string {
	return fmt.Sprintf("%.3f", f)
}

// Summary as text for the console.
func (ev evaluation) Summary() string {
	var b strings.Builder
	line := func(name string, s span, what string) {
		if s.Count == 0 {
			return
		}
		fmt.Fprintf(&b, "  %-10s %2d questions, score %s … %s, %2d/%d %s\n",
			name, s.Count, score(s.Min), score(s.Max), s.Passed, s.Count, what)
	}
	fmt.Fprintf(&b, "Threshold %s (cos %s):\n", score(ev.Threshold), score(core.CosineFromScore(ev.Threshold)))
	line("relevant", ev.Relevant, "sources get through (should: all)")
	line("  multiple", ev.Multiple, "of them with several entries, all get through")
	line("near", ev.Near, "go to the language model (allowed, it answers with the fixed sentence)")
	line("unrelated", ev.Unrelated, "go to the language model (should: none)")
	if ev.WrongOnTop > 0 {
		fmt.Fprintf(&b, "  %d relevant questions: best hit from another entry\n", ev.WrongOnTop)
	}
	if ev.NotFound > 0 {
		fmt.Fprintf(&b, "  %d relevant questions: at least one expected entry not among the hits\n", ev.NotFound)
	}
	if ev.Separable() {
		fmt.Fprintf(&b, "Separable: every threshold above %s up to and including %s separates relevant from unrelated.\n",
			score(ev.Unrelated.Max), score(ev.Relevant.Min))
	} else if ev.Relevant.Count > 0 && ev.Unrelated.Count > 0 {
		fmt.Fprintf(&b, "Not separable: unrelated reaches %s, relevant starts at %s.\n",
			score(ev.Unrelated.Max), score(ev.Relevant.Min))
	}
	return b.String()
}
