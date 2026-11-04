import 'package:flutter_test/flutter_test.dart';
import 'package:lifelog/api/api_fehler.dart';
import 'package:lifelog/api/mock_api_client.dart';
import 'package:lifelog/model/datum.dart';
import 'package:lifelog/model/eintrag_aenderung.dart';

import '../hilfen.dart';

void main() {
  late MockApiClient mock;
  final jetzt = DateTime.utc(2026, 11, 4, 9, 30, 0);

  setUp(() => mock = schnellerMock(uhr: () => jetzt));

  group('Korrekturregeln aus docs/API.md, Abschnitt 2', () {
    test(
      'Regel 1 und 2: aktueller Wert im Feld, erkannt bleibt das Original',
      () async {
        // Ausgangslage: i2 wurde schon von 6,3 auf 6,8 korrigiert.
        final eintrag = await mock.korrigieren(
          laborId,
          const EintragAenderung(infos: [InfoAenderung('i2', wert: '7,0')]),
        );

        expect(eintrag.infos[1].wert, '7,0');
        final korrekturen = eintrag.korrekturen.where(
          (k) => k.feld == 'infos.i2.wert',
        );
        expect(korrekturen, hasLength(1));
        expect(korrekturen.single.erkannt, '6,3');
        expect(korrekturen.single.korrigiert, '7,0');
        expect(korrekturen.single.zeitpunkt, jetzt);
      },
    );

    test(
      'Regel 3: zurück auf den erkannten Wert entfernt die Korrektur',
      () async {
        final eintrag = await mock.korrigieren(
          laborId,
          const EintragAenderung(infos: [InfoAenderung('i2', wert: '6,3')]),
        );

        expect(eintrag.infos[1].wert, '6,3');
        expect(eintrag.korrektur('infos.i2.wert'), isNull);
        expect(eintrag.infoKorrigiert('i2'), isFalse);
      },
    );

    test('unveränderter Wert erzeugt keine Korrektur', () async {
      final eintrag = await mock.korrigieren(
        laborId,
        const EintragAenderung(infos: [InfoAenderung('i1', wert: '14,2')]),
      );

      expect(eintrag.korrektur('infos.i1.wert'), isNull);
      // Die bestehende Korrektur bleibt unverändert.
      expect(
        eintrag.korrektur('infos.i2.wert')?.zeitpunkt,
        DateTime.utc(2026, 11, 3, 19, 5, 31),
      );
    });

    test('Beispiel aus 3.5: dokumentdatum und i2 in einer Anfrage', () async {
      final eintrag = await mock.korrigieren(
        laborId,
        EintragAenderung(dokumentdatum: Wert(Datum(2026, 10, 15))),
      );
      expect(eintrag.dokumentdatum, Datum(2026, 10, 15));
      final korrektur = eintrag.korrektur('dokumentdatum')!;
      expect(korrektur.erkannt, '2026-10-14');
      expect(korrektur.korrigiert, '2026-10-15');
    });

    test('Datum löschen und wiederherstellen', () async {
      var eintrag = await mock.korrigieren(
        laborId,
        const EintragAenderung(dokumentdatum: Wert(null)),
      );
      expect(eintrag.dokumentdatum, isNull);
      expect(eintrag.korrektur('dokumentdatum')?.erkannt, '2026-10-14');
      expect(eintrag.korrektur('dokumentdatum')?.korrigiert, isNull);

      // Ein Eintrag ohne Datum steht jetzt am Ende der Zeitleiste.
      final liste = await mock.eintraege(modul: 'gesundheit');
      expect(liste.last.id, laborId);

      eintrag = await mock.korrigieren(
        laborId,
        EintragAenderung(dokumentdatum: Wert(Datum(2026, 10, 14))),
      );
      expect(eintrag.dokumentdatum, Datum(2026, 10, 14));
      expect(eintrag.korrektur('dokumentdatum'), isNull);
    });

    test('Einheit entfernen', () async {
      final eintrag = await mock.korrigieren(
        laborId,
        const EintragAenderung(
          infos: [InfoAenderung('i1', einheit: Wert(null))],
        ),
      );
      expect(eintrag.infos[0].einheit, isNull);
      expect(eintrag.infos[0].wert, '14,2');
      final korrektur = eintrag.korrektur('infos.i1.einheit')!;
      expect(korrektur.erkannt, 'g/dl');
      expect(korrektur.korrigiert, isNull);
    });

    test('Einheit ergänzen, wo keine erkannt wurde', () async {
      final eintrag = await mock.korrigieren(
        laborId,
        const EintragAenderung(
          infos: [InfoAenderung('i4', einheit: Wert('Typ'))],
        ),
      );
      expect(eintrag.infos[3].einheit, 'Typ');
      expect(eintrag.korrektur('infos.i4.einheit')?.erkannt, isNull);
    });

    test('typ und quelle', () async {
      final eintrag = await mock.korrigieren(
        laborId,
        const EintragAenderung(typ: 'arztbrief', quelle: Wert('Labor X')),
      );
      expect(eintrag.typ, 'arztbrief');
      expect(eintrag.quelle, 'Labor X');
      expect(eintrag.korrektur('typ')?.erkannt, 'laborbefund');
      expect(
        eintrag.korrektur('quelle')?.erkannt,
        'Labor Muster GmbH, Salzburg',
      );
    });

    test('Quelle ergänzen, wo keine erkannt wurde', () async {
      final eintrag = await mock.korrigieren(
        sonstigesId,
        const EintragAenderung(quelle: Wert('Impfstelle')),
      );
      expect(eintrag.quelle, 'Impfstelle');
      expect(eintrag.korrektur('quelle')?.erkannt, isNull);
    });
  });

  group('Fehlerfälle', () {
    test('leerer Body', () async {
      await expectLater(
        mock.korrigiereMitJson(laborId, {}),
        wirftApiFehler(FehlerCode.ungueltigeAnfrage, status: 400),
      );
    });

    test('Datum im falschen Format oder nicht im Kalender', () async {
      await expectLater(
        mock.korrigiereMitJson(laborId, {'dokumentdatum': '15.10.2026'}),
        wirftApiFehler(FehlerCode.ungueltigeAnfrage, status: 400),
      );
      await expectLater(
        mock.korrigiereMitJson(laborId, {'dokumentdatum': '2026-02-30'}),
        wirftApiFehler(FehlerCode.ungueltigeAnfrage, status: 400),
      );
    });

    test('unbekannter typ', () async {
      await expectLater(
        mock.korrigiereMitJson(laborId, {'typ': 'kochrezept'}),
        wirftApiFehler(FehlerCode.ungueltigeAnfrage, status: 400),
      );
    });

    test('nicht korrigierbare Felder', () async {
      for (final feld in ['volltext', 'modul', 'importdatum', 'datei']) {
        await expectLater(
          mock.korrigiereMitJson(laborId, {feld: 'x'}),
          wirftApiFehler(FehlerCode.feldNichtKorrigierbar, status: 400),
          reason: feld,
        );
      }
    });

    test('unbekanntes Feld', () async {
      await expectLater(
        mock.korrigiereMitJson(laborId, {'farbe': 'rot'}),
        wirftApiFehler(FehlerCode.ungueltigeAnfrage, status: 400),
      );
    });

    test('Info ohne id', () async {
      await expectLater(
        mock.korrigiereMitJson(laborId, {
          'infos': [
            {'wert': '1'},
          ],
        }),
        wirftApiFehler(FehlerCode.ungueltigeAnfrage, status: 400),
      );
    });

    test('unbekannte Info: 404 INFO_NICHT_GEFUNDEN mit infoId', () async {
      try {
        await mock.korrigieren(
          laborId,
          const EintragAenderung(infos: [InfoAenderung('i9', wert: '1')]),
        );
        fail('Fehler erwartet');
      } on ApiFehler catch (f) {
        expect(f.httpStatus, 404);
        expect(f.code, FehlerCode.infoNichtGefunden);
        expect(f.details['infoId'], 'i9');
      }
    });

    test('ist ein Teil ungültig, ändert sich gar nichts', () async {
      await expectLater(
        mock.korrigiereMitJson(laborId, {
          'dokumentdatum': '2026-10-20',
          'infos': [
            {'id': 'i9', 'wert': '1'},
          ],
        }),
        wirftApiFehler(FehlerCode.infoNichtGefunden),
      );
      final eintrag = await mock.eintrag(laborId);
      expect(eintrag.dokumentdatum, Datum(2026, 10, 14));
      expect(eintrag.korrekturen, hasLength(1));
    });

    test('ungültige und unbekannte Eintrag-ID', () async {
      const aenderung = EintragAenderung(typ: 'rezept');
      await expectLater(
        mock.korrigieren('123', aenderung),
        wirftApiFehler(FehlerCode.ungueltigeId, status: 400),
      );
      await expectLater(
        mock.korrigieren('ffffffffffffffffffffffff', aenderung),
        wirftApiFehler(FehlerCode.eintragNichtGefunden, status: 404),
      );
    });
  });
}
