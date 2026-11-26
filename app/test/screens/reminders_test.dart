// Reminders in the app (docs/API.md 3.16, "Reminders in the app"), with a
// fake instead of the phone's notifications. The mock's clock is
// 2026-11-04 09:30 UTC (10:30 in Vienna); sample routines „Vitamin D“
// (done today), „Blutdruck messen“ (08:00, 20:00) and „Wasser“ (hourly).
import 'package:flutter/material.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:lifelog/api/mock_api_client.dart';
import 'package:lifelog/model/routine.dart';
import 'package:lifelog/platform/reminders.dart';
import 'package:lifelog/reminders/reminder_plan.dart';
import 'package:lifelog/reminders/routine_sync.dart';
import 'package:lifelog/screens/occurrence_sheet.dart';
import 'package:lifelog/screens/routines_screen.dart';

import '../helpers.dart';
import 'routines_screen_test.dart' show inCard, navItem;

Future<void> startApp(
  WidgetTester tester,
  MockApiClient mock,
  FakeReminders reminders,
) async {
  useLargeScreen(tester);
  await tester.pumpWidget(testApp(mock, reminders: reminders));
  await tester.pumpAndSettle();
}

Future<String> idOf(MockApiClient mock, String name) async =>
    (await mock.listRoutines(module: 'health'))
        .firstWhere((r) => r.name == name)
        .id;

void main() {
  testWidgets('the session start plans the open occurrences ahead', (
    tester,
  ) async {
    final reminders = FakeReminders();
    await startApp(tester, fastMock(), reminders);

    final plan = reminders.planned;
    expect(plan, hasLength(maxPlannedReminders));
    expect(plan.every((r) => r.dueAt.isAfter(mockNow)), isTrue);
    // The nearest: water at 11:00 in Vienna.
    expect(plan.first.dueAt, DateTime.utc(2026, 11, 4, 10));
    expect(plan.first.title, 'Wasser');
    expect(plan.first.body, '11:00 · 250 ml – erledigt?');
    final pressure = plan.firstWhere((r) => r.title == 'Blutdruck messen');
    expect(pressure.dueAt, DateTime.utc(2026, 11, 4, 19));
    expect(pressure.body, '20:00 – erledigt?');
    // Vitamin D is done today; its next reminder is tomorrow.
    final vitamin = plan.firstWhere((r) => r.title == 'Vitamin D');
    expect(vitamin.dueAt, DateTime.utc(2026, 11, 5, 7));
  });

  testWidgets('without the permission: a notice, no plan; „Einschalten“', (
    tester,
  ) async {
    final reminders = FakeReminders(permissionAnswer: ReminderPermission.denied)
      ..requestAnswer = ReminderPermission.granted;
    await startApp(tester, fastMock(), reminders);
    expect(reminders.plans, isEmpty);

    await openHealth(tester);
    await tester.tap(navItem('Routinen'));
    await tester.pumpAndSettle();
    expect(
      find.text(
        'Erinnerungen sind ausgeschaltet. Du kannst Routinen trotzdem '
        'eintragen.',
      ),
      findsOneWidget,
    );
    // Logging works anyway.
    expect(inCard('Blutdruck messen', find.text('Ja')), findsOne);

    await tester.tap(find.text('Einschalten'));
    await tester.pumpAndSettle();
    expect(reminders.requests, 1);
    expect(find.byKey(const ValueKey('reminders-off')), findsNothing);
    expect(reminders.planned, isNotEmpty);
  });

  testWidgets('Android asks no more: the settings open, with a hint', (
    tester,
  ) async {
    final reminders = FakeReminders(
      permissionAnswer: ReminderPermission.denied,
    );
    await startApp(tester, fastMock(), reminders);
    await openHealth(tester);
    await tester.tap(navItem('Routinen'));
    await tester.pumpAndSettle();
    await tester.tap(find.text('Einschalten'));
    await tester.pumpAndSettle();
    expect(reminders.settingsOpened, 1);
    expect(
      find.text(
        'Bitte erlaube Mitteilungen unter Einstellungen › Apps › Lifelog › '
        'Benachrichtigungen.',
      ),
      findsOneWidget,
    );
  });

  testWidgets('the first routine asks for the permission, with a reason', (
    tester,
  ) async {
    final reminders = FakeReminders(permissionAnswer: ReminderPermission.denied)
      ..requestAnswer = ReminderPermission.granted;
    await startApp(tester, fastMock(withSampleData: false), reminders);
    await openHealth(tester);
    await tester.tap(navItem('Routinen'));
    await tester.pumpAndSettle();
    await tester.tap(find.text('Neue Routine').last);
    await tester.pumpAndSettle();
    await tester.enterText(
      find.byKey(const ValueKey('routine-name')),
      'Vitamin D',
    );
    await tester.ensureVisible(find.text('Speichern'));
    await tester.tap(find.text('Speichern'));
    await tester.pumpAndSettle();

    expect(find.text('Erinnerungen'), findsOneWidget);
    expect(reminders.requests, 0, reason: 'only after „Weiter“');
    await tester.tap(find.text('Weiter'));
    await tester.pumpAndSettle();
    expect(reminders.requests, 1);
    expect(reminders.planned, isNotEmpty);
  });

  testWidgets('logout removes every reminder', (tester) async {
    final reminders = FakeReminders();
    await startApp(tester, fastMock(), reminders);
    expect(reminders.clears, 0);
    await tester.tap(find.byTooltip('Einstellungen'));
    await tester.pumpAndSettle();
    await tester.tap(find.text('Abmelden'));
    await tester.pumpAndSettle();
    await tester.tap(find.widgetWithText(FilledButton, 'Abmelden'));
    await tester.pumpAndSettle();
    expect(reminders.clears, 1);
    expect(reminders.planned, isEmpty);
  });

  testWidgets('an ended session (401) removes them too', (tester) async {
    final reminders = FakeReminders();
    final mock = fastMock();
    await startApp(tester, mock, reminders);
    await openHealth(tester);
    mock.expireAllSessions();
    await tester.tap(navItem('Routinen'));
    await tester.pumpAndSettle();
    expect(find.text('Bitte melde dich erneut an.'), findsOneWidget);
    expect(reminders.clears, 1);
  });

  testWidgets('an app start without a session removes old reminders', (
    tester,
  ) async {
    final reminders = FakeReminders();
    await tester.pumpWidget(
      testApp(
        fastMock(signedIn: false),
        store: FakeSessionStore(),
        reminders: reminders,
      ),
    );
    await tester.pumpAndSettle();
    expect(reminders.clears, 1);
  });

  testWidgets('a tap on a reminder opens its occurrence', (tester) async {
    final reminders = FakeReminders();
    final mock = fastMock();
    await startApp(tester, mock, reminders);
    final pressure = await idOf(mock, 'Blutdruck messen');

    reminders.onTap!(
      ReminderTap(
        module: 'health',
        routineId: pressure,
        dueAt: DateTime.utc(2026, 11, 4, 7),
      ),
    );
    await tester.pumpAndSettle();
    expect(find.byType(RoutinesScreen), findsOneWidget);
    expect(find.byType(OccurrenceSheet), findsOneWidget);
    await tester.tap(find.text('Ja, erledigt'));
    await tester.pumpAndSettle();

    final log = (await mock.listRoutineLogs(pressure)).single;
    expect(log.source, LogSource.notification);
    expect(log.dueAt, DateTime.utc(2026, 11, 4, 7));
  });

  testWidgets('„Ja“ on a reminder logs at once (path B)', (tester) async {
    final reminders = FakeReminders();
    final mock = fastMock();
    await startApp(tester, mock, reminders);
    final pressure = await idOf(mock, 'Blutdruck messen');
    final plansBefore = reminders.plans.length;

    reminders.onTap!(
      ReminderTap(
        module: 'health',
        routineId: pressure,
        dueAt: DateTime.utc(2026, 11, 4, 7),
        actionId: reminderActionDone,
      ),
    );
    await tester.pumpAndSettle();
    expect(find.text('„Blutdruck messen“ erledigt.'), findsOneWidget);
    expect(find.byType(OccurrenceSheet), findsNothing);
    final log = (await mock.listRoutineLogs(pressure)).single;
    expect(log.source, LogSource.notification);
    expect(log.status, LogStatus.done);
    expect(
      reminders.cancelled,
      contains(
        reminderId(occurrenceKey(pressure, DateTime.utc(2026, 11, 4, 7))),
      ),
    );
    expect(reminders.plans.length, greaterThan(plansBefore));
  });

  testWidgets('„Ja“ on a reminder of an occurrence done elsewhere: no error', (
    tester,
  ) async {
    final reminders = FakeReminders();
    final mock = fastMock();
    await startApp(tester, mock, reminders);
    final vitamin = await idOf(mock, 'Vitamin D'); // done today at 08:00
    reminders.onTap!(
      ReminderTap(
        module: 'health',
        routineId: vitamin,
        dueAt: DateTime.utc(2026, 11, 4, 7),
        actionId: reminderActionDone,
      ),
    );
    await tester.pumpAndSettle();
    // Already done: the sheet says so instead of an error.
    expect(find.text('Schon eingetragen: erledigt.'), findsOneWidget);
    expect((await mock.listRoutineLogs(vitamin)), hasLength(1));
  });

  testWidgets('„Andere Menge“ on a reminder: the sheet with the amount', (
    tester,
  ) async {
    final reminders = FakeReminders();
    final mock = fastMock();
    await startApp(tester, mock, reminders);
    final water = await idOf(mock, 'Wasser');
    reminders.onTap!(
      ReminderTap(
        module: 'health',
        routineId: water,
        dueAt: DateTime.utc(2026, 11, 4, 8),
        actionId: reminderActionAmount,
      ),
    );
    await tester.pumpAndSettle();
    final field = find.byKey(const ValueKey('occurrence-amount'));
    expect(tester.widget<TextField>(field).autofocus, isTrue);
  });

  testWidgets('the reminder that started the app is shown', (tester) async {
    final mock = fastMock();
    final reminders = FakeReminders()
      ..launchedBy = ReminderTap(
        module: 'health',
        routineId: await idOf(mock, 'Blutdruck messen'),
        dueAt: DateTime.utc(2026, 11, 4, 7),
      );
    await startApp(tester, mock, reminders);
    expect(find.byType(OccurrenceSheet), findsOneWidget);
  });

  testWidgets('a module open on another tab switches to „Routinen“', (
    tester,
  ) async {
    final reminders = FakeReminders();
    final mock = fastMock();
    await startApp(tester, mock, reminders);
    await openHealth(tester);
    expect(find.text('Zeitleiste'), findsWidgets);
    reminders.onTap!(
      ReminderTap(
        module: 'health',
        routineId: await idOf(mock, 'Blutdruck messen'),
        dueAt: DateTime.utc(2026, 11, 4, 7),
      ),
    );
    await tester.pumpAndSettle();
    expect(find.byType(OccurrenceSheet), findsOneWidget);
    // Back from the sheet: still one module, on the routines tab.
    await tester.tap(find.text('Nein, ausgelassen'));
    await tester.pumpAndSettle();
    expect(find.byType(RoutinesScreen), findsOneWidget);
    expect(find.text('„Blutdruck messen“ ausgelassen.'), findsOneWidget);
  });

  group('found in the phone test', () {
    Future<(MockApiClient, String)> openTab(WidgetTester tester) async {
      final mock = fastMock();
      await startApp(tester, mock, FakeReminders());
      await openHealth(tester);
      await tester.tap(navItem('Routinen'));
      await tester.pumpAndSettle();
      expect(inCard('Blutdruck messen', find.text('heute 0 von 2')), findsOne);
      return (mock, await idOf(mock, 'Blutdruck messen'));
    }

    Future<void> logBehindTheBack(MockApiClient mock, String id) =>
        mock.addRoutineLog(
          id,
          NewRoutineLog(
            status: LogStatus.done,
            source: LogSource.notification,
            dueAt: DateTime.utc(2026, 11, 4, 7),
          ),
        );

    testWidgets('a reminder button logged in the background: the tab '
        'reloads', (tester) async {
      final (mock, id) = await openTab(tester);
      await logBehindTheBack(mock, id);
      RoutineSyncScope.maybeOf(tester.element(find.byType(RoutinesScreen)))!
          .loggedInBackground();
      await tester.pumpAndSettle();
      expect(inCard('Blutdruck messen', find.text('heute 1 von 2')), findsOne);
    });

    testWidgets('back in the foreground: the tab reloads', (tester) async {
      final (mock, id) = await openTab(tester);
      await logBehindTheBack(mock, id);
      for (final state in [
        AppLifecycleState.inactive,
        AppLifecycleState.hidden,
        AppLifecycleState.paused,
        AppLifecycleState.hidden,
        AppLifecycleState.inactive,
        AppLifecycleState.resumed,
      ]) {
        tester.binding.handleAppLifecycleStateChanged(state);
      }
      await tester.pumpAndSettle();
      expect(inCard('Blutdruck messen', find.text('heute 1 von 2')), findsOne);
    });

    testWidgets('pausing removes the shown reminder of a paused occurrence', (
      tester,
    ) async {
      final reminders = FakeReminders();
      final mock = fastMock();
      await startApp(tester, mock, reminders);
      final pressure = await idOf(mock, 'Blutdruck messen');
      final tonight = reminderId(
        occurrenceKey(pressure, DateTime.utc(2026, 11, 4, 19)),
      );
      expect(reminders.removes.last, isNot(contains(tonight)));

      await openHealth(tester);
      await tester.tap(navItem('Routinen'));
      await tester.pumpAndSettle();
      await tester.tap(find.text('Blutdruck messen'));
      await tester.pumpAndSettle();
      await tester.scrollUntilVisible(
        find.text('Pausieren'),
        300,
        scrollable: find.byType(Scrollable).last,
      );
      await tester.tap(find.text('Pausieren'));
      await tester.pumpAndSettle();
      await tester.tap(find.text('Für eine Woche'));
      await tester.pumpAndSettle();

      expect(reminders.removes.last, contains(tonight));
      expect(reminders.planned.where((r) => r.routineId == pressure), isEmpty);
    });

    testWidgets('deleting a routine removes its shown reminders', (
      tester,
    ) async {
      final reminders = FakeReminders();
      final mock = fastMock();
      await startApp(tester, mock, reminders);
      final water = await idOf(mock, 'Wasser');
      final eleven = reminderId(
        occurrenceKey(water, DateTime.utc(2026, 11, 4, 10)),
      );
      await mock.deleteRoutine(water);
      await RoutineSyncScope.maybeOf(
        tester.element(find.byType(Scaffold).first),
      )!.refresh();
      await tester.pumpAndSettle();
      expect(reminders.removes.last, contains(eleven));
      expect(reminders.planned.where((r) => r.routineId == water), isEmpty);
    });

    testWidgets('a tap while the settings are open opens the module', (
      tester,
    ) async {
      final reminders = FakeReminders();
      final mock = fastMock();
      await startApp(tester, mock, reminders);
      await tester.tap(find.byTooltip('Einstellungen'));
      await tester.pumpAndSettle();
      reminders.onTap!(
        ReminderTap(
          module: 'health',
          routineId: await idOf(mock, 'Wasser'),
          dueAt: DateTime.utc(2026, 11, 4, 8),
          actionId: reminderActionAmount,
        ),
      );
      await tester.pumpAndSettle();
      expect(find.byType(OccurrenceSheet), findsOneWidget);
      expect(find.byType(RoutinesScreen), findsOneWidget);
    });
  });

  testWidgets('a reminder of a deleted routine: a short notice', (
    tester,
  ) async {
    final reminders = FakeReminders();
    await startApp(tester, fastMock(), reminders);
    reminders.onTap!(
      ReminderTap(
        module: 'health',
        routineId: '6742c0a1b2c3d4e5f6a7b8c9',
        dueAt: DateTime.utc(2026, 11, 4, 7),
      ),
    );
    await tester.pumpAndSettle();
    expect(find.text('Diese Routine gibt es nicht mehr.'), findsOneWidget);
  });

  testWidgets('debug: „Nächste Erinnerung jetzt zeigen“', (tester) async {
    final reminders = FakeReminders();
    await startApp(tester, fastMock(), reminders);
    await tester.tap(find.byTooltip('Einstellungen'));
    await tester.pumpAndSettle();
    await tester.tap(find.text('Nächste Erinnerung jetzt zeigen'));
    await tester.pumpAndSettle();
    expect(reminders.shownNow.single.title, 'Wasser');
  });
}
