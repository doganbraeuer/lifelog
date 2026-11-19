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

// Accounts and login (docs/API.md, section 1 and 3.10–3.15; names from
// docs/GLOSSARY.md, "Accounts and login").
const (
	collUsers    = "users"
	collSessions = "sessions"
	collAuditLog = "auditLog"
)

type userDoc struct {
	ID                 bson.ObjectID `bson:"_id"`
	Username           string        `bson:"username"`
	DisplayName        string        `bson:"displayName"`
	PasswordHash       string        `bson:"passwordHash"`
	Role               string        `bson:"role"`
	MustChangePassword bool          `bson:"mustChangePassword"`
	Disabled           bool          `bson:"disabled"`
	CreatedAt          time.Time     `bson:"createdAt"`
	// Stored as null until the consent is recorded (later PR).
	ConsentVersion *string    `bson:"consentVersion"`
	ConsentAt      *time.Time `bson:"consentAt"`
}

type sessionDoc struct {
	ID        bson.ObjectID `bson:"_id"`
	TokenHash string        `bson:"tokenHash"`
	UserID    bson.ObjectID `bson:"userId"`
	CreatedAt time.Time     `bson:"createdAt"`
	// The TTL index expires_at_ttl lets MongoDB delete the session once this
	// time has passed. Its background job runs about once a minute, so the
	// service checks the time itself as well.
	ExpiresAt time.Time `bson:"expiresAt"`
}

type auditDoc struct {
	ID             bson.ObjectID `bson:"_id"`
	Time           time.Time     `bson:"time"`
	Action         string        `bson:"action"`
	TargetUsername string        `bson:"targetUsername,omitempty"`
	Operator       string        `bson:"operator"`
}

func toUserDoc(u core.User) (userDoc, error) {
	id, err := bson.ObjectIDFromHex(u.ID)
	if err != nil {
		return userDoc{}, fmt.Errorf("invalid user ID: %w", err)
	}
	var consentAt *time.Time
	if u.ConsentAt != nil {
		t := u.ConsentAt.UTC()
		consentAt = &t
	}
	return userDoc{
		ID:                 id,
		Username:           u.Username,
		DisplayName:        u.DisplayName,
		PasswordHash:       u.PasswordHash,
		Role:               string(u.Role),
		MustChangePassword: u.MustChangePassword,
		Disabled:           u.Disabled,
		CreatedAt:          u.CreatedAt.UTC(),
		ConsentVersion:     u.ConsentVersion,
		ConsentAt:          consentAt,
	}, nil
}

func (d userDoc) toCore() core.User {
	var consentAt *time.Time
	if d.ConsentAt != nil {
		t := d.ConsentAt.UTC()
		consentAt = &t
	}
	return core.User{
		ID:                 d.ID.Hex(),
		Username:           d.Username,
		DisplayName:        d.DisplayName,
		PasswordHash:       d.PasswordHash,
		Role:               core.Role(d.Role),
		MustChangePassword: d.MustChangePassword,
		Disabled:           d.Disabled,
		CreatedAt:          d.CreatedAt.UTC(),
		ConsentVersion:     d.ConsentVersion,
		ConsentAt:          consentAt,
	}
}

func (d sessionDoc) toCore() core.Session {
	return core.Session{
		ID:        d.ID.Hex(),
		TokenHash: d.TokenHash,
		UserID:    d.UserID.Hex(),
		CreatedAt: d.CreatedAt.UTC(),
		ExpiresAt: d.ExpiresAt.UTC(),
	}
}

// createAccountIndexes is called by CreateIndexes.
func (s *Store) createAccountIndexes(ctx context.Context) error {
	// A username exists only once. The unique index also catches two
	// cmd/users runs that create the same name at the same time.
	_, err := s.users.Indexes().CreateOne(ctx, mongo.IndexModel{
		Keys:    bson.D{{Key: "username", Value: 1}},
		Options: options.Index().SetName("username_unique").SetUnique(true),
	})
	if err != nil {
		return fmt.Errorf("create indexes: %w", err)
	}

	_, err = s.sessions.Indexes().CreateMany(ctx, []mongo.IndexModel{
		{
			// Every request looks up its session by the token hash.
			Keys:    bson.D{{Key: "tokenHash", Value: 1}},
			Options: options.Index().SetName("token_hash_unique").SetUnique(true),
		},
		{
			// TTL index: MongoDB deletes a session once expiresAt has passed
			// (expireAfterSeconds 0 = at exactly that time).
			Keys:    bson.D{{Key: "expiresAt", Value: 1}},
			Options: options.Index().SetName("expires_at_ttl").SetExpireAfterSeconds(0),
		},
		{
			// Password change, disable, revoke and delete remove all
			// sessions of one user.
			Keys:    bson.D{{Key: "userId", Value: 1}},
			Options: options.Index().SetName("userId"),
		},
	})
	if err != nil {
		return fmt.Errorf("create indexes: %w", err)
	}
	return nil
}

// CreateUser inserts a new user. u.ID must be a valid ObjectId (NewID). If
// the username exists, the error is core.ErrUsernameTaken.
func (s *Store) CreateUser(ctx context.Context, u core.User) error {
	doc, err := toUserDoc(u)
	if err != nil {
		return err
	}
	if _, err := s.users.InsertOne(ctx, doc); err != nil {
		if mongo.IsDuplicateKeyError(err) {
			return core.ErrUsernameTaken
		}
		return fmt.Errorf("create user: %w", err)
	}
	return nil
}

// UserByUsername looks up a user by the stored (lowercase) username.
func (s *Store) UserByUsername(ctx context.Context, username string) (u core.User, found bool, err error) {
	return s.findUser(ctx, bson.D{{Key: "username", Value: username}})
}

// UserByID looks up a user by ID.
func (s *Store) UserByID(ctx context.Context, id string) (u core.User, found bool, err error) {
	oid, err := bson.ObjectIDFromHex(id)
	if err != nil {
		return core.User{}, false, core.ErrInvalidID
	}
	return s.findUser(ctx, bson.D{{Key: "_id", Value: oid}})
}

func (s *Store) findUser(ctx context.Context, filter bson.D) (core.User, bool, error) {
	var doc userDoc
	err := s.users.FindOne(ctx, filter).Decode(&doc)
	if errors.Is(err, mongo.ErrNoDocuments) {
		return core.User{}, false, nil
	}
	if err != nil {
		return core.User{}, false, fmt.Errorf("read user: %w", err)
	}
	return doc.toCore(), true, nil
}

// ListUsers returns all users sorted by username.
func (s *Store) ListUsers(ctx context.Context) ([]core.User, error) {
	cur, err := s.users.Find(ctx, bson.D{}, options.Find().SetSort(bson.D{{Key: "username", Value: 1}}))
	if err != nil {
		return nil, fmt.Errorf("read users: %w", err)
	}
	var docs []userDoc
	if err := cur.All(ctx, &docs); err != nil {
		return nil, fmt.Errorf("read users: %w", err)
	}
	list := make([]core.User, 0, len(docs))
	for _, d := range docs {
		list = append(list, d.toCore())
	}
	return list, nil
}

// SetPassword stores a new password hash and mustChangePassword. found is
// false if the user does not exist (anymore).
func (s *Store) SetPassword(ctx context.Context, id, passwordHash string, mustChange bool) (found bool, err error) {
	return s.updateUser(ctx, id, bson.D{
		{Key: "passwordHash", Value: passwordHash},
		{Key: "mustChangePassword", Value: mustChange},
	})
}

// SetDisplayName stores a new display name (already checked by core).
func (s *Store) SetDisplayName(ctx context.Context, id, displayName string) (found bool, err error) {
	return s.updateUser(ctx, id, bson.D{{Key: "displayName", Value: displayName}})
}

// SetDisabled disables or enables an account. Disabling does not delete the
// sessions; the caller does that (DeleteSessionsOfUser). Even without that,
// a disabled account is refused on the next request.
func (s *Store) SetDisabled(ctx context.Context, id string, disabled bool) (found bool, err error) {
	return s.updateUser(ctx, id, bson.D{{Key: "disabled", Value: disabled}})
}

func (s *Store) updateUser(ctx context.Context, id string, set bson.D) (bool, error) {
	oid, err := bson.ObjectIDFromHex(id)
	if err != nil {
		return false, core.ErrInvalidID
	}
	res, err := s.users.UpdateOne(ctx, bson.D{{Key: "_id", Value: oid}}, bson.D{{Key: "$set", Value: set}})
	if err != nil {
		return false, fmt.Errorf("update user: %w", err)
	}
	return res.MatchedCount > 0, nil
}

// DeleteUser deletes only the user document. The caller deletes the
// sessions first (DeleteSessionsOfUser).
func (s *Store) DeleteUser(ctx context.Context, id string) (found bool, err error) {
	oid, err := bson.ObjectIDFromHex(id)
	if err != nil {
		return false, core.ErrInvalidID
	}
	res, err := s.users.DeleteOne(ctx, bson.D{{Key: "_id", Value: oid}})
	if err != nil {
		return false, fmt.Errorf("delete user: %w", err)
	}
	return res.DeletedCount > 0, nil
}

// CreateSession stores a session. It contains the token hash, never the
// token.
func (s *Store) CreateSession(ctx context.Context, sess core.Session) error {
	id, err := bson.ObjectIDFromHex(sess.ID)
	if err != nil {
		return fmt.Errorf("invalid session ID: %w", err)
	}
	userID, err := bson.ObjectIDFromHex(sess.UserID)
	if err != nil {
		return fmt.Errorf("invalid user ID: %w", err)
	}
	_, err = s.sessions.InsertOne(ctx, sessionDoc{
		ID:        id,
		TokenHash: sess.TokenHash,
		UserID:    userID,
		CreatedAt: sess.CreatedAt.UTC(),
		ExpiresAt: sess.ExpiresAt.UTC(),
	})
	if err != nil {
		return fmt.Errorf("create session: %w", err)
	}
	return nil
}

// SessionByTokenHash looks up the session of a token. It may already be
// expired (the TTL job runs only about once a minute); the caller checks
// ExpiresAt.
func (s *Store) SessionByTokenHash(ctx context.Context, tokenHash string) (sess core.Session, found bool, err error) {
	var doc sessionDoc
	err = s.sessions.FindOne(ctx, bson.D{{Key: "tokenHash", Value: tokenHash}}).Decode(&doc)
	if errors.Is(err, mongo.ErrNoDocuments) {
		return core.Session{}, false, nil
	}
	if err != nil {
		return core.Session{}, false, fmt.Errorf("read session: %w", err)
	}
	return doc.toCore(), true, nil
}

// DeleteSession ends one session (logout). A session that is already gone
// is not an error.
func (s *Store) DeleteSession(ctx context.Context, id string) error {
	oid, err := bson.ObjectIDFromHex(id)
	if err != nil {
		return fmt.Errorf("invalid session ID: %w", err)
	}
	if _, err := s.sessions.DeleteOne(ctx, bson.D{{Key: "_id", Value: oid}}); err != nil {
		return fmt.Errorf("delete session: %w", err)
	}
	return nil
}

// DeleteSessionsOfUser ends all sessions of a user and returns how many
// there were.
func (s *Store) DeleteSessionsOfUser(ctx context.Context, userID string) (int64, error) {
	oid, err := bson.ObjectIDFromHex(userID)
	if err != nil {
		return 0, fmt.Errorf("invalid user ID: %w", err)
	}
	return s.deleteSessions(ctx, bson.D{{Key: "userId", Value: oid}})
}

// DeleteOtherSessions ends all sessions of a user except keepID (password
// change: the device that changed it stays logged in).
func (s *Store) DeleteOtherSessions(ctx context.Context, userID, keepID string) (int64, error) {
	oid, err := bson.ObjectIDFromHex(userID)
	if err != nil {
		return 0, fmt.Errorf("invalid user ID: %w", err)
	}
	keep, err := bson.ObjectIDFromHex(keepID)
	if err != nil {
		return 0, fmt.Errorf("invalid session ID: %w", err)
	}
	return s.deleteSessions(ctx, bson.D{
		{Key: "userId", Value: oid},
		{Key: "_id", Value: bson.D{{Key: "$ne", Value: keep}}},
	})
}

// DeleteAllSessions ends every session of every user.
func (s *Store) DeleteAllSessions(ctx context.Context) (int64, error) {
	return s.deleteSessions(ctx, bson.D{})
}

func (s *Store) deleteSessions(ctx context.Context, filter bson.D) (int64, error) {
	res, err := s.sessions.DeleteMany(ctx, filter)
	if err != nil {
		return 0, fmt.Errorf("delete sessions: %w", err)
	}
	return res.DeletedCount, nil
}

// WriteAudit appends one entry to the audit log.
func (s *Store) WriteAudit(ctx context.Context, e core.AuditEntry) error {
	_, err := s.auditLog.InsertOne(ctx, auditDoc{
		ID:             bson.NewObjectID(),
		Time:           e.Time.UTC(),
		Action:         e.Action,
		TargetUsername: e.TargetUsername,
		Operator:       e.Operator,
	})
	if err != nil {
		return fmt.Errorf("write audit log: %w", err)
	}
	return nil
}
