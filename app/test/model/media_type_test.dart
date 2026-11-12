import 'dart:typed_data';

import 'package:flutter_test/flutter_test.dart';
import 'package:lifelog/model/entry.dart';

File getFile(String mimeType) => File(
  id: '6727a1f3c2b4e5d6f7a8b9c1',
  fileName: 'x',
  mimeType: mimeType,
  size: 1,
  sha256: '0' * 64,
);

FileContent inhalt(String mimeType) =>
    FileContent(bytes: Uint8List(0), mimeType: mimeType);

void main() {
  test('medientyp ohne Parameter und klein geschrieben', () {
    expect(mediaType('text/plain'), 'text/plain');
    expect(mediaType('text/plain; charset=utf-8'), 'text/plain');
    expect(mediaType(' Text/Plain ;charset=UTF-8'), 'text/plain');
    expect(mediaType('application/pdf'), 'application/pdf');
  });

  group('istText', () {
    for (final mimeType in [
      'text/plain',
      'text/plain; charset=utf-8',
      'TEXT/PLAIN;charset=UTF-8',
    ]) {
      test('ja: $mimeType', () {
        expect(getFile(mimeType).isText, isTrue);
        expect(inhalt(mimeType).isText, isTrue);
      });
    }

    for (final mimeType in [
      'text/html',
      'text/plainx',
      'application/pdf',
      'image/png',
    ]) {
      test('nein: $mimeType', () {
        expect(getFile(mimeType).isText, isFalse);
        expect(inhalt(mimeType).isText, isFalse);
      });
    }
  });

  test('PDF und Bild auch mit Parametern erkannt', () {
    expect(getFile('application/pdf; name=x').isPdf, isTrue);
    expect(inhalt('IMAGE/JPEG').isImage, isTrue);
    expect(getFile('text/plain').isPdf, isFalse);
    expect(getFile('text/plain').isImage, isFalse);
  });
}
