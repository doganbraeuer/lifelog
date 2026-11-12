// Beispieldaten für den Mock. Alle Daten sind synthetisch.
//
// Die drei Einträge stammen aus docs/API.md (Abschnitt 2 und 3.2). Für
// Arztbrief und "other" nennt die API nur die Kurzform, Volltext und
// Infos sind hier ergänzt.

import 'dart:convert';
import 'dart:typed_data';

import 'package:crypto/crypto.dart';

import 'mock_dateien.dart';

/// Ein Eintrag als JSON (wie vom Backend) plus die Bytes der Originaldatei.
class MockEintrag {
  MockEintrag(this.json, this.bytes);

  final Map<String, dynamic> json;
  final Uint8List bytes;
}

List<MockEintrag> beispieldaten() {
  // Laborbefund genau wie in docs/API.md, Abschnitt 2. Die Metadaten der
  // Datei (Größe, SHA-256) stammen ebenfalls aus der API-Beschreibung; die
  // Datei selbst ist ein kleines Ersatz-PDF mit dem Volltext.
  const laborVolltext =
      'Labor Muster GmbH\nMusterstraße 1, 5020 Salzburg\n\n'
      'Laborbefund vom 14.10.2026\n'
      'Patient: Max Mustermann, geb. 01.01.1990\n\n'
      'Hämoglobin 14,2 g/dl (Ref. 13,5-17,5)\n'
      'Leukozyten 6,8 G/l (Ref. 4,0-10,0)\n'
      'CRP <0,5 mg/dl (Ref. <0,5)\n'
      'Befundart: Blutbild klein';
  final laborbefund = <String, dynamic>{
    'id': '6727a1f3c2b4e5d6f7a8b9c0',
    'module': 'health',
    'type': 'lab_report',
    'documentDate': '2026-10-14',
    'importedAt': '2026-11-03T18:42:07Z',
    'issuer': 'Labor Muster GmbH, Salzburg',
    'fullText': laborVolltext,
    'facts': [
      {'id': 'f1', 'name': 'Hämoglobin', 'value': '14,2', 'unit': 'g/dl'},
      {'id': 'f2', 'name': 'Leukozyten', 'value': '6,8', 'unit': 'G/l'},
      {'id': 'f3', 'name': 'CRP', 'value': '<0,5', 'unit': 'mg/dl'},
      {'id': 'f4', 'name': 'Befundart', 'value': 'Blutbild klein'},
    ],
    'file': {
      'id': '6727a1f3c2b4e5d6f7a8b9c1',
      'fileName': 'befund_oktober.pdf',
      'mimeType': 'application/pdf',
      'size': 184320,
      'sha256':
          '3f8a1c0e9b7d2f4a6c5e8b1d0a9f7e3c2b4d6a8f0e1c3b5d7a9f2e4c6b8d0a1f',
    },
    'corrections': [
      {
        'field': 'facts.f2.value',
        'recognized': '6,3',
        'corrected': '6,8',
        'correctedAt': '2026-11-03T19:05:31Z',
      },
    ],
  };

  final arztbriefBild = erzeugeMockPng();
  final arztbrief = <String, dynamic>{
    'id': '6727a20bc2b4e5d6f7a8b9d4',
    'module': 'health',
    'type': 'doctor_letter',
    'documentDate': '2026-09-02',
    'importedAt': '2026-11-03T18:50:12Z',
    'issuer': 'Dr. Erika Beispiel, Allgemeinmedizin',
    'fullText':
        'Dr. Erika Beispiel\nÄrztin für Allgemeinmedizin\n'
        'Beispielgasse 7, 5020 Salzburg\n\n'
        'Arztbrief vom 02.09.2026\n'
        'Patient: Max Mustermann, geb. 01.01.1990\n\n'
        'Vorstellungsgrund: Kontrolle nach Erkältung\n'
        'Blutdruck 128/82 mmHg\n'
        'Puls 72/min\n'
        'Körpergewicht 78 kg',
    'facts': [
      {
        'id': 'f1',
        'name': 'Vorstellungsgrund',
        'value': 'Kontrolle nach Erkältung',
      },
      {'id': 'f2', 'name': 'Blutdruck', 'value': '128/82', 'unit': 'mmHg'},
      {'id': 'f3', 'name': 'Puls', 'value': '72', 'unit': '/min'},
      {'id': 'f4', 'name': 'Körpergewicht', 'value': '78', 'unit': 'kg'},
    ],
    'file': _dateiJson(
      '6727a20bc2b4e5d6f7a8b9d5',
      'arztbrief_september.png',
      'image/png',
      arztbriefBild,
    ),
    'corrections': <dynamic>[],
  };

  final sonstigesBild = erzeugeMockPng(breite: 200, hoehe: 260);
  final sonstiges = <String, dynamic>{
    'id': '6727a2a9c2b4e5d6f7a8b9e7',
    'module': 'health',
    'type': 'other',
    'documentDate': null,
    'importedAt': '2026-11-03T19:01:44Z',
    'issuer': null,
    'fullText':
        'Impfpass (Auszug)\n'
        'Tetanus/Diphtherie Auffrischung\n'
        'Chargennummer AB1234\n'
        'Unterschrift unleserlich',
    'facts': [
      {'id': 'f1', 'name': 'Impfung', 'value': 'Tetanus/Diphtherie'},
      {'id': 'f2', 'name': 'Chargennummer', 'value': 'AB1234'},
    ],
    'file': _dateiJson(
      '6727a2a9c2b4e5d6f7a8b9e8',
      'scan_impfpass.png',
      'image/png',
      sonstigesBild,
    ),
    'corrections': <dynamic>[],
  };

  return [
    MockEintrag(
      _tiefeKopie(laborbefund),
      erzeugeMockPdf(['(Mock-Datei)', '', ...laborVolltext.split('\n')]),
    ),
    MockEintrag(_tiefeKopie(arztbrief), arztbriefBild),
    MockEintrag(_tiefeKopie(sonstiges), sonstigesBild),
  ];
}

Map<String, dynamic> _dateiJson(
  String id,
  String dateiname,
  String mimeTyp,
  Uint8List bytes,
) => {
  'id': id,
  'fileName': dateiname,
  'mimeType': mimeTyp,
  'size': bytes.length,
  'sha256': sha256.convert(bytes).toString(),
};

/// Wandelt die Literale in veränderbare `Map<String, dynamic>` um, wie sie
/// auch `jsonDecode` liefert. Der Mock ändert die Daten später (PATCH).
Map<String, dynamic> _tiefeKopie(Map<String, dynamic> json) =>
    jsonDecode(jsonEncode(json)) as Map<String, dynamic>;
