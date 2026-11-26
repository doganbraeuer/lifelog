import 'package:flutter/foundation.dart';
import 'package:flutter/widgets.dart';

import '../api/api_exception.dart';
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

  /// The last plan, e.g. for „Nächste Erinnerung jetzt zeigen“.
  List<PlannedReminder> get lastPlan => _lastPlan;
  var _lastPlan = <PlannedReminder>[];

  /// A tap on a reminder that is not handled yet.
  final pendingTap = ValueNotifier<ReminderTap?>(null);

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
    final open = {
      for (final o in occurrences)
        if (o.isOpen) occurrenceKey(o.routineId, o.dueAt),
    };
    final known = {
      for (final o in occurrences) occurrenceKey(o.routineId, o.dueAt),
    };
    await reminders.apply(
      plan,
      // A shown reminder stays while its occurrence is open, or when it is
      // not among the loaded ones (an older day).
      stillOpen: (tap) => open.contains(tap.key) || !known.contains(tap.key),
    );
    _lastPlan = plan;
  }

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

  /// Shows the next planned reminder at once (debug only).
  Future<bool> showNextNow() async {
    if (_lastPlan.isEmpty) return false;
    await reminders.showNow(_lastPlan.first);
    return true;
  }

  /// At the end of a session: every reminder is removed, a tap not yet
  /// handled is dropped, and nothing of the previous user stays.
  Future<void> clear() async {
    _refused.clear();
    _lastPlan = [];
    _permission = null;
    pendingTap.value = null;
    await reminders.clear();
  }

  @override
  void dispose() {
    pendingTap.dispose();
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
