// „Ja“/„Nein“ on a reminder in the background (path A): token from the
// secure storage, log with source notification, 409 counts as done, any
// failure leaves the reminder; after a success the reminders are planned
// anew from the server.
import 'package:flutter_test/flutter_test.dart';
import 'package:lifelog/api/api_exception.dart';
import 'package:lifelog/api/mock_api_client.dart';
import 'package:lifelog/model/plain_date.dart';
import 'package:lifelog/model/routine.dart';
import 'package:lifelog/reminders/reminder_action.dart';
import 'package:lifelog/reminders/reminder_plan.dart';
import 'package:lifelog/session/session_store.dart';

import '../helpers.dart';

/// The mock of [fastMock], whose occurrences can fail to load.
class _Mock extends MockApiClient {
  _Mock()
    : super(
        delay: Duration.zero,
        uploadDelay: Duration.zero,
        transcriptionDelay: Duration.zero,
        clock: () => mockNow,
      );

  var occurrencesFail = false;

  @override
  Future<OccurrenceList> listOccurrences({
    required String module,
    PlainDate? from,
    PlainDate? to,
  }) async {
    if (occurrencesFail) {
      throw const ApiException(
        code: ErrorCode.networkError,
        message: 'Keine Verbindung zum Server.',
      );
    }
    return super.listOccurrences(module: module, from: from, to: to);
  }
}

void main() {
  late _Mock mock;
  late String pressure;
  late FakeSessionStore store;
  late FakeReminders reminders;
  final due = DateTime.utc(2026, 11, 4, 7); // 08:00 in Vienna, today

  setUp(() async {
    mock = _Mock();
    final token = mock.signInForTest('admin');
    store = FakeSessionStore(StoredSession(token: token, expiresAt: farFuture));
    reminders = FakeReminders();
    mock.useToken(token);
    pressure = (await mock.listRoutines(module: 'health'))
        .firstWhere((r) => r.name == 'Blutdruck messen')
        .id;
    mock.useToken(null); // the background starts without the app's token
  });

  ReminderTap tap(String? action, {DateTime? dueAt}) => ReminderTap(
    module: 'health',
    routineId: pressure,
    dueAt: dueAt ?? due,
    actionId: action,
    notificationId: 4711,
  );

  Future<bool> press(String? action, {DateTime? dueAt}) =>
      logFromReminderButton(
        tap(action, dueAt: dueAt),
        store: store,
        createApi: () => mock,
        reminders: reminders,
        modules: const ['health'],
        now: mockNow,
        isIOS: false,
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

  test('after „Ja“ the reminders are planned anew from the server', () async {
    // Someone who only answers reminders never opens the app; without this
    // the seven planned days would run out.
    final tonight = DateTime.utc(2026, 11, 4, 19); // 20:00 in Vienna
    final tonightId = reminderId(occurrenceKey(pressure, tonight));
    expect(await press(reminderActionDone, dueAt: tonight), isTrue);

    mock.useToken(store.session!.token);
    final expected = planReminders(
      occurrences: (await mock.listOccurrences(module: 'health')).occurrences,
      routines: await mock.listRoutines(module: 'health'),
      now: mockNow,
      isIOS: false,
    );
    expect(expected, isNotEmpty);
    expect(reminders.plans, hasLength(1));
    expect(reminders.planned.map((r) => r.id), expected.map((r) => r.id));
    expect(
      reminders.planned.map((r) => r.id),
      isNot(contains(tonightId)),
      reason: 'done now',
    );
    expect(reminders.removes.single, contains(tonightId));
  });

  test('planning anew fails: the old plan stays, the log counts', () async {
    mock.occurrencesFail = true;
    expect(await press(reminderActionDone), isTrue);
    expect(reminders.cancelled, [4711]);
    expect(reminders.plans, isEmpty, reason: 'apply never called');
    expect((await logs()).single.status, LogStatus.done);
  });
}
