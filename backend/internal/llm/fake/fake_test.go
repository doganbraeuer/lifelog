package fake

import (
	"context"
	"errors"
	"slices"
	"testing"

	"lifelog/backend/internal/llm"
)

func TestEinbetter(t *testing.T) {
	e := &Einbetter{}
	v, err := e.AbschnitteEinbetten(context.Background(), []string{"a", "b", "a"})
	if err != nil {
		t.Fatal(err)
	}
	if len(v) != 3 || len(v[0]) != llm.EmbeddingDimensionen {
		t.Fatalf("Form = %d × %d", len(v), len(v[0]))
	}
	if !slices.Equal(v[0], v[2]) || slices.Equal(v[0], v[1]) {
		t.Error("gleicher Text muss gleichen Vektor, anderer Text anderen Vektor ergeben")
	}
	f, _ := e.FrageEinbetten(context.Background(), "a")
	if !slices.Equal(f, v[0]) {
		t.Error("Frage und Abschnitt mit gleichem Text sollen in der Attrappe gleich sein")
	}
	if e.Aufrufe() != 2 {
		t.Errorf("Aufrufe = %d", e.Aufrufe())
	}

	kaputt := &Einbetter{Err: llm.ErrNichtErreichbar}
	if _, err := kaputt.AbschnitteEinbetten(context.Background(), []string{"x"}); !errors.Is(err, llm.ErrNichtErreichbar) {
		t.Errorf("err = %v", err)
	}
}

func TestLeser(t *testing.T) {
	l := &Leser{Err: llm.ErrAntwortUngueltig}
	if _, err := l.DokumentLesen(context.Background(), nil, "", nil); !errors.Is(err, llm.ErrAntwortUngueltig) {
		t.Errorf("err = %v", err)
	}
	if l.Aufrufe() != 1 {
		t.Errorf("Aufrufe = %d", l.Aufrufe())
	}
}
