package core

import "testing"

func TestNormalizeWord(t *testing.T) {
	tests := map[string]string{
		"Beispiel,":        "beispiel",
		"(Ref.":            "ref",
		"14,2":             "14.2",
		"14.2":             "14.2",
		"<0,5":             "<0.5",
		"g/dl":             "g/dl",
		"20.01.2026.":      "20.01.2026",
		"Hämoglobin":      "hämoglobin", // decomposed umlaut (NFKC composes it)
		"ﬁbrinogen":        "fibrinogen", // ligature "ﬁ"
		"–":                "-",
		"13,5–17,5":        "13.5-17.5",
		"„Test“":           "test",
		".":                "",
		"a,b":              "a,b", // comma not between digits
		"1-0-1":            "1-0-1",
		"Jän.":             "jän",
		"LDL-Cholesterin:": "ldl-cholesterin",
		"100%":             "100%",
	}
	for in, want := range tests {
		if got := normalizeWord(in); got != want {
			t.Errorf("normalizeWord(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestNormalizeKey_IgnoresWhitespace(t *testing.T) {
	a, b := normalizeKey("14,2 g/dl"), normalizeKey(" 14,2g/dl ")
	if a != b || a != "14.2g/dl" {
		t.Errorf("keys %q and %q, want both %q", a, b, "14.2g/dl")
	}
	if got := normalizeKey("Dr. Erika  Beispiel,"); got != "drerikabeispiel" {
		t.Errorf("got %q", got)
	}
}

func TestWordsFromText(t *testing.T) {
	words := WordsFromText("Wert: 14,2 g/dl\r\nÜbelkeit  ja")
	want := []OCRWord{
		{Text: "Wert:", Source: SourceText, Line: 0, Start: 0, End: 5},
		{Text: "14,2", Source: SourceText, Line: 0, Start: 6, End: 10},
		{Text: "g/dl", Source: SourceText, Line: 0, Start: 11, End: 15},
		{Text: "Übelkeit", Source: SourceText, Line: 1, Start: 17, End: 25}, // "Ü" is one code point
		{Text: "ja", Source: SourceText, Line: 1, Start: 27, End: 29},
	}
	if len(words) != len(want) {
		t.Fatalf("got %d words: %+v", len(words), words)
	}
	for i := range want {
		if words[i] != want[i] {
			t.Errorf("word %d = %+v, want %+v", i, words[i], want[i])
		}
	}
	if got := WordsFromText(" \n "); got == nil || len(got) != 0 {
		t.Errorf("whitespace only: got %#v, want empty list", got)
	}
}
