// The routine's detail. The mock's clock is 2026-11-04 09:30 UTC (10:30 in
// Vienna, a Wednesday); sample routines: „Vitamin D“ (done today),
// „Blutdruck messen“ (30.10. to 12.11.) and „Wasser“ (hourly, 250 ml).
import 'package:flutter/material.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:lifelog/api/mock_api_client.dart';
import 'package:lifelog/screens/routine_detail_screen.dart';

import '../helpers.dart';
import 'routines_screen_test.dart' show card, inCard, openRoutines;

Future<void> openDetail(
  WidgetTester tester,
  MockApiClient mock,
  String name,
) async {
  await openRoutines(tester, mock);
  await tester.tap(find.text(name));
  await tester.pumpAndSettle();
  expect(find.byType(RoutineDetailScreen), findsOneWidget);
}

/// Scrolls the detail until [finder] is built (the list builds lazily),
/// then taps it.
Future<void> tapVisible(WidgetTester tester, Finder finder) async {
  if (finder.evaluate().isEmpty) {
    await tester.scrollUntilVisible(
      finder,
      300,
      scrollable: find
          .descendant(
            of: find.byType(RoutineDetailScreen),
            matching: find.byType(Scrollable),
          )
          .first,
    );
  }
  await tester.ensureVisible(finder);
  await tester.pumpAndSettle();
  await tester.tap(finder);
  await tester.pumpAndSettle();
}

void main() {
  testWidgets('plan, progress and the occurrences of the next days', (
    tester,
  ) async {
    await openDetail(tester, fastMock(), 'Blutdruck messen');
    expect(find.text('aktiv'), findsOneWidget);
    expect(find.text('Täglich um 08:00 und 20:00'), findsOneWidget);
    expect(find.text('Beginn: 30.10.2026'), findsOneWidget);
    expect(find.text('Ende: 12.11.2026'), findsOneWidget);
    expect(find.text('heute 0 von 2'), findsOneWidget);
    expect(find.text('Woche 0 von 14'), findsOneWidget);
    expect(find.text('Tag 6 von 14'), findsOneWidget);
    expect(find.text('0 von 28 Terminen erledigt'), findsOneWidget);
    // Today's two occurrences can be logged, later ones not yet.
    expect(find.text('Eintragen'), findsNWidgets(2));
    expect(find.text('morgen, 08:00'), findsOneWidget);
    expect(
      find.text('In den letzten 30 Tagen gibt es keinen Verlaufseintrag.'),
      findsOneWidget,
    );
  });

  testWidgets('log an occurrence, then delete it from the history', (
    tester,
  ) async {
    await openDetail(tester, fastMock(), 'Blutdruck messen');
    await tapVisible(tester, find.text('Eintragen').first);
    await tester.tap(find.text('Ja, erledigt'));
    await tester.pumpAndSettle();
    expect(find.text('heute 1 von 2'), findsOneWidget);
    expect(find.textContaining('· erledigt'), findsOneWidget);
    expect(find.text('in der App'), findsOneWidget);

    await tapVisible(tester, find.byTooltip('Verlaufseintrag löschen'));
    expect(find.text('Verlaufseintrag löschen?'), findsOneWidget);
    await tester.tap(find.widgetWithText(FilledButton, 'Löschen'));
    await tester.pumpAndSettle();
    expect(find.text('heute 0 von 2'), findsOneWidget);
    expect(
      find.text('In den letzten 30 Tagen gibt es keinen Verlaufseintrag.'),
      findsOneWidget,
    );
  });

  testWidgets('„Zurücknehmen“ opens a done occurrence again', (tester) async {
    await openDetail(tester, fastMock(), 'Vitamin D');
    expect(find.text('heute 2 von 2 Stück'), findsOneWidget);
    await tapVisible(tester, find.text('Zurücknehmen'));
    expect(find.text('Eintrag zurücknehmen?'), findsOneWidget);
    await tester.tap(find.widgetWithText(FilledButton, 'Zurücknehmen'));
    await tester.pumpAndSettle();
    expect(find.text('heute 0 von 2 Stück'), findsOneWidget);
    expect(find.text('Eintragen'), findsOneWidget);
  });

  testWidgets('pause for a week, then resume', (tester) async {
    await openDetail(tester, fastMock(), 'Vitamin D');
    await tapVisible(tester, find.text('Pausieren'));
    await tester.tap(find.text('Für eine Woche'));
    await tester.pumpAndSettle();
    expect(find.text('„Vitamin D“ pausiert.'), findsOneWidget);
    expect(find.text('pausiert bis 10.11.2026'), findsOneWidget);

    await tapVisible(tester, find.text('Fortsetzen'));
    expect(find.text('„Vitamin D“ läuft wieder.'), findsOneWidget);
    expect(find.text('aktiv'), findsOneWidget);
  });

  testWidgets('pause until a date; the list shows it', (tester) async {
    await openDetail(tester, fastMock(), 'Vitamin D');
    await tapVisible(tester, find.text('Pausieren'));
    await tester.tap(find.text('Bis zu einem Datum …'));
    await tester.pumpAndSettle();
    await tester.tap(find.text('20'));
    await tester.tap(find.text('OK'));
    await tester.pumpAndSettle();
    expect(find.text('pausiert bis 20.11.2026'), findsOneWidget);

    await tapBack(tester);
    expect(find.text('Weitere Routinen'), findsOneWidget);
    expect(
      find.text('pausiert bis 20.11.2026\nTäglich um 08:00'),
      findsOneWidget,
    );
  });

  testWidgets('delete only after confirming', (tester) async {
    await openDetail(tester, fastMock(), 'Wasser');
    await tapVisible(tester, find.widgetWithText(OutlinedButton, 'Löschen'));
    expect(find.text('Routine löschen?'), findsOneWidget);
    await tester.tap(find.text('Abbrechen'));
    await tester.pumpAndSettle();
    expect(find.byType(RoutineDetailScreen), findsOneWidget);

    await tapVisible(tester, find.widgetWithText(OutlinedButton, 'Löschen'));
    await tester.tap(find.widgetWithText(FilledButton, 'Löschen'));
    await tester.pumpAndSettle();
    expect(find.byType(RoutineDetailScreen), findsNothing);
    expect(find.text('„Wasser“ gelöscht.'), findsOneWidget);
    expect(card('Blutdruck messen'), findsOne);
    expect(find.text('Wasser'), findsNothing);
  });

  testWidgets('an extra amount without an occurrence', (tester) async {
    await openDetail(tester, fastMock(), 'Wasser');
    await tapVisible(tester, find.text('Zusätzlich eintragen'));
    final field = find.byKey(const ValueKey('extra-amount'));
    expect(tester.widget<TextField>(field).controller!.text, '250');
    await tester.enterText(field, '300');
    await tester.tap(
      find.descendant(
        of: find.byType(AlertDialog),
        matching: find.widgetWithText(FilledButton, 'Eintragen'),
      ),
    );
    await tester.pumpAndSettle();
    expect(find.text('300 ml eingetragen.'), findsOneWidget);
    expect(find.text('heute 300 von 3.250 ml'), findsOneWidget);
    expect(find.text('zusätzlich, in der App'), findsOneWidget);
  });

  testWidgets('edit from the detail', (tester) async {
    await openDetail(tester, fastMock(), 'Wasser');
    await tapVisible(tester, find.text('Bearbeiten'));
    await tester.enterText(
      find.byKey(const ValueKey('routine-name')),
      'Wasser trinken',
    );
    await tapVisible(tester, find.text('Speichern'));
    expect(find.text('Änderungen gespeichert.'), findsOneWidget);
    expect(
      find.descendant(
        of: find.byType(AppBar),
        matching: find.text('Wasser trinken'),
      ),
      findsOneWidget,
    );

    await tapBack(tester);
    expect(inCard('Wasser trinken', find.text('Andere Menge')), findsOne);
  });
}
