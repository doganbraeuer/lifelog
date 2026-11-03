# Lifelog REST-API (Prototyp)

Stand: 03.11.2026 · Version `v1` · Entwurf

Dieses Dokument legt die Schnittstelle zwischen der Flutter-App (`app/`) und
dem Go-Backend (`backend/`) fest. Beide Seiten bauen gegen diesen Vertrag.
Änderungen an der API werden zuerst hier eingetragen.

Alle Beispiele enthalten nur synthetische Daten.

---

## 1. Grundsätze

### Basis

- Basis-Pfad: `/api/v1`
- Anfragen und Antworten sind JSON in UTF-8
  (`Content-Type: application/json; charset=utf-8`). Ausnahmen sind der Upload
  (`multipart/form-data`) und der Abruf der Originaldatei (Binärdaten).
- Feldnamen sind deutsch und folgen dem Datenmodell aus `CLAUDE.md`.
- IDs von Einträgen sind MongoDB-ObjectIds als Hex-String mit 24 Zeichen,
  z. B. `"6727a1f3c2b4e5d6f7a8b9c0"`.
- Im Prototyp gibt es keine Anmeldung. Das Backend kennt genau einen Nutzer
  (siehe [Offene Punkte](#4-offene-punkte)).

### Datum und Zeit

| Feld | Format | Beispiel | Bemerkung |
|---|---|---|---|
| `dokumentdatum` | `YYYY-MM-DD` | `"2026-10-14"` | Reines Datum ohne Uhrzeit und ohne Zeitzone. `null`, wenn das Vision-LLM kein Datum erkennt. |
| `importdatum` | RFC 3339, UTC | `"2026-11-03T18:42:07Z"` | Setzt das Backend beim Upload. Immer vorhanden. |
| `zeitpunkt` (Korrektur) | RFC 3339, UTC | `"2026-11-03T19:05:31Z"` | Setzt das Backend beim Korrigieren. |

Warum ein reines Datum für `dokumentdatum`? Ein Befund vom 14.10. soll immer
am 14.10. stehen. Ein Zeitstempel könnte sich je nach Zeitzone des Geräts um
einen Tag verschieben.

### Fehlerformat

Jeder Fehler hat denselben Aufbau. Die App wertet `code` aus. `nachricht` ist
ein deutscher Text, den die App direkt anzeigen darf. `details` ist optional
und hängt vom Code ab.

```json
{
  "fehler": {
    "code": "EINTRAG_NICHT_GEFUNDEN",
    "nachricht": "Diesen Eintrag gibt es nicht.",
    "details": {}
  }
}
```

### Alle Fehlercodes

| HTTP | Code | Wann |
|---|---|---|
| 400 | `UNGUELTIGE_ANFRAGE` | JSON nicht lesbar, Pflichtfeld fehlt, falsches Format, unbekanntes Feld |
| 400 | `UNGUELTIGER_PARAMETER` | Unbekannter Wert für einen Query-Parameter |
| 400 | `UNGUELTIGE_ID` | ID ist keine gültige ObjectId |
| 400 | `DATEI_FEHLT` | Upload ohne Feld `datei` oder mit leerer Datei |
| 400 | `FELD_NICHT_KORRIGIERBAR` | Korrektur eines Felds, das (noch) nicht korrigierbar ist, z. B. `volltext` |
| 400 | `FRAGE_FEHLT` | Frage fehlt oder besteht nur aus Leerzeichen |
| 400 | `FRAGE_ZU_LANG` | Frage länger als 500 Zeichen |
| 404 | `EINTRAG_NICHT_GEFUNDEN` | Kein Eintrag mit dieser ID |
| 404 | `INFO_NICHT_GEFUNDEN` | Korrektur verweist auf eine unbekannte Info-ID |
| 409 | `DUPLIKAT` | Genau diese Datei wurde schon hochgeladen |
| 413 | `DATEI_ZU_GROSS` | Datei größer als 10 MB |
| 415 | `DATEITYP_NICHT_UNTERSTUETZT` | Kein JPEG, PNG, HEIC oder PDF |
| 422 | `NICHT_LESBAR` | Das Vision-LLM erkennt in der Datei kein Dokument |
| 500 | `INTERNER_FEHLER` | Unerwarteter Fehler im Backend, z. B. Datenbank nicht erreichbar |
| 502 | `AUSLESEN_FEHLGESCHLAGEN` | Das Vision-LLM liefert eine Antwort, die nicht zum erwarteten JSON passt |
| 503 | `MODELL_NICHT_ERREICHBAR` | Sprach- oder Embedding-Modell antwortet nicht oder meldet ein Limit |

### Wichtige Regeln

- Die App ruft nie direkt ein Modell auf. Alle Aufrufe an Vision-LLM,
  Sprachmodell und Embedding-Modell laufen über das Backend.
- Die API liefert keine medizinischen Bewertungen oder Empfehlungen. Sie gibt
  nur wieder, was in den Dokumenten steht.

---

## 2. Datenmodell „Eintrag“

### Vollständiger Eintrag

So liefern `POST /eintraege`, `GET /eintraege/{id}` und
`PATCH /eintraege/{id}` einen Eintrag zurück:

```json
{
  "id": "6727a1f3c2b4e5d6f7a8b9c0",
  "modul": "gesundheit",
  "typ": "laborbefund",
  "dokumentdatum": "2026-10-14",
  "importdatum": "2026-11-03T18:42:07Z",
  "quelle": "Labor Muster GmbH, Salzburg",
  "volltext": "Labor Muster GmbH\nMusterstraße 1, 5020 Salzburg\n\nLaborbefund vom 14.10.2026\nPatient: Max Mustermann, geb. 01.01.1990\n\nHämoglobin 14,2 g/dl (Ref. 13,5-17,5)\nLeukozyten 6,8 G/l (Ref. 4,0-10,0)\nCRP <0,5 mg/dl (Ref. <0,5)\nBefundart: Blutbild klein",
  "infos": [
    { "id": "i1", "name": "Hämoglobin", "wert": "14,2", "einheit": "g/dl" },
    { "id": "i2", "name": "Leukozyten", "wert": "6,8", "einheit": "G/l" },
    { "id": "i3", "name": "CRP", "wert": "<0,5", "einheit": "mg/dl" },
    { "id": "i4", "name": "Befundart", "wert": "Blutbild klein" }
  ],
  "datei": {
    "id": "6727a1f3c2b4e5d6f7a8b9c1",
    "dateiname": "befund_oktober.pdf",
    "mimeTyp": "application/pdf",
    "groesse": 184320,
    "sha256": "3f8a1c0e9b7d2f4a6c5e8b1d0a9f7e3c2b4d6a8f0e1c3b5d7a9f2e4c6b8d0a1f"
  },
  "korrekturen": [
    {
      "feld": "infos.i2.wert",
      "erkannt": "6,3",
      "korrigiert": "6,8",
      "zeitpunkt": "2026-11-03T19:05:31Z"
    }
  ]
}
```

### Felder

| Feld | Typ | Bemerkung |
|---|---|---|
| `id` | String | ObjectId des Eintrags |
| `modul` | String | Pflichtfeld. Bereich, zu dem der Eintrag gehört. Im Prototyp hat jeder Eintrag den Wert `gesundheit`. Setzt das Backend beim Upload, nicht das Vision-LLM. Weitere Werte kommen mit neuen Modulen dazu (z. B. Ernährung, Sport). |
| `typ` | String | Dokumenttyp innerhalb des Moduls. Für `gesundheit` einer von `arztbrief`, `laborbefund`, `rezept`, `sonstiges` |
| `dokumentdatum` | String oder `null` | Datum auf dem Dokument, siehe Abschnitt 1 |
| `importdatum` | String | Zeitpunkt des Uploads |
| `quelle` | String oder `null` | Ausstellende Stelle. `null`, wenn nicht erkannt |
| `volltext` | String | Text des Dokuments, wie ihn das Vision-LLM gelesen hat |
| `infos` | Liste | Extrahierte Angaben, siehe unten |
| `datei` | Objekt | Referenz auf die Originaldatei |
| `korrekturen` | Liste | Korrigierte Felder mit ursprünglich erkanntem Wert |

**`infos`**

| Feld | Typ | Bemerkung |
|---|---|---|
| `id` | String | Stabile ID innerhalb des Eintrags (`i1`, `i2`, …). Vergibt das Backend beim Upload. Darüber werden Infos korrigiert. |
| `name` | String | Bezeichnung, z. B. `"Hämoglobin"` |
| `wert` | String | Immer ein String, genau wie auf dem Dokument (`"14,2"`, `"<0,5"`, `"positiv"`). So geht nichts durch Umwandeln in Zahlen verloren. |
| `einheit` | String, optional | Fehlt, wenn das Dokument keine Einheit nennt |

**`datei`**

| Feld | Typ | Bemerkung |
|---|---|---|
| `id` | String | ID der gespeicherten Originaldatei |
| `dateiname` | String | Name beim Upload |
| `mimeTyp` | String | Vom Backend anhand des Inhalts erkannt, nicht aus der Dateiendung |
| `groesse` | Zahl | Größe in Bytes |
| `sha256` | String | SHA-256-Hash des Dateiinhalts als Hex-String (64 Zeichen). Dient der Duplikat-Erkennung. |

Die Datei selbst steckt nicht im JSON. Die App lädt sie über
`GET /eintraege/{id}/datei`.

**`korrekturen`**

| Feld | Typ | Bemerkung |
|---|---|---|
| `feld` | String | Pfad des korrigierten Felds: `typ`, `dokumentdatum`, `quelle`, `infos.<id>.wert` oder `infos.<id>.einheit` |
| `erkannt` | String oder `null` | Ursprünglich vom Vision-LLM erkannter Wert |
| `korrigiert` | String oder `null` | Aktueller, korrigierter Wert |
| `zeitpunkt` | String | Zeitpunkt der letzten Korrektur dieses Felds |

Regeln für Korrekturen:

1. Die Felder des Eintrags (`dokumentdatum`, `infos[].wert` usw.) enthalten
   immer den **aktuellen** Wert. Die App muss für die Anzeige nichts
   zusammensetzen.
2. Pro Feld gibt es höchstens einen Eintrag in `korrekturen`. `erkannt` ist
   immer der ursprüngliche Wert des Vision-LLM, auch wenn ein Feld mehrmals
   korrigiert wird. `korrigiert` und `zeitpunkt` zeigen den letzten Stand.
3. Setzt jemand ein Feld wieder auf den erkannten Wert zurück, entfernt das
   Backend den Eintrag aus `korrekturen`. Das Feld gilt dann als nicht
   korrigiert.
4. Ob eine Info korrigiert wurde, sieht die App daran, ob in `korrekturen` ein
   `feld` mit `infos.<id>.` beginnt.

Diese Regeln sind deterministisch und bekommen im Backend Tests.

### Kurzform für die Zeitleiste

Die Liste liefert nur, was die Zeitleiste braucht. `volltext`, `infos`,
`datei` und `korrekturen` fehlen, damit die Antwort klein bleibt.

```json
{
  "id": "6727a1f3c2b4e5d6f7a8b9c0",
  "modul": "gesundheit",
  "typ": "laborbefund",
  "dokumentdatum": "2026-10-14",
  "importdatum": "2026-11-03T18:42:07Z",
  "quelle": "Labor Muster GmbH, Salzburg"
}
```

---

## 3. Endpunkte

| Methode | Pfad | Zweck |
|---|---|---|
| `POST` | `/api/v1/eintraege` | Datei hochladen und auslesen lassen |
| `GET` | `/api/v1/eintraege` | Alle Einträge für die Zeitleiste |
| `GET` | `/api/v1/eintraege/{id}` | Einen Eintrag ansehen |
| `GET` | `/api/v1/eintraege/{id}/datei` | Originaldatei abrufen |
| `PATCH` | `/api/v1/eintraege/{id}` | Erkannte Werte korrigieren |
| `POST` | `/api/v1/fragen` | Frage stellen, Antwort mit Quellenangabe |

### 3.1 Datei hochladen und auslesen

```http
POST /api/v1/eintraege
Content-Type: multipart/form-data; boundary=----lifelog
```

**Anfrage**

Ein Formularfeld `datei` mit genau einer Datei.

- Erlaubte Typen: `image/jpeg`, `image/png`, `image/heic`, `application/pdf`.
  Das Backend prüft den Typ am Inhalt der Datei, nicht nur an der Endung.
- Maximale Größe: 10 MB.

```http
------lifelog
Content-Disposition: form-data; name="datei"; filename="befund_oktober.pdf"
Content-Type: application/pdf

<Binärdaten>
------lifelog--
```

**Ablauf im Backend**

1. Größe und Typ prüfen.
2. SHA-256-Hash über den Dateiinhalt berechnen und in MongoDB nach
   `datei.sha256` suchen. Gibt es den Hash schon, bricht das Backend mit
   `409 DUPLIKAT` ab. Diese Prüfung kommt **vor** dem Modellaufruf, damit
   für ein Duplikat kein Vision-LLM-Aufruf anfällt. Ein eindeutiger Index auf
   `datei.sha256` fängt zusätzlich den Fall ab, dass dieselbe Datei zweimal
   gleichzeitig hochgeladen wird.
3. Datei an das Vision-LLM schicken. Es liefert `dokumentdatum`, `quelle`,
   `typ`, `volltext` und `infos` als JSON.
4. Das JSON prüfen (Pflichtfelder, Datumsformat, erlaubter `typ`) und
   `modul` auf `gesundheit` setzen.
5. Volltext in Abschnitte zerlegen und Embeddings berechnen.
6. Eintrag, Abschnitte mit Embeddings und Originaldatei speichern.

Schlägt ein Schritt fehl, speichert das Backend nichts. Es bleibt kein halber
Eintrag zurück.

Der Aufruf ist **synchron** und kann bis zu etwa 60 Sekunden dauern. Die App
zeigt währenddessen eine Ladeanzeige und setzt ihren HTTP-Timeout für diesen
Aufruf auf mindestens 90 Sekunden.

**Antwort `201 Created`**

```http
Location: /api/v1/eintraege/6727a1f3c2b4e5d6f7a8b9c0
```

Der Body ist der vollständige Eintrag wie in Abschnitt 2, mit leerer Liste
`korrekturen`:

```json
{
  "id": "6727a1f3c2b4e5d6f7a8b9c0",
  "modul": "gesundheit",
  "typ": "laborbefund",
  "dokumentdatum": "2026-10-14",
  "importdatum": "2026-11-03T18:42:07Z",
  "quelle": "Labor Muster GmbH, Salzburg",
  "volltext": "Labor Muster GmbH\nMusterstraße 1, 5020 Salzburg\n\nLaborbefund vom 14.10.2026\nPatient: Max Mustermann, geb. 01.01.1990\n\nHämoglobin 14,2 g/dl (Ref. 13,5-17,5)\nLeukozyten 6,3 G/l (Ref. 4,0-10,0)\nCRP <0,5 mg/dl (Ref. <0,5)\nBefundart: Blutbild klein",
  "infos": [
    { "id": "i1", "name": "Hämoglobin", "wert": "14,2", "einheit": "g/dl" },
    { "id": "i2", "name": "Leukozyten", "wert": "6,3", "einheit": "G/l" },
    { "id": "i3", "name": "CRP", "wert": "<0,5", "einheit": "mg/dl" },
    { "id": "i4", "name": "Befundart", "wert": "Blutbild klein" }
  ],
  "datei": {
    "id": "6727a1f3c2b4e5d6f7a8b9c1",
    "dateiname": "befund_oktober.pdf",
    "mimeTyp": "application/pdf",
    "groesse": 184320,
    "sha256": "3f8a1c0e9b7d2f4a6c5e8b1d0a9f7e3c2b4d6a8f0e1c3b5d7a9f2e4c6b8d0a1f"
  },
  "korrekturen": []
}
```

**Fehler**

| HTTP | Code | Bemerkung |
|---|---|---|
| 400 | `DATEI_FEHLT` | Feld `datei` fehlt oder Datei ist leer |
| 409 | `DUPLIKAT` | Datei mit gleichem SHA-256-Hash existiert. `details.eintragId` nennt den vorhandenen Eintrag. |
| 413 | `DATEI_ZU_GROSS` | Über 10 MB |
| 415 | `DATEITYP_NICHT_UNTERSTUETZT` | Kein JPEG, PNG, HEIC oder PDF |
| 422 | `NICHT_LESBAR` | Kein Dokument erkennbar, z. B. unscharfes Foto |
| 502 | `AUSLESEN_FEHLGESCHLAGEN` | Antwort des Vision-LLM passt nicht zum erwarteten JSON |
| 503 | `MODELL_NICHT_ERREICHBAR` | Vision-LLM oder Embedding-Modell nicht erreichbar |
| 500 | `INTERNER_FEHLER` | z. B. Datenbank nicht erreichbar |

Beispiel für ein Duplikat. Die App kann mit `details.eintragId` direkt zum
vorhandenen Eintrag springen:

```json
{
  "fehler": {
    "code": "DUPLIKAT",
    "nachricht": "Diese Datei hast du schon hochgeladen.",
    "details": {
      "eintragId": "6727a1f3c2b4e5d6f7a8b9c0"
    }
  }
}
```

### 3.2 Alle Einträge für die Zeitleiste

```http
GET /api/v1/eintraege?sortierung=dokumentdatum&richtung=absteigend
```

**Anfrage**

| Parameter | Werte | Standard |
|---|---|---|
| `sortierung` | `dokumentdatum`, `importdatum` | `dokumentdatum` |
| `richtung` | `absteigend`, `aufsteigend` | `absteigend` |

Beide Parameter sind optional. Ohne Parameter kommt das neueste Dokument
zuerst.

**Sortierregeln**

1. Sortiert wird nach dem gewählten Feld in der gewählten Richtung.
2. Einträge ohne `dokumentdatum` (`null`) stehen bei
   `sortierung=dokumentdatum` immer **am Ende**, egal in welcher Richtung.
   Untereinander sind sie nach `importdatum` absteigend sortiert.
3. Bei gleichem `dokumentdatum` entscheidet `importdatum` in derselben
   Richtung, danach `id`. So ist die Reihenfolge immer eindeutig.

Diese Regeln bekommen im Backend Tests.

**Antwort `200 OK`**

```json
{
  "eintraege": [
    {
      "id": "6727a1f3c2b4e5d6f7a8b9c0",
      "modul": "gesundheit",
      "typ": "laborbefund",
      "dokumentdatum": "2026-10-14",
      "importdatum": "2026-11-03T18:42:07Z",
      "quelle": "Labor Muster GmbH, Salzburg"
    },
    {
      "id": "6727a20bc2b4e5d6f7a8b9d4",
      "modul": "gesundheit",
      "typ": "arztbrief",
      "dokumentdatum": "2026-09-02",
      "importdatum": "2026-11-03T18:50:12Z",
      "quelle": "Dr. Erika Beispiel, Allgemeinmedizin"
    },
    {
      "id": "6727a2a9c2b4e5d6f7a8b9e7",
      "modul": "gesundheit",
      "typ": "sonstiges",
      "dokumentdatum": null,
      "importdatum": "2026-11-03T19:01:44Z",
      "quelle": null
    }
  ],
  "anzahl": 3
}
```

Keine Einträge: `200 OK` mit `"eintraege": []` und `"anzahl": 0`.

Im Prototyp gibt es keine Seitenaufteilung. Die Liste enthält immer alle
Einträge.

**Fehler**

| HTTP | Code | Bemerkung |
|---|---|---|
| 400 | `UNGUELTIGER_PARAMETER` | z. B. `sortierung=quelle` |
| 500 | `INTERNER_FEHLER` | |

```json
{
  "fehler": {
    "code": "UNGUELTIGER_PARAMETER",
    "nachricht": "Unbekannte Sortierung. Erlaubt sind dokumentdatum und importdatum.",
    "details": { "parameter": "sortierung", "wert": "quelle" }
  }
}
```

### 3.3 Einen Eintrag ansehen

```http
GET /api/v1/eintraege/6727a1f3c2b4e5d6f7a8b9c0
```

**Anfrage**

Kein Body. `{id}` ist die ID des Eintrags.

**Antwort `200 OK`**

Der vollständige Eintrag inklusive `volltext` und `korrekturen`, genau wie das
Beispiel in [Abschnitt 2](#vollständiger-eintrag).

**Fehler**

| HTTP | Code | Bemerkung |
|---|---|---|
| 400 | `UNGUELTIGE_ID` | `{id}` ist keine gültige ObjectId |
| 404 | `EINTRAG_NICHT_GEFUNDEN` | |
| 500 | `INTERNER_FEHLER` | |

### 3.4 Originaldatei abrufen

```http
GET /api/v1/eintraege/6727a1f3c2b4e5d6f7a8b9c0/datei
```

Damit zeigt die App das Original neben den erkannten Werten an. So lässt sich
prüfen, ob das Vision-LLM richtig gelesen hat.

**Antwort `200 OK`**

Die Datei als Binärdaten, kein JSON.

```http
Content-Type: application/pdf
Content-Length: 184320
Content-Disposition: inline; filename="befund_oktober.pdf"
```

**Fehler** (als JSON wie in Abschnitt 1)

| HTTP | Code | Bemerkung |
|---|---|---|
| 400 | `UNGUELTIGE_ID` | |
| 404 | `EINTRAG_NICHT_GEFUNDEN` | |
| 500 | `INTERNER_FEHLER` | |

### 3.5 Erkannte Werte korrigieren

```http
PATCH /api/v1/eintraege/6727a1f3c2b4e5d6f7a8b9c0
Content-Type: application/json
```

**Anfrage**

Der Body enthält nur die Felder, die sich ändern sollen. Fehlende Felder
bleiben unverändert.

| Feld | Typ | Bemerkung |
|---|---|---|
| `typ` | String | Einer der erlaubten Typen des Moduls aus Abschnitt 2 |
| `dokumentdatum` | String oder `null` | Format `YYYY-MM-DD`, muss ein gültiges Kalenderdatum sein |
| `quelle` | String oder `null` | |
| `infos` | Liste | Je Element `id` (Pflicht) plus `wert` und/oder `einheit`. `"einheit": null` entfernt die Einheit. |

`volltext` ist im Prototyp nicht korrigierbar und führt zu
`400 FELD_NICHT_KORRIGIERBAR` (Ausbau siehe
[Offene Punkte](#4-offene-punkte)). Neue Infos hinzufügen oder Infos löschen
geht im Prototyp ebenfalls nicht.

```json
{
  "dokumentdatum": "2026-10-15",
  "infos": [
    { "id": "i2", "wert": "6,8" }
  ]
}
```

**Ablauf im Backend**

1. Anfrage prüfen. Ist ein Teil ungültig, ändert das Backend gar nichts.
2. Neue Werte in die Felder schreiben.
3. `korrekturen` nach den Regeln aus Abschnitt 2 pflegen. Ein Feld, dessen
   Wert sich nicht ändert, erzeugt keine Korrektur.

Korrekturen ändern nur `typ`, `dokumentdatum`, `quelle` und `infos`. Die
Abschnitte und Embeddings für die Suche bleiben unverändert, weil sie aus dem
`volltext` entstehen.

**Antwort `200 OK`**

Der aktualisierte vollständige Eintrag. Ausschnitt für das Beispiel oben:

```json
{
  "id": "6727a1f3c2b4e5d6f7a8b9c0",
  "modul": "gesundheit",
  "typ": "laborbefund",
  "dokumentdatum": "2026-10-15",
  "importdatum": "2026-11-03T18:42:07Z",
  "quelle": "Labor Muster GmbH, Salzburg",
  "volltext": "Labor Muster GmbH\nMusterstraße 1, 5020 Salzburg\n\nLaborbefund vom 14.10.2026\nPatient: Max Mustermann, geb. 01.01.1990\n\nHämoglobin 14,2 g/dl (Ref. 13,5-17,5)\nLeukozyten 6,3 G/l (Ref. 4,0-10,0)\nCRP <0,5 mg/dl (Ref. <0,5)\nBefundart: Blutbild klein",
  "infos": [
    { "id": "i1", "name": "Hämoglobin", "wert": "14,2", "einheit": "g/dl" },
    { "id": "i2", "name": "Leukozyten", "wert": "6,8", "einheit": "G/l" },
    { "id": "i3", "name": "CRP", "wert": "<0,5", "einheit": "mg/dl" },
    { "id": "i4", "name": "Befundart", "wert": "Blutbild klein" }
  ],
  "datei": {
    "id": "6727a1f3c2b4e5d6f7a8b9c1",
    "dateiname": "befund_oktober.pdf",
    "mimeTyp": "application/pdf",
    "groesse": 184320,
    "sha256": "3f8a1c0e9b7d2f4a6c5e8b1d0a9f7e3c2b4d6a8f0e1c3b5d7a9f2e4c6b8d0a1f"
  },
  "korrekturen": [
    {
      "feld": "dokumentdatum",
      "erkannt": "2026-10-14",
      "korrigiert": "2026-10-15",
      "zeitpunkt": "2026-11-03T19:05:31Z"
    },
    {
      "feld": "infos.i2.wert",
      "erkannt": "6,3",
      "korrigiert": "6,8",
      "zeitpunkt": "2026-11-03T19:05:31Z"
    }
  ]
}
```

**Fehler**

| HTTP | Code | Bemerkung |
|---|---|---|
| 400 | `UNGUELTIGE_ID` | |
| 400 | `UNGUELTIGE_ANFRAGE` | z. B. `"dokumentdatum": "15.10.2026"`, unbekanntes Feld, unbekannter `typ`, Info ohne `id`, leerer Body |
| 400 | `FELD_NICHT_KORRIGIERBAR` | z. B. `volltext`, `modul`, `importdatum`, `datei` |
| 404 | `EINTRAG_NICHT_GEFUNDEN` | |
| 404 | `INFO_NICHT_GEFUNDEN` | `details.infoId` nennt die unbekannte ID |
| 500 | `INTERNER_FEHLER` | |

```json
{
  "fehler": {
    "code": "UNGUELTIGE_ANFRAGE",
    "nachricht": "Das Dokumentdatum muss im Format JJJJ-MM-TT angegeben werden.",
    "details": { "feld": "dokumentdatum", "wert": "15.10.2026" }
  }
}
```

### 3.6 Frage stellen

```http
POST /api/v1/fragen
Content-Type: application/json
```

**Anfrage**

| Feld | Typ | Bemerkung |
|---|---|---|
| `frage` | String | Pflicht, höchstens 500 Zeichen |

```json
{
  "frage": "Welchen Hämoglobinwert hatte ich im Oktober?"
}
```

**Ablauf im Backend**

1. Embedding der Frage berechnen.
2. Per Atlas Vector Search die passendsten Abschnitte finden.
3. Gibt es keinen passenden Abschnitt, antwortet das Backend ohne Aufruf des
   Sprachmodells mit dem festen Satz (siehe unten).
4. Sonst bekommt das Sprachmodell die Frage und nur diese Abschnitte. Es
   antwortet nur aus diesen Abschnitten und setzt Verweise wie `[1]`.

Die Antwort stützt sich auf den `volltext`. Korrekturen an `infos` ändern sie
nicht.

**Antwort `200 OK` mit Treffer**

```json
{
  "antwort": "Im Laborbefund der Labor Muster GmbH vom 14.10.2026 steht: Hämoglobin 14,2 g/dl [1].",
  "gefunden": true,
  "quellen": [
    {
      "nr": 1,
      "eintragId": "6727a1f3c2b4e5d6f7a8b9c0",
      "typ": "laborbefund",
      "quelle": "Labor Muster GmbH, Salzburg",
      "dokumentdatum": "2026-10-14",
      "auszug": "Hämoglobin 14,2 g/dl (Ref. 13,5-17,5)"
    }
  ]
}
```

| Feld | Typ | Bemerkung |
|---|---|---|
| `antwort` | String | Antworttext. Verweise `[n]` zeigen auf `quellen[].nr`. |
| `gefunden` | Boolean | `false`, wenn kein passender Abschnitt gefunden wurde |
| `quellen` | Liste | Verwendete Abschnitte. Mit `eintragId` öffnet die App den Eintrag. |
| `quellen[].auszug` | String | Der Abschnitt aus dem Volltext, auf den sich die Antwort stützt |

`dokumentdatum` und `quelle` in `quellen` zeigen die aktuellen Werte des
Eintrags, also auch korrigierte.

**Antwort `200 OK` ohne Treffer**

Kein Treffer ist kein Fehler. Die Antwort lautet immer wörtlich:

```json
{
  "antwort": "Dazu finde ich nichts in deinen Dokumenten.",
  "gefunden": false,
  "quellen": []
}
```

**Fehler**

| HTTP | Code | Bemerkung |
|---|---|---|
| 400 | `UNGUELTIGE_ANFRAGE` | Body ist kein gültiges JSON |
| 400 | `FRAGE_FEHLT` | Feld fehlt, ist leer oder nur Leerzeichen |
| 400 | `FRAGE_ZU_LANG` | Über 500 Zeichen |
| 503 | `MODELL_NICHT_ERREICHBAR` | Embedding- oder Sprachmodell nicht erreichbar |
| 500 | `INTERNER_FEHLER` | |

---

## 4. Offene Punkte

Diese Punkte sind für den Prototyp bewusst ausgelassen.

### Volltext korrigieren (späterer Ausbau)

Liest das Vision-LLM den Text falsch, sucht die Suche im falschen Text. Später
soll `PATCH /api/v1/eintraege/{id}` deshalb auch `volltext` annehmen:

```json
{
  "volltext": "Labor Muster GmbH\nMusterstraße 1, 5020 Salzburg\n\nLaborbefund vom 14.10.2026\nPatient: Max Mustermann, geb. 01.01.1990\n\nHämoglobin 14,2 g/dl (Ref. 13,5-17,5)\nLeukozyten 6,8 G/l (Ref. 4,0-10,0)\nCRP <0,5 mg/dl (Ref. <0,5)\nBefundart: Blutbild klein"
}
```

Geplantes Verhalten:

1. Der ursprünglich erkannte Volltext bleibt in `korrekturen` erhalten
   (`"feld": "volltext"`), nach denselben Regeln wie bei den anderen Feldern.
2. Das Backend zerlegt den neuen Text in Abschnitte und berechnet die
   Embeddings neu.
3. Erst wenn alle neuen Embeddings da sind, ersetzt es die alten Abschnitte
   dieses Eintrags. So antwortet die Suche nicht mehr aus dem alten Text, und
   es gibt keinen Zustand, in dem ein Teil fehlt.
4. Ist das Embedding-Modell nicht erreichbar, antwortet das Backend mit
   `503 MODELL_NICHT_ERREICHBAR` und der alte Stand bleibt vollständig
   erhalten.
5. Der Aufruf wird dadurch langsamer als andere Korrekturen. Die App zeigt eine
   Ladeanzeige.

### Weitere Punkte

- **Anmeldung und mehrere Nutzer:** Im Prototyp gibt es einen Nutzer ohne
  Anmeldung. Später braucht jeder Aufruf eine Anmeldung, und jeder Eintrag
  gehört einem Nutzer. Die Duplikat-Prüfung gilt dann pro Nutzer.
- **Asynchroner Upload:** Statt bis zu 60 Sekunden zu warten, könnte
  `POST /eintraege` sofort `202 Accepted` liefern. Die App fragt dann den
  Status ab.
- **Seitenaufteilung** der Zeitleiste, sobald es viele Einträge gibt.
- **Einträge löschen**, inklusive Originaldatei und Abschnitten.
- **Infos hinzufügen und löschen**, falls das Vision-LLM einen Wert übersieht.
- **Filter nach `typ`** in der Zeitleiste.
- **Weitere Module** (Ernährung, Rezepte, Sport): Das Feld `modul` trennt die
  Bereiche. Ein `typ` gilt immer nur innerhalb seines Moduls. So bleibt
  `rezept` in `gesundheit` ein ärztliches Rezept, und ein späteres Modul für
  Kochrezepte kann eigene Typen haben, ohne dass es zu Verwechslungen kommt.
  Noch offen: welches Modul ein Upload bekommt (Auswahl in der App oder
  Erkennung durch das Vision-LLM), ob `modul` korrigierbar wird und ein
  Filter nach `modul` in der Zeitleiste.
- **Duplikate nur bei exakt gleichen Bytes:** Ein neues Foto desselben
  Dokuments hat einen anderen Hash und wird nicht als Duplikat erkannt.
