import 'dart:typed_data';

import 'package:flutter_test/flutter_test.dart';
import 'package:lifelog/api/mime_type.dart';

import '../helpers.dart';

Uint8List bytes(List<int> start) => Uint8List.fromList([...start, 0, 0, 0, 0]);

/// MP4 box `ftyp` with the given brand.
Uint8List ftyp(String brand) =>
    Uint8List.fromList([0, 0, 0, 0x18, ...'ftyp$brand'.codeUnits, 0, 0, 0, 0]);

void main() {
  group('detectAudioType', () {
    test('M4A from the package record', () {
      expect(detectAudioType(testM4a()), 'audio/mp4');
      expect(detectAudioType(ftyp('mp42')), 'audio/mp4');
      expect(detectAudioType(ftyp('isom')), 'audio/mp4');
    });

    test('3GP', () {
      expect(detectAudioType(ftyp('3gp4')), 'audio/3gpp');
      expect(detectAudioType(ftyp('3gp5')), 'audio/3gpp');
    });

    test('the other formats from API 3.9', () {
      expect(detectAudioType(bytes('#!AMR\n'.codeUnits)), 'audio/amr');
      expect(
        detectAudioType(
          bytes([...'RIFF'.codeUnits, 1, 2, 3, 4, ...'WAVE'.codeUnits]),
        ),
        'audio/wav',
      );
      expect(detectAudioType(bytes('OggS'.codeUnits)), 'audio/ogg');
      expect(detectAudioType(bytes([0x1A, 0x45, 0xDF, 0xA3])), 'audio/webm');
      expect(detectAudioType(bytes('ID3'.codeUnits)), 'audio/mpeg');
      expect(detectAudioType(bytes([0xFF, 0xFB, 0x90])), 'audio/mpeg');
      expect(detectAudioType(bytes('fLaC'.codeUnits)), 'audio/flac');
    });

    test('not audio: images, PDF, AAC without a container, too short', () {
      expect(detectAudioType(ftyp('heic')), isNull);
      expect(detectAudioType(bytes([0xFF, 0xD8, 0xFF])), isNull); // JPEG
      expect(detectAudioType(bytes('%PDF-'.codeUnits)), isNull);
      expect(detectAudioType(bytes([0xFF, 0xF1, 0x50])), isNull); // ADTS
      expect(detectAudioType(bytes('RIFF'.codeUnits)), isNull); // no WAVE
      expect(detectAudioType(Uint8List(0)), isNull);
      expect(detectAudioType(Uint8List.fromList([0xFF])), isNull);
    });

    test('HEIC stays an image for uploads', () {
      expect(detectMimeType(ftyp('heic')), 'image/heic');
      expect(detectMimeType(testM4a()), isNull);
    });
  });
}
