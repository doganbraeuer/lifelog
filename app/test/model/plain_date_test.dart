import 'package:flutter_test/flutter_test.dart';
import 'package:lifelog/model/plain_date.dart';

void main() {
  group('Datum.tryParse', () {
    test('liest JJJJ-MM-TT', () {
      final datum = PlainDate.tryParse('2026-10-14')!;
      expect(datum.year, 2026);
      expect(datum.month, 10);
      expect(datum.day, 14);
    });

    test('lehnt falsche Formate ab', () {
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

    test('lehnt Daten ab, die es im Kalender nicht gibt', () {
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

    test('akzeptiert Schalttage', () {
      expect(PlainDate.tryParse('2024-02-29'), PlainDate(2024, 2, 29));
      expect(PlainDate.tryParse('2000-02-29'), PlainDate(2000, 2, 29));
    });
  });

  test('Datum.parse wirft FormatException', () {
    expect(() => PlainDate.parse('15.10.2026'), throwsFormatException);
  });

  test('Konstruktor prüft das Datum', () {
    expect(() => PlainDate(2026, 2, 30), throwsArgumentError);
  });

  test('iso und anzeige mit führenden Nullen', () {
    final datum = PlainDate(2026, 3, 5);
    expect(datum.iso, '2026-03-05');
    expect(datum.display, '05.03.2026');
    expect(PlainDate.parse(datum.iso), datum);
  });

  group('kein Tagesversatz', () {
    test('Datum.aus übernimmt die Felder ohne Zeitzonen-Umrechnung', () {
      // Kurz vor Mitternacht UTC wäre es in Salzburg schon der nächste Tag.
      // Datum.aus rechnet nicht um, sondern nimmt Jahr, Monat und Tag.
      expect(
        PlainDate.fromDateTime(DateTime.utc(2026, 10, 14, 23, 30)),
        PlainDate(2026, 10, 14),
      );
      expect(
        PlainDate.fromDateTime(DateTime(2026, 10, 14, 0, 5)),
        PlainDate(2026, 10, 14),
      );
    });

    test('Hin und zurück über alsDateTime bleibt derselbe Tag', () {
      final datum = PlainDate(2026, 3, 29); // Tag der Sommerzeit-Umstellung
      expect(PlainDate.fromDateTime(datum.toDateTime()), datum);
    });
  });

  test('Vergleich und Gleichheit', () {
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
