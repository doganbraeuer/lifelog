import 'package:flutter_test/flutter_test.dart';
import 'package:lifelog/api/api_client.dart';
import 'package:lifelog/model/plain_date.dart';
import 'package:lifelog/model/entry.dart';
import 'package:lifelog/model/timeline_groups.dart';

/// Entry with document date and import time in **local** time.
EntrySummary e(String id, PlainDate? date, DateTime importedLocal) =>
    EntrySummary(
      id: id,
      module: 'health',
      type: 'other',
      documentDate: date,
      importedAt: importedLocal.toUtc(),
      issuer: null,
    );

/// Groups as text, e.g. „Oktober 2026: a, b“ (lists in records would be
/// compared by identity, not by content).
List<String> summary(List<TimelineGroup> groups) => [
  for (final g in groups)
    '${g.title}: ${g.entries.map((x) => x.id).join(', ')}',
];

void main() {
  final today = DateTime(2026, 11, 3, 12);

  group('by document date', () {
    test('months in the order of the backend, without date at the end', () {
      // This is how the backend returns descending: without date already last.
      final list = [
        e('a', PlainDate(2026, 10, 14), today),
        e('b', PlainDate(2026, 10, 2), today),
        e('c', PlainDate(2026, 9, 30), today),
        e('d', null, today),
      ];
      expect(summary(groupTimeline(list, Sort.documentDate)), [
        'Oktober 2026: a, b',
        'September 2026: c',
        'Ohne Datum: d',
      ]);
    });

    test('ascending and across the turn of the year', () {
      final list = [
        e('a', PlainDate(2025, 12, 31), today),
        e('b', PlainDate(2026, 1, 1), today),
        e('c', PlainDate(2026, 1, 20), today),
      ];
      expect(summary(groupTimeline(list, Sort.documentDate)), [
        'Dezember 2025: a',
        'Januar 2026: b, c',
      ]);
    });

    test('without date ends up last, even if it is in the middle', () {
      final list = [
        e('a', null, today),
        e('b', PlainDate(2026, 3, 1), today),
        e('c', null, today),
      ];
      expect(summary(groupTimeline(list, Sort.documentDate)), [
        'März 2026: b',
        'Ohne Datum: a, c',
      ]);
    });

    test('month from the plain date, no time zone', () {
      // 1 October stays October, no matter how late it was imported.
      final list = [e('a', PlainDate(2026, 10, 1), DateTime(2026, 9, 30, 23))];
      expect(
        groupTimeline(list, Sort.documentDate).single.title,
        'Oktober 2026',
      );
    });
  });

  group('by import date', () {
    test('month in local time, entries without document date as usual', () {
      final list = [
        e('a', null, DateTime(2026, 11, 1, 0, 30)),
        e('b', PlainDate(2026, 10, 14), DateTime(2026, 10, 31, 23, 30)),
        e('c', PlainDate(2026, 1, 1), DateTime(2026, 10, 2)),
      ];
      expect(summary(groupTimeline(list, Sort.importedAt)), [
        'November 2026: a',
        'Oktober 2026: b, c',
      ]);
    });
  });

  test('empty list', () {
    expect(groupTimeline([], Sort.documentDate), isEmpty);
    expect(groupTimeline([], Sort.importedAt), isEmpty);
  });

  test('all month names German', () {
    expect(monthNames, hasLength(12));
    expect(monthNames[2], 'März');
  });
}
