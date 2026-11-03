// Kleine Helfer zum sicheren Lesen von JSON.
//
// Statt `json['id'] as String` (wirft bei falschem Typ einen TypeError)
// prüfen diese Funktionen den Typ und werfen eine verständliche
// FormatException. Der HttpApiClient fängt sie ab und meldet eine
// unerwartete Antwort des Servers.

import 'datum.dart';

Map<String, dynamic> objekt(Object? wert, String feld) {
  if (wert is Map<String, dynamic>) return wert;
  throw FormatException('"$feld" ist kein JSON-Objekt');
}

String text(Map<String, dynamic> json, String feld) {
  final wert = json[feld];
  if (wert is String) return wert;
  throw FormatException('Feld "$feld" fehlt oder ist kein Text');
}

String? textOderNull(Map<String, dynamic> json, String feld) {
  final wert = json[feld];
  if (wert == null || wert is String) return wert as String?;
  throw FormatException('Feld "$feld" ist weder Text noch null');
}

int ganzzahl(Map<String, dynamic> json, String feld) {
  final wert = json[feld];
  if (wert is int) return wert;
  throw FormatException('Feld "$feld" fehlt oder ist keine ganze Zahl');
}

bool wahrheitswert(Map<String, dynamic> json, String feld) {
  final wert = json[feld];
  if (wert is bool) return wert;
  throw FormatException('Feld "$feld" fehlt oder ist kein Boolean');
}

List<T> liste<T>(
  Map<String, dynamic> json,
  String feld,
  T Function(Map<String, dynamic>) umwandeln,
) {
  final wert = json[feld];
  if (wert is! List) {
    throw FormatException('Feld "$feld" fehlt oder ist keine Liste');
  }
  return [for (final element in wert) umwandeln(objekt(element, feld))];
}

/// Liest einen Zeitpunkt im Format RFC 3339 (z. B. `2026-11-03T18:42:07Z`)
/// und gibt ihn in UTC zurück.
DateTime zeitstempel(Map<String, dynamic> json, String feld) {
  final wert = DateTime.tryParse(text(json, feld));
  if (wert == null) {
    throw FormatException('Feld "$feld" ist kein gültiger Zeitpunkt');
  }
  return wert.toUtc();
}

/// Liest ein reines Datum `JJJJ-MM-TT` oder `null`.
Datum? datumOderNull(Map<String, dynamic> json, String feld) {
  final wert = textOderNull(json, feld);
  if (wert == null) return null;
  return Datum.tryParse(wert) ??
      (throw FormatException('Feld "$feld" ist kein Datum JJJJ-MM-TT', wert));
}

/// Schreibt einen Zeitpunkt als RFC 3339 in UTC ohne Millisekunden,
/// z. B. `2026-11-03T18:42:07Z`, so wie das Backend ihn liefert.
String formatZeitpunkt(DateTime zeit) {
  final utc = zeit.toUtc();
  String zwei(int n) => n.toString().padLeft(2, '0');
  return '${utc.year.toString().padLeft(4, '0')}-${zwei(utc.month)}-'
      '${zwei(utc.day)}T${zwei(utc.hour)}:${zwei(utc.minute)}:'
      '${zwei(utc.second)}Z';
}
