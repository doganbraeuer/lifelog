// „Ja“/„Nein“ on a reminder in the background (path A): token from the
// secure storage, log with source notification, 409 counts as done, any
// failure leaves the reminder.
import 'package:flutter_test/flutter_test.dart';
import 'package:lifelog/api/mock_api_client.dart';
import 'package:lifelog/model/routine.dart';
import 'package:lifelog/reminders/reminder_action.dart';
import 'package:lifelog/reminders/reminder_plan.dart';
import 'package:lifelog/session/session_store.dart';

import '../helpers.dart';

void main() {
  late MockApiClient mock;
  late String pressure;
  late FakeSessionStore store;
  late FakeReminders reminders;
  final due = DateTime.utc(2026, 11, 4, 7); // 08:00 in Vienna, today

  setUp(() async {
    mock = fastMock(signedIn: false);
    final token = mock.signInForTest('admin');
    store = FakeSessionStore(StoredSession(token: token, expiresAt: farFuture));
    reminders = FakeReminders();
    mock.useToken(token);
    pressure = (await mock.listRoutines(module: 'health'))
        .firstWhere((r) => r.name == 'Blutdruck messen')
        .id;
    mock.useToken(null); // the background starts without the app's token
  });

  ReminderTap tap(String? action) => ReminderTap(
    module: 'health',
    routineId: pressure,
    dueAt: due,
    actionId: action,
    notificationId: 4711,
  );

  Future<bool> press(String? action) => logFromReminderButton(
    tap(action),
    store: store,
    createApi: () => mock,
    reminders: reminders,
    now: mockNow,
  );

  Future<List<RoutineLog>> logs() async {
    mock.useToken(store.session!.token);
    return mock.listRoutineLogs(pressure);
  }

  test('„Ja“: done with source notification, the reminder goes', () async {
    expect(await press(reminderActionDone), isTrue);
    final log = (await logs()).single;
    expect(log.status, LogStatus.done);
    expect(log.source, LogSource.notification);
    expect(log.dueAt, due);
    expect(reminders.cancelled, [4711]);
  });

  test('„Nein“: skipped', () async {
    expect(await press(reminderActionSkipped), isTrue);
    expect((await logs()).single.status, LogStatus.skipped);
  });

  test('409, already done in the app: success, the reminder goes', () async {
    expect(await press(reminderActionDone), isTrue);
    reminders.cancelled.clear();
    expect(await press(reminderActionDone), isTrue);
    expect((await logs()), hasLength(1));
    expect(reminders.cancelled, [4711]);
  });

  test('an error of the backend: the reminder stays', () async {
    mock.simulateError = true;
    expect(await press(reminderActionDone), isFalse);
    expect(reminders.cancelled, isEmpty);
    mock.simulateError = false;
    expect(await logs(), isEmpty);
  });

  test('no token or an expired one: nothing is sent', () async {
    final token = store.session!.token;
    store.session = null;
    expect(await press(reminderActionDone), isFalse);
    store.session = StoredSession(
      token: token,
      expiresAt: mockNow.subtract(const Duration(minutes: 1)),
    );
    expect(await press(reminderActionDone), isFalse);
    store.session = StoredSession(token: token, expiresAt: farFuture);
    expect(await logs(), isEmpty);
    expect(reminders.cancelled, isEmpty);
  });

  test('a tap on the reminder itself is not a button', () async {
    expect(await press(null), isFalse);
    expect(await press(reminderActionAmount), isFalse);
    expect(await logs(), isEmpty);
  });
}
