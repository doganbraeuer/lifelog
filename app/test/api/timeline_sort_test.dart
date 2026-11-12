import 'package:flutter_test/flutter_test.dart';
import 'package:lifelog/api/api_client.dart';
import 'package:lifelog/api/timeline_sort.dart';
import 'package:lifelog/model/plain_date.dart';
import 'package:lifelog/model/entry.dart';

EntrySummary getEntry(String id, PlainDate? datum, int stunde, int minute) =>
    EntrySummary(
      id: id,
      module: 'health',
      type: 'other',
      documentDate: datum,
      importedAt: DateTime.utc(2026, 11, 3, stunde, minute),
      issuer: null,
    );

void main() {
  // a und e/f haben dasselbe Dokumentdatum, e und f zusätzlich dasselbe
  // Importdatum. c und d haben kein Dokumentdatum.
  final a = getEntry('a', PlainDate(2026, 10, 14), 18, 42);
  final b = getEntry('b', PlainDate(2026, 9, 2), 18, 50);
  final c = getEntry('c', null, 19, 1);
  final d = getEntry('d', null, 17, 0);
  final e = getEntry('e', PlainDate(2026, 10, 14), 19, 30);
  final f = getEntry('f', PlainDate(2026, 10, 14), 19, 30);
  final alle = [c, a, f, d, b, e];

  List<String> ids(Sort s, SortOrder r) =>
      sortTimeline(alle, s, r).map((x) => x.id).toList();

  test('dokumentdatum absteigend (Standard)', () {
    // Gleiches Datum: importdatum absteigend, dann id absteigend.
    // Ohne Datum am Ende, untereinander importdatum absteigend.
    expect(ids(Sort.documentDate, SortOrder.desc), [
      'f',
      'e',
      'a',
      'b',
      'c',
      'd',
    ]);
  });

  test('dokumentdatum aufsteigend: ohne Datum trotzdem am Ende', () {
    expect(ids(Sort.documentDate, SortOrder.asc), [
      'b',
      'a',
      'e',
      'f',
      'c',
      'd',
    ]);
  });

  test('importdatum absteigend', () {
    expect(ids(Sort.importedAt, SortOrder.desc), [
      'f',
      'e',
      'c',
      'b',
      'a',
      'd',
    ]);
  });

  test('importdatum aufsteigend', () {
    expect(ids(Sort.importedAt, SortOrder.asc), ['d', 'a', 'b', 'c', 'e', 'f']);
  });

  test('Reihenfolge hängt nicht von der Eingabe-Reihenfolge ab', () {
    final umgedreht = sortTimeline(
      alle.reversed,
      Sort.documentDate,
      SortOrder.desc,
    );
    expect(umgedreht.map((x) => x.id), ['f', 'e', 'a', 'b', 'c', 'd']);
  });
}
