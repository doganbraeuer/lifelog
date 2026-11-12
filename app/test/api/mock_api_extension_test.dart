// Tests für die Erweiterungen aus docs/API.md, Abschnitt 5: modul,
// Text-Import, Löschen und die Regeln zum Korrigieren (Leerraum).

import 'dart:convert';

import 'package:crypto/crypto.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:lifelog/api/api_client.dart';
import 'package:lifelog/api/api_exception.dart';
import 'package:lifelog/api/mock_api_client.dart';
import 'package:lifelog/model/plain_date.dart';
import 'package:lifelog/model/entry_patch.dart';

import '../helpers.dart';
import '../module/test_catalog.dart';

const g = 'health';

void main() {
  group('module', () {
    test('Liste nur mit Einträgen des Moduls', () async {
      // Im Test-Katalog hat Sport den Typ „training“, Import ist möglich.
      final mock = schnellerMock(catalog: testKatalog());
      final sport = await mock.upload(
        testPdf('lauf.pdf').bytes,
        'lauf.pdf',
        module: 'sport',
      );
      expect(sport.module, 'sport');
      expect(sport.type, 'training');

      expect(await mock.listEntries(module: g), hasLength(3));
      final liste = await mock.listEntries(module: 'sport');
      expect(liste.map((e) => e.id), [sport.id]);
    });

    test('ohne oder mit leerem modul gilt gesundheit', () async {
      final mock = schnellerMock();
      expect(await mock.listEntriesWithParameters({}), hasLength(3));
      expect(
        await mock.listEntriesWithParameters({'module': ''}),
        hasLength(3),
      );
    });

    test('unbekanntes Modul in der Liste: 400 INVALID_PARAMETER', () async {
      try {
        await schnellerMock().listEntries(module: 'garten');
        fail('Fehler erwartet');
      } on ApiException catch (f) {
        expect(f.httpStatus, 400);
        expect(f.code, ErrorCode.invalidParameter);
        expect(f.details, {'parameter': 'module', 'value': 'garten'});
      }
    });

    test('unbekanntes Modul bei Upload, Text und Frage: 400 '
        'INVALID_REQUEST', () async {
      final mock = schnellerMock();
      for (final aufruf in [
        () => mock.upload(testPdf('a.pdf').bytes, 'a.pdf', module: 'garten'),
        () => mock.importText('Text', module: 'garten'),
        () => mock.ask('Hämoglobin', module: 'garten'),
      ]) {
        try {
          await aufruf();
          fail('Fehler erwartet');
        } on ApiException catch (f) {
          expect(f.code, ErrorCode.invalidRequest);
          expect(f.details, {'field': 'module', 'value': 'garten'});
        }
      }
    });

    test(
      'Frage: erst die Frage prüfen, dann das Modul (wie Backend)',
      () async {
        await expectLater(
          schnellerMock().ask('  ', module: 'garten'),
          wirftApiFehler(ErrorCode.questionMissing),
        );
      },
    );

    test('Modul ohne Typen: kein Import möglich', () async {
      // Im Standard-Katalog hat Ernährung noch keine Typen.
      await expectLater(
        schnellerMock().importText('Müsli', module: 'nutrition'),
        wirftApiFehler(ErrorCode.invalidRequest, status: 400),
      );
    });

    test('Frage sucht nur im Modul, Quellen tragen modul', () async {
      final mock = schnellerMock(catalog: testKatalog());
      final treffer = await mock.ask('Hämoglobin', module: g);
      expect(treffer.sources.single.module, g);

      final anderes = await mock.ask('Hämoglobin', module: 'sport');
      expect(anderes.found, isFalse);
      expect(anderes.answer, notFoundAnswer);
    });
  });

  group('Text-Import', () {
    const text =
        '  Hausarztpraxis Beispiel  \n'
        'Überweisung vom 31.02.2026, korrigiert am 21.10.2026\n'
        'zur Physiotherapie';

    test('legt einen Eintrag mit text.txt an', () async {
      final mock = schnellerMock();
      final entry = await mock.importText(text, module: g);
      final bytes = utf8.encode(text);

      expect(entry.module, g);
      expect(entry.type, 'other');
      expect(entry.fullText, text); // unverändert
      expect(entry.issuer, 'Hausarztpraxis Beispiel');
      // 31.02. gibt es nicht; das erste gültige Datum zählt.
      expect(entry.documentDate, PlainDate(2026, 10, 21));
      expect(entry.facts, isEmpty);
      expect(entry.corrections, isEmpty);
      expect(entry.file.fileName, 'text.txt');
      expect(entry.file.mimeType, 'text/plain; charset=utf-8');
      expect(entry.file.isText, isTrue);
      expect(entry.file.size, bytes.length);
      expect(entry.file.sha256, sha256.convert(bytes).toString());

      final file = await mock.getFile(entry.id);
      expect(file.isText, isTrue);
      expect(utf8.decode(file.bytes), text);
      expect(await mock.listEntries(module: g), hasLength(4));
    });

    test('ohne Datum im Text: dokumentdatum null', () async {
      final entry = await schnellerMock().importText(
        'Notiz ohne Datum',
        module: g,
      );
      expect(entry.documentDate, isNull);
    });

    test('leer oder nur Leerzeichen: 400 FILE_MISSING', () async {
      for (final leer in ['', '   ', '\n\t ']) {
        await expectLater(
          schnellerMock().importText(leer, module: g),
          wirftApiFehler(ErrorCode.fileMissing, status: 400),
        );
      }
    });

    test('über 100 000 Zeichen: 413, genau 100 000 ist erlaubt', () async {
      await expectLater(
        schnellerMock().importText('a' * (maxTextLength + 1), module: g),
        wirftApiFehler(ErrorCode.fileTooLarge, status: 413),
      );
      final entry = await schnellerMock().importText(
        'a' * maxTextLength,
        module: g,
      );
      expect(entry.fullText.length, maxTextLength);
    });

    test(
      'derselbe Text zweimal: 409 DUPLICATE; ein Leerzeichen mehr nicht',
      () async {
        final mock = schnellerMock();
        final erster = await mock.importText(text, module: g);
        try {
          await mock.importText(text, module: g);
          fail('Fehler erwartet');
        } on ApiException catch (f) {
          expect(f.code, ErrorCode.duplicate);
          expect(f.details['entryId'], erster.id);
        }
        final zweiter = await mock.importText('$text ', module: g);
        expect(zweiter.id, isNot(erster.id));
      },
    );

    test('Fehler-Auslöser im Text, nichts gespeichert', () async {
      for (final (wort, code) in [
        ('unreadable', ErrorCode.unreadable),
        ('extractionerror', ErrorCode.extractionFailed),
        ('modelerror', ErrorCode.modelUnavailable),
      ]) {
        final mock = schnellerMock();
        await expectLater(
          mock.importText('Test $wort', module: g),
          wirftApiFehler(code),
        );
        expect(await mock.listEntries(module: g), hasLength(3), reason: wort);
      }
    });

    test('importierter Text ist durchsuchbar', () async {
      final mock = schnellerMock();
      final entry = await mock.importText(text, module: g);
      final answer = await mock.ask('Physiotherapie', module: g);
      expect(answer.sources.single.entryId, entry.id);
      expect(answer.sources.single.excerpt, 'zur Physiotherapie');
    });
  });

  group('Löschen', () {
    test('Eintrag und Datei sind weg, auch für Liste und Frage', () async {
      final mock = schnellerMock();
      await mock.deleteEntry(laborId);

      await expectLater(
        mock.getEntry(laborId),
        wirftApiFehler(ErrorCode.entryNotFound, status: 404),
      );
      await expectLater(
        mock.getFile(laborId),
        wirftApiFehler(ErrorCode.entryNotFound, status: 404),
      );
      expect(await mock.listEntries(module: g), hasLength(2));
      final answer = await mock.ask('Hämoglobin', module: g);
      expect(answer.found, isFalse);
    });

    test('zweites Löschen: 404; ungültige ID: 400', () async {
      final mock = schnellerMock();
      await mock.deleteEntry(laborId);
      await expectLater(
        mock.deleteEntry(laborId),
        wirftApiFehler(ErrorCode.entryNotFound, status: 404),
      );
      await expectLater(
        mock.deleteEntry('123'),
        wirftApiFehler(ErrorCode.invalidId, status: 400),
      );
    });

    test('danach lässt sich dieselbe Datei wieder hochladen', () async {
      final mock = schnellerMock();
      final file = testPdf('demo.pdf');
      final erster = await mock.upload(file.bytes, 'demo.pdf', module: g);
      await mock.deleteEntry(erster.id);

      final zweiter = await mock.upload(file.bytes, 'demo.pdf', module: g);
      expect(zweiter.id, isNot(erster.id));
    });

    test('Fehler im Backend: nichts gelöscht', () async {
      final mock = schnellerMock()..simulateError = true;
      await expectLater(
        mock.deleteEntry(laborId),
        wirftApiFehler(ErrorCode.internalError, status: 500),
      );
      mock.simulateError = false;
      expect((await mock.getEntry(laborId)).id, laborId);
    });
  });

  group('Korrigieren: Leerraum (docs/API.md 3.5)', () {
    late MockApiClient mock;
    setUp(() => mock = schnellerMock());

    test('Leerraum wird entfernt', () async {
      final entry = await mock.correctWithJson(laborId, {
        'issuer': '  Labor X  ',
        'facts': [
          {'id': 'f1', 'value': ' 15,0 ', 'unit': ' g/l '},
        ],
      });
      expect(entry.issuer, 'Labor X');
      expect(entry.facts.first.value, '15,0');
      expect(entry.facts.first.unit, 'g/l');
    });

    test('leere quelle und einheit gelten als null', () async {
      final entry = await mock.correctWithJson(laborId, {
        'issuer': '   ',
        'facts': [
          {'id': 'f1', 'unit': ' '},
        ],
      });
      expect(entry.issuer, isNull);
      expect(entry.correctionFor('issuer')?.corrected, isNull);
      expect(entry.facts.first.unit, isNull);
      expect(entry.correctionFor('facts.f1.unit')?.recognized, 'g/dl');
    });

    test('leerer wert ist ein Fehler, nichts geändert', () async {
      try {
        await mock.correctWithJson(laborId, {
          'issuer': 'Neu',
          'facts': [
            {'id': 'f1', 'value': '  '},
          ],
        });
        fail('Fehler erwartet');
      } on ApiException catch (f) {
        expect(f.code, ErrorCode.invalidRequest);
        expect(f.details['field'], 'facts.f1.value');
      }
      expect(
        (await mock.getEntry(laborId)).issuer,
        'Labor Muster GmbH, Salzburg',
      );
    });

    test('nach dem Abschneiden unverändert: keine Korrektur', () async {
      final entry = await mock.correct(
        laborId,
        const EntryPatch(facts: [FactPatch('f1', value: ' 14,2 ')]),
      );
      expect(entry.correctionFor('facts.f1.value'), isNull);
    });
  });
}
