// Tab „Routinen“: today's occurrences with „Ja“, „Nein“ and „Andere Menge“,
// and the other routines with their status. The mock's clock is
// 2026-11-04 09:30 UTC (10:30 in Vienna, a Wednesday); its sample routines
// are „Vitamin D“ (done today), „Blutdruck messen“ and „Wasser“.
import 'package:flutter/material.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:lifelog/api/mock_api_client.dart';
import 'package:lifelog/model/plain_date.dart';
import 'package:lifelog/model/routine.dart';

import '../helpers.dart';

Finder navItem(String label) =>
    find.descendant(of: find.byType(NavigationBar), matching: find.text(label));

Finder card(String name) =>
    find.ancestor(of: find.text(name), matching: find.byType(Card)).first;

Finder inCard(String name, Finder finder) =>
    find.descendant(of: card(name), matching: finder);

Future<void> openRoutines(WidgetTester tester, MockApiClient mock) async {
  useLargeScreen(tester);
  await tester.pumpWidget(testApp(mock));
  await tester.pumpAndSettle();
  await openHealth(tester);
  await tester.tap(navItem('Routinen'));
  await tester.pumpAndSettle();
}

Future<String> idOf(MockApiClient mock, String name) async =>
    (await mock.listRoutines(module: 'health'))
        .firstWhere((r) => r.name == name)
        .id;

void main() {
  testWidgets('today: progress, the next occurrence and the buttons', (
    tester,
  ) async {
    await openRoutines(tester, fastMock());

    expect(find.text('Heute'), findsOneWidget);
    expect(inCard('Blutdruck messen', find.text('heute 0 von 2')), findsOne);
    expect(
      inCard('Blutdruck messen', find.text('Termin um 08:00 – noch offen')),
      findsOne,
    );
    expect(inCard('Blutdruck messen', find.text('Ja')), findsOne);
    expect(inCard('Blutdruck messen', find.text('Nein')), findsOne);
    expect(
      inCard('Blutdruck messen', find.text('Andere Menge')),
      findsNothing,
      reason: 'no amount',
    );

    expect(inCard('Vitamin D', find.text('heute 2 von 2 Stück')), findsOne);
    expect(inCard('Vitamin D', find.text('Für heute erledigt')), findsOne);

    expect(inCard('Wasser', find.text('heute 0 von 3.250 ml')), findsOne);
    expect(inCard('Wasser', find.text('0 von 13 Terminen erledigt')), findsOne);
    expect(inCard('Wasser', find.text('Andere Menge')), findsOne);
  });

  testWidgets('„Ja“ logs the occurrence; „Rückgängig“ takes it back', (
    tester,
  ) async {
    final mock = fastMock();
    await openRoutines(tester, mock);

    await tester.tap(inCard('Blutdruck messen', find.text('Ja')));
    await tester.pumpAndSettle();
    expect(find.text('„Blutdruck messen“ erledigt.'), findsOneWidget);
    expect(inCard('Blutdruck messen', find.text('heute 1 von 2')), findsOne);
    expect(inCard('Blutdruck messen', find.textContaining('20:00')), findsOne);
    final logs = await mock.listRoutineLogs(
      await idOf(mock, 'Blutdruck messen'),
    );
    expect(logs.single.source, LogSource.manual);
    expect(logs.single.dueAt, DateTime.utc(2026, 11, 4, 7));

    await tester.tap(find.text('Rückgängig'));
    await tester.pumpAndSettle();
    expect(inCard('Blutdruck messen', find.text('heute 0 von 2')), findsOne);
    expect(
      await mock.listRoutineLogs(await idOf(mock, 'Blutdruck messen')),
      isEmpty,
    );
  });

  testWidgets('the message with „Rückgängig“ goes by itself after 8 s', (
    tester,
  ) async {
    // Flutter keeps a snack bar with an action until it is tapped, unless
    // `persist: false`: it stayed over every page on the phone, and later
    // messages waited behind it.
    await openRoutines(tester, fastMock());
    await tester.tap(inCard('Blutdruck messen', find.text('Ja')));
    await tester.pumpAndSettle();
    expect(find.text('Rückgängig'), findsOneWidget);

    await tester.pump(const Duration(seconds: 7));
    await tester.pumpAndSettle();
    expect(find.text('Rückgängig'), findsOneWidget, reason: 'time to undo');

    await tester.pump(const Duration(seconds: 1));
    await tester.pumpAndSettle();
    expect(find.text('Rückgängig'), findsNothing);
    expect(find.text('„Blutdruck messen“ erledigt.'), findsNothing);
  });

  testWidgets('„Nein“ skips the occurrence', (tester) async {
    final mock = fastMock();
    await openRoutines(tester, mock);
    await tester.tap(inCard('Blutdruck messen', find.text('Nein')));
    await tester.pumpAndSettle();
    expect(find.text('„Blutdruck messen“ ausgelassen.'), findsOneWidget);
    expect(inCard('Blutdruck messen', find.text('heute 0 von 2')), findsOne);
    expect(inCard('Blutdruck messen', find.textContaining('20:00')), findsOne);
    final logs = await mock.listRoutineLogs(
      await idOf(mock, 'Blutdruck messen'),
    );
    expect(logs.single.status, LogStatus.skipped);
  });

  testWidgets('409: done elsewhere already is no error', (tester) async {
    final mock = fastMock();
    await openRoutines(tester, mock);
    // Meanwhile a reminder button (or another phone) logged it.
    await mock.addRoutineLog(
      await idOf(mock, 'Blutdruck messen'),
      NewRoutineLog(
        status: LogStatus.done,
        source: LogSource.notification,
        dueAt: DateTime.utc(2026, 11, 4, 7),
      ),
    );

    await tester.tap(inCard('Blutdruck messen', find.text('Ja')));
    await tester.pumpAndSettle();
    expect(find.text('Dieser Termin ist schon eingetragen.'), findsOneWidget);
    expect(find.text('Rückgängig'), findsNothing);
    expect(inCard('Blutdruck messen', find.text('heute 1 von 2')), findsOne);
  });

  testWidgets('„Andere Menge“: the sheet with the amount', (tester) async {
    final mock = fastMock();
    await openRoutines(tester, mock);
    await tester.tap(inCard('Wasser', find.text('Andere Menge')));
    await tester.pumpAndSettle();

    final field = find.byKey(const ValueKey('occurrence-amount'));
    expect(tester.widget<TextField>(field).controller!.text, '250');
    await tester.enterText(field, 'viel');
    await tester.tap(find.text('Menge speichern'));
    await tester.pumpAndSettle();
    expect(
      find.text('Bitte eine Menge eingeben, z. B. 250 oder 0,5.'),
      findsOneWidget,
    );

    await tester.enterText(field, '300');
    await tester.tap(find.text('Menge speichern'));
    await tester.pumpAndSettle();
    expect(find.text('„Wasser“ erledigt.'), findsOneWidget);
    expect(inCard('Wasser', find.text('heute 300 von 3.250 ml')), findsOne);
  });

  testWidgets('an error of the backend is shown', (tester) async {
    final mock = fastMock();
    await openRoutines(tester, mock);
    mock.simulateError = true;
    await tester.tap(inCard('Blutdruck messen', find.text('Ja')));
    await tester.pumpAndSettle();
    expect(
      find.text(
        'Im Backend ist ein Fehler aufgetreten. '
        'Bitte später noch einmal versuchen.',
      ),
      findsWidgets,
    );
  });

  testWidgets('loading fails: error with „Erneut versuchen“', (tester) async {
    final mock = fastMock();
    useLargeScreen(tester);
    await tester.pumpWidget(testApp(mock));
    await tester.pumpAndSettle();
    await openHealth(tester);
    // The tab is built (and loads) only when it is shown.
    mock.simulateError = true;
    await tester.tap(navItem('Routinen'));
    await tester.pumpAndSettle();
    expect(find.text('Erneut versuchen'), findsWidgets);
    mock.simulateError = false;
    await tester.tap(find.text('Erneut versuchen').last);
    await tester.pumpAndSettle();
    expect(find.text('Heute'), findsOneWidget);
  });

  testWidgets('no routines: an explanation', (tester) async {
    await openRoutines(tester, fastMock(withSampleData: false));
    expect(find.text('Noch keine Routinen'), findsOneWidget);
  });

  testWidgets('other routines: paused, upcoming, ended, not today', (
    tester,
  ) async {
    final mock = fastMock(withSampleData: false);
    Future<Routine> create(String name, Map<String, dynamic> extra) =>
        mock.createRoutineFromJson({
          'name': name,
          'schedule': {
            'frequency': 'daily',
            'interval': 1,
            'times': ['08:00'],
          },
          'startDate': '2026-11-01',
          ...extra,
        });
    final paused = await create('Pausiert', {});
    await mock.pauseRoutine(paused.id, until: PlainDate(2026, 11, 10));
    await create('Später', {'startDate': '2026-11-20'});
    await create('Vorbei', {
      'startDate': '2026-10-01',
      'endDate': '2026-10-10',
    });
    await create('Freitags', {
      'schedule': {
        'frequency': 'weekly',
        'interval': 1,
        'weekdays': ['fri'],
        'times': ['07:00'],
      },
    });

    await openRoutines(tester, mock);
    expect(find.text('Heute'), findsNothing);
    expect(find.text('Weitere Routinen'), findsOneWidget);
    expect(
      find.text('pausiert bis 10.11.2026\nTäglich um 08:00'),
      findsOneWidget,
    );
    expect(
      find.text('beginnt am 20.11.2026\nTäglich um 08:00'),
      findsOneWidget,
    );
    expect(
      find.text('beendet am 10.10.2026\nTäglich um 08:00'),
      findsOneWidget,
    );
    expect(
      find.text(
        'Nächster Termin: Fr, 06.11., 07:00\nWöchentlich am Fr um 07:00',
      ),
      findsOneWidget,
    );
  });
}
