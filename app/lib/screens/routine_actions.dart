import 'package:flutter/material.dart';

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

/// How long a message with „Rückgängig“ stays: longer than Flutter's 4 s,
/// so that there is time to read it and tap (target group: older users).
const undoSnackBarDuration = Duration(seconds: 8);

/// Shows [message] at the bottom in place of the current message. With an
/// [action] (e.g. „Rückgängig“) it stays [undoSnackBarDuration], then goes
/// by itself. `persist: false` is needed for that: by default Flutter keeps
/// a snack bar with an action until it is tapped, over every page, and
/// later messages wait behind it.
void showRoutineSnackBar(
  BuildContext context,
  String message, {
  SnackBarAction? action,
}) {
  ScaffoldMessenger.of(context)
    ..hideCurrentSnackBar()
    ..showSnackBar(
      action == null
          ? SnackBar(content: Text(message))
          : SnackBar(
              content: Text(message),
              action: action,
              persist: false,
              duration: undoSnackBarDuration,
            ),
    );
}

/// Large buttons for the main actions (target group: older users).
const largeButtonSize = Size(88, 56);

ButtonStyle largeFilled(BuildContext context) => FilledButton.styleFrom(
  minimumSize: largeButtonSize,
  textStyle: Theme.of(context).textTheme.titleMedium,
);

ButtonStyle largeOutlined(BuildContext context) => OutlinedButton.styleFrom(
  minimumSize: largeButtonSize,
  textStyle: Theme.of(context).textTheme.titleMedium,
);
