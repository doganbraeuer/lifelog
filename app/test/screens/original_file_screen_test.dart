import 'package:flutter/material.dart';
import 'package:flutter/services.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:lifelog/api/mock_api_client.dart';
import 'package:lifelog/platform/pdf_viewer.dart';
import 'package:lifelog/screens/original_file_screen.dart';

import '../helpers.dart';

/// Instead of pdfx: only shows how many bytes arrived and whether it is a PDF.
class FakePdfViewer implements PdfViewer {
  @override
  Widget buildView(Uint8List bytes) => Text(
    'PDF-Ansicht: ${bytes.length} Bytes, '
    '${String.fromCharCodes(bytes.take(5))}',
  );
}

Future<void> showOriginal(
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
        pdfViewer: FakePdfViewer(),
      ),
    ),
  );
  await tester.pumpAndSettle();
}

void main() {
  testWidgets('PDF is passed to the PDF viewer', (tester) async {
    final mock = fastMock();
    final size = (await mock.getFile(labReportId)).bytes.length;
    await showOriginal(tester, mock, labReportId, 'befund_oktober.pdf');

    expect(find.text('befund_oktober.pdf'), findsOneWidget);
    expect(find.text('PDF-Ansicht: $size Bytes, %PDF-'), findsOneWidget);
  });

  testWidgets('image is shown zoomable', (tester) async {
    await showOriginal(tester, fastMock(), doctorLetterId, 'arztbrief.png');

    expect(find.byType(InteractiveViewer), findsOneWidget);
    expect(find.byType(Image), findsOneWidget);
    expect(find.textContaining('PDF-Ansicht'), findsNothing);
  });

  testWidgets('error while loading: message and retry', (tester) async {
    final mock = fastMock()..simulateError = true;
    await showOriginal(tester, mock, labReportId, 'befund_oktober.pdf');

    expect(find.textContaining('Fehler aufgetreten'), findsOneWidget);

    mock.simulateError = false;
    await tester.tap(find.text('Erneut versuchen'));
    await tester.pumpAndSettle();

    expect(find.textContaining('PDF-Ansicht'), findsOneWidget);
  });

  testWidgets('unknown entry: 404 message of the backend', (tester) async {
    await showOriginal(
      tester,
      fastMock(),
      'ffffffffffffffffffffffff',
      'weg.pdf',
    );

    expect(find.text('Diesen Eintrag gibt es nicht.'), findsOneWidget);
  });

  group('from the entry view', () {
    Future<void> openDoctorLetter(WidgetTester tester) async {
      useLargeScreen(tester);
      await tester.pumpWidget(testApp(fastMock()));
      await tester.pumpAndSettle();
      await openHealth(tester);
      await tester.tap(find.text('Dr. Erika Beispiel, Allgemeinmedizin'));
      await tester.pumpAndSettle();
    }

    testWidgets('button opens the file, back leads to the entry', (
      tester,
    ) async {
      await openDoctorLetter(tester);

      await tester.tap(find.text('Originaldatei öffnen'));
      await tester.pumpAndSettle();
      expect(find.byType(InteractiveViewer), findsOneWidget);
      expect(find.text('arztbrief_september.png'), findsOneWidget);

      await tapBack(tester);
      expect(find.text('Erkannte Angaben'), findsOneWidget);
    });

    testWidgets('tapping the preview image opens it as well', (tester) async {
      await openDoctorLetter(tester);

      await tester.tap(find.byType(Image));
      await tester.pumpAndSettle();

      expect(find.byType(InteractiveViewer), findsOneWidget);
    });

    testWidgets('PDF entries have the button too', (tester) async {
      useLargeScreen(tester);
      await tester.pumpWidget(testApp(fastMock()));
      await tester.pumpAndSettle();
      await openHealth(tester);
      await tester.tap(find.text('Labor Muster GmbH, Salzburg'));
      await tester.pumpAndSettle();

      expect(find.text('Originaldatei öffnen'), findsOneWidget);
    });
  });
}
