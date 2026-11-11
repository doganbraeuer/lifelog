package store

import (
	"encoding/json"
	"testing"

	"lifelog/backend/internal/llm"
)

// The definition in the repository must match the length of the embeddings,
// otherwise Atlas rejects every stored chunk or finds nothing.
func TestVectorIndexDefinition(t *testing.T) {
	var d struct {
		Fields []struct {
			Type          string `json:"type"`
			Path          string `json:"path"`
			NumDimensions int    `json:"numDimensions"`
			Similarity    string `json:"similarity"`
		} `json:"fields"`
	}
	if err := json.Unmarshal(vectorIndexDefinition, &d); err != nil {
		t.Fatalf("abschnitte_vektor.json is not valid JSON: %v", err)
	}
	var vector, moduleFilter bool
	for _, f := range d.Fields {
		switch {
		case f.Type == "vector" && f.Path == "embedding":
			vector = true
			if f.NumDimensions != llm.EmbeddingDimensions {
				t.Errorf("numDimensions = %d, embeddings have %d", f.NumDimensions, llm.EmbeddingDimensions)
			}
			if f.Similarity != "cosine" {
				t.Errorf("similarity = %q; core.MinScore is computed for cosine", f.Similarity)
			}
		case f.Type == "filter" && f.Path == "modul":
			moduleFilter = true
		}
	}
	if !vector || !moduleFilter {
		t.Errorf("vector field embedding: %v, filter field modul: %v", vector, moduleFilter)
	}
}
