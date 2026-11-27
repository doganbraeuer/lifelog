package api

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"

	"lifelog/backend/internal/core"
)

// Routines and logs on the wire (docs/API.md 3.16–3.27). Lists are always
// an array, never null; fields that may be null are visible as null.

// Error codes for routines (API.md, section 1).
const (
	CodeRoutineNotFound      = "ROUTINE_NOT_FOUND"
	CodeRoutineLogNotFound   = "ROUTINE_LOG_NOT_FOUND"
	CodeRoutineNameTaken     = "ROUTINE_NAME_TAKEN"
	CodeRoutineLogExists     = "ROUTINE_LOG_EXISTS"
	CodeRoutineLimitReached  = "ROUTINE_LIMIT_REACHED"
	messageRoutineNotFound   = "Diese Routine gibt es nicht."
	messageInvalidRoutineID  = "Diese Routinen-ID ist ungültig."
	messageInvalidLogID      = "Diese Verlaufseintrag-ID ist ungültig."
	messageRoutineBodyBroken = "Die Anfrage ist zu groß oder unvollständig."
)

// maxRoutineBody: a routine with 24 times and a long name has well under
// 2 KB; 16 KB limits what the server reads before any check.
const maxRoutineBody = 16 << 10

type routineJSON struct {
	ID        string       `json:"id"`
	Module    string       `json:"module"`
	Name      string       `json:"name"`
	Amount    *float64     `json:"amount"`
	Unit      *string      `json:"unit"`
	Schedule  scheduleJSON `json:"schedule"`
	StartDate string       `json:"startDate"`
	EndDate   *string      `json:"endDate"`
	Status    string       `json:"status"`
	Pauses    []pauseJSON  `json:"pauses"`
	CreatedAt string       `json:"createdAt"`
	Progress  progressJSON `json:"progress"`
}

// scheduleJSON has only the fields of its frequency (API.md 3.16
// "Schedule"); the others are left out.
type scheduleJSON struct {
	Frequency   string   `json:"frequency"`
	Interval    int      `json:"interval"`
	Weekdays    []string `json:"weekdays,omitempty"`
	Times       []string `json:"times,omitempty"`
	WindowStart string   `json:"windowStart,omitempty"`
	WindowEnd   string   `json:"windowEnd,omitempty"`
}

type pauseJSON struct {
	From  string  `json:"from"`
	Until *string `json:"until"`
}

type progressJSON struct {
	Today  rangeJSON   `json:"today"`
	Week   rangeJSON   `json:"week"`
	Period *periodJSON `json:"period"`
}

type rangeJSON struct {
	Done         int      `json:"done"`
	Skipped      int      `json:"skipped"`
	Due          int      `json:"due"`
	AmountDone   *float64 `json:"amountDone"`
	AmountTarget *float64 `json:"amountTarget"`
}

type periodJSON struct {
	Day          int      `json:"day"`
	Days         int      `json:"days"`
	Done         int      `json:"done"`
	Skipped      int      `json:"skipped"`
	Due          int      `json:"due"`
	AmountDone   *float64 `json:"amountDone"`
	AmountTarget *float64 `json:"amountTarget"`
}

type routineListJSON struct {
	Routines []routineJSON `json:"routines"`
	Count    int           `json:"count"`
}

type occurrenceJSON struct {
	RoutineID string  `json:"routineId"`
	DueAt     string  `json:"dueAt"`
	Date      string  `json:"date"`
	Time      string  `json:"time"`
	Status    string  `json:"status"`
	LogID     *string `json:"logId"`
}

type occurrenceListJSON struct {
	From        string           `json:"from"`
	To          string           `json:"to"`
	Occurrences []occurrenceJSON `json:"occurrences"`
	Count       int              `json:"count"`
}

type routineLogJSON struct {
	ID        string   `json:"id"`
	RoutineID string   `json:"routineId"`
	DueAt     *string  `json:"dueAt"`
	TakenAt   string   `json:"takenAt"`
	Status    string   `json:"status"`
	Amount    *float64 `json:"amount"`
	Source    string   `json:"source"`
	EntryID   *string  `json:"entryId"`
	TimeZone  string   `json:"timeZone"`
	CreatedAt string   `json:"createdAt"`
}

type routineLogListJSON struct {
	Logs  []routineLogJSON `json:"logs"`
	Count int              `json:"count"`
}

func routineToJSON(v core.RoutineView) routineJSON {
	pauses := make([]pauseJSON, 0, len(v.Pauses))
	for _, p := range v.Pauses {
		pauses = append(pauses, pauseJSON{From: p.From.String(), Until: dateJSON(p.Until)})
	}
	return routineJSON{
		ID:        v.ID,
		Module:    v.Module,
		Name:      v.Name,
		Amount:    v.Amount,
		Unit:      v.Unit,
		Schedule:  scheduleToJSON(v.Schedule),
		StartDate: v.StartDate.String(),
		EndDate:   dateJSON(v.EndDate),
		Status:    string(v.Status),
		Pauses:    pauses,
		CreatedAt: timeJSON(v.CreatedAt),
		Progress:  progressToJSON(v.Progress),
	}
}

func scheduleToJSON(s core.Schedule) scheduleJSON {
	out := scheduleJSON{Frequency: string(s.Frequency), Interval: s.Interval}
	for _, d := range s.Weekdays {
		out.Weekdays = append(out.Weekdays, core.WeekdayCode(d))
	}
	for _, t := range s.Times {
		out.Times = append(out.Times, t.String())
	}
	if s.Frequency == core.FrequencyHourly {
		out.WindowStart, out.WindowEnd = s.WindowStart.String(), s.WindowEnd.String()
	}
	return out
}

func progressToJSON(p core.Progress) progressJSON {
	out := progressJSON{Today: rangeToJSON(p.Today), Week: rangeToJSON(p.Week)}
	if p.Period != nil {
		out.Period = &periodJSON{Day: p.Period.Day, Days: p.Period.Days, Done: p.Period.Done, Skipped: p.Period.Skipped,
			Due: p.Period.Due, AmountDone: p.Period.AmountDone, AmountTarget: p.Period.AmountTarget}
	}
	return out
}

func rangeToJSON(r core.ProgressRange) rangeJSON {
	return rangeJSON{Done: r.Done, Skipped: r.Skipped, Due: r.Due, AmountDone: r.AmountDone, AmountTarget: r.AmountTarget}
}

func occurrenceToJSON(o core.Occurrence) occurrenceJSON {
	out := occurrenceJSON{RoutineID: o.RoutineID, DueAt: timeJSON(o.DueAt), Date: o.Date.String(),
		Time: o.Time.String(), Status: string(o.Status)}
	if o.LogID != "" {
		id := o.LogID
		out.LogID = &id
	}
	return out
}

func routineLogToJSON(l core.RoutineLog) routineLogJSON {
	out := routineLogJSON{ID: l.ID, RoutineID: l.RoutineID, TakenAt: timeJSON(l.TakenAt), Status: string(l.Status),
		Amount: l.Amount, Source: string(l.Source), EntryID: l.EntryID, TimeZone: l.TimeZone, CreatedAt: timeJSON(l.CreatedAt)}
	if l.DueAt != nil {
		due := timeJSON(*l.DueAt)
		out.DueAt = &due
	}
	return out
}

func dateJSON(d *core.Date) *string {
	if d == nil {
		return nil
	}
	s := d.String()
	return &s
}

// readRoutineBody reads a JSON body for core: decoded into a map as
// encoding/json does (numbers float64), nil if it is not a JSON object;
// core then answers with its message. emptyIsObject: an empty body counts
// as {} (pause, API.md 3.21). A body over 16 KB or a broken connection is
// answered here.
func readRoutineBody(w http.ResponseWriter, r *http.Request, emptyIsObject bool) (map[string]any, bool) {
	raw, err := io.ReadAll(http.MaxBytesReader(w, r.Body, maxRoutineBody))
	if err != nil {
		writeError(w, http.StatusBadRequest, CodeInvalidRequest, messageRoutineBodyBroken, nil)
		return nil, false
	}
	if emptyIsObject && len(bytes.TrimSpace(raw)) == 0 {
		return map[string]any{}, true
	}
	var v any
	if err := json.Unmarshal(raw, &v); err != nil {
		return nil, true
	}
	m, _ := v.(map[string]any)
	return m, true
}

// checkRoutineID answers 400 INVALID_ID for a routine ID that is no
// ObjectId, before anything else is read (as for entries).
func checkRoutineID(w http.ResponseWriter, id string) bool {
	if core.CheckID(id) != nil {
		writeError(w, http.StatusBadRequest, CodeInvalidID, messageInvalidRoutineID, nil)
		return false
	}
	return true
}

// writeRoutineError answers the errors of service.Routines. The rejected
// value goes back to the app in details.value, never into the log.
func writeRoutineError(w http.ResponseWriter, r *http.Request, err error) {
	var invalid *core.RoutineError
	var param *core.ParameterError
	var taken *core.RoutineNameTakenError
	var exists *core.RoutineLogExistsError
	var limit *core.RoutineLimitError
	switch {
	case errors.As(err, &invalid):
		details := map[string]any{}
		if invalid.Field != "" {
			details["field"] = invalid.Field
		}
		if invalid.Value != nil {
			details["value"] = invalid.Value
		}
		if len(details) == 0 {
			details = nil
		}
		writeError(w, http.StatusBadRequest, CodeInvalidRequest, invalid.Message, details)
	case errors.As(err, &param):
		writeInvalidParameter(w, param.Parameter, param.Value, param.Message)
	case errors.Is(err, core.ErrInvalidID):
		writeError(w, http.StatusBadRequest, CodeInvalidID, messageInvalidRoutineID, nil)
	case errors.Is(err, core.ErrRoutineNotFound):
		writeError(w, http.StatusNotFound, CodeRoutineNotFound, messageRoutineNotFound, nil)
	case errors.Is(err, core.ErrRoutineLogNotFound):
		writeError(w, http.StatusNotFound, CodeRoutineLogNotFound, "Diesen Verlaufseintrag gibt es nicht.", nil)
	case errors.As(err, &taken):
		var details map[string]any
		if taken.RoutineID != "" {
			details = map[string]any{"routineId": taken.RoutineID}
		}
		writeError(w, http.StatusConflict, CodeRoutineNameTaken, "Eine Routine mit diesem Namen gibt es schon.", details)
	case errors.As(err, &exists):
		var details map[string]any
		if exists.LogID != "" {
			details = map[string]any{"logId": exists.LogID}
		}
		writeError(w, http.StatusConflict, CodeRoutineLogExists, "Dieser Termin ist schon eingetragen.", details)
	case errors.As(err, &limit):
		writeError(w, http.StatusConflict, CodeRoutineLimitReached, routineLimitMessage(limit.Limit),
			map[string]any{"limit": string(limit.Limit), "max": limit.Max})
	case errors.Is(err, context.Canceled):
		// The app gave up; nobody reads the response.
	default:
		writeInternalError(w, r, err)
	}
}

func routineLimitMessage(l core.RoutineLimit) string {
	switch l {
	case core.RoutineLimitRoutines:
		return "Du hast schon 50 Routinen. Lösche eine, bevor du eine neue anlegst."
	case core.RoutineLimitPauses:
		return "Diese Routine wurde schon zu oft pausiert."
	case core.RoutineLimitLogs:
		return "Für heute sind bei dieser Routine keine weiteren Einträge möglich."
	}
	return "Das Limit für Routinen ist erreicht."
}
