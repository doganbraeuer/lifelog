import 'package:flutter/material.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:lifelog/api/mock_api_client.dart';
import 'package:lifelog/screens/entry_screen.dart';
import 'package:lifelog/screens/original_file_screen.dart';
import 'package:lifelog/screens/timeline_screen.dart';

import '../helpers.dart';

const text =
    'Hausarztpraxis Beispiel\n'
    'Überweisung vom 21.10.2026\n'
    'zur Physiotherapie';

Finder textFeld() => find.widgetWithText(TextField, 'Dokumentinhalt');
Finder importieren() => find.widgetWithText(FilledButton, 'Importieren');

/// Startet die App, öffnet Hochladen und das Textfeld.
Future<void> zumTextfeld(WidgetTester tester, MockApiClient mock) async {
  grosserBildschirm(tester);
  await tester.pumpWidget(testApp(mock));
  await tester.pumpAndSettle();
  await zuGesundheit(tester);
  await tester.tap(
    find.descendant(
      of: find.byType(NavigationBar),
      matching: find.text('Hochladen'),
    ),
  );
  await tester.pumpAndSettle();
  await tester.tap(find.text('Text eingeben'));
  await tester.pumpAndSettle();
}

FilledButton knopf(WidgetTester tester, Finder finder) =>
    tester.widget<FilledButton>(finder);

void main() {
  testWidgets('Importieren erst mit Text möglich', (tester) async {
    await zumTextfeld(tester, schnellerMock());

    expect(textFeld(), findsOneWidget);
    expect(find.text('0/100000'), findsOneWidget);
    expect(knopf(tester, importieren()).onPressed, isNull);

    await tester.enterText(textFeld(), '   \n ');
    await tester.pump();
    expect(knopf(tester, importieren()).onPressed, isNull);

    await tester.enterText(textFeld(), text);
    await tester.pump();
    expect(knopf(tester, importieren()).onPressed, isNotNull);
  });

  testWidgets('Text importieren: Eintrag öffnet sich, Originaldatei ist der '
      'Text', (tester) async {
    final mock = schnellerMock();
    await zumTextfeld(tester, mock);
    await tester.enterText(textFeld(), text);
    await tester.pump();
    await tester.tap(importieren());
    await tester.pumpAndSettle();

    expect(find.byType(EintragScreen), findsOneWidget);
    expect(find.text('Text gespeichert.'), findsOneWidget);
    expect(find.text('Hausarztpraxis Beispiel'), findsOneWidget); // Quelle
    expect(find.text('21.10.2026'), findsOneWidget);
    expect(find.textContaining('· Text'), findsOneWidget);
    expect(await mock.listEntries(module: 'health'), hasLength(4));

    await tester.tap(find.text('Originaldatei öffnen'));
    await tester.pumpAndSettle();
    expect(find.byType(OriginaldateiScreen), findsOneWidget);
    expect(find.textContaining('zur Physiotherapie'), findsWidgets);

    // Zurück bis in die Zeitleiste; dort steht der neue Eintrag.
    await zurueck(tester);
    await zurueck(tester);
    expect(find.byType(ZeitleisteScreen), findsOneWidget);
    expect(find.text('Hausarztpraxis Beispiel'), findsOneWidget);
  });

  testWidgets('Ladeanzeige, Textfeld gesperrt', (tester) async {
    final mock = MockApiClient(
      delay: Duration.zero,
      uploadDelay: const Duration(seconds: 2),
    );
    await zumTextfeld(tester, mock);
    await tester.enterText(textFeld(), text);
    await tester.pump();
    await tester.tap(importieren());
    await tester.pump(const Duration(milliseconds: 100));

    expect(find.textContaining('bis zu einer Minute'), findsOneWidget);
    expect(tester.widget<TextField>(textFeld()).enabled, isFalse);

    await tester.pump(const Duration(seconds: 2));
    await tester.pumpAndSettle();
    expect(find.byType(EintragScreen), findsOneWidget);
  });

  testWidgets('Abbrechen schließt das Feld und verwirft den Text', (
    tester,
  ) async {
    await zumTextfeld(tester, schnellerMock());
    await tester.enterText(textFeld(), text);
    await tester.tap(find.text('Abbrechen'));
    await tester.pumpAndSettle();

    expect(textFeld(), findsNothing);
    expect(find.text('Foto aufnehmen'), findsOneWidget);

    await tester.tap(find.text('Text eingeben'));
    await tester.pumpAndSettle();
    expect(tester.widget<TextField>(textFeld()).controller!.text, isEmpty);
  });

  testWidgets('Duplikat: Meldung, Sprung möglich, Text bleibt stehen', (
    tester,
  ) async {
    final mock = schnellerMock();
    final vorhanden = await mock.importText(text, module: 'health');
    await zumTextfeld(tester, mock);
    await tester.enterText(textFeld(), text);
    await tester.pump();
    await tester.tap(importieren());
    await tester.pumpAndSettle();

    expect(find.text('Diesen Text hast du schon importiert.'), findsOneWidget);
    expect(tester.widget<TextField>(textFeld()).controller!.text, text);
    // Kein neuer Versuch sinnvoll: Knopf heißt weiter „Importieren“.
    expect(importieren(), findsOneWidget);

    await tester.tap(find.text('Vorhandenen Eintrag öffnen'));
    await tester.pumpAndSettle();
    expect(find.byType(EintragScreen), findsOneWidget);
    expect(find.textContaining(vorhanden.file.fileName), findsWidgets);
  });

  testWidgets('Handygröße, Tastatur offen, langer Text: Knöpfe erreichbar', (
    tester,
  ) async {
    final mock = schnellerMock();
    await tester.pumpWidget(testApp(mock));
    await tester.pumpAndSettle();
    await zuGesundheit(tester);

    // Wie das OnePlus 8T: 1080 × 2400 px bei Faktor 3, also 360 × 800.
    // Gboard ist dort 897 px hoch (gemessen per Screenshot), also 299.
    // Erst nach „Bereiche“ umstellen: Die Kacheln dort laufen in der
    // Testschrift bei 360 px Breite über (eigenes Thema).
    tester.view.physicalSize = const Size(1080, 2400);
    tester.view.devicePixelRatio = 3;
    addTearDown(tester.view.reset);
    await tester.pumpAndSettle();

    await tester.tap(
      find.descendant(
        of: find.byType(NavigationBar),
        matching: find.text('Hochladen'),
      ),
    );
    await tester.pumpAndSettle();
    await tester.tap(find.text('Text eingeben'));
    await tester.pumpAndSettle();

    // Feld antippen, Tastatur geht auf, langer Text wird eingegeben.
    await tester.tap(textFeld());
    tester.view.viewInsets = const FakeViewPadding(bottom: 897);
    await tester.enterText(
      textFeld(),
      [for (var i = 1; i <= 20; i++) 'Zeile $i Laborwert Beispiel'].join('\n'),
    );
    await tester.pumpAndSettle();

    // Sichtbar ist nur der Bereich zwischen AppBar und Tastatur.
    final oben = tester
        .getRect(find.widgetWithText(AppBar, 'Hochladen'))
        .bottom;
    const unten = (2400 - 897) / 3;
    for (final knopf in [importieren(), find.text('Abbrechen')]) {
      final rect = tester.getRect(knopf);
      expect(rect.top, greaterThanOrEqualTo(oben), reason: '$knopf oben');
      expect(rect.bottom, lessThanOrEqualTo(unten), reason: '$knopf unten');
    }

    // Und der Knopf tut, was er soll.
    await tester.tap(importieren());
    await tester.pumpAndSettle();
    expect(find.byType(EintragScreen), findsOneWidget);
  });

  testWidgets('wiederholbarer Fehler: „Erneut versuchen“, Text bleibt', (
    tester,
  ) async {
    await zumTextfeld(tester, schnellerMock());
    await tester.enterText(textFeld(), 'Test modelerror');
    await tester.pump();
    await tester.tap(importieren());
    await tester.pumpAndSettle();

    expect(find.textContaining('nicht erreichbar'), findsOneWidget);
    expect(
      find.widgetWithText(FilledButton, 'Erneut versuchen'),
      findsOneWidget,
    );
    expect(
      tester.widget<TextField>(textFeld()).controller!.text,
      'Test modelerror',
    );
  });
}
