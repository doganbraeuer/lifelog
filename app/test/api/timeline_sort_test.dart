import 'package:flutter_test/flutter_test.dart';
import 'package:lifelog/api/api_client.dart';
import 'package:lifelog/api/timeline_sort.dart';
import 'package:lifelog/model/plain_date.dart';
import 'package:lifelog/model/entry.dart';

EntrySummary summary(String id, PlainDate? date, int hour, int minute) =>
    EntrySummary(
      id: id,
      module: 'health',
      type: 'other',
      documentDate: date,
      importedAt: DateTime.utc(2026, 11, 3, hour, minute),
      issuer: null,
    );

void main() {
  // a and e/f have the same document date, e and f also the same import
  // date. c and d have no document date.
  final a = summary('a', PlainDate(2026, 10, 14), 18, 42);
  final b = summary('b', PlainDate(2026, 9, 2), 18, 50);
  final c = summary('c', null, 19, 1);
  final d = summary('d', null, 17, 0);
  final e = summary('e', PlainDate(2026, 10, 14), 19, 30);
  final f = summary('f', PlainDate(2026, 10, 14), 19, 30);
  final unsorted = [c, a, f, d, b, e];

  List<String> ids(Sort s, SortOrder r) =>
      sortTimeline(unsorted, s, r).map((x) => x.id).toList();

  test('documentDate descending (default)', () {
    // Same date: importedAt descending, then id descending.
    // Without date at the end, among themselves importedAt descending.
    expect(ids(Sort.documentDate, SortOrder.desc), [
      'f',
      'e',
      'a',
      'b',
      'c',
      'd',
    ]);
  });

  test('documentDate ascending: without date still at the end', () {
    expect(ids(Sort.documentDate, SortOrder.asc), [
      'b',
      'a',
      'e',
      'f',
      'c',
      'd',
    ]);
  });

  test('importedAt descending', () {
    expect(ids(Sort.importedAt, SortOrder.desc), [
      'f',
      'e',
      'c',
      'b',
      'a',
      'd',
    ]);
  });

  test('importedAt ascending', () {
    expect(ids(Sort.importedAt, SortOrder.asc), ['d', 'a', 'b', 'c', 'e', 'f']);
  });

  test('order does not depend on the input order', () {
    final reversedOrder = sortTimeline(
      unsorted.reversed,
      Sort.documentDate,
      SortOrder.desc,
    );
    expect(reversedOrder.map((x) => x.id), ['f', 'e', 'a', 'b', 'c', 'd']);
  });
}
