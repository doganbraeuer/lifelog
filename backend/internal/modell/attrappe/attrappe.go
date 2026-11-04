// Package attrappe ersetzt die Modelle in Tests: kein Netz, keine Kosten,
// vorhersagbare Antworten. Die Attrappen zählen ihre Aufrufe, damit Tests
// prüfen können, dass z. B. bei einem Duplikat kein Modell gefragt wurde.
package attrappe

import (
	"context"
	"crypto/sha256"
	"encoding/binary"
	"sync"

	"lifelog/backend/internal/kern"
	"lifelog/backend/internal/modell"
)

// Leser liefert immer Erkennung bzw. Err.
type Leser struct {
	Erkennung kern.Erkennung
	Err       error

	mu      sync.Mutex
	aufrufe int
}

var _ modell.Leser = (*Leser)(nil)

func (l *Leser) DokumentLesen(_ context.Context, _ []byte, _ string, _ []string) (kern.Erkennung, error) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.aufrufe++
	return l.Erkennung, l.Err
}

// Aufrufe: wie oft DokumentLesen aufgerufen wurde.
func (l *Leser) Aufrufe() int {
	l.mu.Lock()
	defer l.mu.Unlock()
	return l.aufrufe
}

// Einbetter liefert für jeden Text einen festen Vektor, der nur vom Text
// abhängt: gleicher Text, gleicher Vektor. Mit Err scheitert jeder Aufruf.
type Einbetter struct {
	Err error

	mu      sync.Mutex
	aufrufe int
}

var _ modell.Einbetter = (*Einbetter)(nil)

func (e *Einbetter) AbschnitteEinbetten(_ context.Context, texte []string) ([][]float32, error) {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.aufrufe++
	if e.Err != nil {
		return nil, e.Err
	}
	vektoren := make([][]float32, len(texte))
	for i, t := range texte {
		vektoren[i] = Vektor(t)
	}
	return vektoren, nil
}

func (e *Einbetter) FrageEinbetten(_ context.Context, frage string) ([]float32, error) {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.aufrufe++
	if e.Err != nil {
		return nil, e.Err
	}
	return Vektor(frage), nil
}

// Aufrufe: wie oft eine der beiden Methoden aufgerufen wurde.
func (e *Einbetter) Aufrufe() int {
	e.mu.Lock()
	defer e.mu.Unlock()
	return e.aufrufe
}

// Vektor erzeugt aus einem Text einen festen Pseudo-Vektor mit
// modell.EmbeddingDimensionen Einträgen. Er hat keine Bedeutung, ist aber
// deterministisch, das reicht für Tests.
func Vektor(text string) []float32 {
	v := make([]float32, modell.EmbeddingDimensionen)
	block := sha256.Sum256([]byte(text))
	for i := range v {
		if i%8 == 0 && i > 0 {
			block = sha256.Sum256(block[:])
		}
		v[i] = float32(binary.BigEndian.Uint32(block[(i%8)*4:])) / float32(1<<32)
	}
	return v
}
