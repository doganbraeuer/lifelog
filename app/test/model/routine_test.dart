import 'dart:convert';

import 'package:flutter_test/flutter_test.dart';
import 'package:lifelog/model/entry_patch.dart';
import 'package:lifelog/model/plain_date.dart';
import 'package:lifelog/model/routine.dart';

Map<String, dynamic> _json(String text) =>
    jsonDecode(text) as Map<String, dynamic>;

/// The routine from docs/API.md 3.16 "The routine".
const bloodPressureJson = '''
{
  "id": "6742c0a1b2c3d4e5f6a7b8c9",
  "module": "health",
  "name": "Blutdruck messen",
  "amount": null,
  "unit": null,
  "schedule": {
    "frequency": "daily",
    "interval": 1,
    "times": ["08:00", "20:00"]
  },
  "startDate": "2026-11-18",
  "endDate": "2026-12-01",
  "status": "active",
  "pauses": [],
  "createdAt": "2026-11-18T07:02:11Z",
  "progress": {
    "today": { "done": 1, "skipped": 0, "due": 2, "amountDone": null, "amountTarget": null },
    "week": { "done": 1, "skipped": 0, "due": 14, "amountDone": null, "amountTarget": null },
    "period": { "day": 6, "days": 14, "done": 9, "skipped": 1, "due": 28, "amountDone": null, "amountTarget": null }
  }
}
''';

/// The response of 3.18 for „Wasser“.
const waterJson = '''
{
  "id": "6742c5d7e8f9a0b1c2d3e4f5",
  "module": "health",
  "name": "Wasser",
  "amount": 250,
  "unit": "ml",
  "schedule": { "frequency": "hourly", "interval": 1, "windowStart": "08:00", "windowEnd": "20:00" },
  "startDate": "2026-11-23",
  "endDate": null,
  "status": "active",
  "pauses": [],
  "createdAt": "2026-11-23T08:15:03Z",
  "progress": {
    "today": { "done": 0, "skipped": 0, "due": 13, "amountDone": 0, "amountTarget": 3250 },
    "week": { "done": 0, "skipped": 0, "due": 91, "amountDone": 0, "amountTarget": 22750 },
    "period": null
  }
}
''';

/// The response of 3.24 for one day.
const occurrencesJson = '''
{
  "from": "2026-11-23",
  "to": "2026-11-23",
  "occurrences": [
    { "routineId": "6742bf10a1b2c3d4e5f6a7b8", "dueAt": "2026-11-23T07:00:00Z", "date": "2026-11-23", "time": "08:00", "status": "done", "logId": "6742d2b1c3d4e5f6a7b8c9d0" },
    { "routineId": "6742c0a1b2c3d4e5f6a7b8c9", "dueAt": "2026-11-23T07:00:00Z", "date": "2026-11-23", "time": "08:00", "status": "done", "logId": "6742d3e4f5a6b7c8d9e0f1a2" },
    { "routineId": "6742c0a1b2c3d4e5f6a7b8c9", "dueAt": "2026-11-23T19:00:00Z", "date": "2026-11-23", "time": "20:00", "status": "open", "logId": null }
  ],
  "count": 3
}
''';

/// The log from 3.16 "Logs".
const logJson = '''
{
  "id": "6742d3e4f5a6b7c8d9e0f1a2",
  "routineId": "6742c0a1b2c3d4e5f6a7b8c9",
  "dueAt": "2026-11-23T07:00:00Z",
  "takenAt": "2026-11-23T07:04:12Z",
  "status": "done",
  "amount": null,
  "source": "notification",
  "entryId": null,
  "timeZone": "Europe/Vienna",
  "createdAt": "2026-11-23T07:04:12Z"
}
''';

void main() {
  group('reading', () {
    test('the routine from 3.16 with a period', () {
      final routine = Routine.fromJson(_json(bloodPressureJson));
      expect(routine.id, '6742c0a1b2c3d4e5f6a7b8c9');
      expect(routine.name, 'Blutdruck messen');
      expect(routine.amount, isNull);
      expect(routine.hasAmount, isFalse);
      expect(routine.schedule.frequency, Frequency.daily);
      expect(routine.schedule.times, ['08:00', '20:00']);
      expect(routine.startDate, PlainDate(2026, 11, 18));
      expect(routine.endDate, PlainDate(2026, 12, 1));
      expect(routine.status, RoutineStatus.active);
      expect(routine.pauses, isEmpty);
      expect(routine.createdAt, DateTime.utc(2026, 11, 18, 7, 2, 11));
      expect(routine.progress.today.done, 1);
      expect(routine.progress.today.due, 2);
      expect(routine.progress.week.due, 14);
      expect(routine.progress.period!.day, 6);
      expect(routine.progress.period!.days, 14);
      expect(routine.progress.period!.skipped, 1);
    });

    test('an hourly routine with an amount and without a period', () {
      final routine = Routine.fromJson(_json(waterJson));
      expect(routine.amount, 250);
      expect(routine.unit, 'ml');
      expect(routine.schedule.frequency, Frequency.hourly);
      expect(routine.schedule.windowStart, '08:00');
      expect(routine.schedule.windowEnd, '20:00');
      expect(routine.schedule.times, isEmpty);
      expect(routine.endDate, isNull);
      expect(routine.progress.today.amountTarget, 3250);
      expect(routine.progress.today.amountDone, 0);
      expect(routine.progress.period, isNull);
    });

    test('a weekly schedule and pauses', () {
      final json = _json(bloodPressureJson)
        ..['schedule'] = {
          'frequency': 'weekly',
          'interval': 2,
          'weekdays': ['tue', 'fri'],
          'times': ['07:00'],
        }
        ..['status'] = 'paused'
        ..['pauses'] = [
          {'from': '2026-11-02', 'until': '2026-11-08'},
          {'from': '2026-11-23', 'until': null},
        ];
      final routine = Routine.fromJson(json);
      expect(routine.schedule.weekdays, [Weekday.tue, Weekday.fri]);
      expect(routine.schedule.interval, 2);
      expect(routine.status, RoutineStatus.paused);
      expect(routine.pauses[0].until, PlainDate(2026, 11, 8));
      expect(routine.pauses[1].until, isNull);
      expect(routine.pauseOn(PlainDate(2026, 11, 5)), same(routine.pauses[0]));
      expect(routine.pauseOn(PlainDate(2026, 11, 25)), same(routine.pauses[1]));
      expect(routine.pauseOn(PlainDate(2026, 11, 15)), isNull);
    });

    test('the longest of two overlapping pauses counts', () {
      final json = _json(bloodPressureJson)
        ..['pauses'] = [
          {'from': '2026-11-20', 'until': '2026-11-25'},
          {'from': '2026-11-21', 'until': '2026-11-30'},
        ];
      final routine = Routine.fromJson(json);
      expect(
        routine.pauseOn(PlainDate(2026, 11, 22))!.until,
        PlainDate(2026, 11, 30),
      );
    });

    test('occurrences from 3.24', () {
      final list = OccurrenceList.fromJson(_json(occurrencesJson));
      expect(list.from, PlainDate(2026, 11, 23));
      expect(list.occurrences, hasLength(3));
      final open = list.occurrences.last;
      expect(open.dueAt, DateTime.utc(2026, 11, 23, 19));
      expect(open.date, PlainDate(2026, 11, 23));
      expect(open.time, '20:00');
      expect(open.status, OccurrenceStatus.open);
      expect(open.logId, isNull);
      expect(open.isOverdue(DateTime.utc(2026, 11, 23, 19, 1)), isTrue);
      expect(open.isOverdue(DateTime.utc(2026, 11, 23, 18, 59)), isFalse);
      expect(list.of('6742c0a1b2c3d4e5f6a7b8c9'), hasLength(2));
      expect(list.occurrences.first.isOverdue(DateTime.utc(2027)), isFalse);
    });

    test('a log, also without an occurrence', () {
      final log = RoutineLog.fromJson(_json(logJson));
      expect(log.dueAt, DateTime.utc(2026, 11, 23, 7));
      expect(log.takenAt, DateTime.utc(2026, 11, 23, 7, 4, 12));
      expect(log.status, LogStatus.done);
      expect(log.source, LogSource.notification);
      expect(log.amount, isNull);
      expect(log.timeZone, 'Europe/Vienna');

      final extra = RoutineLog.fromJson(
        _json(logJson)
          ..['dueAt'] = null
          ..['amount'] = 0.5
          ..['source'] = 'manual',
      );
      expect(extra.dueAt, isNull);
      expect(extra.amount, 0.5);
      expect(extra.source, LogSource.manual);
    });

    test('unknown values and missing fields throw FormatException', () {
      expect(
        () => Routine.fromJson(_json(bloodPressureJson)..['status'] = 'gone'),
        throwsFormatException,
      );
      expect(
        () => Routine.fromJson(
          _json(bloodPressureJson)
            ..['schedule'] = {'frequency': 'monthly', 'interval': 1},
        ),
        throwsFormatException,
      );
      expect(
        () => Routine.fromJson(
          _json(bloodPressureJson)
            ..['schedule'] = {
              'frequency': 'weekly',
              'interval': 1,
              'weekdays': ['montag'],
              'times': ['08:00'],
            },
        ),
        throwsFormatException,
      );
      expect(
        () => Routine.fromJson(_json(bloodPressureJson)..remove('startDate')),
        throwsFormatException,
      );
      expect(
        () => Routine.fromJson(_json(bloodPressureJson)..['pauses'] = null),
        throwsFormatException,
      );
      expect(
        () => RoutineLog.fromJson(_json(logJson)..['status'] = 'open'),
        throwsFormatException,
      );
    });

    test('toString leaves name, amount and unit out', () {
      final routine = Routine.fromJson(_json(waterJson));
      final log = RoutineLog.fromJson(_json(logJson)..['amount'] = 250);
      for (final text in [
        '$routine',
        '$log',
        '${routine.schedule}',
        '${OccurrenceList.fromJson(_json(occurrencesJson)).occurrences.first}',
      ]) {
        expect(text, isNot(contains('Wasser')));
        expect(text, isNot(contains('250')));
        expect(text, isNot(contains('ml')));
        expect(text, isNot(contains('07:04')));
      }
    });
  });

  group('requests', () {
    test('a new routine sends only the fields of its frequency', () {
      final hourly = NewRoutine(
        module: 'health',
        name: 'Wasser',
        amount: 250,
        unit: 'ml',
        schedule: const RoutineSchedule.hourly(
          interval: 1,
          windowStart: '08:00',
          windowEnd: '20:00',
        ),
        startDate: PlainDate(2026, 11, 23),
      );
      expect(hourly.toJson(), {
        'module': 'health',
        'name': 'Wasser',
        'amount': 250,
        'unit': 'ml',
        'schedule': {
          'frequency': 'hourly',
          'interval': 1,
          'windowStart': '08:00',
          'windowEnd': '20:00',
        },
        'startDate': '2026-11-23',
      });
      expect(hourly.toJson()['amount'], isA<int>());

      final weekly = NewRoutine(
        module: 'health',
        name: 'Eisbaden',
        schedule: const RoutineSchedule.weekly(
          interval: 1,
          weekdays: [Weekday.tue, Weekday.fri],
          times: ['07:00'],
        ),
        startDate: PlainDate(2026, 11, 23),
        endDate: PlainDate(2026, 12, 31),
      );
      expect(weekly.toJson(), {
        'module': 'health',
        'name': 'Eisbaden',
        'schedule': {
          'frequency': 'weekly',
          'interval': 1,
          'weekdays': ['tue', 'fri'],
          'times': ['07:00'],
        },
        'startDate': '2026-11-23',
        'endDate': '2026-12-31',
      });

      final daily = const RoutineSchedule.daily(interval: 2, times: ['08:00']);
      expect(daily.toJson(), {
        'frequency': 'daily',
        'interval': 2,
        'times': ['08:00'],
      });
    });

    test('schedules compare by value', () {
      expect(
        const RoutineSchedule.daily(interval: 1, times: ['08:00', '20:00']),
        RoutineSchedule.daily(interval: 1, times: ['08:00', '20:00'].toList()),
      );
      expect(
        const RoutineSchedule.daily(interval: 1, times: ['08:00']) ==
            const RoutineSchedule.daily(interval: 1, times: ['08:30']),
        isFalse,
      );
      expect(
        const RoutineSchedule.daily(interval: 1, times: ['07:00']) ==
            const RoutineSchedule.weekly(
              interval: 1,
              weekdays: [Weekday.mon],
              times: ['07:00'],
            ),
        isFalse,
      );
    });

    test('a patch tells a missing field from null', () {
      final patch = RoutinePatch(
        amount: const Value(null),
        endDate: Value(PlainDate(2026, 12, 7)),
      );
      expect(patch.toJson(), {'amount': null, 'endDate': '2026-12-07'});
      expect(patch.toJson().containsKey('unit'), isFalse);
      expect(const RoutinePatch(endDate: Value(null)).toJson(), {
        'endDate': null,
      });
      expect(const RoutinePatch().isEmpty, isTrue);
      expect('$patch', 'RoutinePatch(amount, endDate)');
    });

    test('the patch between a routine and its edited form', () {
      final routine = Routine.fromJson(_json(waterJson));
      NewRoutine edited({
        String name = 'Wasser',
        double? amount = 250,
        String? unit = 'ml',
        RoutineSchedule schedule = const RoutineSchedule.hourly(
          interval: 1,
          windowStart: '08:00',
          windowEnd: '20:00',
        ),
        PlainDate? endDate,
      }) => NewRoutine(
        module: 'health',
        name: name,
        amount: amount,
        unit: unit,
        schedule: schedule,
        startDate: PlainDate(2026, 11, 23),
        endDate: endDate,
      );

      // Unchanged: nothing to send.
      expect(RoutinePatch.between(routine, edited()).isEmpty, isTrue);

      // Only the changed fields.
      expect(
        RoutinePatch.between(
          routine,
          edited(
            name: 'Wasser trinken',
            schedule: const RoutineSchedule.hourly(
              interval: 2,
              windowStart: '08:00',
              windowEnd: '20:00',
            ),
            endDate: PlainDate(2026, 12, 7),
          ),
        ).toJson(),
        {
          'name': 'Wasser trinken',
          'schedule': {
            'frequency': 'hourly',
            'interval': 2,
            'windowStart': '08:00',
            'windowEnd': '20:00',
          },
          'endDate': '2026-12-07',
        },
      );

      // Removing the amount removes the unit as well; it is not sent.
      expect(
        RoutinePatch.between(
          routine,
          edited(amount: null, unit: null),
        ).toJson(),
        {'amount': null},
      );

      // Only the unit.
      expect(RoutinePatch.between(routine, edited(unit: null)).toJson(), {
        'unit': null,
      });
    });

    test('a new log never sends takenAt', () {
      final log = NewRoutineLog(
        status: LogStatus.done,
        source: LogSource.notification,
        dueAt: DateTime.utc(2026, 11, 23, 7),
      );
      expect(log.toJson(), {
        'status': 'done',
        'source': 'notification',
        'dueAt': '2026-11-23T07:00:00Z',
      });
      expect(
        const NewRoutineLog(
          status: LogStatus.done,
          source: LogSource.manual,
          amount: 0.5,
        ).toJson(),
        {'status': 'done', 'source': 'manual', 'amount': 0.5},
      );
    });
  });
}
