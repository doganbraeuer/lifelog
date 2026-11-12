import 'package:flutter_test/flutter_test.dart';
import 'package:lifelog/api/mock/mock_verification.dart';
import 'package:lifelog/model/verification.dart';

import '../helpers.dart';

/// An entry as JSON with the given values; corrections as field paths.
Map<String, dynamic> entry({
  String? documentDate,
  String? issuer,
  List<Map<String, dynamic>> facts = const [],
  List<String> corrected = const [],
}) => {
  'documentDate': documentDate,
  'issuer': issuer,
  'facts': facts,
  'corrections': [
    for (final field in corrected)
      {
        'field': field,
        'recognized': 'x',
        'corrected': 'y',
        'correctedAt': '2026-11-04T09:30:00Z',
      },
  ],
};

Map<String, dynamic> fact(String value, [String? unit, String name = 'Wert']) =>
    {'id': 'f1', 'name': name, 'value': value, 'unit': ?unit};

/// Checks [json] against one page read by [source].
Verification check(
  Map<String, dynamic> json,
  List<String> lines, {
  VerificationSource source = VerificationSource.pdfText,
}) => Verification.fromJson(
  checkEntry(
    json,
    MockOriginal.page(source, lines),
    checkedAt: '2026-11-04T09:30:00Z',
  ),
);

FieldCheckStatus factStatus(Map<String, dynamic> f, List<String> lines) =>
    check(entry(facts: [f]), lines).checkFor('facts.f1')!.status;

const found = FieldCheckStatus.confirmed;
const notFound = FieldCheckStatus.unconfirmed;

void main() {
  group('comparison (docs/API.md, verification)', () {
    test('decimal comma = point, case and whitespace do not count', () {
      expect(factStatus(fact('14,2', 'g/dl'), ['Hb 14.2 G/DL']), found);
      expect(factStatus(fact('14,2', 'g/dl'), ['Hb 14,2g/dl']), found);
      expect(factStatus(fact('64', '/min'), ['Puls 64/min']), found);
      expect(
        factStatus(fact('Blutbild klein'), ['Art: Blutbild  klein']),
        found,
      );
    });

    test('only whole words: 4 is not found inside 14 or 14,2', () {
      expect(factStatus(fact('4', 'g'), ['Wert 14 g']), notFound);
      expect(factStatus(fact('2', 'g/dl'), ['Wert 14,2 g/dl']), notFound);
      expect(factStatus(fact('6', '/min'), ['Puls 64/min']), notFound);
    });

    test('a fact with a unit needs value and unit in the same line', () {
      expect(factStatus(fact('72', '/min'), ['Puls 72/mln']), notFound);
      expect(factStatus(fact('72', '/min'), ['Puls 72', '/min']), notFound);
      // The unit has to come after the value.
      expect(factStatus(fact('72', '/min'), ['/min Puls 72']), notFound);
    });

    test('a short value without unit only in the line of the name', () {
      expect(factStatus(fact('3', null, 'Packungen'), ['Packungen: 3']), found);
      expect(
        factStatus(fact('3', null, 'Packungen'), ['Packungen: 2', 'Seite 3']),
        notFound,
      );
      // Longer values count anywhere.
      expect(
        factStatus(fact('1-0-1', null, 'Einnahme'), ['Abends 1-0-1']),
        found,
      );
    });

    test('the date in common German forms', () {
      for (final line in [
        'vom 14.10.2026',
        'vom 14.10.26',
        'am 2026-10-14',
        'am 14/10/2026',
        'Salzburg, 14. Oktober 2026',
      ]) {
        expect(
          check(entry(documentDate: '2026-10-14'), [
            line,
          ]).checkFor('documentDate')!.status,
          found,
          reason: line,
        );
      }
      expect(
        check(entry(documentDate: '2026-01-05'), [
          '5. Jänner 2026',
        ]).checkFor('documentDate')!.status,
        found,
      );
      expect(
        check(entry(documentDate: '2026-09-02'), [
          'vom 2.9.2026',
        ]).checkFor('documentDate')!.status,
        found,
      );
      // 14.10.26 is not found inside 14.10.2026's neighbour 14.10.262.
      expect(
        check(entry(documentDate: '2026-10-14'), [
          'Nr. 14.10.262',
        ]).checkFor('documentDate')!.status,
        notFound,
      );
    });

    test('issuer as a whole or each comma-separated part', () {
      final whole = check(entry(issuer: 'Labor Muster GmbH, Salzburg'), [
        'Labor Muster GmbH, Salzburg',
      ]).checkFor('issuer')!;
      expect(whole.status, found);
      expect(whole.locations, hasLength(1));

      final parts = check(entry(issuer: 'Labor Muster GmbH, Salzburg'), [
        'Labor Muster GmbH',
        'Musterstraße 1, 5020 Salzburg',
      ]).checkFor('issuer')!;
      expect(parts.status, found);
      expect(parts.locations, hasLength(2));

      expect(
        check(entry(issuer: 'Labor Muster GmbH, Linz'), [
          'Labor Muster GmbH',
          'Salzburg',
        ]).checkFor('issuer')!.status,
        notFound,
      );
    });
  });

  group('result', () {
    test('counts, match percent and fields in the order of the contract', () {
      final v = check(
        entry(
          documentDate: '2026-10-14',
          issuer: 'Labor Muster GmbH',
          facts: [
            {'id': 'f1', 'name': 'Hb', 'value': '14,2', 'unit': 'g/dl'},
            {'id': 'f2', 'name': 'CRP', 'value': '<0,5', 'unit': 'mg/dl'},
          ],
          corrected: ['facts.f1.value'],
        ),
        ['Labor Muster GmbH', 'vom 14.10.2026', 'Hb 14,2 g/dl'],
      );
      expect(v.status, VerificationStatus.checked);
      expect(v.reason, isNull);
      expect(v.source, VerificationSource.pdfText);
      expect(v.engine, 'pdftotext 26.11.0');
      expect(v.checkedAt, DateTime.utc(2026, 11, 4, 9, 30));
      expect(v.fields.map((f) => f.field), [
        'documentDate',
        'issuer',
        'facts.f1',
        'facts.f2',
      ]);
      expect(
        v.checkFor('facts.f1')!.status,
        FieldCheckStatus.correctedConfirmed,
      );
      expect(v.checkFor('facts.f2')!.status, FieldCheckStatus.unconfirmed);
      expect(v.checkFor('facts.f2')!.source, isNull);
      expect(v.checkFor('facts.f2')!.locations, isEmpty);
      expect(v.confirmed, 3);
      expect(v.corrected, 1);
      expect(v.total, 4);
      expect(v.matchPercent, 75);
    });

    test('a correction whose value is not found: corrected_unconfirmed', () {
      final v = check(
        entry(
          facts: [fact('3', null, 'Packungen')],
          corrected: ['facts.f1.unit'],
        ),
        ['Packungen: 2'],
      );
      expect(
        v.checkFor('facts.f1')!.status,
        FieldCheckStatus.correctedUnconfirmed,
      );
      expect(v.corrected, 1);
      expect(v.confirmed, 0);
      expect(v.matchPercent, 0);
    });

    test('null fields are not checked; total 0 gives matchPercent null', () {
      final v = check(entry(), ['irgendein Text']);
      expect(v.total, 0);
      expect(v.fields, isEmpty);
      expect(v.matchPercent, isNull);
      expect(v.isChecked, isTrue);
    });

    test('matchPercent is rounded to a whole number', () {
      final v = check(
        entry(
          facts: [
            {'id': 'f1', 'name': 'A', 'value': 'eins'},
            {'id': 'f2', 'name': 'B', 'value': 'zwei'},
            {'id': 'f3', 'name': 'C', 'value': 'drei'},
          ],
        ),
        ['eins zwei'],
      );
      expect(v.matchPercent, 67);
    });

    test('OCR: confidence between 80 and 96, only for ocr', () {
      final ocr = check(entry(facts: [fact('78', 'kg', 'Gewicht')]), [
        'Gewicht 78 kg',
      ], source: VerificationSource.ocr);
      final field = ocr.checkFor('facts.f1')!;
      expect(field.source, VerificationSource.ocr);
      expect(field.confidence, inInclusiveRange(80, 96));
      expect(ocr.engine, 'tesseract 5.5.4 deu');

      final pdf = check(entry(facts: [fact('78', 'kg', 'Gewicht')]), [
        'Gewicht 78 kg',
      ]);
      expect(pdf.checkFor('facts.f1')!.confidence, isNull);
    });

    test('mixed PDF: source per field from its page, both engines', () {
      final v = Verification.fromJson(
        checkEntry(
          entry(
            facts: [
              {'id': 'f1', 'name': 'Ramipril', 'value': '5', 'unit': 'mg'},
              {'id': 'f2', 'name': 'Metformin', 'value': '500', 'unit': 'mg'},
            ],
          ),
          const MockOriginal([
            MockPage(VerificationSource.pdfText, ['Ramipril 5 mg']),
            MockPage(VerificationSource.ocr, ['Metformin 500 mg']),
          ]),
          checkedAt: '2026-11-04T09:30:00Z',
        ),
      );
      expect(v.source, VerificationSource.mixed);
      expect(v.engine, 'pdftotext 26.11.0, tesseract 5.5.4 deu');
      expect(v.checkFor('facts.f1')!.source, VerificationSource.pdfText);
      expect(v.checkFor('facts.f2')!.source, VerificationSource.ocr);
      final location = v.checkFor('facts.f2')!.locations.single as PageLocation;
      expect(location.page, 2);
    });

    test('text import: no engine, ranges in code points of fullText', () {
      const fullText = 'Größe 🙂 notiert\nGewicht 81 kg';
      final v = Verification.fromJson(
        checkEntry(
          entry(facts: [fact('81', 'kg', 'Gewicht')]),
          MockOriginal.text(fullText),
          checkedAt: '2026-11-04T09:30:00Z',
        ),
      );
      expect(v.source, VerificationSource.text);
      expect(v.engine, isNull);
      final location = v.checkFor('facts.f1')!.locations.single as TextLocation;
      expect(location.textIn(fullText), '81 kg');
      // 15 code points in the first line (the emoji counts once), one line
      // break, then „Gewicht “.
      expect(location.start, 24);
    });

    test('rectangles are fractions of the page inside the page', () {
      final v = check(entry(facts: [fact('14,2', 'g/dl', 'Hb')]), [
        '(Mock-Datei)',
        '',
        'Hb 14,2 g/dl',
      ]);
      final r = v.checkFor('facts.f1')!.locations.single as PageLocation;
      expect(r.page, 1);
      for (final value in [r.x, r.y, r.width, r.height, r.x + r.width]) {
        expect(value, inInclusiveRange(0, 1));
      }
      // Third line, after "Hb ": further down and right than the first.
      final first =
          check(entry(facts: [fact('Mock', null, 'M')]), [
                'Mock',
              ]).checkFor('facts.f1')!.locations.single
              as PageLocation;
      expect(r.y, greaterThan(first.y));
      expect(r.x, greaterThan(first.x));
    });

    test('at most three locations, the line of the name first', () {
      final v = check(entry(facts: [fact('12', 'mg', 'Dosis')]), [
        'A 12 mg',
        'B 12 mg',
        'C 12 mg',
        'Dosis 12 mg',
      ]);
      final locations = v.checkFor('facts.f1')!.locations.cast<PageLocation>();
      expect(locations, hasLength(3));
      // Line 4 (index 3) comes first.
      expect(locations.first.y, greaterThan(locations[1].y));
    });
  });

  test('unchecked has the shape of the contract', () {
    final v = Verification.fromJson(
      uncheckedVerification(VerificationReason.tooManyPages),
    );
    expect(v.isChecked, isFalse);
    expect(v.reason, VerificationReason.tooManyPages);
    expect(v.fields, isEmpty);
    expect(v.matchPercent, isNull);
  });

  group('sample data', () {
    test('every case of the verification is there', () async {
      final mock = fastMock();
      Future<Verification> of(String id) async =>
          (await mock.getEntry(id)).verification;

      final lab = await of(labReportId);
      expect(lab.source, VerificationSource.pdfText);
      // As in the example of docs/API.md, section 2.
      expect(
        (lab.confirmed, lab.corrected, lab.total, lab.matchPercent),
        (5, 1, 6, 83),
      );
      expect(
        lab.checkFor('facts.f2')!.status,
        FieldCheckStatus.correctedConfirmed,
      );
      expect(lab.checkFor('facts.f3')!.status, FieldCheckStatus.unconfirmed);
      expect(lab.checkFor('issuer')!.locations, hasLength(2));
      expect(lab.checkedAt, DateTime.utc(2026, 11, 3, 19, 5, 31));

      final letter = await of(doctorLetterId);
      expect(letter.source, VerificationSource.ocr);
      expect(letter.checkFor('facts.f3')!.status, FieldCheckStatus.unconfirmed);
      expect(letter.checkFor('facts.f2')!.confidence, isNotNull);

      final note = await of(noteId);
      expect(note.source, VerificationSource.text);
      expect(note.checkFor('facts.f1')!.locations.single, isA<TextLocation>());
      expect(note.checkFor('facts.f3')!.status, FieldCheckStatus.unconfirmed);

      final prescription = await of(prescriptionId);
      expect(prescription.source, VerificationSource.mixed);
      expect(
        prescription.checkFor('facts.f2')!.status,
        FieldCheckStatus.correctedUnconfirmed,
      );
      expect(prescription.checkFor('facts.f3')!.source, VerificationSource.ocr);

      expect((await of(otherId)).reason, VerificationReason.noText);
      expect((await of(xRayId)).reason, VerificationReason.ocrUnavailable);
      expect((await of(oldLabId)).reason, VerificationReason.notRun);
    });

    test('the text-import locations point at the values in fullText', () async {
      final note = await fastMock().getEntry(noteId);
      String textOf(String field) =>
          (note.verification.checkFor(field)!.locations.first as TextLocation)
              .textIn(note.fullText);
      expect(textOf('documentDate'), '05.11.2026');
      expect(textOf('issuer'), 'Blutdruckmessung zu Hause');
      expect(textOf('facts.f1'), '135/85 mmHg');
      expect(textOf('facts.f2'), '64/min');
    });
  });

  group('new entries are checked', () {
    test('PDF upload: text layer', () async {
      final mock = fastMock();
      final file = testPdf('neu.pdf');
      final entry = await mock.upload(file.bytes, file.name, module: 'health');
      final v = entry.verification;
      expect(v.isChecked, isTrue);
      expect(v.source, VerificationSource.pdfText);
      expect(v.checkedAt, entry.importedAt);
      expect(v.checkFor('issuer')!.status, FieldCheckStatus.confirmed);
      expect(v.checkFor('facts.f1')!.status, FieldCheckStatus.confirmed);
      // Same result on the next GET.
      expect((await mock.getEntry(entry.id)).verification.total, v.total);
    });

    test('text import: the entered text', () async {
      const text = 'Hausarztpraxis Beispiel\nBefund vom 21.10.2026';
      final entry = await fastMock().importText(text, module: 'health');
      final v = entry.verification;
      expect(v.source, VerificationSource.text);
      expect(v.engine, isNull);
      final date = v.checkFor('documentDate')!;
      expect(date.status, FieldCheckStatus.confirmed);
      expect(
        (date.locations.single as TextLocation).textIn(text),
        '21.10.2026',
      );
    });

    test('a deleted entry leaves nothing behind for a new upload', () async {
      final mock = fastMock();
      await mock.deleteEntry(noteId);
      const text =
          'Blutdruckmessung zu Hause\n'
          'Messung am 05.11.2026 morgens\n'
          'Blutdruck 135/85 mmHg\n'
          'Puls 64/min\n'
          'Gewicht etwa 81 kg';
      final entry = await mock.importText(text, module: 'health');
      expect(entry.verification.source, VerificationSource.text);
    });
  });
}
