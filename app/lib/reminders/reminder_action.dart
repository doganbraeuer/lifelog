import 'dart:async';
import 'dart:ui';

import 'package:flutter_local_notifications/flutter_local_notifications.dart';

import '../api/api_client.dart';
import '../api/api_client_factory.dart';
import '../api/api_exception.dart';
import '../api/http_api_client.dart';
import '../model/routine.dart';
import '../platform/reminders.dart';
import '../screens/routine_actions.dart';
import '../session/session_store.dart';
import 'reminder_plan.dart';

// „Ja“ and „Nein“ on a reminder without opening the app (path A, docs/API.md
// 3.16, reminders, point 6). Android and iOS run the handler below in an
// isolate of its own, also when the app is closed; it has no access to the
// app's memory, only to the secure storage and the network.

/// The logic of a reminder button, testable without a phone: reads the
/// token from [store], sends the log with `source: notification`, and on
/// `201` or `409 ROUTINE_LOG_EXISTS` (already done, e.g. in the app)
/// removes the reminder. Returns `true` on success.
///
/// On any other outcome (no token, expired, no network, an error of the
/// backend) nothing is changed: the reminder stays, and a tap on it opens
/// the app at the occurrence, where the error is visible.
Future<bool> logFromReminderButton(
  ReminderTap tap, {
  required SessionStore store,
  required ApiClient Function() createApi,
  required Reminders reminders,
  required DateTime now,
}) async {
  final status = switch (tap.actionId) {
    reminderActionDone => LogStatus.done,
    reminderActionSkipped => LogStatus.skipped,
    _ => null,
  };
  if (status == null) return false;
  final stored = await store.read();
  if (stored == null || stored.isExpired(now)) return false;
  final api = createApi()..useToken(stored.token);
  try {
    await logOccurrence(
      api,
      routineId: tap.routineId,
      dueAt: tap.dueAt,
      status: status,
      source: LogSource.notification,
    );
  } on ApiException {
    // Only the code would be worth logging; nothing of the routine.
    return false;
  } finally {
    api.close();
  }
  if (tap.notificationId case final id?) await reminders.cancel(id);
  return true;
}

/// Entry point of the reminder buttons in the background. Top level and
/// marked, so that the compiler keeps it for the isolate.
@pragma('vm:entry-point')
void reminderButtonInBackground(NotificationResponse response) {
  // Plugins (secure storage, notifications) in this isolate too.
  DartPluginRegistrant.ensureInitialized();
  final tap = ReminderTap.fromPayload(
    response.payload,
    actionId: response.actionId,
    notificationId: response.id,
  );
  if (tap == null || apiBaseUrl.trim().isEmpty) return;
  unawaited(
    logFromReminderButton(
      tap,
      store: SecureSessionStore(),
      createApi: () => HttpApiClient(baseUrl: apiBaseUrl),
      reminders: PluginReminders(),
      now: DateTime.now().toUtc(),
    ),
  );
}
