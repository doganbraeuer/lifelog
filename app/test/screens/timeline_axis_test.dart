import 'package:flutter/material.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:lifelog/api/api_client.dart';
import 'package:lifelog/api/mock_api_client.dart';
import 'package:lifelog/screens/upload_screen.dart';

import '../helpers.dart';

const labIssuer = 'Labor Muster GmbH, Salzburg';
const doctorIssuer = 'Dr. Erika Beispiel, Allgemeinmedizin';
const unknown = 'Quelle unbekannt';

double topOf(WidgetTester tester, String text) =>
    tester.getTopLeft(find.textContaining(text).first).dy;

Future<void> goToTimeline(WidgetTester tester, MockApiClient mock) async {
  useLargeScreen(tester);
  await tester.pumpWidget(testApp(mock));
  await tester.pumpAndSettle();
  await openHealth(tester);
}

void main() {
  testWidgets('month headers, entries below them, without date at the end', (
    tester,
  ) async {
    await goToTimeline(tester, fastMock());

    // Order: October (lab), September (doctor letter), without date.
    expect(topOf(tester, 'Oktober 2026'), lessThan(topOf(tester, labIssuer)));
    expect(topOf(tester, labIssuer), lessThan(topOf(tester, 'September 2026')));
    expect(
      topOf(tester, 'September 2026'),
      lessThan(topOf(tester, doctorIssuer)),
    );
    expect(topOf(tester, doctorIssuer), lessThan(topOf(tester, 'Ohne Datum')));
    expect(topOf(tester, 'Ohne Datum'), lessThan(topOf(tester, unknown)));
  });

  testWidgets(
    'ascending: September before October, without date stays at the end',
    (tester) async {
      await goToTimeline(tester, fastMock());
      await tester.tap(find.byIcon(Icons.arrow_downward));
      await tester.pumpAndSettle();

      expect(
        topOf(tester, 'September 2026'),
        lessThan(topOf(tester, 'Oktober 2026')),
      );
      expect(
        topOf(tester, 'Oktober 2026'),
        lessThan(topOf(tester, 'Ohne Datum')),
      );
    },
  );

  testWidgets('by import date: groups by import month, no '
      '„Ohne Datum“ group', (tester) async {
    await goToTimeline(tester, fastMock());
    await tester.tap(find.byIcon(Icons.sort));
    await tester.pumpAndSettle();
    await tester.tap(
      find.widgetWithText(CheckedPopupMenuItem<Sort>, 'Nach Importdatum'),
    );
    await tester.pumpAndSettle();

    // All mock entries were imported on 03.11.2026.
    expect(find.text('November 2026'), findsOneWidget);
    expect(find.text('Ohne Datum'), findsNothing);
    expect(find.text('Oktober 2026'), findsNothing);
  });

  testWidgets('headers are marked as headers for screen readers', (
    tester,
  ) async {
    final semantics = tester.ensureSemantics();
    await goToTimeline(tester, fastMock());

    expect(
      tester.getSemantics(find.text('Oktober 2026')),
      isSemantics(isHeader: true),
    );
    semantics.dispose();
  });

  testWidgets('empty state: button leads to the upload page', (tester) async {
    await goToTimeline(tester, fastMock(withSampleData: false));

    expect(find.text('Noch keine Dokumente.'), findsOneWidget);
    await tester.tap(find.text('Dokument hochladen'));
    await tester.pumpAndSettle();

    expect(find.byType(UploadScreen), findsOneWidget);
    expect(find.text('Text eingeben'), findsOneWidget);
  });

  group('phone size (1080 × 2400, factor 3)', () {
    void usePhoneScreen(WidgetTester tester) {
      tester.view.physicalSize = const Size(1080, 2400);
      tester.view.devicePixelRatio = 3;
      addTearDown(tester.view.reset);
    }

    testWidgets('modules screen: nothing overflows, all tiles visible', (
      tester,
    ) async {
      usePhoneScreen(tester);
      await tester.pumpWidget(testApp(fastMock()));
      await tester.pumpAndSettle();

      expect(tester.takeException(), isNull);
      for (final name in ['Gesundheit', 'Ernährung', 'Rezepte', 'Sport']) {
        final rect = tester.getRect(find.text(name));
        expect(rect.bottom, lessThanOrEqualTo(800), reason: name);
      }
    });

    testWidgets('timeline axis: nothing overflows, even with a long issuer and '
        'by import date', (tester) async {
      usePhoneScreen(tester);
      final mock = fastMock();
      await mock.importText(
        '${'Sehr lange Bezeichnung einer ausstellenden Stelle ' * 4}\n'
        'Befund vom 21.10.2026',
        module: 'health',
      );
      await tester.pumpWidget(testApp(mock));
      await tester.pumpAndSettle();
      await openHealth(tester);
      expect(tester.takeException(), isNull);
      expect(find.text('Oktober 2026'), findsOneWidget);

      await tester.tap(find.byIcon(Icons.sort));
      await tester.pumpAndSettle();
      await tester.tap(
        find.widgetWithText(CheckedPopupMenuItem<Sort>, 'Nach Importdatum'),
      );
      await tester.pumpAndSettle();
      expect(tester.takeException(), isNull);
    });

    testWidgets('empty state: nothing overflows', (tester) async {
      usePhoneScreen(tester);
      await tester.pumpWidget(testApp(fastMock(withSampleData: false)));
      await tester.pumpAndSettle();
      await openHealth(tester);

      expect(tester.takeException(), isNull);
      expect(find.text('Dokument hochladen'), findsOneWidget);
    });
  });
}
