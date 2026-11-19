package store

import (
	"context"
	"errors"
	"fmt"
	"time"

	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"
	"go.mongodb.org/mongo-driver/v2/mongo/options"

	"lifelog/backend/internal/core"
)

// collUsage holds one counter per user, day and limit (docs/API.md, daily
// limits). In the database, so a restart does not reset it.
const collUsage = "usage"

// usageRetries: how often IncrementUsage tries again after two first
// requests of a day raced to create the counter.
const usageRetries = 3

func (s *Store) createUsageIndexes(ctx context.Context) error {
	_, err := s.usage.Indexes().CreateMany(ctx, []mongo.IndexModel{
		{
			// One counter per user, day and limit. The unique index is what
			// makes "count only below the limit" safe (see IncrementUsage).
			Keys:    bson.D{{Key: "userId", Value: 1}, {Key: "day", Value: 1}, {Key: "kind", Value: 1}},
			Options: options.Index().SetName("usage_unique").SetUnique(true),
		},
		{
			// Old counters are deleted by MongoDB itself.
			Keys:    bson.D{{Key: "expiresAt", Value: 1}},
			Options: options.Index().SetName("expires_at_ttl").SetExpireAfterSeconds(0),
		},
	})
	if err != nil {
		return fmt.Errorf("create indexes: %w", err)
	}
	return nil
}

// IncrementUsage counts one request of the user against the limit max for
// the day (core.UsageDay) and reports whether it was allowed. Checking and
// counting are one atomic step: an upsert with the filter {userId, day,
// kind, count < max} and the update {$inc: {count: 1}}.
//
//   - The counter exists and is below max: it is incremented, allowed.
//   - It does not exist yet: the upsert creates it with count 1, allowed.
//   - It is at max: the filter matches nothing, the upsert tries to create
//     a second counter for the same user, day and kind, and the unique index
//     refuses it (duplicate key): not allowed.
//
// A duplicate key can also mean that two first requests of the day raced
// to create the counter; then the counter is read again and the step
// repeated. expiresAt is when MongoDB may delete the counter.
func (s *Store) IncrementUsage(ctx context.Context, userID, day string, kind core.LimitKind, max int, expiresAt time.Time) (allowed bool, err error) {
	uid, err := ownerID(userID)
	if err != nil {
		return false, err
	}
	if max < 1 {
		return false, nil
	}
	key := bson.D{{Key: "userId", Value: uid}, {Key: "day", Value: day}, {Key: "kind", Value: string(kind)}}
	filter := append(bson.D{}, key...)
	filter = append(filter, bson.E{Key: "count", Value: bson.D{{Key: "$lt", Value: max}}})
	update := bson.D{
		{Key: "$inc", Value: bson.D{{Key: "count", Value: 1}}},
		{Key: "$setOnInsert", Value: bson.D{{Key: "expiresAt", Value: expiresAt.UTC()}}},
	}

	for attempt := 0; attempt < usageRetries; attempt++ {
		_, err := s.usage.UpdateOne(ctx, filter, update, options.UpdateOne().SetUpsert(true))
		if err == nil {
			return true, nil
		}
		if !mongo.IsDuplicateKeyError(err) {
			return false, fmt.Errorf("count usage: %w", err)
		}
		var doc struct {
			Count int `bson:"count"`
		}
		err = s.usage.FindOne(ctx, key).Decode(&doc)
		if err == nil && doc.Count >= max {
			return false, nil
		}
		if err != nil && !errors.Is(err, mongo.ErrNoDocuments) {
			return false, fmt.Errorf("count usage: %w", err)
		}
		// A race on the first request of the day: try again.
	}
	return false, errors.New("count usage: too many conflicts")
}
