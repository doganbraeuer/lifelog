import 'dart:typed_data';

import 'package:flutter/widgets.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:lifelog/api/api_client.dart';
import 'package:lifelog/api/api_exception.dart';
import 'package:lifelog/api/mock_api_client.dart';
import 'package:lifelog/main.dart';
import 'package:lifelog/module/module_catalog.dart';
import 'package:lifelog/platform/file_chooser.dart';

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
  ModuleCatalog? catalog,
}) => LifelogApp(
  api: api,
  fileChooser: chooser ?? FakeFileChooser(),
  catalog: catalog ?? ModuleCatalog.defaults(),
);

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
  clock: clock ?? () => DateTime.utc(2026, 11, 4, 9, 30, 0),
  catalog: catalog,
);

const labReportId = '6727a1f3c2b4e5d6f7a8b9c0';
const doctorLetterId = '6727a20bc2b4e5d6f7a8b9d4';
const otherId = '6727a2a9c2b4e5d6f7a8b9e7';
