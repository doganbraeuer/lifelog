import 'package:flutter/material.dart';
import 'package:flutter/services.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:lifelog/api/api_client.dart';
import 'package:lifelog/api/mock_api_client.dart';
import 'package:lifelog/plattform/datei_auswahl.dart';
import 'package:lifelog/screens/upload_screen.dart';

import '../hilfen.dart';

/// Startet die App und wechselt zum Bereich „Hochladen“.
Future<void> zumUpload(
  WidgetTester tester,
  MockApiClient mock,
  FakeDateiAuswahl auswahl,
) async {
  grosserBildschirm(tester);
  await tester.pumpWidget(testApp(mock, auswahl: auswahl));
  await tester.pumpAndSettle();
  await zuGesundheit(tester);
  await tester.tap(
    find.descendant(
      of: find.byType(NavigationBar),
      matching: find.text('Hochladen'),
    ),
  );
  await tester.pumpAndSettle();
}

/// Hochladen-Knopf; der Text steht auch in der Navigationsleiste und der
/// AppBar, deshalb über den Knopf-Typ suchen.
Finder hochladenKnopf() => find.widgetWithText(FilledButton, 'Hochladen');

void main() {
  testWidgets('zeigt den Cloud-Hinweis und die drei Auswahlknöpfe', (
    tester,
  ) async {
    await zumUpload(tester, schnellerMock(), FakeDateiAuswahl());

    expect(find.text(cloudHinweis), findsOneWidget);
    expect(find.text('Foto aufnehmen'), findsOneWidget);
    expect(find.text('Bild auswählen'), findsOneWidget);
    expect(find.text('PDF auswählen'), findsOneWidget);
  });

  testWidgets('Abbrechen der Auswahl ändert nichts', (tester) async {
    await zumUpload(tester, schnellerMock(), FakeDateiAuswahl());

    await tester.tap(find.text('Foto aufnehmen'));
    await tester.pumpAndSettle();

    expect(find.text('Foto aufnehmen'), findsOneWidget);
    expect(hochladenKnopf(), findsNothing);
  });

  testWidgets('PDF wählen, prüfen, hochladen, Eintrag öffnet sich', (
    tester,
  ) async {
    final mock = MockApiClient(
      verzoegerung: Duration.zero,
      uploadVerzoegerung: const Duration(seconds: 2),
    );
    final auswahl = FakeDateiAuswahl()..pdf = testPdf('befund.pdf');
    await zumUpload(tester, mock, auswahl);

    await tester.tap(find.text('PDF auswählen'));
    await tester.pumpAndSettle();
    expect(find.text('befund.pdf'), findsOneWidget);
    expect(find.byTooltip('Datei verwerfen'), findsOneWidget);

    await tester.tap(hochladenKnopf());
    await tester.pump(const Duration(milliseconds: 100));
    expect(find.textContaining('bis zu einer Minute'), findsOneWidget);
    // Während des Uploads lässt sich die Datei nicht verwerfen.
    expect(find.byTooltip('Datei verwerfen'), findsNothing);

    await tester.pump(const Duration(seconds: 2));
    await tester.pumpAndSettle();

    // Der neue Eintrag ist offen ...
    expect(find.text('Erkannte Angaben'), findsOneWidget);
    expect(find.text('Blutdruck'), findsOneWidget);

    // ... und nach „Zurück“ steht er in der Zeitleiste.
    await zurueck(tester);
    expect(find.text('Mock-Praxis Beispiel'), findsOneWidget);
    expect(await mock.eintraege(modul: 'gesundheit'), hasLength(4));
  });

  testWidgets('X auf der Vorschau verwirft die Datei', (tester) async {
    final mock = schnellerMock();
    final auswahl = FakeDateiAuswahl()..pdf = testPdf('falsch.pdf');
    await zumUpload(tester, mock, auswahl);

    await tester.tap(find.text('PDF auswählen'));
    await tester.pumpAndSettle();
    expect(find.text('falsch.pdf'), findsOneWidget);

    await tester.tap(find.byTooltip('Datei verwerfen'));
    await tester.pumpAndSettle();

    expect(find.text('falsch.pdf'), findsNothing);
    expect(find.text('PDF auswählen'), findsOneWidget);
    expect(await mock.eintraege(modul: 'gesundheit'), hasLength(3));
  });

  testWidgets('Duplikat: Hinweis und Sprung zum vorhandenen Eintrag', (
    tester,
  ) async {
    final mock = schnellerMock();
    final vorhanden = await mock.hochladen(
      testPdf('doppelt.pdf').bytes,
      'doppelt.pdf',
      modul: 'gesundheit',
    );
    final auswahl = FakeDateiAuswahl()..pdf = testPdf('doppelt.pdf');
    await zumUpload(tester, mock, auswahl);

    await tester.tap(find.text('PDF auswählen'));
    await tester.pumpAndSettle();
    await tester.tap(hochladenKnopf());
    await tester.pumpAndSettle();

    expect(find.text('Diese Datei hast du schon hochgeladen.'), findsOneWidget);
    // Die Datei ist verworfen, die Auswahlknöpfe sind wieder da.
    expect(find.text('PDF auswählen'), findsOneWidget);

    await tester.tap(find.text('Vorhandenen Eintrag öffnen'));
    await tester.pumpAndSettle();
    expect(find.text('Erkannte Angaben'), findsOneWidget);
    expect(find.textContaining(vorhanden.datei.dateiname), findsWidgets);
  });

  testWidgets('über 10 MB: Meldung ohne Upload', (tester) async {
    final mock = schnellerMock();
    final gross = Uint8List(maxDateiGroesse + 1)..setAll(0, '%PDF-'.codeUnits);
    final auswahl = FakeDateiAuswahl()
      ..bild = AusgewaehlteDatei(bytes: gross, name: 'riesig.pdf');
    await zumUpload(tester, mock, auswahl);

    await tester.tap(find.text('Bild auswählen'));
    await tester.pumpAndSettle();

    expect(find.textContaining('größer als 10 MB'), findsOneWidget);
    expect(hochladenKnopf(), findsNothing);
    expect(await mock.eintraege(modul: 'gesundheit'), hasLength(3));
  });

  testWidgets('Größe schon bei der Auswahl bekannt (DateiZuGross)', (
    tester,
  ) async {
    await zumUpload(tester, schnellerMock(), _ZuGrosseAuswahl());

    await tester.tap(find.text('PDF auswählen'));
    await tester.pumpAndSettle();

    expect(find.textContaining('größer als 10 MB'), findsOneWidget);
  });

  testWidgets('wiederholbarer Fehler: Datei bleibt, „Erneut versuchen“', (
    tester,
  ) async {
    final auswahl = FakeDateiAuswahl()..pdf = testPdf('modellfehler.pdf');
    await zumUpload(tester, schnellerMock(), auswahl);

    await tester.tap(find.text('PDF auswählen'));
    await tester.pumpAndSettle();
    await tester.tap(hochladenKnopf());
    await tester.pumpAndSettle();

    expect(find.textContaining('nicht erreichbar'), findsOneWidget);
    expect(find.text('modellfehler.pdf'), findsOneWidget);
    expect(find.text('Erneut versuchen'), findsOneWidget);
  });

  testWidgets('Kein Zugriff auf die Kamera: verständliche Meldung', (
    tester,
  ) async {
    await zumUpload(tester, schnellerMock(), _KeinZugriffAuswahl());

    await tester.tap(find.text('Foto aufnehmen'));
    await tester.pumpAndSettle();

    expect(find.textContaining('Kein Zugriff'), findsOneWidget);
  });

  testWidgets('Foto nach Neustart (Android) wird wieder angeboten', (
    tester,
  ) async {
    final auswahl = FakeDateiAuswahl()..verloren = testPdf('verloren.pdf');
    await zumUpload(tester, schnellerMock(), auswahl);

    expect(find.text('verloren.pdf'), findsOneWidget);
    expect(hochladenKnopf(), findsOneWidget);
  });
}

class _ZuGrosseAuswahl extends FakeDateiAuswahl {
  @override
  Future<AusgewaehlteDatei?> pdfAuswaehlen() async =>
      throw const DateiZuGross(20 * 1024 * 1024);
}

class _KeinZugriffAuswahl extends FakeDateiAuswahl {
  @override
  Future<AusgewaehlteDatei?> fotoAufnehmen() async =>
      throw PlatformException(code: 'camera_access_denied');
}
