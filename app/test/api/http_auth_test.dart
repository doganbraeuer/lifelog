// Token and login in the HttpApiClient (docs/API.md section 1 "Login and
// tokens", 3.10–3.15). All tokens and passwords here are dummies.
import 'dart:convert';
import 'dart:io';

import 'package:flutter_test/flutter_test.dart';
import 'package:http/http.dart' as http;
import 'package:http/testing.dart';
import 'package:lifelog/api/api_exception.dart';
import 'package:lifelog/api/http_api_client.dart';
import 'package:lifelog/model/entry_patch.dart';
import 'package:lifelog/model/user.dart';
import 'package:lifelog/session/session_store.dart';

import '../helpers.dart';
import '../model/entry_test.dart' show sample;
import 'http_api_client_test.dart' show jsonResponse, testBase;

const dummyToken = 'dummy-token-for-tests-only';

const userJson = {
  'id': '673b0c1fa2d4e5f6a7b8c9d0',
  'username': 'tester01',
  'displayName': 'Erika',
  'mustChangePassword': false,
  'consentVersion': '2026-11-18',
};

/// A fitting success response for every endpoint, so that every method of
/// the client runs through.
Future<http.Response> anyEndpoint(http.Request request) async {
  final path = request.url.path.replaceFirst('/api/v1', '');
  return switch ((request.method, path)) {
    ('GET', '/entries') => jsonResponse({'entries': [], 'count': 0}),
    ('GET', '/status') => jsonResponse({
      'status': 'ok',
      'database': 'ok',
      'speech': 'ok',
    }),
    ('POST', '/auth/login') => jsonResponse({
      'token': dummyToken,
      'expiresAt': '2026-12-18T09:15:42Z',
      'mustChangePassword': true,
      'user': {
        'id': '673b0c1fa2d4e5f6a7b8c9d0',
        'username': 'tester01',
        'displayName': 'tester01',
        'consentVersion': null,
      },
    }),
    ('GET' || 'PATCH', '/me') => jsonResponse(userJson),
    ('POST', '/questions') => jsonResponse({
      'answer': 'Dazu finde ich nichts in deinen Dokumenten.',
      'found': false,
      'sources': [],
    }),
    ('POST', '/transcriptions') => jsonResponse({
      'text': 'Hallo.',
      'durationSeconds': 1.0,
      'language': 'de',
      'engine': 'x',
    }),
    ('GET', final p) when p.endsWith('/file') => http.Response.bytes(
      [1, 2, 3],
      200,
      headers: {'content-type': 'application/pdf'},
    ),
    ('GET' || 'PATCH', _) || ('POST', '/entries') => jsonResponse(sample()),
    _ => http.Response('', 204),
  };
}

/// Every method of the ApiClient that needs a token, with a call.
final protectedCalls = <String, Future<Object?> Function(HttpApiClient)>{
  'listEntries': (api) => api.listEntries(module: 'health'),
  'getEntry': (api) => api.getEntry(labReportId),
  'getFile': (api) => api.getFile(labReportId),
  'upload': (api) =>
      api.upload(testPdf('a.pdf').bytes, 'a.pdf', module: 'health'),
  'importText': (api) => api.importText('Notiz', module: 'health'),
  'correct': (api) =>
      api.correct(labReportId, const EntryPatch(issuer: Value('X'))),
  'deleteEntry': (api) => api.deleteEntry(labReportId),
  'ask': (api) => api.ask('Wie war der Wert?', module: 'health'),
  'transcribe': (api) => api.transcribe(testM4a(), 'note.m4a'),
  'logout': (api) => api.logout(),
  'me': (api) => api.me(),
  'updateDisplayName': (api) => api.updateDisplayName('Erika'),
  'changePassword': (api) =>
      api.changePassword('dummy-current-1', 'dummy-new-12345'),
  'recordConsent': (api) => api.recordConsent('2026-11-18'),
};

/// The two public endpoints: never with a token.
final publicCalls = <String, Future<Object?> Function(HttpApiClient)>{
  'getStatus': (api) => api.getStatus(),
  'login': (api) => api.login('tester01', 'dummy-password'),
};

/// Names of all async methods in the interface, read from the source. A
/// new method without an entry in the tables above fails the test.
Set<String> interfaceMethods() {
  final source = File('lib/api/api_client.dart').readAsStringSync();
  return RegExp(r'Future<.*?>\s+(\w+)\s*\(')
      .allMatches(source)
      .map((m) => m[1]!)
      .toSet();
}

void main() {
  group('Authorization header', () {
    test('every method of the interface is in a table', () {
      final methods = interfaceMethods();
      expect(methods, isNotEmpty);
      expect(
        {...protectedCalls.keys, ...publicCalls.keys},
        equals(methods),
        reason: 'Add a new ApiClient method to protectedCalls or publicCalls',
      );
    });

    for (final MapEntry(key: name, value: call) in protectedCalls.entries) {
      test('$name sends Bearer <token>', () async {
        final headers = <String?>[];
        final api = HttpApiClient(
          baseUrl: testBase,
          client: MockClient((request) {
            headers.add(request.headers['authorization']);
            return anyEndpoint(request);
          }),
        )..useToken(dummyToken);
        await call(api);
        expect(headers, ['Bearer $dummyToken']);
      });
    }

    for (final MapEntry(key: name, value: call) in publicCalls.entries) {
      test('$name sends no token', () async {
        final requests = <http.Request>[];
        final api = HttpApiClient(
          baseUrl: testBase,
          client: MockClient((request) {
            requests.add(request);
            return anyEndpoint(request);
          }),
        )..useToken(dummyToken);
        await call(api);
        expect(requests, hasLength(1));
        expect(requests.single.headers.containsKey('authorization'), isFalse);
      });
    }

    test('without a token no header at all', () async {
      late http.Request captured;
      final api = HttpApiClient(
        baseUrl: testBase,
        client: MockClient((request) {
          captured = request;
          return anyEndpoint(request);
        }),
      );
      await api.listEntries(module: 'health');
      expect(captured.headers.containsKey('authorization'), isFalse);
    });
  });

  group('login and logout', () {
    test('login: request as in 3.10, token used afterwards', () async {
      final requests = <http.Request>[];
      final api = HttpApiClient(
        baseUrl: testBase,
        client: MockClient((request) {
          requests.add(request);
          return anyEndpoint(request);
        }),
      );
      final result = await api.login('tester01', 'dummy-password');

      expect(requests.first.url.path, '/api/v1/auth/login');
      expect(jsonDecode(requests.first.body), {
        'username': 'tester01',
        'password': 'dummy-password',
      });
      expect(result.token, dummyToken);
      expect(result.expiresAt, DateTime.utc(2026, 12, 18, 9, 15, 42));
      // mustChangePassword sits next to `user` in the response.
      expect(result.user.mustChangePassword, isTrue);
      expect(result.user.consentVersion, isNull);

      await api.me();
      expect(requests.last.headers['authorization'], 'Bearer $dummyToken');
    });

    test('logout forgets the token, also after a 500', () async {
      final requests = <http.Request>[];
      final api = HttpApiClient(
        baseUrl: testBase,
        client: MockClient((request) async {
          requests.add(request);
          if (request.url.path.endsWith('/auth/logout')) {
            return jsonResponse({
              'error': {'code': 'INTERNAL_ERROR', 'message': 'Fehler.'},
            }, 500);
          }
          return anyEndpoint(request);
        }),
      )..useToken(dummyToken);

      await expectLater(api.logout(), throwsApiException('INTERNAL_ERROR'));
      await api.listEntries(module: 'health');
      expect(requests.last.headers.containsKey('authorization'), isFalse);
    });

    test('me, display name, password and consent as in 3.12–3.15', () async {
      final requests = <http.Request>[];
      final api = HttpApiClient(
        baseUrl: testBase,
        client: MockClient((request) {
          requests.add(request);
          return anyEndpoint(request);
        }),
      )..useToken(dummyToken);

      final user = await api.updateDisplayName('Erika');
      await api.changePassword('dummy-current-1', 'dummy-new-12345');
      await api.recordConsent('2026-11-18');

      expect(user.displayName, 'Erika');
      expect(user.consentVersion, '2026-11-18');
      expect(
        [for (final r in requests) '${r.method} ${r.url.path}'],
        [
          'PATCH /api/v1/me',
          'POST /api/v1/me/password',
          'PUT /api/v1/me/consent',
        ],
      );
      expect(jsonDecode(requests[0].body), {'displayName': 'Erika'});
      expect(jsonDecode(requests[1].body), {
        'currentPassword': 'dummy-current-1',
        'newPassword': 'dummy-new-12345',
      });
      expect(jsonDecode(requests[2].body), {'version': '2026-11-18'});
    });
  });

  group('error codes', () {
    HttpApiClient failing(
      int status,
      String code, {
      Map<String, dynamic>? details,
      Map<String, String> headers = const {},
    }) => HttpApiClient(
      baseUrl: testBase,
      client: MockClient(
        (_) async => http.Response.bytes(
          utf8.encode(
            jsonEncode({
              'error': {
                'code': code,
                'message': 'Meldung vom Server.',
                'details': ?details,
              },
            }),
          ),
          status,
          headers: {'content-type': 'application/json', ...headers},
        ),
      ),
    );

    for (final (status, code) in [
      (401, ErrorCode.unauthorized),
      (403, ErrorCode.passwordChangeRequired),
      (401, ErrorCode.invalidCredentials),
      (400, ErrorCode.wrongPassword),
      (400, ErrorCode.weakPassword),
      (429, ErrorCode.limitReached),
    ]) {
      test('$status $code', () async {
        await expectLater(
          failing(status, code).me(),
          throwsApiException(code, status: status),
        );
      });
    }

    test('429 TOO_MANY_ATTEMPTS: retryAfterSeconds from details', () async {
      final api = failing(
        429,
        ErrorCode.tooManyAttempts,
        details: {'retryAfterSeconds': 120},
        headers: {'retry-after': '120'},
      );
      final error = await api
          .login('tester01', 'dummy-password')
          .then<ApiException?>(
            (_) => null,
            onError: (Object e) => e as ApiException,
          );
      expect(error!.code, ErrorCode.tooManyAttempts);
      expect(error.httpStatus, 429);
      expect(error.retryAfterSeconds, 120);
      expect(error.message, 'Meldung vom Server.');
    });

    test('429 TOO_MANY_ATTEMPTS: Retry-After if details lack it', () async {
      final api = failing(
        429,
        ErrorCode.tooManyAttempts,
        headers: {'retry-after': '61'},
      );
      final error = await api
          .login('tester01', 'dummy-password')
          .then<ApiException?>(
            (_) => null,
            onError: (Object e) => e as ApiException,
          );
      expect(error!.retryAfterSeconds, 61);
    });

    test('other errors have no retryAfterSeconds', () async {
      final error = await failing(429, ErrorCode.limitReached)
          .ask('x', module: 'health')
          .then<ApiException?>(
            (_) => null,
            onError: (Object e) => e as ApiException,
          );
      expect(error!.retryAfterSeconds, isNull);
    });

    test('the token never appears in an error', () async {
      final api = failing(500, ErrorCode.internalError)..useToken(dummyToken);
      final error = await api.me().then<ApiException?>(
        (_) => null,
        onError: (Object e) => e as ApiException,
      );
      expect(
        '$error ${error!.message} ${error.details}',
        isNot(contains(dummyToken)),
      );
    });
  });

  group('no secret in toString', () {
    test('LoginResult and StoredSession hide the token', () {
      final result = LoginResult(
        token: dummyToken,
        expiresAt: farFuture,
        user: User.fromJson(userJson),
      );
      final stored = StoredSession(token: dummyToken, expiresAt: farFuture);
      expect('$result', isNot(contains(dummyToken)));
      expect('$stored', isNot(contains(dummyToken)));
    });
  });
}
