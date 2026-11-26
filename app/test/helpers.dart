import 'dart:async';
import 'dart:convert';
import 'dart:typed_data';

import 'package:flutter/material.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:lifelog/api/api_client.dart';
import 'package:lifelog/api/api_exception.dart';
import 'package:lifelog/api/mock/mock_files.dart';
import 'package:lifelog/api/mock_api_client.dart';
import 'package:lifelog/main.dart';
import 'package:lifelog/module/module_catalog.dart';
import 'package:lifelog/platform/device_time_zone.dart';
import 'package:lifelog/platform/file_chooser.dart';
import 'package:lifelog/platform/pdf_viewer.dart';
import 'package:lifelog/platform/reminders.dart';
import 'package:lifelog/reminders/reminder_plan.dart';
import 'package:lifelog/platform/voice_recorder.dart';
import 'package:lifelog/session/local_data.dart';
import 'package:lifelog/session/session_store.dart';

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
///
/// Without [store] the app starts logged in as the mock's `admin`, who has
/// agreed to the current consent notice, so that it opens on the „Bereiche“
/// screen as before the login existed.
LifelogApp testApp(
  ApiClient api, {
  FileChooser? chooser,
  VoiceRecorder? recorder,
  ModuleCatalog? catalog,
  SessionStore? store,
  LocalData? localData,
  DeviceTimeZone? deviceTimeZone,
  Reminders? reminders,
  DateTime Function()? clock,
}) => LifelogApp(
  clock: clock ?? () => mockNow,
  api: api,
  fileChooser: chooser ?? FakeFileChooser(),
  voiceRecorder: recorder ?? FakeVoiceRecorder(),
  catalog: catalog ?? ModuleCatalog.defaults(),
  sessionStore: store ?? signedInStore(api),
  localData: localData ?? FakeLocalData(),
  deviceTimeZone: deviceTimeZone ?? FakeDeviceTimeZone(),
  reminders: reminders ?? FakeReminders(),
);

/// Remembers what the app planned instead of using the phone's
/// notifications. [permission] is what the phone would answer.
class FakeReminders implements Reminders {
  FakeReminders({this.permissionAnswer = ReminderPermission.granted});

  ReminderPermission permissionAnswer;

  /// What a request answers; `null`: the same as [permissionAnswer], after
  /// which [permissionAnswer] follows it.
  ReminderPermission? requestAnswer;

  void Function(ReminderTap)? onTap;
  ReminderTap? launchedBy;

  /// Every plan given to [apply], the last one at the end.
  final plans = <List<PlannedReminder>>[];
  final cancelled = <int>[];
  final shownNow = <PlannedReminder>[];
  var clears = 0;
  var requests = 0;
  var settingsOpened = 0;

  /// The last plan, or empty.
  List<PlannedReminder> get planned => plans.isEmpty ? const [] : plans.last;

  @override
  Future<void> initialize({required void Function(ReminderTap) onTap}) async =>
      this.onTap = onTap;

  @override
  Future<ReminderTap?> launchTap() async => launchedBy;

  @override
  Future<ReminderPermission> permission() async => permissionAnswer;

  @override
  Future<ReminderPermission> requestPermission() async {
    requests++;
    if (requestAnswer case final answer?) permissionAnswer = answer;
    return permissionAnswer;
  }

  @override
  Future<void> openSettings() async => settingsOpened++;

  @override
  Future<void> apply(
    List<PlannedReminder> plan, {
    required bool Function(ReminderTap) stillOpen,
  }) async => plans.add(plan);

  @override
  Future<void> cancel(int id) async => cancelled.add(id);

  @override
  Future<void> clear() async {
    clears++;
    plans.add(const []);
  }

  @override
  Future<void> showNow(PlannedReminder reminder) async =>
      shownNow.add(reminder);
}

/// A phone in Vienna unless the test says otherwise.
class FakeDeviceTimeZone implements DeviceTimeZone {
  FakeDeviceTimeZone([this.zone = 'Europe/Vienna']);

  /// What the phone reports; `null`: it does not tell.
  String? zone;
  var calls = 0;

  @override
  Future<String?> name() async {
    calls++;
    return zone;
  }
}

/// Field „Benutzername“ on the login screen.
Finder usernameField() => find.widgetWithText(TextField, 'Benutzername');

/// Field „Passwort“ on the login screen.
Finder passwordField() => find.widgetWithText(TextField, 'Passwort');

/// Starts the app without a token (login screen) and returns the store.
Future<FakeSessionStore> startSignedOut(
  WidgetTester tester,
  MockApiClient mock, {
  FakeLocalData? localData,
}) async {
  final store = FakeSessionStore();
  await tester.pumpWidget(testApp(mock, store: store, localData: localData));
  await tester.pumpAndSettle();
  return store;
}

/// Types username and password and taps „Anmelden“.
Future<void> logIn(
  WidgetTester tester,
  String username,
  String password,
) async {
  await tester.enterText(usernameField(), username);
  await tester.enterText(passwordField(), password);
  await tester.tap(find.widgetWithText(FilledButton, 'Anmelden'));
  await tester.pumpAndSettle();
}

/// Far in the future, for tokens that do not expire during a test.
final farFuture = DateTime.utc(2099);

/// A store with a valid session of [username] in the mock. With [consent]
/// the user has agreed to the current notice.
FakeSessionStore signedInStore(
  ApiClient api, {
  String username = 'admin',
  bool consent = true,
}) => FakeSessionStore(
  api is MockApiClient
      ? StoredSession(
          token: api.signInForTest(username, consent: consent),
          expiresAt: farFuture,
        )
      : null,
);

/// Keeps the session in memory instead of the secure storage. [session] is
/// visible, so tests can check that the token was stored or deleted.
class FakeSessionStore implements SessionStore {
  FakeSessionStore([this.session]);

  StoredSession? session;
  var clears = 0;

  @override
  Future<StoredSession?> read() async => session;

  @override
  Future<void> write(StoredSession session) async => this.session = session;

  @override
  Future<void> clear() async {
    clears++;
    session = null;
  }
}

/// Counts how often the local data would have been deleted.
class FakeLocalData implements LocalData {
  var clears = 0;

  @override
  Future<void> clear() async => clears++;
}

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

/// Instead of pdfx (which needs Android or iOS): the view only shows how
/// many bytes arrived and whether it is a PDF; a rendered page is a small
/// grey PNG in A4 proportions. [renderedPages] records which pages were
/// asked for.
class FakePdfViewer implements PdfViewer {
  /// Number of pages of every PDF.
  int pageCount = 2;

  final renderedPages = <int>[];

  @override
  Widget buildView(Uint8List bytes) => Text(
    'PDF-Ansicht: ${bytes.length} Bytes, '
    '${String.fromCharCodes(bytes.take(5))}',
  );

  @override
  Future<RenderedPdfPage> renderPage(Uint8List bytes, int page) async {
    renderedPages.add(page);
    if (page < 1 || page > pageCount) {
      throw RangeError.range(page, 1, pageCount, 'page');
    }
    return RenderedPdfPage(
      image: createMockPng(width: 60, height: 85),
      width: 595,
      height: 842,
      pageCount: pageCount,
    );
  }
}

/// A small "PDF": the magic bytes are right, the rest is arbitrary.
ChosenFile testPdf(String name, [String content = '']) => ChosenFile(
  bytes: Uint8List.fromList('%PDF-1.4 $name $content'.codeUnits),
  name: name,
);

/// A real, decodable WebP image (1×1 pixel, lossy VP8).
Uint8List testWebpBytes() =>
    base64Decode('UklGRiIAAABXRUJQVlA4IBYAAAAwAQCdASoBAAEADsD+JaQAA3AAAAAA');

/// [testWebpBytes] as a chosen file.
ChosenFile testWebp(String name) =>
    ChosenFile(bytes: testWebpBytes(), name: name);

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

/// The fixed clock of [fastMock]: Wednesday 2026-11-04, 10:30 in Vienna.
final mockNow = DateTime.utc(2026, 11, 4, 9, 30, 0);

/// Mock without artificial delay and with a fixed clock.
///
/// With [signedIn] (default) the mock already uses a token of `admin`, so
/// that direct calls such as `listEntries` work as before the login
/// existed. Tests of the login pass `signedIn: false`.
MockApiClient fastMock({
  bool withSampleData = true,
  DateTime Function()? clock,
  ModuleCatalog? catalog,
  bool signedIn = true,
}) {
  final mock = MockApiClient(
    withSampleData: withSampleData,
    delay: Duration.zero,
    uploadDelay: Duration.zero,
    transcriptionDelay: Duration.zero,
    clock: clock ?? () => mockNow,
    catalog: catalog,
  );
  if (signedIn) mock.useToken(mock.signInForTest('admin'));
  return mock;
}

const labReportId = '6727a1f3c2b4e5d6f7a8b9c0';
const doctorLetterId = '6727a20bc2b4e5d6f7a8b9d4';
const otherId = '6727a2a9c2b4e5d6f7a8b9e7';

// Further sample entries for the cases of the verification.
const noteId = '6727a301c2b4e5d6f7a8b9f1';
const prescriptionId = '6727a2f0c2b4e5d6f7a8b9f4';
const xRayId = '6727a2c4c2b4e5d6f7a8b9f6';
const oldLabId = '6727a18ec2b4e5d6f7a8b9f8';
