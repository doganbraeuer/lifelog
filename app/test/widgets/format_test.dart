import 'package:flutter_test/flutter_test.dart';
import 'package:lifelog/widgets/format.dart';

void main() {
  test('Zeitpunkt wird in lokaler Zeit angezeigt', () {
    // Unabhängig von der Zeitzone des Rechners: lokal 19:42 erzeugen,
    // als UTC übergeben, es muss wieder 19:42 herauskommen.
    final lokal = DateTime(2026, 11, 3, 19, 42);
    expect(formatZeitpunktLokal(lokal.toUtc()), '03.11.2026, 19:42');
  });
}
