import 'package:flutter_test/flutter_test.dart';
import 'package:lifelog/api/api_exception.dart';
import 'package:lifelog/api/mock_api_client.dart';
import 'package:lifelog/model/plain_date.dart';
import 'package:lifelog/model/entry_patch.dart';

import '../helpers.dart';

void main() {
  late MockApiClient mock;
  final jetzt = DateTime.utc(2026, 11, 4, 9, 30, 0);

  setUp(() => mock = schnellerMock(clock: () => jetzt));

  group('Korrekturregeln aus docs/API.md, Abschnitt 2', () {
    test(
      'Regel 1 und 2: aktueller Wert im Feld, erkannt bleibt das Original',
      () async {
        // Ausgangslage: i2 wurde schon von 6,3 auf 6,8 korrigiert.
        final entry = await mock.correct(
          laborId,
          const EntryPatch(facts: [FactPatch('f2', value: '7,0')]),
        );

        expect(entry.facts[1].value, '7,0');
        final corrections = entry.corrections.where(
          (k) => k.field == 'facts.f2.value',
        );
        expect(corrections, hasLength(1));
        expect(corrections.single.recognized, '6,3');
        expect(corrections.single.corrected, '7,0');
        expect(corrections.single.correctedAt, jetzt);
      },
    );

    test(
      'Regel 3: zurück auf den erkannten Wert entfernt die Korrektur',
      () async {
        final entry = await mock.correct(
          laborId,
          const EntryPatch(facts: [FactPatch('f2', value: '6,3')]),
        );

        expect(entry.facts[1].value, '6,3');
        expect(entry.correctionFor('facts.f2.value'), isNull);
        expect(entry.isFactCorrected('f2'), isFalse);
      },
    );

    test('unveränderter Wert erzeugt keine Korrektur', () async {
      final entry = await mock.correct(
        laborId,
        const EntryPatch(facts: [FactPatch('f1', value: '14,2')]),
      );

      expect(entry.correctionFor('facts.f1.value'), isNull);
      // Die bestehende Korrektur bleibt unverändert.
      expect(
        entry.correctionFor('facts.f2.value')?.correctedAt,
        DateTime.utc(2026, 11, 3, 19, 5, 31),
      );
    });

    test('Beispiel aus 3.5: dokumentdatum und i2 in einer Anfrage', () async {
      final entry = await mock.correct(
        laborId,
        EntryPatch(documentDate: Value(PlainDate(2026, 10, 15))),
      );
      expect(entry.documentDate, PlainDate(2026, 10, 15));
      final correction = entry.correctionFor('documentDate')!;
      expect(correction.recognized, '2026-10-14');
      expect(correction.corrected, '2026-10-15');
    });

    test('Datum löschen und wiederherstellen', () async {
      var entry = await mock.correct(
        laborId,
        const EntryPatch(documentDate: Value(null)),
      );
      expect(entry.documentDate, isNull);
      expect(entry.correctionFor('documentDate')?.recognized, '2026-10-14');
      expect(entry.correctionFor('documentDate')?.corrected, isNull);

      // Ein Eintrag ohne Datum steht jetzt am Ende der Zeitleiste.
      final liste = await mock.listEntries(module: 'health');
      expect(liste.last.id, laborId);

      entry = await mock.correct(
        laborId,
        EntryPatch(documentDate: Value(PlainDate(2026, 10, 14))),
      );
      expect(entry.documentDate, PlainDate(2026, 10, 14));
      expect(entry.correctionFor('documentDate'), isNull);
    });

    test('Einheit entfernen', () async {
      final entry = await mock.correct(
        laborId,
        const EntryPatch(facts: [FactPatch('f1', unit: Value(null))]),
      );
      expect(entry.facts[0].unit, isNull);
      expect(entry.facts[0].value, '14,2');
      final correction = entry.correctionFor('facts.f1.unit')!;
      expect(correction.recognized, 'g/dl');
      expect(correction.corrected, isNull);
    });

    test('Einheit ergänzen, wo keine erkannt wurde', () async {
      final entry = await mock.correct(
        laborId,
        const EntryPatch(facts: [FactPatch('f4', unit: Value('Typ'))]),
      );
      expect(entry.facts[3].unit, 'Typ');
      expect(entry.correctionFor('facts.f4.unit')?.recognized, isNull);
    });

    test('typ und quelle', () async {
      final entry = await mock.correct(
        laborId,
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

    test('Quelle ergänzen, wo keine erkannt wurde', () async {
      final entry = await mock.correct(
        sonstigesId,
        const EntryPatch(issuer: Value('Impfstelle')),
      );
      expect(entry.issuer, 'Impfstelle');
      expect(entry.correctionFor('issuer')?.recognized, isNull);
    });
  });

  group('Fehlerfälle', () {
    test('leerer Body', () async {
      await expectLater(
        mock.correctWithJson(laborId, {}),
        wirftApiFehler(ErrorCode.invalidRequest, status: 400),
      );
    });

    test('Datum im falschen Format oder nicht im Kalender', () async {
      await expectLater(
        mock.correctWithJson(laborId, {'documentDate': '15.10.2026'}),
        wirftApiFehler(ErrorCode.invalidRequest, status: 400),
      );
      await expectLater(
        mock.correctWithJson(laborId, {'documentDate': '2026-02-30'}),
        wirftApiFehler(ErrorCode.invalidRequest, status: 400),
      );
    });

    test('unbekannter typ', () async {
      await expectLater(
        mock.correctWithJson(laborId, {'type': 'kochrezept'}),
        wirftApiFehler(ErrorCode.invalidRequest, status: 400),
      );
    });

    test('nicht korrigierbare Felder', () async {
      for (final field in ['fullText', 'module', 'importedAt', 'file']) {
        await expectLater(
          mock.correctWithJson(laborId, {field: 'x'}),
          wirftApiFehler(ErrorCode.fieldNotCorrectable, status: 400),
          reason: field,
        );
      }
    });

    test('unbekanntes Feld', () async {
      await expectLater(
        mock.correctWithJson(laborId, {'farbe': 'rot'}),
        wirftApiFehler(ErrorCode.invalidRequest, status: 400),
      );
    });

    test('Info ohne id', () async {
      await expectLater(
        mock.correctWithJson(laborId, {
          'facts': [
            {'value': '1'},
          ],
        }),
        wirftApiFehler(ErrorCode.invalidRequest, status: 400),
      );
    });

    test('unbekannte Info: 404 FACT_NOT_FOUND mit infoId', () async {
      try {
        await mock.correct(
          laborId,
          const EntryPatch(facts: [FactPatch('f9', value: '1')]),
        );
        fail('Fehler erwartet');
      } on ApiException catch (f) {
        expect(f.httpStatus, 404);
        expect(f.code, ErrorCode.factNotFound);
        expect(f.details['factId'], 'f9');
      }
    });

    test('ist ein Teil ungültig, ändert sich gar nichts', () async {
      await expectLater(
        mock.correctWithJson(laborId, {
          'documentDate': '2026-10-20',
          'facts': [
            {'id': 'f9', 'value': '1'},
          ],
        }),
        wirftApiFehler(ErrorCode.factNotFound),
      );
      final entry = await mock.getEntry(laborId);
      expect(entry.documentDate, PlainDate(2026, 10, 14));
      expect(entry.corrections, hasLength(1));
    });

    test('ungültige und unbekannte Eintrag-ID', () async {
      const aenderung = EntryPatch(type: 'prescription');
      await expectLater(
        mock.correct('123', aenderung),
        wirftApiFehler(ErrorCode.invalidId, status: 400),
      );
      await expectLater(
        mock.correct('ffffffffffffffffffffffff', aenderung),
        wirftApiFehler(ErrorCode.entryNotFound, status: 404),
      );
    });
  });
}
