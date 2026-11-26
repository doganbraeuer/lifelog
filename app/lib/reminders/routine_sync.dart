import 'package:flutter/foundation.dart';
import 'package:flutter/widgets.dart';

import '../api/api_exception.dart';
import '../model/plain_date.dart';
import '../model/routine.dart';
import '../platform/device_time_zone.dart';
import '../platform/reminders.dart';
import '../session/session_controller.dart';
import 'reminder_plan.dart';

/// Keeps the routines of the logged-in user in step with the phone:
///
/// - the time zone: the phone's IANA name is compared with `timeZone` from
///   `GET /me`; if they differ, `PATCH /me` sends it (docs/API.md 3.13).
/// - the reminders: the occurrences of today and the next six days are
///   loaded and planned as local reminders (3.16, reminders, points 1–2).
///   If loading fails (no network), the planned reminders stay as they are.
///
/// [refresh] runs at the start of a session, when the app comes back to the
/// foreground and after every change of a routine. Calls never overlap: a
/// call while one is running makes it run once more afterwards.
///
/// Taps on reminders wait in [pendingTap] until the routines tab of the
/// module handles them.
class RoutineSync extends ChangeNotifier {
  RoutineSync({
    required this.session,
    required this.deviceTimeZone,
    required this.reminders,
    required this.modules,
    DateTime Function()? clock,
    bool? isIOS,
  }) : _clock = clock ?? (() => DateTime.now().toUtc()),
       _isIOS = isIOS ?? defaultTargetPlatform == TargetPlatform.iOS;

  final SessionController session;
  final DeviceTimeZone deviceTimeZone;
  final Reminders reminders;

  /// The active modules; each has its own routines.
  final List<String> modules;
  final DateTime Function() _clock;
  final bool _isIOS;

  Future<void>? _running;
  var _again = false;

  /// Zones the backend refused (e.g. `GMT` on some phones): not sent again
  /// in this session, so there is no loop; the server's zone stays.
  final _refused = <String>{};

  /// `null` until known.
  ReminderPermission? get permission => _permission;
  ReminderPermission? _permission;

  /// A tap on a reminder that is not handled yet.
  final pendingTap = ValueNotifier<ReminderTap?>(null);

  /// Counts logs made outside the screens, by a reminder button in the
  /// background: the routines tab reloads when it changes.
  final dataChanged = ValueNotifier<int>(0);

  /// Modules whose shell is open, with how often (a shell registers
  /// itself). A tap on a reminder of an open module is handled there.
  final _openModules = <String, int>{};

  void moduleOpened(String module) =>
      _openModules[module] = (_openModules[module] ?? 0) + 1;

  void moduleClosed(String module) {
    final count = (_openModules[module] ?? 1) - 1;
    if (count <= 0) {
      _openModules.remove(module);
    } else {
      _openModules[module] = count;
    }
  }

  bool isModuleOpen(String module) => _openModules.containsKey(module);

  /// A reminder button logged an occurrence in the background (path A),
  /// while the app runs: the screens reload, the reminders are planned
  /// again.
  void loggedInBackground() {
    dataChanged.value++;
    refresh();
  }

  /// Brings everything up to date. Errors (no network, …) are swallowed:
  /// the next refresh tries again.
  Future<void> refresh() {
    if (_running != null) {
      _again = true;
      return _running!;
    }
    return _running = _loop().whenComplete(() => _running = null);
  }

  Future<void> _loop() async {
    do {
      _again = false;
      await _once();
    } while (_again);
  }

  Future<void> _once() async {
    if (session.phase != SessionPhase.ready) return;
    await _syncTimeZone();
    await _updatePermission();
    if (_permission == ReminderPermission.granted) await _plan();
  }

  Future<void> _syncTimeZone() async {
    final phone = await deviceTimeZone.name();
    final user = session.user;
    if (phone == null ||
        phone.isEmpty ||
        user == null ||
        user.timeZone == phone ||
        _refused.contains(phone)) {
      return;
    }
    try {
      await session.updateTimeZone(phone);
    } on ApiException catch (e) {
      if (e.code == ErrorCode.invalidRequest) _refused.add(phone);
    }
  }

  Future<void> _updatePermission() async {
    final now = await reminders.permission();
    if (now != _permission) {
      _permission = now;
      notifyListeners();
    }
  }

  Future<void> _plan() async {
    final routines = <Routine>[];
    final occurrences = <Occurrence>[];
    try {
      for (final module in modules) {
        routines.addAll(await session.api.listRoutines(module: module));
        occurrences.addAll(
          (await session.api.listOccurrences(module: module)).occurrences,
        );
      }
    } on ApiException {
      return; // keep the planned reminders
    }
    if (session.phase != SessionPhase.ready) return;
    final plan = planReminders(
      occurrences: occurrences,
      routines: routines,
      now: _clock(),
      isIOS: _isIOS,
    );
    int idOf(Occurrence o) => reminderId(occurrenceKey(o.routineId, o.dueAt));
    final routineIds = {for (final r in routines) r.id};
    // Shown reminders that must go: their occurrence is done, skipped or
    // paused, or their routine is gone (known from the last run). A shown
    // reminder of an open occurrence, or of an older day, stays.
    final remove = {
      for (final o in occurrences)
        if (!o.isOpen) idOf(o),
      for (final MapEntry(key: id, value: routineId) in _lastKnown.entries)
        if (!routineIds.contains(routineId)) id,
    };
    await reminders.apply(plan, remove: remove);
    _lastKnown = {for (final o in occurrences) idOf(o): o.routineId};
  }

  /// Reminder ID → routine of every occurrence of the last run, to remove
  /// the shown reminders of a routine that was deleted since.
  var _lastKnown = <int, String>{};

  /// Asks for the permission; if Android no longer asks, opens the
  /// settings. Plans the reminders when it is granted.
  Future<ReminderPermission> requestPermission({
    bool openSettings = false,
  }) async {
    var result = await reminders.requestPermission();
    if (result != ReminderPermission.granted && openSettings) {
      await reminders.openSettings();
      result = await reminders.permission();
    }
    _permission = result;
    notifyListeners();
    if (result == ReminderPermission.granted) await refresh();
    return result;
  }

  /// A tap on a reminder (also the one that started the app).
  void handleTap(ReminderTap tap) => pendingTap.value = tap;

  /// The reminder of an occurrence that was just logged in the app.
  Future<void> cancelReminderOf(String routineId, DateTime dueAt) =>
      reminders.cancel(reminderId(occurrenceKey(routineId, dueAt)));

  /// Shows the reminder of an open occurrence of today at once (debug only,
  /// see [reminderForToday]). Loads the occurrences itself, so the day is
  /// the user's today even after midnight. `false` if no occurrence of
  /// today is open; an [ApiException] reaches the caller.
  Future<bool> showNextNow() async {
    if (modules.isEmpty) return false;
    final routines = <Routine>[];
    final occurrences = <Occurrence>[];
    late PlainDate today;
    for (final module in modules) {
      routines.addAll(await session.api.listRoutines(module: module));
      final list = await session.api.listOccurrences(module: module);
      occurrences.addAll(list.occurrences);
      today = list.from; // without `from` in the request: the user's today
    }
    final reminder = reminderForToday(
      occurrences: occurrences,
      routines: routines,
      today: today,
      now: _clock(),
      isIOS: _isIOS,
    );
    if (reminder == null) return false;
    await reminders.showNow(reminder);
    return true;
  }

  /// At the end of a session: every reminder is removed, a tap not yet
  /// handled is dropped, and nothing of the previous user stays.
  Future<void> clear() async {
    _refused.clear();
    _lastKnown = {};
    _permission = null;
    pendingTap.value = null;
    await reminders.clear();
  }

  @override
  void dispose() {
    pendingTap.dispose();
    dataChanged.dispose();
    super.dispose();
  }
}

/// Makes the [RoutineSync] available below the MaterialApp. Widgets that
/// use [of] rebuild when it changes (e.g. the permission).
class RoutineSyncScope extends InheritedNotifier<RoutineSync> {
  const RoutineSyncScope({
    super.key,
    required RoutineSync sync,
    required super.child,
  }) : super(notifier: sync);

  /// `null` outside the app (e.g. a screen tested on its own).
  static RoutineSync? maybeOf(BuildContext context) =>
      context.dependOnInheritedWidgetOfExactType<RoutineSyncScope>()?.notifier;
}
