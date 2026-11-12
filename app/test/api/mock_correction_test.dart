import 'package:flutter_test/flutter_test.dart';
import 'package:lifelog/api/api_exception.dart';
import 'package:lifelog/api/mock_api_client.dart';
import 'package:lifelog/model/plain_date.dart';
import 'package:lifelog/model/entry_patch.dart';

import '../helpers.dart';

void main() {
  late MockApiClient mock;
  final now = DateTime.utc(2026, 11, 4, 9, 30, 0);

  setUp(() => mock = fastMock(clock: () => now));

  group('correction rules from docs/API.md, section 2', () {
    test('rules 1 and 2: current value in the field, recognized stays the original', () async {
      // Starting point: f2 was already corrected from 6,3 to 6,8.
      final entry = await mock.correct(
        labReportId,
        const EntryPatch(facts: [FactPatch('f2', value: '7,0')]),
      );

      expect(entry.facts[1].value, '7,0');
      final corrections = entry.corrections.where(
        (k) => k.field == 'facts.f2.value',
      );
      expect(corrections, hasLength(1));
      expect(corrections.single.recognized, '6,3');
      expect(corrections.single.corrected, '7,0');
      expect(corrections.single.correctedAt, now);
    });

    test(
      'rule 3: back to the recognized value removes the correction',
      () async {
        final entry = await mock.correct(
          labReportId,
          const EntryPatch(facts: [FactPatch('f2', value: '6,3')]),
        );

        expect(entry.facts[1].value, '6,3');
        expect(entry.correctionFor('facts.f2.value'), isNull);
        expect(entry.isFactCorrected('f2'), isFalse);
      },
    );

    test('unchanged value creates no correction', () async {
      final entry = await mock.correct(
        labReportId,
        const EntryPatch(facts: [FactPatch('f1', value: '14,2')]),
      );

      expect(entry.correctionFor('facts.f1.value'), isNull);
      // The existing correction stays unchanged.
      expect(
        entry.correctionFor('facts.f2.value')?.correctedAt,
        DateTime.utc(2026, 11, 3, 19, 5, 31),
      );
    });

    test('example from 3.5: documentDate and f2 in one request', () async {
      final entry = await mock.correct(
        labReportId,
        EntryPatch(documentDate: Value(PlainDate(2026, 10, 15))),
      );
      expect(entry.documentDate, PlainDate(2026, 10, 15));
      final correction = entry.correctionFor('documentDate')!;
      expect(correction.recognized, '2026-10-14');
      expect(correction.corrected, '2026-10-15');
    });

    test('clear and restore the date', () async {
      var entry = await mock.correct(
        labReportId,
        const EntryPatch(documentDate: Value(null)),
      );
      expect(entry.documentDate, isNull);
      expect(entry.correctionFor('documentDate')?.recognized, '2026-10-14');
      expect(entry.correctionFor('documentDate')?.corrected, isNull);

      // An entry without a date is now at the end of the timeline.
      final list = await mock.listEntries(module: 'health');
      expect(list.last.id, labReportId);

      entry = await mock.correct(
        labReportId,
        EntryPatch(documentDate: Value(PlainDate(2026, 10, 14))),
      );
      expect(entry.documentDate, PlainDate(2026, 10, 14));
      expect(entry.correctionFor('documentDate'), isNull);
    });

    test('remove the unit', () async {
      final entry = await mock.correct(
        labReportId,
        const EntryPatch(facts: [FactPatch('f1', unit: Value(null))]),
      );
      expect(entry.facts[0].unit, isNull);
      expect(entry.facts[0].value, '14,2');
      final correction = entry.correctionFor('facts.f1.unit')!;
      expect(correction.recognized, 'g/dl');
      expect(correction.corrected, isNull);
    });

    test('add a unit where none was recognized', () async {
      final entry = await mock.correct(
        labReportId,
        const EntryPatch(facts: [FactPatch('f4', unit: Value('Typ'))]),
      );
      expect(entry.facts[3].unit, 'Typ');
      expect(entry.correctionFor('facts.f4.unit')?.recognized, isNull);
    });

    test('type and issuer', () async {
      final entry = await mock.correct(
        labReportId,
        const EntryPatch(type: 'doctor_letter', issuer: Value('Labor X')),
      );
      expect(entry.type, 'doctor_letter');
      expect(entry.issuer, 'Labor X');
      expect(entry.correctionFor('type')?.recognized, 'lab_report');
      expect(
        entry.correctionFor('issuer')?.recognized,
        'Labor Muster GmbH, Salzburg',
      );
    });

    test('add an issuer where none was recognized', () async {
      final entry = await mock.correct(
        otherId,
        const EntryPatch(issuer: Value('Impfstelle')),
      );
      expect(entry.issuer, 'Impfstelle');
      expect(entry.correctionFor('issuer')?.recognized, isNull);
    });
  });

  group('error cases', () {
    test('empty body', () async {
      await expectLater(
        mock.correctWithJson(labReportId, {}),
        throwsApiException(ErrorCode.invalidRequest, status: 400),
      );
    });

    test('date in the wrong format or not in the calendar', () async {
      await expectLater(
        mock.correctWithJson(labReportId, {'documentDate': '15.10.2026'}),
        throwsApiException(ErrorCode.invalidRequest, status: 400),
      );
      await expectLater(
        mock.correctWithJson(labReportId, {'documentDate': '2026-02-30'}),
        throwsApiException(ErrorCode.invalidRequest, status: 400),
      );
    });

    test('unknown type', () async {
      await expectLater(
        mock.correctWithJson(labReportId, {'type': 'kochrezept'}),
        throwsApiException(ErrorCode.invalidRequest, status: 400),
      );
    });

    test('fields that cannot be corrected', () async {
      for (final field in ['fullText', 'module', 'importedAt', 'file']) {
        await expectLater(
          mock.correctWithJson(labReportId, {field: 'x'}),
          throwsApiException(ErrorCode.fieldNotCorrectable, status: 400),
          reason: field,
        );
      }
    });

    test('unknown field', () async {
      await expectLater(
        mock.correctWithJson(labReportId, {'farbe': 'rot'}),
        throwsApiException(ErrorCode.invalidRequest, status: 400),
      );
    });

    test('fact without id', () async {
      await expectLater(
        mock.correctWithJson(labReportId, {
          'facts': [
            {'value': '1'},
          ],
        }),
        throwsApiException(ErrorCode.invalidRequest, status: 400),
      );
    });

    test('unknown fact: 404 FACT_NOT_FOUND with factId', () async {
      try {
        await mock.correct(
          labReportId,
          const EntryPatch(facts: [FactPatch('f9', value: '1')]),
        );
        fail('Fehler erwartet');
      } on ApiException catch (f) {
        expect(f.httpStatus, 404);
        expect(f.code, ErrorCode.factNotFound);
        expect(f.details['factId'], 'f9');
      }
    });

    test('if any part is invalid, nothing changes at all', () async {
      await expectLater(
        mock.correctWithJson(labReportId, {
          'documentDate': '2026-10-20',
          'facts': [
            {'id': 'f9', 'value': '1'},
          ],
        }),
        throwsApiException(ErrorCode.factNotFound),
      );
      final entry = await mock.getEntry(labReportId);
      expect(entry.documentDate, PlainDate(2026, 10, 14));
      expect(entry.corrections, hasLength(1));
    });

    test('invalid and unknown entry ID', () async {
      const patch = EntryPatch(type: 'prescription');
      await expectLater(
        mock.correct('123', patch),
        throwsApiException(ErrorCode.invalidId, status: 400),
      );
      await expectLater(
        mock.correct('ffffffffffffffffffffffff', patch),
        throwsApiException(ErrorCode.entryNotFound, status: 404),
      );
    });
  });
}
