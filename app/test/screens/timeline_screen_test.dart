import 'package:flutter/material.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:lifelog/api/api_client.dart';

import '../helpers.dart';

/// Vertical position of a text, to check the order.
/// `textContaining`, because when sorting by import date the issuer and
/// the import time are in one text.
double topOf(WidgetTester tester, String text) =>
    tester.getTopLeft(find.textContaining(text)).dy;

const labIssuer = 'Labor Muster GmbH, Salzburg';
const doctorIssuer = 'Dr. Erika Beispiel, Allgemeinmedizin';
const unknown = 'Quelle unbekannt';

void main() {
  testWidgets('shows the entries in the order of the backend', (tester) async {
    await tester.pumpWidget(testApp(fastMock()));
    await tester.pumpAndSettle();
    await openHealth(tester);

    expect(find.text('Mock'), findsOneWidget);
    expect(find.text('14.10.2026'), findsOneWidget);
    expect(find.text('02.09.2026'), findsOneWidget);
    expect(find.text('ohne Datum'), findsOneWidget);
    expect(find.text('Dokumentdatum · neueste zuerst'), findsOneWidget);

    expect(topOf(tester, labIssuer), lessThan(topOf(tester, doctorIssuer)));
    expect(topOf(tester, doctorIssuer), lessThan(topOf(tester, unknown)));
  });

  testWidgets('toggle the order: without date stays at the end', (
    tester,
  ) async {
    await tester.pumpWidget(testApp(fastMock()));
    await tester.pumpAndSettle();
    await openHealth(tester);

    await tester.tap(find.byIcon(Icons.arrow_downward));
    await tester.pumpAndSettle();

    expect(find.text('Dokumentdatum · älteste zuerst'), findsOneWidget);
    expect(topOf(tester, doctorIssuer), lessThan(topOf(tester, labIssuer)));
    expect(topOf(tester, labIssuer), lessThan(topOf(tester, unknown)));
  });

  testWidgets('sort by import date', (tester) async {
    await tester.pumpWidget(testApp(fastMock()));
    await tester.pumpAndSettle();
    await openHealth(tester);

    await tester.tap(find.byIcon(Icons.sort));
    await tester.pumpAndSettle();
    await tester.tap(
      find.widgetWithText(CheckedPopupMenuItem<Sort>, 'Nach Importdatum'),
    );
    await tester.pumpAndSettle();

    expect(find.text('Importdatum · neueste zuerst'), findsOneWidget);
    // Import: other 19:01, doctor letter 18:50, lab report 18:42 (UTC).
    expect(topOf(tester, unknown), lessThan(topOf(tester, doctorIssuer)));
    expect(topOf(tester, doctorIssuer), lessThan(topOf(tester, labIssuer)));
    expect(find.textContaining('Importiert:'), findsNWidgets(3));
  });

  testWidgets('show an error and retry', (tester) async {
    final mock = fastMock()..simulateError = true;
    await tester.pumpWidget(testApp(mock));
    await tester.pumpAndSettle();
    await openHealth(tester);

    expect(find.textContaining('Fehler aufgetreten'), findsOneWidget);
    expect(find.text(labIssuer), findsNothing);

    mock.simulateError = false;
    await tester.tap(find.text('Erneut versuchen'));
    await tester.pumpAndSettle();

    expect(find.text(labIssuer), findsOneWidget);
  });

  testWidgets('empty timeline', (tester) async {
    await tester.pumpWidget(testApp(fastMock(withSampleData: false)));
    await tester.pumpAndSettle();
    await openHealth(tester);

    expect(find.textContaining('Noch keine Dokumente'), findsOneWidget);
  });

  testWidgets('tapping opens the entry', (tester) async {
    useLargeScreen(tester);
    await tester.pumpWidget(testApp(fastMock()));
    await tester.pumpAndSettle();
    await openHealth(tester);

    await tester.tap(find.text(labIssuer));
    await tester.pumpAndSettle();

    expect(find.text('Erkannte Angaben'), findsOneWidget);
    expect(find.text('Hämoglobin'), findsOneWidget);
    expect(find.text('14,2 g/dl'), findsOneWidget);
    expect(find.text('Blutbild klein'), findsOneWidget);
  });
}
