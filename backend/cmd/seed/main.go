// seed inserts synthetic entries into the database, so the timeline and
// search can be tested without upload and without the vision LLM. All data
// is invented (internal/sampledata).
//
//	go run ./cmd/seed --username admin2               insert, with chunks and embeddings (Gemini)
//	go run ./cmd/seed --username admin2 -no-chunks    insert without Gemini (search does not find them then)
//	go run ./cmd/seed --username admin2 -remove       delete them again
//	go run ./cmd/seed --username admin2 -throwaway    create a throwaway entry for testing DELETE
//
// Since 2026-11-18 every entry belongs to an account (docs/API.md, per-user
// data). --username is required and names the account that owns the seed
// entries, their chunks and files. Because the seed entries have fixed IDs,
// only one account per database can hold them.
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
	"errors"
	"flag"
	"fmt"
	"os"
	"strings"
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
	username := flag.String("username", "", "account that owns the seed entries (required)")
	flag.Parse()

	var err error
	switch {
	case strings.TrimSpace(*username) == "":
		err = errors.New("--username is required: entries belong to an account since 2026-11-18, e.g. go run ./cmd/seed --username admin2")
	case *throwaway:
		err = createThrowaway(*username)
	default:
		err = run(*username, *remove, *noChunks)
	}
	if err != nil {
		fmt.Fprintln(os.Stderr, "Error:", err)
		os.Exit(1)
	}
}

// owner looks up the account that owns the seed entries.
func owner(ctx context.Context, db *store.Store, username, database string) (string, error) {
	u, found, err := db.UserByUsername(ctx, core.NormalizeUsername(username))
	if err != nil {
		return "", err
	}
	if !found {
		return "", fmt.Errorf("no user %q in database %q (create it with go run ./cmd/users add)", username, database)
	}
	return u.ID, nil
}

func run(username string, onlyRemove, noChunks bool) error {
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
	userID, err := owner(ctx, db, username, c.MongoDB)
	if err != nil {
		return err
	}

	// Remove in the order of API.md 3.7: chunks, file, entry. Only the
	// account's own seed entries.
	for _, e := range entries {
		if err := db.DeleteChunks(ctx, userID, e.ID); err != nil {
			return err
		}
		if err := db.DeleteFile(ctx, userID, e.File.ID); err != nil {
			return err
		}
		if _, err := db.DeleteEntry(ctx, userID, e.ID); err != nil {
			return err
		}
	}
	if onlyRemove {
		fmt.Printf("%d seed entries of %s removed with their chunks (database %q).\n", len(entries), username, c.MongoDB)
		return nil
	}

	// Insert as on upload: file, chunks, entry last.
	count := 0
	for _, e := range entries {
		if err := db.SaveFile(ctx, userID, e.File.ID, e.File.FileName, e.File.MimeType, []byte(e.FullText)); err != nil {
			return err
		}
		if err := db.SaveChunks(ctx, userID, chunks[e.ID]); err != nil {
			return err
		}
		if err := db.SaveEntry(ctx, userID, e); err != nil {
			return err
		}
		count += len(chunks[e.ID])
	}
	fmt.Printf("%d seed entries with %d chunks inserted for %s (database %q).\n", len(entries), count, username, c.MongoDB)
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
func createThrowaway(username string) error {
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
	userID, err := owner(ctx, db, username, c.MongoDB)
	if err != nil {
		return err
	}

	e, chunks := sampledata.Throwaway(store.NewID(), store.NewID(), time.Now())
	if err := db.SaveFile(ctx, userID, e.File.ID, e.File.FileName, e.File.MimeType, []byte(e.FullText)); err != nil {
		return err
	}
	if err := db.SaveChunks(ctx, userID, chunks); err != nil {
		return err
	}
	if err := db.SaveEntry(ctx, userID, e); err != nil {
		return err
	}
	fmt.Printf("throwaway entry with %d chunk(s) created (database %q):\n%s\n", len(chunks), c.MongoDB, e.ID)
	return nil
}
