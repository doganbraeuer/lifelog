import 'package:flutter/material.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:lifelog/api/api_exception.dart';
import 'package:lifelog/api/mock_api_client.dart';
import 'package:lifelog/model/server_status.dart';
import 'package:lifelog/screens/entry_screen.dart';
import 'package:lifelog/screens/modules_screen.dart';

import '../helpers.dart';

Finder voiceButton() => find.widgetWithText(OutlinedButton, 'Sprachnotiz');
Finder stopButton() => find.widgetWithText(FilledButton, 'Stopp');
Finder saveButton() => find.widgetWithText(FilledButton, 'Speichern');
Finder textField() => find.widgetWithText(TextField, 'Dokumentinhalt');

const hintWithSpeech =
    'Erlaubt sind JPEG, PNG, HEIC und PDF bis 10 MB, Text bis 100 000 '
    'Zeichen oder eine Sprachnotiz bis 5 Minuten.';
const hintWithoutSpeech =
    'Erlaubt sind JPEG, PNG, HEIC und PDF bis 10 MB oder Text bis 100 000 '
    'Zeichen.';
const unavailableNotice = 'Sprachnotizen sind gerade nicht verfügbar.';

/// Starts the app and switches to the „Hochladen“ page.
Future<void> goToUpload(
  WidgetTester tester,
  MockApiClient mock,
  FakeVoiceRecorder recorder,
) async {
  useLargeScreen(tester);
  await tester.pumpWidget(testApp(mock, recorder: recorder));
  await tester.pumpAndSettle();
  await openHealth(tester);
  await tester.tap(
    find.descendant(
      of: find.byType(NavigationBar),
      matching: find.text('Hochladen'),
    ),
  );
  await tester.pumpAndSettle();
}

/// Taps „Sprachnotiz“ and waits until the recording runs. No
/// pumpAndSettle while recording: the clock ticks and the microphone
/// pulses, so the screen never settles.
Future<void> startRecording(WidgetTester tester) async {
  await tester.tap(voiceButton());
  await tester.pump();
  await tester.pump(const Duration(milliseconds: 500));
}

/// Taps „Stopp“; with [fastMock] the transcription is done right away.
Future<void> stopAndTranscribe(WidgetTester tester) async {
  await tester.tap(stopButton());
  await tester.pumpAndSettle();
}

bool enabled(WidgetTester tester, Finder button) =>
    tester.widget<ButtonStyleButton>(button).onPressed != null;

/// Mock whose `/status` cannot be reached.
class _StatusFailsMock extends MockApiClient {
  _StatusFailsMock()
    : super(
        delay: Duration.zero,
        uploadDelay: Duration.zero,
        transcriptionDelay: Duration.zero,
      );

  @override
  Future<ServerStatus> getStatus() async => throw const ApiException(
    code: ErrorCode.networkError,
    message: 'Keine Verbindung zum Server.',
  );
}

void main() {
  group('entry depends on /status', () {
    testWidgets('speech ok: button active, hint promises spoken notes', (
      tester,
    ) async {
      await goToUpload(tester, fastMock(), FakeVoiceRecorder());

      expect(voiceButton(), findsOneWidget);
      expect(enabled(tester, voiceButton()), isTrue);
      expect(find.text(hintWithSpeech), findsOneWidget);
      expect(find.text(unavailableNotice), findsNothing);
    });

    testWidgets('speech off: no button, hint without spoken notes', (
      tester,
    ) async {
      final mock = fastMock()..speechStatus = SpeechStatus.off;
      await goToUpload(tester, mock, FakeVoiceRecorder());

      expect(voiceButton(), findsNothing);
      expect(find.text(hintWithoutSpeech), findsOneWidget);
      expect(find.text(unavailableNotice), findsNothing);
    });

    testWidgets('speech unavailable: button disabled, „Erneut prüfen“', (
      tester,
    ) async {
      final mock = fastMock()..speechStatus = SpeechStatus.unavailable;
      await goToUpload(tester, mock, FakeVoiceRecorder());

      expect(enabled(tester, voiceButton()), isFalse);
      expect(find.text(unavailableNotice), findsOneWidget);
      expect(find.text(hintWithoutSpeech), findsOneWidget);

      // The speech recognition is back: checking again enables the button.
      mock.speechStatus = SpeechStatus.ok;
      await tester.tap(find.text('Erneut prüfen'));
      await tester.pumpAndSettle();

      expect(enabled(tester, voiceButton()), isTrue);
      expect(find.text(unavailableNotice), findsNothing);
      expect(find.text(hintWithSpeech), findsOneWidget);
    });

    testWidgets('status cannot be loaded: button disabled with notice', (
      tester,
    ) async {
      await goToUpload(tester, _StatusFailsMock(), FakeVoiceRecorder());

      expect(enabled(tester, voiceButton()), isFalse);
      expect(find.text(unavailableNotice), findsOneWidget);
      expect(find.text(hintWithoutSpeech), findsOneWidget);
    });
  });

  group('microphone', () {
    testWidgets('permission denied: German message, nothing recorded', (
      tester,
    ) async {
      final recorder = FakeVoiceRecorder()..permission = false;
      await goToUpload(tester, fastMock(), recorder);

      await tester.tap(voiceButton());
      await tester.pumpAndSettle();

      expect(
        find.text(
          'Kein Zugriff auf das Mikrofon. Bitte in den Einstellungen des '
          'Geräts erlauben.',
        ),
        findsOneWidget,
      );
      expect(recorder.permissionRequests, 1);
      expect(recorder.starts, 0);
      expect(stopButton(), findsNothing);
      expect(voiceButton(), findsOneWidget);
    });

    testWidgets('recorder does not start: message, back to the choice', (
      tester,
    ) async {
      final recorder = FakeVoiceRecorder()..failStart = true;
      await goToUpload(tester, fastMock(), recorder);

      await tester.tap(voiceButton());
      await tester.pumpAndSettle();

      expect(
        find.text(
          'Die Aufnahme konnte nicht gestartet werden. '
          'Bitte noch einmal versuchen.',
        ),
        findsOneWidget,
      );
      expect(recorder.hasFile, isFalse);
      expect(voiceButton(), findsOneWidget);
    });
  });

  testWidgets('record, stop, check the text, edit, save: entry opens, '
      'recording deleted', (tester) async {
    final mock = MockApiClient(
      delay: Duration.zero,
      uploadDelay: Duration.zero,
      transcriptionDelay: const Duration(seconds: 2),
    );
    final recorder = FakeVoiceRecorder();
    await goToUpload(tester, mock, recorder);

    await startRecording(tester);
    expect(recorder.recording, isTrue);
    expect(find.text('0:00'), findsOneWidget);
    expect(find.text('Aufnahme läuft · höchstens 5 Minuten'), findsOneWidget);

    await tester.pump(const Duration(seconds: 7));
    expect(find.text('0:07'), findsOneWidget);

    // Stop: loading indicator while the backend transcribes.
    await tester.tap(stopButton());
    await tester.pump();
    await tester.pump(const Duration(milliseconds: 500));
    expect(find.textContaining('in Text umgewandelt'), findsOneWidget);
    expect(recorder.recording, isFalse);
    expect(recorder.hasFile, isTrue); // still needed for the upload

    await tester.pump(const Duration(seconds: 2));
    await tester.pumpAndSettle();

    // The text is shown for checking, the recording is gone.
    expect(recorder.hasFile, isFalse);
    expect(
      find.textContaining('Erkannter Text aus deiner Sprachnotiz'),
      findsOneWidget,
    );
    expect(
      tester.widget<TextField>(textField()).controller!.text,
      MockApiClient.sampleTranscript,
    );
    // Nothing is saved before the confirmation.
    expect(await mock.listEntries(module: 'health'), hasLength(3));

    const edited =
        '${MockApiClient.sampleTranscript} Kontrolle in zwei Wochen.';
    await tester.enterText(textField(), edited);
    await tester.pump();
    await tester.tap(saveButton());
    await tester.pumpAndSettle();

    expect(find.byType(EntryScreen), findsOneWidget);
    expect(find.text('Text gespeichert.'), findsOneWidget);
    final entries = await mock.listEntries(module: 'health');
    expect(entries, hasLength(4));
    final created = entries.firstWhere(
      (e) => ![labReportId, doctorLetterId, otherId].contains(e.id),
    );
    expect((await mock.getEntry(created.id)).fullText, edited);
  });

  testWidgets('„Abbrechen“ while recording: deleted, back to the choice', (
    tester,
  ) async {
    final mock = fastMock();
    final recorder = FakeVoiceRecorder();
    await goToUpload(tester, mock, recorder);

    await startRecording(tester);
    await tester.pump(const Duration(seconds: 3));
    expect(recorder.hasFile, isTrue);

    await tester.tap(find.text('Abbrechen'));
    await tester.pumpAndSettle();

    expect(recorder.hasFile, isFalse);
    expect(recorder.discards, greaterThan(0));
    expect(stopButton(), findsNothing);
    expect(voiceButton(), findsOneWidget);
    expect(await mock.listEntries(module: 'health'), hasLength(3));
  });

  testWidgets('stops on its own at 4:58 and transcribes', (tester) async {
    final recorder = FakeVoiceRecorder();
    await goToUpload(tester, fastMock(), recorder);

    await startRecording(tester);
    await tester.pump(const Duration(minutes: 4, seconds: 57));
    expect(find.text('4:57'), findsOneWidget);
    expect(recorder.recording, isTrue);

    await tester.pump(const Duration(seconds: 1));
    await tester.pumpAndSettle();

    expect(recorder.recording, isFalse);
    expect(recorder.hasFile, isFalse);
    expect(
      tester.widget<TextField>(textField()).controller!.text,
      MockApiClient.sampleTranscript,
    );
  });

  testWidgets('the system stops the recording (call): like „Stopp“', (
    tester,
  ) async {
    final recorder = FakeVoiceRecorder();
    await goToUpload(tester, fastMock(), recorder);

    await startRecording(tester);
    recorder.stopBySystem();
    await tester.pumpAndSettle();

    expect(recorder.hasFile, isFalse);
    expect(
      tester.widget<TextField>(textField()).controller!.text,
      MockApiClient.sampleTranscript,
    );
  });

  for (final (fileName, message) in [
    ('note_nospeech.m4a', 'In der Aufnahme ist keine Sprache zu hören.'),
    (
      'note_toolong.m4a',
      'Die Aufnahme ist länger als 5 Minuten. Bitte kürzer sprechen oder '
          'aufteilen.',
    ),
  ]) {
    testWidgets('$fileName: backend message, recording deleted', (
      tester,
    ) async {
      final recorder = FakeVoiceRecorder()..fileName = fileName;
      await goToUpload(tester, fastMock(), recorder);

      await startRecording(tester);
      await stopAndTranscribe(tester);

      expect(find.text(message), findsOneWidget);
      expect(recorder.hasFile, isFalse);
      // No chance of success with the same recording: back to the choice.
      expect(find.text('Erneut versuchen'), findsNothing);
      expect(voiceButton(), findsOneWidget);
      expect(textField(), findsNothing);
    });
  }

  testWidgets('speech recognition unreachable: recording kept, retry works', (
    tester,
  ) async {
    final mock = fastMock();
    final recorder = FakeVoiceRecorder();
    await goToUpload(tester, mock, recorder);

    await startRecording(tester);
    mock.speechStatus = SpeechStatus.unavailable; // fails from now on
    await stopAndTranscribe(tester);

    expect(
      find.textContaining('Spracherkennung ist gerade nicht erreichbar'),
      findsOneWidget,
    );
    expect(recorder.hasFile, isTrue);

    mock.speechStatus = SpeechStatus.ok;
    await tester.tap(find.text('Erneut versuchen'));
    await tester.pumpAndSettle();

    expect(recorder.hasFile, isFalse);
    expect(find.textContaining('nicht erreichbar'), findsNothing);
    expect(
      tester.widget<TextField>(textField()).controller!.text,
      MockApiClient.sampleTranscript,
    );
  });

  testWidgets('„Verwerfen“ after an error deletes the recording', (
    tester,
  ) async {
    final recorder = FakeVoiceRecorder()..fileName = 'note_modelerror.m4a';
    await goToUpload(tester, fastMock(), recorder);

    await startRecording(tester);
    await stopAndTranscribe(tester);
    expect(recorder.hasFile, isTrue);

    await tester.tap(find.text('Verwerfen'));
    await tester.pumpAndSettle();

    expect(recorder.hasFile, isFalse);
    expect(find.textContaining('nicht erreichbar'), findsNothing);
    expect(voiceButton(), findsOneWidget);
  });

  testWidgets('„Abbrechen“ on the recognized text saves nothing', (
    tester,
  ) async {
    final mock = fastMock();
    await goToUpload(tester, mock, FakeVoiceRecorder());

    await startRecording(tester);
    await stopAndTranscribe(tester);
    expect(saveButton(), findsOneWidget);

    await tester.tap(find.text('Abbrechen'));
    await tester.pumpAndSettle();

    expect(textField(), findsNothing);
    expect(voiceButton(), findsOneWidget);
    expect(await mock.listEntries(module: 'health'), hasLength(3));

    // „Text eingeben“ afterwards is the normal text import again.
    await tester.tap(find.text('Text eingeben'));
    await tester.pumpAndSettle();
    expect(find.textContaining('Erkannter Text'), findsNothing);
    expect(find.widgetWithText(FilledButton, 'Importieren'), findsOneWidget);
  });

  testWidgets('leaving the module while recording deletes the recording', (
    tester,
  ) async {
    final recorder = FakeVoiceRecorder();
    await goToUpload(tester, fastMock(), recorder);

    await startRecording(tester);
    expect(recorder.hasFile, isTrue);

    await tester.tap(find.byTooltip('Bereiche'));
    await tester.pumpAndSettle();

    expect(find.byType(ModulesScreen), findsOneWidget);
    expect(recorder.hasFile, isFalse);
  });
}
