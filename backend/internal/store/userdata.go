package store

import (
	"context"
	"fmt"

	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"
)

// DataCounts: how many documents of each kind an operation on one user's
// data touched (cmd/users assign-orphans and delete).
type DataCounts struct {
	Entries int64
	Chunks  int64
	OCR     int64
	Files   int64
	Usage   int64
	// Routines and RoutineLogs: only delete and count; assign-orphans does
	// not touch them (they never existed without an owner).
	Routines    int64
	RoutineLogs int64
}

// Total of all counts.
func (c DataCounts) Total() int64 {
	return c.Entries + c.Chunks + c.OCR + c.Files + c.Usage + c.Routines + c.RoutineLogs
}

// AssignOrphans gives every entry, chunk, OCR document and original file
// without an owner to the user (data from before 2026-11-18). Until then
// such documents are visible to nobody, because every query filters by
// userId. Documents that already have an owner are never touched, so the
// function can be run again: the second time all counts are 0.
func (s *Store) AssignOrphans(ctx context.Context, userID string) (DataCounts, error) {
	uid, err := ownerID(userID)
	if err != nil {
		return DataCounts{}, err
	}
	var c DataCounts
	if c.Entries, err = assignOwner(ctx, s.entries, uid); err != nil {
		return c, fmt.Errorf("assign orphans (entries): %w", err)
	}
	if c.Chunks, err = assignOwner(ctx, s.chunks, uid); err != nil {
		return c, fmt.Errorf("assign orphans (chunks): %w", err)
	}
	if c.OCR, err = assignOwner(ctx, s.ocr, uid); err != nil {
		return c, fmt.Errorf("assign orphans (ocr): %w", err)
	}
	if c.Files, err = s.assignOrphanFiles(ctx, uid); err != nil {
		return c, fmt.Errorf("assign orphans (originals): %w", err)
	}
	return c, nil
}

// assignOwner sets userId on every document of coll that has none.
func assignOwner(ctx context.Context, coll *mongo.Collection, uid bson.ObjectID) (int64, error) {
	res, err := coll.UpdateMany(ctx,
		bson.D{{Key: "userId", Value: bson.D{{Key: "$exists", Value: false}}}},
		bson.D{{Key: "$set", Value: bson.D{{Key: "userId", Value: uid}}}})
	if err != nil {
		return 0, err
	}
	return res.ModifiedCount, nil
}

// assignOrphanFiles: the owner of a file is metadata.userId. A file
// without metadata (or metadata null) gets a metadata document first,
// otherwise MongoDB cannot set a field inside it.
func (s *Store) assignOrphanFiles(ctx context.Context, uid bson.ObjectID) (int64, error) {
	noMeta, err := s.fileDocs.UpdateMany(ctx,
		bson.D{{Key: "metadata", Value: nil}},
		bson.D{{Key: "$set", Value: bson.D{{Key: "metadata", Value: bson.D{{Key: "userId", Value: uid}}}}}})
	if err != nil {
		return 0, err
	}
	withMeta, err := s.fileDocs.UpdateMany(ctx,
		bson.D{{Key: "metadata.userId", Value: bson.D{{Key: "$exists", Value: false}}}},
		bson.D{{Key: "$set", Value: bson.D{{Key: "metadata.userId", Value: uid}}}})
	if err != nil {
		return 0, err
	}
	return noMeta.ModifiedCount + withMeta.ModifiedCount, nil
}

// DeleteUserData deletes everything the user owns: routine logs, routines,
// chunks, OCR words, original files (with their GridFS pieces), entries
// and usage counters. Logs before their routines and everything before the
// entries, like deleting one routine or entry (API.md 3.7, 3.23): an
// interrupted run leaves data the user can still see, and a second run
// cleans up the rest. Account and sessions are not touched here.
func (s *Store) DeleteUserData(ctx context.Context, userID string) (DataCounts, error) {
	uid, err := ownerID(userID)
	if err != nil {
		return DataCounts{}, err
	}
	owner := bson.D{{Key: "userId", Value: uid}}
	var c DataCounts

	res, err := s.routineLogs.DeleteMany(ctx, owner)
	if err != nil {
		return c, fmt.Errorf("delete routine logs: %w", err)
	}
	c.RoutineLogs = res.DeletedCount

	res, err = s.routines.DeleteMany(ctx, owner)
	if err != nil {
		return c, fmt.Errorf("delete routines: %w", err)
	}
	c.Routines = res.DeletedCount

	res, err = s.chunks.DeleteMany(ctx, owner)
	if err != nil {
		return c, fmt.Errorf("delete chunks: %w", err)
	}
	c.Chunks = res.DeletedCount

	res, err = s.ocr.DeleteMany(ctx, owner)
	if err != nil {
		return c, fmt.Errorf("delete ocr words: %w", err)
	}
	c.OCR = res.DeletedCount

	cur, err := s.fileDocs.Find(ctx, bson.D{{Key: "metadata.userId", Value: uid}})
	if err != nil {
		return c, fmt.Errorf("list files: %w", err)
	}
	var files []struct {
		ID bson.ObjectID `bson:"_id"`
	}
	if err := cur.All(ctx, &files); err != nil {
		return c, fmt.Errorf("list files: %w", err)
	}
	for _, f := range files {
		// Delete removes the file document and its pieces.
		if err := s.files.Delete(ctx, f.ID); err != nil {
			return c, fmt.Errorf("delete file: %w", err)
		}
		c.Files++
	}

	res, err = s.entries.DeleteMany(ctx, owner)
	if err != nil {
		return c, fmt.Errorf("delete entries: %w", err)
	}
	c.Entries = res.DeletedCount

	res, err = s.usage.DeleteMany(ctx, owner)
	if err != nil {
		return c, fmt.Errorf("delete usage: %w", err)
	}
	c.Usage = res.DeletedCount
	return c, nil
}

// CountUserData counts what the user still owns, e.g. to prove after
// DeleteUserData that nothing is left.
func (s *Store) CountUserData(ctx context.Context, userID string) (DataCounts, error) {
	uid, err := ownerID(userID)
	if err != nil {
		return DataCounts{}, err
	}
	owner := bson.D{{Key: "userId", Value: uid}}
	var c DataCounts
	var errs []error
	count := func(n *int64, f func() (int64, error)) {
		v, err := f()
		*n = v
		if err != nil {
			errs = append(errs, err)
		}
	}
	count(&c.Entries, func() (int64, error) { return s.entries.CountDocuments(ctx, owner) })
	count(&c.Chunks, func() (int64, error) { return s.chunks.CountDocuments(ctx, owner) })
	count(&c.OCR, func() (int64, error) { return s.ocr.CountDocuments(ctx, owner) })
	count(&c.Files, func() (int64, error) {
		return s.fileDocs.CountDocuments(ctx, bson.D{{Key: "metadata.userId", Value: uid}})
	})
	count(&c.Usage, func() (int64, error) { return s.usage.CountDocuments(ctx, owner) })
	count(&c.Routines, func() (int64, error) { return s.routines.CountDocuments(ctx, owner) })
	count(&c.RoutineLogs, func() (int64, error) { return s.routineLogs.CountDocuments(ctx, owner) })
	if len(errs) > 0 {
		return c, fmt.Errorf("count user data: %w", errs[0])
	}
	return c, nil
}
