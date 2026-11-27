import '../api/api_client.dart';
import '../api/api_exception.dart';
import '../model/routine.dart';
import '../platform/reminders.dart';
import 'reminder_plan.dart';

/// Plans the reminders anew from the server (docs/API.md 3.16, "Reminders
/// in the app", points 1–2): loads the routines and the occurrences of today
/// and the next six days of [modules] with [api], and replaces the planned
/// reminders by those of the open occurrences ([planReminders]).
///
/// Shown reminders are removed if their occurrence is no longer open
/// (logged, skipped, paused), or if their routine is in [lastKnown] (the
/// occurrences of the last run) but no longer on the server (deleted). A
/// shown reminder of an open occurrence, or of an older day, stays.
///
/// Used by `RoutineSync` in the app and by the reminder buttons in the
/// background, so that the seven planned days move on even if the app is
/// never opened.
///
/// Returns reminder ID → routine of every occurrence loaded, the next
/// [lastKnown]. Returns `null` and changes nothing if loading fails (no
/// network, an error of the backend), or if [proceed] answers `false` after
/// loading (e.g. the session ended meanwhile).
Future<Map<int, String>?> replanReminders({
  required ApiClient api,
  required List<String> modules,
  required Reminders reminders,
  required DateTime now,
  required bool isIOS,
  Map<int, String> lastKnown = const {},
  bool Function()? proceed,
}) async {
  final routines = <Routine>[];
  final occurrences = <Occurrence>[];
  try {
    for (final module in modules) {
      routines.addAll(await api.listRoutines(module: module));
      occurrences.addAll(
        (await api.listOccurrences(module: module)).occurrences,
      );
    }
  } on ApiException {
    return null; // keep the planned reminders
  }
  if (proceed != null && !proceed()) return null;
  final plan = planReminders(
    occurrences: occurrences,
    routines: routines,
    now: now,
    isIOS: isIOS,
  );
  int idOf(Occurrence o) => reminderId(occurrenceKey(o.routineId, o.dueAt));
  final routineIds = {for (final r in routines) r.id};
  final remove = {
    for (final o in occurrences)
      if (!o.isOpen) idOf(o),
    for (final MapEntry(key: id, value: routineId) in lastKnown.entries)
      if (!routineIds.contains(routineId)) id,
  };
  await reminders.apply(plan, remove: remove);
  return {for (final o in occurrences) idOf(o): o.routineId};
}
