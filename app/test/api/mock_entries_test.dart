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
        final liste = await schnellerMock().listEntries(module: 'health');
        expect(liste.map((e) => e.id), [laborId, arztbriefId, sonstigesId]);
        expect(liste.last.documentDate, isNull);
        expect(liste.last.issuer, isNull);
      },
    );

    test('importdatum aufsteigend', () async {
      final liste = await schnellerMock().listEntries(
        sort: Sort.importedAt,
        order: SortOrder.asc,
        module: 'health',
      );
      expect(liste.map((e) => e.id), [laborId, arztbriefId, sonstigesId]);
    });

    test('dokumentdatum aufsteigend: ohne Datum am Ende', () async {
      final liste = await schnellerMock().listEntries(
        order: SortOrder.asc,
        module: 'health',
      );
      expect(liste.map((e) => e.id), [arztbriefId, laborId, sonstigesId]);
    });

    test('ohne Parameter gelten die Standardwerte', () async {
      final liste = await schnellerMock().listEntriesWithParameters({});
      expect(liste.first.id, laborId);
    });

    test('unbekannte Sortierung: 400 INVALID_PARAMETER', () async {
      final mock = schnellerMock();
      await expectLater(
        mock.listEntriesWithParameters({'sort': 'issuer'}),
        wirftApiFehler(ErrorCode.invalidParameter, status: 400),
      );
      try {
        await mock.listEntriesWithParameters({'sort': 'issuer'});
      } on ApiException catch (f) {
        expect(f.details, {'parameter': 'sort', 'value': 'issuer'});
      }
    });

    test('unbekannte Richtung: 400 INVALID_PARAMETER', () async {
      await expectLater(
        schnellerMock().listEntriesWithParameters({'order': 'seitwaerts'}),
        wirftApiFehler(ErrorCode.invalidParameter, status: 400),
      );
    });

    test('ohne Einträge eine leere Liste', () async {
      expect(
        await schnellerMock(withSampleData: false)
            .listEntries(module: 'health'),
        isEmpty,
      );
    });
  });

  group('eintrag', () {
    test('liefert den vollständigen Laborbefund', () async {
      final entry = await schnellerMock().getEntry(laborId);
      expect(entry.type, 'lab_report');
      expect(entry.documentDate, PlainDate(2026, 10, 14));
      expect(entry.facts, hasLength(4));
      expect(entry.correctionFor('facts.f2.value')?.recognized, '6,3');
    });

    test('ungültige ID: 400 INVALID_ID', () async {
      await expectLater(
        schnellerMock().getEntry('123'),
        wirftApiFehler(ErrorCode.invalidId, status: 400),
      );
      await expectLater(
        schnellerMock().getEntry('zzzzzzzzzzzzzzzzzzzzzzzz'),
        wirftApiFehler(ErrorCode.invalidId, status: 400),
      );
    });

    test('unbekannte ID: 404 ENTRY_NOT_FOUND', () async {
      await expectLater(
        schnellerMock().getEntry('ffffffffffffffffffffffff'),
        wirftApiFehler(ErrorCode.entryNotFound, status: 404),
      );
    });
  });

  group('file', () {
    test('Laborbefund ist ein PDF', () async {
      final file = await schnellerMock().getFile(laborId);
      expect(file.mimeType, 'application/pdf');
      expect(String.fromCharCodes(file.bytes.take(5)), '%PDF-');
      expect(
        String.fromCharCodes(file.bytes.skip(file.bytes.length - 6)),
        '%%EOF\n',
      );
    });

    test('Arztbrief ist ein PNG', () async {
      final file = await schnellerMock().getFile(arztbriefId);
      expect(file.mimeType, 'image/png');
      expect(file.bytes.take(4), [0x89, 0x50, 0x4E, 0x47]);
    });

    test('Fehler wie bei eintrag', () async {
      await expectLater(
        schnellerMock().getFile('123'),
        wirftApiFehler(ErrorCode.invalidId),
      );
      await expectLater(
        schnellerMock().getFile('ffffffffffffffffffffffff'),
        wirftApiFehler(ErrorCode.entryNotFound),
      );
    });
  });

  test('simulierterFehler: 500 INTERNAL_ERROR', () async {
    final mock = schnellerMock()..simulateError = true;
    await expectLater(
      mock.listEntries(module: 'health'),
      wirftApiFehler(ErrorCode.internalError, status: 500),
    );
  });
}
