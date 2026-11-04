// Tests für die Erweiterungen aus docs/API.md, Abschnitt 5: modul,
// Text-Import, Löschen und die Regeln zum Korrigieren (Leerraum).

import 'dart:convert';

import 'package:crypto/crypto.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:lifelog/api/api_client.dart';
import 'package:lifelog/api/api_fehler.dart';
import 'package:lifelog/api/mock_api_client.dart';
import 'package:lifelog/model/datum.dart';
import 'package:lifelog/model/eintrag_aenderung.dart';

import '../hilfen.dart';
import '../module/test_katalog.dart';

const g = 'gesundheit';

void main() {
  group('modul', () {
    test('Liste nur mit Einträgen des Moduls', () async {
      // Im Test-Katalog hat Sport den Typ „training“, Import ist möglich.
      final mock = schnellerMock(katalog: testKatalog());
      final sport = await mock.hochladen(
        testPdf('lauf.pdf').bytes,
        'lauf.pdf',
        modul: 'sport',
      );
      expect(sport.modul, 'sport');
      expect(sport.typ, 'training');

      expect(await mock.eintraege(modul: g), hasLength(3));
      final liste = await mock.eintraege(modul: 'sport');
      expect(liste.map((e) => e.id), [sport.id]);
    });

    test('ohne oder mit leerem modul gilt gesundheit', () async {
      final mock = schnellerMock();
      expect(await mock.eintraegeMitParametern({}), hasLength(3));
      expect(await mock.eintraegeMitParametern({'modul': ''}), hasLength(3));
    });

    test('unbekanntes Modul in der Liste: 400 UNGUELTIGER_PARAMETER', () async {
      try {
        await schnellerMock().eintraege(modul: 'garten');
        fail('Fehler erwartet');
      } on ApiFehler catch (f) {
        expect(f.httpStatus, 400);
        expect(f.code, FehlerCode.ungueltigerParameter);
        expect(f.details, {'parameter': 'modul', 'wert': 'garten'});
      }
    });

    test('unbekanntes Modul bei Upload, Text und Frage: 400 '
        'UNGUELTIGE_ANFRAGE', () async {
      final mock = schnellerMock();
      for (final aufruf in [
        () => mock.hochladen(testPdf('a.pdf').bytes, 'a.pdf', modul: 'garten'),
        () => mock.textImportieren('Text', modul: 'garten'),
        () => mock.fragen('Hämoglobin', modul: 'garten'),
      ]) {
        try {
          await aufruf();
          fail('Fehler erwartet');
        } on ApiFehler catch (f) {
          expect(f.code, FehlerCode.ungueltigeAnfrage);
          expect(f.details, {'feld': 'modul', 'wert': 'garten'});
        }
      }
    });

    test(
      'Frage: erst die Frage prüfen, dann das Modul (wie Backend)',
      () async {
        await expectLater(
          schnellerMock().fragen('  ', modul: 'garten'),
          wirftApiFehler(FehlerCode.frageFehlt),
        );
      },
    );

    test('Modul ohne Typen: kein Import möglich', () async {
      // Im Standard-Katalog hat Ernährung noch keine Typen.
      await expectLater(
        schnellerMock().textImportieren('Müsli', modul: 'ernaehrung'),
        wirftApiFehler(FehlerCode.ungueltigeAnfrage, status: 400),
      );
    });

    test('Frage sucht nur im Modul, Quellen tragen modul', () async {
      final mock = schnellerMock(katalog: testKatalog());
      final treffer = await mock.fragen('Hämoglobin', modul: g);
      expect(treffer.quellen.single.modul, g);

      final anderes = await mock.fragen('Hämoglobin', modul: 'sport');
      expect(anderes.gefunden, isFalse);
      expect(anderes.antwort, keinTrefferAntwort);
    });
  });

  group('Text-Import', () {
    const text =
        '  Hausarztpraxis Beispiel  \n'
        'Überweisung vom 31.02.2026, korrigiert am 21.10.2026\n'
        'zur Physiotherapie';

    test('legt einen Eintrag mit text.txt an', () async {
      final mock = schnellerMock();
      final eintrag = await mock.textImportieren(text, modul: g);
      final bytes = utf8.encode(text);

      expect(eintrag.modul, g);
      expect(eintrag.typ, 'sonstiges');
      expect(eintrag.volltext, text); // unverändert
      expect(eintrag.quelle, 'Hausarztpraxis Beispiel');
      // 31.02. gibt es nicht; das erste gültige Datum zählt.
      expect(eintrag.dokumentdatum, Datum(2026, 10, 21));
      expect(eintrag.infos, isEmpty);
      expect(eintrag.korrekturen, isEmpty);
      expect(eintrag.datei.dateiname, 'text.txt');
      expect(eintrag.datei.mimeTyp, 'text/plain; charset=utf-8');
      expect(eintrag.datei.istText, isTrue);
      expect(eintrag.datei.groesse, bytes.length);
      expect(eintrag.datei.sha256, sha256.convert(bytes).toString());

      final datei = await mock.datei(eintrag.id);
      expect(datei.istText, isTrue);
      expect(utf8.decode(datei.bytes), text);
      expect(await mock.eintraege(modul: g), hasLength(4));
    });

    test('ohne Datum im Text: dokumentdatum null', () async {
      final eintrag = await schnellerMock().textImportieren(
        'Notiz ohne Datum',
        modul: g,
      );
      expect(eintrag.dokumentdatum, isNull);
    });

    test('leer oder nur Leerzeichen: 400 DATEI_FEHLT', () async {
      for (final leer in ['', '   ', '\n\t ']) {
        await expectLater(
          schnellerMock().textImportieren(leer, modul: g),
          wirftApiFehler(FehlerCode.dateiFehlt, status: 400),
        );
      }
    });

    test('über 100 000 Zeichen: 413, genau 100 000 ist erlaubt', () async {
      await expectLater(
        schnellerMock().textImportieren('a' * (maxTextLaenge + 1), modul: g),
        wirftApiFehler(FehlerCode.dateiZuGross, status: 413),
      );
      final eintrag = await schnellerMock().textImportieren(
        'a' * maxTextLaenge,
        modul: g,
      );
      expect(eintrag.volltext.length, maxTextLaenge);
    });

    test(
      'derselbe Text zweimal: 409 DUPLIKAT; ein Leerzeichen mehr nicht',
      () async {
        final mock = schnellerMock();
        final erster = await mock.textImportieren(text, modul: g);
        try {
          await mock.textImportieren(text, modul: g);
          fail('Fehler erwartet');
        } on ApiFehler catch (f) {
          expect(f.code, FehlerCode.duplikat);
          expect(f.details['eintragId'], erster.id);
        }
        final zweiter = await mock.textImportieren('$text ', modul: g);
        expect(zweiter.id, isNot(erster.id));
      },
    );

    test('Fehler-Auslöser im Text, nichts gespeichert', () async {
      for (final (wort, code) in [
        ('unlesbar', FehlerCode.nichtLesbar),
        ('auslesefehler', FehlerCode.auslesenFehlgeschlagen),
        ('modellfehler', FehlerCode.modellNichtErreichbar),
      ]) {
        final mock = schnellerMock();
        await expectLater(
          mock.textImportieren('Test $wort', modul: g),
          wirftApiFehler(code),
        );
        expect(await mock.eintraege(modul: g), hasLength(3), reason: wort);
      }
    });

    test('importierter Text ist durchsuchbar', () async {
      final mock = schnellerMock();
      final eintrag = await mock.textImportieren(text, modul: g);
      final antwort = await mock.fragen('Physiotherapie', modul: g);
      expect(antwort.quellen.single.eintragId, eintrag.id);
      expect(antwort.quellen.single.auszug, 'zur Physiotherapie');
    });
  });

  group('Löschen', () {
    test('Eintrag und Datei sind weg, auch für Liste und Frage', () async {
      final mock = schnellerMock();
      await mock.loeschen(laborId);

      await expectLater(
        mock.eintrag(laborId),
        wirftApiFehler(FehlerCode.eintragNichtGefunden, status: 404),
      );
      await expectLater(
        mock.datei(laborId),
        wirftApiFehler(FehlerCode.eintragNichtGefunden, status: 404),
      );
      expect(await mock.eintraege(modul: g), hasLength(2));
      final antwort = await mock.fragen('Hämoglobin', modul: g);
      expect(antwort.gefunden, isFalse);
    });

    test('zweites Löschen: 404; ungültige ID: 400', () async {
      final mock = schnellerMock();
      await mock.loeschen(laborId);
      await expectLater(
        mock.loeschen(laborId),
        wirftApiFehler(FehlerCode.eintragNichtGefunden, status: 404),
      );
      await expectLater(
        mock.loeschen('123'),
        wirftApiFehler(FehlerCode.ungueltigeId, status: 400),
      );
    });

    test('danach lässt sich dieselbe Datei wieder hochladen', () async {
      final mock = schnellerMock();
      final datei = testPdf('demo.pdf');
      final erster = await mock.hochladen(datei.bytes, 'demo.pdf', modul: g);
      await mock.loeschen(erster.id);

      final zweiter = await mock.hochladen(datei.bytes, 'demo.pdf', modul: g);
      expect(zweiter.id, isNot(erster.id));
    });

    test('Fehler im Backend: nichts gelöscht', () async {
      final mock = schnellerMock()..simulierterFehler = true;
      await expectLater(
        mock.loeschen(laborId),
        wirftApiFehler(FehlerCode.internerFehler, status: 500),
      );
      mock.simulierterFehler = false;
      expect((await mock.eintrag(laborId)).id, laborId);
    });
  });

  group('Korrigieren: Leerraum (docs/API.md 3.5)', () {
    late MockApiClient mock;
    setUp(() => mock = schnellerMock());

    test('Leerraum wird entfernt', () async {
      final eintrag = await mock.korrigiereMitJson(laborId, {
        'quelle': '  Labor X  ',
        'infos': [
          {'id': 'i1', 'wert': ' 15,0 ', 'einheit': ' g/l '},
        ],
      });
      expect(eintrag.quelle, 'Labor X');
      expect(eintrag.infos.first.wert, '15,0');
      expect(eintrag.infos.first.einheit, 'g/l');
    });

    test('leere quelle und einheit gelten als null', () async {
      final eintrag = await mock.korrigiereMitJson(laborId, {
        'quelle': '   ',
        'infos': [
          {'id': 'i1', 'einheit': ' '},
        ],
      });
      expect(eintrag.quelle, isNull);
      expect(eintrag.korrektur('quelle')?.korrigiert, isNull);
      expect(eintrag.infos.first.einheit, isNull);
      expect(eintrag.korrektur('infos.i1.einheit')?.erkannt, 'g/dl');
    });

    test('leerer wert ist ein Fehler, nichts geändert', () async {
      try {
        await mock.korrigiereMitJson(laborId, {
          'quelle': 'Neu',
          'infos': [
            {'id': 'i1', 'wert': '  '},
          ],
        });
        fail('Fehler erwartet');
      } on ApiFehler catch (f) {
        expect(f.code, FehlerCode.ungueltigeAnfrage);
        expect(f.details['feld'], 'infos.i1.wert');
      }
      expect(
        (await mock.eintrag(laborId)).quelle,
        'Labor Muster GmbH, Salzburg',
      );
    });

    test('nach dem Abschneiden unverändert: keine Korrektur', () async {
      final eintrag = await mock.korrigieren(
        laborId,
        const EintragAenderung(infos: [InfoAenderung('i1', wert: ' 14,2 ')]),
      );
      expect(eintrag.korrektur('infos.i1.wert'), isNull);
    });
  });
}
