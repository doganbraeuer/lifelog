import 'package:flutter_test/flutter_test.dart';
import 'package:lifelog/api/api_client.dart';
import 'package:lifelog/model/datum.dart';
import 'package:lifelog/model/eintrag.dart';
import 'package:lifelog/model/zeitleisten_gruppen.dart';

/// Eintrag mit Dokumentdatum und Importzeit in **lokaler** Zeit.
EintragKurz e(String id, Datum? datum, DateTime importLokal) => EintragKurz(
  id: id,
  modul: 'health',
  typ: 'other',
  dokumentdatum: datum,
  importdatum: importLokal.toUtc(),
  quelle: null,
);

/// Gruppen als Text, z. B. „Oktober 2026: a, b“ (Listen in Records würden
/// nach Identität verglichen, nicht nach Inhalt).
List<String> kurz(List<ZeitleistenGruppe> gruppen) => [
  for (final g in gruppen)
    '${g.titel}: ${g.eintraege.map((x) => x.id).join(', ')}',
];

void main() {
  final heute = DateTime(2026, 11, 3, 12);

  group('nach Dokumentdatum', () {
    test('Monate in der Reihenfolge des Backends, ohne Datum am Ende', () {
      // So liefert das Backend absteigend: ohne Datum schon am Ende.
      final liste = [
        e('a', Datum(2026, 10, 14), heute),
        e('b', Datum(2026, 10, 2), heute),
        e('c', Datum(2026, 9, 30), heute),
        e('d', null, heute),
      ];
      expect(kurz(gruppiereZeitleiste(liste, Sortierung.documentDate)), [
        'Oktober 2026: a, b',
        'September 2026: c',
        'Ohne Datum: d',
      ]);
    });

    test('aufsteigend und über den Jahreswechsel', () {
      final liste = [
        e('a', Datum(2025, 12, 31), heute),
        e('b', Datum(2026, 1, 1), heute),
        e('c', Datum(2026, 1, 20), heute),
      ];
      expect(kurz(gruppiereZeitleiste(liste, Sortierung.documentDate)), [
        'Dezember 2025: a',
        'Januar 2026: b, c',
      ]);
    });

    test('ohne Datum landet am Ende, auch wenn es mittendrin steht', () {
      final liste = [
        e('a', null, heute),
        e('b', Datum(2026, 3, 1), heute),
        e('c', null, heute),
      ];
      expect(kurz(gruppiereZeitleiste(liste, Sortierung.documentDate)), [
        'März 2026: b',
        'Ohne Datum: a, c',
      ]);
    });

    test('Monat aus dem reinen Datum, keine Zeitzone', () {
      // Der 1. Oktober bleibt Oktober, egal wie spät importiert wurde.
      final liste = [e('a', Datum(2026, 10, 1), DateTime(2026, 9, 30, 23))];
      expect(
        gruppiereZeitleiste(liste, Sortierung.documentDate).single.titel,
        'Oktober 2026',
      );
    });
  });

  group('nach Importdatum', () {
    test('Monat in lokaler Zeit, Einträge ohne Dokumentdatum normal', () {
      final liste = [
        e('a', null, DateTime(2026, 11, 1, 0, 30)),
        e('b', Datum(2026, 10, 14), DateTime(2026, 10, 31, 23, 30)),
        e('c', Datum(2026, 1, 1), DateTime(2026, 10, 2)),
      ];
      expect(kurz(gruppiereZeitleiste(liste, Sortierung.importedAt)), [
        'November 2026: a',
        'Oktober 2026: b, c',
      ]);
    });
  });

  test('leere Liste', () {
    expect(gruppiereZeitleiste([], Sortierung.documentDate), isEmpty);
    expect(gruppiereZeitleiste([], Sortierung.importedAt), isEmpty);
  });

  test('alle Monatsnamen deutsch', () {
    expect(monatsnamen, hasLength(12));
    expect(monatsnamen[2], 'März');
  });
}
