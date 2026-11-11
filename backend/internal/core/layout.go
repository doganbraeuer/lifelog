package core

import (
	"cmp"
	"slices"
)

// Rows: when two lines are one visual row.
const (
	// rowMinOverlap: the lines overlap vertically by at least this share of
	// the lower one's height.
	rowMinOverlap = 0.5
	// rowMinHeightRatio: the lower line is at least this share of the
	// higher one's height. Keeps a tall box (e.g. two rows read as one)
	// from joining two real rows.
	rowMinHeightRatio = 0.5
)

// arrangeRows puts the words of a file into visual rows: reading order is
// row by row from top to bottom, within a row from left to right, and Line
// is the row number.
//
// Why: pdftotext and Tesseract often read a table column by column. Then
// "Hämoglobin", "14,2" and "g/dl" are three lines of their own, and the rule
// "the unit follows the value in the same line" never holds. What a reader
// sees as one line is decided here from the positions instead.
//
// The lines of the program are kept as units (they follow the baseline,
// also in a slightly tilted photo). Two lines on the same page join one row
// if they overlap vertically (rowMinOverlap) and have a similar height
// (rowMinHeightRatio). Joining is transitive, so the cells of a slightly
// tilted table row join through their neighbours.
//
// Words of a text import (Page 0) are returned unchanged. The input is not
// changed.
func arrangeRows(words []OCRWord) []OCRWord {
	type lineKey struct{ page, line int }
	type line struct {
		key   lineKey
		box   Box
		words []OCRWord
	}

	var lines []*line
	byKey := map[lineKey]*line{}
	for _, w := range words {
		if w.Page == 0 {
			return words
		}
		k := lineKey{w.Page, w.Line}
		l, ok := byKey[k]
		if !ok {
			l = &line{key: k, box: w.Box}
			byKey[k] = l
			lines = append(lines, l)
		}
		l.box = l.box.union(w.Box)
		l.words = append(l.words, w)
	}

	// Union-find over the lines.
	parent := make([]int, len(lines))
	for i := range parent {
		parent[i] = i
	}
	var find func(int) int
	find = func(i int) int {
		if parent[i] != i {
			parent[i] = find(parent[i])
		}
		return parent[i]
	}
	for i := range lines {
		for j := i + 1; j < len(lines); j++ {
			if lines[i].key.page == lines[j].key.page && sameRow(lines[i].box, lines[j].box) {
				parent[find(j)] = find(i)
			}
		}
	}

	type row struct {
		page    int
		centerY float64 // mean of the line centers
		words   []OCRWord
	}
	rowOf := map[int]*row{}
	var rows []*row
	counts := map[int]int{}
	for i, l := range lines {
		root := find(i)
		r, ok := rowOf[root]
		if !ok {
			r = &row{page: l.key.page}
			rowOf[root] = r
			rows = append(rows, r)
		}
		r.centerY += l.box.Y + l.box.H/2
		counts[root]++
		r.words = append(r.words, l.words...)
	}
	for root, r := range rowOf {
		r.centerY /= float64(counts[root])
	}

	slices.SortStableFunc(rows, func(a, b *row) int {
		if c := cmp.Compare(a.page, b.page); c != 0 {
			return c
		}
		return cmp.Compare(a.centerY, b.centerY)
	})
	out := make([]OCRWord, 0, len(words))
	for n, r := range rows {
		slices.SortStableFunc(r.words, func(a, b OCRWord) int { return cmp.Compare(a.Box.X, b.Box.X) })
		for _, w := range r.words {
			w.Line = n
			out = append(out, w)
		}
	}
	return out
}

// sameRow: do two line boxes lie in one visual row?
func sameRow(a, b Box) bool {
	low, high := min(a.H, b.H), max(a.H, b.H)
	if low <= 0 || low < rowMinHeightRatio*high {
		return false
	}
	overlap := min(a.Y+a.H, b.Y+b.H) - max(a.Y, b.Y)
	return overlap >= rowMinOverlap*low
}
