import 'dart:convert';

import 'package:flutter_test/flutter_test.dart';
import 'package:lifelog/model/datum.dart';
import 'package:lifelog/model/eintrag.dart';

/// Vollständiger Eintrag aus docs/API.md, Abschnitt 2.
const beispielJson = '''
{
  "id": "6727a1f3c2b4e5d6f7a8b9c0",
  "modul": "gesundheit",
  "typ": "laborbefund",
  "dokumentdatum": "2026-10-14",
  "importdatum": "2026-11-03T18:42:07Z",
  "quelle": "Labor Muster GmbH, Salzburg",
  "volltext": "Labor Muster GmbH\\nHämoglobin 14,2 g/dl",
  "infos": [
    { "id": "i1", "name": "Hämoglobin", "wert": "14,2", "einheit": "g/dl" },
    { "id": "i2", "name": "Leukozyten", "wert": "6,8", "einheit": "G/l" },
    { "id": "i3", "name": "CRP", "wert": "<0,5", "einheit": "mg/dl" },
    { "id": "i4", "name": "Befundart", "wert": "Blutbild klein" }
  ],
  "datei": {
    "id": "6727a1f3c2b4e5d6f7a8b9c1",
    "dateiname": "befund_oktober.pdf",
    "mimeTyp": "application/pdf",
    "groesse": 184320,
    "sha256": "3f8a1c0e9b7d2f4a6c5e8b1d0a9f7e3c2b4d6a8f0e1c3b5d7a9f2e4c6b8d0a1f"
  },
  "korrekturen": [
    {
      "feld": "infos.i2.wert",
      "erkannt": "6,3",
      "korrigiert": "6,8",
      "zeitpunkt": "2026-11-03T19:05:31Z"
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
    expect(eintrag.modul, 'gesundheit');
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
    expect(eintrag.korrektur('infos.i2.wert')?.erkannt, '6,3');
    expect(eintrag.korrektur('dokumentdatum'), isNull);
    expect(eintrag.infoKorrigiert('i2'), isTrue);
    expect(eintrag.infoKorrigiert('i1'), isFalse);
  });

  test('null für dokumentdatum und quelle', () {
    final json = beispiel()
      ..['dokumentdatum'] = null
      ..['quelle'] = null;
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
      () => Eintrag.fromJson(beispiel()..['dokumentdatum'] = '14.10.2026'),
      throwsFormatException,
    );
    expect(
      () => Eintrag.fromJson(beispiel()..['infos'] = 'keine Liste'),
      throwsFormatException,
    );
    expect(
      () => Eintrag.fromJson(beispiel()..['importdatum'] = 'gestern'),
      throwsFormatException,
    );
  });
}
