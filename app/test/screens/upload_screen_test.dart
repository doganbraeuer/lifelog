import 'package:flutter/material.dart';
import 'package:flutter/services.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:lifelog/api/mock/mock_data.dart' show sampleCount;
import 'package:lifelog/api/api_client.dart';
import 'package:lifelog/api/mock_api_client.dart';
import 'package:lifelog/platform/file_chooser.dart';
import 'package:lifelog/screens/upload_screen.dart';

import '../helpers.dart';

/// Starts the app and switches to the „Hochladen“ page.
Future<void> goToUpload(
  WidgetTester tester,
  MockApiClient mock,
  FakeFileChooser chooser,
) async {
  useLargeScreen(tester);
  await tester.pumpWidget(testApp(mock, chooser: chooser));
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

/// The upload button; the text is also in the navigation bar and the app
/// bar, so search by the button type.
Finder uploadButton() => find.widgetWithText(FilledButton, 'Hochladen');

void main() {
  testWidgets('shows the cloud notice and the three choice buttons', (
    tester,
  ) async {
    await goToUpload(tester, fastMock(), FakeFileChooser());

    expect(find.text(cloudNotice), findsOneWidget);
    expect(find.text('Foto aufnehmen'), findsOneWidget);
    expect(find.text('Bild auswählen'), findsOneWidget);
    expect(find.text('PDF auswählen'), findsOneWidget);
  });

  testWidgets('cancelling the choice changes nothing', (tester) async {
    await goToUpload(tester, fastMock(), FakeFileChooser());

    await tester.tap(find.text('Foto aufnehmen'));
    await tester.pumpAndSettle();

    expect(find.text('Foto aufnehmen'), findsOneWidget);
    expect(uploadButton(), findsNothing);
  });

  testWidgets('choose a PDF, check it, upload it, the entry opens', (
    tester,
  ) async {
    final mock = MockApiClient(
      delay: Duration.zero,
      uploadDelay: const Duration(seconds: 2),
    );
    final chooser = FakeFileChooser()..pdf = testPdf('befund.pdf');
    await goToUpload(tester, mock, chooser);

    await tester.tap(find.text('PDF auswählen'));
    await tester.pumpAndSettle();
    expect(find.text('befund.pdf'), findsOneWidget);
    expect(find.byTooltip('Datei verwerfen'), findsOneWidget);

    await tester.tap(uploadButton());
    await tester.pump(const Duration(milliseconds: 100));
    expect(find.textContaining('bis zu einer Minute'), findsOneWidget);
    // During the upload the file cannot be discarded.
    expect(find.byTooltip('Datei verwerfen'), findsNothing);

    await tester.pump(const Duration(seconds: 2));
    await tester.pumpAndSettle();

    // The new entry is open ...
    expect(find.text('Erkannte Angaben'), findsOneWidget);
    expect(find.text('Blutdruck'), findsOneWidget);

    // ... and after „Zurück“ it is in the timeline.
    await tapBack(tester);
    expect(find.text('Mock-Praxis Beispiel'), findsOneWidget);
    expect(
      await mock.listEntries(module: 'health'),
      hasLength(sampleCount + 1),
    );
  });

  testWidgets('a WebP image gets an image preview and can be uploaded', (
    tester,
  ) async {
    final mock = fastMock();
    final chooser = FakeFileChooser()..image = testWebp('befund.webp');
    await goToUpload(tester, mock, chooser);

    await tester.tap(find.text('Bild auswählen'));
    await tester.pumpAndSettle();
    expect(find.text('befund.webp'), findsOneWidget);
    // Shown as an image, not with the PDF icon.
    final preview = tester.widget<Image>(find.byType(Image));
    expect(preview.image, isA<MemoryImage>());
    expect(find.byIcon(Icons.picture_as_pdf_outlined), findsNothing);

    final before = {
      for (final e in await mock.listEntries(module: 'health')) e.id,
    };
    await tester.tap(uploadButton());
    await tester.pumpAndSettle();
    final added = (await mock.listEntries(module: 'health'))
        .where((e) => !before.contains(e.id))
        .single;
    expect((await mock.getEntry(added.id)).file.mimeType, 'image/webp');
  });

  testWidgets('X on the preview discards the file', (tester) async {
    final mock = fastMock();
    final chooser = FakeFileChooser()..pdf = testPdf('falsch.pdf');
    await goToUpload(tester, mock, chooser);

    await tester.tap(find.text('PDF auswählen'));
    await tester.pumpAndSettle();
    expect(find.text('falsch.pdf'), findsOneWidget);

    await tester.tap(find.byTooltip('Datei verwerfen'));
    await tester.pumpAndSettle();

    expect(find.text('falsch.pdf'), findsNothing);
    expect(find.text('PDF auswählen'), findsOneWidget);
    expect(await mock.listEntries(module: 'health'), hasLength(sampleCount));
  });

  testWidgets('duplicate: notice and jump to the existing entry', (
    tester,
  ) async {
    final mock = fastMock();
    final preloaded = await mock.upload(
      testPdf('doppelt.pdf').bytes,
      'doppelt.pdf',
      module: 'health',
    );
    final chooser = FakeFileChooser()..pdf = testPdf('doppelt.pdf');
    await goToUpload(tester, mock, chooser);

    await tester.tap(find.text('PDF auswählen'));
    await tester.pumpAndSettle();
    await tester.tap(uploadButton());
    await tester.pumpAndSettle();

    expect(find.text('Diese Datei hast du schon hochgeladen.'), findsOneWidget);
    // The file is discarded, the choice buttons are back.
    expect(find.text('PDF auswählen'), findsOneWidget);

    await tester.tap(find.text('Vorhandenen Eintrag öffnen'));
    await tester.pumpAndSettle();
    expect(find.text('Erkannte Angaben'), findsOneWidget);
    expect(find.textContaining(preloaded.file.fileName), findsWidgets);
  });

  testWidgets('over 10 MB: message without upload', (tester) async {
    final mock = fastMock();
    final large = Uint8List(maxFileSize + 1)..setAll(0, '%PDF-'.codeUnits);
    final chooser = FakeFileChooser()
      ..image = ChosenFile(bytes: large, name: 'riesig.pdf');
    await goToUpload(tester, mock, chooser);

    await tester.tap(find.text('Bild auswählen'));
    await tester.pumpAndSettle();

    expect(find.textContaining('größer als 10 MB'), findsOneWidget);
    expect(uploadButton(), findsNothing);
    expect(await mock.listEntries(module: 'health'), hasLength(sampleCount));
  });

  testWidgets('size already known when choosing (FileTooLargeException)', (
    tester,
  ) async {
    await goToUpload(tester, fastMock(), _TooLargeChooser());

    await tester.tap(find.text('PDF auswählen'));
    await tester.pumpAndSettle();

    expect(find.textContaining('größer als 10 MB'), findsOneWidget);
  });

  testWidgets('retryable error: the file stays, „Erneut versuchen“', (
    tester,
  ) async {
    final chooser = FakeFileChooser()..pdf = testPdf('modelerror.pdf');
    await goToUpload(tester, fastMock(), chooser);

    await tester.tap(find.text('PDF auswählen'));
    await tester.pumpAndSettle();
    await tester.tap(uploadButton());
    await tester.pumpAndSettle();

    expect(find.textContaining('nicht erreichbar'), findsOneWidget);
    expect(find.text('modelerror.pdf'), findsOneWidget);
    expect(find.text('Erneut versuchen'), findsOneWidget);
  });

  testWidgets('no access to the camera: understandable message', (
    tester,
  ) async {
    await goToUpload(tester, fastMock(), _NoAccessChooser());

    await tester.tap(find.text('Foto aufnehmen'));
    await tester.pumpAndSettle();

    expect(find.textContaining('Kein Zugriff'), findsOneWidget);
  });

  testWidgets('photo after a restart (Android) is offered again', (
    tester,
  ) async {
    final chooser = FakeFileChooser()..lost = testPdf('verloren.pdf');
    await goToUpload(tester, fastMock(), chooser);

    expect(find.text('verloren.pdf'), findsOneWidget);
    expect(uploadButton(), findsOneWidget);
  });
}

class _TooLargeChooser extends FakeFileChooser {
  @override
  Future<ChosenFile?> pickPdf() async =>
      throw const FileTooLargeException(20 * 1024 * 1024);
}

class _NoAccessChooser extends FakeFileChooser {
  @override
  Future<ChosenFile?> takePhoto() async =>
      throw PlatformException(code: 'camera_access_denied');
}
