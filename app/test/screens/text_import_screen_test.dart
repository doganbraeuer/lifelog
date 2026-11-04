import 'package:flutter/material.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:lifelog/api/mock_api_client.dart';
import 'package:lifelog/screens/eintrag_screen.dart';
import 'package:lifelog/screens/originaldatei_screen.dart';
import 'package:lifelog/screens/zeitleiste_screen.dart';

import '../hilfen.dart';

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
    expect(await mock.eintraege(modul: 'gesundheit'), hasLength(4));

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
      verzoegerung: Duration.zero,
      uploadVerzoegerung: const Duration(seconds: 2),
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
    final vorhanden = await mock.textImportieren(text, modul: 'gesundheit');
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
    expect(find.textContaining(vorhanden.datei.dateiname), findsWidgets);
  });

  testWidgets('wiederholbarer Fehler: „Erneut versuchen“, Text bleibt', (
    tester,
  ) async {
    await zumTextfeld(tester, schnellerMock());
    await tester.enterText(textFeld(), 'Test modellfehler');
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
      'Test modellfehler',
    );
  });
}
