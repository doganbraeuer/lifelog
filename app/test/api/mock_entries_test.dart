import 'package:flutter_test/flutter_test.dart';
import 'package:lifelog/api/api_client.dart';
import 'package:lifelog/api/api_exception.dart';
import 'package:lifelog/model/plain_date.dart';

import '../helpers.dart';

void main() {
  group('entries', () {
    test(
      'Standard: Reihenfolge wie im Beispiel aus docs/API.md, 3.2',
      () async {
        final liste = await schnellerMock().eintraege(modul: 'health');
        expect(liste.map((e) => e.id), [laborId, arztbriefId, sonstigesId]);
        expect(liste.last.dokumentdatum, isNull);
        expect(liste.last.quelle, isNull);
      },
    );

    test('importdatum aufsteigend', () async {
      final liste = await schnellerMock().eintraege(
        sortierung: Sortierung.importedAt,
        richtung: Richtung.asc,
        modul: 'health',
      );
      expect(liste.map((e) => e.id), [laborId, arztbriefId, sonstigesId]);
    });

    test('dokumentdatum aufsteigend: ohne Datum am Ende', () async {
      final liste = await schnellerMock().eintraege(
        richtung: Richtung.asc,
        modul: 'health',
      );
      expect(liste.map((e) => e.id), [arztbriefId, laborId, sonstigesId]);
    });

    test('ohne Parameter gelten die Standardwerte', () async {
      final liste = await schnellerMock().eintraegeMitParametern({});
      expect(liste.first.id, laborId);
    });

    test('unbekannte Sortierung: 400 INVALID_PARAMETER', () async {
      final mock = schnellerMock();
      await expectLater(
        mock.eintraegeMitParametern({'sort': 'issuer'}),
        wirftApiFehler(FehlerCode.ungueltigerParameter, status: 400),
      );
      try {
        await mock.eintraegeMitParametern({'sort': 'issuer'});
      } on ApiFehler catch (f) {
        expect(f.details, {'parameter': 'sort', 'value': 'issuer'});
      }
    });

    test('unbekannte Richtung: 400 INVALID_PARAMETER', () async {
      await expectLater(
        schnellerMock().eintraegeMitParametern({'order': 'seitwaerts'}),
        wirftApiFehler(FehlerCode.ungueltigerParameter, status: 400),
      );
    });

    test('ohne Einträge eine leere Liste', () async {
      expect(
        await schnellerMock(mitBeispieldaten: false)
            .eintraege(modul: 'health'),
        isEmpty,
      );
    });
  });

  group('eintrag', () {
    test('liefert den vollständigen Laborbefund', () async {
      final eintrag = await schnellerMock().eintrag(laborId);
      expect(eintrag.typ, 'lab_report');
      expect(eintrag.dokumentdatum, Datum(2026, 10, 14));
      expect(eintrag.infos, hasLength(4));
      expect(eintrag.korrektur('facts.f2.value')?.erkannt, '6,3');
    });

    test('ungültige ID: 400 INVALID_ID', () async {
      await expectLater(
        schnellerMock().eintrag('123'),
        wirftApiFehler(FehlerCode.ungueltigeId, status: 400),
      );
      await expectLater(
        schnellerMock().eintrag('zzzzzzzzzzzzzzzzzzzzzzzz'),
        wirftApiFehler(FehlerCode.ungueltigeId, status: 400),
      );
    });

    test('unbekannte ID: 404 ENTRY_NOT_FOUND', () async {
      await expectLater(
        schnellerMock().eintrag('ffffffffffffffffffffffff'),
        wirftApiFehler(FehlerCode.eintragNichtGefunden, status: 404),
      );
    });
  });

  group('file', () {
    test('Laborbefund ist ein PDF', () async {
      final datei = await schnellerMock().datei(laborId);
      expect(datei.mimeTyp, 'application/pdf');
      expect(String.fromCharCodes(datei.bytes.take(5)), '%PDF-');
      expect(
        String.fromCharCodes(datei.bytes.skip(datei.bytes.length - 6)),
        '%%EOF\n',
      );
    });

    test('Arztbrief ist ein PNG', () async {
      final datei = await schnellerMock().datei(arztbriefId);
      expect(datei.mimeTyp, 'image/png');
      expect(datei.bytes.take(4), [0x89, 0x50, 0x4E, 0x47]);
    });

    test('Fehler wie bei eintrag', () async {
      await expectLater(
        schnellerMock().datei('123'),
        wirftApiFehler(FehlerCode.ungueltigeId),
      );
      await expectLater(
        schnellerMock().datei('ffffffffffffffffffffffff'),
        wirftApiFehler(FehlerCode.eintragNichtGefunden),
      );
    });
  });

  test('simulierterFehler: 500 INTERNAL_ERROR', () async {
    final mock = schnellerMock()..simulierterFehler = true;
    await expectLater(
      mock.eintraege(modul: 'health'),
      wirftApiFehler(FehlerCode.internerFehler, status: 500),
    );
  });
}
