import 'package:flutter_test/flutter_test.dart';
import 'package:lifelog/model/plain_date.dart';
import 'package:lifelog/screens/correction_dialogs.dart';

void main() {
  group('startdatumFuerAuswahl', () {
    final heute = DateTime(2026, 11, 4);

    test('nimmt das aktuelle Dokumentdatum', () {
      expect(
        initialPickerDate(PlainDate(2026, 10, 14), heute),
        DateTime(2026, 10, 14),
      );
    });

    test('ohne Datum: heute', () {
      expect(initialPickerDate(null, heute), heute);
    });

    test('außerhalb des Bereichs (falsch erkanntes Jahr): heute', () {
      expect(initialPickerDate(PlainDate(1026, 10, 14), heute), heute);
      expect(initialPickerDate(PlainDate(1899, 12, 31), heute), heute);
      expect(initialPickerDate(PlainDate(2101, 1, 1), heute), heute);
      expect(initialPickerDate(PlainDate(3026, 10, 14), heute), heute);
    });

    test('Grenzen gehören noch dazu', () {
      expect(initialPickerDate(PlainDate(1900, 1, 1), heute), DateTime(1900));
      expect(
        initialPickerDate(PlainDate(2100, 12, 31), heute),
        DateTime(2100, 12, 31),
      );
    });
  });
}
