package gemini

import (
	"fmt"

	"google.golang.org/genai"

	"lifelog/backend/internal/llm"
)

// gemini-embedding-2 has no task_type. Instead, a prefix in the text says
// whether it is a document or a search query (formats from
// aistudio.google.com/docs/embeddings, checked 2026-11-04). Our chunks have
// no title of their own, which according to the docs means "none".
//
// CAUTION: these prefixes go into every vector. Changing them changes all
// scores, so all chunks would have to be re-embedded and the threshold
// recalibrated (core.MinScore).
func documentInput(text string) string     { return "title: none | text: " + text }
func questionInput(question string) string { return "task: search result | query: " + question }

// batchSize: at most this many texts go into one call. A long full text
// (100,000 characters / 500) has ~200 chunks; that becomes several calls
// instead of one huge one.
const batchSize = 100

// batches splits n elements into ranges [from, to) with at most size
// elements.
func batches(n, size int) [][2]int {
	ranges := [][2]int{}
	for from := 0; from < n; from += size {
		ranges = append(ranges, [2]int{from, min(from+size, n)})
	}
	return ranges
}

// embeddingContents makes a separate Content object from every text.
// Important: gemini-embedding-2 merges several parts of ONE Content into a
// single vector. Only separate Contents give one vector per text.
func embeddingContents(inputs []string) []*genai.Content {
	contents := make([]*genai.Content, len(inputs))
	for i, in := range inputs {
		contents[i] = genai.NewContentFromText(in, genai.RoleUser)
	}
	return contents
}

// vectorsFromResponse checks that exactly count vectors of the right length
// came back. A vector of the wrong length would fail later when storing or
// in the vector index; we catch that early here.
func vectorsFromResponse(resp *genai.EmbedContentResponse, count int) ([][]float32, error) {
	if resp == nil || len(resp.Embeddings) != count {
		got := 0
		if resp != nil {
			got = len(resp.Embeddings)
		}
		return nil, fmt.Errorf("expected %d embeddings, got %d: %w", count, got, llm.ErrInvalidResponse)
	}
	vectors := make([][]float32, count)
	for i, e := range resp.Embeddings {
		if e == nil || len(e.Values) != llm.EmbeddingDimensions {
			length := 0
			if e != nil {
				length = len(e.Values)
			}
			return nil, fmt.Errorf("embedding %d has %d instead of %d dimensions: %w",
				i, length, llm.EmbeddingDimensions, llm.ErrInvalidResponse)
		}
		vectors[i] = e.Values
	}
	return vectors, nil
}
