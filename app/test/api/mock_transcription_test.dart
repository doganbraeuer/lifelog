import 'dart:typed_data';

import 'package:flutter_test/flutter_test.dart';
import 'package:lifelog/api/api_client.dart';
import 'package:lifelog/api/api_exception.dart';
import 'package:lifelog/api/mock_api_client.dart';
import 'package:lifelog/model/server_status.dart';

import '../helpers.dart';

void main() {
  group('transcribe', () {
    test('returns the fixed German sample text and stores nothing', () async {
      final mock = fastMock();
      final before = await mock.listEntries(module: 'health');

      final transcript = await mock.transcribe(testM4a('ton'), 'note.m4a');

      expect(transcript.text, MockApiClient.sampleTranscript);
      expect(transcript.language, 'de');
      expect(transcript.durationSeconds, greaterThan(0));
      expect(transcript.engine, isNotEmpty);
      // Nothing is stored (3.9): the timeline is unchanged.
      expect(
        await mock.listEntries(module: 'health'),
        hasLength(before.length),
      );
    });

    test('the confirmed text is saved through the text import', () async {
      final mock = fastMock();
      final transcript = await mock.transcribe(testM4a(), 'note.m4a');
      final entry = await mock.importText(transcript.text, module: 'health');

      expect(entry.fullText, MockApiClient.sampleTranscript);
      expect(entry.file.fileName, 'text.txt');
      expect(entry.documentDate?.iso, '2026-11-12');
    });

    test('empty recording: 400 FILE_MISSING', () async {
      await expectLater(
        fastMock().transcribe(Uint8List(0), 'note.m4a'),
        throwsApiException(ErrorCode.fileMissing, status: 400),
      );
    });

    test('over 10 MB: 413 FILE_TOO_LARGE', () async {
      final bytes = Uint8List(maxFileSize + 1)..setAll(0, testM4a());
      await expectLater(
        fastMock().transcribe(bytes, 'note.m4a'),
        throwsApiException(ErrorCode.fileTooLarge, status: 413),
      );
    });

    test('not an audio format: 415 UNSUPPORTED_FILE_TYPE', () async {
      await expectLater(
        fastMock().transcribe(
          Uint8List.fromList('%PDF-1.4'.codeUnits),
          'note.m4a',
        ),
        throwsApiException(ErrorCode.unsupportedFileType, status: 415),
      );
    });

    for (final (trigger, code, status) in [
      ('nospeech', ErrorCode.noSpeech, 422),
      ('toolong', ErrorCode.audioTooLong, 413),
      ('unreadable', ErrorCode.unreadable, 422),
      ('modelerror', ErrorCode.modelUnavailable, 503),
    ]) {
      test('trigger "$trigger" in the file name: $status $code', () async {
        await expectLater(
          fastMock().transcribe(testM4a(), 'note_$trigger.m4a'),
          throwsApiException(code, status: status),
        );
      });
    }

    test(
      'AUDIO_TOO_LONG has details.maxSeconds and the German message',
      () async {
        try {
          await fastMock().transcribe(testM4a(), 'toolong.m4a');
          fail('error expected');
        } on ApiException catch (e) {
          expect(e.details['maxSeconds'], 300);
          expect(
            e.message,
            'Die Aufnahme ist länger als 5 Minuten. Bitte kürzer sprechen oder '
            'aufteilen.',
          );
        }
      },
    );

    test('NO_SPEECH has the German message from the contract', () async {
      try {
        await fastMock().transcribe(testM4a(), 'nospeech.m4a');
        fail('error expected');
      } on ApiException catch (e) {
        expect(e.message, 'In der Aufnahme ist keine Sprache zu hören.');
      }
    });

    for (final speech in [SpeechStatus.unavailable, SpeechStatus.off]) {
      test('speech ${speech.name}: 503 MODEL_UNAVAILABLE', () async {
        final mock = fastMock()..speechStatus = speech;
        await expectLater(
          mock.transcribe(testM4a(), 'note.m4a'),
          throwsApiException(ErrorCode.modelUnavailable, status: 503),
        );
      });
    }

    test('simulateError: 500 INTERNAL_ERROR', () async {
      final mock = fastMock()..simulateError = true;
      await expectLater(
        mock.transcribe(testM4a(), 'note.m4a'),
        throwsApiException(ErrorCode.internalError, status: 500),
      );
    });
  });

  group('getStatus', () {
    test('everything ok by default', () async {
      final status = await fastMock().getStatus();
      expect(status.status, 'ok');
      expect(status.database, 'ok');
      expect(status.speech, SpeechStatus.ok);
      expect(status.speechAvailable, isTrue);
    });

    for (final speech in [SpeechStatus.unavailable, SpeechStatus.off]) {
      test('speech ${speech.name}', () async {
        final mock = fastMock()..speechStatus = speech;
        final status = await mock.getStatus();
        expect(status.speech, speech);
        expect(status.speechAvailable, isFalse);
      });
    }
  });
}
