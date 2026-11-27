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

// Routines and their logs (docs/API.md 3.16–3.27; names from
// docs/GLOSSARY.md, "Routines"). Like entries, every document carries its
// owner in userId, and every function takes the user ID and filters by it
// (ownerID). Logs are found by userId and routineId together, so a log ID
// of another routine is never touched.
//
// Duplicate keys are translated into core.ErrRoutineNameTaken and
// core.ErrRoutineLogExists and never passed on as they are: MongoDB's
// E11000 message contains the key value, i.e. the name of the routine, and
// the caller could log it.
const (
	collRoutines    = "routines"
	collRoutineLogs = "routineLogs"
)

type routineDoc struct {
	ID     bson.ObjectID `bson:"_id"`
	UserID bson.ObjectID `bson:"userId"`
	Module string        `bson:"module"`
	Name   string        `bson:"name"`
	// NameKey is the name in lower case (core.RoutineNameKey), for the
	// unique index user_module_name_unique.
	NameKey   string      `bson:"nameKey"`
	Amount    *float64    `bson:"amount"`
	Unit      *string     `bson:"unit"`
	Schedule  scheduleDoc `bson:"schedule"`
	StartDate string      `bson:"startDate"` // YYYY-MM-DD, like documentDate
	EndDate   *string     `bson:"endDate"`
	Pauses    []pauseDoc  `bson:"pauses"` // never null, so SetPauses can compare
	CreatedAt time.Time   `bson:"createdAt"`
}

// scheduleDoc holds only the fields of its frequency.
type scheduleDoc struct {
	Frequency   string   `bson:"frequency"`
	Interval    int      `bson:"interval"`
	Weekdays    []string `bson:"weekdays,omitempty"` // "mon" … "sun"
	Times       []string `bson:"times,omitempty"`    // "HH:MM"
	WindowStart string   `bson:"windowStart,omitempty"`
	WindowEnd   string   `bson:"windowEnd,omitempty"`
}

type pauseDoc struct {
	From  string  `bson:"from"`
	Until *string `bson:"until"`
}

type routineLogDoc struct {
	ID        bson.ObjectID `bson:"_id"`
	UserID    bson.ObjectID `bson:"userId"`
	RoutineID bson.ObjectID `bson:"routineId"`
	DueAt     *time.Time    `bson:"dueAt"`
	// DueLocal is missing (not null) for a log without an occurrence: the
	// unique index routine_due_unique only covers documents where it is a
	// string.
	DueLocal  string         `bson:"dueLocal,omitempty"`
	TakenAt   time.Time      `bson:"takenAt"`
	Status    string         `bson:"status"`
	Amount    *float64       `bson:"amount"`
	Source    string         `bson:"source"`
	EntryID   *bson.ObjectID `bson:"entryId"`
	TimeZone  string         `bson:"timeZone"`
	CreatedAt time.Time      `bson:"createdAt"`
}

func toRoutineDoc(r core.Routine) (routineDoc, error) {
	id, err := bson.ObjectIDFromHex(r.ID)
	if err != nil {
		return routineDoc{}, fmt.Errorf("invalid routine ID: %w", err)
	}
	doc := routineDoc{
		ID:        id,
		Module:    r.Module,
		Name:      r.Name,
		NameKey:   core.RoutineNameKey(r.Name),
		Amount:    r.Amount,
		Unit:      r.Unit,
		Schedule:  toScheduleDoc(r.Schedule),
		StartDate: r.StartDate.String(),
		EndDate:   dateString(r.EndDate),
		Pauses:    toPauseDocs(r.Pauses),
		CreatedAt: r.CreatedAt.UTC(),
	}
	return doc, nil
}

func toScheduleDoc(s core.Schedule) scheduleDoc {
	doc := scheduleDoc{Frequency: string(s.Frequency), Interval: s.Interval}
	for _, d := range s.Weekdays {
		doc.Weekdays = append(doc.Weekdays, core.WeekdayCode(d))
	}
	for _, t := range s.Times {
		doc.Times = append(doc.Times, t.String())
	}
	if s.Frequency == core.FrequencyHourly {
		doc.WindowStart, doc.WindowEnd = s.WindowStart.String(), s.WindowEnd.String()
	}
	return doc
}

func toPauseDocs(pauses []core.Pause) []pauseDoc {
	docs := make([]pauseDoc, 0, len(pauses))
	for _, p := range pauses {
		docs = append(docs, pauseDoc{From: p.From.String(), Until: dateString(p.Until)})
	}
	return docs
}

func dateString(d *core.Date) *string {
	if d == nil {
		return nil
	}
	s := d.String()
	return &s
}

// errStoredRoutine: a stored value cannot be read back. Only the ID and
// the field name go into the error, never the value.
func errStoredRoutine(id bson.ObjectID, field string) error {
	return fmt.Errorf("stored routine %s has an invalid %s", id.Hex(), field)
}

func (d routineDoc) toCore() (core.Routine, error) {
	r := core.Routine{
		ID:        d.ID.Hex(),
		UserID:    d.UserID.Hex(),
		Module:    d.Module,
		Name:      d.Name,
		Amount:    d.Amount,
		Unit:      d.Unit,
		CreatedAt: d.CreatedAt.UTC(),
		Pauses:    make([]core.Pause, 0, len(d.Pauses)),
	}
	var ok bool
	if r.StartDate, ok = core.ParseDate(d.StartDate); !ok {
		return core.Routine{}, errStoredRoutine(d.ID, "startDate")
	}
	if d.EndDate != nil {
		end, ok := core.ParseDate(*d.EndDate)
		if !ok {
			return core.Routine{}, errStoredRoutine(d.ID, "endDate")
		}
		r.EndDate = &end
	}
	s := core.Schedule{Frequency: core.Frequency(d.Schedule.Frequency), Interval: d.Schedule.Interval}
	if core.MaxInterval(s.Frequency) == 0 || s.Interval < 1 {
		return core.Routine{}, errStoredRoutine(d.ID, "schedule")
	}
	for _, code := range d.Schedule.Weekdays {
		wd, ok := core.ParseWeekdayCode(code)
		if !ok {
			return core.Routine{}, errStoredRoutine(d.ID, "schedule.weekdays")
		}
		s.Weekdays = append(s.Weekdays, wd)
	}
	for _, t := range d.Schedule.Times {
		c, ok := core.ParseClockTime(t)
		if !ok {
			return core.Routine{}, errStoredRoutine(d.ID, "schedule.times")
		}
		s.Times = append(s.Times, c)
	}
	if s.Frequency == core.FrequencyHourly {
		if s.WindowStart, ok = core.ParseClockTime(d.Schedule.WindowStart); !ok {
			return core.Routine{}, errStoredRoutine(d.ID, "schedule.windowStart")
		}
		if s.WindowEnd, ok = core.ParseClockTime(d.Schedule.WindowEnd); !ok {
			return core.Routine{}, errStoredRoutine(d.ID, "schedule.windowEnd")
		}
	}
	r.Schedule = s
	for _, p := range d.Pauses {
		from, ok := core.ParseDate(p.From)
		if !ok {
			return core.Routine{}, errStoredRoutine(d.ID, "pauses")
		}
		pause := core.Pause{From: from}
		if p.Until != nil {
			until, ok := core.ParseDate(*p.Until)
			if !ok {
				return core.Routine{}, errStoredRoutine(d.ID, "pauses")
			}
			pause.Until = &until
		}
		r.Pauses = append(r.Pauses, pause)
	}
	return r, nil
}

func toRoutineLogDoc(l core.RoutineLog) (routineLogDoc, error) {
	id, err := bson.ObjectIDFromHex(l.ID)
	if err != nil {
		return routineLogDoc{}, fmt.Errorf("invalid log ID: %w", err)
	}
	routineID, err := bson.ObjectIDFromHex(l.RoutineID)
	if err != nil {
		return routineLogDoc{}, fmt.Errorf("invalid routine ID: %w", err)
	}
	doc := routineLogDoc{
		ID:        id,
		RoutineID: routineID,
		DueLocal:  l.DueLocal,
		TakenAt:   l.TakenAt.UTC(),
		Status:    string(l.Status),
		Amount:    l.Amount,
		Source:    string(l.Source),
		TimeZone:  l.TimeZone,
		CreatedAt: l.CreatedAt.UTC(),
	}
	if l.DueAt != nil {
		due := l.DueAt.UTC()
		doc.DueAt = &due
	}
	if l.EntryID != nil {
		entry, err := bson.ObjectIDFromHex(*l.EntryID)
		if err != nil {
			return routineLogDoc{}, fmt.Errorf("invalid entry ID: %w", err)
		}
		doc.EntryID = &entry
	}
	return doc, nil
}

func (d routineLogDoc) toCore() core.RoutineLog {
	l := core.RoutineLog{
		ID:        d.ID.Hex(),
		UserID:    d.UserID.Hex(),
		RoutineID: d.RoutineID.Hex(),
		DueLocal:  d.DueLocal,
		TakenAt:   d.TakenAt.UTC(),
		Status:    core.LogStatus(d.Status),
		Amount:    d.Amount,
		Source:    core.LogSource(d.Source),
		TimeZone:  d.TimeZone,
		CreatedAt: d.CreatedAt.UTC(),
	}
	if d.DueAt != nil {
		due := d.DueAt.UTC()
		l.DueAt = &due
	}
	if d.EntryID != nil {
		entry := d.EntryID.Hex()
		l.EntryID = &entry
	}
	return l
}

// createRoutineIndexes is called by CreateIndexes. These are normal
// indexes, not search indexes; the free tier's limit on search indexes is
// not touched.
func (s *Store) createRoutineIndexes(ctx context.Context) error {
	_, err := s.routines.Indexes().CreateMany(ctx, []mongo.IndexModel{
		{
			// The list of routines filters by user and module.
			Keys:    bson.D{{Key: "userId", Value: 1}, {Key: "module", Value: 1}},
			Options: options.Index().SetName("user_module"),
		},
		{
			// A name exists once per user and module, ignoring upper and
			// lower case. The index also catches two simultaneous requests.
			Keys:    bson.D{{Key: "userId", Value: 1}, {Key: "module", Value: 1}, {Key: "nameKey", Value: 1}},
			Options: options.Index().SetName("user_module_name_unique").SetUnique(true),
		},
	})
	if err != nil {
		return fmt.Errorf("create indexes: %w", err)
	}
	_, err = s.routineLogs.Indexes().CreateMany(ctx, []mongo.IndexModel{
		{
			// At most one log per occurrence (API.md 3.16 "Logs", rule 1),
			// also for a reminder button sent twice at the same moment.
			// Partial: only logs whose dueLocal is a string. With
			// {$exists: true} a stored null would count as well; therefore
			// logs without an occurrence have no dueLocal at all.
			Keys: bson.D{{Key: "userId", Value: 1}, {Key: "routineId", Value: 1}, {Key: "dueLocal", Value: 1}},
			Options: options.Index().SetName("routine_due_unique").SetUnique(true).
				SetPartialFilterExpression(bson.D{{Key: "dueLocal", Value: bson.D{{Key: "$type", Value: "string"}}}}),
		},
		{
			// The logs of a routine by takenAt (list, limit per day,
			// progress).
			Keys:    bson.D{{Key: "userId", Value: 1}, {Key: "routineId", Value: 1}, {Key: "takenAt", Value: 1}},
			Options: options.Index().SetName("user_routine_taken"),
		},
	})
	if err != nil {
		return fmt.Errorf("create indexes: %w", err)
	}
	return nil
}

// routineFilter: the user's routine with this ID. An invalid ID is
// core.ErrInvalidID.
func routineFilter(uid bson.ObjectID, id string) (bson.D, error) {
	oid, err := bson.ObjectIDFromHex(id)
	if err != nil {
		return nil, core.ErrInvalidID
	}
	return bson.D{{Key: "_id", Value: oid}, {Key: "userId", Value: uid}}, nil
}

// CreateRoutine inserts a new routine of the user. r.ID must be a valid
// ObjectId; r.UserID, if set, must be userID. If the user already has a
// routine with this name in the module (ignoring case), the error is
// core.ErrRoutineNameTaken.
func (s *Store) CreateRoutine(ctx context.Context, userID string, r core.Routine) error {
	uid, err := ownerID(userID)
	if err != nil {
		return err
	}
	if r.UserID != "" && r.UserID != userID {
		return fmt.Errorf("routine %s: owner %s does not match %s", r.ID, r.UserID, userID)
	}
	doc, err := toRoutineDoc(r)
	if err != nil {
		return err
	}
	doc.UserID = uid
	if _, err := s.routines.InsertOne(ctx, doc); err != nil {
		if mongo.IsDuplicateKeyError(err) {
			return core.ErrRoutineNameTaken
		}
		return fmt.Errorf("create routine: %w", err)
	}
	return nil
}

// ListRoutines returns all routines of the user in the module, sorted by
// createdAt, then ID (API.md 3.17).
func (s *Store) ListRoutines(ctx context.Context, userID, module string) ([]core.Routine, error) {
	uid, err := ownerID(userID)
	if err != nil {
		return nil, err
	}
	cur, err := s.routines.Find(ctx,
		bson.D{{Key: "userId", Value: uid}, {Key: "module", Value: module}},
		options.Find().SetSort(bson.D{{Key: "createdAt", Value: 1}, {Key: "_id", Value: 1}}))
	if err != nil {
		return nil, fmt.Errorf("read routines: %w", err)
	}
	var docs []routineDoc
	if err := cur.All(ctx, &docs); err != nil {
		return nil, fmt.Errorf("read routines: %w", err)
	}
	list := make([]core.Routine, 0, len(docs))
	for _, d := range docs {
		r, err := d.toCore()
		if err != nil {
			return nil, err
		}
		list = append(list, r)
	}
	return list, nil
}

// GetRoutine returns the user's routine. found is false if there is none,
// also if it belongs to someone else.
func (s *Store) GetRoutine(ctx context.Context, userID, id string) (r core.Routine, found bool, err error) {
	uid, err := ownerID(userID)
	if err != nil {
		return core.Routine{}, false, err
	}
	filter, err := routineFilter(uid, id)
	if err != nil {
		return core.Routine{}, false, err
	}
	var doc routineDoc
	err = s.routines.FindOne(ctx, filter).Decode(&doc)
	if errors.Is(err, mongo.ErrNoDocuments) {
		return core.Routine{}, false, nil
	}
	if err != nil {
		return core.Routine{}, false, fmt.Errorf("read routine: %w", err)
	}
	r, err = doc.toCore()
	if err != nil {
		return core.Routine{}, false, err
	}
	return r, true, nil
}

// CountRoutines returns how many routines the user has, over all modules
// (limit of 50, API.md 3.16).
func (s *Store) CountRoutines(ctx context.Context, userID string) (int64, error) {
	uid, err := ownerID(userID)
	if err != nil {
		return 0, err
	}
	n, err := s.routines.CountDocuments(ctx, bson.D{{Key: "userId", Value: uid}})
	if err != nil {
		return 0, fmt.Errorf("count routines: %w", err)
	}
	return n, nil
}

// RoutineIDByName returns the ID of the user's routine with this name key
// in the module, for details.routineId of ROUTINE_NAME_TAKEN.
func (s *Store) RoutineIDByName(ctx context.Context, userID, module, nameKey string) (id string, found bool, err error) {
	uid, err := ownerID(userID)
	if err != nil {
		return "", false, err
	}
	var hit struct {
		ID bson.ObjectID `bson:"_id"`
	}
	err = s.routines.FindOne(ctx,
		bson.D{{Key: "userId", Value: uid}, {Key: "module", Value: module}, {Key: "nameKey", Value: nameKey}},
		options.FindOne().SetProjection(bson.D{{Key: "_id", Value: 1}}),
	).Decode(&hit)
	if errors.Is(err, mongo.ErrNoDocuments) {
		return "", false, nil
	}
	if err != nil {
		return "", false, fmt.Errorf("find routine by name: %w", err)
	}
	return hit.ID.Hex(), true, nil
}

// UpdateRoutine stores the fields of r named in fields (API names as
// core.ApplyRoutinePatch returns them: name, amount, unit, schedule,
// startDate, endDate) with one $set; a new name also sets nameKey. Only
// these fields are written, so two simultaneous changes of different
// fields do not undo each other. found is false if the routine does not
// exist (anymore) or is not the user's. A taken name is
// core.ErrRoutineNameTaken.
func (s *Store) UpdateRoutine(ctx context.Context, userID string, r core.Routine, fields []string) (found bool, err error) {
	uid, err := ownerID(userID)
	if err != nil {
		return false, err
	}
	filter, err := routineFilter(uid, r.ID)
	if err != nil {
		return false, err
	}
	doc, err := toRoutineDoc(r)
	if err != nil {
		return false, err
	}
	var set bson.D
	for _, f := range fields {
		switch f {
		case "name":
			set = append(set, bson.E{Key: "name", Value: doc.Name}, bson.E{Key: "nameKey", Value: doc.NameKey})
		case "amount":
			set = append(set, bson.E{Key: "amount", Value: doc.Amount})
		case "unit":
			set = append(set, bson.E{Key: "unit", Value: doc.Unit})
		case "schedule":
			set = append(set, bson.E{Key: "schedule", Value: doc.Schedule})
		case "startDate":
			set = append(set, bson.E{Key: "startDate", Value: doc.StartDate})
		case "endDate":
			set = append(set, bson.E{Key: "endDate", Value: doc.EndDate})
		default:
			return false, fmt.Errorf("update routine: field %q cannot be stored", f)
		}
	}
	if len(set) == 0 {
		return false, errors.New("update routine: no field given")
	}
	res, err := s.routines.UpdateOne(ctx, filter, bson.D{{Key: "$set", Value: set}})
	if err != nil {
		if mongo.IsDuplicateKeyError(err) {
			return false, core.ErrRoutineNameTaken
		}
		return false, fmt.Errorf("update routine: %w", err)
	}
	return res.MatchedCount > 0, nil
}

// SetPauses replaces the pauses of the user's routine, but only if they
// are still exactly old (compare and set). updated is false if the routine
// changed meanwhile, or is gone; the caller reads it again and retries.
func (s *Store) SetPauses(ctx context.Context, userID, id string, old, pauses []core.Pause) (updated bool, err error) {
	uid, err := ownerID(userID)
	if err != nil {
		return false, err
	}
	filter, err := routineFilter(uid, id)
	if err != nil {
		return false, err
	}
	filter = append(filter, bson.E{Key: "pauses", Value: toPauseDocs(old)})
	res, err := s.routines.UpdateOne(ctx, filter,
		bson.D{{Key: "$set", Value: bson.D{{Key: "pauses", Value: toPauseDocs(pauses)}}}})
	if err != nil {
		return false, fmt.Errorf("set pauses: %w", err)
	}
	return res.MatchedCount > 0, nil
}

// DeleteRoutine deletes only the routine document of the user. The caller
// deletes its logs before (and after) it (API.md 3.23).
func (s *Store) DeleteRoutine(ctx context.Context, userID, id string) (found bool, err error) {
	uid, err := ownerID(userID)
	if err != nil {
		return false, err
	}
	filter, err := routineFilter(uid, id)
	if err != nil {
		return false, err
	}
	res, err := s.routines.DeleteOne(ctx, filter)
	if err != nil {
		return false, fmt.Errorf("delete routine: %w", err)
	}
	return res.DeletedCount > 0, nil
}

// CreateRoutineLog inserts a log of the user. l.ID and l.RoutineID must be
// valid ObjectIds. If the occurrence (dueLocal) already has a log, the
// error is core.ErrRoutineLogExists.
func (s *Store) CreateRoutineLog(ctx context.Context, userID string, l core.RoutineLog) error {
	uid, err := ownerID(userID)
	if err != nil {
		return err
	}
	if l.UserID != "" && l.UserID != userID {
		return fmt.Errorf("log %s: owner %s does not match %s", l.ID, l.UserID, userID)
	}
	doc, err := toRoutineLogDoc(l)
	if err != nil {
		return err
	}
	doc.UserID = uid
	if _, err := s.routineLogs.InsertOne(ctx, doc); err != nil {
		if mongo.IsDuplicateKeyError(err) {
			return core.ErrRoutineLogExists
		}
		return fmt.Errorf("create routine log: %w", err)
	}
	return nil
}

// RoutineLogIDByDue returns the ID of the log of the user's routine for the
// occurrence dueLocal, for details.logId of ROUTINE_LOG_EXISTS.
func (s *Store) RoutineLogIDByDue(ctx context.Context, userID, routineID, dueLocal string) (id string, found bool, err error) {
	uid, err := ownerID(userID)
	if err != nil {
		return "", false, err
	}
	rid, err := bson.ObjectIDFromHex(routineID)
	if err != nil {
		return "", false, core.ErrInvalidID
	}
	var hit struct {
		ID bson.ObjectID `bson:"_id"`
	}
	err = s.routineLogs.FindOne(ctx,
		bson.D{{Key: "userId", Value: uid}, {Key: "routineId", Value: rid}, {Key: "dueLocal", Value: dueLocal}},
		options.FindOne().SetProjection(bson.D{{Key: "_id", Value: 1}}),
	).Decode(&hit)
	if errors.Is(err, mongo.ErrNoDocuments) {
		return "", false, nil
	}
	if err != nil {
		return "", false, fmt.Errorf("find routine log: %w", err)
	}
	return hit.ID.Hex(), true, nil
}

// CountRoutineLogsTaken counts the logs of the user's routine with takenAt
// in [from, to) (limit per day, API.md 3.26).
func (s *Store) CountRoutineLogsTaken(ctx context.Context, userID, routineID string, from, to time.Time) (int64, error) {
	uid, err := ownerID(userID)
	if err != nil {
		return 0, err
	}
	rid, err := bson.ObjectIDFromHex(routineID)
	if err != nil {
		return 0, core.ErrInvalidID
	}
	n, err := s.routineLogs.CountDocuments(ctx, bson.D{
		{Key: "userId", Value: uid}, {Key: "routineId", Value: rid},
		{Key: "takenAt", Value: bson.D{{Key: "$gte", Value: from.UTC()}, {Key: "$lt", Value: to.UTC()}}},
	})
	if err != nil {
		return 0, fmt.Errorf("count routine logs: %w", err)
	}
	return n, nil
}

// RoutineLogs returns the logs of the user's routine with takenAt in
// [from, to), sorted by takenAt descending, then ID descending (API.md
// 3.25).
func (s *Store) RoutineLogs(ctx context.Context, userID, routineID string, from, to time.Time) ([]core.RoutineLog, error) {
	uid, err := ownerID(userID)
	if err != nil {
		return nil, err
	}
	rid, err := bson.ObjectIDFromHex(routineID)
	if err != nil {
		return nil, core.ErrInvalidID
	}
	return s.findRoutineLogs(ctx, bson.D{
		{Key: "userId", Value: uid}, {Key: "routineId", Value: rid},
		{Key: "takenAt", Value: bson.D{{Key: "$gte", Value: from.UTC()}, {Key: "$lt", Value: to.UTC()}}},
	}, options.Find().SetSort(bson.D{{Key: "takenAt", Value: -1}, {Key: "_id", Value: -1}}))
}

// RoutineLogsInRange returns the logs of the user's routines that may count
// for the local dates from..to: those whose occurrence (the date in
// dueLocal) lies in it, and those whose takenAt lies in [takenFrom,
// takenTo) (progress and occurrences, core.ProgressSpan). takenFrom equal
// to takenTo leaves out the second kind. Unsorted.
func (s *Store) RoutineLogsInRange(ctx context.Context, userID string, routineIDs []string, from, to core.Date, takenFrom, takenTo time.Time) ([]core.RoutineLog, error) {
	uid, err := ownerID(userID)
	if err != nil {
		return nil, err
	}
	if len(routineIDs) == 0 {
		return []core.RoutineLog{}, nil
	}
	ids := make([]bson.ObjectID, 0, len(routineIDs))
	for _, id := range routineIDs {
		oid, err := bson.ObjectIDFromHex(id)
		if err != nil {
			return nil, core.ErrInvalidID
		}
		ids = append(ids, oid)
	}
	// "YYYY-MM-DDTHH:MM" sorts like the point in time, so a range of dates
	// is a range of strings: from "2026-11-23" up to, not including, the
	// day after to.
	return s.findRoutineLogs(ctx, bson.D{
		{Key: "userId", Value: uid},
		{Key: "routineId", Value: bson.D{{Key: "$in", Value: ids}}},
		{Key: "$or", Value: bson.A{
			bson.D{{Key: "dueLocal", Value: bson.D{{Key: "$gte", Value: from.String()}, {Key: "$lt", Value: (to + 1).String()}}}},
			bson.D{{Key: "takenAt", Value: bson.D{{Key: "$gte", Value: takenFrom.UTC()}, {Key: "$lt", Value: takenTo.UTC()}}}},
		}},
	}, options.Find())
}

func (s *Store) findRoutineLogs(ctx context.Context, filter bson.D, opts *options.FindOptionsBuilder) ([]core.RoutineLog, error) {
	cur, err := s.routineLogs.Find(ctx, filter, opts)
	if err != nil {
		return nil, fmt.Errorf("read routine logs: %w", err)
	}
	var docs []routineLogDoc
	if err := cur.All(ctx, &docs); err != nil {
		return nil, fmt.Errorf("read routine logs: %w", err)
	}
	list := make([]core.RoutineLog, 0, len(docs))
	for _, d := range docs {
		list = append(list, d.toCore())
	}
	return list, nil
}

// DeleteRoutineLog deletes one log of the user's routine. found is false
// if there is no such log in this routine (API.md 3.27).
func (s *Store) DeleteRoutineLog(ctx context.Context, userID, routineID, logID string) (found bool, err error) {
	uid, err := ownerID(userID)
	if err != nil {
		return false, err
	}
	rid, err := bson.ObjectIDFromHex(routineID)
	if err != nil {
		return false, core.ErrInvalidID
	}
	lid, err := bson.ObjectIDFromHex(logID)
	if err != nil {
		return false, core.ErrInvalidID
	}
	res, err := s.routineLogs.DeleteOne(ctx,
		bson.D{{Key: "_id", Value: lid}, {Key: "userId", Value: uid}, {Key: "routineId", Value: rid}})
	if err != nil {
		return false, fmt.Errorf("delete routine log: %w", err)
	}
	return res.DeletedCount > 0, nil
}

// DeleteRoutineLogs deletes all logs of the user's routine and returns how
// many there were. None is not an error.
func (s *Store) DeleteRoutineLogs(ctx context.Context, userID, routineID string) (int64, error) {
	uid, err := ownerID(userID)
	if err != nil {
		return 0, err
	}
	rid, err := bson.ObjectIDFromHex(routineID)
	if err != nil {
		return 0, core.ErrInvalidID
	}
	res, err := s.routineLogs.DeleteMany(ctx, bson.D{{Key: "userId", Value: uid}, {Key: "routineId", Value: rid}})
	if err != nil {
		return 0, fmt.Errorf("delete routine logs: %w", err)
	}
	return res.DeletedCount, nil
}
