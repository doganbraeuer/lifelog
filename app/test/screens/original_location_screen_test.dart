import 'dart:ui' as ui;

import 'package:flutter/material.dart';
import 'package:flutter_localizations/flutter_localizations.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:lifelog/api/mock/mock_files.dart';
import 'package:lifelog/api/mock_api_client.dart';
import 'package:lifelog/model/verification.dart';
import 'package:lifelog/module/module_catalog.dart';
import 'package:lifelog/screens/entry_screen.dart';
import 'package:lifelog/screens/original_file_screen.dart';
import 'package:lifelog/screens/original_location_screen.dart';
import 'package:lifelog/screens/verification_view.dart';
import 'package:lifelog/widgets/location_overlay.dart';

import '../helpers.dart';

/// Opens the entry [id] directly, in German (for the tooltip „Zurück“), with
/// a fake PDF viewer.
Future<FakePdfViewer> openEntry(
  WidgetTester tester,
  String id, {
  MockApiClient? api,
  FakePdfViewer? viewer,
}) async {
  useLargeScreen(tester);
  final pdf = viewer ?? FakePdfViewer();
  await tester.pumpWidget(
    ModuleCatalogScope(
      catalog: ModuleCatalog.defaults(),
      child: MaterialApp(
        locale: const Locale('de'),
        supportedLocales: const [Locale('de')],
        localizationsDelegates: GlobalMaterialLocalizations.delegates,
        home: EntryScreen(api: api ?? fastMock(), entryId: id, pdfViewer: pdf),
      ),
    ),
  );
  await tester.pumpAndSettle();
  return pdf;
}

/// Taps the row with the value [value].
Future<void> tapValue(WidgetTester tester, String value) async {
  await tester.tap(find.text(value));
  await tester.pumpAndSettle();
}

/// The painter of the marks on the visible page.
LocationMarksPainter marks(WidgetTester tester) =>
    tester
            .widget<CustomPaint>(
              find.byWidgetPredicate(
                (w) => w is CustomPaint && w.painter is LocationMarksPainter,
              ),
            )
            .painter!
        as LocationMarksPainter;

/// The highlighted pieces of the text of a text import.
List<String> highlighted(WidgetTester tester) {
  final text = tester.widget<SelectableText>(find.byType(SelectableText));
  final pieces = <String>[];
  text.textSpan!.visitChildren((span) {
    if (span is TextSpan && span.style?.backgroundColor != null) {
      pieces.add(span.text!);
    }
    return true;
  });
  return pieces;
}

/// Lets real image decoding run (it does not progress in fake time).
Future<void> decodeImages(WidgetTester tester, Finder until) async {
  for (var i = 0; i < 50 && until.evaluate().isEmpty; i++) {
    await tester.runAsync(
      () => Future<void>.delayed(const Duration(milliseconds: 20)),
    );
    await tester.pump();
  }
}

final jumpIcon = find.byIcon(Icons.find_in_page_outlined);

void main() {
  group('entry: which values can be tapped', () {
    testWidgets('exactly the values with a location carry the hint icon', (
      tester,
    ) async {
      final api = fastMock();
      final entry = await api.getEntry(labReportId);
      final withLocation = entry.verification.fields
          .where((f) => f.locations.isNotEmpty)
          .length;
      await openEntry(tester, labReportId, api: api);

      // Date, issuer, Hämoglobin, Leukozyten (corrected and found),
      // Befundart; not CRP, and never type or import date.
      expect(withLocation, 5);
      expect(jumpIcon, findsNWidgets(withLocation));
    });

    testWidgets('screen readers hear „Im Original zeigen“ only there', (
      tester,
    ) async {
      final semantics = tester.ensureSemantics();
      await openEntry(tester, labReportId);

      expect(
        tester.getSemantics(find.text('14,2 g/dl')),
        isSemantics(hint: showInOriginalHint, hasTapAction: true),
      );
      expect(
        tester.getSemantics(find.text('<0,5 mg/dl')),
        isNot(isSemantics(hint: showInOriginalHint)),
      );
      expect(
        tester.getSemantics(find.text('Laborbefund').last),
        isNot(isSemantics(hint: showInOriginalHint)),
      );
      semantics.dispose();
    });

    testWidgets('a value without a location opens nothing', (tester) async {
      await openEntry(tester, labReportId);

      await tapValue(tester, '<0,5 mg/dl');

      expect(find.byType(OriginalLocationScreen), findsNothing);
      expect(find.text('Erkannte Angaben'), findsOneWidget);
    });

    testWidgets('the pencil still corrects and does not jump', (tester) async {
      await openEntry(tester, labReportId);

      await tester.tap(find.byTooltip('Hämoglobin korrigieren'));
      await tester.pumpAndSettle();

      expect(find.text('Hämoglobin korrigieren'), findsWidgets);
      expect(find.byType(AlertDialog), findsOneWidget);
      expect(find.byType(OriginalLocationScreen), findsNothing);
    });
  });

  group('PDF', () {
    testWidgets('shows the page of the location with its rectangle', (
      tester,
    ) async {
      final viewer = await openEntry(tester, labReportId);

      await tapValue(tester, '14,2 g/dl');

      expect(find.byType(OriginalLocationScreen), findsOneWidget);
      expect(viewer.renderedPages, [1]);
      // App bar: name of the field and the value.
      expect(find.text('Hämoglobin'), findsOneWidget);
      expect(find.text('14,2 g/dl'), findsOneWidget);
      expect(marks(tester).locations, hasLength(1));
      // One location: no stepping, only the page.
      expect(find.text('Seite 1'), findsOneWidget);
      expect(find.byTooltip('Nächste Fundstelle'), findsNothing);
    });

    testWidgets('all rectangles of the value on the page are marked', (
      tester,
    ) async {
      await openEntry(tester, labReportId);

      // „Labor Muster GmbH, Salzburg“: name and city in two places.
      await tapValue(tester, 'Labor Muster GmbH, Salzburg');

      expect(marks(tester).locations, hasLength(2));
      expect(find.text('Fundstelle 1 von 2 · Seite 1'), findsOneWidget);
    });

    testWidgets('stepping to a location on another page renders that page', (
      tester,
    ) async {
      final viewer = await openEntry(tester, prescriptionId);

      // „Ramipril 5 mg“ is on page 1 and on page 2.
      await tapValue(tester, '5 mg');
      expect(find.text('Fundstelle 1 von 2 · Seite 1'), findsOneWidget);
      final first = marks(tester).current;

      await tester.tap(find.byTooltip('Nächste Fundstelle'));
      await tester.pumpAndSettle();

      expect(find.text('Fundstelle 2 von 2 · Seite 2'), findsOneWidget);
      expect(viewer.renderedPages, [1, 2]);
      expect(marks(tester).current.page, 2);
      expect(marks(tester).current, isNot(same(first)));

      // Back to the first one: page 1 is not rendered again.
      await tester.tap(find.byTooltip('Vorherige Fundstelle'));
      await tester.pumpAndSettle();
      expect(find.text('Fundstelle 1 von 2 · Seite 1'), findsOneWidget);
      expect(viewer.renderedPages, [1, 2]);
    });

    testWidgets('a page the PDF does not have: calm message, whole original', (
      tester,
    ) async {
      final viewer = FakePdfViewer()..pageCount = 1;
      await openEntry(tester, prescriptionId, viewer: viewer);

      await tapValue(tester, '5 mg');
      await tester.tap(find.byTooltip('Nächste Fundstelle'));
      await tester.pumpAndSettle();

      expect(
        find.text('Diese Stelle kann nicht angezeigt werden.'),
        findsOneWidget,
      );
      await tester.tap(find.text(openWholeOriginalLabel));
      await tester.pumpAndSettle();
      expect(find.byType(OriginalFileScreen), findsOneWidget);
    });
  });

  group('photo', () {
    testWidgets('the rectangle lies over the upright photo', (tester) async {
      await openEntry(tester, doctorLetterId);

      // No pumpAndSettle: the progress indicator spins until the photo is
      // decoded, which only happens in real time.
      await tester.tap(find.text('128/82 mmHg'));
      await tester.pump();
      await tester.pump(const Duration(seconds: 1)); // page transition
      expect(find.byType(PhotoLocationView), findsOneWidget);
      await decodeImages(tester, find.byType(LocationOverlay));
      await tester.pump(const Duration(milliseconds: 100));

      final overlay = tester.widget<LocationOverlay>(
        find.byType(LocationOverlay),
      );
      // The size comes from the decoded image (upright), here the mock
      // photo of 600×800.
      expect(overlay.pageSize, const Size(600, 800));
      expect(marks(tester).locations, hasLength(1));
      // Photos have no page number in the bar, and one location no bar.
      expect(find.textContaining('Seite'), findsNothing);
    });
  });

  group('text import', () {
    testWidgets('the range in fullText is highlighted', (tester) async {
      await openEntry(tester, noteId);

      await tapValue(tester, '64 /min');

      expect(find.byType(TextLocationView), findsOneWidget);
      expect(highlighted(tester), ['64/min']);
    });

    testWidgets('code points, not UTF-16: umlauts and emojis before it', (
      tester,
    ) async {
      const text = 'Größe 182 cm 🙂\nPuls 🫀 64/min\nÄrztin: Dr. Ö';
      // „64/min“ starts at code point 22 („Größe 182 cm 🙂\n“ = 15,
      // „Puls 🫀 “ = 7) but at UTF-16 index 24 (two emojis before it).
      const location = TextLocation(start: 22, end: 28);
      expect(location.textIn(text), '64/min');
      expect(location.utf16Range(text), (24, 30));
      await tester.pumpWidget(
        const MaterialApp(
          home: Scaffold(
            body: TextLocationView(
              fullText: text,
              locations: [location],
              current: location,
            ),
          ),
        ),
      );

      expect(highlighted(tester), ['64/min']);
    });

    testWidgets('a location far down is scrolled into view', (tester) async {
      final lines = [for (var i = 0; i < 200; i++) 'Zeile $i'];
      final text = [...lines, 'Puls 64/min'].join('\n');
      final start = text.runes.length - 6;
      final location = TextLocation(start: start, end: start + 6);
      await tester.pumpWidget(
        MaterialApp(
          home: Scaffold(
            body: TextLocationView(
              fullText: text,
              locations: [location],
              current: location,
            ),
          ),
        ),
      );
      await tester.pump();

      final position = tester
          .state<ScrollableState>(find.byType(Scrollable).first)
          .position;
      // The last line is at the bottom of a long text: the view scrolled
      // down to it (as far as possible).
      expect(position.maxScrollExtent, greaterThan(1000));
      expect(position.pixels, position.maxScrollExtent);
    });

    testWidgets('a location at the top: no scrolling', (tester) async {
      final lines = [for (var i = 0; i < 200; i++) 'Zeile $i'];
      final text = ['Puls 64/min', ...lines].join('\n');
      const location = TextLocation(start: 5, end: 11);
      await tester.pumpWidget(
        MaterialApp(
          home: Scaffold(
            body: TextLocationView(
              fullText: text,
              locations: const [location],
              current: location,
            ),
          ),
        ),
      );
      await tester.pump();

      final position = tester
          .state<ScrollableState>(find.byType(Scrollable).first)
          .position;
      expect(position.pixels, 0);
    });
  });

  group('back', () {
    testWidgets('back leads into the entry', (tester) async {
      await openEntry(tester, labReportId);
      await tapValue(tester, '14,2 g/dl');
      expect(find.byType(OriginalLocationScreen), findsOneWidget);

      await tapBack(tester);

      expect(find.byType(OriginalLocationScreen), findsNothing);
      expect(find.text('Erkannte Angaben'), findsOneWidget);
    });

    testWidgets('whole original from the location, then back twice', (
      tester,
    ) async {
      await openEntry(tester, noteId);
      await tapValue(tester, '64 /min');

      await tester.tap(find.byTooltip(openWholeOriginalLabel));
      await tester.pumpAndSettle();
      expect(find.byType(OriginalFileScreen), findsOneWidget);
      expect(find.text('text.txt'), findsOneWidget);

      await tapBack(tester);
      expect(find.byType(TextLocationView), findsOneWidget);

      await tapBack(tester);
      expect(find.text('Erkannte Angaben'), findsOneWidget);
    });
  });

  testWidgets('the mock photo: every location of the doctor letter lies on '
      'dark „words“', (tester) async {
    final api = fastMock();
    final entry = await api.getEntry(doctorLetterId);
    final file = await api.getFile(doctorLetterId);
    final pixels = await tester.runAsync(() async {
      final codec = await ui.instantiateImageCodec(file.bytes);
      final image = (await codec.getNextFrame()).image;
      final data = await image.toByteData(format: ui.ImageByteFormat.rawRgba);
      final result = (image.width, image.height, data!);
      image.dispose();
      codec.dispose();
      return result;
    });
    final (width, height, data) = pixels!;
    expect(
      (width, height),
      (MockLayout.photo.pageWidth.toInt(), MockLayout.photo.pageHeight.toInt()),
    );
    int grey(double x, double y) =>
        data.getUint8((y.floor() * width + x.floor()) * 4);

    final located = entry.verification.fields.where(
      (f) => f.locations.isNotEmpty,
    );
    expect(located, isNotEmpty);
    for (final field in located) {
      for (final l in field.locations.cast<PageLocation>()) {
        // The middle of the first character of the location is dark, the
        // area just left of it (a space or the margin) is white.
        final left = l.x * width;
        final middle = (l.y + l.height / 2) * height;
        expect(grey(left + 4, middle), lessThan(128), reason: field.field);
        expect(grey(left - 4, middle), 255, reason: field.field);
      }
    }
  });
}
