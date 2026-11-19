import 'dart:convert';

/// Minimum length of a new password in Unicode characters (docs/API.md 3.14).
const minPasswordLength = 10;

/// Maximum length of a new password in UTF-8 bytes (bcrypt's limit). An
/// umlaut counts as 2 bytes.
const maxPasswordBytes = 72;

/// Checks a new password in the app before it is sent, with the same rules
/// as the backend (3.14) plus the repetition. Returns a German message for
/// the first rule that is broken, or `null` if everything is fine.
///
/// The backend checks again; its message is shown if it still refuses.
String? checkNewPassword({
  required String current,
  required String next,
  required String repeat,
}) {
  if (current.isEmpty) return 'Bitte gib dein bisheriges Passwort ein.';
  // runes: Unicode characters, as the backend counts them (not UTF-16).
  if (next.runes.length < minPasswordLength) {
    return 'Das neue Passwort muss mindestens $minPasswordLength Zeichen lang '
        'sein.';
  }
  if (utf8.encode(next).length > maxPasswordBytes) {
    return 'Das neue Passwort ist zu lang. Erlaubt sind höchstens '
        '$maxPasswordBytes Byte, Umlaute zählen doppelt.';
  }
  if (next == current) {
    return 'Das neue Passwort muss sich vom bisherigen unterscheiden.';
  }
  if (next != repeat) {
    return 'Die beiden neuen Passwörter stimmen nicht überein.';
  }
  return null;
}

/// Wait after too many failed attempts as a German sentence, in whole
/// minutes rounded up and at least 1: „Bitte warte etwa 1 Minute.“,
/// „Bitte warte etwa 3 Minuten.“
String waitHint(int seconds) {
  final minutes = (seconds / 60).ceil().clamp(1, 1 << 30);
  return minutes == 1
      ? 'Bitte warte etwa 1 Minute.'
      : 'Bitte warte etwa $minutes Minuten.';
}
