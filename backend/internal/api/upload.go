package api

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"mime/multipart"
	"net/http"
	"strings"

	"lifelog/backend/internal/core"
	"lifelog/backend/internal/llm"
	"lifelog/backend/internal/service"
)

// Error codes for the upload (API.md 3.1).
const (
	CodeInvalidRequest      = "INVALID_REQUEST"
	CodeFileMissing         = "FILE_MISSING"
	CodeDuplicate           = "DUPLICATE"
	CodeFileTooLarge        = "FILE_TOO_LARGE"
	CodeUnsupportedFileType = "UNSUPPORTED_FILE_TYPE"
	CodeUnreadable          = "UNREADABLE"
	CodeExtractionFailed    = "EXTRACTION_FAILED"
	CodeModelUnavailable    = "MODEL_UNAVAILABLE"
)

// formOverhead: room for the form overhead (boundaries, headers, field
// module) in addition to the file. If the file itself is over 10 MB,
// core.CheckFile reports that precisely; anything beyond breaks off while
// reading.
const formOverhead = 1 << 20 // 1 MiB

// Uploader runs the upload or text import. Implemented by service.Uploader.
type Uploader interface {
	Upload(ctx context.Context, u service.Upload) (core.Entry, error)
	UploadText(ctx context.Context, u service.TextUpload) (core.Entry, error)
}

// uploadKind: file upload or text import. Only decides the error messages.
type uploadKind int

const (
	kindFile uploadKind = iota
	kindText
)

// uploadHandler answers POST /api/v1/entries (API.md 3.1).
func uploadHandler(u Uploader) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		r.Body = http.MaxBytesReader(w, r.Body, core.MaxFileBytes+formOverhead)

		// maxMemory as large as the allowed request: the whole form stays in
		// memory. Otherwise Go stores large files as temporary files on disk,
		// and health documents should not lie around there.
		if err := r.ParseMultipartForm(core.MaxFileBytes + formOverhead); err != nil {
			var tooLarge *http.MaxBytesError
			switch {
			case errors.As(err, &tooLarge):
				writeError(w, http.StatusRequestEntityTooLarge, CodeFileTooLarge, "Die Datei ist größer als 10 MB.", nil)
			case errors.Is(err, http.ErrNotMultipart):
				writeError(w, http.StatusBadRequest, CodeInvalidRequest,
					"Bitte die Datei als multipart/form-data im Feld file schicken.", nil)
			default:
				writeError(w, http.StatusBadRequest, CodeInvalidRequest, "Das Formular ist nicht lesbar.", nil)
			}
			return
		}
		defer func() { _ = r.MultipartForm.RemoveAll() }()

		files := r.MultipartForm.File["file"]
		texts := r.MultipartForm.Value["text"]
		// Empty or only whitespace counts as missing (API.md 3.1).
		hasText := len(texts) > 0 && strings.TrimSpace(texts[0]) != ""
		module := firstValue(r.MultipartForm.Value["module"])
		switch {
		case len(texts) > 1:
			writeError(w, http.StatusBadRequest, CodeInvalidRequest, "Bitte genau einen Text schicken.", nil)
			return
		case len(files) > 0 && hasText:
			writeError(w, http.StatusBadRequest, CodeInvalidRequest,
				"Bitte entweder eine Datei oder einen Text schicken, nicht beides.", nil)
			return
		case hasText:
			// The text goes on unchanged, not trimmed.
			entry, err := u.UploadText(r.Context(), service.TextUpload{Text: texts[0], Module: module})
			writeUploadResult(w, r, entry, err, kindText, module)
			return
		case len(files) == 0 && len(texts) == 1:
			writeError(w, http.StatusBadRequest, CodeFileMissing, "Bitte einen Text eingeben.", nil)
			return
		case len(files) == 0:
			writeError(w, http.StatusBadRequest, CodeFileMissing, "Bitte eine Datei auswählen.", nil)
			return
		case len(files) > 1:
			writeError(w, http.StatusBadRequest, CodeInvalidRequest, "Bitte genau eine Datei schicken.", nil)
			return
		}

		data, err := readFile(files[0])
		if err != nil {
			writeInternalError(w, r, err)
			return
		}
		entry, err := u.Upload(r.Context(), service.Upload{
			Data: data, FileName: files[0].Filename, Module: module,
		})
		writeUploadResult(w, r, entry, err, kindFile, module)
	}
}

// writeUploadResult: 201 with Location and entry, or the matching error.
func writeUploadResult(w http.ResponseWriter, r *http.Request, entry core.Entry, err error, kind uploadKind, module string) {
	if err != nil {
		writeUploadError(w, r, err, kind, module)
		return
	}
	w.Header().Set("Location", "/api/v1/entries/"+entry.ID)
	writeJSON(w, http.StatusCreated, entryToJSON(entry))
}

// writeUploadError maps every error of the service to the code from
// API.md 3.1.
func writeUploadError(w http.ResponseWriter, r *http.Request, err error, kind uploadKind, module string) {
	var duplicate *service.DuplicateError
	switch {
	case kind == kindText && errors.Is(err, core.ErrFileEmpty):
		writeError(w, http.StatusBadRequest, CodeFileMissing, "Bitte einen Text eingeben.", nil)
	case errors.Is(err, core.ErrTextNotUTF8):
		writeError(w, http.StatusBadRequest, CodeInvalidRequest,
			"Der Text ist kein gültiges UTF-8.", map[string]any{"field": "text"})
	case errors.Is(err, core.ErrTextTooLong):
		writeError(w, http.StatusRequestEntityTooLarge, CodeFileTooLarge,
			"Der Text ist länger als 100 000 Zeichen.", map[string]any{"maxChars": core.MaxTextChars})
	case kind == kindText && errors.As(err, &duplicate):
		writeError(w, http.StatusConflict, CodeDuplicate, "Diesen Text hast du schon importiert.",
			map[string]any{"entryId": duplicate.EntryID})
	case errors.Is(err, core.ErrUnknownModule):
		writeError(w, http.StatusBadRequest, CodeInvalidRequest,
			"Unbekanntes Modul. Im Prototyp gibt es nur health.",
			map[string]any{"field": "module", "value": module})
	case errors.Is(err, core.ErrFileEmpty):
		writeError(w, http.StatusBadRequest, CodeFileMissing, "Die Datei ist leer.", nil)
	case errors.Is(err, core.ErrFileTooLarge):
		writeError(w, http.StatusRequestEntityTooLarge, CodeFileTooLarge, "Die Datei ist größer als 10 MB.", nil)
	case errors.Is(err, core.ErrUnsupportedFileType):
		writeError(w, http.StatusUnsupportedMediaType, CodeUnsupportedFileType,
			"Dieser Dateityp wird nicht unterstützt. Erlaubt sind JPEG, PNG, HEIC und PDF.", nil)
	case errors.As(err, &duplicate):
		writeError(w, http.StatusConflict, CodeDuplicate, "Diese Datei hast du schon hochgeladen.",
			map[string]any{"entryId": duplicate.EntryID})
	case errors.Is(err, core.ErrUnreadable):
		writeError(w, http.StatusUnprocessableEntity, CodeUnreadable,
			"Darauf ist kein lesbares Dokument zu erkennen. Versuche es mit einem schärferen Foto.", nil)
	case errors.Is(err, core.ErrInvalidExtraction), errors.Is(err, llm.ErrInvalidResponse):
		slog.Warn("extraction failed", "error", err)
		writeError(w, http.StatusBadGateway, CodeExtractionFailed,
			"Das Dokument konnte nicht ausgelesen werden. Bitte versuche es noch einmal.", nil)
	case errors.Is(err, llm.ErrUnavailable):
		slog.Warn("model unavailable", "error", err)
		writeError(w, http.StatusServiceUnavailable, CodeModelUnavailable,
			"Die Texterkennung ist gerade nicht erreichbar. Bitte versuche es später noch einmal.", nil)
	case errors.Is(err, context.Canceled):
		// The app dropped the connection; nobody reads the response anymore.
		slog.Info("upload canceled")
	default:
		writeInternalError(w, r, err)
	}
}

func readFile(fh *multipart.FileHeader) ([]byte, error) {
	f, err := fh.Open()
	if err != nil {
		return nil, err
	}
	defer f.Close()
	// Read one byte more than allowed, so core.CheckFile recognizes "too
	// large" instead of checking a truncated file.
	return io.ReadAll(io.LimitReader(f, core.MaxFileBytes+1))
}

func firstValue(values []string) string {
	if len(values) == 0 {
		return ""
	}
	return values[0]
}
