import 'package:flutter/material.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:lifelog/api/api_client.dart';

import '../hilfen.dart';

/// Vertikale Position eines Texts, um die Reihenfolge zu prüfen.
/// `textContaining`, weil bei Sortierung nach Importdatum Quelle und
/// Importzeit in einem gemeinsamen Text stehen.
double oben(WidgetTester tester, String text) =>
    tester.getTopLeft(find.textContaining(text)).dy;

const labor = 'Labor Muster GmbH, Salzburg';
const arzt = 'Dr. Erika Beispiel, Allgemeinmedizin';
const unbekannt = 'Quelle unbekannt';

void main() {
  testWidgets('zeigt die Einträge in der Reihenfolge des Backends', (
    tester,
  ) async {
    await tester.pumpWidget(testApp(schnellerMock()));
    await tester.pumpAndSettle();

    expect(find.text('Mock'), findsOneWidget);
    expect(find.text('14.10.2026'), findsOneWidget);
    expect(find.text('02.09.2026'), findsOneWidget);
    expect(find.text('ohne Datum'), findsOneWidget);
    expect(find.text('Dokumentdatum · neueste zuerst'), findsOneWidget);

    expect(oben(tester, labor), lessThan(oben(tester, arzt)));
    expect(oben(tester, arzt), lessThan(oben(tester, unbekannt)));
  });

  testWidgets('Richtung umschalten: ohne Datum bleibt am Ende', (tester) async {
    await tester.pumpWidget(testApp(schnellerMock()));
    await tester.pumpAndSettle();

    await tester.tap(find.byIcon(Icons.arrow_downward));
    await tester.pumpAndSettle();

    expect(find.text('Dokumentdatum · älteste zuerst'), findsOneWidget);
    expect(oben(tester, arzt), lessThan(oben(tester, labor)));
    expect(oben(tester, labor), lessThan(oben(tester, unbekannt)));
  });

  testWidgets('nach Importdatum sortieren', (tester) async {
    await tester.pumpWidget(testApp(schnellerMock()));
    await tester.pumpAndSettle();

    await tester.tap(find.byIcon(Icons.sort));
    await tester.pumpAndSettle();
    await tester.tap(
      find.widgetWithText(CheckedPopupMenuItem<Sortierung>, 'Nach Importdatum'),
    );
    await tester.pumpAndSettle();

    expect(find.text('Importdatum · neueste zuerst'), findsOneWidget);
    // Import: sonstiges 19:01, Arztbrief 18:50, Laborbefund 18:42 (UTC).
    expect(oben(tester, unbekannt), lessThan(oben(tester, arzt)));
    expect(oben(tester, arzt), lessThan(oben(tester, labor)));
    expect(find.textContaining('Importiert:'), findsNWidgets(3));
  });

  testWidgets('Fehler anzeigen und erneut versuchen', (tester) async {
    final mock = schnellerMock()..simulierterFehler = true;
    await tester.pumpWidget(testApp(mock));
    await tester.pumpAndSettle();

    expect(find.textContaining('Fehler aufgetreten'), findsOneWidget);
    expect(find.text(labor), findsNothing);

    mock.simulierterFehler = false;
    await tester.tap(find.text('Erneut versuchen'));
    await tester.pumpAndSettle();

    expect(find.text(labor), findsOneWidget);
  });

  testWidgets('leere Zeitleiste', (tester) async {
    await tester.pumpWidget(testApp(schnellerMock(mitBeispieldaten: false)));
    await tester.pumpAndSettle();

    expect(find.textContaining('Noch keine Dokumente'), findsOneWidget);
  });

  testWidgets('Tippen öffnet den Eintrag', (tester) async {
    grosserBildschirm(tester);
    await tester.pumpWidget(testApp(schnellerMock()));
    await tester.pumpAndSettle();

    await tester.tap(find.text(labor));
    await tester.pumpAndSettle();

    expect(find.text('Erkannte Angaben'), findsOneWidget);
    expect(find.text('Hämoglobin'), findsOneWidget);
    expect(find.text('14,2 g/dl'), findsOneWidget);
    expect(find.text('Blutbild klein'), findsOneWidget);
  });
}
