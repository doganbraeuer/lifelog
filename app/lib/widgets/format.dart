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
