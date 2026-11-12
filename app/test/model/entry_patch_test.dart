import 'dart:convert';

import 'package:flutter_test/flutter_test.dart';
import 'package:lifelog/model/plain_date.dart';
import 'package:lifelog/model/entry_patch.dart';

void main() {
  test('leere Änderung ergibt leeren Body', () {
    const aenderung = EintragAenderung();
    expect(aenderung.istLeer, isTrue);
    expect(aenderung.toJson(), isEmpty);
  });

  test('Beispiel aus docs/API.md, Abschnitt 3.5', () {
    final aenderung = EintragAenderung(
      dokumentdatum: Wert(Datum(2026, 10, 15)),
      infos: const [InfoAenderung('f2', wert: '6,8')],
    );
    expect(
      jsonEncode(aenderung.toJson()),
      '{"documentDate":"2026-10-15","facts":[{"id":"f2","value":"6,8"}]}',
    );
  });

  group('fehlendes Feld und null sind verschieden', () {
    test('documentDate', () {
      expect(
        const EintragAenderung(typ: 'prescription').toJson(),
        isNot(contains('documentDate')),
      );

      final geloescht = const EintragAenderung(dokumentdatum: Wert(null))
          .toJson();
      expect(geloescht.containsKey('documentDate'), isTrue);
      expect(geloescht['documentDate'], isNull);
    });

    test('issuer', () {
      expect(
        const EintragAenderung(typ: 'prescription').toJson(),
        isNot(contains('issuer')),
      );

      final geloescht = const EintragAenderung(quelle: Wert(null)).toJson();
      expect(geloescht.containsKey('issuer'), isTrue);
      expect(geloescht['issuer'], isNull);

      expect(const EintragAenderung(quelle: Wert('Labor X')).toJson(), {
        'issuer': 'Labor X',
      });
    });

    test('einheit einer Info', () {
      expect(const InfoAenderung('f1', wert: '7').toJson(), {
        'id': 'f1',
        'value': '7',
      });

      final geloescht = const InfoAenderung('f1', einheit: Wert(null)).toJson();
      expect(geloescht.containsKey('unit'), isTrue);
      expect(geloescht['unit'], isNull);
      expect(geloescht.containsKey('value'), isFalse);
    });
  });
}
