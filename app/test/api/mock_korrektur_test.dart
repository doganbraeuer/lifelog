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
          const EintragAenderung(infos: [InfoAenderung('f2', wert: '7,0')]),
        );

        expect(eintrag.infos[1].wert, '7,0');
        final korrekturen = eintrag.korrekturen.where(
          (k) => k.feld == 'facts.f2.value',
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
          const EintragAenderung(infos: [InfoAenderung('f2', wert: '6,3')]),
        );

        expect(eintrag.infos[1].wert, '6,3');
        expect(eintrag.korrektur('facts.f2.value'), isNull);
        expect(eintrag.infoKorrigiert('f2'), isFalse);
      },
    );

    test('unveränderter Wert erzeugt keine Korrektur', () async {
      final eintrag = await mock.korrigieren(
        laborId,
        const EintragAenderung(infos: [InfoAenderung('f1', wert: '14,2')]),
      );

      expect(eintrag.korrektur('facts.f1.value'), isNull);
      // Die bestehende Korrektur bleibt unverändert.
      expect(
        eintrag.korrektur('facts.f2.value')?.zeitpunkt,
        DateTime.utc(2026, 11, 3, 19, 5, 31),
      );
    });

    test('Beispiel aus 3.5: dokumentdatum und i2 in einer Anfrage', () async {
      final eintrag = await mock.korrigieren(
        laborId,
        EintragAenderung(dokumentdatum: Wert(Datum(2026, 10, 15))),
      );
      expect(eintrag.dokumentdatum, Datum(2026, 10, 15));
      final korrektur = eintrag.korrektur('documentDate')!;
      expect(korrektur.erkannt, '2026-10-14');
      expect(korrektur.korrigiert, '2026-10-15');
    });

    test('Datum löschen und wiederherstellen', () async {
      var eintrag = await mock.korrigieren(
        laborId,
        const EintragAenderung(dokumentdatum: Wert(null)),
      );
      expect(eintrag.dokumentdatum, isNull);
      expect(eintrag.korrektur('documentDate')?.erkannt, '2026-10-14');
      expect(eintrag.korrektur('documentDate')?.korrigiert, isNull);

      // Ein Eintrag ohne Datum steht jetzt am Ende der Zeitleiste.
      final liste = await mock.eintraege(modul: 'health');
      expect(liste.last.id, laborId);

      eintrag = await mock.korrigieren(
        laborId,
        EintragAenderung(dokumentdatum: Wert(Datum(2026, 10, 14))),
      );
      expect(eintrag.dokumentdatum, Datum(2026, 10, 14));
      expect(eintrag.korrektur('documentDate'), isNull);
    });

    test('Einheit entfernen', () async {
      final eintrag = await mock.korrigieren(
        laborId,
        const EintragAenderung(
          infos: [InfoAenderung('f1', einheit: Wert(null))],
        ),
      );
      expect(eintrag.infos[0].einheit, isNull);
      expect(eintrag.infos[0].wert, '14,2');
      final korrektur = eintrag.korrektur('facts.f1.unit')!;
      expect(korrektur.erkannt, 'g/dl');
      expect(korrektur.korrigiert, isNull);
    });

    test('Einheit ergänzen, wo keine erkannt wurde', () async {
      final eintrag = await mock.korrigieren(
        laborId,
        const EintragAenderung(
          infos: [InfoAenderung('f4', einheit: Wert('Typ'))],
        ),
      );
      expect(eintrag.infos[3].einheit, 'Typ');
      expect(eintrag.korrektur('facts.f4.unit')?.erkannt, isNull);
    });

    test('typ und quelle', () async {
      final eintrag = await mock.korrigieren(
        laborId,
        const EintragAenderung(typ: 'doctor_letter', quelle: Wert('Labor X')),
      );
      expect(eintrag.typ, 'doctor_letter');
      expect(eintrag.quelle, 'Labor X');
      expect(eintrag.korrektur('type')?.erkannt, 'lab_report');
      expect(
        eintrag.korrektur('issuer')?.erkannt,
        'Labor Muster GmbH, Salzburg',
      );
    });

    test('Quelle ergänzen, wo keine erkannt wurde', () async {
      final eintrag = await mock.korrigieren(
        sonstigesId,
        const EintragAenderung(quelle: Wert('Impfstelle')),
      );
      expect(eintrag.quelle, 'Impfstelle');
      expect(eintrag.korrektur('issuer')?.erkannt, isNull);
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
        mock.korrigiereMitJson(laborId, {'documentDate': '15.10.2026'}),
        wirftApiFehler(FehlerCode.ungueltigeAnfrage, status: 400),
      );
      await expectLater(
        mock.korrigiereMitJson(laborId, {'documentDate': '2026-02-30'}),
        wirftApiFehler(FehlerCode.ungueltigeAnfrage, status: 400),
      );
    });

    test('unbekannter typ', () async {
      await expectLater(
        mock.korrigiereMitJson(laborId, {'type': 'kochrezept'}),
        wirftApiFehler(FehlerCode.ungueltigeAnfrage, status: 400),
      );
    });

    test('nicht korrigierbare Felder', () async {
      for (final feld in ['fullText', 'module', 'importedAt', 'file']) {
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
          'facts': [
            {'value': '1'},
          ],
        }),
        wirftApiFehler(FehlerCode.ungueltigeAnfrage, status: 400),
      );
    });

    test('unbekannte Info: 404 FACT_NOT_FOUND mit infoId', () async {
      try {
        await mock.korrigieren(
          laborId,
          const EintragAenderung(infos: [InfoAenderung('f9', wert: '1')]),
        );
        fail('Fehler erwartet');
      } on ApiFehler catch (f) {
        expect(f.httpStatus, 404);
        expect(f.code, FehlerCode.infoNichtGefunden);
        expect(f.details['factId'], 'f9');
      }
    });

    test('ist ein Teil ungültig, ändert sich gar nichts', () async {
      await expectLater(
        mock.korrigiereMitJson(laborId, {
          'documentDate': '2026-10-20',
          'facts': [
            {'id': 'f9', 'value': '1'},
          ],
        }),
        wirftApiFehler(FehlerCode.infoNichtGefunden),
      );
      final eintrag = await mock.eintrag(laborId);
      expect(eintrag.dokumentdatum, Datum(2026, 10, 14));
      expect(eintrag.korrekturen, hasLength(1));
    });

    test('ungültige und unbekannte Eintrag-ID', () async {
      const aenderung = EintragAenderung(typ: 'prescription');
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
