/// Version of the consent notice below. The app sends it with
/// `PUT /me/consent` and compares it with `consentVersion` from `GET /me`;
/// if they differ, the notice is shown again (docs/API.md 3.15).
///
/// Change the version whenever the text changes in substance.
const consentVersion = '2026-11-23';

/// Heading of the consent notice. German on purpose: the user reads it.
const consentTitle = 'Bevor du startest';

/// Paragraphs of the consent notice, version [consentVersion]. Each entry
/// is one paragraph; an optional lead-in ends with a colon and is shown in
/// bold.
const consentParagraphs = [
  'Lifelog ist eine Testversion aus einem Softwarepraktikum an der '
      'Universität Salzburg (Dylan Müller und Dogan Bräuer, betreut von '
      'Prof. Pree). Es kann Fehler geben. Die Antworten der KI können falsch '
      'sein und sind kein ärztlicher Rat.',
  'Was mit deinen Dokumenten passiert: Was du hochlädst, eintippst oder '
      'einsprichst, wird auf unserem Server beim Anbieter netcup in '
      'Deutschland verarbeitet und in einer Datenbank bei MongoDB Atlas in '
      'Frankfurt gespeichert. Zum Auslesen und für Antworten schicken wir den '
      'Inhalt an Google Gemini, bei Sprachnotizen auch die Tonaufnahme '
      'selbst, damit sie in Text umgewandelt wird; die Aufnahme speichern wir '
      'nicht. Wir nutzen die bezahlte Version: Google verwendet die Inhalte '
      'dort nicht zum Verbessern seiner Produkte, speichert sie aber für '
      'begrenzte Zeit.',
  'Wer deine Daten sehen kann: Andere Nutzer sehen deine Dokumente nie. '
      'Technisch haben nur Dylan und Dogan Zugriff auf Server und Datenbank, '
      'dazu die genannten Dienste. Wir haben vereinbart, nicht in deine '
      'Daten zu schauen. Falls das für die Fehlersuche nötig wäre, fragen wir '
      'dich vorher.',
  'Wie lange: Nach der Testphase löschen wir alles, außer du möchtest dein '
      'Konto behalten. Es gibt keine Sicherungskopie. Bei einem technischen '
      'Fehler können Daten verloren gehen.',
  'Widerruf: Du kannst jederzeit widerrufen. Schreib an '
      'dylan.mueller@stud.plus.ac.at, dann löschen wir dein Konto mit allen '
      'Daten.',
  'Gesundheitsdaten sind besonders geschützt. Mit „Zustimmen“ willigst du '
      'ausdrücklich ein, dass wir sie für diesen Test so verarbeiten.',
];
