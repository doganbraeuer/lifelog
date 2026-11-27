// Routines in the HttpApiClient: methods, paths, query parameters and
// bodies as in docs/API.md 3.13 and 3.17–3.27.
import 'dart:convert';

import 'package:flutter_test/flutter_test.dart';
import 'package:http/http.dart' as http;
import 'package:lifelog/api/api_exception.dart';
import 'package:lifelog/model/entry_patch.dart';
import 'package:lifelog/model/plain_date.dart';
import 'package:lifelog/model/routine.dart';

import '../model/routine_test.dart' show occurrencesJson;
import 'http_api_client_test.dart' show client, jsonResponse;
import 'http_auth_test.dart' show anyEndpoint, logId, routineId, userJson;

/// Every request of [call], as `METHOD path?query` and the decoded body.
Future<List<(String, Object?)>> requestsOf(
  Future<Object?> Function(dynamic api) call,
) async {
  final seen = <(String, Object?)>[];
  final api = client((request) async {
    final query = request.url.hasQuery ? '?${request.url.query}' : '';
    seen.add((
      '${request.method} ${request.url.path}$query',
      request.body.isEmpty ? null : jsonDecode(request.body),
    ));
    return anyEndpoint(request);
  });
  await call(api);
  return seen;
}

/// Compares requests by value: records compare maps by identity, lists
/// compare them deeply.
Future<void> expectRequests(
  Future<List<(String, Object?)>> actual,
  List<(String, Object?)> expected,
) async {
  List<List<Object?>> asLists(List<(String, Object?)> requests) => [
    for (final (line, body) in requests) [line, body],
  ];
  expect(asLists(await actual), equals(asLists(expected)));
}

void main() {
  test('list, get, create, change and delete a routine', () async {
    await expectRequests(
      requestsOf((api) => api.listRoutines(module: 'health')),
      [('GET /api/v1/routines?module=health', null)],
    );
    await expectRequests(requestsOf((api) => api.getRoutine(routineId)), [
      ('GET /api/v1/routines/$routineId', null),
    ]);
    await expectRequests(
      requestsOf(
        (api) => api.createRoutine(
          NewRoutine(
            module: 'health',
            name: 'Vitamin D',
            amount: 2,
            unit: 'Stück',
            schedule: const RoutineSchedule.daily(
              interval: 1,
              times: ['08:00'],
            ),
            startDate: PlainDate(2026, 11, 23),
          ),
        ),
      ),
      [
        (
          'POST /api/v1/routines',
          {
            'module': 'health',
            'name': 'Vitamin D',
            'amount': 2,
            'unit': 'Stück',
            'schedule': {
              'frequency': 'daily',
              'interval': 1,
              'times': ['08:00'],
            },
            'startDate': '2026-11-23',
          },
        ),
      ],
    );
    await expectRequests(
      requestsOf(
        (api) => api.updateRoutine(
          routineId,
          const RoutinePatch(amount: Value(null), endDate: Value(null)),
        ),
      ),
      [
        (
          'PATCH /api/v1/routines/$routineId',
          {'amount': null, 'endDate': null},
        ),
      ],
    );
    await expectRequests(requestsOf((api) => api.deleteRoutine(routineId)), [
      ('DELETE /api/v1/routines/$routineId', null),
    ]);
  });

  test('pause with and without an end, resume without a body', () async {
    await expectRequests(
      requestsOf(
        (api) => api.pauseRoutine(routineId, until: PlainDate(2026, 11, 30)),
      ),
      [
        ('POST /api/v1/routines/$routineId/pause', {'until': '2026-11-30'}),
      ],
    );
    await expectRequests(requestsOf((api) => api.pauseRoutine(routineId)), [
      ('POST /api/v1/routines/$routineId/pause', <String, dynamic>{}),
    ]);
    await expectRequests(requestsOf((api) => api.resumeRoutine(routineId)), [
      ('POST /api/v1/routines/$routineId/resume', null),
    ]);
  });

  test(
    'occurrences: parameters only when given, response as in 3.24',
    () async {
      await expectRequests(
        requestsOf((api) => api.listOccurrences(module: 'health')),
        [('GET /api/v1/routines/occurrences?module=health', null)],
      );
      await expectRequests(
        requestsOf(
          (api) => api.listOccurrences(
            module: 'health',
            from: PlainDate(2026, 11, 23),
            to: PlainDate(2026, 11, 29),
          ),
        ),
        [
          (
            'GET /api/v1/routines/occurrences'
                '?module=health&from=2026-11-23&to=2026-11-29',
            null,
          ),
        ],
      );
      final list = await client(
        (_) async => jsonResponse(jsonDecode(occurrencesJson) as Object),
      ).listOccurrences(module: 'health');
      expect(list.occurrences, hasLength(3));
      expect(list.from, PlainDate(2026, 11, 23));
    },
  );

  test('logs: list, add and delete', () async {
    await expectRequests(requestsOf((api) => api.listRoutineLogs(routineId)), [
      ('GET /api/v1/routines/$routineId/logs', null),
    ]);
    await expectRequests(
      requestsOf(
        (api) => api.listRoutineLogs(
          routineId,
          from: PlainDate(2026, 10, 25),
          to: PlainDate(2026, 11, 23),
        ),
      ),
      [
        (
          'GET /api/v1/routines/$routineId/logs?from=2026-10-25&to=2026-11-23',
          null,
        ),
      ],
    );
    await expectRequests(
      requestsOf(
        (api) => api.addRoutineLog(
          routineId,
          NewRoutineLog(
            status: LogStatus.done,
            source: LogSource.notification,
            dueAt: DateTime.utc(2026, 11, 23, 7),
          ),
        ),
      ),
      [
        (
          'POST /api/v1/routines/$routineId/logs',
          {
            'status': 'done',
            'source': 'notification',
            'dueAt': '2026-11-23T07:00:00Z',
          },
        ),
      ],
    );
    await expectRequests(
      requestsOf((api) => api.deleteRoutineLog(routineId, logId)),
      [('DELETE /api/v1/routines/$routineId/logs/$logId', null)],
    );
  });

  test('responses are read into the models', () async {
    final api = client((request) => anyEndpoint(request));
    final routines = await api.listRoutines(module: 'health');
    expect(routines.single.name, 'Wasser');
    final log = await api.addRoutineLog(
      routineId,
      const NewRoutineLog(status: LogStatus.done, source: LogSource.manual),
    );
    expect(log.id, logId);
    final logs = await api.listRoutineLogs(routineId);
    expect(logs.single.timeZone, 'Europe/Vienna');
  });

  test('PATCH /me with the time zone', () async {
    await expectRequests(
      requestsOf((api) => api.updateTimeZone('Europe/Lisbon')),
      [
        ('PATCH /api/v1/me', {'timeZone': 'Europe/Lisbon'}),
      ],
    );
    final user = await client(
      (_) async => jsonResponse({...userJson, 'timeZone': 'Europe/Lisbon'}),
    ).updateTimeZone('Europe/Lisbon');
    expect(user.timeZone, 'Europe/Lisbon');
  });

  group('errors of the routine endpoints', () {
    Future<ApiException> errorOf(
      int status,
      Map<String, dynamic> error,
      Future<Object?> Function(dynamic api) call,
    ) async {
      final api = client(
        (_) async => http.Response.bytes(
          utf8.encode(jsonEncode({'error': error})),
          status,
          headers: {'content-type': 'application/json'},
        ),
      );
      try {
        await call(api);
      } on ApiException catch (e) {
        return e;
      }
      throw StateError('no error');
    }

    test('409 ROUTINE_LOG_EXISTS with the existing log', () async {
      final error = await errorOf(
        409,
        {
          'code': 'ROUTINE_LOG_EXISTS',
          'message': 'Dieser Termin ist schon eingetragen.',
          'details': {'logId': logId},
        },
        (api) => api.addRoutineLog(
          routineId,
          const NewRoutineLog(status: LogStatus.done, source: LogSource.manual),
        ),
      );
      expect(error.code, ErrorCode.routineLogExists);
      expect(error.httpStatus, 409);
      expect(error.details['logId'], logId);
      expect(error.message, 'Dieser Termin ist schon eingetragen.');
    });

    test('409 ROUTINE_NAME_TAKEN and 400 with the field', () async {
      final taken = await errorOf(409, {
        'code': 'ROUTINE_NAME_TAKEN',
        'message': 'Eine Routine mit diesem Namen gibt es schon.',
        'details': {'routineId': routineId},
      }, (api) => api.getRoutine(routineId));
      expect(taken.code, ErrorCode.routineNameTaken);
      expect(taken.details['routineId'], routineId);

      final invalid = await errorOf(400, {
        'code': 'INVALID_REQUEST',
        'message': 'Uhrzeiten bitte im Format HH:MM angeben, z. B. 08:00.',
        'details': {'field': 'schedule.times', 'value': '8 Uhr'},
      }, (api) => api.getRoutine(routineId));
      expect(invalid.details['field'], 'schedule.times');
    });

    test('404 ROUTINE_NOT_FOUND', () async {
      final error = await errorOf(404, {
        'code': 'ROUTINE_NOT_FOUND',
        'message': 'Diese Routine gibt es nicht.',
      }, (api) => api.deleteRoutine(routineId));
      expect(error.code, ErrorCode.routineNotFound);
    });
  });

  test('an ID is encoded in the path', () async {
    await expectRequests(requestsOf((api) => api.getRoutine('a/b')), [
      ('GET /api/v1/routines/a%2Fb', null),
    ]);
  });
}
