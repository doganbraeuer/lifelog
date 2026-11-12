import 'dart:convert';

import 'package:flutter_test/flutter_test.dart';
import 'package:lifelog/model/plain_date.dart';
import 'package:lifelog/model/entry_patch.dart';

void main() {
  test('leere Änderung ergibt leeren Body', () {
    const aenderung = EntryPatch();
    expect(aenderung.isEmpty, isTrue);
    expect(aenderung.toJson(), isEmpty);
  });

  test('Beispiel aus docs/API.md, Abschnitt 3.5', () {
    final aenderung = EntryPatch(
      documentDate: Value(PlainDate(2026, 10, 15)),
      facts: const [FactPatch('f2', value: '6,8')],
    );
    expect(
      jsonEncode(aenderung.toJson()),
      '{"documentDate":"2026-10-15","facts":[{"id":"f2","value":"6,8"}]}',
    );
  });

  group('fehlendes Feld und null sind verschieden', () {
    test('documentDate', () {
      expect(
        const EntryPatch(type: 'prescription').toJson(),
        isNot(contains('documentDate')),
      );

      final geloescht = const EntryPatch(documentDate: Value(null)).toJson();
      expect(geloescht.containsKey('documentDate'), isTrue);
      expect(geloescht['documentDate'], isNull);
    });

    test('issuer', () {
      expect(
        const EntryPatch(type: 'prescription').toJson(),
        isNot(contains('issuer')),
      );

      final geloescht = const EntryPatch(issuer: Value(null)).toJson();
      expect(geloescht.containsKey('issuer'), isTrue);
      expect(geloescht['issuer'], isNull);

      expect(const EntryPatch(issuer: Value('Labor X')).toJson(), {
        'issuer': 'Labor X',
      });
    });

    test('einheit einer Info', () {
      expect(const FactPatch('f1', value: '7').toJson(), {
        'id': 'f1',
        'value': '7',
      });

      final geloescht = const FactPatch('f1', unit: Value(null)).toJson();
      expect(geloescht.containsKey('unit'), isTrue);
      expect(geloescht['unit'], isNull);
      expect(geloescht.containsKey('value'), isFalse);
    });
  });
}
