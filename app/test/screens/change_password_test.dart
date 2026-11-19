// „Eigenes Passwort festlegen“ after the first login (docs/API.md 3.14).
import 'package:flutter/material.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:lifelog/api/mock_api_client.dart';
import 'package:lifelog/screens/change_password_screen.dart';

import '../helpers.dart';

const start = MockApiClient.mockStartPassword;
const newPassword = 'Sonnenblume am Berg';

Finder currentField() => find.widgetWithText(TextField, 'Bisheriges Passwort');
Finder newField() => find.widgetWithText(TextField, 'Neues Passwort');
Finder repeatField() =>
    find.widgetWithText(TextField, 'Neues Passwort wiederholen');

Future<void> fillIn(
  WidgetTester tester, {
  String current = start,
  required String next,
  String? repeat,
}) async {
  await tester.enterText(currentField(), current);
  await tester.enterText(newField(), next);
  await tester.enterText(repeatField(), repeat ?? next);
  await tester.tap(find.widgetWithText(FilledButton, 'Passwort speichern'));
  await tester.pumpAndSettle();
}

/// Logs in as tester01 with the start password.
Future<MockApiClient> toMandatoryChange(WidgetTester tester) async {
  final mock = fastMock(signedIn: false);
  await startSignedOut(tester, mock);
  await logIn(tester, 'tester01', start);
  return mock;
}

void main() {
  testWidgets('start password leads to „Eigenes Passwort festlegen“', (
    tester,
  ) async {
    await toMandatoryChange(tester);
    expect(find.text('Eigenes Passwort festlegen'), findsOneWidget);
    // No back arrow: the only way out is „Abmelden“.
    expect(find.byTooltip('Zurück'), findsNothing);
    expect(find.widgetWithText(TextButton, 'Abmelden'), findsOneWidget);
  });

  for (final (name, next, repeat, message) in [
    (
      'too short',
      'kurz12345',
      null,
      'Das neue Passwort muss mindestens 10 Zeichen lang sein.',
    ),
    (
      'umlauts over 72 bytes',
      'ä' * 37,
      null,
      'Das neue Passwort ist zu lang. Erlaubt sind höchstens 72 Byte, '
          'Umlaute zählen doppelt.',
    ),
    (
      'repetition differs',
      newPassword,
      'Sonnenblume am Berk',
      'Die beiden neuen Passwörter stimmen nicht überein.',
    ),
    (
      'equal to the start password',
      start,
      null,
      'Das neue Passwort muss sich vom bisherigen unterscheiden.',
    ),
  ]) {
    testWidgets('checked in the app: $name', (tester) async {
      final mock = await toMandatoryChange(tester);
      await fillIn(tester, next: next, repeat: repeat);

      expect(find.text(message), findsOneWidget);
      // Nothing was sent: the start password still applies.
      expect(mock.userForTest('tester01').mustChangePassword, isTrue);
    });
  }

  testWidgets('wrong current password: the server message', (tester) async {
    await toMandatoryChange(tester);
    await fillIn(tester, current: 'falsch-falsch', next: newPassword);
    expect(find.text('Das bisherige Passwort stimmt nicht.'), findsOneWidget);
  });

  testWidgets('success: message, then the consent notice', (tester) async {
    final mock = await toMandatoryChange(tester);
    await fillIn(tester, next: newPassword);

    expect(find.text(passwordChangedMessage), findsOneWidget);
    expect(find.text('Bevor du startest'), findsOneWidget);
    expect(mock.userForTest('tester01').mustChangePassword, isFalse);
  });

  testWidgets('„Abmelden“ instead of changing', (tester) async {
    final mock = fastMock(signedIn: false);
    final store = await startSignedOut(tester, mock);
    await logIn(tester, 'tester01', start);

    await tester.tap(find.widgetWithText(TextButton, 'Abmelden'));
    await tester.pumpAndSettle();

    expect(store.session, isNull);
    expect(usernameField(), findsOneWidget);
  });
}
