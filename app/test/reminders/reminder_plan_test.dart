// "Occurrences → reminder plan" (docs/API.md 3.16, reminders).
import 'dart:convert';

import 'package:flutter_test/flutter_test.dart';
import 'package:lifelog/model/plain_date.dart';
import 'package:lifelog/model/routine.dart';
import 'package:lifelog/reminders/reminder_plan.dart';

import '../model/routine_test.dart' show bloodPressureJson, waterJson;

Routine routine(String json, {String? id}) {
  final map = jsonDecode(json) as Map<String, dynamic>;
  if (id != null) map['id'] = id;
  return Routine.fromJson(map);
}

final water = routine(waterJson); // 250 ml, id …e4f5
final pressure = routine(bloodPressureJson); // no amount, id …b8c9

Occurrence occurrence(
  Routine r,
  DateTime dueAt,
  String time, {
  OccurrenceStatus status = OccurrenceStatus.open,
}) => Occurrence(
  routineId: r.id,
  dueAt: dueAt,
  date: PlainDate(dueAt.year, dueAt.month, dueAt.day),
  time: time,
  status: status,
  logId: status == OccurrenceStatus.done ? 'log' : null,
);

final now = DateTime.utc(2026, 11, 23, 9, 30);

void main() {
  test('only open occurrences in the future, nearest first', () {
    final plan = planReminders(
      occurrences: [
        occurrence(pressure, DateTime.utc(2026, 11, 23, 19), '20:00'),
        occurrence(water, DateTime.utc(2026, 11, 23, 9), '10:00'), // past
        occurrence(water, DateTime.utc(2026, 11, 23, 10), '11:00'),
        occurrence(
          water,
          DateTime.utc(2026, 11, 23, 11),
          '12:00',
          status: OccurrenceStatus.done,
        ),
        occurrence(
          water,
          DateTime.utc(2026, 11, 23, 12),
          '13:00',
          status: OccurrenceStatus.skipped,
        ),
        occurrence(
          water,
          DateTime.utc(2026, 11, 23, 13),
          '14:00',
          status: OccurrenceStatus.paused,
        ),
      ],
      routines: [water, pressure],
      now: now,
      isIOS: false,
    );
    expect(
      [for (final r in plan) r.dueAt],
      [DateTime.utc(2026, 11, 23, 10), DateTime.utc(2026, 11, 23, 19)],
    );
  });

  test('same point in time: ordered by routine; unknown routines skipped', () {
    final at = DateTime.utc(2026, 11, 23, 19);
    final plan = planReminders(
      occurrences: [
        occurrence(water, at, '20:00'),
        occurrence(pressure, at, '20:00'),
        Occurrence(
          routineId: 'deleted',
          dueAt: at,
          date: PlainDate(2026, 11, 23),
          time: '20:00',
          status: OccurrenceStatus.open,
          logId: null,
        ),
      ],
      routines: [water, pressure],
      now: now,
      isIOS: false,
    );
    expect([for (final r in plan) r.routineId], [pressure.id, water.id]);
  });

  test('Android: name and amount in the text; buttons by amount', () {
    final plan = planReminders(
      occurrences: [
        occurrence(water, DateTime.utc(2026, 11, 23, 10), '11:00'),
        occurrence(pressure, DateTime.utc(2026, 11, 23, 19), '20:00'),
      ],
      routines: [water, pressure],
      now: now,
      isIOS: false,
    );
    expect(plan[0].title, 'Wasser');
    expect(plan[0].body, '11:00 · 250 ml – erledigt?');
    expect(plan[0].withAmountButton, isTrue);
    expect(plan[1].title, 'Blutdruck messen');
    expect(plan[1].body, '20:00 – erledigt?');
    expect(plan[1].withAmountButton, isFalse);
  });

  test('iOS: no name in title or text', () {
    final plan = planReminders(
      occurrences: [occurrence(water, DateTime.utc(2026, 11, 23, 10), '11:00')],
      routines: [water],
      now: now,
      isIOS: true,
    );
    expect(plan.single.title, 'Lifelog: Erinnerung');
    expect(plan.single.body, 'Zeit für deine Routine · 11:00');
    expect(
      '${plan.single.title} ${plan.single.body}',
      isNot(contains('Wasser')),
    );
  });

  test('the payload holds IDs and the time, never the name', () {
    final reminder = planReminders(
      occurrences: [occurrence(water, DateTime.utc(2026, 11, 23, 10), '11:00')],
      routines: [water],
      now: now,
      isIOS: false,
    ).single;
    expect(jsonDecode(reminder.payload), {
      'm': 'health',
      'r': water.id,
      'd': '2026-11-23T10:00:00Z',
    });
    expect(reminder.payload, isNot(contains('Wasser')));
    expect('$reminder', isNot(contains('Wasser')));

    final tap = ReminderTap.fromPayload(
      reminder.payload,
      actionId: reminderActionDone,
      notificationId: reminder.id,
    )!;
    expect(tap.routineId, water.id);
    expect(tap.dueAt, DateTime.utc(2026, 11, 23, 10));
    expect(tap.actionId, reminderActionDone);
    expect(tap.key, occurrenceKey(water.id, reminder.dueAt));
    expect(ReminderTap.fromPayload('kein json'), isNull);
    expect(ReminderTap.fromPayload('{"x":1}'), isNull);
    expect(
      ReminderTap.fromPayload(reminder.payload, actionId: '')!.actionId,
      isNull,
    );
  });

  test('at most 60, the nearest ones', () {
    // Hourly water for a week: 91 occurrences.
    final occurrences = [
      for (var day = 23; day <= 29; day++)
        for (var hour = 7; hour <= 19; hour++)
          occurrence(
            water,
            DateTime.utc(2026, 11, day, hour),
            '${(hour + 1).toString().padLeft(2, '0')}:00',
          ),
    ];
    final plan = planReminders(
      occurrences: occurrences,
      routines: [water],
      now: DateTime.utc(2026, 11, 23, 6),
      isIOS: false,
    );
    expect(plan, hasLength(maxPlannedReminders));
    expect(plan.first.dueAt, DateTime.utc(2026, 11, 23, 7));
    expect(plan.last.dueAt, occurrences[59].dueAt);
  });

  test('IDs are stable, positive and unique', () {
    final at = DateTime.utc(2026, 11, 23, 10);
    final key = occurrenceKey(water.id, at);
    expect(reminderId(key), reminderId(key));
    expect(reminderId(key), inInclusiveRange(0, 0x7fffffff));
    expect(reminderId(key), isNot(reminderId(occurrenceKey(pressure.id, at))));

    final plan = planReminders(
      occurrences: [
        for (var i = 0; i < 50; i++)
          occurrence(water, at.add(Duration(hours: i)), '00:00'),
      ],
      routines: [water],
      now: now,
      isIOS: false,
    );
    expect({for (final r in plan) r.id}, hasLength(plan.length));
    expect(plan.first.id, reminderId(key));
  });

  test('no occurrences, no reminders', () {
    expect(
      planReminders(occurrences: [], routines: [water], now: now, isIOS: false),
      isEmpty,
    );
  });

  group('reminderForToday (debug button)', () {
    final today = PlainDate(2026, 11, 23);
    final tomorrowWater = occurrence(
      water,
      DateTime.utc(2026, 11, 24, 7),
      '08:00',
    );

    test('the next one still to come today, as a planned one looks', () {
      final reminder = reminderForToday(
        occurrences: [
          occurrence(water, DateTime.utc(2026, 11, 23, 9), '10:00'), // overdue
          occurrence(
            water,
            DateTime.utc(2026, 11, 23, 10),
            '11:00',
            status: OccurrenceStatus.done,
          ),
          occurrence(pressure, DateTime.utc(2026, 11, 23, 19), '20:00'),
          occurrence(water, DateTime.utc(2026, 11, 23, 11), '12:00'),
          tomorrowWater,
        ],
        routines: [water, pressure],
        today: today,
        now: now,
        isIOS: false,
      )!;
      final dueAt = DateTime.utc(2026, 11, 23, 11);
      expect(reminder.dueAt, dueAt);
      expect(reminder.id, reminderId(occurrenceKey(water.id, dueAt)));
      expect(reminder.title, water.name);
      expect(reminder.body, '12:00 · 250 ml – erledigt?');
      expect(reminder.withAmountButton, isTrue);
    });

    test('none left to come today: the last overdue one, not tomorrow', () {
      final reminder = reminderForToday(
        occurrences: [
          occurrence(water, DateTime.utc(2026, 11, 23, 8), '09:00'),
          occurrence(pressure, DateTime.utc(2026, 11, 23, 9), '10:00'),
          occurrence(
            water,
            DateTime.utc(2026, 11, 23, 19),
            '20:00',
            status: OccurrenceStatus.paused,
          ),
          occurrence(
            pressure,
            DateTime.utc(2026, 11, 23, 19),
            '20:00',
            status: OccurrenceStatus.skipped,
          ),
          tomorrowWater,
        ],
        routines: [water, pressure],
        today: today,
        now: now,
        isIOS: true,
      )!;
      expect(reminder.dueAt, DateTime.utc(2026, 11, 23, 9));
      expect(reminder.routineId, pressure.id);
      expect(reminder.title, 'Lifelog: Erinnerung');
      expect(reminder.body, 'Zeit für deine Routine · 10:00');
    });

    test('nothing open today: none, even if tomorrow is open', () {
      expect(
        reminderForToday(
          occurrences: [
            occurrence(
              water,
              DateTime.utc(2026, 11, 23, 11),
              '12:00',
              status: OccurrenceStatus.done,
            ),
            occurrence(pressure, DateTime.utc(2026, 11, 23, 19), '20:00'),
            tomorrowWater,
          ],
          routines: [water], // the blood pressure routine is unknown
          today: today,
          now: now,
          isIOS: false,
        ),
        isNull,
      );
    });
  });
}
