import 'dart:convert';
import 'dart:typed_data';

import 'package:crypto/crypto.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:lifelog/api/api_client.dart';
import 'package:lifelog/api/api_exception.dart';
import 'package:lifelog/api/mime_type.dart';

import '../helpers.dart';

Uint8List pdf(String inhalt) =>
    Uint8List.fromList(utf8.encode('%PDF-1.4 $inhalt'));

void main() {
  test('Upload legt einen neuen Eintrag an', () async {
    final mock = schnellerMock();
    final bytes = pdf('neu');
    final entry = await mock.upload(bytes, 'befund.pdf', module: 'health');

    expect(entry.id, matches(RegExp(r'^[0-9a-f]{24}$')));
    expect(entry.module, 'health');
    expect(entry.corrections, isEmpty);
    expect(entry.importedAt, DateTime.utc(2026, 11, 4, 9, 30));
    expect(entry.file.fileName, 'befund.pdf');
    expect(entry.file.mimeType, 'application/pdf');
    expect(entry.file.size, bytes.length);
    expect(entry.file.sha256, sha256.convert(bytes).toString());

    expect(await mock.listEntries(module: 'health'), hasLength(4));
    expect((await mock.getFile(entry.id)).bytes, bytes);
  });

  test('dieselbe Datei zweimal: 409 DUPLICATE mit eintragId', () async {
    final mock = schnellerMock();
    final erster = await mock.upload(pdf('doppelt'), 'a.pdf', module: 'health');
    try {
      await mock.upload(pdf('doppelt'), 'anderer_name.pdf', module: 'health');
      fail('Fehler erwartet');
    } on ApiException catch (f) {
      expect(f.httpStatus, 409);
      expect(f.code, ErrorCode.duplicate);
      expect(f.details['entryId'], erster.id);
    }
    expect(await mock.listEntries(module: 'health'), hasLength(4));
  });

  test('leere Datei: 400 FILE_MISSING', () async {
    await expectLater(
      schnellerMock().upload(Uint8List(0), 'leer.pdf', module: 'health'),
      wirftApiFehler(ErrorCode.fileMissing, status: 400),
    );
  });

  test('über 10 MB: 413 FILE_TOO_LARGE', () async {
    final gross = Uint8List(maxFileSize + 1)..setAll(0, '%PDF-'.codeUnits);
    await expectLater(
      schnellerMock().upload(gross, 'gross.pdf', module: 'health'),
      wirftApiFehler(ErrorCode.fileTooLarge, status: 413),
    );
  });

  test('genau 10 MB sind erlaubt', () async {
    final grenze = Uint8List(maxFileSize)..setAll(0, '%PDF-'.codeUnits);
    final entry = await schnellerMock().upload(
      grenze,
      'grenze.pdf',
      module: 'health',
    );
    expect(entry.file.size, maxFileSize);
  });

  test('anderer Dateityp: 415, egal welche Endung', () async {
    await expectLater(
      schnellerMock().upload(
        Uint8List.fromList(utf8.encode('hallo')),
        'text.pdf',
        module: 'health',
      ),
      wirftApiFehler(ErrorCode.unsupportedFileType, status: 415),
    );
  });

  group('Fehler des Vision-LLM über den Dateinamen', () {
    for (final (name, code, status) in [
      ('photo_unreadable.jpg', ErrorCode.unreadable, 422),
      ('extractionerror.pdf', ErrorCode.extractionFailed, 502),
      ('modelerror.pdf', ErrorCode.modelUnavailable, 503),
    ]) {
      test('$name: $status $code, nichts gespeichert', () async {
        final mock = schnellerMock();
        await expectLater(
          mock.upload(pdf(name), name, module: 'health'),
          wirftApiFehler(code, status: status),
        );
        expect(await mock.listEntries(module: 'health'), hasLength(3));
      });
    }
  });

  group('erkenneMimeTyp', () {
    Uint8List bytes(List<int> b) => Uint8List.fromList(b);

    test('JPEG, PNG, PDF, HEIC', () {
      expect(detectMimeType(bytes([0xFF, 0xD8, 0xFF, 0xE0])), 'image/jpeg');
      expect(
        detectMimeType(
          bytes([0x89, 0x50, 0x4E, 0x47, 0x0D, 0x0A, 0x1A, 0x0A, 0]),
        ),
        'image/png',
      );
      expect(detectMimeType(pdf('')), 'application/pdf');
      expect(
        detectMimeType(bytes([0, 0, 0, 24, ...'ftypheic'.codeUnits])),
        'image/heic',
      );
    });

    test('unbekannt oder zu kurz', () {
      expect(detectMimeType(bytes([0xFF, 0xD8])), isNull);
      expect(detectMimeType(bytes(utf8.encode('GIF89a'))), isNull);
      expect(
        detectMimeType(bytes([0, 0, 0, 24, ...'ftypmp42'.codeUnits])),
        isNull,
      );
    });
  });
}
