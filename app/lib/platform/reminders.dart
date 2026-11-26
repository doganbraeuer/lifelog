import 'package:flutter/foundation.dart';
import 'package:flutter/services.dart';
import 'package:flutter_local_notifications/flutter_local_notifications.dart';
import 'package:timezone/timezone.dart' as tz;

import '../reminders/reminder_plan.dart';

/// Whether the app may show reminders.
enum ReminderPermission { granted, denied }

/// Local reminders on the phone (docs/API.md 3.16, "Reminders in the app").
/// Behind an interface so that tests can use a fake.
abstract interface class Reminders {
  /// Registers what happens when the user taps a reminder or a button.
  Future<void> initialize({required void Function(ReminderTap) onTap});

  /// The reminder that started the app, if any.
  Future<ReminderTap?> launchTap();

  Future<ReminderPermission> permission();

  /// Asks the system (Android 13 and later, iOS). Android asks at most
  /// twice; after that the answer stays „denied“ without a dialog.
  Future<ReminderPermission> requestPermission();

  /// Opens the app's notification settings, for a permission Android no
  /// longer asks for.
  Future<void> openSettings();

  /// Replaces the planned reminders by [plan]. Reminders already shown
  /// stay while [stillOpen] says their occurrence is still open; the others
  /// are removed.
  Future<void> apply(
    List<PlannedReminder> plan, {
    required bool Function(ReminderTap) stillOpen,
  });

  Future<void> cancel(int id);

  /// Removes every planned and shown reminder (logout).
  Future<void> clear();

  /// Shows a reminder at once (debug only, for trying it out).
  Future<void> showNow(PlannedReminder reminder);
}

/// [Reminders] with the package flutter_local_notifications.
///
/// - Approximate, not exact (`AndroidScheduleMode.inexactAllowWhileIdle`):
///   since Android 14 exact alarms are reserved for alarm clock and
///   calendar apps. A reminder can come some minutes late; the occurrence
///   keeps its `dueAt` (3.16, reminders, point 5).
/// - Planned with the point in time from the server in UTC; the phone
///   converts nothing.
/// - `visibility: private`: on a locked screen Android hides the content
///   (and with it the routine's name) if the phone is set to hide
///   sensitive content. The package has no `publicVersion`.
/// - After a restart the package's boot receiver plans the last list again
///   (AndroidManifest.xml); the next start of the app plans anew anyway.
class PluginReminders implements Reminders {
  PluginReminders({
    FlutterLocalNotificationsPlugin? plugin,
    this.backgroundButtons,
  }) : _plugin = plugin ?? FlutterLocalNotificationsPlugin();

  final FlutterLocalNotificationsPlugin _plugin;

  /// Path A: „Ja“ and „Nein“ send the log in the background, without
  /// opening the app; the reminder stays until it worked. `null` (the
  /// mock, whose data lives only in the app's memory): path B, the buttons
  /// open the app, which sends the log.
  final DidReceiveBackgroundNotificationResponseCallback? backgroundButtons;

  bool get _inBackground => backgroundButtons != null;

  static const channelId = 'routine_reminders';
  static const _category = 'routine_reminder';
  static const _categoryWithAmount = 'routine_reminder_amount';

  static bool get _iOS => defaultTargetPlatform == TargetPlatform.iOS;

  AndroidFlutterLocalNotificationsPlugin? get _android => _plugin
      .resolvePlatformSpecificImplementation<
        AndroidFlutterLocalNotificationsPlugin
      >();

  IOSFlutterLocalNotificationsPlugin? get _ios => _plugin
      .resolvePlatformSpecificImplementation<
        IOSFlutterLocalNotificationsPlugin
      >();

  @override
  Future<void> initialize({required void Function(ReminderTap) onTap}) async {
    // Without `foreground` iOS runs the button in the background.
    final open = _inBackground
        ? const <DarwinNotificationActionOption>{}
        : {DarwinNotificationActionOption.foreground};
    final darwinActions = [
      DarwinNotificationAction.plain(reminderActionDone, 'Ja', options: open),
      DarwinNotificationAction.plain(
        reminderActionSkipped,
        'Nein',
        options: open,
      ),
    ];
    await _plugin.initialize(
      settings: InitializationSettings(
        android: const AndroidInitializationSettings('ic_stat_lifelog'),
        iOS: DarwinInitializationSettings(
          // The app asks itself, after the first routine.
          requestAlertPermission: false,
          requestSoundPermission: false,
          requestBadgePermission: false,
          notificationCategories: [
            DarwinNotificationCategory(_category, actions: darwinActions),
            DarwinNotificationCategory(
              _categoryWithAmount,
              actions: [
                ...darwinActions,
                DarwinNotificationAction.plain(
                  reminderActionAmount,
                  'Andere Menge',
                  options: {DarwinNotificationActionOption.foreground},
                ),
              ],
            ),
          ],
        ),
      ),
      onDidReceiveNotificationResponse: (response) {
        final tap = ReminderTap.fromPayload(
          response.payload,
          actionId: response.actionId,
          notificationId: response.id,
        );
        if (tap != null) onTap(tap);
      },
      onDidReceiveBackgroundNotificationResponse: backgroundButtons,
    );
  }

  @override
  Future<ReminderTap?> launchTap() async {
    final details = await _plugin.getNotificationAppLaunchDetails();
    final response = details?.notificationResponse;
    if (details?.didNotificationLaunchApp != true || response == null) {
      return null;
    }
    return ReminderTap.fromPayload(
      response.payload,
      actionId: response.actionId,
      notificationId: response.id,
    );
  }

  @override
  Future<ReminderPermission> permission() async {
    final bool? enabled;
    if (_iOS) {
      enabled = (await _ios?.checkPermissions())?.isEnabled;
    } else {
      enabled = await _android?.areNotificationsEnabled();
    }
    return enabled == true
        ? ReminderPermission.granted
        : ReminderPermission.denied;
  }

  @override
  Future<ReminderPermission> requestPermission() async {
    final bool? granted;
    if (_iOS) {
      granted = await _ios?.requestPermissions(alert: true, sound: true);
    } else {
      granted = await _android?.requestNotificationsPermission();
    }
    return granted == true
        ? ReminderPermission.granted
        : ReminderPermission.denied;
  }

  @override
  Future<void> openSettings() async {
    try {
      await _plugin.openAppNotificationSettings();
    } on PlatformException {
      // No settings screen: the hint in the app names the way.
    }
  }

  NotificationDetails _details(PlannedReminder reminder) => NotificationDetails(
    android: AndroidNotificationDetails(
      channelId,
      'Erinnerungen an Routinen',
      channelDescription: 'Erinnert an die Termine deiner Routinen.',
      icon: 'ic_stat_lifelog',
      importance: Importance.high,
      priority: Priority.high,
      category: AndroidNotificationCategory.reminder,
      visibility: NotificationVisibility.private,
      actions: [
        // Path A: sent in the background; the reminder stays until the
        // log worked (the handler removes it). Path B: the app opens and
        // sends it.
        AndroidNotificationAction(
          reminderActionDone,
          'Ja',
          showsUserInterface: !_inBackground,
          cancelNotification: !_inBackground,
        ),
        AndroidNotificationAction(
          reminderActionSkipped,
          'Nein',
          showsUserInterface: !_inBackground,
          cancelNotification: !_inBackground,
        ),
        if (reminder.withAmountButton)
          const AndroidNotificationAction(
            reminderActionAmount,
            'Andere Menge',
            showsUserInterface: true,
          ),
      ],
    ),
    iOS: DarwinNotificationDetails(
      categoryIdentifier: reminder.withAmountButton
          ? _categoryWithAmount
          : _category,
    ),
  );

  @override
  Future<void> apply(
    List<PlannedReminder> plan, {
    required bool Function(ReminderTap) stillOpen,
  }) async {
    await _plugin.cancelAllPendingNotifications();
    try {
      for (final shown in await _plugin.getActiveNotifications()) {
        final tap = ReminderTap.fromPayload(shown.payload);
        final id = shown.id;
        if (tap != null && id != null && !stillOpen(tap)) {
          await _plugin.cancel(id: id);
        }
      }
    } on PlatformException {
      // Older systems cannot list shown notifications; they stay.
    }
    final now = DateTime.now().toUtc();
    for (final reminder in plan) {
      if (!reminder.dueAt.isAfter(now)) continue;
      await _plugin.zonedSchedule(
        id: reminder.id,
        scheduledDate: tz.TZDateTime.from(reminder.dueAt, tz.UTC),
        notificationDetails: _details(reminder),
        androidScheduleMode: AndroidScheduleMode.inexactAllowWhileIdle,
        title: reminder.title,
        body: reminder.body,
        payload: reminder.payload,
      );
    }
  }

  @override
  Future<void> cancel(int id) => _plugin.cancel(id: id);

  @override
  Future<void> clear() => _plugin.cancelAll();

  @override
  Future<void> showNow(PlannedReminder reminder) => _plugin.show(
    id: reminder.id,
    title: reminder.title,
    body: reminder.body,
    notificationDetails: _details(reminder),
    payload: reminder.payload,
  );
}
