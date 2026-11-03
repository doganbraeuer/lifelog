/// Dateigröße für die Anzeige, z. B. `340 KB` oder `2,4 MB`.
String formatGroesse(int bytes) {
  const kb = 1024;
  const mb = 1024 * 1024;
  if (bytes < kb) return '$bytes Bytes';
  if (bytes < mb) return '${(bytes / kb).round()} KB';
  return '${(bytes / mb).toStringAsFixed(1).replaceAll('.', ',')} MB';
}

/// Zeitpunkt in lokaler Zeit des Geräts, z. B. `03.11.2026, 19:42`.
///
/// Nur für `importdatum` und Korrektur-Zeitpunkte. Für `dokumentdatum` gibt
/// es `Datum.anzeige`, das nie in eine Zeitzone umgerechnet wird.
String formatZeitpunktLokal(DateTime zeitpunkt) {
  final lokal = zeitpunkt.toLocal();
  String zwei(int n) => n.toString().padLeft(2, '0');
  return '${zwei(lokal.day)}.${zwei(lokal.month)}.${lokal.year}, '
      '${zwei(lokal.hour)}:${zwei(lokal.minute)}';
}
