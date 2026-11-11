package service

import (
	"context"
	"log/slog"
	"time"

	"lifelog/backend/internal/core"
)

// DeleteStore is what deleting needs from the database. Implemented by
// store.Store. DeleteChunks, DeleteOCR and DeleteFile must not return an
// error if there is nothing (left) to delete.
type DeleteStore interface {
	GetEntry(ctx context.Context, id string) (e core.Entry, found bool, err error)
	DeleteChunks(ctx context.Context, entryID string) error
	DeleteOCR(ctx context.Context, entryID string) error
	DeleteFile(ctx context.Context, id string) error
	DeleteEntry(ctx context.Context, id string) (found bool, err error)
}

// deleteTimeout: time for the four delete steps. They keep running without
// the request's context (see Delete), so they need a limit of their own.
const deleteTimeout = 30 * time.Second

// Deleter deletes an entry with its chunks, stored words and original file
// (docs/API.md 3.7).
type Deleter struct {
	store DeleteStore
}

func NewDeleter(s DeleteStore) *Deleter {
	return &Deleter{store: s}
}

// Delete:
//
//  1. Check the ID, load the entry (it names the ID of the original file).
//  2. Delete the chunks: from now on search no longer finds the entry.
//  3. Delete the words read from the original (collection ocr).
//  4. Delete the original file. If it is already missing, that is no error.
//  5. Delete the entry last.
//
// If a step fails, Delete stops. The entry is then still visible, and
// calling again cleans up the rest.
//
// From step 2 on, deleting runs to completion even if the app drops the
// connection (context.WithoutCancel). Otherwise an entry without file and
// chunks could be left behind, which the timeline shows but whose original
// can no longer be opened.
//
// Errors: core.ErrInvalidID, core.ErrEntryNotFound (also when deleting a
// second time) or an internal error.
func (d *Deleter) Delete(ctx context.Context, id string) error {
	if err := core.CheckID(id); err != nil {
		return err
	}
	e, found, err := d.store.GetEntry(ctx, id)
	if err != nil {
		return err
	}
	if !found {
		return core.ErrEntryNotFound
	}

	ctx, cancel := context.WithTimeout(context.WithoutCancel(ctx), deleteTimeout)
	defer cancel()

	if err := d.store.DeleteChunks(ctx, id); err != nil {
		return err
	}
	if err := d.store.DeleteOCR(ctx, id); err != nil {
		return err
	}
	if err := d.store.DeleteFile(ctx, e.File.ID); err != nil {
		return err
	}
	found, err = d.store.DeleteEntry(ctx, id)
	if err != nil {
		return err
	}
	if !found {
		// A simultaneous call was faster.
		return core.ErrEntryNotFound
	}
	// Only the ID, never content.
	slog.Info("entry deleted", "entry", id)
	return nil
}
