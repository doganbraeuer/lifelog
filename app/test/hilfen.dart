import 'dart:typed_data';

import 'package:flutter/widgets.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:lifelog/api/api_client.dart';
import 'package:lifelog/api/api_fehler.dart';
import 'package:lifelog/api/mock_api_client.dart';
import 'package:lifelog/main.dart';
import 'package:lifelog/plattform/datei_auswahl.dart';

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

/// Die App mit Mock-Backend und Fake-Dateiauswahl.
LifelogApp testApp(ApiClient api, {DateiAuswahl? auswahl}) =>
    LifelogApp(api: api, dateiAuswahl: auswahl ?? FakeDateiAuswahl());

/// Liefert vorgegebene Dateien statt Kamera und Dateiauswahl zu öffnen.
/// `null` steht für „Nutzer bricht ab“.
class FakeDateiAuswahl implements DateiAuswahl {
  AusgewaehlteDatei? foto;
  AusgewaehlteDatei? bild;
  AusgewaehlteDatei? pdf;
  AusgewaehlteDatei? verloren;

  @override
  Future<AusgewaehlteDatei?> fotoAufnehmen() async => foto;

  @override
  Future<AusgewaehlteDatei?> bildAuswaehlen() async => bild;

  @override
  Future<AusgewaehlteDatei?> pdfAuswaehlen() async => pdf;

  @override
  Future<AusgewaehlteDatei?> verloreneDatei() async => verloren;
}

/// Ein kleines „PDF“: Die Magic Bytes stimmen, der Rest ist beliebig.
AusgewaehlteDatei testPdf(String name, [String inhalt = '']) =>
    AusgewaehlteDatei(
      bytes: Uint8List.fromList('%PDF-1.4 $name $inhalt'.codeUnits),
      name: name,
    );

/// Erwartet, dass ein Future mit einem [ApiFehler] mit diesem Code scheitert.
Matcher wirftApiFehler(String code, {int? status}) => throwsA(
  isA<ApiFehler>()
      .having((f) => f.code, 'code', code)
      .having((f) => f.httpStatus, 'httpStatus', status ?? anything),
);

/// Mock ohne künstliche Wartezeit und mit fester Uhr.
MockApiClient schnellerMock({
  bool mitBeispieldaten = true,
  DateTime Function()? uhr,
}) => MockApiClient(
  mitBeispieldaten: mitBeispieldaten,
  verzoegerung: Duration.zero,
  uploadVerzoegerung: Duration.zero,
  uhr: uhr ?? () => DateTime.utc(2026, 11, 4, 9, 30, 0),
);

const laborId = '6727a1f3c2b4e5d6f7a8b9c0';
const arztbriefId = '6727a20bc2b4e5d6f7a8b9d4';
const sonstigesId = '6727a2a9c2b4e5d6f7a8b9e7';
