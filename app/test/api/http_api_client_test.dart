import 'dart:convert';
import 'dart:typed_data';

import 'package:flutter_test/flutter_test.dart';
import 'package:http/http.dart' as http;
import 'package:http/testing.dart';
import 'package:lifelog/api/api_client.dart';
import 'package:lifelog/api/api_exception.dart';
import 'package:lifelog/api/http_api_client.dart';
import 'package:lifelog/model/plain_date.dart';
import 'package:lifelog/model/entry_patch.dart';
import 'package:lifelog/model/server_status.dart';

import '../helpers.dart';
import '../model/entry_test.dart' show sampleJson;

const testBase = 'http://test.local:8080';

/// Response with a JSON body as UTF-8, but deliberately without `charset`.
http.Response jsonResponse(Object json, [int status = 200]) =>
    http.Response.bytes(
      utf8.encode(jsonEncode(json)),
      status,
      headers: {'content-type': 'application/json'},
    );

HttpApiClient client(MockClientHandler handler, {String baseUrl = testBase}) =>
    HttpApiClient(baseUrl: baseUrl, client: MockClient(handler));

void main() {
  group('entries', () {
    test('URL with query parameters and response as in 3.2', () async {
      late http.Request captured;
      final api = client((request) async {
        captured = request;
        return jsonResponse({
          'entries': [
            {
              'id': labReportId,
              'module': 'health',
              'type': 'lab_report',
              'documentDate': '2026-10-14',
              'importedAt': '2026-11-03T18:42:07Z',
              'issuer': 'Labor Muster GmbH, Salzburg',
            },
            {
              'id': otherId,
              'module': 'health',
              'type': 'other',
              'documentDate': null,
              'importedAt': '2026-11-03T19:01:44Z',
              'issuer': null,
            },
          ],
          'count': 2,
        });
      });

      final list = await api.listEntries(
        sort: Sort.importedAt,
        order: SortOrder.asc,
        module: 'health',
      );

      expect(captured.method, 'GET');
      expect(
        captured.url.toString(),
        '$testBase/api/v1/entries'
        '?module=health&sort=importedAt&order=asc',
      );
      expect(list, hasLength(2));
      expect(list.first.documentDate, PlainDate(2026, 10, 14));
      expect(list.last.documentDate, isNull);
    });

    test('trailing slash in the base URL does no harm', () async {
      late Uri url;
      final api = client(baseUrl: '$testBase/', (request) async {
        url = request.url;
        return jsonResponse({'entries': [], 'count': 0});
      });
      expect(await api.listEntries(module: 'health'), isEmpty);
      expect(url.path, '/api/v1/entries');
    });
  });

  group('getEntry', () {
    test('path and umlauts, even without charset in the header', () async {
      late Uri url;
      final api = client((request) async {
        url = request.url;
        return http.Response.bytes(
          utf8.encode(sampleJson),
          200,
          headers: {'content-type': 'application/json'},
        );
      });
      final entry = await api.getEntry(labReportId);
      expect(url.toString(), '$testBase/api/v1/entries/$labReportId');
      expect(entry.facts.first.name, 'Hämoglobin');
    });

    test('error JSON becomes ApiException', () async {
      final api = client(
        (_) async => jsonResponse({
          'error': {
            'code': 'ENTRY_NOT_FOUND',
            'message': 'Diesen Eintrag gibt es nicht.',
            'details': {},
          },
        }, 404),
      );
      try {
        await api.getEntry('ffffffffffffffffffffffff');
        fail('Fehler erwartet');
      } on ApiException catch (f) {
        expect(f.httpStatus, 404);
        expect(f.code, ErrorCode.entryNotFound);
        expect(f.message, 'Diesen Eintrag gibt es nicht.');
      }
    });

    test('details are passed on', () async {
      final api = client(
        (_) async => jsonResponse({
          'error': {
            'code': 'DUPLICATE',
            'message': 'Diese Datei hast du schon hochgeladen.',
            'details': {'entryId': labReportId},
          },
        }, 409),
      );
      try {
        await api.upload(
          Uint8List.fromList('%PDF-'.codeUnits),
          'a.pdf',
          module: 'health',
        );
        fail('Fehler erwartet');
      } on ApiException catch (f) {
        expect(f.details['entryId'], labReportId);
      }
    });

    test(
      'error without JSON (e.g. HTML from a proxy): UNEXPECTED_RESPONSE',
      () async {
        final api = client(
          (_) async => http.Response('<html>Bad Gateway</html>', 502),
        );
        await expectLater(
          api.getEntry(labReportId),
          throwsApiException(ErrorCode.unexpectedResponse, status: 502),
        );
      },
    );

    test('success with mismatching JSON: UNEXPECTED_RESPONSE', () async {
      final api = client((_) async => jsonResponse({'foo': 1}));
      await expectLater(
        api.getEntry(labReportId),
        throwsApiException(ErrorCode.unexpectedResponse, status: 200),
      );
    });

    test('no connection: NETWORK_ERROR', () async {
      final api = client((_) async => throw http.ClientException('offline'));
      await expectLater(
        api.getEntry(labReportId),
        throwsApiException(ErrorCode.networkError),
      );
    });
  });

  test('getFile returns bytes and type without parameters', () async {
    late Uri url;
    final api = client((request) async {
      url = request.url;
      return http.Response.bytes(
        [1, 2, 3],
        200,
        headers: {'content-type': 'application/pdf; name=x'},
      );
    });
    final file = await api.getFile(labReportId);
    expect(url.path, '/api/v1/entries/$labReportId/file');
    expect(file.bytes, [1, 2, 3]);
    expect(file.mimeType, 'application/pdf');
    expect(file.isPdf, isTrue);
  });

  test('upload sends multipart with the field "file"', () async {
    late http.Request captured;
    final api = client((request) async {
      captured = request;
      return http.Response.bytes(
        utf8.encode(sampleJson),
        201,
        headers: {'content-type': 'application/json'},
      );
    });
    final bytes = Uint8List.fromList(utf8.encode('%PDF-1.4 Inhalt'));
    final entry = await api.upload(bytes, 'befund.pdf', module: 'health');

    expect(captured.method, 'POST');
    expect(captured.url.toString(), '$testBase/api/v1/entries');
    expect(captured.headers['content-type'], startsWith('multipart/form-data'));
    final body = latin1.decode(captured.bodyBytes).toLowerCase();
    expect(body, contains('name="file"; filename="befund.pdf"'));
    expect(body, contains('content-type: application/pdf'));
    expect(body, contains('%pdf-1.4 inhalt'));
    expect(body, contains('name="module"\r\n\r\nhealth'));
    expect(body, isNot(contains('name="text"')));
    expect(entry.id, labReportId);
  });

  test('importText sends multipart with "text" and "module"', () async {
    late http.Request captured;
    final api = client((request) async {
      captured = request;
      return http.Response.bytes(
        utf8.encode(sampleJson),
        201,
        headers: {'content-type': 'application/json'},
      );
    });
    await api.importText('Überweisung vom 21.10.2026', module: 'health');

    expect(captured.method, 'POST');
    expect(captured.url.toString(), '$testBase/api/v1/entries');
    expect(captured.headers['content-type'], startsWith('multipart/form-data'));
    // Form fields as UTF-8, so that umlauts arrive. For non-ASCII the http
    // package adds its own part header with charset.
    final body = utf8.decode(captured.bodyBytes);
    expect(
      body,
      contains(
        'name="text"\r\n'
        'content-type: text/plain; charset=utf-8\r\n'
        'content-transfer-encoding: binary\r\n'
        '\r\n'
        'Überweisung vom 21.10.2026\r\n',
      ),
    );
    expect(body, contains('name="module"\r\n\r\nhealth'));
    expect(body, isNot(contains('name="file"')));
  });

  group('deleteEntry', () {
    test('sends DELETE, 204 without a body is success', () async {
      late http.Request captured;
      final api = client((request) async {
        captured = request;
        return http.Response('', 204);
      });
      await api.deleteEntry(labReportId);

      expect(captured.method, 'DELETE');
      expect(captured.url.path, '/api/v1/entries/$labReportId');
    });

    test('404 becomes ApiException', () async {
      final api = client(
        (_) async => jsonResponse({
          'error': {
            'code': 'ENTRY_NOT_FOUND',
            'message': 'Diesen Eintrag gibt es nicht.',
          },
        }, 404),
      );
      await expectLater(
        api.deleteEntry(labReportId),
        throwsApiException(ErrorCode.entryNotFound, status: 404),
      );
    });
  });

  test('list null instead of [] does not match the contract', () async {
    // docs/API.md: lists are always an array, never null.
    final api = client(
      (_) async => jsonResponse({'entries': null, 'count': 0}),
    );
    await expectLater(
      api.listEntries(module: 'health'),
      throwsApiException(ErrorCode.unexpectedResponse, status: 200),
    );
  });

  test('correct sends PATCH with null for cleared fields', () async {
    late http.Request captured;
    final api = client((request) async {
      captured = request;
      return http.Response.bytes(
        utf8.encode(sampleJson),
        200,
        headers: {'content-type': 'application/json'},
      );
    });
    await api.correct(
      labReportId,
      const EntryPatch(
        documentDate: Value(null),
        facts: [FactPatch('f1', unit: Value(null))],
      ),
    );

    expect(captured.method, 'PATCH');
    expect(captured.url.path, '/api/v1/entries/$labReportId');
    expect(captured.headers['content-type'], startsWith('application/json'));
    expect(jsonDecode(captured.body), {
      'documentDate': null,
      'facts': [
        {'id': 'f1', 'unit': null},
      ],
    });
  });

  test('ask sends POST and reads the answer with sources', () async {
    late http.Request captured;
    final api = client((request) async {
      captured = request;
      return jsonResponse({
        'answer': 'Im Laborbefund steht: Hämoglobin 14,2 g/dl [1].',
        'found': true,
        'sources': [
          {
            'number': 3,
            'entryId': labReportId,
            'module': 'health',
            'type': 'lab_report',
            'issuer': 'Labor Muster GmbH, Salzburg',
            'documentDate': '2026-10-14',
            'excerpt': 'Hämoglobin 14,2 g/dl (Ref. 13,5-17,5)',
          },
        ],
      });
    });
    final answer = await api.ask('Hämoglobin?', module: 'health');

    expect(captured.method, 'POST');
    expect(captured.url.path, '/api/v1/questions');
    expect(jsonDecode(captured.body), {
      'question': 'Hämoglobin?',
      'module': 'health',
    });
    expect(answer.found, isTrue);
    final source = answer.sources.single;
    // number need not start at 1 (only cited chunks, API.md 3.6).
    expect(source.number, 3);
    expect(source.module, 'health');
    expect(source.excerpt, startsWith('Hämoglobin'));
    expect(source.documentDate, PlainDate(2026, 10, 14));
  });

  test('source without module does not match the contract', () async {
    final api = client(
      (_) async => jsonResponse({
        'answer': 'x [1].',
        'found': true,
        'sources': [
          {
            'number': 1,
            'entryId': labReportId,
            'type': 'lab_report',
            'issuer': null,
            'documentDate': null,
            'excerpt': 'x',
          },
        ],
      }),
    );
    await expectLater(
      api.ask('x', module: 'health'),
      throwsApiException(ErrorCode.unexpectedResponse),
    );
  });

  group('transcribe', () {
    test('sends multipart with the field "audio" and reads the text', () async {
      late http.Request captured;
      final api = client((request) async {
        captured = request;
        return jsonResponse({
          'text': 'Kniegelenk rechts, leichter Erguss.',
          'durationSeconds': 21.4,
          'language': 'de',
          'engine': 'whisper.cpp large-v3-turbo',
        });
      });
      final transcript = await api.transcribe(testM4a('ton'), 'note_1.m4a');

      expect(captured.method, 'POST');
      expect(captured.url.toString(), '$testBase/api/v1/transcriptions');
      expect(
        captured.headers['content-type'],
        startsWith('multipart/form-data'),
      );
      final body = latin1.decode(captured.bodyBytes).toLowerCase();
      expect(body, contains('name="audio"; filename="note_1.m4a"'));
      expect(body, contains('content-type: audio/mp4'));
      expect(body, contains('ftypm4a '));
      expect(body, isNot(contains('name="module"')));
      expect(transcript.text, 'Kniegelenk rechts, leichter Erguss.');
      expect(transcript.durationSeconds, 21.4);
      expect(transcript.language, 'de');
      expect(transcript.engine, 'whisper.cpp large-v3-turbo');
    });

    test('a whole number as durationSeconds is fine', () async {
      final api = client(
        (_) async => jsonResponse({
          'text': 'Hallo.',
          'durationSeconds': 3,
          'language': 'de',
          'engine': 'x',
        }),
      );
      expect((await api.transcribe(testM4a(), 'a.m4a')).durationSeconds, 3.0);
    });

    test('empty text does not match the contract', () async {
      final api = client(
        (_) async => jsonResponse({
          'text': ' ',
          'durationSeconds': 1.0,
          'language': 'de',
          'engine': 'x',
        }),
      );
      await expectLater(
        api.transcribe(testM4a(), 'a.m4a'),
        throwsApiException(ErrorCode.unexpectedResponse, status: 200),
      );
    });

    test('NO_SPEECH: German message from the backend', () async {
      final api = client(
        (_) async => jsonResponse({
          'error': {
            'code': 'NO_SPEECH',
            'message': 'In der Aufnahme ist keine Sprache zu hören.',
          },
        }, 422),
      );
      await expectLater(
        api.transcribe(testM4a(), 'a.m4a'),
        throwsA(
          isA<ApiException>()
              .having((e) => e.code, 'code', ErrorCode.noSpeech)
              .having((e) => e.httpStatus, 'httpStatus', 422)
              .having(
                (e) => e.message,
                'message',
                'In der Aufnahme ist keine Sprache zu hören.',
              ),
        ),
      );
    });

    test('AUDIO_TOO_LONG with details.maxSeconds', () async {
      final api = client(
        (_) async => jsonResponse({
          'error': {
            'code': 'AUDIO_TOO_LONG',
            'message':
                'Die Aufnahme ist länger als 5 Minuten. Bitte kürzer '
                'sprechen oder aufteilen.',
            'details': {'maxSeconds': 300},
          },
        }, 413),
      );
      await expectLater(
        api.transcribe(testM4a(), 'a.m4a'),
        throwsA(
          isA<ApiException>()
              .having((e) => e.code, 'code', ErrorCode.audioTooLong)
              .having((e) => e.details['maxSeconds'], 'maxSeconds', 300)
              .having((e) => e.message, 'message', contains('5 Minuten')),
        ),
      );
    });

    test('timeout of at least 130 s (API 3.9)', () {
      expect(
        HttpApiClient.transcriptionTimeout,
        greaterThanOrEqualTo(const Duration(seconds: 130)),
      );
    });
  });

  group('getStatus', () {
    test('200 with speech ok', () async {
      late http.Request captured;
      final api = client((request) async {
        captured = request;
        return jsonResponse({'status': 'ok', 'database': 'ok', 'speech': 'ok'});
      });
      final status = await api.getStatus();

      expect(captured.method, 'GET');
      expect(captured.url.toString(), '$testBase/api/v1/status');
      expect(status.status, 'ok');
      expect(status.database, 'ok');
      expect(status.speech, SpeechStatus.ok);
      expect(status.speechAvailable, isTrue);
    });

    test('503 has the status body, not the error format', () async {
      final api = client(
        (_) async => jsonResponse({
          'status': 'error',
          'database': 'unreachable',
          'speech': 'ok',
        }, 503),
      );
      final status = await api.getStatus();
      expect(status.status, 'error');
      expect(status.database, 'unreachable');
      expect(status.speech, SpeechStatus.ok);
    });

    for (final (json, expected) in [
      ('unavailable', SpeechStatus.unavailable),
      ('off', SpeechStatus.off),
    ]) {
      test('speech $json', () async {
        final api = client(
          (_) async =>
              jsonResponse({'status': 'ok', 'database': 'ok', 'speech': json}),
        );
        final status = await api.getStatus();
        expect(status.speech, expected);
        expect(status.speechAvailable, isFalse);
      });
    }

    test('without speech (older backend): unavailable', () async {
      final api = client(
        (_) async => jsonResponse({'status': 'ok', 'database': 'ok'}),
      );
      expect((await api.getStatus()).speech, SpeechStatus.unavailable);
    });

    test('other status codes stay errors', () async {
      final api = client((_) async => http.Response('nope', 500));
      await expectLater(
        api.getStatus(),
        throwsApiException(ErrorCode.unexpectedResponse, status: 500),
      );
    });
  });
}
