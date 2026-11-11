package core

import (
	"errors"
	"slices"
	"testing"
	"time"
)

// e builds an entry summary. date "" means: no document date (null).
// importMinute is the minute of the import on 2026-11-03, which keeps the
// cases short.
func e(id, date string, importMinute int) EntrySummary {
	s := EntrySummary{
		ID:         id,
		ImportedAt: time.Date(2026, 11, 3, 18, importMinute, 0, 0, time.UTC),
	}
	if date != "" {
		s.DocumentDate = &date
	}
	return s
}

func ids(entries []EntrySummary) []string {
	out := make([]string, len(entries))
	for i, x := range entries {
		out[i] = x.ID
	}
	return out
}

func TestSortEntries(t *testing.T) {
	// Mixed set for several cases:
	// a: 14 Oct, import :10 | b: 2 Sep, import :20 | c: no date, import :05
	// d: no date, import :30 | e: 14 Oct, import :40 (same date as a)
	set := func() []EntrySummary {
		return []EntrySummary{
			e("a", "2026-10-14", 10),
			e("c", "", 5),
			e("b", "2026-09-02", 20),
			e("d", "", 30),
			e("e", "2026-10-14", 40),
		}
	}

	cases := []struct {
		name  string
		input []EntrySummary
		sort  Sort
		order SortOrder
		want  []string
	}{
		{
			name:  "rules 1+2+3: document date descending, null at the end, tie by import descending",
			input: set(), sort: ByDocumentDate, order: Descending,
			want: []string{"e", "a", "b", "d", "c"},
		},
		{
			name:  "rule 2: null at the end also ascending, and descending among themselves",
			input: set(), sort: ByDocumentDate, order: Ascending,
			want: []string{"b", "a", "e", "d", "c"},
		},
		{
			name:  "import descending, document date does not matter",
			input: set(), sort: ByImportedAt, order: Descending,
			want: []string{"e", "d", "b", "a", "c"},
		},
		{
			name:  "import ascending",
			input: set(), sort: ByImportedAt, order: Ascending,
			want: []string{"c", "a", "b", "d", "e"},
		},
		{
			name: "rule 3: same date and same import, then id descending",
			input: []EntrySummary{
				e("x1", "2026-10-14", 10), e("x3", "2026-10-14", 10), e("x2", "2026-10-14", 10),
			},
			sort: ByDocumentDate, order: Descending,
			want: []string{"x3", "x2", "x1"},
		},
		{
			name: "rule 3: same date and same import, then id ascending",
			input: []EntrySummary{
				e("x1", "2026-10-14", 10), e("x3", "2026-10-14", 10), e("x2", "2026-10-14", 10),
			},
			sort: ByDocumentDate, order: Ascending,
			want: []string{"x1", "x2", "x3"},
		},
		{
			name: "two entries without date with the same import: id descending",
			input: []EntrySummary{
				e("n1", "", 10), e("n2", "", 10),
			},
			sort: ByDocumentDate, order: Ascending,
			want: []string{"n2", "n1"},
		},
		{
			name: "same import when sorting by import: id in the same order",
			input: []EntrySummary{
				e("m1", "2026-01-01", 10), e("m2", "", 10),
			},
			sort: ByImportedAt, order: Descending,
			want: []string{"m2", "m1"},
		},
		{
			name: "turn of the year: text comparison of dates is chronological",
			input: []EntrySummary{
				e("old", "2025-12-31", 10), e("new", "2026-01-01", 10),
			},
			sort: ByDocumentDate, order: Descending,
			want: []string{"new", "old"},
		},
		{
			name: "empty list", input: []EntrySummary{},
			sort: ByDocumentDate, order: Descending, want: []string{},
		},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			SortEntries(c.input, c.sort, c.order)
			if got := ids(c.input); !slices.Equal(got, c.want) {
				t.Errorf("order = %v, want %v", got, c.want)
			}
		})
	}
}

func TestSortEntries_IndependentOfInputOrder(t *testing.T) {
	// A unique order means: every permutation gives the same result.
	base := []EntrySummary{
		e("a", "2026-10-14", 10), e("b", "2026-10-14", 10), e("c", "", 10),
		e("d", "", 10), e("e", "2026-09-02", 10),
	}
	var reference []string
	for i := range 20 {
		cp := slices.Clone(base)
		// simple, deterministic shuffle
		for j := range cp {
			k := (j*7 + i*3) % len(cp)
			cp[j], cp[k] = cp[k], cp[j]
		}
		SortEntries(cp, ByDocumentDate, Descending)
		if reference == nil {
			reference = ids(cp)
			continue
		}
		if got := ids(cp); !slices.Equal(got, reference) {
			t.Fatalf("run %d: %v, want %v", i, got, reference)
		}
	}
}

func TestParseSort(t *testing.T) {
	cases := map[string]Sort{"": ByDocumentDate, "dokumentdatum": ByDocumentDate, "importdatum": ByImportedAt}
	for value, want := range cases {
		got, err := ParseSort(value)
		if err != nil || got != want {
			t.Errorf("ParseSort(%q) = %q, %v", value, got, err)
		}
	}
	for _, value := range []string{"quelle", "Dokumentdatum", " importdatum"} {
		if _, err := ParseSort(value); !errors.Is(err, ErrUnknownValue) {
			t.Errorf("ParseSort(%q): want ErrUnknownValue, got %v", value, err)
		}
	}
}

func TestParseSortOrder(t *testing.T) {
	cases := map[string]SortOrder{"": Descending, "absteigend": Descending, "aufsteigend": Ascending}
	for value, want := range cases {
		got, err := ParseSortOrder(value)
		if err != nil || got != want {
			t.Errorf("ParseSortOrder(%q) = %q, %v", value, got, err)
		}
	}
	for _, value := range []string{"desc", "asc", "Absteigend"} {
		if _, err := ParseSortOrder(value); !errors.Is(err, ErrUnknownValue) {
			t.Errorf("ParseSortOrder(%q): want ErrUnknownValue, got %v", value, err)
		}
	}
}
