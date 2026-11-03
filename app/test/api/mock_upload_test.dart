import 'dart:convert';
import 'dart:typed_data';

import 'package:crypto/crypto.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:lifelog/api/api_client.dart';
import 'package:lifelog/api/api_fehler.dart';
import 'package:lifelog/api/mime_typ.dart';

import '../hilfen.dart';

Uint8List pdf(String inhalt) =>
    Uint8List.fromList(utf8.encode('%PDF-1.4 $inhalt'));

void main() {
  test('Upload legt einen neuen Eintrag an', () async {
    final mock = schnellerMock();
    final bytes = pdf('neu');
    final eintrag = await mock.hochladen(bytes, 'befund.pdf');

    expect(eintrag.id, matches(RegExp(r'^[0-9a-f]{24}$')));
    expect(eintrag.modul, 'gesundheit');
    expect(eintrag.korrekturen, isEmpty);
    expect(eintrag.importdatum, DateTime.utc(2026, 11, 4, 9, 30));
    expect(eintrag.datei.dateiname, 'befund.pdf');
    expect(eintrag.datei.mimeTyp, 'application/pdf');
    expect(eintrag.datei.groesse, bytes.length);
    expect(eintrag.datei.sha256, sha256.convert(bytes).toString());

    expect(await mock.eintraege(), hasLength(4));
    expect((await mock.datei(eintrag.id)).bytes, bytes);
  });

  test('dieselbe Datei zweimal: 409 DUPLIKAT mit eintragId', () async {
    final mock = schnellerMock();
    final erster = await mock.hochladen(pdf('doppelt'), 'a.pdf');
    try {
      await mock.hochladen(pdf('doppelt'), 'anderer_name.pdf');
      fail('Fehler erwartet');
    } on ApiFehler catch (f) {
      expect(f.httpStatus, 409);
      expect(f.code, FehlerCode.duplikat);
      expect(f.details['eintragId'], erster.id);
    }
    expect(await mock.eintraege(), hasLength(4));
  });

  test('leere Datei: 400 DATEI_FEHLT', () async {
    await expectLater(
      schnellerMock().hochladen(Uint8List(0), 'leer.pdf'),
      wirftApiFehler(FehlerCode.dateiFehlt, status: 400),
    );
  });

  test('über 10 MB: 413 DATEI_ZU_GROSS', () async {
    final gross = Uint8List(maxDateiGroesse + 1)..setAll(0, '%PDF-'.codeUnits);
    await expectLater(
      schnellerMock().hochladen(gross, 'gross.pdf'),
      wirftApiFehler(FehlerCode.dateiZuGross, status: 413),
    );
  });

  test('genau 10 MB sind erlaubt', () async {
    final grenze = Uint8List(maxDateiGroesse)..setAll(0, '%PDF-'.codeUnits);
    final eintrag = await schnellerMock().hochladen(grenze, 'grenze.pdf');
    expect(eintrag.datei.groesse, maxDateiGroesse);
  });

  test('anderer Dateityp: 415, egal welche Endung', () async {
    await expectLater(
      schnellerMock().hochladen(
        Uint8List.fromList(utf8.encode('hallo')),
        'text.pdf',
      ),
      wirftApiFehler(FehlerCode.dateitypNichtUnterstuetzt, status: 415),
    );
  });

  group('Fehler des Vision-LLM über den Dateinamen', () {
    for (final (name, code, status) in [
      ('foto_unlesbar.jpg', FehlerCode.nichtLesbar, 422),
      ('auslesefehler.pdf', FehlerCode.auslesenFehlgeschlagen, 502),
      ('modellfehler.pdf', FehlerCode.modellNichtErreichbar, 503),
    ]) {
      test('$name: $status $code, nichts gespeichert', () async {
        final mock = schnellerMock();
        await expectLater(
          mock.hochladen(pdf(name), name),
          wirftApiFehler(code, status: status),
        );
        expect(await mock.eintraege(), hasLength(3));
      });
    }
  });

  group('erkenneMimeTyp', () {
    Uint8List bytes(List<int> b) => Uint8List.fromList(b);

    test('JPEG, PNG, PDF, HEIC', () {
      expect(erkenneMimeTyp(bytes([0xFF, 0xD8, 0xFF, 0xE0])), 'image/jpeg');
      expect(
        erkenneMimeTyp(
          bytes([0x89, 0x50, 0x4E, 0x47, 0x0D, 0x0A, 0x1A, 0x0A, 0]),
        ),
        'image/png',
      );
      expect(erkenneMimeTyp(pdf('')), 'application/pdf');
      expect(
        erkenneMimeTyp(bytes([0, 0, 0, 24, ...'ftypheic'.codeUnits])),
        'image/heic',
      );
    });

    test('unbekannt oder zu kurz', () {
      expect(erkenneMimeTyp(bytes([0xFF, 0xD8])), isNull);
      expect(erkenneMimeTyp(bytes(utf8.encode('GIF89a'))), isNull);
      expect(
        erkenneMimeTyp(bytes([0, 0, 0, 24, ...'ftypmp42'.codeUnits])),
        isNull,
      );
    });
  });
}
