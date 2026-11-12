# Lifelog

## Projekt
Softwarepraktikum WS 2026/27, Universität Salzburg (Prof. Pree), Gruppe 2:
Dogan Bräuer und Dylan Müller.

Lifelog ist eine erweiterbare Plattform zum Einlesen, Strukturieren, Speichern
und Durchsuchen persönlicher Dokumente. Erster Anwendungsfall ist Gesundheit
(Befunde, Arztbriefe, Laborwerte). Später kommen weitere Module dazu, etwa
Ernährung, Rezepte oder Sport. Der Kern muss deshalb unabhängig vom
Dokumenttyp funktionieren.

## Architektur
- `app/`: Flutter-App (Dart) für iOS und Android
- `backend/`: Go-Backend mit REST-API
- Datenbank: MongoDB Atlas (Free Tier) inklusive Atlas Vector Search
- Dokumente liest ein Vision-LLM in der Cloud. Es liefert als JSON:
  Dokumentdatum, ausstellende Stelle, Typ, Volltext und extrahierte Infos.
- Suche (RAG): Volltext in Abschnitte zerlegen, Embeddings in MongoDB
  speichern, per Vektorsuche passende Abschnitte finden. Antworten nur aus
  diesen Abschnitten, immer mit Quellenangabe. Ohne passenden Treffer lautet
  die Antwort: "Dazu finde ich nichts in deinen Dokumenten."
- Aufrufe an Sprachmodell und Embedding-Modell laufen nur über eine
  Schnittstelle im Backend, damit der Anbieter austauschbar bleibt. Die App
  ruft nie direkt ein Modell auf.

## Datenmodell: Eintrag (entry)
Namen nach `docs/GLOSSARY.md`, verbindlich ist `docs/API.md`.
- `module` (Pflichtfeld, z. B. `health`; im Prototyp immer `health`)
- `type` (z. B. `doctor_letter`, `lab_report`, `prescription`; gilt
  innerhalb des Moduls)
- `documentDate` und `importedAt` (beide speichern)
- `issuer` (ausstellende Stelle)
- `fullText` (der Text des Dokuments)
- `facts`: Liste aus `name`, `value`, optional `unit`
- `file`: Referenz auf die Originaldatei
- `corrections`: ursprünglich erkannter und korrigierter Wert bleiben erhalten

## Aktuelles Ziel
Prototyp bis 09.11.2026: Foto oder PDF in der App hochladen → Backend →
Vision-LLM → MongoDB → Zeitleiste in der App → eine Frage mit Quellenangabe.

## Sprache
Vorgabe von Prof. Pree (seit 09.11.2026):
- **Englisch** ist der gesamte Code: Bezeichner, Kommentare, Datei-,
  Ordner- und Paketnamen, Tests, Log-Meldungen, Fehlertexte im Code,
  Ausgaben der Programme in `cmd/` und Commit-Nachrichten. Ebenso die API
  (Pfade, JSON-Felder, Fehlercodes, Modul- und Typcodes), Collections und
  Felder in MongoDB, Umgebungsvariablen, die Befehle in `cmd/` und die
  Dokumentation (`docs/`, READMEs). Prompts an die Modelle sind englisch.
- **Deutsch** bleibt nur, was der Nutzer sieht: Anzeigetexte der App, das
  API-Feld `message`, der feste Satz „Dazu finde ich nichts in deinen
  Dokumenten.“ und die Antworten des Sprachmodells. Ebenso
  Dokumentinhalte (Testdokumente, Beispieldaten, Kalibrierfragen), weil sie
  echte Nutzereingaben nachbilden. Sie werden nicht übersetzt, sonst
  ändern sich die Vektoren und die Kalibrierung.
- Neue Namen zuerst in `docs/GLOSSARY.md` nachschlagen bzw. eintragen,
  damit Backend und App dieselben verwenden.
- Diese Datei (`CLAUDE.md`) und Claudes Antworten bleiben deutsch.

## Regeln
- Antworte auf Deutsch.
- Wir lernen mit diesem Projekt: Erkläre bei jeder Änderung kurz, was sie
  tut und warum.
- Arbeite in kleinen Schritten. Bei größeren Änderungen zuerst einen Plan
  vorlegen.
- Geheimnisse (API-Schlüssel, Datenbankzugang) stehen nur in `.env`, nie im
  Code, und werden nie committet. Neue Variablen in `.env.example` ohne
  echte Werte eintragen.
- Die App gibt keine medizinischen Bewertungen oder Empfehlungen. Sie zeigt
  nur, was in den Dokumenten steht.
- Wir verwenden nur synthetische Testdaten, keine echten Gesundheitsdaten.
- Für deterministische Logik (z. B. Datumsverarbeitung) Tests schreiben.