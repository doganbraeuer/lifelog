package api

import (
	"context"
	"io"
	"net/http"

	"lifelog/backend/internal/core"
)

// Error codes for the entry endpoints (API.md, section 1).
const (
	CodeInvalidParameter = "INVALID_PARAMETER"
)

// EntryStore is what the entry handlers need from the database. Implemented
// by store.Store, in tests by a fake.
type EntryStore interface {
	ListSummaries(ctx context.Context, module string) ([]core.EntrySummary, error)
	GetEntry(ctx context.Context, id string) (e core.Entry, found bool, err error)
	OpenFile(ctx context.Context, fileID string) (content io.ReadCloser, length int64, err error)
}

// listEntriesHandler answers GET /api/v1/entries (API.md, 3.2).
func listEntriesHandler(s EntryStore) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		q := r.URL.Query()

		module, err := core.CheckModule(q.Get("module"))
		if err != nil {
			writeInvalidParameter(w, "module", q.Get("module"),
				"Unbekanntes Modul. Im Prototyp gibt es nur health.")
			return
		}
		sort, err := core.ParseSort(q.Get("sort"))
		if err != nil {
			writeInvalidParameter(w, "sort", q.Get("sort"),
				"Unbekannte Sortierung. Erlaubt sind documentDate und importedAt.")
			return
		}
		order, err := core.ParseSortOrder(q.Get("order"))
		if err != nil {
			writeInvalidParameter(w, "order", q.Get("order"),
				"Unbekannte Richtung. Erlaubt sind desc und asc.")
			return
		}

		list, err := s.ListSummaries(r.Context(), module)
		if err != nil {
			writeInternalError(w, r, err)
			return
		}
		core.SortEntries(list, sort, order)

		writeJSON(w, http.StatusOK, entryListToJSON(list))
	}
}

func writeInvalidParameter(w http.ResponseWriter, parameter, value, message string) {
	writeError(w, http.StatusBadRequest, CodeInvalidParameter, message,
		map[string]any{"parameter": parameter, "value": value})
}
