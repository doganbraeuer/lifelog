import 'package:flutter_test/flutter_test.dart';
import 'package:lifelog/model/plain_date.dart';

void main() {
  group('PlainDate.tryParse', () {
    test('reads YYYY-MM-DD', () {
      final date = PlainDate.tryParse('2026-10-14')!;
      expect(date.year, 2026);
      expect(date.month, 10);
      expect(date.day, 14);
    });

    test('rejects wrong formats', () {
      for (final text in [
        '15.10.2026',
        '2026-1-5',
        '2026/10/14',
        '2026-10-14T00:00:00Z',
        ' 2026-10-14',
        '2026-10-14 ',
        '',
        '20261014',
      ]) {
        expect(PlainDate.tryParse(text), isNull, reason: text);
      }
    });

    test('rejects dates that do not exist in the calendar', () {
      for (final text in [
        '2026-02-30',
        '2026-02-29',
        '1900-02-29',
        '2026-13-01',
        '2026-00-10',
        '2026-04-31',
        '2026-10-00',
        '0000-01-01',
      ]) {
        expect(PlainDate.tryParse(text), isNull, reason: text);
      }
    });

    test('accepts leap days', () {
      expect(PlainDate.tryParse('2024-02-29'), PlainDate(2024, 2, 29));
      expect(PlainDate.tryParse('2000-02-29'), PlainDate(2000, 2, 29));
    });
  });

  test('PlainDate.parse throws FormatException', () {
    expect(() => PlainDate.parse('15.10.2026'), throwsFormatException);
  });

  test('constructor checks the date', () {
    expect(() => PlainDate(2026, 2, 30), throwsArgumentError);
  });

  test('iso and display with leading zeros', () {
    final date = PlainDate(2026, 3, 5);
    expect(date.iso, '2026-03-05');
    expect(date.display, '05.03.2026');
    expect(PlainDate.parse(date.iso), date);
  });

  group('no day shift', () {
    test('PlainDate.fromDateTime takes the fields without time zone conversion', () {
      // Shortly before midnight UTC it would already be the next day in Salzburg.
      // PlainDate.fromDateTime does not convert, it takes year, month and day.
      expect(
        PlainDate.fromDateTime(DateTime.utc(2026, 10, 14, 23, 30)),
        PlainDate(2026, 10, 14),
      );
      expect(
        PlainDate.fromDateTime(DateTime(2026, 10, 14, 0, 5)),
        PlainDate(2026, 10, 14),
      );
    });

    test('round trip via toDateTime keeps the same day', () {
      final date = PlainDate(
        2026,
        3,
        29,
      ); // day of the daylight saving time change
      expect(PlainDate.fromDateTime(date.toDateTime()), date);
    });
  });

  test('comparison and equality', () {
    expect(
      PlainDate(2026, 9, 2).compareTo(PlainDate(2026, 10, 14)),
      lessThan(0),
    );
    expect(PlainDate(2026, 10, 14).compareTo(PlainDate(2026, 10, 14)), 0);
    expect(
      PlainDate(2025, 12, 31).compareTo(PlainDate(2026, 1, 1)),
      lessThan(0),
    );
    expect(PlainDate(2026, 10, 14), PlainDate(2026, 10, 14));
    expect(PlainDate(2026, 10, 14).hashCode, PlainDate(2026, 10, 14).hashCode);
  });
}
