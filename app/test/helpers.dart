import 'dart:typed_data';

import 'package:flutter/widgets.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:lifelog/api/api_client.dart';
import 'package:lifelog/api/api_exception.dart';
import 'package:lifelog/api/mock_api_client.dart';
import 'package:lifelog/main.dart';
import 'package:lifelog/module/module_catalog.dart';
import 'package:lifelog/platform/file_chooser.dart';

/// Hoher Test-Bildschirm, damit lange Listen ganz gebaut werden
/// (eine ListView baut nur, was sichtbar ist).
void grosserBildschirm(WidgetTester tester) {
  tester.view.physicalSize = const Size(800, 2400);
  tester.view.devicePixelRatio = 1.0;
  addTearDown(tester.view.reset);
}

/// Tippt auf den Zurück-Pfeil. `tester.pageBack()` sucht den englischen
/// Tooltip „Back“, die App ist aber deutsch („Zurück“).
Future<void> zurueck(WidgetTester tester) async {
  await tester.tap(find.byTooltip('Zurück'));
  await tester.pumpAndSettle();
}

/// Die App startet im Screen „Bereiche“; das öffnet Gesundheit.
Future<void> zuGesundheit(WidgetTester tester) async {
  await tester.tap(find.text('Gesundheit'));
  await tester.pumpAndSettle();
}

/// Die App mit Mock-Backend und Fake-Dateiauswahl. Ohne [katalog] gelten
/// die Standard-Module.
LifelogApp testApp(
  ApiClient api, {
  FileChooser? auswahl,
  ModuleCatalog? catalog,
}) => LifelogApp(
  api: api,
  dateiAuswahl: auswahl ?? FakeDateiAuswahl(),
  catalog: catalog ?? ModuleCatalog.defaults(),
);

/// Liefert vorgegebene Dateien statt Kamera und Dateiauswahl zu öffnen.
/// `null` steht für „Nutzer bricht ab“.
class FakeDateiAuswahl implements FileChooser {
  ChosenFile? foto;
  ChosenFile? bild;
  ChosenFile? pdf;
  ChosenFile? verloren;

  @override
  Future<ChosenFile?> takePhoto() async => foto;

  @override
  Future<ChosenFile?> pickImage() async => bild;

  @override
  Future<ChosenFile?> pickPdf() async => pdf;

  @override
  Future<ChosenFile?> retrieveLostFile() async => verloren;
}

/// Ein kleines „PDF“: Die Magic Bytes stimmen, der Rest ist beliebig.
ChosenFile testPdf(String name, [String inhalt = '']) => ChosenFile(
  bytes: Uint8List.fromList('%PDF-1.4 $name $inhalt'.codeUnits),
  name: name,
);

/// Erwartet, dass ein Future mit einem [ApiFehler] mit diesem Code scheitert.
Matcher wirftApiFehler(String code, {int? status}) => throwsA(
  isA<ApiException>()
      .having((f) => f.code, 'code', code)
      .having((f) => f.httpStatus, 'httpStatus', status ?? anything),
);

/// Mock ohne künstliche Wartezeit und mit fester Uhr.
MockApiClient schnellerMock({
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

const laborId = '6727a1f3c2b4e5d6f7a8b9c0';
const arztbriefId = '6727a20bc2b4e5d6f7a8b9d4';
const sonstigesId = '6727a2a9c2b4e5d6f7a8b9e7';
