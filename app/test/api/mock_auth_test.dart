// Accounts and login in the mock, as in docs/API.md 3.10–3.15.
import 'package:flutter_test/flutter_test.dart';
import 'package:lifelog/api/api_exception.dart';
import 'package:lifelog/api/mock_api_client.dart';

import '../helpers.dart';

const start = MockApiClient.mockStartPassword;
const admin = MockApiClient.mockAdminPassword;

void main() {
  test('without a token every call except status and login is 401', () async {
    final mock = fastMock(signedIn: false);
    await expectLater(
      mock.listEntries(module: 'health'),
      throwsApiException(ErrorCode.unauthorized, status: 401),
    );
    await expectLater(mock.me(), throwsApiException(ErrorCode.unauthorized));
    expect((await mock.getStatus()).status, 'ok');
  });

  test('login: username trimmed and lowercase, then usable', () async {
    final mock = fastMock(signedIn: false);
    final result = await mock.login(' Admin ', admin);
    expect(result.user.username, 'admin');
    expect(result.user.mustChangePassword, isFalse);
    expect(result.token, hasLength(43));
    expect(await mock.listEntries(module: 'health'), isNotEmpty);
  });

  test('wrong password and unknown user: same error', () async {
    final mock = fastMock(signedIn: false);
    for (final (name, password) in [
      ('admin', 'falsch-falsch'),
      ('niemand', admin),
    ]) {
      await expectLater(
        mock.login(name, password),
        throwsApiException(ErrorCode.invalidCredentials, status: 401),
      );
    }
  });

  test('after 5 failed attempts 429, even with the right password', () async {
    final mock = fastMock(signedIn: false);
    for (var i = 0; i < 5; i++) {
      await expectLater(
        mock.login('admin', 'falsch-falsch'),
        throwsApiException(ErrorCode.invalidCredentials),
      );
    }
    final error = await mock
        .login('admin', admin)
        .then<ApiException?>(
          (_) => null,
          onError: (Object e) => e as ApiException,
        );
    expect(error!.code, ErrorCode.tooManyAttempts);
    expect(error.httpStatus, 429);
    expect(error.retryAfterSeconds, 60);
    // Another username is not locked.
    await mock.login('tester01', start);
  });

  test('the lock ends after 60 s on the mock clock', () async {
    var now = DateTime.utc(2026, 11, 18, 10);
    final mock = fastMock(signedIn: false, clock: () => now);
    for (var i = 0; i < 5; i++) {
      await expectLater(
        mock.login('admin', 'falsch-falsch'),
        throwsApiException(ErrorCode.invalidCredentials),
      );
    }
    now = now.add(const Duration(seconds: 30));
    final error = await mock
        .login('admin', admin)
        .then<ApiException?>(
          (_) => null,
          onError: (Object e) => e as ApiException,
        );
    expect(error!.retryAfterSeconds, 30);
    now = now.add(const Duration(seconds: 30));
    await mock.login('admin', admin);
  });

  group('start password', () {
    test('everything except me, password and logout is 403', () async {
      final mock = fastMock(signedIn: false);
      final result = await mock.login('tester01', start);
      expect(result.user.mustChangePassword, isTrue);
      await expectLater(
        mock.listEntries(module: 'health'),
        throwsApiException(ErrorCode.passwordChangeRequired, status: 403),
      );
      await expectLater(
        mock.recordConsent('2026-11-18'),
        throwsApiException(ErrorCode.passwordChangeRequired),
      );
      expect((await mock.me()).mustChangePassword, isTrue);
    });

    test('rules and messages of 3.14', () async {
      final mock = fastMock(signedIn: false);
      await mock.login('tester01', start);
      for (final (current, next, code) in [
        ('falsch', 'Sonnenblume am Berg', ErrorCode.wrongPassword),
        (start, 'kurz', ErrorCode.weakPassword),
        (start, 'ä' * 37, ErrorCode.weakPassword),
        (start, start, ErrorCode.weakPassword),
      ]) {
        await expectLater(
          mock.changePassword(current, next),
          throwsApiException(code, status: 400),
        );
      }
    });

    test('after the change everything works; other devices are out', () async {
      final mock = fastMock(signedIn: false);
      final other = (await mock.login('tester01', start)).token;
      await mock.login('tester01', start);
      await mock.changePassword(start, 'Sonnenblume am Berg');

      expect((await mock.me()).mustChangePassword, isFalse);
      expect(await mock.listEntries(module: 'health'), isNotEmpty);

      mock.useToken(other);
      await expectLater(mock.me(), throwsApiException(ErrorCode.unauthorized));
    });
  });

  test('display name: trimmed, 1–50 characters', () async {
    final mock = fastMock();
    expect((await mock.updateDisplayName('  Erika ')).displayName, 'Erika');
    for (final bad in ['   ', 'x' * 51, 'Zeile\nzwei']) {
      await expectLater(
        mock.updateDisplayName(bad),
        throwsApiException(ErrorCode.invalidRequest, status: 400),
      );
    }
  });

  test('consent: stored, shown by me', () async {
    final mock = fastMock();
    mock.setConsentVersion('admin', null);
    expect((await mock.me()).consentVersion, isNull);
    await mock.recordConsent('2026-11-18');
    expect((await mock.me()).consentVersion, '2026-11-18');
  });

  test('logout: the token is invalid afterwards', () async {
    final mock = fastMock(signedIn: false);
    final token = (await mock.login('admin', admin)).token;
    await mock.logout();
    mock.useToken(token);
    await expectLater(mock.me(), throwsApiException(ErrorCode.unauthorized));
  });

  test('daily limit: upload, question and transcription answer 429', () async {
    final mock = fastMock();
    await expectLater(
      mock.importText('limitreached', module: 'health'),
      throwsApiException(ErrorCode.limitReached, status: 429),
    );
    await expectLater(
      mock.ask('Wert limitreached', module: 'health'),
      throwsApiException(ErrorCode.limitReached),
    );
    await expectLater(
      mock.transcribe(testM4a(), 'limitreached.m4a'),
      throwsApiException(ErrorCode.limitReached),
    );
  });
}
