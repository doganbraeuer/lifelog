import 'dart:convert';

import 'package:flutter_test/flutter_test.dart';
import 'package:lifelog/model/plain_date.dart';
import 'package:lifelog/model/entry_patch.dart';

void main() {
  test('empty patch gives an empty body', () {
    const patch = EntryPatch();
    expect(patch.isEmpty, isTrue);
    expect(patch.toJson(), isEmpty);
  });

  test('example from docs/API.md, section 3.5', () {
    final patch = EntryPatch(
      documentDate: Value(PlainDate(2026, 10, 15)),
      facts: const [FactPatch('f2', value: '6,8')],
    );
    expect(
      jsonEncode(patch.toJson()),
      '{"documentDate":"2026-10-15","facts":[{"id":"f2","value":"6,8"}]}',
    );
  });

  group('missing field and null are different', () {
    test('documentDate', () {
      expect(
        const EntryPatch(type: 'prescription').toJson(),
        isNot(contains('documentDate')),
      );

      final cleared = const EntryPatch(documentDate: Value(null)).toJson();
      expect(cleared.containsKey('documentDate'), isTrue);
      expect(cleared['documentDate'], isNull);
    });

    test('issuer', () {
      expect(
        const EntryPatch(type: 'prescription').toJson(),
        isNot(contains('issuer')),
      );

      final cleared = const EntryPatch(issuer: Value(null)).toJson();
      expect(cleared.containsKey('issuer'), isTrue);
      expect(cleared['issuer'], isNull);

      expect(const EntryPatch(issuer: Value('Labor X')).toJson(), {
        'issuer': 'Labor X',
      });
    });

    test('unit of a fact', () {
      expect(const FactPatch('f1', value: '7').toJson(), {
        'id': 'f1',
        'value': '7',
      });

      final cleared = const FactPatch('f1', unit: Value(null)).toJson();
      expect(cleared.containsKey('unit'), isTrue);
      expect(cleared['unit'], isNull);
      expect(cleared.containsKey('value'), isFalse);
    });
  });
}
