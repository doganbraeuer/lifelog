import 'package:flutter_test/flutter_test.dart';
import 'package:lifelog/api/api_client.dart';
import 'package:lifelog/api/api_exception.dart';
import 'package:lifelog/model/plain_date.dart';

import '../helpers.dart';

void main() {
  group('entries', () {
    test('default: order as in the example from docs/API.md, 3.2', () async {
      final list = await fastMock().listEntries(module: 'health');
      expect(list.map((e) => e.id), [labReportId, doctorLetterId, otherId]);
      expect(list.last.documentDate, isNull);
      expect(list.last.issuer, isNull);
    });

    test('importedAt ascending', () async {
      final list = await fastMock().listEntries(
        sort: Sort.importedAt,
        order: SortOrder.asc,
        module: 'health',
      );
      expect(list.map((e) => e.id), [labReportId, doctorLetterId, otherId]);
    });

    test('documentDate ascending: without date at the end', () async {
      final list = await fastMock().listEntries(
        order: SortOrder.asc,
        module: 'health',
      );
      expect(list.map((e) => e.id), [doctorLetterId, labReportId, otherId]);
    });

    test('without parameters the defaults apply', () async {
      final list = await fastMock().listEntriesWithParameters({});
      expect(list.first.id, labReportId);
    });

    test('unknown sort: 400 INVALID_PARAMETER', () async {
      final mock = fastMock();
      await expectLater(
        mock.listEntriesWithParameters({'sort': 'issuer'}),
        throwsApiException(ErrorCode.invalidParameter, status: 400),
      );
      try {
        await mock.listEntriesWithParameters({'sort': 'issuer'});
      } on ApiException catch (f) {
        expect(f.details, {'parameter': 'sort', 'value': 'issuer'});
      }
    });

    test('unknown order: 400 INVALID_PARAMETER', () async {
      await expectLater(
        fastMock().listEntriesWithParameters({'order': 'seitwaerts'}),
        throwsApiException(ErrorCode.invalidParameter, status: 400),
      );
    });

    test('without entries an empty list', () async {
      expect(
        await fastMock(withSampleData: false).listEntries(module: 'health'),
        isEmpty,
      );
    });
  });

  group('getEntry', () {
    test('returns the full lab report', () async {
      final entry = await fastMock().getEntry(labReportId);
      expect(entry.type, 'lab_report');
      expect(entry.documentDate, PlainDate(2026, 10, 14));
      expect(entry.facts, hasLength(4));
      expect(entry.correctionFor('facts.f2.value')?.recognized, '6,3');
    });

    test('invalid ID: 400 INVALID_ID', () async {
      await expectLater(
        fastMock().getEntry('123'),
        throwsApiException(ErrorCode.invalidId, status: 400),
      );
      await expectLater(
        fastMock().getEntry('zzzzzzzzzzzzzzzzzzzzzzzz'),
        throwsApiException(ErrorCode.invalidId, status: 400),
      );
    });

    test('unknown ID: 404 ENTRY_NOT_FOUND', () async {
      await expectLater(
        fastMock().getEntry('ffffffffffffffffffffffff'),
        throwsApiException(ErrorCode.entryNotFound, status: 404),
      );
    });
  });

  group('getFile', () {
    test('lab report is a PDF', () async {
      final file = await fastMock().getFile(labReportId);
      expect(file.mimeType, 'application/pdf');
      expect(String.fromCharCodes(file.bytes.take(5)), '%PDF-');
      expect(
        String.fromCharCodes(file.bytes.skip(file.bytes.length - 6)),
        '%%EOF\n',
      );
    });

    test('doctor letter is a PNG', () async {
      final file = await fastMock().getFile(doctorLetterId);
      expect(file.mimeType, 'image/png');
      expect(file.bytes.take(4), [0x89, 0x50, 0x4E, 0x47]);
    });

    test('errors as for getEntry', () async {
      await expectLater(
        fastMock().getFile('123'),
        throwsApiException(ErrorCode.invalidId),
      );
      await expectLater(
        fastMock().getFile('ffffffffffffffffffffffff'),
        throwsApiException(ErrorCode.entryNotFound),
      );
    });
  });

  test('simulateError: 500 INTERNAL_ERROR', () async {
    final mock = fastMock()..simulateError = true;
    await expectLater(
      mock.listEntries(module: 'health'),
      throwsApiException(ErrorCode.internalError, status: 500),
    );
  });
}
