import 'package:flutter_test/flutter_test.dart';
import 'package:lifelog/api/api_client.dart';
import 'package:lifelog/model/plain_date.dart';
import 'package:lifelog/model/entry.dart';
import 'package:lifelog/model/timeline_groups.dart';

/// Eintrag mit Dokumentdatum und Importzeit in **lokaler** Zeit.
EntrySummary e(String id, PlainDate? datum, DateTime importLokal) =>
    EntrySummary(
      id: id,
      module: 'health',
      type: 'other',
      documentDate: datum,
      importedAt: importLokal.toUtc(),
      issuer: null,
    );

/// Gruppen als Text, z. B. „Oktober 2026: a, b“ (Listen in Records würden
/// nach Identität verglichen, nicht nach Inhalt).
List<String> kurz(List<TimelineGroup> gruppen) => [
  for (final g in gruppen)
    '${g.title}: ${g.entries.map((x) => x.id).join(', ')}',
];

void main() {
  final heute = DateTime(2026, 11, 3, 12);

  group('nach Dokumentdatum', () {
    test('Monate in der Reihenfolge des Backends, ohne Datum am Ende', () {
      // So liefert das Backend absteigend: ohne Datum schon am Ende.
      final liste = [
        e('a', PlainDate(2026, 10, 14), heute),
        e('b', PlainDate(2026, 10, 2), heute),
        e('c', PlainDate(2026, 9, 30), heute),
        e('d', null, heute),
      ];
      expect(kurz(groupTimeline(liste, Sort.documentDate)), [
        'Oktober 2026: a, b',
        'September 2026: c',
        'Ohne Datum: d',
      ]);
    });

    test('aufsteigend und über den Jahreswechsel', () {
      final liste = [
        e('a', PlainDate(2025, 12, 31), heute),
        e('b', PlainDate(2026, 1, 1), heute),
        e('c', PlainDate(2026, 1, 20), heute),
      ];
      expect(kurz(groupTimeline(liste, Sort.documentDate)), [
        'Dezember 2025: a',
        'Januar 2026: b, c',
      ]);
    });

    test('ohne Datum landet am Ende, auch wenn es mittendrin steht', () {
      final liste = [
        e('a', null, heute),
        e('b', PlainDate(2026, 3, 1), heute),
        e('c', null, heute),
      ];
      expect(kurz(groupTimeline(liste, Sort.documentDate)), [
        'März 2026: b',
        'Ohne Datum: a, c',
      ]);
    });

    test('Monat aus dem reinen Datum, keine Zeitzone', () {
      // Der 1. Oktober bleibt Oktober, egal wie spät importiert wurde.
      final liste = [e('a', PlainDate(2026, 10, 1), DateTime(2026, 9, 30, 23))];
      expect(
        groupTimeline(liste, Sort.documentDate).single.title,
        'Oktober 2026',
      );
    });
  });

  group('nach Importdatum', () {
    test('Monat in lokaler Zeit, Einträge ohne Dokumentdatum normal', () {
      final liste = [
        e('a', null, DateTime(2026, 11, 1, 0, 30)),
        e('b', PlainDate(2026, 10, 14), DateTime(2026, 10, 31, 23, 30)),
        e('c', PlainDate(2026, 1, 1), DateTime(2026, 10, 2)),
      ];
      expect(kurz(groupTimeline(liste, Sort.importedAt)), [
        'November 2026: a',
        'Oktober 2026: b, c',
      ]);
    });
  });

  test('leere Liste', () {
    expect(groupTimeline([], Sort.documentDate), isEmpty);
    expect(groupTimeline([], Sort.importedAt), isEmpty);
  });

  test('alle Monatsnamen deutsch', () {
    expect(monthNames, hasLength(12));
    expect(monthNames[2], 'März');
  });
}
