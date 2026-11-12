import 'package:flutter/material.dart';
import 'package:flutter/services.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:lifelog/api/mock_api_client.dart';
import 'package:lifelog/platform/pdf_viewer.dart';
import 'package:lifelog/screens/original_file_screen.dart';

import '../helpers.dart';

/// Statt pdfx: zeigt nur, wie viele Bytes ankamen und ob es ein PDF ist.
class FakePdfAnzeige implements PdfViewer {
  @override
  Widget buildView(Uint8List bytes) => Text(
    'PDF-Ansicht: ${bytes.length} Bytes, '
    '${String.fromCharCodes(bytes.take(5))}',
  );
}

Future<void> zeigen(
  WidgetTester tester,
  MockApiClient mock,
  String entryId,
  String fileName,
) async {
  await tester.pumpWidget(
    MaterialApp(
      home: OriginalFileScreen(
        api: mock,
        entryId: entryId,
        fileName: fileName,
        pdfViewer: FakePdfAnzeige(),
      ),
    ),
  );
  await tester.pumpAndSettle();
}

void main() {
  testWidgets('PDF wird an die PDF-Anzeige übergeben', (tester) async {
    final mock = schnellerMock();
    final size = (await mock.getFile(laborId)).bytes.length;
    await zeigen(tester, mock, laborId, 'befund_oktober.pdf');

    expect(find.text('befund_oktober.pdf'), findsOneWidget);
    expect(find.text('PDF-Ansicht: $size Bytes, %PDF-'), findsOneWidget);
  });

  testWidgets('Bild wird zoombar angezeigt', (tester) async {
    await zeigen(tester, schnellerMock(), arztbriefId, 'arztbrief.png');

    expect(find.byType(InteractiveViewer), findsOneWidget);
    expect(find.byType(Image), findsOneWidget);
    expect(find.textContaining('PDF-Ansicht'), findsNothing);
  });

  testWidgets('Fehler beim Laden: Meldung und erneuter Versuch', (
    tester,
  ) async {
    final mock = schnellerMock()..simulateError = true;
    await zeigen(tester, mock, laborId, 'befund_oktober.pdf');

    expect(find.textContaining('Fehler aufgetreten'), findsOneWidget);

    mock.simulateError = false;
    await tester.tap(find.text('Erneut versuchen'));
    await tester.pumpAndSettle();

    expect(find.textContaining('PDF-Ansicht'), findsOneWidget);
  });

  testWidgets('unbekannter Eintrag: 404-Meldung des Backends', (tester) async {
    await zeigen(
      tester,
      schnellerMock(),
      'ffffffffffffffffffffffff',
      'weg.pdf',
    );

    expect(find.text('Diesen Eintrag gibt es nicht.'), findsOneWidget);
  });

  group('aus der Eintrag-Ansicht', () {
    Future<void> zumArztbrief(WidgetTester tester) async {
      grosserBildschirm(tester);
      await tester.pumpWidget(testApp(schnellerMock()));
      await tester.pumpAndSettle();
      await zuGesundheit(tester);
      await tester.tap(find.text('Dr. Erika Beispiel, Allgemeinmedizin'));
      await tester.pumpAndSettle();
    }

    testWidgets('Knopf öffnet die Datei, Zurück führt zum Eintrag', (
      tester,
    ) async {
      await zumArztbrief(tester);

      await tester.tap(find.text('Originaldatei öffnen'));
      await tester.pumpAndSettle();
      expect(find.byType(InteractiveViewer), findsOneWidget);
      expect(find.text('arztbrief_september.png'), findsOneWidget);

      await zurueck(tester);
      expect(find.text('Erkannte Angaben'), findsOneWidget);
    });

    testWidgets('Tippen auf das Vorschaubild öffnet ebenfalls', (tester) async {
      await zumArztbrief(tester);

      await tester.tap(find.byType(Image));
      await tester.pumpAndSettle();

      expect(find.byType(InteractiveViewer), findsOneWidget);
    });

    testWidgets('auch PDF-Einträge haben den Knopf', (tester) async {
      grosserBildschirm(tester);
      await tester.pumpWidget(testApp(schnellerMock()));
      await tester.pumpAndSettle();
      await zuGesundheit(tester);
      await tester.tap(find.text('Labor Muster GmbH, Salzburg'));
      await tester.pumpAndSettle();

      expect(find.text('Originaldatei öffnen'), findsOneWidget);
    });
  });
}
