// Sample data for the mock. All data is synthetic.
//
// The first three entries come from docs/API.md (sections 2 and 3.2). For
// the doctor letter and "other" the API only gives the short form; full text
// and facts are added here. The other entries show the remaining cases of
// the verification (text import, mixed PDF, unchecked with a reason).
//
// `verification` is computed by the mock's second reader from the
// "original" of each entry (mock_verification.dart), so that it changes
// after a correction exactly as it does in the backend.

import 'dart:convert';
import 'dart:typed_data';

import 'package:crypto/crypto.dart';

import '../../model/verification.dart';
import 'mock_files.dart';
import 'mock_verification.dart';

/// An entry as JSON (as from the backend) plus the bytes of the original
/// file and what the second reader sees of it (`null`: unchecked).
class MockEntry {
  MockEntry(this.json, this.bytes, [this.original]);

  final Map<String, dynamic> json;
  final Uint8List bytes;
  final MockOriginal? original;
}

/// Number of sample entries, for tests.
const sampleCount = 7;

List<MockEntry> sampleData() {
  // Lab report exactly as in docs/API.md, section 2. The file metadata
  // (size, SHA-256) also comes from the API description; the file itself is
  // a small substitute PDF. Its text layer says „CRP unter 0,5“, so the
  // value „<0,5“ is unconfirmed, as in the example (5 of 6 confirmed).
  const labReportText =
      'Labor Muster GmbH\nMusterstraße 1, 5020 Salzburg\n\n'
      'Laborbefund vom 14.10.2026\n'
      'Patient: Max Mustermann, geb. 01.01.1990\n\n'
      'Hämoglobin 14,2 g/dl (Ref. 13,5-17,5)\n'
      'Leukozyten 6,8 G/l (Ref. 4,0-10,0)\n'
      'CRP <0,5 mg/dl (Ref. <0,5)\n'
      'Befundart: Blutbild klein';
  final labReportPage = [
    '(Mock-Datei)',
    '',
    ...labReportText
        .replaceFirst('CRP <0,5 mg/dl', 'CRP unter 0,5 mg/dl')
        .split('\n'),
  ];
  final labReport = <String, dynamic>{
    'id': '6727a1f3c2b4e5d6f7a8b9c0',
    'module': 'health',
    'type': 'lab_report',
    'documentDate': '2026-10-14',
    'importedAt': '2026-11-03T18:42:07Z',
    'issuer': 'Labor Muster GmbH, Salzburg',
    'fullText': labReportText,
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

  // Photo read by OCR: „/min“ was read as „/mln“, so the pulse is
  // unconfirmed although its value is right.
  const doctorLetterText =
      'Dr. Erika Beispiel\nÄrztin für Allgemeinmedizin\n'
      'Beispielgasse 7, 5020 Salzburg\n\n'
      'Arztbrief vom 02.09.2026\n'
      'Patient: Max Mustermann, geb. 01.01.1990\n\n'
      'Vorstellungsgrund: Kontrolle nach Erkältung\n'
      'Blutdruck 128/82 mmHg\n'
      'Puls 72/min\n'
      'Körpergewicht 78 kg';
  final doctorLetterImage = createMockPng();
  final doctorLetter = <String, dynamic>{
    'id': '6727a20bc2b4e5d6f7a8b9d4',
    'module': 'health',
    'type': 'doctor_letter',
    'documentDate': '2026-09-02',
    'importedAt': '2026-11-03T18:50:12Z',
    'issuer': 'Dr. Erika Beispiel, Allgemeinmedizin',
    'fullText': doctorLetterText,
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
    'file': _fileJson(
      '6727a20bc2b4e5d6f7a8b9d5',
      'arztbrief_september.png',
      'image/png',
      doctorLetterImage,
    ),
    'corrections': <dynamic>[],
  };

  // A blurry scan: OCR found no text.
  final otherImage = createMockPng(width: 200, height: 260);
  final other = <String, dynamic>{
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
    'file': _fileJson(
      '6727a2a9c2b4e5d6f7a8b9e8',
      'scan_impfpass.png',
      'image/png',
      otherImage,
    ),
    'corrections': <dynamic>[],
    'verification': uncheckedVerification(VerificationReason.noText),
  };

  // Text import: compared with the entered text; locations are ranges in
  // fullText. The weight was recognized as 81,5, the text says „etwa 81“.
  const noteText =
      'Blutdruckmessung zu Hause\n'
      'Messung am 05.11.2026 morgens\n'
      'Blutdruck 135/85 mmHg\n'
      'Puls 64/min\n'
      'Gewicht etwa 81 kg';
  final noteBytes = Uint8List.fromList(utf8.encode(noteText));
  final note = <String, dynamic>{
    'id': '6727a301c2b4e5d6f7a8b9f1',
    'module': 'health',
    'type': 'other',
    'documentDate': '2026-11-05',
    'importedAt': '2026-11-05T07:15:02Z',
    'issuer': 'Blutdruckmessung zu Hause',
    'fullText': noteText,
    'facts': [
      {'id': 'f1', 'name': 'Blutdruck', 'value': '135/85', 'unit': 'mmHg'},
      {'id': 'f2', 'name': 'Puls', 'value': '64', 'unit': '/min'},
      {'id': 'f3', 'name': 'Gewicht', 'value': '81,5', 'unit': 'kg'},
    ],
    'file': _fileJson(
      '6727a301c2b4e5d6f7a8b9f2',
      'text.txt',
      'text/plain; charset=utf-8',
      noteBytes,
    ),
    'corrections': <dynamic>[],
  };

  // PDF with a digital first page (text layer) and a scanned second page
  // (OCR): source „mixed“. The number of packages was corrected to a value
  // that is not on the prescription.
  const prescriptionPage1 = [
    'Dr. Jonas Muster',
    'Facharzt für Innere Medizin',
    'Ringstraße 12, 5020 Salzburg',
    '',
    'Rezept vom 20.08.2026',
    'Patient: Max Mustermann, geb. 01.01.1990',
    '',
    'Ramipril 5 mg Tabletten',
    'Packungen: 2',
  ];
  const prescriptionPage2 = [
    'Einnahmeplan (Seite 2, gescannt)',
    '',
    'Ramipril 5 mg: 1-0-0',
    'Metformin 500 mg: 1-0-1',
  ];
  final prescriptionPdf = createMockPdfPages([
    prescriptionPage1,
    prescriptionPage2,
  ]);
  final prescription = <String, dynamic>{
    'id': '6727a2f0c2b4e5d6f7a8b9f4',
    'module': 'health',
    'type': 'prescription',
    'documentDate': '2026-08-20',
    'importedAt': '2026-11-04T16:20:41Z',
    'issuer': 'Dr. Jonas Muster, Innere Medizin',
    'fullText': [...prescriptionPage1, '', ...prescriptionPage2].join('\n'),
    'facts': [
      {'id': 'f1', 'name': 'Ramipril', 'value': '5', 'unit': 'mg'},
      {'id': 'f2', 'name': 'Packungen', 'value': '3'},
      {'id': 'f3', 'name': 'Metformin', 'value': '500', 'unit': 'mg'},
      {'id': 'f4', 'name': 'Einnahme Metformin', 'value': '1-0-1'},
    ],
    'file': _fileJson(
      '6727a2f0c2b4e5d6f7a8b9f5',
      'rezept_august.pdf',
      'application/pdf',
      prescriptionPdf,
    ),
    'corrections': [
      {
        'field': 'facts.f2.value',
        'recognized': '2',
        'corrected': '3',
        'correctedAt': '2026-11-04T16:25:10Z',
      },
    ],
  };

  // Photo on a server without OCR.
  final xRayImage = createMockPng(width: 260, height: 200);
  final xRay = <String, dynamic>{
    'id': '6727a2c4c2b4e5d6f7a8b9f6',
    'module': 'health',
    'type': 'doctor_letter',
    'documentDate': '2026-07-15',
    'importedAt': '2026-11-04T10:02:33Z',
    'issuer': 'Radiologie Beispiel, Salzburg',
    'fullText':
        'Radiologie Beispiel\nBahnhofstraße 3, 5020 Salzburg\n\n'
        'Röntgen Thorax vom 15.07.2026\n'
        'Patient: Max Mustermann, geb. 01.01.1990\n\n'
        'Untersuchung: Röntgen Thorax in zwei Ebenen',
    'facts': [
      {
        'id': 'f1',
        'name': 'Untersuchung',
        'value': 'Röntgen Thorax in zwei Ebenen',
      },
    ],
    'file': _fileJson(
      '6727a2c4c2b4e5d6f7a8b9f7',
      'roentgen_juli.png',
      'image/png',
      xRayImage,
    ),
    'corrections': <dynamic>[],
    'verification': uncheckedVerification(VerificationReason.ocrUnavailable),
  };

  // Entry from before the verification existed: never checked.
  const oldLabText =
      'Laborpraxis Nord\nNordweg 9, 5020 Salzburg\n\n'
      'Laborbefund vom 03.06.2026\n'
      'Patient: Max Mustermann, geb. 01.01.1990\n\n'
      'Nüchternglukose 92 mg/dl (Ref. 70-99)\n'
      'Cholesterin gesamt 185 mg/dl (Ref. <200)';
  final oldLabPdf = createMockPdf([
    '(Mock-Datei)',
    '',
    ...oldLabText.split('\n'),
  ]);
  final oldLab = <String, dynamic>{
    'id': '6727a18ec2b4e5d6f7a8b9f8',
    'module': 'health',
    'type': 'lab_report',
    'documentDate': '2026-06-03',
    'importedAt': '2026-11-02T09:12:20Z',
    'issuer': 'Laborpraxis Nord, Salzburg',
    'fullText': oldLabText,
    'facts': [
      {'id': 'f1', 'name': 'Nüchternglukose', 'value': '92', 'unit': 'mg/dl'},
      {
        'id': 'f2',
        'name': 'Cholesterin gesamt',
        'value': '185',
        'unit': 'mg/dl',
      },
    ],
    'file': _fileJson(
      '6727a18ec2b4e5d6f7a8b9f9',
      'befund_juni.pdf',
      'application/pdf',
      oldLabPdf,
    ),
    'corrections': <dynamic>[],
    'verification': uncheckedVerification(VerificationReason.notRun),
  };

  return [
    _checked(
      labReport,
      createMockPdf(labReportPage),
      MockOriginal.page(VerificationSource.pdfText, labReportPage),
    ),
    _checked(
      doctorLetter,
      doctorLetterImage,
      MockOriginal.page(
        VerificationSource.ocr,
        doctorLetterText.replaceFirst('72/min', '72/mln').split('\n'),
      ),
    ),
    MockEntry(_deepCopy(other), otherImage),
    _checked(note, noteBytes, MockOriginal.text(noteText)),
    _checked(
      prescription,
      prescriptionPdf,
      const MockOriginal([
        MockPage(VerificationSource.pdfText, prescriptionPage1),
        MockPage(VerificationSource.ocr, prescriptionPage2),
      ]),
    ),
    MockEntry(_deepCopy(xRay), xRayImage),
    MockEntry(_deepCopy(oldLab), oldLabPdf),
  ];
}

/// An entry checked against [original]. The check ran at the upload or at
/// the latest correction.
MockEntry _checked(
  Map<String, dynamic> json,
  Uint8List bytes,
  MockOriginal original,
) {
  final copy = _deepCopy(json);
  final times = [
    copy['importedAt'] as String,
    for (final c in copy['corrections'] as List)
      (c as Map)['correctedAt'] as String,
  ]..sort();
  copy['verification'] = checkEntry(copy, original, checkedAt: times.last);
  return MockEntry(copy, bytes, original);
}

Map<String, dynamic> _fileJson(
  String id,
  String fileName,
  String mimeType,
  Uint8List bytes,
) => {
  'id': id,
  'fileName': fileName,
  'mimeType': mimeType,
  'size': bytes.length,
  'sha256': sha256.convert(bytes).toString(),
};

/// Turns the literals into mutable `Map<String, dynamic>`, as returned by
/// `jsonDecode`. The mock changes the data later (PATCH).
Map<String, dynamic> _deepCopy(Map<String, dynamic> json) =>
    jsonDecode(jsonEncode(json)) as Map<String, dynamic>;
