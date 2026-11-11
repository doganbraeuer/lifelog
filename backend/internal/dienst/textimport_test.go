package dienst

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"log/slog"
	"slices"
	"strings"
	"testing"

	"lifelog/backend/internal/core"
	"lifelog/backend/internal/modell"
	"lifelog/backend/internal/modell/attrappe"
)

// Ein synthetischer Text mit Leerraum am Rand, \r\n und Umlauten: Er muss
// bytegenau so gespeichert werden, wie er ankommt.
const eingabe = "  Hausarztpraxis Beispiel\r\nÜberweisung vom 21.10.2026\n\nDiagnose: Rückenschmerzen \n"

func textErkennung() core.Erkennung {
	// Wie die Attrappe sie liefert. Volltext und Lesbar setzt das Modell beim
	// Text nicht; der Dienst muss sie selbst setzen und Abweichungen
	// überschreiben.
	return core.Erkennung{
		Lesbar:        false,
		Volltext:      "vom Modell erfunden",
		Dokumentdatum: zeiger("2026-10-21"),
		Quelle:        zeiger("Hausarztpraxis Beispiel"),
		Typ:           "arztbrief",
		Infos:         []core.ErkannteInfo{{Name: "Diagnose", Wert: "Rückenschmerzen"}},
	}
}

func textAufbau() aufbau {
	a := neuerAufbau()
	a.leser.Erkennung = textErkennung()
	return a
}

func TestTextImport_Erfolg(t *testing.T) {
	a := textAufbau()
	e, err := a.dienst.TextAusfuehren(context.Background(), TextUpload{Text: eingabe})
	if err != nil {
		t.Fatal(err)
	}

	// Nur TextLesen, nie DokumentLesen, und mit dem Text unverändert.
	n, gesendet := a.leser.TextAufrufe()
	if n != 1 || a.leser.Aufrufe() != 0 || gesendet != eingabe {
		t.Errorf("TextLesen %d×, DokumentLesen %d×, Text %q", n, a.leser.Aufrufe(), gesendet)
	}

	// volltext ist der eingegebene Text, nicht der des Modells.
	if e.Volltext != eingabe {
		t.Errorf("Volltext = %q", e.Volltext)
	}
	summe := sha256.Sum256([]byte(eingabe))
	want := core.Datei{
		ID: e.Datei.ID, Dateiname: "text.txt", MimeTyp: "text/plain; charset=utf-8",
		Groesse: int64(len(eingabe)), Sha256: hex.EncodeToString(summe[:]),
	}
	if e.Datei != want {
		t.Errorf("Datei = %+v\nerwartet %+v", e.Datei, want)
	}
	// Bytes, nicht Zeichen: Ü und ü sind je zwei Bytes.
	if e.Datei.Groesse != int64(len([]rune(eingabe))+2) {
		t.Errorf("Größe %d ist nicht in Bytes", e.Datei.Groesse)
	}
	if got := string(a.speicher.dateien[e.Datei.ID]); got != eingabe {
		t.Errorf("gespeicherte Datei = %q", got)
	}

	if e.Modul != "gesundheit" || e.Typ != "arztbrief" || len(e.Infos) != 1 || e.Infos[0].ID != "i1" {
		t.Errorf("Eintrag %+v", e)
	}
	if len(a.speicher.eintraege) != 1 || a.speicher.eintraege[0].Volltext != eingabe {
		t.Error("Eintrag nicht mit dem eingegebenen Text gespeichert")
	}
	// Abschnitte aus dem eingegebenen Text.
	if len(a.speicher.abschnitte) == 0 || !strings.Contains(a.speicher.abschnitte[0].Text, "Hausarztpraxis") {
		t.Errorf("Abschnitte %+v", a.speicher.abschnitte)
	}
}

func TestTextImport_DuplikatVorModellaufruf(t *testing.T) {
	a := textAufbau()
	summe := sha256.Sum256([]byte(eingabe))
	a.speicher.hashes[hex.EncodeToString(summe[:])] = "bbbbbbbbbbbbbbbbbbbbbbbb"

	_, err := a.dienst.TextAusfuehren(context.Background(), TextUpload{Text: eingabe})

	var dup *DuplikatFehler
	if !errors.As(err, &dup) || dup.EintragID != "bbbbbbbbbbbbbbbbbbbbbbbb" {
		t.Fatalf("err = %v", err)
	}
	if n, _ := a.leser.TextAufrufe(); n != 0 || a.einbetter.Aufrufe() != 0 {
		t.Errorf("Modell trotz Duplikat aufgerufen: Lesen %d, Einbetten %d", n, a.einbetter.Aufrufe())
	}
}

// Schon ein zusätzliches Leerzeichen ergibt einen anderen Hash (API.md 4).
func TestTextImport_LeerzeichenIstKeinDuplikat(t *testing.T) {
	a := textAufbau()
	summe := sha256.Sum256([]byte(eingabe))
	a.speicher.hashes[hex.EncodeToString(summe[:])] = "bbbbbbbbbbbbbbbbbbbbbbbb"
	if _, err := a.dienst.TextAusfuehren(context.Background(), TextUpload{Text: eingabe + " "}); err != nil {
		t.Errorf("err = %v", err)
	}
}

func TestTextImport_PruefungenVorJedemAufruf(t *testing.T) {
	for _, f := range []struct {
		name   string
		upload TextUpload
		fehler error
	}{
		{"leer", TextUpload{Text: ""}, core.ErrDateiLeer},
		{"nur Leerraum", TextUpload{Text: " \r\n\t"}, core.ErrDateiLeer},
		{"ungültiges UTF-8", TextUpload{Text: "Befund \xff"}, core.ErrTextKeinUTF8},
		{"zu lang", TextUpload{Text: strings.Repeat("ä", core.MaxTextZeichen+1)}, core.ErrTextZuLang},
		{"unbekanntes Modul", TextUpload{Text: eingabe, Modul: "sport"}, core.ErrUnbekanntesModul},
	} {
		t.Run(f.name, func(t *testing.T) {
			a := textAufbau()
			_, err := a.dienst.TextAusfuehren(context.Background(), f.upload)
			if !errors.Is(err, f.fehler) {
				t.Errorf("err = %v, erwartet %v", err, f.fehler)
			}
			if n, _ := a.leser.TextAufrufe(); n != 0 || len(a.speicher.protokoll) != 0 {
				t.Errorf("Aufrufe trotz Fehler: Lesen %d, Speicher %v", n, a.speicher.protokoll)
			}
		})
	}
}

// Genau 100 000 Zeichen mit Umlauten (200 000 Bytes) sind erlaubt.
func TestTextImport_GrenzeInZeichen(t *testing.T) {
	a := textAufbau()
	text := strings.Repeat("ü", core.MaxTextZeichen)
	e, err := a.dienst.TextAusfuehren(context.Background(), TextUpload{Text: text})
	if err != nil {
		t.Fatal(err)
	}
	if e.Datei.Groesse != 2*core.MaxTextZeichen {
		t.Errorf("Größe %d", e.Datei.Groesse)
	}
}

func TestTextImport_ModellFehlerSpeichertNichts(t *testing.T) {
	for _, f := range []struct {
		name string
		err  error
	}{
		{"nicht erreichbar", modell.ErrNichtErreichbar},
		{"Antwort ungültig", modell.ErrAntwortUngueltig},
	} {
		t.Run(f.name, func(t *testing.T) {
			a := textAufbau()
			a.dienst.leser = &attrappe.Leser{Err: f.err}
			_, err := a.dienst.TextAusfuehren(context.Background(), TextUpload{Text: eingabe})
			if !errors.Is(err, f.err) {
				t.Errorf("err = %v", err)
			}
			if !slices.Equal(a.speicher.protokoll, []string{"EintragMitHash"}) || !a.speicher.leer() {
				t.Errorf("gespeichert: %v", a.speicher.protokoll)
			}
		})
	}
}

// Ein unerlaubter Typ vom Modell ist ungültig (502), kein NICHT_LESBAR.
func TestTextImport_TypNichtErlaubt(t *testing.T) {
	a := textAufbau()
	e := textErkennung()
	e.Typ = "kochrezept"
	a.leser.Erkennung = e
	_, err := a.dienst.TextAusfuehren(context.Background(), TextUpload{Text: eingabe})
	if !errors.Is(err, core.ErrErkennungUngueltig) || errors.Is(err, core.ErrNichtLesbar) {
		t.Errorf("err = %v", err)
	}
}

func TestTextImport_AufraeumenBeiSpeicherfehler(t *testing.T) {
	a := textAufbau()
	dbFehler := errors.New("Verbindung verloren")
	a.speicher.fehler["EintragSpeichern"] = dbFehler
	if _, err := a.dienst.TextAusfuehren(context.Background(), TextUpload{Text: eingabe}); !errors.Is(err, dbFehler) {
		t.Errorf("err = %v", err)
	}
	if !a.speicher.leer() {
		t.Error("nicht aufgeräumt")
	}
}

func TestTextImport_TextNichtImLog(t *testing.T) {
	var log bytes.Buffer
	alt := slog.Default()
	slog.SetDefault(slog.New(slog.NewTextHandler(&log, nil)))
	t.Cleanup(func() { slog.SetDefault(alt) })

	a := textAufbau()
	e := textErkennung()
	e.Infos = append(e.Infos, core.ErkannteInfo{Name: "Dosis", Wert: ""}) // wird ausgelassen und geloggt
	a.leser.Erkennung = e
	if _, err := a.dienst.TextAusfuehren(context.Background(), TextUpload{Text: eingabe}); err != nil {
		t.Fatal(err)
	}
	for _, inhalt := range []string{"Hausarztpraxis", "Rückenschmerzen", "Überweisung", "Dosis"} {
		if strings.Contains(log.String(), inhalt) {
			t.Errorf("Log enthält %q: %s", inhalt, log.String())
		}
	}
}
