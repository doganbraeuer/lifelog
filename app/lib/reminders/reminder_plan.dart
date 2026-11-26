import 'dart:convert';

import '../model/json_helpers.dart' show formatTimestamp;
import '../model/routine.dart';
import '../model/routine_texts.dart';

// Which reminders the phone plans, as a pure function (docs/API.md 3.16,
// "Reminders in the app"). The platform part is in platform/reminders.dart.

/// At most this many planned reminders: iOS keeps only 64 per app, and
/// some Android phones limit alarms too. With many hourly routines the
/// window of seven days shrinks; every start of the app plans again.
const maxPlannedReminders = 60;

/// IDs of the buttons; the same values as the log's `status`.
const reminderActionDone = 'done';
const reminderActionSkipped = 'skipped';
const reminderActionAmount = 'amount';

/// A reminder to plan for one occurrence.
class PlannedReminder {
  const PlannedReminder({
    required this.id,
    required this.dueAt,
    required this.module,
    required this.routineId,
    required this.title,
    required this.body,
    required this.withAmountButton,
  });

  /// Stable for the occurrence, so that a reminder can be removed after a
  /// log without knowing the rest of the plan.
  final int id;

  /// When it should come (the occurrence's `dueAt`, UTC).
  final DateTime dueAt;
  final String module;
  final String routineId;
  final String title;
  final String body;

  /// „Andere Menge“ only for a routine with an amount.
  final bool withAmountButton;

  /// IDs and the point in time, never a name: Android stores the payload
  /// with the planned notification.
  String get payload => reminderPayload(module, routineId, dueAt);

  @override
  String toString() => 'PlannedReminder($id)';
}

String reminderPayload(String module, String routineId, DateTime dueAt) =>
    jsonEncode({'m': module, 'r': routineId, 'd': formatTimestamp(dueAt)});

/// A tap on a reminder or one of its buttons.
class ReminderTap {
  const ReminderTap({
    required this.module,
    required this.routineId,
    required this.dueAt,
    this.actionId,
    this.notificationId,
  });

  /// `null` if the payload is not one of ours.
  static ReminderTap? fromPayload(
    String? payload, {
    String? actionId,
    int? notificationId,
  }) {
    if (payload == null) return null;
    try {
      final json = jsonDecode(payload);
      if (json case {
        'm': final String module,
        'r': final String routineId,
        'd': final String due,
      }) {
        final dueAt = DateTime.tryParse(due);
        if (dueAt == null) return null;
        return ReminderTap(
          module: module,
          routineId: routineId,
          dueAt: dueAt.toUtc(),
          actionId: actionId == null || actionId.isEmpty ? null : actionId,
          notificationId: notificationId,
        );
      }
    } on FormatException {
      // Not ours.
    }
    return null;
  }

  final String module;
  final String routineId;
  final DateTime dueAt;

  /// [reminderActionDone], [reminderActionSkipped], [reminderActionAmount],
  /// or `null` for a tap on the reminder itself.
  final String? actionId;
  final int? notificationId;

  /// The occurrence it is about, for comparing with the open ones.
  String get key => occurrenceKey(routineId, dueAt);

  @override
  String toString() => 'ReminderTap($routineId, ${actionId ?? 'open'})';
}

/// One occurrence: routine and point in time.
String occurrenceKey(String routineId, DateTime dueAt) =>
    '$routineId|${formatTimestamp(dueAt)}';

/// The ID of an occurrence's reminder: 31 bits of FNV-1a over [key].
int reminderId(String key) {
  var hash = 0x811c9dc5;
  for (final unit in utf8.encode(key)) {
    hash ^= unit;
    hash = (hash * 0x01000193) & 0xffffffff;
  }
  return hash & 0x7fffffff;
}

/// The reminders for [occurrences] (`GET /routines/occurrences`, today and
/// the next six days):
///
/// - only `open` occurrences whose `dueAt` lies after [now] (never `done`,
///   `skipped` or `paused`), of a routine in [routines];
/// - the nearest first (by `dueAt`, then routine), at most [max];
/// - Android: the routine's name as title and „08:00 · 2 Stück –
///   erledigt?“ as text, hidden on a locked screen by `visibility: private`;
///   iOS: no name at all („Zeit für deine Routine · 08:00“), because iOS
///   shows previews on the lock screen depending on the user's setting.
List<PlannedReminder> planReminders({
  required List<Occurrence> occurrences,
  required List<Routine> routines,
  required DateTime now,
  required bool isIOS,
  int max = maxPlannedReminders,
}) {
  final byId = {for (final r in routines) r.id: r};
  final candidates =
      [
        for (final o in occurrences)
          if (o.isOpen && o.dueAt.isAfter(now) && byId.containsKey(o.routineId))
            o,
      ]..sort((a, b) {
        final byTime = a.dueAt.compareTo(b.dueAt);
        return byTime != 0 ? byTime : a.routineId.compareTo(b.routineId);
      });
  final used = <int>{};
  final plan = <PlannedReminder>[];
  for (final o in candidates.take(max)) {
    final routine = byId[o.routineId]!;
    var id = reminderId(occurrenceKey(o.routineId, o.dueAt));
    while (!used.add(id)) {
      id = (id + 1) & 0x7fffffff; // collision: the next free one
    }
    final amount = routine.amount;
    plan.add(
      PlannedReminder(
        id: id,
        dueAt: o.dueAt,
        module: routine.module,
        routineId: routine.id,
        title: isIOS ? 'Lifelog: Erinnerung' : routine.name,
        body: isIOS
            ? 'Zeit für deine Routine · ${o.time}'
            : amount == null
            ? '${o.time} – erledigt?'
            : '${o.time} · ${amountWithUnit(amount, routine.unit)} – erledigt?',
        withAmountButton: amount != null,
      ),
    );
  }
  return plan;
}
