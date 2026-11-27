import 'dart:async';
import 'dart:ui';

import 'package:flutter/foundation.dart';
import 'package:flutter_local_notifications/flutter_local_notifications.dart';

import '../api/api_client.dart';
import '../api/api_client_factory.dart';
import '../api/api_exception.dart';
import '../api/http_api_client.dart';
import '../model/routine.dart';
import '../module/module_catalog.dart';
import '../platform/reminders.dart';
import '../screens/routine_actions.dart';
import '../session/session_store.dart';
import 'reminder_plan.dart';
import 'replan_reminders.dart';

// „Ja“ and „Nein“ on a reminder without opening the app (path A, docs/API.md
// 3.16, reminders, point 6). Android and iOS run the handler below in an
// isolate of its own, also when the app is closed; it has no access to the
// app's memory, only to the secure storage and the network.

/// The logic of a reminder button, testable without a phone: reads the
/// token from [store], sends the log with `source: notification`, and on
/// `201` or `409 ROUTINE_LOG_EXISTS` (already done, e.g. in the app)
/// removes the reminder. Returns `true` on success.
///
/// After a success it plans the reminders of [modules] anew with the same
/// token ([replanReminders]): the app plans only seven days ahead, and
/// someone who answers only reminders and never opens the app would get
/// none after a week. If that fails, the old plan stays and the result is
/// still `true`.
///
/// On any other outcome (no token, expired, no network, an error of the
/// backend) nothing is changed: the reminder stays, and a tap on it opens
/// the app at the occurrence, where the error is visible.
Future<bool> logFromReminderButton(
  ReminderTap tap, {
  required SessionStore store,
  required ApiClient Function() createApi,
  required Reminders reminders,
  required List<String> modules,
  required DateTime now,
  required bool isIOS,
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
    }
    if (tap.notificationId case final id?) await reminders.cancel(id);
    try {
      await replanReminders(
        api: api,
        modules: modules,
        reminders: reminders,
        now: now,
        isIOS: isIOS,
      );
    } on Exception {
      // The old plan stays; nothing of the routines is logged.
    }
    return true;
  } finally {
    api.close();
  }
}

/// Name of the port through which the background tells a running app that
/// a reminder button logged something (dart:ui IsolateNameServer).
const reminderPortName = 'lifelog_reminder_logged';

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
      // With this handler, so that the reminders planned anew here also
      // send „Ja“ and „Nein“ in the background (path A), as in the app.
      reminders: PluginReminders(backgroundButtons: reminderButtonInBackground),
      modules: ModuleCatalog.defaults().activeIds,
      now: DateTime.now().toUtc(),
      isIOS: defaultTargetPlatform == TargetPlatform.iOS,
    ).then((logged) {
      // Only after planning anew: if the app is running, its screens
      // reload, and it plans once more (which settles any overlap).
      if (logged) {
        IsolateNameServer.lookupPortByName(reminderPortName)?.send('logged');
      }
    }),
  );
}
