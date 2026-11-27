import 'package:flutter_test/flutter_test.dart';
import 'package:lifelog/model/plain_date.dart';
import 'package:lifelog/model/routine.dart';
import 'package:lifelog/model/routine_texts.dart';

void main() {
  group('formatAmount', () {
    for (final (value, text) in [
      (1250.0, '1.250'),
      (3250.0, '3.250'),
      (100000.0, '100.000'),
      (2.0, '2'),
      (0.5, '0,5'),
      (0.3, '0,3'),
      (1234.5, '1.234,5'),
      (0.000001, '0,000001'),
      (12.0000001, '12'),
      (999.0, '999'),
      (1000.0, '1.000'),
      (0.0, '0'),
    ]) {
      test('$value → $text', () => expect(formatAmount(value), text));
    }

    test('with a unit', () {
      expect(amountWithUnit(2, 'Stück'), '2 Stück');
      expect(amountWithUnit(250, 'ml'), '250 ml');
      expect(amountWithUnit(20, null), '20');
    });
  });

  group('parseAmount', () {
    for (final (text, value) in [
      ('2', 2.0),
      (' 250 ', 250.0),
      ('0,5', 0.5),
      ('0.5', 0.5),
      ('1.25', 1.25),
      ('1.250', 1250.0),
      ('12.500', 12500.0),
      ('1.250,5', 1250.5),
      ('0.500', 0.5),
      ('100000', 100000.0),
      ('1 000', 1000.0),
    ]) {
      test('„$text“ → $value', () => expect(parseAmount(text), value));
    }

    for (final text in [
      '',
      'zwei',
      '1,2,3',
      '1.2.3',
      ',5',
      '5,',
      '-1',
      '1e3',
    ]) {
      test('„$text“ is no amount', () => expect(parseAmount(text), isNull));
    }
  });

  group('describeSchedule', () {
    for (final (schedule, text) in [
      (
        const RoutineSchedule.daily(interval: 1, times: ['08:00']),
        'täglich um 08:00',
      ),
      (
        const RoutineSchedule.daily(interval: 1, times: ['08:00', '20:00']),
        'täglich um 08:00 und 20:00',
      ),
      (
        const RoutineSchedule.daily(
          interval: 1,
          times: ['07:00', '12:00', '19:00'],
        ),
        'täglich um 07:00, 12:00 und 19:00',
      ),
      (
        const RoutineSchedule.daily(interval: 2, times: ['08:00']),
        'jeden 2. Tag um 08:00',
      ),
      (
        const RoutineSchedule.hourly(
          interval: 1,
          windowStart: '08:00',
          windowEnd: '20:00',
        ),
        'stündlich von 08:00 bis 20:00',
      ),
      (
        const RoutineSchedule.hourly(
          interval: 3,
          windowStart: '08:00',
          windowEnd: '20:00',
        ),
        'alle 3 Stunden von 08:00 bis 20:00',
      ),
      (
        const RoutineSchedule.weekly(
          interval: 1,
          weekdays: [Weekday.tue, Weekday.fri],
          times: ['07:00'],
        ),
        'wöchentlich am Di und Fr um 07:00',
      ),
      (
        const RoutineSchedule.weekly(
          interval: 2,
          weekdays: [Weekday.mon, Weekday.wed, Weekday.sun],
          times: ['07:00', '19:30'],
        ),
        'jede 2. Woche am Mo, Mi und So um 07:00 und 19:30',
      ),
    ]) {
      test(text, () => expect(describeSchedule(schedule), text));
    }

    test('interval labels for the form', () {
      expect(intervalLabel(Frequency.hourly, 1), 'jede Stunde');
      expect(intervalLabel(Frequency.hourly, 2), 'alle 2 Stunden');
      expect(intervalLabel(Frequency.daily, 1), 'jeden Tag');
      expect(intervalLabel(Frequency.daily, 3), 'jeden 3. Tag');
      expect(intervalLabel(Frequency.weekly, 1), 'jede Woche');
      expect(intervalLabel(Frequency.weekly, 2), 'jede 2. Woche');
    });

    test('the preview with start and end', () {
      const schedule = RoutineSchedule.daily(
        interval: 1,
        times: ['08:00', '20:00'],
      );
      final today = PlainDate(2026, 11, 23);
      expect(
        describeRoutinePlan(
          schedule,
          today,
          PlainDate(2026, 12, 1),
          today: today,
        ),
        'Täglich um 08:00 und 20:00, bis 01.12.2026',
      );
      expect(
        describeRoutinePlan(
          schedule,
          PlainDate(2026, 11, 25),
          null,
          today: today,
        ),
        'Täglich um 08:00 und 20:00, ab 25.11.2026',
      );
    });
  });

  group('progress texts', () {
    test('counts, amounts and the period', () {
      const today = ProgressCounts(
        done: 5,
        skipped: 0,
        due: 13,
        amountDone: 1250,
        amountTarget: 3250,
      );
      expect(countText('heute', today), 'heute 5 von 13');
      expect(amountText('heute', today, 'ml'), 'heute 1.250 von 3.250 ml');
      expect(amountText('heute', today, null), 'heute 1.250 von 3.250');
      const noAmount = ProgressCounts(done: 1, skipped: 0, due: 2);
      expect(amountText('heute', noAmount, 'ml'), isNull);
      expect(countText('Woche', noAmount), 'Woche 1 von 2');
      const period = ProgressCounts(
        done: 9,
        skipped: 1,
        due: 28,
        day: 6,
        days: 14,
      );
      expect(periodDayText(period), 'Tag 6 von 14');
    });
  });

  group('labels and dates', () {
    test('status labels from the glossary', () {
      expect(
        [for (final s in RoutineStatus.values) routineStatusLabel(s)],
        ['geplant', 'aktiv', 'pausiert', 'beendet'],
      );
      expect(
        [for (final s in OccurrenceStatus.values) occurrenceStatusLabel(s)],
        ['offen', 'erledigt', 'ausgelassen', 'pausiert'],
      );
    });

    test('a date with its weekday', () {
      expect(weekdayDate(PlainDate(2026, 11, 27)), 'Fr, 27.11.');
      expect(weekdayDate(PlainDate(2026, 11, 23)), 'Mo, 23.11.');
      expect(weekdayDate(PlainDate(2026, 11, 29)), 'So, 29.11.');
    });

    test('when an occurrence is, relative to today', () {
      Occurrence at(PlainDate date) => Occurrence(
        routineId: 'r',
        dueAt: DateTime.utc(2026),
        date: date,
        time: '07:00',
        status: OccurrenceStatus.open,
        logId: null,
      );
      final today = PlainDate(2026, 11, 30);
      expect(occurrenceWhen(at(today), today), '07:00');
      expect(
        occurrenceWhen(at(PlainDate(2026, 12, 1)), today),
        'morgen, 07:00',
      );
      expect(
        occurrenceWhen(at(PlainDate(2026, 12, 4)), today),
        'Fr, 04.12., 07:00',
      );
    });

    test('lists in German', () {
      expect(germanList([]), '');
      expect(germanList(['a']), 'a');
      expect(germanList(['a', 'b']), 'a und b');
      expect(germanList(['a', 'b', 'c']), 'a, b und c');
    });
  });
}
