import 'package:flutter_test/flutter_test.dart';
import 'package:lifelog/model/plain_date.dart';
import 'package:lifelog/screens/correction_dialogs.dart';

void main() {
  group('initialPickerDate', () {
    final today = DateTime(2026, 11, 4);

    test('takes the current document date', () {
      expect(
        initialPickerDate(PlainDate(2026, 10, 14), today),
        DateTime(2026, 10, 14),
      );
    });

    test('without a date: today', () {
      expect(initialPickerDate(null, today), today);
    });

    test('outside the range (wrongly recognized year): today', () {
      expect(initialPickerDate(PlainDate(1026, 10, 14), today), today);
      expect(initialPickerDate(PlainDate(1899, 12, 31), today), today);
      expect(initialPickerDate(PlainDate(2101, 1, 1), today), today);
      expect(initialPickerDate(PlainDate(3026, 10, 14), today), today);
    });

    test('the limits are still included', () {
      expect(initialPickerDate(PlainDate(1900, 1, 1), today), DateTime(1900));
      expect(
        initialPickerDate(PlainDate(2100, 12, 31), today),
        DateTime(2100, 12, 31),
      );
    });
  });
}
