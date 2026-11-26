// Settings with the large icon on the „Bereiche“ screen.
import 'package:flutter/material.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:lifelog/api/mock_api_client.dart';
import 'package:lifelog/screens/change_password_screen.dart';
import 'package:lifelog/screens/settings_screen.dart';

import '../helpers.dart';

Future<FakeSessionStore> openSettings(
  WidgetTester tester,
  MockApiClient mock, {
  FakeLocalData? localData,
}) async {
  final store = signedInStore(mock);
  await tester.pumpWidget(testApp(mock, store: store, localData: localData));
  await tester.pumpAndSettle();
  await tester.tap(find.byTooltip('Einstellungen'));
  await tester.pumpAndSettle();
  return store;
}

void main() {
  testWidgets('large, visible settings icon on the „Bereiche“ screen', (
    tester,
  ) async {
    await tester.pumpWidget(testApp(fastMock()));
    await tester.pumpAndSettle();

    final button = find.byTooltip('Einstellungen');
    expect(button, findsOneWidget);
    final icon = find.descendant(of: button, matching: find.byType(Icon));
    // Normal icons are 24 px; this one is clearly larger.
    expect(tester.getSize(icon).width, greaterThanOrEqualTo(32));
    expect(tester.getSize(button).width, greaterThanOrEqualTo(48));
  });

  testWidgets('shows who is logged in and every entry', (tester) async {
    await openSettings(tester, fastMock());

    expect(find.text('Angemeldet als admin (admin)'), findsOneWidget);
    for (final title in [
      'Anzeigename ändern',
      'Passwort ändern',
      'Hinweis zum Datenschutz',
      'Abmelden',
    ]) {
      expect(find.text(title), findsOneWidget, reason: title);
    }
  });

  testWidgets('change the display name', (tester) async {
    final mock = fastMock();
    await openSettings(tester, mock);

    await tester.tap(find.text('Anzeigename ändern'));
    await tester.pumpAndSettle();
    await tester.enterText(find.byType(TextField), '   ');
    await tester.tap(find.widgetWithText(FilledButton, 'Speichern'));
    await tester.pump();
    expect(
      find.text('Der Anzeigename muss 1 bis 50 Zeichen lang sein.'),
      findsOne,
    );

    await tester.enterText(find.byType(TextField), '  Erika ');
    await tester.tap(find.widgetWithText(FilledButton, 'Speichern'));
    await tester.pumpAndSettle();

    expect(find.text('Angemeldet als Erika (admin)'), findsOneWidget);
    expect(find.text('Anzeigename geändert.'), findsOneWidget);
    expect(mock.userForTest('admin').displayName, 'Erika');
  });

  testWidgets('change the password from the settings', (tester) async {
    final mock = fastMock();
    await openSettings(tester, mock);

    await tester.tap(find.text('Passwort ändern'));
    await tester.pumpAndSettle();
    await tester.enterText(
      find.widgetWithText(TextField, 'Bisheriges Passwort'),
      MockApiClient.mockAdminPassword,
    );
    await tester.enterText(
      find.widgetWithText(TextField, 'Neues Passwort'),
      'Sonnenblume am Berg',
    );
    await tester.enterText(
      find.widgetWithText(TextField, 'Neues Passwort wiederholen'),
      'Sonnenblume am Berg',
    );
    await tester.tap(find.widgetWithText(FilledButton, 'Passwort speichern'));
    await tester.pumpAndSettle();

    expect(find.text(passwordChangedMessage), findsOneWidget);
    // Back in the settings, still logged in.
    expect(find.byType(SettingsScreen), findsOneWidget);
    expect(find.text('Angemeldet als admin (admin)'), findsOneWidget);
  });

  testWidgets('privacy notice opens read-only', (tester) async {
    await openSettings(tester, fastMock());
    await tester.tap(find.text('Hinweis zum Datenschutz'));
    await tester.pumpAndSettle();

    expect(find.text('Bevor du startest'), findsOneWidget);
    expect(find.text('Zustimmen'), findsNothing);
    expect(find.text('Nicht jetzt'), findsNothing);
  });

  testWidgets('privacy notice shows the text of version 2026-11-26', (
    tester,
  ) async {
    await openSettings(tester, fastMock());
    await tester.tap(find.text('Hinweis zum Datenschutz'));
    await tester.pumpAndSettle();

    for (final part in [
      'beim Anbieter netcup in Deutschland',
      'bei MongoDB Atlas in Frankfurt gespeichert',
      'bei Sprachnotizen auch die Tonaufnahme selbst',
      'die Aufnahme speichern wir nicht',
      'Sie gehen nicht an Google Gemini',
      'der Name einer Routine in der Mitteilung',
      'Deine Dokumente und Routinen sind nur für dich sichtbar',
    ]) {
      expect(find.textContaining(part), findsOneWidget, reason: part);
    }
    await tester.scrollUntilVisible(find.text('Version 2026-11-26'), 200);
  });

  testWidgets('logout: token and local data deleted, back at the login', (
    tester,
  ) async {
    final localData = FakeLocalData();
    final store = await openSettings(tester, fastMock(), localData: localData);

    await tester.tap(find.text('Abmelden'));
    await tester.pumpAndSettle();
    await tester.tap(find.widgetWithText(FilledButton, 'Abmelden'));
    await tester.pumpAndSettle();

    expect(store.session, isNull);
    expect(localData.clears, 1);
    expect(usernameField(), findsOneWidget);
    // Every page is closed, no way back.
    expect(find.byType(SettingsScreen), findsNothing);
    expect(find.byTooltip('Zurück'), findsNothing);
  });

  testWidgets('logout can be cancelled', (tester) async {
    final store = await openSettings(tester, fastMock());

    await tester.tap(find.text('Abmelden'));
    await tester.pumpAndSettle();
    await tester.tap(find.widgetWithText(TextButton, 'Abbrechen'));
    await tester.pumpAndSettle();

    expect(store.session, isNotNull);
    expect(find.byType(SettingsScreen), findsOneWidget);
  });
}
