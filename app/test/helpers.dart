import 'dart:async';
import 'dart:typed_data';

import 'package:flutter/widgets.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:lifelog/api/api_client.dart';
import 'package:lifelog/api/api_exception.dart';
import 'package:lifelog/api/mock_api_client.dart';
import 'package:lifelog/main.dart';
import 'package:lifelog/module/module_catalog.dart';
import 'package:lifelog/platform/file_chooser.dart';
import 'package:lifelog/platform/voice_recorder.dart';

/// Tall test screen, so that long lists are built completely
/// (a ListView only builds what is visible).
void useLargeScreen(WidgetTester tester) {
  tester.view.physicalSize = const Size(800, 2400);
  tester.view.devicePixelRatio = 1.0;
  addTearDown(tester.view.reset);
}

/// Taps the back arrow. `tester.pageBack()` looks for the English tooltip
/// „Back“, but the app is German („Zurück“).
Future<void> tapBack(WidgetTester tester) async {
  await tester.tap(find.byTooltip('Zurück'));
  await tester.pumpAndSettle();
}

/// The app starts on the „Bereiche“ screen; this opens health.
Future<void> openHealth(WidgetTester tester) async {
  await tester.tap(find.text('Gesundheit'));
  await tester.pumpAndSettle();
}

/// The app with a mock backend and a fake file chooser. Without [catalog]
/// the default modules apply.
LifelogApp testApp(
  ApiClient api, {
  FileChooser? chooser,
  VoiceRecorder? recorder,
  ModuleCatalog? catalog,
}) => LifelogApp(
  api: api,
  fileChooser: chooser ?? FakeFileChooser(),
  voiceRecorder: recorder ?? FakeVoiceRecorder(),
  catalog: catalog ?? ModuleCatalog.defaults(),
);

/// Records nothing; it pretends a file exists between [start] and
/// [discard], so tests can check that the recording gets deleted.
class FakeVoiceRecorder implements VoiceRecorder {
  /// What [requestPermission] answers.
  bool permission = true;

  /// If `true`, [start] fails like a broken microphone.
  bool failStart = false;

  /// File name of the recording; mock triggers such as `nospeech` go here.
  String fileName = 'note_1.m4a';

  /// `true` while a recording file would be on the phone.
  bool hasFile = false;

  /// `true` between [start] and [stop].
  bool recording = false;

  var permissionRequests = 0;
  var starts = 0;
  var discards = 0;

  final _stoppedBySystem = StreamController<void>.broadcast();

  /// Simulates a phone call that pauses the recording.
  void stopBySystem() => _stoppedBySystem.add(null);

  @override
  Stream<void> get stoppedBySystem => _stoppedBySystem.stream;

  @override
  Future<bool> requestPermission() async {
    permissionRequests++;
    return permission;
  }

  @override
  Future<void> start() async {
    if (failStart) throw const RecorderException('no microphone');
    starts++;
    hasFile = true;
    recording = true;
  }

  @override
  Future<Recording> stop() async {
    if (!hasFile) throw const RecorderException('no recording');
    recording = false;
    return Recording(bytes: testM4a('ton'), fileName: fileName);
  }

  @override
  Future<void> discard() async {
    discards++;
    hasFile = false;
    recording = false;
  }
}

/// Returns given files instead of opening the camera and file picker.
/// `null` stands for "the user cancels".
class FakeFileChooser implements FileChooser {
  ChosenFile? photo;
  ChosenFile? image;
  ChosenFile? pdf;
  ChosenFile? lost;

  @override
  Future<ChosenFile?> takePhoto() async => photo;

  @override
  Future<ChosenFile?> pickImage() async => image;

  @override
  Future<ChosenFile?> pickPdf() async => pdf;

  @override
  Future<ChosenFile?> retrieveLostFile() async => lost;
}

/// A small "PDF": the magic bytes are right, the rest is arbitrary.
ChosenFile testPdf(String name, [String content = '']) => ChosenFile(
  bytes: Uint8List.fromList('%PDF-1.4 $name $content'.codeUnits),
  name: name,
);

/// A small "M4A recording": an MP4 box `ftyp` with the brand `M4A `, as the
/// package record writes it, then arbitrary bytes.
Uint8List testM4a([String content = '']) => Uint8List.fromList([
  0, 0, 0, 0x18, ...'ftypM4A '.codeUnits, 0, 0, 0, 0, ...'isom'.codeUnits, //
  ...'mp42'.codeUnits, ...content.codeUnits,
]);

/// Expects a future to fail with an [ApiException] with this code.
Matcher throwsApiException(String code, {int? status}) => throwsA(
  isA<ApiException>()
      .having((f) => f.code, 'code', code)
      .having((f) => f.httpStatus, 'httpStatus', status ?? anything),
);

/// Mock without artificial delay and with a fixed clock.
MockApiClient fastMock({
  bool withSampleData = true,
  DateTime Function()? clock,
  ModuleCatalog? catalog,
}) => MockApiClient(
  withSampleData: withSampleData,
  delay: Duration.zero,
  uploadDelay: Duration.zero,
  transcriptionDelay: Duration.zero,
  clock: clock ?? () => DateTime.utc(2026, 11, 4, 9, 30, 0),
  catalog: catalog,
);

const labReportId = '6727a1f3c2b4e5d6f7a8b9c0';
const doctorLetterId = '6727a20bc2b4e5d6f7a8b9d4';
const otherId = '6727a2a9c2b4e5d6f7a8b9e7';
