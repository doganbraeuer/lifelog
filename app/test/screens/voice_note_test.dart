import 'package:flutter/material.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:lifelog/api/api_exception.dart';
import 'package:lifelog/api/mock/mock_data.dart' show sampleCount, sampleData;
import 'package:lifelog/api/mock_api_client.dart';
import 'package:lifelog/model/server_status.dart';
import 'package:lifelog/screens/entry_screen.dart';
import 'package:lifelog/screens/modules_screen.dart';

import '../helpers.dart';

/// The microphone in the field „Dokumentinhalt“.
Finder micButton() => find.widgetWithIcon(IconButton, Icons.mic_none);

/// The old button of its own in the upload choice; it is gone.
Finder oldVoiceButton() => find.widgetWithText(OutlinedButton, 'Sprachnotiz');
Finder stopButton() => find.widgetWithText(FilledButton, 'Stopp');
Finder saveButton() => find.widgetWithText(FilledButton, 'Speichern');
Finder importButton() => find.widgetWithText(FilledButton, 'Importieren');
Finder textField() => find.widgetWithText(TextField, 'Dokumentinhalt');

const hintWithSpeech =
    'Erlaubt sind JPEG, PNG, HEIC, WebP und PDF bis 10 MB oder Text bis '
    '100 000 Zeichen. Unter „Text eingeben“ kannst du den Text auch '
    'diktieren (bis 5 Minuten).';
const hintWithoutSpeech =
    'Erlaubt sind JPEG, PNG, HEIC, WebP und PDF bis 10 MB oder Text bis 100 000 '
    'Zeichen.';
const unavailableNotice = 'Sprachnotizen sind gerade nicht verfügbar.';
const transcript = MockApiClient.sampleTranscript;

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

/// Opens the text screen („Text eingeben“).
Future<void> openText(WidgetTester tester) async {
  await tester.tap(find.text('Text eingeben'));
  await tester.pumpAndSettle();
}

/// The upload page, then the text screen.
Future<void> goToText(
  WidgetTester tester,
  MockApiClient mock,
  FakeVoiceRecorder recorder,
) async {
  await goToUpload(tester, mock, recorder);
  await openText(tester);
}

/// Taps the microphone and waits until the recording runs. No
/// pumpAndSettle while recording: the clock ticks and the microphone
/// pulses, so the screen never settles.
Future<void> startRecording(WidgetTester tester) async {
  await tester.tap(micButton());
  await tester.pump();
  await tester.pump(const Duration(milliseconds: 500));
}

/// Taps „Stopp“; with [fastMock] the transcription is done right away.
Future<void> stopAndTranscribe(WidgetTester tester) async {
  await tester.tap(stopButton());
  await tester.pumpAndSettle();
}

bool micEnabled(WidgetTester tester) =>
    tester.widget<IconButton>(micButton()).onPressed != null;

TextEditingController fieldController(WidgetTester tester) =>
    tester.widget<TextField>(textField()).controller!;

const quantityHint =
    'Zahlen und Einheiten sind markiert. Bitte besonders genau prüfen.';

/// The marked parts of the text field, as the field draws them.
List<String> markedInField(WidgetTester tester) {
  final editable = tester.state<EditableTextState>(
    find.descendant(of: textField(), matching: find.byType(EditableText)),
  );
  final marked = <String>[];
  editable.renderEditable.text!.visitChildren((span) {
    if (span is TextSpan &&
        span.text != null &&
        span.style?.backgroundColor != null) {
      marked.add(span.text!);
    }
    return true;
  });
  return marked;
}

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
    testWidgets('speech ok: no button of its own in the upload, hint names '
        'dictation, the microphone in the field works', (tester) async {
      await goToUpload(tester, fastMock(), FakeVoiceRecorder());

      expect(oldVoiceButton(), findsNothing);
      expect(micButton(), findsNothing);
      expect(find.text(hintWithSpeech), findsOneWidget);
      expect(find.text(unavailableNotice), findsNothing);

      await openText(tester);
      expect(micButton(), findsOneWidget);
      expect(micEnabled(tester), isTrue);
      expect(find.byTooltip('Sprachnotiz aufnehmen'), findsOneWidget);
      expect(find.text(unavailableNotice), findsNothing);
    });

    testWidgets('speech off: no microphone, hint without dictation', (
      tester,
    ) async {
      final mock = fastMock()..speechStatus = SpeechStatus.off;
      await goToUpload(tester, mock, FakeVoiceRecorder());

      expect(find.text(hintWithoutSpeech), findsOneWidget);
      await openText(tester);
      expect(micButton(), findsNothing);
      expect(find.text(unavailableNotice), findsNothing);
    });

    testWidgets('speech unavailable: microphone disabled, „Erneut prüfen“', (
      tester,
    ) async {
      final mock = fastMock()..speechStatus = SpeechStatus.unavailable;
      await goToUpload(tester, mock, FakeVoiceRecorder());
      expect(find.text(hintWithoutSpeech), findsOneWidget);
      // The notice belongs to the microphone, so it is in the text screen.
      expect(find.text(unavailableNotice), findsNothing);

      await openText(tester);
      expect(micEnabled(tester), isFalse);
      expect(find.text(unavailableNotice), findsOneWidget);

      // The speech recognition is back: checking again enables it.
      mock.speechStatus = SpeechStatus.ok;
      await tester.tap(find.text('Erneut prüfen'));
      await tester.pumpAndSettle();

      expect(micEnabled(tester), isTrue);
      expect(find.text(unavailableNotice), findsNothing);
    });

    testWidgets('status cannot be loaded: microphone disabled with notice', (
      tester,
    ) async {
      await goToText(tester, _StatusFailsMock(), FakeVoiceRecorder());

      expect(micEnabled(tester), isFalse);
      expect(find.text(unavailableNotice), findsOneWidget);
    });
  });

  group('microphone', () {
    testWidgets('permission denied: German message, nothing recorded', (
      tester,
    ) async {
      final recorder = FakeVoiceRecorder()..permission = false;
      await goToText(tester, fastMock(), recorder);

      await tester.tap(micButton());
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
      expect(textField(), findsOneWidget);
    });

    testWidgets('recorder does not start: message, back to the text', (
      tester,
    ) async {
      final recorder = FakeVoiceRecorder()..failStart = true;
      await goToText(tester, fastMock(), recorder);
      await tester.enterText(textField(), 'Schon getippt');

      await tester.tap(micButton());
      await tester.pumpAndSettle();

      expect(
        find.text(
          'Die Aufnahme konnte nicht gestartet werden. '
          'Bitte noch einmal versuchen.',
        ),
        findsOneWidget,
      );
      expect(recorder.hasFile, isFalse);
      expect(fieldController(tester).text, 'Schon getippt');
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
    await goToText(tester, mock, recorder);

    await startRecording(tester);
    expect(recorder.recording, isTrue);
    expect(find.text('0:00'), findsOneWidget);
    expect(find.text('Aufnahme läuft · höchstens 5 Minuten'), findsOneWidget);
    // The recording replaces the field; nothing can be saved meanwhile.
    expect(textField(), findsNothing);
    expect(importButton(), findsNothing);
    expect(saveButton(), findsNothing);

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
    expect(fieldController(tester).text, transcript);
    // Nothing is saved before the confirmation.
    expect(await mock.listEntries(module: 'health'), hasLength(sampleCount));

    const edited = '$transcript Kontrolle in zwei Wochen.';
    await tester.enterText(textField(), edited);
    await tester.pump();
    await tester.tap(saveButton());
    await tester.pumpAndSettle();

    expect(find.byType(EntryScreen), findsOneWidget);
    expect(find.text('Text gespeichert.'), findsOneWidget);
    final entries = await mock.listEntries(module: 'health');
    expect(entries, hasLength(sampleCount + 1));
    final created = entries.firstWhere(
      (e) => !{for (final s in sampleData()) s.json['id']}.contains(e.id),
    );
    expect((await mock.getEntry(created.id)).fullText, edited);
  });

  group('the recognized text goes into the field', () {
    testWidgets('at the cursor, with spaces around it', (tester) async {
      await goToText(tester, fastMock(), FakeVoiceRecorder());
      await tester.enterText(textField(), 'Arztbrief: Ende.');
      // Cursor after „Arztbrief:“.
      fieldController(tester).selection = const TextSelection.collapsed(
        offset: 10,
      );

      await startRecording(tester);
      await stopAndTranscribe(tester);

      expect(fieldController(tester).text, 'Arztbrief: $transcript Ende.');
      expect(
        fieldController(tester).selection,
        TextSelection.collapsed(offset: 11 + transcript.length),
      );
    });

    testWidgets('after typed text the cursor is at its end: a space', (
      tester,
    ) async {
      await goToText(tester, fastMock(), FakeVoiceRecorder());
      await tester.enterText(textField(), 'Arztbrief:');

      await startRecording(tester);
      await stopAndTranscribe(tester);

      expect(fieldController(tester).text, 'Arztbrief: $transcript');
    });

    testWidgets('without a cursor: at the end, on a new line', (tester) async {
      await goToText(tester, fastMock(), FakeVoiceRecorder());
      // Set without tapping the field: no cursor.
      fieldController(tester).text = 'Kopfzeile';
      expect(fieldController(tester).selection.isValid, isFalse);

      await startRecording(tester);
      await stopAndTranscribe(tester);

      expect(fieldController(tester).text, 'Kopfzeile\n$transcript');
    });

    testWidgets('dictating twice adds the second text, everything marked', (
      tester,
    ) async {
      await goToText(tester, fastMock(), FakeVoiceRecorder());
      await tester.enterText(textField(), 'Puls 64/min');

      await startRecording(tester);
      await stopAndTranscribe(tester);
      await startRecording(tester);
      await stopAndTranscribe(tester);

      expect(
        fieldController(tester).text,
        'Puls 64/min $transcript $transcript',
      );
      // The marks cover the whole field, the typed number too.
      expect(markedInField(tester).first, '64');
      expect(find.text(quantityHint), findsOneWidget);
      expect(saveButton(), findsOneWidget);
    });
  });

  testWidgets('„Abbrechen“ while recording: deleted, back to the text', (
    tester,
  ) async {
    final mock = fastMock();
    final recorder = FakeVoiceRecorder();
    await goToText(tester, mock, recorder);
    await tester.enterText(textField(), 'Schon getippt');

    await startRecording(tester);
    await tester.pump(const Duration(seconds: 3));
    expect(recorder.hasFile, isTrue);

    await tester.tap(find.text('Abbrechen'));
    await tester.pumpAndSettle();

    expect(recorder.hasFile, isFalse);
    expect(recorder.discards, greaterThan(0));
    expect(stopButton(), findsNothing);
    // The text is unchanged and still the normal text import.
    expect(fieldController(tester).text, 'Schon getippt');
    expect(find.textContaining('Erkannter Text'), findsNothing);
    expect(importButton(), findsOneWidget);
    expect(await mock.listEntries(module: 'health'), hasLength(sampleCount));
  });

  testWidgets('stops on its own at 4:58 and transcribes', (tester) async {
    final recorder = FakeVoiceRecorder();
    await goToText(tester, fastMock(), recorder);

    await startRecording(tester);
    await tester.pump(const Duration(minutes: 4, seconds: 57));
    expect(find.text('4:57'), findsOneWidget);
    expect(recorder.recording, isTrue);

    await tester.pump(const Duration(seconds: 1));
    await tester.pumpAndSettle();

    expect(recorder.recording, isFalse);
    expect(recorder.hasFile, isFalse);
    expect(fieldController(tester).text, transcript);
  });

  testWidgets('the system stops the recording (call): like „Stopp“', (
    tester,
  ) async {
    final recorder = FakeVoiceRecorder();
    await goToText(tester, fastMock(), recorder);

    await startRecording(tester);
    recorder.stopBySystem();
    await tester.pumpAndSettle();

    expect(recorder.hasFile, isFalse);
    expect(fieldController(tester).text, transcript);
  });

  testWidgets('the app goes to the background: like „Stopp“', (tester) async {
    final recorder = FakeVoiceRecorder();
    await goToText(tester, fastMock(), recorder);

    await startRecording(tester);
    tester.binding.handleAppLifecycleStateChanged(AppLifecycleState.inactive);
    tester.binding.handleAppLifecycleStateChanged(AppLifecycleState.hidden);
    await tester.pump();
    tester.binding.handleAppLifecycleStateChanged(AppLifecycleState.inactive);
    tester.binding.handleAppLifecycleStateChanged(AppLifecycleState.resumed);
    await tester.pumpAndSettle();

    expect(recorder.recording, isFalse);
    expect(recorder.hasFile, isFalse);
    expect(fieldController(tester).text, transcript);
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
      await goToText(tester, fastMock(), recorder);
      await tester.enterText(textField(), 'Schon getippt');

      await startRecording(tester);
      await stopAndTranscribe(tester);

      expect(find.text(message), findsOneWidget);
      expect(recorder.hasFile, isFalse);
      // No chance of success with the same recording: back to the text,
      // which is unchanged.
      expect(find.text('Erneut versuchen'), findsNothing);
      expect(fieldController(tester).text, 'Schon getippt');
      expect(micButton(), findsOneWidget);
    });
  }

  testWidgets('speech recognition unreachable: recording kept, retry works', (
    tester,
  ) async {
    final mock = fastMock();
    final recorder = FakeVoiceRecorder();
    await goToText(tester, mock, recorder);

    await startRecording(tester);
    mock.speechStatus = SpeechStatus.unavailable; // fails from now on
    await stopAndTranscribe(tester);

    expect(
      find.textContaining('Spracherkennung ist gerade nicht erreichbar'),
      findsOneWidget,
    );
    expect(recorder.hasFile, isTrue);
    // Still in the recording view: no saving meanwhile.
    expect(importButton(), findsNothing);

    mock.speechStatus = SpeechStatus.ok;
    await tester.tap(find.text('Erneut versuchen'));
    await tester.pumpAndSettle();

    expect(recorder.hasFile, isFalse);
    expect(find.textContaining('nicht erreichbar'), findsNothing);
    expect(fieldController(tester).text, transcript);
  });

  testWidgets('„Verwerfen“ after an error deletes the recording', (
    tester,
  ) async {
    final recorder = FakeVoiceRecorder()..fileName = 'note_modelerror.m4a';
    await goToText(tester, fastMock(), recorder);

    await startRecording(tester);
    await stopAndTranscribe(tester);
    expect(recorder.hasFile, isTrue);

    await tester.tap(find.text('Verwerfen'));
    await tester.pumpAndSettle();

    expect(recorder.hasFile, isFalse);
    expect(find.textContaining('nicht erreichbar'), findsNothing);
    expect(textField(), findsOneWidget);
  });

  testWidgets('„Abbrechen“ on the recognized text saves nothing', (
    tester,
  ) async {
    final mock = fastMock();
    await goToText(tester, mock, FakeVoiceRecorder());

    await startRecording(tester);
    await stopAndTranscribe(tester);
    expect(saveButton(), findsOneWidget);

    await tester.tap(find.text('Abbrechen'));
    await tester.pumpAndSettle();

    expect(textField(), findsNothing);
    expect(await mock.listEntries(module: 'health'), hasLength(sampleCount));

    // „Text eingeben“ afterwards is the normal text import again: no
    // notice, no marks.
    await openText(tester);
    expect(fieldController(tester).text, isEmpty);
    expect(find.textContaining('Erkannter Text'), findsNothing);
    expect(find.text(quantityHint), findsNothing);
    expect(importButton(), findsOneWidget);
    await tester.enterText(textField(), 'Ibuprofen 400 Milligramm');
    await tester.pump();
    expect(markedInField(tester), isEmpty);
  });

  testWidgets('recognized text: numbers and units are marked, new ones too', (
    tester,
  ) async {
    await goToText(tester, fastMock(), FakeVoiceRecorder());

    await startRecording(tester);
    await stopAndTranscribe(tester);

    expect(find.text(quantityHint), findsOneWidget);
    expect(markedInField(tester), [
      '12.11.2026',
      '400 Milligramm',
      '2,5 Milliunits pro Liter',
      '130/85 mmHg',
      '37,6 Grad',
    ]);

    // Marks follow the editing: a corrected unit, an added number.
    await tester.enterText(
      textField(),
      transcript
          .replaceFirst('Milliunits', 'mU')
          .replaceFirst('Grad.', 'Grad. Kontrolle in 14 Tagen.'),
    );
    await tester.pump();
    expect(markedInField(tester), contains('2,5 mU pro Liter'));
    expect(markedInField(tester), contains('14 Tagen'));
    expect(find.text(quantityHint), findsOneWidget);
  });

  testWidgets('the text import of typed text marks nothing', (tester) async {
    await goToText(tester, fastMock(), FakeVoiceRecorder());

    await tester.enterText(textField(), 'Ibuprofen 400 Milligramm');
    await tester.pump();

    expect(markedInField(tester), isEmpty);
    expect(find.text(quantityHint), findsNothing);
  });

  testWidgets('leaving the module while recording deletes the recording', (
    tester,
  ) async {
    final recorder = FakeVoiceRecorder();
    await goToText(tester, fastMock(), recorder);

    await startRecording(tester);
    expect(recorder.hasFile, isTrue);

    await tester.tap(find.byTooltip('Bereiche'));
    await tester.pumpAndSettle();

    expect(find.byType(ModulesScreen), findsOneWidget);
    expect(recorder.hasFile, isFalse);
  });
}
