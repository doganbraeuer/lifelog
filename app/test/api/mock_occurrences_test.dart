// The mock's occurrences against the cases of the backend's own test
// (backend/internal/core/testdata/routine_occurrences.json, used by
// TestOccurrences_Cases in schedule_test.go). The expected values were
// worked out by hand from the rules in docs/API.md 3.16.
import 'dart:convert';
import 'dart:io';

import 'package:flutter_test/flutter_test.dart';
import 'package:lifelog/api/mock/mock_routines.dart';
import 'package:lifelog/api/mock/mock_time_zones.dart';
import 'package:lifelog/model/plain_date.dart';
import 'package:lifelog/model/routine.dart';

/// `flutter test` runs in app/.
const casesPath = '../backend/internal/core/testdata/routine_occurrences.json';

List<Map<String, dynamic>> readCases() {
  final json = jsonDecode(File(casesPath).readAsStringSync());
  return (json['cases'] as List).cast<Map<String, dynamic>>();
}

void main() {
  final cases = readCases();

  test('the file holds the cases of the backend test', () {
    expect(cases.length, greaterThanOrEqualTo(20));
  });

  // The package timezone stores a table of zone changes, not the recurring
  // rule (lib/src/location.dart: after the last change the last offset
  // applies). Its table ends before 2090, so it knows no summer time there.
  // Only the mock is affected; the backend uses Go's rules. If this guard
  // fails, the package knows the changes after 2037 and the case below need
  // not be skipped any more.
  const tableLimitReason =
      'the package timezone has no zone changes after 2037 (only the mock)';
  test('guard: the zone table of the package ends before 2038', () {
    final vienna = locationOrNull('Europe/Vienna')!;
    expect(
      vienna.transitionAt.last,
      lessThan(DateTime.utc(2038).millisecondsSinceEpoch),
    );
  });

  for (final c in cases) {
    final name = c['name'] as String;
    test(name, skip: name.contains('in 2090') ? tableLimitReason : null, () {
      final got = expandSchedule(
        schedule: RoutineSchedule.fromJson(
          c['schedule'] as Map<String, dynamic>,
        ),
        startDate: PlainDate.parse(c['startDate'] as String),
        endDate: c['endDate'] == null
            ? null
            : PlainDate.parse(c['endDate'] as String),
        from: PlainDate.parse(c['from'] as String),
        to: PlainDate.parse(c['to'] as String),
        timeZone: c['timeZone'] as String,
      );
      final want = (c['occurrences'] as List).cast<Map<String, dynamic>>();
      expect([
        for (final o in got)
          {
            'dueAt': o.dueAt.toIso8601String().replaceFirst('.000', ''),
            'date': o.date.iso,
            'time': o.time,
          },
      ], want);
    });
  }
}
