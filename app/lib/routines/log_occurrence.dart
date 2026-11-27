import '../api/api_client.dart';
import '../api/api_exception.dart';
import '../model/routine.dart';

/// What happened when a log was sent for an occurrence.
class LogResult {
  const LogResult.logged(RoutineLog this.log) : alreadyLoggedMessage = null;

  const LogResult.alreadyLogged(String this.alreadyLoggedMessage) : log = null;

  /// The new log; `null` if the occurrence already had one.
  final RoutineLog? log;

  /// The backend's message for `409 ROUTINE_LOG_EXISTS` („Dieser Termin ist
  /// schon eingetragen.“), shown as information, not as an error.
  final String? alreadyLoggedMessage;
}

/// `POST /routines/{id}/logs` for an occurrence. `409 ROUTINE_LOG_EXISTS`
/// means the occurrence is already done, e.g. on another device or by a
/// reminder button; it counts as success (docs/API.md 3.16, reminders,
/// point 6). Any other error is thrown.
Future<LogResult> logOccurrence(
  ApiClient api, {
  required String routineId,
  required DateTime dueAt,
  required LogStatus status,
  required LogSource source,
  double? amount,
}) async {
  try {
    final log = await api.addRoutineLog(
      routineId,
      NewRoutineLog(
        status: status,
        source: source,
        dueAt: dueAt,
        amount: amount,
      ),
    );
    return LogResult.logged(log);
  } on ApiException catch (e) {
    if (e.code == ErrorCode.routineLogExists) {
      return LogResult.alreadyLogged(e.message);
    }
    rethrow;
  }
}
