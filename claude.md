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

## Datenmodell: Eintrag
- `typ` (z. B. arztbrief, laborbefund, rezept)
- `dokumentdatum` und `importdatum` (beide speichern)
- `quelle` (ausstellende Stelle)
- `volltext` (der Text des Dokuments)
- `infos`: Liste aus `name`, `wert`, optional `einheit`
- Referenz auf die Originaldatei
- Korrekturen: ursprünglich erkannter und korrigierter Wert bleiben erhalten

## Aktuelles Ziel
Prototyp bis 09.11.2026: Foto oder PDF in der App hochladen → Backend →
Vision-LLM → MongoDB → Zeitleiste in der App → eine Frage mit Quellenangabe.

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