// Routines in the mock, as in docs/API.md 3.16–3.27. The progress cases are
// those of backend/internal/core/routine_progress_test.go, so that mock and
// backend are checked against the same numbers.
import 'package:flutter_test/flutter_test.dart';
import 'package:lifelog/api/api_exception.dart';
import 'package:lifelog/api/mock_api_client.dart';
import 'package:lifelog/model/entry_patch.dart';
import 'package:lifelog/model/plain_date.dart';
import 'package:lifelog/model/routine.dart';

import '../helpers.dart';

/// The mock clock of a test; tests move it.
var now = DateTime.utc(2026, 11, 23, 12);

MockApiClient emptyMock() => fastMock(withSampleData: false, clock: () => now);

Future<ApiException> errorOf(Future<Object?> call) async {
  try {
    await call;
  } on ApiException catch (e) {
    return e;
  }
  throw StateError('no error');
}

const daily0800 = {
  'frequency': 'daily',
  'interval': 1,
  'times': ['08:00'],
};

Map<String, dynamic> body({
  String name = 'Vitamin D',
  Object? schedule = daily0800,
  String startDate = '2026-11-18',
  Map<String, dynamic> extra = const {},
}) => {'name': name, 'schedule': schedule, 'startDate': startDate, ...extra};

/// A log for the occurrence at [dueAt], taken two minutes later.
Map<String, dynamic> occurrenceLog(
  String dueAt, {
  String status = 'done',
  num? amount,
}) => {
  'status': status,
  'source': 'manual',
  'dueAt': dueAt,
  'takenAt': DateTime.parse(dueAt)
      .add(const Duration(minutes: 2))
      .toIso8601String(),
  'amount': ?amount,
};

/// A done log without an occurrence.
Map<String, dynamic> freeLog(String takenAt, {num? amount}) => {
  'status': 'done',
  'source': 'manual',
  'takenAt': takenAt,
  'amount': ?amount,
};

/// done, skipped, due, amountDone, amountTarget.
(int, int, int, double?, double?) counts(ProgressCounts p) =>
    (p.done, p.skipped, p.due, p.amountDone, p.amountTarget);

void main() {
  setUp(() => now = DateTime.utc(2026, 11, 23, 12));

  group('create, list, get', () {
    test('a new routine with status and progress (3.18)', () async {
      now = DateTime.utc(2026, 11, 23, 8, 15, 3);
      final mock = emptyMock();
      final water = await mock.createRoutineFromJson({
        'name': '  Wasser ',
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
      expect(water.name, 'Wasser');
      expect(water.module, 'health');
      expect(water.status, RoutineStatus.active);
      expect(water.createdAt, DateTime.utc(2026, 11, 23, 8, 15, 3));
      expect(counts(water.progress.today), (0, 0, 13, 0.0, 3250.0));
      expect(counts(water.progress.week), (0, 0, 91, 0.0, 22750.0));
      expect(water.progress.period, isNull);
      expect((await mock.getRoutine(water.id)).name, 'Wasser');
    });

    test('times and weekdays are stored sorted', () async {
      final mock = emptyMock();
      final r = await mock.createRoutineFromJson(
        body(
          schedule: {
            'frequency': 'weekly',
            'interval': 1,
            'weekdays': ['fri', 'mon'],
            'times': ['20:00', '07:30'],
          },
        ),
      );
      expect(r.schedule.weekdays, [Weekday.mon, Weekday.fri]);
      expect(r.schedule.times, ['07:30', '20:00']);
    });

    test('the list is sorted by createdAt, then id', () async {
      final mock = emptyMock();
      for (final name in ['A', 'B', 'C']) {
        await mock.createRoutineFromJson(body(name: name));
      }
      now = DateTime.utc(2026, 11, 22); // created "earlier"
      await mock.createRoutineFromJson(body(name: 'D'));
      final list = await mock.listRoutines(module: 'health');
      expect([for (final r in list) r.name], ['D', 'A', 'B', 'C']);
    });

    test('sample data has three routines', () async {
      final mock = fastMock(clock: () => now);
      final list = await mock.listRoutines(module: 'health');
      expect(
        [for (final r in list) r.name],
        ['Vitamin D', 'Blutdruck messen', 'Wasser'],
      );
      // 12:00 UTC is after 08:00 in Vienna: Vitamin D is done today.
      expect(list.first.progress.today.done, 1);
    });

    test('unknown module in the list: 400 INVALID_PARAMETER', () async {
      final error = await errorOf(emptyMock().listRoutines(module: 'garden'));
      expect(error.code, ErrorCode.invalidParameter);
      expect(
        error.message,
        'Unbekanntes Modul. Im Prototyp gibt es nur health.',
      );
      expect(error.details, {'parameter': 'module', 'value': 'garden'});
    });

    test('invalid ID first, then 404', () async {
      final mock = emptyMock();
      final invalid = await errorOf(mock.getRoutine('123'));
      expect(invalid.code, ErrorCode.invalidId);
      expect(invalid.message, 'Diese Routinen-ID ist ungültig.');
      final missing = await errorOf(
        mock.getRoutine('6742c0a1b2c3d4e5f6a7b8c9'),
      );
      expect(missing.code, ErrorCode.routineNotFound);
      expect(missing.httpStatus, 404);
      expect(missing.message, 'Diese Routine gibt es nicht.');
    });
  });

  group('rules and messages of 3.18', () {
    for (final (title, request, field, message, value)
        in <(String, Map<String, dynamic>?, String?, String, Object?)>[
          (
            'not an object',
            null,
            null,
            'Bitte die Routine als JSON-Objekt schicken.',
            null,
          ),
          (
            'unknown field before everything',
            body(name: '', extra: {'color': 'rot'}),
            'color',
            'Unbekanntes Feld: color.',
            null,
          ),
          (
            'module',
            body(extra: {'module': 'garden'}),
            'module',
            'Unbekanntes Modul. Im Prototyp gibt es nur health.',
            'garden',
          ),
          (
            'empty name',
            body(name: '   '),
            'name',
            'Der Name muss 1 bis 60 Zeichen lang sein.',
            '   ',
          ),
          (
            'long name',
            body(name: 'x' * 61),
            'name',
            'Der Name muss 1 bis 60 Zeichen lang sein.',
            'x' * 61,
          ),
          (
            'amount 0',
            body(extra: {'amount': 0}),
            'amount',
            'Die Menge muss größer als 0 und höchstens 100.000 sein.',
            0,
          ),
          (
            'amount too large',
            body(extra: {'amount': 100001}),
            'amount',
            'Die Menge muss größer als 0 und höchstens 100.000 sein.',
            100001,
          ),
          (
            'unit without amount',
            body(extra: {'unit': 'ml'}),
            'unit',
            'Eine Einheit braucht eine Menge.',
            'ml',
          ),
          (
            'long unit',
            body(extra: {'amount': 1, 'unit': 'u' * 21}),
            'unit',
            'Die Einheit darf höchstens 20 Zeichen lang sein.',
            'u' * 21,
          ),
          (
            'unit not a text',
            body(extra: {'amount': 1, 'unit': 5}),
            'unit',
            'Die Einheit muss ein Text sein.',
            5,
          ),
          (
            'no schedule',
            body(schedule: null),
            'schedule',
            'Bitte einen Rhythmus angeben.',
            null,
          ),
          (
            'unknown frequency',
            body(schedule: {'frequency': 'monthly', 'interval': 1}),
            'schedule.frequency',
            'Unbekannter Rhythmus.',
            'monthly',
          ),
          (
            'unknown schedule field',
            body(schedule: {...daily0800, 'every': 2}),
            'schedule.every',
            'Unbekanntes Feld: schedule.every.',
            null,
          ),
          (
            'field of another frequency',
            body(
              schedule: {
                'frequency': 'hourly',
                'interval': 1,
                'times': ['08:00'],
                'windowStart': '08:00',
                'windowEnd': '20:00',
              },
            ),
            'schedule.times',
            'Dieses Feld passt nicht zum gewählten Rhythmus.',
            null,
          ),
          (
            'interval 1.5',
            body(schedule: {...daily0800, 'interval': 1.5}),
            'schedule.interval',
            'Das Intervall muss zwischen 1 und 365 liegen.',
            1.5,
          ),
          (
            'hourly interval 24',
            body(
              schedule: {
                'frequency': 'hourly',
                'interval': 24,
                'windowStart': '08:00',
                'windowEnd': '20:00',
              },
            ),
            'schedule.interval',
            'Das Intervall muss zwischen 1 und 23 liegen.',
            24,
          ),
          (
            'time "8 Uhr" (example of 3.18)',
            body(
              schedule: {
                ...daily0800,
                'times': ['8 Uhr'],
              },
            ),
            'schedule.times',
            'Uhrzeiten bitte im Format HH:MM angeben, z. B. 08:00.',
            '8 Uhr',
          ),
          (
            'time 24:00',
            body(
              schedule: {
                ...daily0800,
                'times': ['24:00'],
              },
            ),
            'schedule.times',
            'Uhrzeiten bitte im Format HH:MM angeben, z. B. 08:00.',
            '24:00',
          ),
          (
            'no times',
            body(schedule: {...daily0800, 'times': <String>[]}),
            'schedule.times',
            'Bitte mindestens eine Uhrzeit angeben.',
            null,
          ),
          (
            '25 times',
            body(
              schedule: {
                ...daily0800,
                'times': [
                  for (var h = 0; h < 25; h++)
                    '${(h % 24).toString().padLeft(2, '0')}:0${h ~/ 24}',
                ],
              },
            ),
            'schedule.times',
            'Höchstens 24 Uhrzeiten pro Tag.',
            null,
          ),
          (
            'a time twice',
            body(
              schedule: {
                ...daily0800,
                'times': ['08:00', '08:00'],
              },
            ),
            'schedule.times',
            'Jede Uhrzeit nur einmal.',
            '08:00',
          ),
          (
            'window order',
            body(
              schedule: {
                'frequency': 'hourly',
                'interval': 1,
                'windowStart': '20:00',
                'windowEnd': '08:00',
              },
            ),
            'schedule.windowEnd',
            'Das Ende des Zeitfensters muss nach dem Beginn liegen.',
            '08:00',
          ),
          (
            'no weekdays',
            body(
              schedule: {
                'frequency': 'weekly',
                'interval': 1,
                'weekdays': <String>[],
                'times': ['07:00'],
              },
            ),
            'schedule.weekdays',
            'Bitte mindestens einen Wochentag wählen.',
            <String>[],
          ),
          (
            'unknown weekday',
            body(
              schedule: {
                'frequency': 'weekly',
                'interval': 1,
                'weekdays': ['montag'],
                'times': ['07:00'],
              },
            ),
            'schedule.weekdays',
            'Unbekannter Wochentag.',
            'montag',
          ),
          (
            'a weekday twice',
            body(
              schedule: {
                'frequency': 'weekly',
                'interval': 1,
                'weekdays': ['tue', 'tue'],
                'times': ['07:00'],
              },
            ),
            'schedule.weekdays',
            'Jeden Wochentag nur einmal.',
            'tue',
          ),
          (
            'date format',
            body(startDate: '23.11.2026'),
            'startDate',
            'Das Datum muss im Format JJJJ-MM-TT angegeben werden.',
            '23.11.2026',
          ),
          (
            'date before 2000',
            body(startDate: '1999-12-31'),
            'startDate',
            'Das Datum muss zwischen 2000 und 2099 liegen.',
            '1999-12-31',
          ),
          (
            'end before start',
            body(extra: {'endDate': '2026-11-17'}),
            'endDate',
            'Das Enddatum darf nicht vor dem Startdatum liegen.',
            '2026-11-17',
          ),
        ]) {
      test(title, () async {
        final error = await errorOf(emptyMock().createRoutineFromJson(request));
        expect(error.httpStatus, 400);
        expect(error.code, ErrorCode.invalidRequest);
        expect(error.message, message);
        expect(error.details, {'field': ?field, 'value': ?value});
      });
    }

    test('names are unique ignoring case; ß and ss stay different', () async {
      final mock = emptyMock();
      final first = await mock.createRoutineFromJson(body(name: 'Vitamin D'));
      final taken = await errorOf(
        mock.createRoutineFromJson(body(name: 'vitamin d')),
      );
      expect(taken.code, ErrorCode.routineNameTaken);
      expect(taken.httpStatus, 409);
      expect(taken.message, 'Eine Routine mit diesem Namen gibt es schon.');
      expect(taken.details, {'routineId': first.id});

      await mock.createRoutineFromJson(body(name: 'Straße'));
      await mock.createRoutineFromJson(body(name: 'STRASSE'));
    });

    test('at most 50 routines', () async {
      final mock = emptyMock();
      for (var i = 0; i < 50; i++) {
        await mock.createRoutineFromJson(body(name: 'Routine $i'));
      }
      final error = await errorOf(
        mock.createRoutineFromJson(body(name: 'Routine 50')),
      );
      expect(error.code, ErrorCode.routineLimitReached);
      expect(
        error.message,
        'Du hast schon 50 Routinen. Lösche eine, bevor du eine neue anlegst.',
      );
      expect(error.details, {'limit': 'routines', 'max': 50});
    });
  });

  group('change (3.20)', () {
    test('only the fields sent change; null clears', () async {
      final mock = emptyMock();
      final r = await mock.createRoutineFromJson(
        body(extra: {'amount': 2, 'unit': 'Stück', 'endDate': '2026-12-01'}),
      );
      final changed = await mock.updateRoutine(
        r.id,
        RoutinePatch(
          schedule: const RoutineSchedule.daily(
            interval: 1,
            times: ['07:30', '20:00'],
          ),
          endDate: Value(PlainDate(2026, 12, 7)),
        ),
      );
      expect(changed.schedule.times, ['07:30', '20:00']);
      expect(changed.endDate, PlainDate(2026, 12, 7));
      expect(changed.amount, 2);

      final noAmount = await mock.updateRoutine(
        r.id,
        const RoutinePatch(amount: Value(null), endDate: Value(null)),
      );
      expect(noAmount.amount, isNull);
      expect(noAmount.unit, isNull, reason: 'amount null clears the unit');
      expect(noAmount.endDate, isNull);
      expect(noAmount.progress.today.amountTarget, isNull);
    });

    test('values that already apply: 200, unchanged', () async {
      final mock = emptyMock();
      final r = await mock.createRoutineFromJson(body());
      final same = await mock.updateRoutineFromJson(r.id, {
        'name': 'Vitamin D',
      });
      expect(same.name, 'Vitamin D');
    });

    for (final (title, request, field, message)
        in <(String, Map<String, dynamic>?, String?, String)>[
          (
            'not an object',
            null,
            null,
            'Bitte die Änderung als JSON-Objekt schicken.',
          ),
          (
            'empty body',
            {},
            null,
            'Bitte mindestens ein Feld zum Ändern schicken.',
          ),
          (
            'a field that cannot change',
            {'status': 'paused'},
            'status',
            'Das Feld status kann nicht geändert werden.',
          ),
          (
            'null for the name',
            {'name': null},
            'name',
            'Der Name muss 1 bis 60 Zeichen lang sein.',
          ),
          (
            'start after the stored end',
            {'startDate': '2026-12-02'},
            'startDate',
            'Das Startdatum darf nicht nach dem Enddatum liegen.',
          ),
          (
            'end before the start',
            {'endDate': '2026-11-01'},
            'endDate',
            'Das Enddatum darf nicht vor dem Startdatum liegen.',
          ),
          (
            'a unit without amount',
            {'unit': 'ml'},
            'unit',
            'Eine Einheit braucht eine Menge.',
          ),
        ]) {
      test(title, () async {
        final mock = emptyMock();
        final r = await mock.createRoutineFromJson(
          body(extra: {'endDate': '2026-12-01'}),
        );
        final error = await errorOf(mock.updateRoutineFromJson(r.id, request));
        expect(error.code, ErrorCode.invalidRequest);
        expect(error.message, message);
        expect(error.details['field'], field);
      });
    }

    test('404 comes before the body', () async {
      final error = await errorOf(
        emptyMock().updateRoutineFromJson('6742c0a1b2c3d4e5f6a7b8c9', null),
      );
      expect(error.code, ErrorCode.routineNotFound);
    });

    test('a new name that is taken: 409', () async {
      final mock = emptyMock();
      final a = await mock.createRoutineFromJson(body(name: 'A'));
      final b = await mock.createRoutineFromJson(body(name: 'B'));
      final error = await errorOf(
        mock.updateRoutine(b.id, const RoutinePatch(name: 'a')),
      );
      expect(error.code, ErrorCode.routineNameTaken);
      expect(error.details, {'routineId': a.id});
    });
  });

  group('pause, resume and status (3.21, 3.22, 3.16)', () {
    test('pause until a day, change the end, resume', () async {
      final mock = emptyMock();
      final r = await mock.createRoutineFromJson(body());
      final paused = await mock.pauseRoutine(
        r.id,
        until: PlainDate(2026, 11, 30),
      );
      expect(paused.status, RoutineStatus.paused);
      expect(paused.pauses.single.from, PlainDate(2026, 11, 23));
      expect(paused.pauses.single.until, PlainDate(2026, 11, 30));

      // A pause that includes today only gets the new end.
      final open = await mock.pauseRoutine(r.id);
      expect(open.pauses, hasLength(1));
      expect(open.pauses.single.until, isNull);

      // It began today: resuming removes it.
      final resumed = await mock.resumeRoutine(r.id);
      expect(resumed.status, RoutineStatus.active);
      expect(resumed.pauses, isEmpty);
    });

    test('resuming a pause that began earlier ends it yesterday', () async {
      final mock = emptyMock();
      final r = await mock.createRoutineFromJson(body());
      now = DateTime.utc(2026, 11, 20, 12);
      await mock.pauseRoutine(r.id);
      now = DateTime.utc(2026, 11, 23, 12);
      final resumed = await mock.resumeRoutine(r.id);
      expect(resumed.pauses.single.from, PlainDate(2026, 11, 20));
      expect(resumed.pauses.single.until, PlainDate(2026, 11, 22));
      // Today's occurrence is due again.
      expect(resumed.progress.today.due, 1);
    });

    test('an end before today is refused', () async {
      final mock = emptyMock();
      final r = await mock.createRoutineFromJson(body());
      final error = await errorOf(
        mock.pauseRoutine(r.id, until: PlainDate(2026, 11, 22)),
      );
      expect(error.message, 'Das Ende der Pause darf nicht vor heute liegen.');
      expect(error.details, {'field': 'until', 'value': '2026-11-22'});
      final unknown = await errorOf(
        mock.pauseRoutineFromJson(r.id, {'from': '2026-11-23'}),
      );
      expect(unknown.message, 'Unbekanntes Feld: from.');
    });

    test('upcoming, ended; ended wins over paused', () async {
      final mock = emptyMock();
      final upcoming = await mock.createRoutineFromJson(
        body(name: 'A', startDate: '2026-11-25'),
      );
      expect(upcoming.status, RoutineStatus.upcoming);
      final ended = await mock.createRoutineFromJson(
        body(
          name: 'B',
          startDate: '2026-11-01',
          extra: {'endDate': '2026-11-10'},
        ),
      );
      expect(ended.status, RoutineStatus.ended);
      expect((await mock.pauseRoutine(ended.id)).status, RoutineStatus.ended);
    });
  });

  group('progress (cases of routine_progress_test.go)', () {
    test('„Blutdruck messen“ on day 6 of 14 (3.16)', () async {
      now = DateTime.utc(2026, 11, 23, 20, 30);
      final mock = emptyMock();
      final r = await mock.createRoutineFromJson(
        body(
          name: 'Blutdruck messen',
          schedule: {
            'frequency': 'daily',
            'interval': 1,
            'times': ['08:00', '20:00'],
          },
          extra: {'endDate': '2026-12-01'},
        ),
      );
      for (final due in [
        '2026-11-18T07:00:00Z',
        '2026-11-18T19:00:00Z',
        '2026-11-19T07:00:00Z',
        '2026-11-20T07:00:00Z',
        '2026-11-20T19:00:00Z',
        '2026-11-21T07:00:00Z',
        '2026-11-22T07:00:00Z',
        '2026-11-22T19:00:00Z',
        '2026-11-23T07:00:00Z',
      ]) {
        await mock.addRoutineLogFromJson(r.id, occurrenceLog(due));
      }
      await mock.addRoutineLogFromJson(
        r.id,
        occurrenceLog('2026-11-19T19:00:00Z', status: 'skipped'),
      );
      final p = (await mock.getRoutine(r.id)).progress;
      expect(counts(p.today), (1, 0, 2, null, null));
      expect(counts(p.week), (1, 0, 14, null, null));
      expect(counts(p.period!), (9, 1, 28, null, null));
      expect((p.period!.day, p.period!.days), (6, 14));
    });

    test('„Wasser“ with two extra glasses (3.16)', () async {
      now = DateTime.utc(2026, 11, 25, 19, 30);
      final mock = emptyMock();
      final r = await mock.createRoutineFromJson({
        'name': 'Wasser',
        'amount': 250,
        'unit': 'ml',
        'schedule': {
          'frequency': 'hourly',
          'interval': 1,
          'windowStart': '08:00',
          'windowEnd': '20:00',
        },
        'startDate': '2026-11-16',
      });
      String at(int day, int hour) =>
          '2026-11-${day}T${hour.toString().padLeft(2, '0')}:00:00Z';
      for (var h = 7; h <= 18; h++) {
        await mock.addRoutineLogFromJson(r.id, occurrenceLog(at(23, h)));
      }
      for (var h = 7; h <= 17; h++) {
        await mock.addRoutineLogFromJson(r.id, occurrenceLog(at(24, h)));
      }
      await mock.addRoutineLogFromJson(
        r.id,
        occurrenceLog(at(24, 18), status: 'skipped'),
      );
      await mock.addRoutineLogFromJson(r.id, freeLog('2026-11-24T11:30:00Z'));
      await mock.addRoutineLogFromJson(r.id, freeLog('2026-11-24T14:30:00Z'));
      for (var h = 7; h <= 11; h++) {
        await mock.addRoutineLogFromJson(r.id, occurrenceLog(at(25, h)));
      }
      final p = (await mock.getRoutine(r.id)).progress;
      expect(counts(p.today), (5, 0, 13, 1250.0, 3250.0));
      expect(counts(p.week), (28, 1, 91, 7500.0, 22750.0));
      expect(p.period, isNull);
    });

    test('„Eiweiß“ in portions (3.16)', () async {
      now = DateTime.utc(2026, 11, 23, 16);
      final mock = emptyMock();
      final r = await mock.createRoutineFromJson(
        body(
          name: 'Eiweiß',
          schedule: {
            'frequency': 'daily',
            'interval': 1,
            'times': ['20:00'],
          },
          startDate: '2026-11-20',
          extra: {'amount': 30, 'unit': 'g'},
        ),
      );
      await mock.addRoutineLogFromJson(
        r.id,
        freeLog('2026-11-23T11:00:00Z', amount: 10),
      );
      await mock.addRoutineLogFromJson(
        r.id,
        freeLog('2026-11-23T14:00:00Z', amount: 15),
      );
      expect(counts((await mock.getRoutine(r.id)).progress.today), (
        0,
        0,
        1,
        25.0,
        30.0,
      ));
      now = DateTime.utc(2026, 11, 23, 19, 5);
      await mock.addRoutineLogFromJson(
        r.id,
        occurrenceLog('2026-11-23T19:00:00Z', amount: 5),
      );
      expect(counts((await mock.getRoutine(r.id)).progress.today), (
        1,
        0,
        1,
        30.0,
        30.0,
      ));
    });

    test('paused days are not due, unless an occurrence has a log', () async {
      final mock = emptyMock();
      now = DateTime.utc(2026, 11, 16, 12);
      final r = await mock.createRoutineFromJson(
        body(
          name: 'x',
          schedule: {
            'frequency': 'daily',
            'interval': 1,
            'times': ['08:00', '20:00'],
          },
          startDate: '2026-11-16',
          extra: {'endDate': '2026-11-29'},
        ),
      );
      await mock.addRoutineLogFromJson(
        r.id,
        occurrenceLog('2026-11-16T07:00:00Z'),
      );
      now = DateTime.utc(2026, 11, 17, 12);
      await mock.pauseRoutine(r.id, until: PlainDate(2026, 11, 18));
      await mock.addRoutineLogFromJson(
        r.id,
        occurrenceLog('2026-11-17T07:00:00Z', status: 'skipped'),
      );
      now = DateTime.utc(2026, 11, 23, 12);
      await mock.pauseRoutine(r.id);
      await mock.addRoutineLogFromJson(
        r.id,
        occurrenceLog('2026-11-23T07:00:00Z'),
      );

      now = DateTime.utc(2026, 11, 23, 20);
      final p = (await mock.getRoutine(r.id)).progress;
      expect(counts(p.today), (1, 0, 1, null, null));
      expect(counts(p.week), (1, 0, 1, null, null));
      // 14 days × 2 = 28; paused 17, 18 and 23–29: 18; plus two logged.
      expect(counts(p.period!), (2, 1, 12, null, null));
      expect(p.period!.day, 8);

      final today = await mock.listOccurrences(
        module: 'health',
        from: PlainDate(2026, 11, 23),
        to: PlainDate(2026, 11, 23),
      );
      expect(
        [for (final o in today.occurrences) o.status],
        [OccurrenceStatus.done, OccurrenceStatus.paused],
      );
      expect(today.occurrences.last.logId, isNull);
    });

    test('a log whose occurrence is gone counts only the amount', () async {
      final mock = emptyMock();
      final r = await mock.createRoutineFromJson(
        body(startDate: '2026-11-16', extra: {'amount': 2, 'unit': 'Stück'}),
      );
      await mock.addRoutineLogFromJson(
        r.id,
        occurrenceLog('2026-11-23T07:00:00Z', amount: 2),
      );
      await mock.addRoutineLogFromJson(r.id, {
        ...occurrenceLog('2026-11-22T07:00:00Z', amount: 1),
        'takenAt': '2026-11-23T09:00:00Z',
      });
      final changed = await mock.updateRoutine(
        r.id,
        const RoutinePatch(
          schedule: RoutineSchedule.daily(interval: 1, times: ['07:30']),
        ),
      );
      expect(counts(changed.progress.today), (0, 0, 1, 3.0, 2.0));
      final list = await mock.listOccurrences(
        module: 'health',
        from: PlainDate(2026, 11, 23),
        to: PlainDate(2026, 11, 23),
      );
      expect(list.occurrences.single.time, '07:30');
      expect(list.occurrences.single.status, OccurrenceStatus.open);
    });

    test(
      'after a change of the time zone a log keeps its occurrence',
      () async {
        final mock = emptyMock();
        final r = await mock.createRoutineFromJson(
          body(startDate: '2026-11-16'),
        );
        final log = await mock.addRoutineLogFromJson(
          r.id,
          occurrenceLog('2026-11-23T07:00:00Z'),
        );
        expect(log.timeZone, 'Europe/Vienna');
        await mock.updateTimeZone('Europe/Lisbon');
        final list = await mock.listOccurrences(
          module: 'health',
          from: PlainDate(2026, 11, 23),
          to: PlainDate(2026, 11, 23),
        );
        final o = list.occurrences.single;
        expect(o.dueAt, DateTime.utc(2026, 11, 23, 8));
        expect(o.status, OccurrenceStatus.done);
        expect(o.logId, log.id);
        expect(counts((await mock.getRoutine(r.id)).progress.today), (
          1,
          0,
          1,
          null,
          null,
        ));
      },
    );

    test('amounts: none on the routine, rounding, own amount', () async {
      now = DateTime.utc(2026, 11, 23, 20);
      final mock = emptyMock();
      final none = await mock.createRoutineFromJson(
        body(name: 'ohne', startDate: '2026-11-23'),
      );
      await mock.addRoutineLogFromJson(
        none.id,
        occurrenceLog('2026-11-23T07:00:00Z', amount: 3),
      );
      expect(counts((await mock.getRoutine(none.id)).progress.today), (
        1,
        0,
        1,
        null,
        null,
      ));

      final tenth = await mock.createRoutineFromJson(
        body(
          name: 'zehntel',
          schedule: {
            'frequency': 'daily',
            'interval': 1,
            'times': ['08:00', '12:00', '16:00'],
          },
          startDate: '2026-11-23',
          extra: {'amount': 0.1},
        ),
      );
      for (final h in [7, 11, 15]) {
        await mock.addRoutineLogFromJson(
          tenth.id,
          occurrenceLog('2026-11-23T${h.toString().padLeft(2, '0')}:00:00Z'),
        );
      }
      expect(counts((await mock.getRoutine(tenth.id)).progress.today), (
        3,
        0,
        3,
        0.3,
        0.3,
      ));
    });

    test('daylight saving time: 23 occurrences on the day forward', () async {
      now = DateTime.utc(2026, 3, 29, 10);
      final mock = emptyMock();
      final r = await mock.createRoutineFromJson(
        body(
          name: 'x',
          schedule: {
            'frequency': 'hourly',
            'interval': 1,
            'windowStart': '00:00',
            'windowEnd': '23:00',
          },
          startDate: '2026-03-16',
          extra: {'endDate': '2026-11-01'},
        ),
      );
      expect(r.progress.today.due, 23);
      expect(r.progress.week.due, 6 * 24 + 23);
      now = DateTime.utc(2026, 10, 25, 10);
      final back = await mock.getRoutine(r.id);
      expect(back.progress.today.due, 24);
      expect(back.progress.week.due, 7 * 24);
      expect(back.progress.period!.due, 231 * 24 - 1);
      expect(back.progress.period!.days, 231);
    });
  });

  group('logs (3.25–3.27)', () {
    test('one log per occurrence: 409 with the existing log', () async {
      final mock = emptyMock();
      final r = await mock.createRoutineFromJson(body());
      final first = await mock.addRoutineLog(
        r.id,
        NewRoutineLog(
          status: LogStatus.done,
          source: LogSource.notification,
          dueAt: DateTime.utc(2026, 11, 23, 7),
        ),
      );
      expect(first.source, LogSource.notification);
      expect(first.takenAt, now);
      expect(first.amount, isNull);
      final error = await errorOf(
        mock.addRoutineLog(
          r.id,
          NewRoutineLog(
            status: LogStatus.skipped,
            source: LogSource.manual,
            dueAt: DateTime.utc(2026, 11, 23, 7),
          ),
        ),
      );
      expect(error.code, ErrorCode.routineLogExists);
      expect(error.httpStatus, 409);
      expect(error.message, 'Dieser Termin ist schon eingetragen.');
      expect(error.details, {'logId': first.id});
    });

    test('done without amount takes the routine amount', () async {
      final mock = emptyMock();
      final r = await mock.createRoutineFromJson(
        body(extra: {'amount': 2, 'unit': 'Stück'}),
      );
      final log = await mock.addRoutineLog(
        r.id,
        NewRoutineLog(
          status: LogStatus.done,
          source: LogSource.manual,
          dueAt: DateTime.utc(2026, 11, 23, 7),
        ),
      );
      expect(log.amount, 2);
    });

    for (final (title, request, field, message)
        in <(String, Map<String, dynamic>?, String?, String)>[
          (
            'not an object',
            null,
            null,
            'Bitte den Verlaufseintrag als JSON-Objekt schicken.',
          ),
          (
            'unknown field',
            {'status': 'done', 'source': 'manual', 'entryId': 'x'},
            'entryId',
            'Unbekanntes Feld: entryId.',
          ),
          (
            'unknown status',
            {'status': 'open', 'source': 'manual'},
            'status',
            'Unbekannter Status.',
          ),
          (
            'source entry',
            {'status': 'done', 'source': 'entry'},
            'source',
            'Unbekannte Herkunft.',
          ),
          (
            'dueAt not a point in time',
            {'status': 'done', 'source': 'manual', 'dueAt': '2026-11-23'},
            'dueAt',
            'Bitte einen gültigen Zeitpunkt angeben.',
          ),
          (
            'dueAt no occurrence',
            {
              'status': 'done',
              'source': 'manual',
              'dueAt': '2026-11-23T07:30:00Z',
            },
            'dueAt',
            'Zu diesem Zeitpunkt ist kein Termin der Routine geplant.',
          ),
          (
            'dueAt tomorrow',
            {
              'status': 'done',
              'source': 'manual',
              'dueAt': '2026-11-24T07:00:00Z',
            },
            'dueAt',
            'Termine in der Zukunft können noch nicht eingetragen werden.',
          ),
          (
            'takenAt in the future',
            {
              'status': 'done',
              'source': 'manual',
              'takenAt': '2026-11-23T12:06:00Z',
            },
            'takenAt',
            'Der Zeitpunkt darf nicht in der Zukunft liegen.',
          ),
          (
            'amount for skipped',
            {'status': 'skipped', 'source': 'manual', 'amount': 1},
            'amount',
            'Ausgelassene Termine haben keine Menge.',
          ),
          (
            'amount 0',
            {'status': 'done', 'source': 'manual', 'amount': 0},
            'amount',
            'Die Menge muss größer als 0 und höchstens 100.000 sein.',
          ),
        ]) {
      test(title, () async {
        final mock = emptyMock();
        final r = await mock.createRoutineFromJson(body());
        final error = await errorOf(mock.addRoutineLogFromJson(r.id, request));
        expect(error.code, ErrorCode.invalidRequest);
        expect(error.message, message);
        expect(error.details['field'], field);
      });
    }

    test('takenAt up to 5 minutes ahead is fine', () async {
      final mock = emptyMock();
      final r = await mock.createRoutineFromJson(body());
      final log = await mock.addRoutineLogFromJson(r.id, {
        'status': 'done',
        'source': 'manual',
        'takenAt': '2026-11-23T12:05:00Z',
      });
      expect(log.dueAt, isNull);
    });

    test('list newest first; delete opens the occurrence again', () async {
      final mock = emptyMock();
      final r = await mock.createRoutineFromJson(body());
      final older = await mock.addRoutineLogFromJson(
        r.id,
        occurrenceLog('2026-11-22T07:00:00Z'),
      );
      final newer = await mock.addRoutineLogFromJson(
        r.id,
        occurrenceLog('2026-11-23T07:00:00Z'),
      );
      expect(
        [for (final l in await mock.listRoutineLogs(r.id)) l.id],
        [newer.id, older.id],
      );
      await mock.deleteRoutineLog(r.id, newer.id);
      final list = await mock.listOccurrences(
        module: 'health',
        from: PlainDate(2026, 11, 23),
        to: PlainDate(2026, 11, 23),
      );
      expect(list.occurrences.single.status, OccurrenceStatus.open);

      final again = await errorOf(mock.deleteRoutineLog(r.id, newer.id));
      expect(again.code, ErrorCode.routineLogNotFound);
      expect(again.message, 'Diesen Verlaufseintrag gibt es nicht.');
      final invalid = await errorOf(mock.deleteRoutineLog(r.id, 'x'));
      expect(invalid.code, ErrorCode.invalidId);
      expect(invalid.message, 'Diese Verlaufseintrag-ID ist ungültig.');
    });

    test('range of the log list', () async {
      final mock = emptyMock();
      final r = await mock.createRoutineFromJson(body());
      final reversed = await errorOf(
        mock.listRoutineLogsWithParameters(r.id, {
          'from': '2026-11-23',
          'to': '2026-11-01',
        }),
      );
      expect(reversed.code, ErrorCode.invalidParameter);
      expect(
        reversed.message,
        'Das Ende des Zeitraums darf nicht vor dem Anfang liegen.',
      );
      expect(reversed.details, {'parameter': 'to', 'value': '2026-11-01'});
      final long = await errorOf(
        mock.listRoutineLogsWithParameters(r.id, {'from': '2025-11-22'}),
      );
      expect(long.message, 'Der Zeitraum darf höchstens 366 Tage umfassen.');
      expect(long.details, {'parameter': 'from', 'value': '2025-11-22'});
    });

    test('deleting a routine deletes its logs', () async {
      final mock = emptyMock();
      final r = await mock.createRoutineFromJson(body());
      final log = await mock.addRoutineLogFromJson(
        r.id,
        occurrenceLog('2026-11-23T07:00:00Z'),
      );
      await mock.deleteRoutine(r.id);
      expect(
        (await errorOf(mock.deleteRoutine(r.id))).code,
        ErrorCode.routineNotFound,
      );
      expect(
        (await errorOf(mock.deleteRoutineLog(r.id, log.id))).code,
        ErrorCode.routineNotFound,
      );
    });
  });

  group('occurrences (3.24)', () {
    test('default: today and six days, sorted by dueAt and routine', () async {
      final mock = emptyMock();
      final b = await mock.createRoutineFromJson(body(name: 'B'));
      final a = await mock.createRoutineFromJson(
        body(
          name: 'A',
          schedule: {
            'frequency': 'daily',
            'interval': 1,
            'times': ['08:00', '20:00'],
          },
        ),
      );
      final list = await mock.listOccurrences(module: 'health');
      expect(list.from, PlainDate(2026, 11, 23));
      expect(list.to, PlainDate(2026, 11, 29));
      expect(list.occurrences, hasLength(7 + 14));
      final first = list.occurrences.take(3).toList();
      expect([for (final o in first) o.time], ['08:00', '08:00', '20:00']);
      // Same point in time: ordered by routine ID (b was created first).
      expect([for (final o in first.take(2)) o.routineId], [b.id, a.id]);
    });

    for (final (title, parameters, parameter, value, message) in [
      (
        'module first',
        {'module': 'garden', 'from': 'x'},
        'module',
        'garden',
        'Unbekanntes Modul. Im Prototyp gibt es nur health.',
      ),
      (
        'from',
        {'module': 'health', 'from': '23.11.2026'},
        'from',
        '23.11.2026',
        'Das Datum muss im Format JJJJ-MM-TT angegeben werden.',
      ),
      (
        'to before from',
        {'module': 'health', 'from': '2026-11-23', 'to': '2026-11-22'},
        'to',
        '2026-11-22',
        'Das Ende des Zeitraums darf nicht vor dem Anfang liegen.',
      ),
      (
        '32 days',
        {'module': 'health', 'from': '2026-11-01', 'to': '2026-12-02'},
        'to',
        '2026-12-02',
        'Der Zeitraum darf höchstens 31 Tage umfassen.',
      ),
    ]) {
      test('parameters: $title', () async {
        final error = await errorOf(
          emptyMock().listOccurrencesWithParameters(parameters),
        );
        expect(error.code, ErrorCode.invalidParameter);
        expect(error.message, message);
        expect(error.details, {'parameter': parameter, 'value': value});
      });
    }
  });
}
