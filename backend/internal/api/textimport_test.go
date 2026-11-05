package api

import (
	"bytes"
	"fmt"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"lifelog/backend/internal/dienst"
	"lifelog/backend/internal/kern"
	"lifelog/backend/internal/modell"
)

// textFormular baut ein Formular aus Feldern in fester Reihenfolge; ein
// Name darf mehrfach vorkommen. Ein Feld "datei" wird als Datei angehängt.
func textFormular(t *testing.T, felder ...[2]string) *http.Request {
	t.Helper()
	var body bytes.Buffer
	mw := multipart.NewWriter(&body)
	for _, f := range felder {
		if f[0] == "datei" {
			fw, err := mw.CreateFormFile("datei", "a.pdf")
			if err != nil {
				t.Fatal(err)
			}
			fw.Write([]byte(f[1]))
			continue
		}
		mw.WriteField(f[0], f[1])
	}
	mw.Close()
	r := httptest.NewRequest(http.MethodPost, "/api/v1/eintraege", &body)
	r.Header.Set("Content-Type", mw.FormDataContentType())
	return r
}

func textEintrag() kern.Eintrag {
	e := vollerEintrag()
	e.Volltext = "  Hausarztpraxis Beispiel\r\nÜberweisung\n"
	e.Datei = kern.Datei{ID: "6727a1f3c2b4e5d6f7a8b9c1", Dateiname: "text.txt",
		MimeTyp: "text/plain; charset=utf-8", Groesse: 42, Sha256: strings.Repeat("cd", 32)}
	return e
}

func TestTextImport_Erfolg(t *testing.T) {
	text := "  Hausarztpraxis Beispiel\r\nÜberweisung\n"
	h := &hochladerAttrappe{eintrag: textEintrag()}
	rec := hochladen(t, h, textFormular(t, [2]string{"text", text}, [2]string{"modul", "gesundheit"}))

	if rec.Code != http.StatusCreated {
		t.Fatalf("Status %d: %s", rec.Code, rec.Body)
	}
	if rec.Header().Get("Location") != "/api/v1/eintraege/6727a1f3c2b4e5d6f7a8b9c0" {
		t.Errorf("Location = %q", rec.Header().Get("Location"))
	}
	// Unverändert an den Dienst, auch Leerraum und \r\n.
	if h.textUpload == nil || h.textUpload.Text != text || h.textUpload.Modul != "gesundheit" || h.upload != nil {
		t.Errorf("an den Dienst: text %+v, datei %+v", h.textUpload, h.upload)
	}
	for _, muss := range []string{`"mimeTyp":"text/plain; charset=utf-8"`, `"dateiname":"text.txt"`, `"korrekturen":[]`} {
		if !strings.Contains(rec.Body.String(), muss) {
			t.Errorf("Antwort ohne %s: %s", muss, rec.Body)
		}
	}
}

// Fehler, die schon das Formular zeigt; der Dienst wird nicht aufgerufen.
func TestTextImport_Formularfehler(t *testing.T) {
	for _, f := range []struct {
		name      string
		felder    [][2]string
		status    int
		code      string
		nachricht string
	}{
		{"weder datei noch text", [][2]string{{"modul", "gesundheit"}}, 400, CodeDateiFehlt, "Datei"},
		{"text leer", [][2]string{{"text", ""}}, 400, CodeDateiFehlt, "Text"},
		{"text nur Leerraum", [][2]string{{"text", " \r\n\t "}}, 400, CodeDateiFehlt, "Text"},
		{"datei und text", [][2]string{{"datei", "%PDF-"}, {"text", "Befund"}}, 400, CodeUngueltigeAnfrage, "nicht beides"},
		{"zwei Texte", [][2]string{{"text", "a"}, {"text", "b"}}, 400, CodeUngueltigeAnfrage, "genau einen Text"},
		{"zwei Texte, einer leer", [][2]string{{"text", ""}, {"text", "b"}}, 400, CodeUngueltigeAnfrage, "genau einen Text"},
	} {
		t.Run(f.name, func(t *testing.T) {
			h := &hochladerAttrappe{}
			rec := hochladen(t, h, textFormular(t, f.felder...))
			a := pruefeFehler(t, rec, f.status, f.code)
			if !strings.Contains(a.Fehler.Nachricht, f.nachricht) {
				t.Errorf("Nachricht %q enthält %q nicht", a.Fehler.Nachricht, f.nachricht)
			}
			if h.upload != nil || h.textUpload != nil {
				t.Error("Dienst aufgerufen")
			}
		})
	}
}

// Die Prüfungen aus kern.PruefeText, über den echten Dienst: So ist auch
// belegt, dass Zeichen und nicht Bytes zählen.
func TestTextImport_PruefungUeberDenDienst(t *testing.T) {
	for _, f := range []struct {
		name    string
		felder  [][2]string
		status  int
		code    string
		details string
	}{
		{"100 001 Zeichen", [][2]string{{"text", strings.Repeat("a", kern.MaxTextZeichen+1)}},
			413, CodeDateiZuGross, "map[maxZeichen:100000]"},
		{"100 001 Umlaute", [][2]string{{"text", strings.Repeat("ü", kern.MaxTextZeichen+1)}},
			413, CodeDateiZuGross, "map[maxZeichen:100000]"},
		{"ungültiges UTF-8", [][2]string{{"text", "Befund \xff\xfe"}},
			400, CodeUngueltigeAnfrage, "map[feld:text]"},
		{"unbekanntes Modul", [][2]string{{"text", "Befund"}, {"modul", "sport"}},
			400, CodeUngueltigeAnfrage, "map[feld:modul wert:sport]"},
	} {
		t.Run(f.name, func(t *testing.T) {
			// Dienst ohne Speicher und Modelle: Die Prüfung kommt vor allem anderen.
			d := dienst.NeuHochladen(nil, nil, nil, nil)
			router := NeuerRouter(Abhaengigkeiten{DB: pingerAttrappe{}, Hochladen: d})
			rec := httptest.NewRecorder()
			router.ServeHTTP(rec, textFormular(t, f.felder...))

			a := pruefeFehler(t, rec, f.status, f.code)
			if fmt.Sprint(a.Fehler.Details) != f.details {
				t.Errorf("details = %v, erwartet %s", a.Fehler.Details, f.details)
			}
		})
	}
}

// 100 000 Umlaute sind 200 000 Bytes, aber erlaubt: Der Handler lässt sie
// durch.
func TestTextImport_GrenzeErlaubt(t *testing.T) {
	h := &hochladerAttrappe{eintrag: textEintrag()}
	rec := hochladen(t, h, textFormular(t, [2]string{"text", strings.Repeat("ü", kern.MaxTextZeichen)}))
	if rec.Code != http.StatusCreated || h.textUpload == nil {
		t.Errorf("Status %d", rec.Code)
	}
}

func TestTextImport_FehlerDesDienstes(t *testing.T) {
	for _, f := range []struct {
		err       error
		status    int
		code      string
		nachricht string
		details   string
	}{
		{kern.ErrDateiLeer, 400, CodeDateiFehlt, "Bitte einen Text eingeben.", "map[]"},
		{&dienst.DuplikatFehler{EintragID: "6727a1f3c2b4e5d6f7a8b9c0"}, 409, CodeDuplikat,
			"Diesen Text hast du schon importiert.", "map[eintragId:6727a1f3c2b4e5d6f7a8b9c0]"},
		{&kern.ErkennungFehler{Feld: "typ", Grund: "x"}, 502, CodeAuslesenFehlgeschlagen, "", "map[]"},
		{fmt.Errorf("JSON kaputt: %w", modell.ErrAntwortUngueltig), 502, CodeAuslesenFehlgeschlagen, "", "map[]"},
		{fmt.Errorf("Gemini 503: %w", modell.ErrNichtErreichbar), 503, CodeModellNichtErreichbar, "", "map[]"},
		{fmt.Errorf("Datenbank weg"), 500, CodeInternerFehler, "", "map[]"},
	} {
		t.Run(f.code, func(t *testing.T) {
			h := &hochladerAttrappe{err: f.err}
			rec := hochladen(t, h, textFormular(t, [2]string{"text", "Befund"}))
			a := pruefeFehler(t, rec, f.status, f.code)
			if f.nachricht != "" && a.Fehler.Nachricht != f.nachricht {
				t.Errorf("Nachricht %q", a.Fehler.Nachricht)
			}
			if fmt.Sprint(a.Fehler.Details) != f.details && !(f.details == "map[]" && a.Fehler.Details == nil) {
				t.Errorf("details = %v, erwartet %s", a.Fehler.Details, f.details)
			}
			for _, intern := range []string{"Datenbank weg", "Gemini", "JSON kaputt", "Befund"} {
				if strings.Contains(rec.Body.String(), intern) {
					t.Errorf("%q in der Antwort: %s", intern, rec.Body)
				}
			}
		})
	}
}
