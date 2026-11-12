import 'dart:convert';

import 'package:flutter_test/flutter_test.dart';
import 'package:lifelog/model/entry.dart';
import 'package:lifelog/model/verification.dart';

import 'entry_test.dart' show sample;

/// `verification` of the full entry in docs/API.md, section 2.
const apiExample = '''
{
  "status": "checked",
  "reason": null,
  "source": "pdf_text",
  "engine": "pdftotext 26.11.0",
  "checkedAt": "2026-11-03T19:05:31Z",
  "confirmed": 5,
  "corrected": 1,
  "total": 6,
  "matchPercent": 83,
  "fields": [
    { "field": "documentDate", "status": "confirmed", "source": "pdf_text", "confidence": null,
      "locations": [{ "page": 1, "x": 0.2803, "y": 0.1821, "width": 0.1105, "height": 0.0142 }] },
    { "field": "issuer", "status": "confirmed", "source": "pdf_text", "confidence": null,
      "locations": [{ "page": 1, "x": 0.1210, "y": 0.0702, "width": 0.2104, "height": 0.0160 },
                    { "page": 1, "x": 0.3391, "y": 0.0911, "width": 0.0802, "height": 0.0139 }] },
    { "field": "facts.f1", "status": "confirmed", "source": "pdf_text", "confidence": null,
      "locations": [{ "page": 1, "x": 0.3012, "y": 0.2950, "width": 0.1120, "height": 0.0145 }] },
    { "field": "facts.f2", "status": "corrected_confirmed", "source": "pdf_text", "confidence": null,
      "locations": [{ "page": 1, "x": 0.3018, "y": 0.3157, "width": 0.0988, "height": 0.0143 }] },
    { "field": "facts.f3", "status": "unconfirmed", "source": null, "confidence": null, "locations": [] },
    { "field": "facts.f4", "status": "confirmed", "source": "pdf_text", "confidence": null,
      "locations": [{ "page": 1, "x": 0.2590, "y": 0.3571, "width": 0.1702, "height": 0.0151 }] }
  ]
}
''';

Map<String, dynamic> example() =>
    jsonDecode(apiExample) as Map<String, dynamic>;

Map<String, dynamic> unchecked(String reason) => {
  'status': 'unchecked',
  'reason': reason,
  'source': null,
  'engine': null,
  'checkedAt': null,
  'confirmed': 0,
  'corrected': 0,
  'total': 0,
  'matchPercent': null,
  'fields': <dynamic>[],
};

void main() {
  test('reads the example from docs/API.md completely', () {
    final v = Verification.fromJson(example());
    expect(v.status, VerificationStatus.checked);
    expect(v.isChecked, isTrue);
    expect(v.reason, isNull);
    expect(v.source, VerificationSource.pdfText);
    expect(v.engine, 'pdftotext 26.11.0');
    expect(v.checkedAt, DateTime.utc(2026, 11, 3, 19, 5, 31));
    expect(v.confirmed, 5);
    expect(v.corrected, 1);
    expect(v.total, 6);
    expect(v.matchPercent, 83);
    expect(v.fields.map((f) => f.field), [
      'documentDate',
      'issuer',
      'facts.f1',
      'facts.f2',
      'facts.f3',
      'facts.f4',
    ]);

    final issuer = v.checkFor('issuer')!;
    expect(issuer.status, FieldCheckStatus.confirmed);
    expect(issuer.source, VerificationSource.pdfText);
    expect(issuer.confidence, isNull);
    expect(issuer.locations, hasLength(2));
    final second = issuer.locations[1] as PageLocation;
    expect(second.page, 1);
    expect(second.x, 0.3391);
    expect(second.y, 0.0911);
    expect(second.width, 0.0802);
    expect(second.height, 0.0139);

    expect(v.checkFor('facts.f2')!.status, FieldCheckStatus.correctedConfirmed);
    final crp = v.checkFor('facts.f3')!;
    expect(crp.status, FieldCheckStatus.unconfirmed);
    expect(crp.source, isNull);
    expect(crp.locations, isEmpty);
    expect(v.checkFor('facts.f9'), isNull);
  });

  test('a field confirmed by OCR carries its confidence', () {
    final check = FieldCheck.fromJson(
      jsonDecode('''
        { "field": "facts.f1", "status": "confirmed", "source": "ocr", "confidence": 93,
          "locations": [{ "page": 1, "x": 0, "y": 1, "width": 0.5, "height": 0.1 }] }
      ''') as Map<String, dynamic>,
    );
    expect(check.source, VerificationSource.ocr);
    expect(check.confidence, 93.0);
    // Whole numbers are allowed for fractions too (0 and 1 are edges).
    final location = check.locations.single as PageLocation;
    expect(location.x, 0.0);
    expect(location.y, 1.0);
  });

  test('a text import has ranges in code points of fullText', () {
    const fullText = 'Größe 182 cm\nGewicht 81 kg';
    final check = FieldCheck.fromJson({
      'field': 'facts.f1',
      'status': 'confirmed',
      'source': 'text',
      'confidence': null,
      'locations': [
        {'start': 6, 'end': 12},
      ],
    });
    final location = check.locations.single as TextLocation;
    expect(location.start, 6);
    expect(location.end, 12);
    expect(location.textIn(fullText), '182 cm');
    // Code points, not UTF-16 units: an emoji counts as one.
    expect(const TextLocation(start: 2, end: 4).textIn('a🙂bc'), 'bc');
  });

  test('every status of a field', () {
    for (final (code, status) in [
      ('confirmed', FieldCheckStatus.confirmed),
      ('unconfirmed', FieldCheckStatus.unconfirmed),
      ('corrected_confirmed', FieldCheckStatus.correctedConfirmed),
      ('corrected_unconfirmed', FieldCheckStatus.correctedUnconfirmed),
    ]) {
      expect(FieldCheckStatus.parse(code), status, reason: code);
    }
    expect(FieldCheckStatus.confirmed.isFound, isTrue);
    expect(FieldCheckStatus.correctedConfirmed.isFound, isTrue);
    expect(FieldCheckStatus.unconfirmed.isNotFound, isTrue);
    expect(FieldCheckStatus.correctedUnconfirmed.isNotFound, isTrue);
    expect(FieldCheckStatus.unknown.isFound, isFalse);
    expect(FieldCheckStatus.unknown.isNotFound, isFalse);
  });

  test('unchecked with every reason from docs/API.md', () {
    for (final reason in VerificationReason.values) {
      if (reason == VerificationReason.unknown) continue;
      final v = Verification.fromJson(unchecked(reason.code));
      expect(v.isChecked, isFalse);
      expect(v.reason, reason);
      expect(v.source, isNull);
      expect(v.checkedAt, isNull);
      expect(v.matchPercent, isNull);
      expect(v.fields, isEmpty);
    }
  });

  test('mixed source and engine of both programs', () {
    final json = example()
      ..['source'] = 'mixed'
      ..['engine'] = 'pdftotext 26.11.0, tesseract 5.5.4 deu';
    final v = Verification.fromJson(json);
    expect(v.source, VerificationSource.mixed);
    expect(v.engine, 'pdftotext 26.11.0, tesseract 5.5.4 deu');
  });

  test('unknown values from a newer backend do not break the entry', () {
    final v = Verification.fromJson(
      unchecked('disk_full')..['source'] = 'paddle',
    );
    expect(v.reason, VerificationReason.unknown);
    expect(v.source, VerificationSource.unknown);
    final field = FieldCheck.fromJson({
      'field': 'facts.f1',
      'status': 'value_confirmed_unit_uncertain',
      'source': null,
      'confidence': null,
      'locations': <dynamic>[],
    });
    expect(field.status, FieldCheckStatus.unknown);
    // An unknown status counts as not checked.
    expect(
      Verification.fromJson(example()..['status'] = 'pending').isChecked,
      isFalse,
    );
  });

  test('wrong types throw FormatException', () {
    expect(
      () => Verification.fromJson(example()..['total'] = '6'),
      throwsFormatException,
    );
    expect(
      () => Verification.fromJson(example()..['matchPercent'] = 83.4),
      throwsFormatException,
    );
    expect(
      () => Verification.fromJson(example()..['fields'] = null),
      throwsFormatException,
    );
    expect(
      () => Verification.fromJson(example()..['checkedAt'] = 'gestern'),
      throwsFormatException,
    );
    expect(
      () => Location.fromJson({
        'page': 1,
        'x': '0.1',
        'y': 0,
        'width': 0,
        'height': 0,
      }),
      throwsFormatException,
    );
    expect(
      () => Location.fromJson({'start': 1, 'end': null}),
      throwsFormatException,
    );
  });

  group('in the entry', () {
    test('is read with the full entry', () {
      final entry = Entry.fromJson(sample()..['verification'] = example());
      expect(entry.verification.isChecked, isTrue);
      expect(entry.verification.total, 6);
    });

    test('an entry without verification counts as not checked', () {
      final json = sample()..remove('verification');
      final entry = Entry.fromJson(json);
      expect(entry.verification.isChecked, isFalse);
      expect(entry.verification.reason, VerificationReason.notRun);
      expect(entry.verification.fields, isEmpty);

      final withNull = Entry.fromJson(sample()..['verification'] = null);
      expect(withNull.verification.reason, VerificationReason.notRun);
    });

    test('verification that is not an object throws', () {
      expect(
        () => Entry.fromJson(sample()..['verification'] = 'checked'),
        throwsFormatException,
      );
    });
  });
}
