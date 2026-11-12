import 'dart:typed_data';

import 'package:flutter_test/flutter_test.dart';
import 'package:lifelog/model/entry.dart';

File fileWith(String mimeType) => File(
  id: '6727a1f3c2b4e5d6f7a8b9c1',
  fileName: 'x',
  mimeType: mimeType,
  size: 1,
  sha256: '0' * 64,
);

FileContent content(String mimeType) =>
    FileContent(bytes: Uint8List(0), mimeType: mimeType);

void main() {
  test('mediaType without parameters and in lower case', () {
    expect(mediaType('text/plain'), 'text/plain');
    expect(mediaType('text/plain; charset=utf-8'), 'text/plain');
    expect(mediaType(' Text/Plain ;charset=UTF-8'), 'text/plain');
    expect(mediaType('application/pdf'), 'application/pdf');
  });

  group('isText', () {
    for (final mimeType in [
      'text/plain',
      'text/plain; charset=utf-8',
      'TEXT/PLAIN;charset=UTF-8',
    ]) {
      test('yes: $mimeType', () {
        expect(fileWith(mimeType).isText, isTrue);
        expect(content(mimeType).isText, isTrue);
      });
    }

    for (final mimeType in [
      'text/html',
      'text/plainx',
      'application/pdf',
      'image/png',
    ]) {
      test('no: $mimeType', () {
        expect(fileWith(mimeType).isText, isFalse);
        expect(content(mimeType).isText, isFalse);
      });
    }
  });

  test('PDF and image recognized with parameters too', () {
    expect(fileWith('application/pdf; name=x').isPdf, isTrue);
    expect(content('IMAGE/JPEG').isImage, isTrue);
    expect(fileWith('text/plain').isPdf, isFalse);
    expect(fileWith('text/plain').isImage, isFalse);
  });
}
