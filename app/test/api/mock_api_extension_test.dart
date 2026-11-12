// Tests for the extensions from docs/API.md, section 5: module,
// text import, delete and the rules for correcting (whitespace).

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
    test('list only with entries of the module', () async {
      // In the test catalog sport has the type „training“, import is possible.
      final mock = fastMock(catalog: testCatalog());
      final sport = await mock.upload(
        testPdf('lauf.pdf').bytes,
        'lauf.pdf',
        module: 'sport',
      );
      expect(sport.module, 'sport');
      expect(sport.type, 'training');

      expect(await mock.listEntries(module: g), hasLength(3));
      final list = await mock.listEntries(module: 'sport');
      expect(list.map((e) => e.id), [sport.id]);
    });

    test('without or with an empty module, health applies', () async {
      final mock = fastMock();
      expect(await mock.listEntriesWithParameters({}), hasLength(3));
      expect(
        await mock.listEntriesWithParameters({'module': ''}),
        hasLength(3),
      );
    });

    test('unknown module in the list: 400 INVALID_PARAMETER', () async {
      try {
        await fastMock().listEntries(module: 'garden');
        fail('Fehler erwartet');
      } on ApiException catch (f) {
        expect(f.httpStatus, 400);
        expect(f.code, ErrorCode.invalidParameter);
        expect(f.details, {'parameter': 'module', 'value': 'garden'});
      }
    });

    test('unknown module for upload, text and question: 400 '
        'INVALID_REQUEST', () async {
      final mock = fastMock();
      for (final methodCall in [
        () => mock.upload(testPdf('a.pdf').bytes, 'a.pdf', module: 'garden'),
        () => mock.importText('Text', module: 'garden'),
        () => mock.ask('Hämoglobin', module: 'garden'),
      ]) {
        try {
          await methodCall();
          fail('Fehler erwartet');
        } on ApiException catch (f) {
          expect(f.code, ErrorCode.invalidRequest);
          expect(f.details, {'field': 'module', 'value': 'garden'});
        }
      }
    });

    test(
      'question: check the question first, then the module (as the backend)',
      () async {
        await expectLater(
          fastMock().ask('  ', module: 'garden'),
          throwsApiException(ErrorCode.questionMissing),
        );
      },
    );

    test('module without types: no import possible', () async {
      // In the default catalog nutrition has no types yet.
      await expectLater(
        fastMock().importText('Müsli', module: 'nutrition'),
        throwsApiException(ErrorCode.invalidRequest, status: 400),
      );
    });

    test('question only searches the module, sources carry module', () async {
      final mock = fastMock(catalog: testCatalog());
      final hit = await mock.ask('Hämoglobin', module: g);
      expect(hit.sources.single.module, g);

      final otherModule = await mock.ask('Hämoglobin', module: 'sport');
      expect(otherModule.found, isFalse);
      expect(otherModule.answer, notFoundAnswer);
    });
  });

  group('text import', () {
    const text =
        '  Hausarztpraxis Beispiel  \n'
        'Überweisung vom 31.02.2026, korrigiert am 21.10.2026\n'
        'zur Physiotherapie';

    test('creates an entry with text.txt', () async {
      final mock = fastMock();
      final entry = await mock.importText(text, module: g);
      final bytes = utf8.encode(text);

      expect(entry.module, g);
      expect(entry.type, 'other');
      expect(entry.fullText, text); // unchanged
      expect(entry.issuer, 'Hausarztpraxis Beispiel');
      // 31 February does not exist; the first valid date counts.
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

    test('without a date in the text: documentDate null', () async {
      final entry = await fastMock().importText('Notiz ohne Datum', module: g);
      expect(entry.documentDate, isNull);
    });

    test('empty or only spaces: 400 FILE_MISSING', () async {
      for (final empty in ['', '   ', '\n\t ']) {
        await expectLater(
          fastMock().importText(empty, module: g),
          throwsApiException(ErrorCode.fileMissing, status: 400),
        );
      }
    });

    test('over 100 000 characters: 413, exactly 100 000 is allowed', () async {
      await expectLater(
        fastMock().importText('a' * (maxTextLength + 1), module: g),
        throwsApiException(ErrorCode.fileTooLarge, status: 413),
      );
      final entry = await fastMock().importText('a' * maxTextLength, module: g);
      expect(entry.fullText.length, maxTextLength);
    });

    test(
      'the same text twice: 409 DUPLICATE; one more space does not',
      () async {
        final mock = fastMock();
        final firstEntry = await mock.importText(text, module: g);
        try {
          await mock.importText(text, module: g);
          fail('Fehler erwartet');
        } on ApiException catch (f) {
          expect(f.code, ErrorCode.duplicate);
          expect(f.details['entryId'], firstEntry.id);
        }
        final secondEntry = await mock.importText('$text ', module: g);
        expect(secondEntry.id, isNot(firstEntry.id));
      },
    );

    test('error triggers in the text, nothing stored', () async {
      for (final (word, code) in [
        ('unreadable', ErrorCode.unreadable),
        ('extractionerror', ErrorCode.extractionFailed),
        ('modelerror', ErrorCode.modelUnavailable),
      ]) {
        final mock = fastMock();
        await expectLater(
          mock.importText('Test $word', module: g),
          throwsApiException(code),
        );
        expect(await mock.listEntries(module: g), hasLength(3), reason: word);
      }
    });

    test('imported text is searchable', () async {
      final mock = fastMock();
      final entry = await mock.importText(text, module: g);
      final answer = await mock.ask('Physiotherapie', module: g);
      expect(answer.sources.single.entryId, entry.id);
      expect(answer.sources.single.excerpt, 'zur Physiotherapie');
    });
  });

  group('delete', () {
    test('entry and file are gone, also for list and question', () async {
      final mock = fastMock();
      await mock.deleteEntry(labReportId);

      await expectLater(
        mock.getEntry(labReportId),
        throwsApiException(ErrorCode.entryNotFound, status: 404),
      );
      await expectLater(
        mock.getFile(labReportId),
        throwsApiException(ErrorCode.entryNotFound, status: 404),
      );
      expect(await mock.listEntries(module: g), hasLength(2));
      final answer = await mock.ask('Hämoglobin', module: g);
      expect(answer.found, isFalse);
    });

    test('second delete: 404; invalid ID: 400', () async {
      final mock = fastMock();
      await mock.deleteEntry(labReportId);
      await expectLater(
        mock.deleteEntry(labReportId),
        throwsApiException(ErrorCode.entryNotFound, status: 404),
      );
      await expectLater(
        mock.deleteEntry('123'),
        throwsApiException(ErrorCode.invalidId, status: 400),
      );
    });

    test('afterwards the same file can be uploaded again', () async {
      final mock = fastMock();
      final file = testPdf('demo.pdf');
      final firstEntry = await mock.upload(file.bytes, 'demo.pdf', module: g);
      await mock.deleteEntry(firstEntry.id);

      final secondEntry = await mock.upload(file.bytes, 'demo.pdf', module: g);
      expect(secondEntry.id, isNot(firstEntry.id));
    });

    test('error in the backend: nothing deleted', () async {
      final mock = fastMock()..simulateError = true;
      await expectLater(
        mock.deleteEntry(labReportId),
        throwsApiException(ErrorCode.internalError, status: 500),
      );
      mock.simulateError = false;
      expect((await mock.getEntry(labReportId)).id, labReportId);
    });
  });

  group('correcting: whitespace (docs/API.md 3.5)', () {
    late MockApiClient mock;
    setUp(() => mock = fastMock());

    test('whitespace is removed', () async {
      final entry = await mock.correctWithJson(labReportId, {
        'issuer': '  Labor X  ',
        'facts': [
          {'id': 'f1', 'value': ' 15,0 ', 'unit': ' g/l '},
        ],
      });
      expect(entry.issuer, 'Labor X');
      expect(entry.facts.first.value, '15,0');
      expect(entry.facts.first.unit, 'g/l');
    });

    test('empty issuer and unit count as null', () async {
      final entry = await mock.correctWithJson(labReportId, {
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

    test('empty value is an error, nothing changed', () async {
      try {
        await mock.correctWithJson(labReportId, {
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
        (await mock.getEntry(labReportId)).issuer,
        'Labor Muster GmbH, Salzburg',
      );
    });

    test('unchanged after trimming: no correction', () async {
      final entry = await mock.correct(
        labReportId,
        const EntryPatch(facts: [FactPatch('f1', value: ' 14,2 ')]),
      );
      expect(entry.correctionFor('facts.f1.value'), isNull);
    });
  });
}
