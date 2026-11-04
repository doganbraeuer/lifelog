import 'package:flutter/material.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:lifelog/api/api_client.dart';
import 'package:lifelog/api/mock_api_client.dart';
import 'package:lifelog/screens/upload_screen.dart';

import '../hilfen.dart';

const labor = 'Labor Muster GmbH, Salzburg';
const arzt = 'Dr. Erika Beispiel, Allgemeinmedizin';
const unbekannt = 'Quelle unbekannt';

double oben(WidgetTester tester, String text) =>
    tester.getTopLeft(find.textContaining(text).first).dy;

Future<void> zurZeitleiste(WidgetTester tester, MockApiClient mock) async {
  grosserBildschirm(tester);
  await tester.pumpWidget(testApp(mock));
  await tester.pumpAndSettle();
  await zuGesundheit(tester);
}

void main() {
  testWidgets('Monatsüberschriften, Einträge darunter, ohne Datum am Ende', (
    tester,
  ) async {
    await zurZeitleiste(tester, schnellerMock());

    // Reihenfolge: Oktober (Labor), September (Arztbrief), Ohne Datum.
    expect(oben(tester, 'Oktober 2026'), lessThan(oben(tester, labor)));
    expect(oben(tester, labor), lessThan(oben(tester, 'September 2026')));
    expect(oben(tester, 'September 2026'), lessThan(oben(tester, arzt)));
    expect(oben(tester, arzt), lessThan(oben(tester, 'Ohne Datum')));
    expect(oben(tester, 'Ohne Datum'), lessThan(oben(tester, unbekannt)));
  });

  testWidgets('aufsteigend: September vor Oktober, ohne Datum bleibt am Ende', (
    tester,
  ) async {
    await zurZeitleiste(tester, schnellerMock());
    await tester.tap(find.byIcon(Icons.arrow_downward));
    await tester.pumpAndSettle();

    expect(
      oben(tester, 'September 2026'),
      lessThan(oben(tester, 'Oktober 2026')),
    );
    expect(oben(tester, 'Oktober 2026'), lessThan(oben(tester, 'Ohne Datum')));
  });

  testWidgets('nach Importdatum: Gruppen nach Importmonat, keine '
      '„Ohne Datum“-Gruppe', (tester) async {
    await zurZeitleiste(tester, schnellerMock());
    await tester.tap(find.byIcon(Icons.sort));
    await tester.pumpAndSettle();
    await tester.tap(
      find.widgetWithText(CheckedPopupMenuItem<Sortierung>, 'Nach Importdatum'),
    );
    await tester.pumpAndSettle();

    // Alle Mock-Einträge wurden am 03.11.2026 importiert.
    expect(find.text('November 2026'), findsOneWidget);
    expect(find.text('Ohne Datum'), findsNothing);
    expect(find.text('Oktober 2026'), findsNothing);
  });

  testWidgets('Überschriften sind für Screenreader als Überschrift markiert', (
    tester,
  ) async {
    final semantik = tester.ensureSemantics();
    await zurZeitleiste(tester, schnellerMock());

    expect(
      tester.getSemantics(find.text('Oktober 2026')),
      isSemantics(isHeader: true),
    );
    semantik.dispose();
  });

  testWidgets('leerer Zustand: Knopf führt zu Hochladen', (tester) async {
    await zurZeitleiste(tester, schnellerMock(mitBeispieldaten: false));

    expect(find.text('Noch keine Dokumente.'), findsOneWidget);
    await tester.tap(find.text('Dokument hochladen'));
    await tester.pumpAndSettle();

    expect(find.byType(UploadScreen), findsOneWidget);
    expect(find.text('Text eingeben'), findsOneWidget);
  });

  group('Handygröße (1080 × 2400, Faktor 3)', () {
    void handy(WidgetTester tester) {
      tester.view.physicalSize = const Size(1080, 2400);
      tester.view.devicePixelRatio = 3;
      addTearDown(tester.view.reset);
    }

    testWidgets('Bereiche: nichts läuft über, alle Kacheln sichtbar', (
      tester,
    ) async {
      handy(tester);
      await tester.pumpWidget(testApp(schnellerMock()));
      await tester.pumpAndSettle();

      expect(tester.takeException(), isNull);
      for (final name in ['Gesundheit', 'Ernährung', 'Rezepte', 'Sport']) {
        final rect = tester.getRect(find.text(name));
        expect(rect.bottom, lessThanOrEqualTo(800), reason: name);
      }
    });

    testWidgets('Zeitachse: nichts läuft über, auch mit langer Quelle und '
        'nach Importdatum', (tester) async {
      handy(tester);
      final mock = schnellerMock();
      await mock.textImportieren(
        '${'Sehr lange Bezeichnung einer ausstellenden Stelle ' * 4}\n'
        'Befund vom 21.10.2026',
        modul: 'gesundheit',
      );
      await tester.pumpWidget(testApp(mock));
      await tester.pumpAndSettle();
      await zuGesundheit(tester);
      expect(tester.takeException(), isNull);
      expect(find.text('Oktober 2026'), findsOneWidget);

      await tester.tap(find.byIcon(Icons.sort));
      await tester.pumpAndSettle();
      await tester.tap(
        find.widgetWithText(
          CheckedPopupMenuItem<Sortierung>,
          'Nach Importdatum',
        ),
      );
      await tester.pumpAndSettle();
      expect(tester.takeException(), isNull);
    });

    testWidgets('leerer Zustand: nichts läuft über', (tester) async {
      handy(tester);
      await tester.pumpWidget(testApp(schnellerMock(mitBeispieldaten: false)));
      await tester.pumpAndSettle();
      await zuGesundheit(tester);

      expect(tester.takeException(), isNull);
      expect(find.text('Dokument hochladen'), findsOneWidget);
    });
  });
}
