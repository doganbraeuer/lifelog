import 'dart:convert';

import 'package:flutter_test/flutter_test.dart';
import 'package:lifelog/model/datum.dart';
import 'package:lifelog/model/eintrag_aenderung.dart';

void main() {
  test('leere Änderung ergibt leeren Body', () {
    const aenderung = EintragAenderung();
    expect(aenderung.istLeer, isTrue);
    expect(aenderung.toJson(), isEmpty);
  });

  test('Beispiel aus docs/API.md, Abschnitt 3.5', () {
    final aenderung = EintragAenderung(
      dokumentdatum: Wert(Datum(2026, 10, 15)),
      infos: const [InfoAenderung('i2', wert: '6,8')],
    );
    expect(
      jsonEncode(aenderung.toJson()),
      '{"dokumentdatum":"2026-10-15","infos":[{"id":"i2","wert":"6,8"}]}',
    );
  });

  group('fehlendes Feld und null sind verschieden', () {
    test('dokumentdatum', () {
      expect(
        const EintragAenderung(typ: 'rezept').toJson(),
        isNot(contains('dokumentdatum')),
      );

      final geloescht = const EintragAenderung(dokumentdatum: Wert(null))
          .toJson();
      expect(geloescht.containsKey('dokumentdatum'), isTrue);
      expect(geloescht['dokumentdatum'], isNull);
    });

    test('quelle', () {
      expect(
        const EintragAenderung(typ: 'rezept').toJson(),
        isNot(contains('quelle')),
      );

      final geloescht = const EintragAenderung(quelle: Wert(null)).toJson();
      expect(geloescht.containsKey('quelle'), isTrue);
      expect(geloescht['quelle'], isNull);

      expect(const EintragAenderung(quelle: Wert('Labor X')).toJson(), {
        'quelle': 'Labor X',
      });
    });

    test('einheit einer Info', () {
      expect(const InfoAenderung('i1', wert: '7').toJson(), {
        'id': 'i1',
        'wert': '7',
      });

      final geloescht = const InfoAenderung('i1', einheit: Wert(null)).toJson();
      expect(geloescht.containsKey('einheit'), isTrue);
      expect(geloescht['einheit'], isNull);
      expect(geloescht.containsKey('wert'), isFalse);
    });
  });
}
