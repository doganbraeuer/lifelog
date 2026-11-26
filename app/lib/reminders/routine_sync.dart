import '../api/api_exception.dart';
import '../platform/device_time_zone.dart';
import '../session/session_controller.dart';

/// Keeps the routines of the logged-in user in step with the phone:
///
/// - the time zone: the phone's IANA name is compared with `timeZone` from
///   `GET /me`; if they differ, `PATCH /me` sends it (docs/API.md 3.13).
///   Routines compute in that zone on the server.
///
/// [refresh] runs at the start of a session, whenever the app comes back to
/// the foreground, and after every change of a routine. Calls never
/// overlap: a call while one is running makes it run once more afterwards.
class RoutineSync {
  RoutineSync({required this.session, required this.deviceTimeZone});

  final SessionController session;
  final DeviceTimeZone deviceTimeZone;

  Future<void>? _running;
  var _again = false;

  /// Zones the backend refused (e.g. `GMT` on some phones). They are not
  /// sent again in this session, so there is no loop; the server's zone
  /// stays.
  final _refused = <String>{};

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
  }

  /// `true` if the time zone was changed on the server.
  Future<bool> _syncTimeZone() async {
    final phone = await deviceTimeZone.name();
    final user = session.user;
    if (phone == null ||
        phone.isEmpty ||
        user == null ||
        user.timeZone == phone ||
        _refused.contains(phone)) {
      return false;
    }
    try {
      await session.updateTimeZone(phone);
      return true;
    } on ApiException catch (e) {
      if (e.code == ErrorCode.invalidRequest) _refused.add(phone);
      return false;
    }
  }

  /// At the end of a session: nothing of the previous user stays.
  void clear() => _refused.clear();
}
