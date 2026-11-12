import 'dart:typed_data';

import 'package:flutter_test/flutter_test.dart';
import 'package:lifelog/model/entry.dart';

Datei datei(String mimeTyp) => Datei(
  id: '6727a1f3c2b4e5d6f7a8b9c1',
  dateiname: 'x',
  mimeTyp: mimeTyp,
  groesse: 1,
  sha256: '0' * 64,
);

DateiInhalt inhalt(String mimeTyp) =>
    DateiInhalt(bytes: Uint8List(0), mimeTyp: mimeTyp);

void main() {
  test('medientyp ohne Parameter und klein geschrieben', () {
    expect(medientyp('text/plain'), 'text/plain');
    expect(medientyp('text/plain; charset=utf-8'), 'text/plain');
    expect(medientyp(' Text/Plain ;charset=UTF-8'), 'text/plain');
    expect(medientyp('application/pdf'), 'application/pdf');
  });

  group('istText', () {
    for (final mimeTyp in [
      'text/plain',
      'text/plain; charset=utf-8',
      'TEXT/PLAIN;charset=UTF-8',
    ]) {
      test('ja: $mimeTyp', () {
        expect(datei(mimeTyp).istText, isTrue);
        expect(inhalt(mimeTyp).istText, isTrue);
      });
    }

    for (final mimeTyp in [
      'text/html',
      'text/plainx',
      'application/pdf',
      'image/png',
    ]) {
      test('nein: $mimeTyp', () {
        expect(datei(mimeTyp).istText, isFalse);
        expect(inhalt(mimeTyp).istText, isFalse);
      });
    }
  });

  test('PDF und Bild auch mit Parametern erkannt', () {
    expect(datei('application/pdf; name=x').istPdf, isTrue);
    expect(inhalt('IMAGE/JPEG').istBild, isTrue);
    expect(datei('text/plain').istPdf, isFalse);
    expect(datei('text/plain').istBild, isFalse);
  });
}
