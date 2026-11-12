import 'dart:convert';

import 'package:flutter_test/flutter_test.dart';
import 'package:lifelog/model/plain_date.dart';
import 'package:lifelog/model/entry.dart';

/// Full entry from docs/API.md, section 2.
const sampleJson = '''
{
  "id": "6727a1f3c2b4e5d6f7a8b9c0",
  "module": "health",
  "type": "lab_report",
  "documentDate": "2026-10-14",
  "importedAt": "2026-11-03T18:42:07Z",
  "issuer": "Labor Muster GmbH, Salzburg",
  "fullText": "Labor Muster GmbH\\nHämoglobin 14,2 g/dl",
  "facts": [
    { "id": "f1", "name": "Hämoglobin", "value": "14,2", "unit": "g/dl" },
    { "id": "f2", "name": "Leukozyten", "value": "6,8", "unit": "G/l" },
    { "id": "f3", "name": "CRP", "value": "<0,5", "unit": "mg/dl" },
    { "id": "f4", "name": "Befundart", "value": "Blutbild klein" }
  ],
  "file": {
    "id": "6727a1f3c2b4e5d6f7a8b9c1",
    "fileName": "befund_oktober.pdf",
    "mimeType": "application/pdf",
    "size": 184320,
    "sha256": "3f8a1c0e9b7d2f4a6c5e8b1d0a9f7e3c2b4d6a8f0e1c3b5d7a9f2e4c6b8d0a1f"
  },
  "corrections": [
    {
      "field": "facts.f2.value",
      "recognized": "6,3",
      "corrected": "6,8",
      "correctedAt": "2026-11-03T19:05:31Z"
    }
  ]
}
''';

Map<String, dynamic> sample() => jsonDecode(sampleJson) as Map<String, dynamic>;

void main() {
  test('reads the full entry', () {
    final entry = Entry.fromJson(sample());
    expect(entry.id, '6727a1f3c2b4e5d6f7a8b9c0');
    expect(entry.module, 'health');
    expect(entry.documentDate, PlainDate(2026, 10, 14));
    expect(entry.importedAt, DateTime.utc(2026, 11, 3, 18, 42, 7));
    expect(entry.importedAt.isUtc, isTrue);
    expect(entry.facts, hasLength(4));
    expect(entry.facts[2].value, '<0,5');
    expect(entry.facts[3].unit, isNull);
    expect(entry.file.isPdf, isTrue);
    expect(entry.file.size, 184320);
  });

  test('corrections can be looked up per field', () {
    final entry = Entry.fromJson(sample());
    expect(entry.correctionFor('facts.f2.value')?.recognized, '6,3');
    expect(entry.correctionFor('documentDate'), isNull);
    expect(entry.isFactCorrected('f2'), isTrue);
    expect(entry.isFactCorrected('f1'), isFalse);
  });

  test('null for documentDate and issuer', () {
    final json = sample()
      ..['documentDate'] = null
      ..['issuer'] = null;
    final summary = EntrySummary.fromJson(json);
    expect(summary.documentDate, isNull);
    expect(summary.issuer, isNull);
  });

  test('wrong data throws FormatException', () {
    expect(() => Entry.fromJson(sample()..remove('id')), throwsFormatException);
    expect(
      () => Entry.fromJson(sample()..['documentDate'] = '14.10.2026'),
      throwsFormatException,
    );
    expect(
      () => Entry.fromJson(sample()..['facts'] = 'keine Liste'),
      throwsFormatException,
    );
    expect(
      () => Entry.fromJson(sample()..['importedAt'] = 'gestern'),
      throwsFormatException,
    );
  });
}
