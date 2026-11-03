import 'package:flutter_test/flutter_test.dart';
import 'package:lifelog/widgets/format.dart';

void main() {
  test('Dateigröße', () {
    expect(formatGroesse(512), '512 Bytes');
    expect(formatGroesse(340 * 1024), '340 KB');
    expect(formatGroesse((2.4 * 1024 * 1024).round()), '2,4 MB');
    expect(formatGroesse(10 * 1024 * 1024), '10,0 MB');
  });

  test('Zeitpunkt wird in lokaler Zeit angezeigt', () {
    // Unabhängig von der Zeitzone des Rechners: lokal 19:42 erzeugen,
    // als UTC übergeben, es muss wieder 19:42 herauskommen.
    final lokal = DateTime(2026, 11, 3, 19, 42);
    expect(formatZeitpunktLokal(lokal.toUtc()), '03.11.2026, 19:42');
  });
}
