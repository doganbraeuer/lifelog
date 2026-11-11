package speicher

import (
	"encoding/json"
	"testing"

	"lifelog/backend/internal/llm"
)

// Die Definition im Repo muss zur Länge der Embeddings passen, sonst lehnt
// Atlas jeden gespeicherten Abschnitt ab oder findet nichts.
func TestVektorindexDefinition(t *testing.T) {
	var d struct {
		Fields []struct {
			Type          string `json:"type"`
			Path          string `json:"path"`
			NumDimensions int    `json:"numDimensions"`
			Similarity    string `json:"similarity"`
		} `json:"fields"`
	}
	if err := json.Unmarshal(vektorindexDefinition, &d); err != nil {
		t.Fatalf("abschnitte_vektor.json ist kein gültiges JSON: %v", err)
	}
	var vektor, filterModul bool
	for _, f := range d.Fields {
		switch {
		case f.Type == "vector" && f.Path == "embedding":
			vektor = true
			if f.NumDimensions != llm.EmbeddingDimensions {
				t.Errorf("numDimensions = %d, Embeddings haben %d", f.NumDimensions, llm.EmbeddingDimensions)
			}
			if f.Similarity != "cosine" {
				t.Errorf("similarity = %q; core.MinScore ist für cosine berechnet", f.Similarity)
			}
		case f.Type == "filter" && f.Path == "modul":
			filterModul = true
		}
	}
	if !vektor || !filterModul {
		t.Errorf("Vektorfeld embedding: %v, Filterfeld modul: %v", vektor, filterModul)
	}
}
