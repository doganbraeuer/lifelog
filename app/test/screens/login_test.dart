// Login screen and the start of the session (docs/API.md 3.10, 3.12).
// All passwords and tokens are dummies or the mock's synthetic ones.
import 'package:flutter/material.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:lifelog/api/api_exception.dart';
import 'package:lifelog/api/mock_api_client.dart';
import 'package:lifelog/model/user.dart';
import 'package:lifelog/session/session_controller.dart';
import 'package:lifelog/session/session_store.dart';

import '../helpers.dart';

const admin = MockApiClient.mockAdminPassword;

/// Answers every login with a lockout of [seconds].
class _LockedMock extends MockApiClient {
  _LockedMock(this.seconds) : super(delay: Duration.zero);

  final int seconds;

  @override
  Future<LoginResult> login(String username, String password) async =>
      throw ApiException(
        httpStatus: 429,
        code: ErrorCode.tooManyAttempts,
        message:
            'Zu viele Fehlversuche. Bitte versuche es in einigen Minuten '
            'erneut.',
        details: {'retryAfterSeconds': seconds},
      );
}

/// `GET /me` fails without network while [offline] is `true`.
class _OfflineMock extends MockApiClient {
  _OfflineMock() : super(delay: Duration.zero);

  var offline = true;

  @override
  Future<User> me() async {
    if (offline) {
      throw const ApiException(
        code: ErrorCode.networkError,
        message: 'Keine Verbindung zum Server.',
      );
    }
    return super.me();
  }
}

void main() {
  group('login screen', () {
    testWidgets('fields, hint and no „Passwort vergessen“ button', (
      tester,
    ) async {
      await startSignedOut(tester, fastMock(signedIn: false));

      expect(usernameField(), findsOneWidget);
      expect(passwordField(), findsOneWidget);
      expect(find.text('Passwort vergessen? Melde dich bei uns.'), findsOne);
      expect(
        find.widgetWithText(TextButton, 'Passwort vergessen'),
        findsNothing,
      );

      final username = tester.widget<TextField>(usernameField());
      expect(username.autocorrect, isFalse);
      expect(username.enableSuggestions, isFalse);
      expect(username.textCapitalization, TextCapitalization.none);
    });

    testWidgets('the eye shows and hides the password', (tester) async {
      await startSignedOut(tester, fastMock(signedIn: false));
      bool obscured() => tester.widget<TextField>(passwordField()).obscureText;

      expect(obscured(), isTrue);
      await tester.tap(find.byTooltip('Passwort anzeigen'));
      await tester.pump();
      expect(obscured(), isFalse);
      await tester.tap(find.byTooltip('Passwort verbergen'));
      await tester.pump();
      expect(obscured(), isTrue);
    });

    testWidgets('login works: token stored, consent next', (tester) async {
      final store = await startSignedOut(tester, fastMock(signedIn: false));

      await logIn(tester, 'Admin', admin);

      expect(store.session, isNotNull);
      expect(store.session!.token, hasLength(43));
      expect(find.text('Bevor du startest'), findsOneWidget);
    });

    testWidgets('wrong password: the message, nothing stored', (tester) async {
      final store = await startSignedOut(tester, fastMock(signedIn: false));

      await logIn(tester, 'admin', 'falsch-falsch');

      expect(find.text('Benutzername oder Passwort stimmt nicht.'), findsOne);
      expect(store.session, isNull);
      // The password is typed again from scratch.
      expect(tester.widget<TextField>(passwordField()).controller!.text, '');
    });

    testWidgets('empty fields: asked in the app, no request', (tester) async {
      await startSignedOut(tester, fastMock(signedIn: false));
      await tester.tap(find.widgetWithText(FilledButton, 'Anmelden'));
      await tester.pump();
      expect(find.text('Bitte Benutzername und Passwort eingeben.'), findsOne);
    });

    testWidgets('429 after 5 attempts: the message and „1 Minute“', (
      tester,
    ) async {
      await startSignedOut(tester, fastMock(signedIn: false));
      for (var i = 0; i < 5; i++) {
        await logIn(tester, 'admin', 'falsch-falsch');
      }
      await logIn(tester, 'admin', admin);

      expect(
        find.text(
          'Zu viele Fehlversuche. Bitte versuche es in einigen Minuten '
          'erneut. Bitte warte etwa 1 Minute.',
        ),
        findsOneWidget,
      );
    });

    testWidgets('429 with 150 s: „3 Minuten“ (rounded up)', (tester) async {
      await startSignedOut(tester, _LockedMock(150));
      await logIn(tester, 'admin', admin);
      expect(find.textContaining('Bitte warte etwa 3 Minuten.'), findsOne);
    });
  });

  group('start with a token', () {
    testWidgets('valid token: straight to the modules', (tester) async {
      await tester.pumpWidget(testApp(fastMock()));
      await tester.pumpAndSettle();
      expect(find.text('Gesundheit'), findsOneWidget);
    });

    testWidgets('401 at /me: token deleted, login with notice', (tester) async {
      final store = FakeSessionStore(
        StoredSession(token: 'unknown-dummy-token', expiresAt: farFuture),
      );
      await tester.pumpWidget(testApp(fastMock(signedIn: false), store: store));
      await tester.pumpAndSettle();

      expect(store.session, isNull);
      expect(usernameField(), findsOneWidget);
      expect(find.text(reloginNotice), findsOneWidget);
    });

    testWidgets('no network at /me: token kept, „Erneut versuchen“', (
      tester,
    ) async {
      final mock = _OfflineMock();
      final store = signedInStore(mock);
      await tester.pumpWidget(testApp(mock, store: store));
      await tester.pumpAndSettle();

      expect(find.text('Keine Verbindung zum Server.'), findsOneWidget);
      expect(store.session, isNotNull);

      mock.offline = false;
      await tester.tap(find.text('Erneut versuchen'));
      await tester.pumpAndSettle();
      expect(find.text('Gesundheit'), findsOneWidget);
    });

    testWidgets('expired token: login without asking the server', (
      tester,
    ) async {
      final mock = fastMock(signedIn: false);
      final store = FakeSessionStore(
        StoredSession(
          token: mock.signInForTest('admin'),
          expiresAt: DateTime.utc(2000),
        ),
      );
      await tester.pumpWidget(testApp(mock, store: store));
      await tester.pumpAndSettle();

      expect(store.session, isNull);
      expect(find.text(reloginNotice), findsOneWidget);
    });
  });
}
