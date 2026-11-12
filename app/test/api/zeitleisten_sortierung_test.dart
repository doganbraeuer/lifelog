import 'package:flutter_test/flutter_test.dart';
import 'package:lifelog/api/api_client.dart';
import 'package:lifelog/api/zeitleisten_sortierung.dart';
import 'package:lifelog/model/datum.dart';
import 'package:lifelog/model/eintrag.dart';

EintragKurz eintrag(String id, Datum? datum, int stunde, int minute) =>
    EintragKurz(
      id: id,
      modul: 'health',
      typ: 'other',
      dokumentdatum: datum,
      importdatum: DateTime.utc(2026, 11, 3, stunde, minute),
      quelle: null,
    );

void main() {
  // a und e/f haben dasselbe Dokumentdatum, e und f zusätzlich dasselbe
  // Importdatum. c und d haben kein Dokumentdatum.
  final a = eintrag('a', Datum(2026, 10, 14), 18, 42);
  final b = eintrag('b', Datum(2026, 9, 2), 18, 50);
  final c = eintrag('c', null, 19, 1);
  final d = eintrag('d', null, 17, 0);
  final e = eintrag('e', Datum(2026, 10, 14), 19, 30);
  final f = eintrag('f', Datum(2026, 10, 14), 19, 30);
  final alle = [c, a, f, d, b, e];

  List<String> ids(Sortierung s, Richtung r) =>
      sortiereZeitleiste(alle, s, r).map((x) => x.id).toList();

  test('dokumentdatum absteigend (Standard)', () {
    // Gleiches Datum: importdatum absteigend, dann id absteigend.
    // Ohne Datum am Ende, untereinander importdatum absteigend.
    expect(ids(Sortierung.documentDate, Richtung.desc), [
      'f',
      'e',
      'a',
      'b',
      'c',
      'd',
    ]);
  });

  test('dokumentdatum aufsteigend: ohne Datum trotzdem am Ende', () {
    expect(ids(Sortierung.documentDate, Richtung.asc), [
      'b',
      'a',
      'e',
      'f',
      'c',
      'd',
    ]);
  });

  test('importdatum absteigend', () {
    expect(ids(Sortierung.importedAt, Richtung.desc), [
      'f',
      'e',
      'c',
      'b',
      'a',
      'd',
    ]);
  });

  test('importdatum aufsteigend', () {
    expect(ids(Sortierung.importedAt, Richtung.asc), [
      'd',
      'a',
      'b',
      'c',
      'e',
      'f',
    ]);
  });

  test('Reihenfolge hängt nicht von der Eingabe-Reihenfolge ab', () {
    final umgedreht = sortiereZeitleiste(
      alle.reversed,
      Sortierung.documentDate,
      Richtung.desc,
    );
    expect(umgedreht.map((x) => x.id), ['f', 'e', 'a', 'b', 'c', 'd']);
  });
}
