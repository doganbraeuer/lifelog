import 'package:flutter/material.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:lifelog/api/api_exception.dart';
import 'package:lifelog/api/mock_api_client.dart';
import 'package:lifelog/screens/entry_screen.dart';
import 'package:lifelog/screens/question_screen.dart';
import 'package:lifelog/screens/timeline_screen.dart';

import '../helpers.dart';

const labor = 'Labor Muster GmbH, Salzburg';

Future<void> zumLaborbefund(WidgetTester tester, MockApiClient mock) async {
  grosserBildschirm(tester);
  await tester.pumpWidget(testApp(mock));
  await tester.pumpAndSettle();
  await zuGesundheit(tester);
  await tester.tap(find.text(labor));
  await tester.pumpAndSettle();
}

Future<void> loeschenTippen(WidgetTester tester) async {
  await tester.tap(find.byTooltip('Eintrag löschen'));
  await tester.pumpAndSettle();
}

Finder loeschenBestaetigen() => find.widgetWithText(FilledButton, 'Löschen');

void main() {
  testWidgets('Abbrechen löscht nichts', (tester) async {
    final mock = schnellerMock();
    await zumLaborbefund(tester, mock);

    await loeschenTippen(tester);
    expect(find.text('Eintrag löschen?'), findsOneWidget);
    expect(find.textContaining('endgültig'), findsOneWidget);

    await tester.tap(find.text('Abbrechen'));
    await tester.pumpAndSettle();

    expect(find.byType(EintragScreen), findsOneWidget);
    expect((await mock.eintrag(laborId)).id, laborId);
  });

  testWidgets('Bestätigen löscht, zurück in der Zeitleiste ohne Eintrag', (
    tester,
  ) async {
    final mock = schnellerMock();
    await zumLaborbefund(tester, mock);

    await loeschenTippen(tester);
    await tester.tap(loeschenBestaetigen());
    await tester.pumpAndSettle();

    expect(find.byType(EintragScreen), findsNothing);
    expect(find.byType(ZeitleisteScreen), findsOneWidget);
    expect(find.text('Eintrag gelöscht.'), findsOneWidget);
    expect(find.text(labor), findsNothing);
    await expectLater(
      mock.eintrag(laborId),
      wirftApiFehler(FehlerCode.eintragNichtGefunden),
    );
  });

  testWidgets('Fehler: Meldung, Ansicht bleibt offen', (tester) async {
    final mock = schnellerMock();
    await zumLaborbefund(tester, mock);

    await loeschenTippen(tester);
    mock.simulierterFehler = true;
    await tester.tap(loeschenBestaetigen());
    await tester.pumpAndSettle();

    expect(find.byType(EintragScreen), findsOneWidget);
    expect(find.textContaining('Fehler aufgetreten'), findsOneWidget);
    // Der Knopf ist wieder bedienbar.
    final knopf = tester.widget<IconButton>(
      find.ancestor(
        of: find.byIcon(Icons.delete_outline),
        matching: find.byType(IconButton),
      ),
    );
    expect(knopf.onPressed, isNotNull);

    mock.simulierterFehler = false;
    expect((await mock.eintrag(laborId)).id, laborId);
  });

  testWidgets('aus einer Quelle gelöscht: zurück zur Frage, Zeitleiste neu', (
    tester,
  ) async {
    final mock = schnellerMock();
    grosserBildschirm(tester);
    await tester.pumpWidget(testApp(mock));
    await tester.pumpAndSettle();
    await zuGesundheit(tester);
    await tester.tap(
      find.descendant(
        of: find.byType(NavigationBar),
        matching: find.text('Fragen'),
      ),
    );
    await tester.pumpAndSettle();
    await tester.enterText(find.byType(TextField), 'Hämoglobin');
    await tester.pump();
    await tester.tap(find.byTooltip('Frage senden'));
    await tester.pumpAndSettle();
    await tester.tap(find.text('Laborbefund · 14.10.2026'));
    await tester.pumpAndSettle();

    await loeschenTippen(tester);
    await tester.tap(loeschenBestaetigen());
    await tester.pumpAndSettle();

    expect(find.byType(FrageScreen), findsOneWidget);
    await tester.tap(
      find.descendant(
        of: find.byType(NavigationBar),
        matching: find.text('Zeitleiste'),
      ),
    );
    await tester.pumpAndSettle();
    expect(find.text(labor), findsNothing);
  });
}
