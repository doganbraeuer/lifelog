// seed inserts synthetic entries into the database, so the timeline and
// search can be tested without upload and without the vision LLM. All data
// is invented (internal/sampledata).
//
//	go run ./cmd/seed               insert, with chunks and embeddings (Gemini)
//	go run ./cmd/seed -no-chunks    insert without Gemini (search does not find them then)
//	go run ./cmd/seed -remove       delete them again
//	go run ./cmd/seed -throwaway    create a throwaway entry for testing DELETE
//
// The seed entries have fixed IDs. That is why the program can be run any
// number of times: it first deletes its own entries and inserts them again.
// Other entries in the database stay untouched.
//
// -throwaway creates one more entry with a new ID on every call (without
// Gemini, see sampledata.Throwaway) and prints the ID. Only use this ID for
// trying out DELETE.
//
// The entries look like text imports: the original file is the full text
// as text.txt in GridFS. The chunks are created as on upload (same
// splitting, same embedding model), so POST /questions and cmd/calibrate find
// them.
package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"time"

	"lifelog/backend/internal/config"
	"lifelog/backend/internal/core"
	"lifelog/backend/internal/llm/gemini"
	"lifelog/backend/internal/sampledata"
	"lifelog/backend/internal/store"
)

func main() {
	remove := flag.Bool("remove", false, "only delete the seed entries, do not insert them again")
	noChunks := flag.Bool("no-chunks", false, "create no chunks and embeddings (no Gemini call)")
	throwaway := flag.Bool("throwaway", false, "only create a throwaway entry with a new ID (for testing DELETE)")
	flag.Parse()

	var err error
	if *throwaway {
		err = createThrowaway()
	} else {
		err = run(*remove, *noChunks)
	}
	if err != nil {
		fmt.Fprintln(os.Stderr, "Error:", err)
		os.Exit(1)
	}
}

func run(onlyRemove, noChunks bool) error {
	if err := config.LoadDotenv(); err != nil {
		return err
	}
	c, err := config.Load(os.Getenv)
	if err != nil {
		return err
	}

	// 60 s: embeddings up to 20 s plus the database over the network.
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	entries := sampledata.Entries()

	// Embeddings first: if Gemini fails, the database stays as it was.
	chunks := map[string][]core.Chunk{}
	if !onlyRemove && !noChunks {
		chunks, err = embed(ctx, c, entries)
		if err != nil {
			return fmt.Errorf("%w\nWithout Gemini, use -no-chunks (then search does not find the seed data)", err)
		}
	}

	db, err := store.Connect(ctx, c.MongoURI, c.MongoDB)
	if err != nil {
		return err
	}
	defer db.Disconnect(context.Background())

	if err := db.CreateIndexes(ctx); err != nil {
		return err
	}

	// Remove in the order of API.md 3.7: chunks, file, entry.
	for _, e := range entries {
		if err := db.DeleteChunks(ctx, e.ID); err != nil {
			return err
		}
		if err := db.DeleteFile(ctx, e.File.ID); err != nil {
			return err
		}
		if _, err := db.DeleteEntry(ctx, e.ID); err != nil {
			return err
		}
	}
	if onlyRemove {
		fmt.Printf("%d seed entries removed with their chunks (database %q).\n", len(entries), c.MongoDB)
		return nil
	}

	// Insert as on upload: file, chunks, entry last.
	count := 0
	for _, e := range entries {
		if err := db.SaveFile(ctx, e.File.ID, e.File.FileName, e.File.MimeType, []byte(e.FullText)); err != nil {
			return err
		}
		if err := db.SaveChunks(ctx, chunks[e.ID]); err != nil {
			return err
		}
		if err := db.SaveEntry(ctx, e); err != nil {
			return err
		}
		count += len(chunks[e.ID])
	}
	fmt.Printf("%d seed entries with %d chunks inserted (database %q).\n", len(entries), count, c.MongoDB)
	if count > 0 {
		fmt.Println("Atlas needs a few seconds until new chunks show up in the vector search.")
	}
	return nil
}

// embed splits all seed entries and gets the embeddings in one go
// (EmbedChunks splits into batches itself). Result by entry ID.
func embed(ctx context.Context, c config.Config, entries []core.Entry) (map[string][]core.Chunk, error) {
	g, err := gemini.New(ctx, gemini.Settings{
		APIKey:         c.GeminiKey,
		Model:          c.GeminiModel,
		EmbeddingModel: c.GeminiEmbeddingModel,
	})
	if err != nil {
		return nil, err
	}

	var all []core.Chunk
	var texts []string
	for _, e := range entries {
		for _, ch := range sampledata.Chunks(e) {
			all = append(all, ch)
			texts = append(texts, ch.Text)
		}
	}
	vectors, err := g.EmbedChunks(ctx, texts)
	if err != nil {
		return nil, fmt.Errorf("embeddings (%s): %w", c.GeminiEmbeddingModel, err)
	}
	if len(vectors) != len(texts) {
		return nil, fmt.Errorf("%d embeddings for %d chunks", len(vectors), len(texts))
	}

	result := map[string][]core.Chunk{}
	for i, ch := range all {
		ch.Embedding = vectors[i]
		result[ch.EntryID] = append(result[ch.EntryID], ch)
	}
	return result, nil
}

// createThrowaway stores a throwaway entry as on upload (file, chunks,
// entry last) and prints its ID. The fixed seed entries and all other
// entries stay untouched.
func createThrowaway() error {
	if err := config.LoadDotenv(); err != nil {
		return err
	}
	c, err := config.Load(os.Getenv)
	if err != nil {
		return err
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	db, err := store.Connect(ctx, c.MongoURI, c.MongoDB)
	if err != nil {
		return err
	}
	defer db.Disconnect(context.Background())

	e, chunks := sampledata.Throwaway(store.NewID(), store.NewID(), time.Now())
	if err := db.SaveFile(ctx, e.File.ID, e.File.FileName, e.File.MimeType, []byte(e.FullText)); err != nil {
		return err
	}
	if err := db.SaveChunks(ctx, chunks); err != nil {
		return err
	}
	if err := db.SaveEntry(ctx, e); err != nil {
		return err
	}
	fmt.Printf("throwaway entry with %d chunk(s) created (database %q):\n%s\n", len(chunks), c.MongoDB, e.ID)
	return nil
}
