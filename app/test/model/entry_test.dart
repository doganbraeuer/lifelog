import 'dart:convert';

import 'package:flutter_test/flutter_test.dart';
import 'package:lifelog/model/plain_date.dart';
import 'package:lifelog/model/entry.dart';

/// Vollständiger Eintrag aus docs/API.md, Abschnitt 2.
const beispielJson = '''
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

Map<String, dynamic> beispiel() =>
    jsonDecode(beispielJson) as Map<String, dynamic>;

void main() {
  test('liest den vollständigen Eintrag', () {
    final eintrag = Eintrag.fromJson(beispiel());
    expect(eintrag.id, '6727a1f3c2b4e5d6f7a8b9c0');
    expect(eintrag.modul, 'health');
    expect(eintrag.dokumentdatum, Datum(2026, 10, 14));
    expect(eintrag.importdatum, DateTime.utc(2026, 11, 3, 18, 42, 7));
    expect(eintrag.importdatum.isUtc, isTrue);
    expect(eintrag.infos, hasLength(4));
    expect(eintrag.infos[2].wert, '<0,5');
    expect(eintrag.infos[3].einheit, isNull);
    expect(eintrag.datei.istPdf, isTrue);
    expect(eintrag.datei.groesse, 184320);
  });

  test('Korrekturen lassen sich pro Feld abfragen', () {
    final eintrag = Eintrag.fromJson(beispiel());
    expect(eintrag.korrektur('facts.f2.value')?.erkannt, '6,3');
    expect(eintrag.korrektur('documentDate'), isNull);
    expect(eintrag.infoKorrigiert('f2'), isTrue);
    expect(eintrag.infoKorrigiert('f1'), isFalse);
  });

  test('null für dokumentdatum und quelle', () {
    final json = beispiel()
      ..['documentDate'] = null
      ..['issuer'] = null;
    final kurz = EintragKurz.fromJson(json);
    expect(kurz.dokumentdatum, isNull);
    expect(kurz.quelle, isNull);
  });

  test('falsche Daten werfen FormatException', () {
    expect(
      () => Eintrag.fromJson(beispiel()..remove('id')),
      throwsFormatException,
    );
    expect(
      () => Eintrag.fromJson(beispiel()..['documentDate'] = '14.10.2026'),
      throwsFormatException,
    );
    expect(
      () => Eintrag.fromJson(beispiel()..['facts'] = 'keine Liste'),
      throwsFormatException,
    );
    expect(
      () => Eintrag.fromJson(beispiel()..['importedAt'] = 'gestern'),
      throwsFormatException,
    );
  });
}
