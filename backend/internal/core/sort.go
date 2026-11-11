package core

import (
	"errors"
	"slices"
	"strings"
)

// Sort is the field the timeline is sorted by.
type Sort string

// The values are the query parameter values of GET /eintraege.
const (
	ByDocumentDate Sort = "dokumentdatum"
	ByImportedAt   Sort = "importdatum"
)

// SortOrder is the direction of sorting.
type SortOrder string

const (
	Descending SortOrder = "absteigend"
	Ascending  SortOrder = "aufsteigend"
)

// ErrUnknownValue: a parameter has a value that is not allowed.
var ErrUnknownValue = errors.New("unknown value")

// ParseSort reads the query parameter "sortierung". Empty gives the default
// ByDocumentDate.
func ParseSort(value string) (Sort, error) {
	switch Sort(value) {
	case "", ByDocumentDate:
		return ByDocumentDate, nil
	case ByImportedAt:
		return ByImportedAt, nil
	}
	return "", ErrUnknownValue
}

// ParseSortOrder reads the query parameter "richtung". Empty gives the
// default Descending (newest document first).
func ParseSortOrder(value string) (SortOrder, error) {
	switch SortOrder(value) {
	case "", Descending:
		return Descending, nil
	case Ascending:
		return Ascending, nil
	}
	return "", ErrUnknownValue
}

// SortEntries orders the entries by the rules in docs/API.md, 3.2:
//
//  1. By the chosen field in the chosen order.
//  2. When sorting by document date, entries without a date are always at
//     the end, regardless of the order. Among themselves by import time
//     descending.
//  3. Tie: import time in the same order, then id.
//
// The order is therefore always unique, even for equal dates.
func SortEntries(entries []EntrySummary, s Sort, o SortOrder) {
	slices.SortFunc(entries, func(a, b EntrySummary) int {
		return compare(a, b, s, o)
	})
}

// compare returns <0 if a should come before b, >0 if after.
func compare(a, b EntrySummary, s Sort, o SortOrder) int {
	if s == ByDocumentDate {
		switch {
		case a.DocumentDate == nil && b.DocumentDate == nil:
			// Rule 2: among themselves always descending, regardless of o.
			return -compareImportAndID(a, b)
		case a.DocumentDate == nil:
			return 1 // a without date → to the end
		case b.DocumentDate == nil:
			return -1
		}
		// "YYYY-MM-DD" can be compared as text: the order of the characters
		// matches the chronological order.
		if c := strings.Compare(*a.DocumentDate, *b.DocumentDate); c != 0 {
			return inOrder(c, o)
		}
	}
	return inOrder(compareImportAndID(a, b), o)
}

// compareImportAndID compares ascending by import time, then id.
func compareImportAndID(a, b EntrySummary) int {
	if c := a.ImportedAt.Compare(b.ImportedAt); c != 0 {
		return c
	}
	return strings.Compare(a.ID, b.ID)
}

func inOrder(c int, o SortOrder) int {
	if o == Descending {
		return -c
	}
	return c
}
