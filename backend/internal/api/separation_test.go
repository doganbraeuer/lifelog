package api

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"slices"
	"strings"
	"sync"
	"testing"
	"time"

	"go.mongodb.org/mongo-driver/v2/bson"

	"lifelog/backend/internal/core"
	"lifelog/backend/internal/llm/fake"
	"lifelog/backend/internal/service"
	speechfake "lifelog/backend/internal/speech/fake"
)

// Separation tests (docs/API.md, per-user data): two users, the real
// services, and an in-memory store that filters by user like store.Store
// and records the user of every call. User A is tester01 (role user), user
// B is an admin: admins have no additional rights.

const tokenB = "dGVzdC10b2tlbi1mb3ItdXNlci1iLTAwMDAwMDAwMDA"

// memData stands in for store.Store: everything is kept per owner, and
// every method only sees the user's own documents.
type memData struct {
	mu      sync.Mutex
	entries map[string]core.Entry // entry ID → entry (UserID = owner)
	files   map[string]memFile    // file ID → file
	chunks  []core.Chunk
	owners  map[string]string // entry ID → owner of its chunks
	calls   []memCall
}

type memFile struct {
	owner string
	data  []byte
}

type memCall struct{ method, userID string }

func newMemData() *memData {
	return &memData{entries: map[string]core.Entry{}, files: map[string]memFile{}, owners: map[string]string{}}
}

func (m *memData) record(method, userID string) {
	m.calls = append(m.calls, memCall{method, userID})
}

// usersOfCalls returns the user IDs of all calls since mark.
func (m *memData) usersOfCalls(mark int) []string {
	m.mu.Lock()
	defer m.mu.Unlock()
	var users []string
	for _, c := range m.calls[mark:] {
		users = append(users, c.userID)
	}
	return users
}

func (m *memData) mark() int {
	m.mu.Lock()
	defer m.mu.Unlock()
	return len(m.calls)
}

func (m *memData) ListSummaries(_ context.Context, userID, module string) ([]core.EntrySummary, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.record("ListSummaries", userID)
	list := []core.EntrySummary{}
	for _, e := range m.entries {
		if e.UserID == userID && e.Module == module {
			list = append(list, core.EntrySummary{ID: e.ID, Module: e.Module, Type: e.Type,
				DocumentDate: e.DocumentDate, ImportedAt: e.ImportedAt, Issuer: e.Issuer})
		}
	}
	return list, nil
}

func (m *memData) GetEntry(_ context.Context, userID, id string) (core.Entry, bool, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.record("GetEntry", userID)
	e, ok := m.entries[id]
	if !ok || e.UserID != userID {
		return core.Entry{}, false, nil
	}
	return e, true, nil
}

func (m *memData) OpenFile(_ context.Context, userID, fileID string) (io.ReadCloser, int64, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.record("OpenFile", userID)
	f, ok := m.files[fileID]
	if !ok || f.owner != userID {
		return nil, 0, core.ErrOriginalMissing
	}
	return io.NopCloser(bytes.NewReader(f.data)), int64(len(f.data)), nil
}

func (m *memData) EntryByHash(_ context.Context, userID, sha string) (string, bool, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.record("EntryByHash", userID)
	for _, e := range m.entries {
		if e.UserID == userID && e.File.Sha256 == sha {
			return e.ID, true, nil
		}
	}
	return "", false, nil
}

func (m *memData) SaveFile(_ context.Context, userID, id, _, _ string, data []byte) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.record("SaveFile", userID)
	m.files[id] = memFile{owner: userID, data: data}
	return nil
}

func (m *memData) DeleteFile(_ context.Context, userID, id string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.record("DeleteFile", userID)
	if f, ok := m.files[id]; ok && f.owner == userID {
		delete(m.files, id)
	}
	return nil
}

func (m *memData) SaveChunks(_ context.Context, userID string, chunks []core.Chunk) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.record("SaveChunks", userID)
	for _, c := range chunks {
		m.chunks = append(m.chunks, c)
		m.owners[c.EntryID] = userID
	}
	return nil
}

func (m *memData) DeleteChunks(_ context.Context, userID, entryID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.record("DeleteChunks", userID)
	if m.owners[entryID] == userID {
		m.chunks = slices.DeleteFunc(m.chunks, func(c core.Chunk) bool { return c.EntryID == entryID })
	}
	return nil
}

func (m *memData) SaveOCR(_ context.Context, userID, _ string, _ core.OCRResult) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.record("SaveOCR", userID)
	return nil
}

func (m *memData) GetOCR(_ context.Context, userID, _ string) (core.OCRResult, bool, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.record("GetOCR", userID)
	return core.OCRResult{}, false, nil
}

func (m *memData) DeleteOCR(_ context.Context, userID, _ string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.record("DeleteOCR", userID)
	return nil
}

func (m *memData) SaveEntry(_ context.Context, userID string, e core.Entry) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.record("SaveEntry", userID)
	for _, other := range m.entries {
		if other.UserID == userID && other.File.Sha256 == e.File.Sha256 {
			return core.ErrDuplicate
		}
	}
	e.UserID = userID
	m.entries[e.ID] = e
	return nil
}

func (m *memData) SaveCorrection(_ context.Context, userID string, e core.Entry) (bool, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.record("SaveCorrection", userID)
	old, ok := m.entries[e.ID]
	if !ok || old.UserID != userID {
		return false, nil
	}
	e.UserID = userID
	m.entries[e.ID] = e
	return true, nil
}

func (m *memData) DeleteEntry(_ context.Context, userID, id string) (bool, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.record("DeleteEntry", userID)
	e, ok := m.entries[id]
	if !ok || e.UserID != userID {
		return false, nil
	}
	delete(m.entries, id)
	return true, nil
}

// SimilarChunks filters by user and module, as the $vectorSearch filter
// does; every chunk of the user scores 0.9.
func (m *memData) SimilarChunks(_ context.Context, userID string, _ []float32, module string, k int) ([]core.Hit, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.record("SimilarChunks", userID)
	var hits []core.Hit
	for _, c := range m.chunks {
		if m.owners[c.EntryID] == userID && c.Module == module && len(hits) < k {
			hits = append(hits, core.Hit{EntryID: c.EntryID, Position: c.Position, Text: c.Text, Score: 0.9})
		}
	}
	return hits, nil
}

func (m *memData) SummariesByIDs(_ context.Context, userID string, ids []string) (map[string]core.EntrySummary, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.record("SummariesByIDs", userID)
	out := map[string]core.EntrySummary{}
	for _, id := range ids {
		if e, ok := m.entries[id]; ok && e.UserID == userID {
			out[id] = core.EntrySummary{ID: e.ID, Module: e.Module, Type: e.Type, DocumentDate: e.DocumentDate, Issuer: e.Issuer}
		}
	}
	return out, nil
}

// recordingUsage allows everything and records who was counted.
type recordingUsage struct {
	mu    sync.Mutex
	users []string
}

func (u *recordingUsage) Use(_ context.Context, userID string, _ core.LimitKind) error {
	u.mu.Lock()
	defer u.mu.Unlock()
	u.users = append(u.users, userID)
	return nil
}

func (u *recordingUsage) since(mark int) []string {
	u.mu.Lock()
	defer u.mu.Unlock()
	return slices.Clone(u.users[mark:])
}

func (u *recordingUsage) mark() int {
	u.mu.Lock()
	defer u.mu.Unlock()
	return len(u.users)
}

// twoUsers: A = tester01 (testToken), B = admin (tokenB), the real services
// on one memData.
type twoUsers struct {
	data     *memData
	usage    *recordingUsage
	router   http.Handler
	userA    core.User
	userB    core.User
	answerer *fake.Answerer
}

func newTwoUsers(t *testing.T) *twoUsers {
	t.Helper()
	ta := newTestAccounts()
	b := ta.addUser("admin2", "Admin-Passwort-2026", false)
	b.Role = core.RoleAdmin
	ta.store.users[b.ID] = b
	ta.addSession(b.ID, tokenB, time.Now().Add(time.Hour))

	data := newMemData()
	usage := &recordingUsage{}
	reader := &fake.Reader{Extraction: core.Extraction{
		Readable: true, Type: "lab_report", DocumentDate: ptr("2026-11-12"), Issuer: ptr("Labor A"),
		FullText: "Laborbefund 12.11.2026: Hämoglobin 14,2 g/dl",
		Facts:    []core.ExtractedFact{{Name: "Hämoglobin", Value: "14,2", Unit: ptr("g/dl")}},
	}}
	answerer := &fake.Answerer{Text: "Im Laborbefund steht Hämoglobin 14,2 g/dl [1]."}
	newID := func() string { return bson.NewObjectID().Hex() }
	deps := Dependencies{
		Entries:     data,
		Uploader:    service.NewUploader(data, reader, &fake.Embedder{}, nil, usage, newID),
		Asker:       service.NewAsker(data, &fake.Embedder{}, answerer, usage, nil),
		Corrector:   service.NewCorrector(data),
		Deleter:     service.NewDeleter(data),
		Transcriber: &speechfake.Transcriber{Result: core.Transcript{Text: "Notiz", Language: "de"}},
		Usage:       usage,
	}
	return &twoUsers{data: data, usage: usage, router: ta.router(deps), userA: ta.user, userB: b, answerer: answerer}
}

func (s *twoUsers) do(r *http.Request, token string) *httptest.ResponseRecorder {
	r.Header.Set("Authorization", "Bearer "+token)
	rec := httptest.NewRecorder()
	s.router.ServeHTTP(rec, r)
	return rec
}

func jsonRequest(method, path, body string) *http.Request {
	var r *http.Request
	if body == "" {
		r = httptest.NewRequest(method, path, nil)
	} else {
		r = httptest.NewRequest(method, path, strings.NewReader(body))
		r.Header.Set("Content-Type", "application/json")
	}
	return r
}

const textOfA = "Laborbefund 12.11.2026: Hämoglobin 14,2 g/dl"

// uploadAsA stores a text entry of A and returns its ID.
func (s *twoUsers) uploadAsA(t *testing.T) string {
	t.Helper()
	rec := s.do(textForm(t, [2]string{"text", textOfA}), testToken)
	if rec.Code != http.StatusCreated {
		t.Fatalf("A's upload: %d %s", rec.Code, rec.Body)
	}
	var e struct{ ID string }
	if err := json.Unmarshal(rec.Body.Bytes(), &e); err != nil {
		t.Fatal(err)
	}
	return e.ID
}

func TestSeparation_TimelineShowsOnlyOwnEntries(t *testing.T) {
	s := newTwoUsers(t)
	id := s.uploadAsA(t)

	rec := s.do(jsonRequest(http.MethodGet, "/api/v1/entries", ""), tokenB)
	if rec.Code != http.StatusOK || strings.TrimSpace(rec.Body.String()) != `{"entries":[],"count":0}` {
		t.Errorf("B's timeline: %d %s, want empty", rec.Code, rec.Body)
	}
	recA := s.do(jsonRequest(http.MethodGet, "/api/v1/entries", ""), testToken)
	if !strings.Contains(recA.Body.String(), id) {
		t.Errorf("A's timeline misses A's entry: %s", recA.Body)
	}
}

// GET, file, PATCH and DELETE on A's entry as B: 404 each, and A's entry
// stays as it was.
func TestSeparation_OtherUsersEntryIsNotFound(t *testing.T) {
	s := newTwoUsers(t)
	id := s.uploadAsA(t)
	before := s.data.entries[id]

	for _, r := range []*http.Request{
		jsonRequest(http.MethodGet, "/api/v1/entries/"+id, ""),
		jsonRequest(http.MethodGet, "/api/v1/entries/"+id+"/file", ""),
		jsonRequest(http.MethodPatch, "/api/v1/entries/"+id, `{"issuer": "Von B geändert"}`),
		jsonRequest(http.MethodDelete, "/api/v1/entries/"+id, ""),
	} {
		t.Run(r.Method+" "+r.URL.Path, func(t *testing.T) {
			rec := s.do(r, tokenB)
			checkError(t, rec, http.StatusNotFound, CodeEntryNotFound)
			if strings.Contains(rec.Body.String(), "Hämoglobin") {
				t.Error("content of A's entry in the response")
			}
		})
	}

	after, ok := s.data.entries[id]
	if !ok {
		t.Fatal("A's entry was deleted by B")
	}
	if *after.Issuer != *before.Issuer || after.FullText != before.FullText || len(after.Corrections) != 0 {
		t.Errorf("A's entry changed: %+v", after)
	}
	if _, ok := s.data.files[before.File.ID]; !ok {
		t.Error("A's file was deleted by B")
	}
	// A still sees and opens it.
	if rec := s.do(jsonRequest(http.MethodGet, "/api/v1/entries/"+id+"/file", ""), testToken); rec.Code != http.StatusOK {
		t.Errorf("A's own file: %d", rec.Code)
	}
}

// B asks the question that A's document answers: the search runs with B's
// user ID, finds nothing of A, and no model is asked.
func TestSeparation_QuestionsOnlySearchOwnDocuments(t *testing.T) {
	s := newTwoUsers(t)
	s.uploadAsA(t)

	mark := s.data.mark()
	rec := s.do(jsonRequest(http.MethodPost, "/api/v1/questions", `{"question": "Wie hoch war mein Hämoglobin?"}`), tokenB)
	if rec.Code != http.StatusOK {
		t.Fatalf("status %d: %s", rec.Code, rec.Body)
	}
	want := `{"answer":"Dazu finde ich nichts in deinen Dokumenten.","found":false,"sources":[]}`
	if got := strings.TrimSpace(rec.Body.String()); got != want {
		t.Errorf("B's answer = %s, want %s", got, want)
	}
	users := s.data.usersOfCalls(mark)
	if len(users) == 0 {
		t.Fatal("no search call recorded")
	}
	for _, u := range users {
		if u != s.userB.ID {
			t.Errorf("search called with user %s, want B %s", u, s.userB.ID)
		}
	}
	if s.answerer.Calls() != 0 {
		t.Error("the language model was called for B")
	}

	// A gets the answer with A's source.
	recA := s.do(jsonRequest(http.MethodPost, "/api/v1/questions", `{"question": "Wie hoch war mein Hämoglobin?"}`), testToken)
	if !strings.Contains(recA.Body.String(), `"found":true`) {
		t.Errorf("A's answer: %s", recA.Body)
	}
}

// The same text by B: a new entry (201), not 409. By A again: 409.
func TestSeparation_DuplicateOnlyWithinOwnAccount(t *testing.T) {
	s := newTwoUsers(t)
	s.uploadAsA(t)

	recB := s.do(textForm(t, [2]string{"text", textOfA}), tokenB)
	if recB.Code != http.StatusCreated {
		t.Errorf("B uploads A's text: %d %s, want 201", recB.Code, recB.Body)
	}
	recA := s.do(textForm(t, [2]string{"text", textOfA}), testToken)
	checkError(t, recA, http.StatusConflict, CodeDuplicate)
	if strings.Contains(recB.Body.String(), "entryId") {
		t.Error("B's response mentions another entry")
	}
}

// Every route of the API is classified. Routes that touch entries are
// called as B and must pass B's user ID on to every store and usage call;
// routes that do not touch entries are listed explicitly. A new route that
// is in neither list makes this test fail, so nobody can add a route
// without thinking about the owner filter.
func TestSeparation_EveryRoutePassesTheUser(t *testing.T) {
	notTouchingEntries := map[string]bool{
		"GET /api/v1/status":       true,
		"POST /api/v1/auth/login":  true,
		"POST /api/v1/auth/logout": true,
		"GET /api/v1/me":           true,
		"PATCH /api/v1/me":         true,
		"POST /api/v1/me/password": true,
		"PUT /api/v1/me/consent":   true,
	}
	type entryRoute struct {
		request func(t *testing.T, entryID string) *http.Request
		status  int // expected status for B on A's entry or own data
	}
	touchingEntries := map[string]entryRoute{
		"GET /api/v1/entries": {func(*testing.T, string) *http.Request {
			return jsonRequest(http.MethodGet, "/api/v1/entries", "")
		}, http.StatusOK},
		"POST /api/v1/entries": {func(t *testing.T, _ string) *http.Request {
			return textForm(t, [2]string{"text", "Notiz von B"})
		}, http.StatusCreated},
		"GET /api/v1/entries/{id}": {func(_ *testing.T, id string) *http.Request {
			return jsonRequest(http.MethodGet, "/api/v1/entries/"+id, "")
		}, http.StatusNotFound},
		"PATCH /api/v1/entries/{id}": {func(_ *testing.T, id string) *http.Request {
			return jsonRequest(http.MethodPatch, "/api/v1/entries/"+id, `{"issuer": "B"}`)
		}, http.StatusNotFound},
		"DELETE /api/v1/entries/{id}": {func(_ *testing.T, id string) *http.Request {
			return jsonRequest(http.MethodDelete, "/api/v1/entries/"+id, "")
		}, http.StatusNotFound},
		"GET /api/v1/entries/{id}/file": {func(_ *testing.T, id string) *http.Request {
			return jsonRequest(http.MethodGet, "/api/v1/entries/"+id+"/file", "")
		}, http.StatusNotFound},
		"POST /api/v1/questions": {func(*testing.T, string) *http.Request {
			return jsonRequest(http.MethodPost, "/api/v1/questions", `{"question": "Hämoglobin?"}`)
		}, http.StatusOK},
		"POST /api/v1/transcriptions": {func(t *testing.T, _ string) *http.Request {
			return audioForm(t, "audio", m4a)
		}, http.StatusOK},
	}

	s := newTwoUsers(t)
	idOfA := s.uploadAsA(t)

	for _, rt := range routes(Dependencies{}) {
		t.Run(rt.pattern, func(t *testing.T) {
			if notTouchingEntries[rt.pattern] {
				return
			}
			er, ok := touchingEntries[rt.pattern]
			if !ok {
				t.Fatalf("route %s is not classified: add it to the routes touching entries "+
					"(with a check that it passes the user on) or to the routes that do not", rt.pattern)
			}
			markData, markUsage := s.data.mark(), s.usage.mark()
			rec := s.do(er.request(t, idOfA), tokenB)
			if rec.Code != er.status {
				t.Fatalf("status %d, want %d: %s", rec.Code, er.status, rec.Body)
			}
			users := append(s.data.usersOfCalls(markData), s.usage.since(markUsage)...)
			if len(users) == 0 {
				t.Fatal("no store or usage call recorded: the route must pass the user on")
			}
			for _, u := range users {
				if u != s.userB.ID {
					t.Errorf("called with user %q, want B %q", u, s.userB.ID)
				}
			}
		})
	}
}

// Nothing of a document in the log: no full text, no fact value, no
// question, no answer, no file name (docs/API.md, backend/README.md).
func TestSeparation_NoDocumentContentInTheLog(t *testing.T) {
	var buf bytes.Buffer
	old := slog.Default()
	slog.SetDefault(slog.New(slog.NewTextHandler(&buf, &slog.HandlerOptions{Level: slog.LevelDebug})))
	defer slog.SetDefault(old)

	s := newTwoUsers(t)
	id := s.uploadAsA(t)
	file := form(t, map[string][]byte{"Befund_Geheimname.pdf": []byte("%PDF-1.7\nsynthetisch")}, nil)
	if rec := s.do(file, testToken); rec.Code != http.StatusCreated {
		t.Fatalf("file upload: %d %s", rec.Code, rec.Body)
	}
	s.do(jsonRequest(http.MethodPost, "/api/v1/questions", `{"question": "Geheime Frage zum Hämoglobin?"}`), testToken)
	s.do(jsonRequest(http.MethodPatch, "/api/v1/entries/"+id, `{"issuer": "Geheimes Labor"}`), testToken)
	s.do(jsonRequest(http.MethodDelete, "/api/v1/entries/"+id, ""), testToken)

	log := buf.String()
	if !strings.Contains(log, "verification") || !strings.Contains(log, "question") {
		t.Fatalf("log looks empty, the test would prove nothing:\n%s", log)
	}
	for _, secret := range []string{
		"Laborbefund", "Hämoglobin", "14,2", "g/dl", "Labor A", "Geheime Frage", "Im Laborbefund steht",
		"Befund_Geheimname", ".pdf", "text.txt", "Geheimes Labor",
	} {
		if strings.Contains(log, secret) {
			t.Errorf("log contains %q:\n%s", secret, log)
		}
	}
}
