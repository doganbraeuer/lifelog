package core

import (
	"slices"
	"strings"
	"testing"
)

// cell is a word with its own program line and a box.
func cell(text string, line int, x, y, h float64) OCRWord {
	return OCRWord{Text: text, Source: SourcePDFText, Page: 1, Line: line, Box: Box{X: x, Y: y, W: 0.05, H: h}}
}

func texts(words []OCRWord) string {
	var out []string
	for _, w := range words {
		out = append(out, w.Text)
	}
	return strings.Join(out, " ")
}

// lineTexts returns the words per Line, in order.
func lineTexts(words []OCRWord) []string {
	var out []string
	for i, w := range words {
		if i == 0 || w.Line != words[i-1].Line {
			out = append(out, w.Text)
		} else {
			out[len(out)-1] += " " + w.Text
		}
	}
	return out
}

// TestArrangeRows_TableReadByColumn: pdftotext gives each column as a
// block, i.e. the table below comes as 6 lines, column by column.
func TestArrangeRows_TableReadByColumn(t *testing.T) {
	words := []OCRWord{
		cell("Hämoglobin", 0, 0.03, 0.188, 0.019),
		cell("Leukozyten", 1, 0.03, 0.203, 0.019),
		cell("14,2", 2, 0.27, 0.188, 0.019),
		cell("6,8", 3, 0.27, 0.203, 0.019),
		cell("g/dl", 4, 0.40, 0.188, 0.019),
		cell("G/l", 5, 0.40, 0.203, 0.019),
	}
	got := arrangeRows(words)
	want := []string{"Hämoglobin 14,2 g/dl", "Leukozyten 6,8 G/l"}
	if lines := lineTexts(got); !slices.Equal(lines, want) {
		t.Errorf("rows %q, want %q", lines, want)
	}
	if words[2].Line != 2 {
		t.Error("input was changed")
	}
}

func TestArrangeRows_TiltedRowChains(t *testing.T) {
	// A slightly tilted row: each cell 0.006 lower than the one before.
	// First and last cell no longer overlap enough, but each neighbour does.
	words := []OCRWord{
		cell("a", 0, 0.1, 0.300, 0.012),
		cell("b", 1, 0.3, 0.306, 0.012),
		cell("c", 2, 0.5, 0.312, 0.012),
		cell("next", 3, 0.1, 0.330, 0.012),
	}
	if sameRow(words[0].Box, words[2].Box) {
		t.Fatal("test setup: a and c should not overlap enough on their own")
	}
	if lines := lineTexts(arrangeRows(words)); !slices.Equal(lines, []string{"a b c", "next"}) {
		t.Errorf("rows %q", lines)
	}
}

func TestArrangeRows_KeepsSeparateRows(t *testing.T) {
	words := []OCRWord{
		cell("oben", 0, 0.1, 0.10, 0.02),
		cell("unten", 1, 0.1, 0.115, 0.02), // overlaps by 0.005 = 25 %
		// A tall box over both rows (e.g. a stamp read as one line) joins
		// neither.
		cell("Stempel", 2, 0.6, 0.095, 0.05),
	}
	got := lineTexts(arrangeRows(words))
	if len(got) != 3 {
		t.Errorf("rows %q, want 3 separate rows", got)
	}
}

func TestArrangeRows_PagesAndOrder(t *testing.T) {
	p2 := cell("zwei", 5, 0.1, 0.01, 0.02)
	p2.Page = 2
	words := []OCRWord{
		p2,
		cell("rechts", 1, 0.6, 0.5, 0.02),
		cell("links", 0, 0.1, 0.5, 0.02),
		cell("oben", 2, 0.1, 0.1, 0.02),
	}
	got := arrangeRows(words)
	if s := texts(got); s != "oben links rechts zwei" {
		t.Errorf("order %q", s)
	}
	if got[3].Page != 2 || got[3].Line == got[2].Line {
		t.Errorf("page 2 must be its own row: %+v", got[3])
	}
}

func TestArrangeRows_TextUnchanged(t *testing.T) {
	words := WordsFromText("b\na")
	if got := arrangeRows(words); texts(got) != "b a" || got[0].Line != 0 || got[1].Line != 1 {
		t.Errorf("text import changed: %+v", got)
	}
}

// TestVerify_TableReadByColumn: the whole effect on the check.
func TestVerify_TableReadByColumn(t *testing.T) {
	words := []OCRWord{
		cell("Hämoglobin", 0, 0.03, 0.188, 0.019),
		cell("14,2", 1, 0.27, 0.188, 0.019),
		cell("g/dl", 2, 0.40, 0.188, 0.019),
	}
	e := Entry{Facts: []Fact{{ID: "f1", Name: "Hämoglobin", Value: "14,2", Unit: ptr("g/dl")}}}
	v := Verify(e, OCRResult{Words: words}, checkTime)
	fc := field(t, v, "facts.f1")
	if fc.Status != FieldConfirmed || len(fc.Locations) != 1 {
		t.Fatalf("got %+v, want confirmed with one location", fc)
	}
	// One box from the value to the end of the unit.
	if b := fc.Locations[0].Box; b.X != 0.27 || b.X+b.W < 0.449 {
		t.Errorf("box %+v", b)
	}
}
