package api

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strings"

	"lifelog/backend/internal/auth"
	"lifelog/backend/internal/core"
)

// maxAccountBody: login, display name and password change are small; 4 KB
// is plenty and limits what the server reads before any check (API.md 3.10).
const maxAccountBody = 4 << 10

type loginRequest struct {
	// Pointers: a missing field and "username": null are both nil and
	// count as missing.
	Username *string `json:"username"`
	Password *string `json:"password"`
}

type loginJSON struct {
	Token              string        `json:"token"`
	ExpiresAt          string        `json:"expiresAt"`
	MustChangePassword bool          `json:"mustChangePassword"`
	User               loginUserJSON `json:"user"`
}

type loginUserJSON struct {
	ID          string `json:"id"`
	Username    string `json:"username"`
	DisplayName string `json:"displayName"`
	// Without omitempty: null stays visible (API.md 3.10).
	ConsentVersion *string `json:"consentVersion"`
}

// meJSON is the user as GET /me and PATCH /me return it (API.md 3.12).
// Deliberately without passwordHash, role, disabled and consentAt.
type meJSON struct {
	ID                 string  `json:"id"`
	Username           string  `json:"username"`
	DisplayName        string  `json:"displayName"`
	MustChangePassword bool    `json:"mustChangePassword"`
	ConsentVersion     *string `json:"consentVersion"` // null until the consent is recorded
	TimeZone           string  `json:"timeZone"`       // always present; Europe/Vienna until set
}

func meToJSON(u core.User) meJSON {
	zone, _ := core.UserTimeZone(u)
	return meJSON{ID: u.ID, Username: u.Username, DisplayName: u.DisplayName,
		MustChangePassword: u.MustChangePassword, ConsentVersion: u.ConsentVersion, TimeZone: zone}
}

type passwordRequest struct {
	CurrentPassword *string `json:"currentPassword"`
	NewPassword     *string `json:"newPassword"`
}

// decodeStrict reads a small JSON body into dst: unknown fields, trailing
// data and bodies over maxAccountBody are an error.
func decodeStrict(w http.ResponseWriter, r *http.Request, dst any) error {
	dec := json.NewDecoder(http.MaxBytesReader(w, r.Body, maxAccountBody))
	dec.DisallowUnknownFields()
	if err := dec.Decode(dst); err != nil {
		return err
	}
	if dec.More() {
		return errors.New("trailing data after the JSON object")
	}
	return nil
}

// loginHandler answers POST /api/v1/auth/login (API.md 3.10). Public: the
// middleware lets it through without a token.
func loginHandler(deps Dependencies) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req loginRequest
		if err := decodeStrict(w, r, &req); err != nil ||
			req.Username == nil || strings.TrimSpace(*req.Username) == "" ||
			req.Password == nil || *req.Password == "" {
			writeError(w, http.StatusBadRequest, CodeInvalidRequest,
				"Bitte Benutzername und Passwort schicken.", nil)
			return
		}

		res, err := deps.Accounts.Login(r.Context(), *req.Username, *req.Password, clientIP(r, deps))
		var tooMany *core.TooManyAttemptsError
		switch {
		case err == nil:
		case errors.As(err, &tooMany):
			writeTooManyAttempts(w, tooMany)
			return
		case errors.Is(err, core.ErrInvalidCredentials):
			writeError(w, http.StatusUnauthorized, CodeInvalidCredentials,
				"Benutzername oder Passwort stimmt nicht.", nil)
			return
		case errors.Is(err, context.Canceled):
			return // the app gave up; nobody reads the response
		default:
			writeInternalError(w, r, err)
			return
		}

		// The token must not end up in any cache.
		w.Header().Set("Cache-Control", "no-store")
		writeJSON(w, http.StatusOK, loginJSON{
			Token:              res.Token,
			ExpiresAt:          timeJSON(res.ExpiresAt),
			MustChangePassword: res.User.MustChangePassword,
			User: loginUserJSON{
				ID:             res.User.ID,
				Username:       res.User.Username,
				DisplayName:    res.User.DisplayName,
				ConsentVersion: res.User.ConsentVersion,
			},
		})
	}
}

// logoutHandler answers POST /api/v1/auth/logout (API.md 3.11): ends the
// session of this request's token.
func logoutHandler(a Accounts) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		sessionID, ok := auth.SessionIDFrom(r.Context())
		if !ok {
			writeInternalError(w, r, errors.New("logout without a session in the context"))
			return
		}
		// Finish even if the app drops the connection: a half logout would
		// leave the token valid.
		if err := a.Logout(context.WithoutCancel(r.Context()), sessionID); err != nil {
			writeInternalError(w, r, err)
			return
		}
		w.WriteHeader(http.StatusNoContent)
	}
}

// meHandler answers GET /api/v1/me (API.md 3.12) from the user the
// middleware put into the context.
func meHandler() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		user, ok := auth.UserFrom(r.Context())
		if !ok {
			writeInternalError(w, r, errors.New("GET /me without a user in the context"))
			return
		}
		writeJSON(w, http.StatusOK, meToJSON(user))
	}
}

// updateMeHandler answers PATCH /api/v1/me (API.md 3.13): displayName
// and/or timeZone.
//
// The body is read as raw JSON (as in correct.go), so that an unknown field
// can be named in details.field and null is told apart from a string. A
// field that is present but not a string (also null) is passed on as "":
// both rules refuse an empty value, so the service reports it in its fixed
// order (display name, then time zone) and nothing is stored.
func updateMeHandler(a Accounts) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		user, ok := auth.UserFrom(r.Context())
		if !ok {
			writeInternalError(w, r, errors.New("PATCH /me without a user in the context"))
			return
		}
		body, err := io.ReadAll(http.MaxBytesReader(w, r.Body, maxAccountBody))
		var fields map[string]json.RawMessage
		if err != nil || json.Unmarshal(body, &fields) != nil || fields == nil {
			writeError(w, http.StatusBadRequest, CodeInvalidRequest,
				"Bitte den Anzeigenamen als JSON schicken, z. B. {\"displayName\": \"…\"}.", nil)
			return
		}
		for _, name := range sortedKeys(fields) {
			if name != "displayName" && name != "timeZone" {
				writeError(w, http.StatusBadRequest, CodeInvalidRequest,
					"Unbekanntes Feld: "+name+".", map[string]any{"field": name})
				return
			}
		}
		var change core.ProfileChange
		if raw, ok := fields["displayName"]; ok {
			s, _ := asString(raw) // "" if not a string
			change.DisplayName = &s
		}
		if raw, ok := fields["timeZone"]; ok {
			s, _ := asString(raw)
			change.TimeZone = &s
		}

		updated, err := a.UpdateMe(r.Context(), user, change)
		switch {
		case err == nil:
			writeJSON(w, http.StatusOK, meToJSON(updated))
		case errors.Is(err, core.ErrEmptyProfileChange):
			writeError(w, http.StatusBadRequest, CodeInvalidRequest,
				"Bitte einen Anzeigenamen oder eine Zeitzone schicken.", nil)
		case errors.Is(err, core.ErrInvalidDisplayName):
			writeInvalidDisplayName(w)
		case errors.Is(err, core.ErrInvalidTimeZone):
			details := map[string]any{"field": "timeZone"}
			if v := asValue(fields["timeZone"]); v != nil {
				details["value"] = v
			}
			writeError(w, http.StatusBadRequest, CodeInvalidRequest, "Unbekannte Zeitzone.", details)
		case errors.Is(err, core.ErrUnauthorized):
			writeUnauthorized(w)
		case errors.Is(err, context.Canceled):
		default:
			writeInternalError(w, r, err)
		}
	}
}

// consentHandler answers PUT /api/v1/me/consent (API.md 3.15): stores the
// version of the consent text the user agreed to, with the server time.
// Like PATCH /me, the body is read as raw JSON, so an unknown field can be
// named and null is told apart from a string.
func consentHandler(a Accounts) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		user, ok := requireUser(w, r)
		if !ok {
			return
		}
		body, err := io.ReadAll(http.MaxBytesReader(w, r.Body, maxAccountBody))
		var fields map[string]json.RawMessage
		if err != nil || json.Unmarshal(body, &fields) != nil || fields == nil {
			writeInvalidConsentVersion(w)
			return
		}
		for _, name := range sortedKeys(fields) {
			if name != "version" {
				writeError(w, http.StatusBadRequest, CodeInvalidRequest,
					"Unbekanntes Feld: "+name+".", map[string]any{"field": name})
				return
			}
		}
		version, isString := asString(fields["version"])
		if !isString {
			writeInvalidConsentVersion(w)
			return
		}

		err = a.RecordConsent(r.Context(), user, version)
		switch {
		case err == nil:
			w.WriteHeader(http.StatusNoContent)
		case errors.Is(err, core.ErrInvalidConsentVersion):
			writeInvalidConsentVersion(w)
		case errors.Is(err, core.ErrUnauthorized):
			writeUnauthorized(w)
		case errors.Is(err, context.Canceled):
		default:
			writeInternalError(w, r, err)
		}
	}
}

func writeInvalidConsentVersion(w http.ResponseWriter) {
	writeError(w, http.StatusBadRequest, CodeInvalidRequest,
		"Bitte die Version des Hinweises schicken.", map[string]any{"field": "version"})
}

func writeInvalidDisplayName(w http.ResponseWriter) {
	writeError(w, http.StatusBadRequest, CodeInvalidRequest,
		"Der Anzeigename muss 1 bis 50 Zeichen lang sein.", map[string]any{"field": "displayName"})
}

// changePasswordHandler answers POST /api/v1/me/password (API.md 3.14).
func changePasswordHandler(deps Dependencies) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		user, okUser := auth.UserFrom(r.Context())
		sessionID, okSession := auth.SessionIDFrom(r.Context())
		if !okUser || !okSession {
			writeInternalError(w, r, errors.New("password change without a user in the context"))
			return
		}
		var req passwordRequest
		if err := decodeStrict(w, r, &req); err != nil || req.CurrentPassword == nil || req.NewPassword == nil {
			writeError(w, http.StatusBadRequest, CodeInvalidRequest,
				"Bitte das bisherige und das neue Passwort schicken.", nil)
			return
		}

		err := deps.Accounts.ChangePassword(r.Context(), user, sessionID,
			*req.CurrentPassword, *req.NewPassword, clientIP(r, deps))
		var tooMany *core.TooManyAttemptsError
		var weak *core.WeakPasswordError
		switch {
		case err == nil:
			w.WriteHeader(http.StatusNoContent)
		case errors.As(err, &tooMany):
			writeTooManyAttempts(w, tooMany)
		case errors.Is(err, core.ErrWrongPassword):
			// 400, not 401: the token is still valid, the app must not log out.
			writeError(w, http.StatusBadRequest, CodeWrongPassword, "Das bisherige Passwort stimmt nicht.", nil)
		case errors.As(err, &weak):
			writeError(w, http.StatusBadRequest, CodeWeakPassword, weakPasswordMessage(weak.Problem), nil)
		case errors.Is(err, core.ErrUnauthorized):
			writeUnauthorized(w)
		case errors.Is(err, context.Canceled):
		default:
			writeInternalError(w, r, err)
		}
	}
}

func weakPasswordMessage(p core.PasswordProblem) string {
	switch p {
	case core.PasswordTooShort:
		return "Das neue Passwort muss mindestens 10 Zeichen lang sein."
	case core.PasswordTooLong:
		return "Das neue Passwort ist zu lang. Erlaubt sind höchstens 72 Byte, Umlaute zählen doppelt."
	case core.PasswordUnchanged:
		return "Das neue Passwort muss sich vom bisherigen unterscheiden."
	}
	return "Das neue Passwort ist nicht sicher genug."
}
