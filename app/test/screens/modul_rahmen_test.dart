import 'dart:convert';
import 'dart:typed_data';

import 'package:flutter/material.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:lifelog/api/api_client.dart';
import 'package:lifelog/api/mock_api_client.dart';
import 'package:lifelog/screens/eintrag_screen.dart';
import 'package:lifelog/screens/frage_screen.dart';
import 'package:lifelog/screens/upload_screen.dart';
import 'package:lifelog/screens/zeitleiste_screen.dart';

import '../hilfen.dart';

/// Welche Seite die Navigationsleiste markiert.
int markiert(WidgetTester tester) =>
    tester.widget<NavigationBar>(find.byType(NavigationBar)).selectedIndex;

/// Welche Seite sichtbar ist. Unsichtbare (am Leben gehaltene) Seiten
/// zählen für die Finder nicht.
int sichtbar(WidgetTester tester) {
  final seiten = [
    find.byType(ZeitleisteScreen),
    find.byType(UploadScreen),
    find.byType(FrageScreen),
  ];
  final treffer = [
    for (var i = 0; i < seiten.length; i++)
      if (seiten[i].evaluate().isNotEmpty) i,
  ];
  expect(treffer, hasLength(1), reason: 'genau eine Seite sichtbar');
  return treffer.single;
}

/// Wischen: nach links blättert vor, nach rechts zurück.
Future<void> wischen(WidgetTester tester, {required bool nachLinks}) async {
  await tester.fling(
    find.byType(PageView),
    Offset(nachLinks ? -300 : 300, 0),
    1000,
  );
  await tester.pumpAndSettle();
}

/// Lässt eine Seitenanimation Bild für Bild ablaufen (0,6 s). Nötig, wo
/// pumpAndSettle() nicht geht: Ein einzelnes pump(400 ms) erzeugt nur ein
/// Bild, in dem die Animation gerade erst beginnt.
Future<void> animationAbwarten(WidgetTester tester) async {
  for (var i = 0; i < 30; i++) {
    await tester.pump(const Duration(milliseconds: 20));
  }
}

Finder navigation(String label) =>
    find.descendant(of: find.byType(NavigationBar), matching: find.text(label));

Future<void> starten(
  WidgetTester tester,
  MockApiClient mock, {
  FakeDateiAuswahl? auswahl,
}) async {
  await tester.pumpWidget(testApp(mock, auswahl: auswahl));
  await tester.pumpAndSettle();
  await zuGesundheit(tester);
}

void main() {
  group('Wischen und Navigationsleiste', () {
    testWidgets('Wischen wechselt die Seite, die Leiste folgt', (tester) async {
      await starten(tester, schnellerMock());
      expect((sichtbar(tester), markiert(tester)), (0, 0));

      await wischen(tester, nachLinks: true);
      expect((sichtbar(tester), markiert(tester)), (1, 1));

      await wischen(tester, nachLinks: true);
      expect((sichtbar(tester), markiert(tester)), (2, 2));

      // Am Ende geht es nicht weiter.
      await wischen(tester, nachLinks: true);
      expect((sichtbar(tester), markiert(tester)), (2, 2));

      await wischen(tester, nachLinks: false);
      expect((sichtbar(tester), markiert(tester)), (1, 1));
    });

    testWidgets('Tippen in der Leiste blättert zur Seite', (tester) async {
      await starten(tester, schnellerMock());

      await tester.tap(navigation('Hochladen'));
      await tester.pumpAndSettle();
      expect((sichtbar(tester), markiert(tester)), (1, 1));

      await tester.tap(navigation('Zeitleiste'));
      await tester.pumpAndSettle();
      expect((sichtbar(tester), markiert(tester)), (0, 0));
    });

    testWidgets('Sprung über eine Seite: Leiste flackert nicht', (
      tester,
    ) async {
      await starten(tester, schnellerMock());

      await tester.tap(navigation('Fragen'));
      // Während der Animation läuft die PageView an „Hochladen“ vorbei;
      // die Leiste soll trotzdem durchgehend „Fragen“ zeigen.
      for (var i = 0; i < 25; i++) {
        await tester.pump(const Duration(milliseconds: 16));
        expect(markiert(tester), 2, reason: 'Frame $i');
      }
      await tester.pumpAndSettle();
      expect(sichtbar(tester), 2);
    });
  });

  group('Zustand bleibt beim Wechsel erhalten', () {
    testWidgets('Sortierung der Zeitleiste', (tester) async {
      await starten(tester, schnellerMock());

      await tester.tap(find.byIcon(Icons.sort));
      await tester.pumpAndSettle();
      await tester.tap(
        find.widgetWithText(
          CheckedPopupMenuItem<Sortierung>,
          'Nach Importdatum',
        ),
      );
      await tester.pumpAndSettle();
      await tester.tap(find.byIcon(Icons.arrow_downward));
      await tester.pumpAndSettle();

      await wischen(tester, nachLinks: true);
      await wischen(tester, nachLinks: true);
      await tester.tap(navigation('Zeitleiste'));
      await tester.pumpAndSettle();

      expect(find.text('Importdatum · älteste zuerst'), findsOneWidget);
    });

    testWidgets('Scroll-Position der Zeitleiste', (tester) async {
      // Genug Einträge, damit die Liste scrollt.
      final mock = schnellerMock();
      for (var i = 0; i < 20; i++) {
        await mock.hochladen(
          Uint8List.fromList(utf8.encode('%PDF-1.4 Nr. $i')),
          'dokument_$i.pdf',
        );
      }
      await starten(tester, mock);

      Finder liste() => find
          .descendant(
            of: find.byType(ZeitleisteScreen),
            matching: find.byType(Scrollable),
          )
          .first;
      double position() =>
          tester.state<ScrollableState>(liste()).position.pixels;

      await tester.drag(liste(), const Offset(0, -400));
      await tester.pumpAndSettle();
      final vorher = position();
      expect(vorher, greaterThan(0));

      await wischen(tester, nachLinks: true);
      await wischen(tester, nachLinks: true);
      await wischen(tester, nachLinks: false);
      await wischen(tester, nachLinks: false);

      expect(sichtbar(tester), 0);
      expect(position(), vorher);
    });

    testWidgets('gewählte Datei auf „Hochladen“', (tester) async {
      final auswahl = FakeDateiAuswahl()..pdf = testPdf('befund.pdf');
      await starten(tester, schnellerMock(), auswahl: auswahl);
      await tester.tap(navigation('Hochladen'));
      await tester.pumpAndSettle();
      await tester.tap(find.text('PDF auswählen'));
      await tester.pumpAndSettle();

      await wischen(tester, nachLinks: false); // zur Zeitleiste
      await wischen(tester, nachLinks: true); // zurück

      expect(find.text('befund.pdf'), findsOneWidget);
      expect(find.widgetWithText(FilledButton, 'Hochladen'), findsOneWidget);
    });

    testWidgets('letzte Antwort auf „Fragen“', (tester) async {
      await starten(tester, schnellerMock());
      await tester.tap(navigation('Fragen'));
      await tester.pumpAndSettle();
      await tester.enterText(find.byType(TextField), 'Hämoglobin');
      await tester.pump();
      await tester.tap(find.byTooltip('Frage senden'));
      await tester.pumpAndSettle();
      expect(find.text('Quellen'), findsOneWidget);

      await tester.tap(navigation('Zeitleiste'));
      await tester.pumpAndSettle();
      await wischen(tester, nachLinks: true);
      await wischen(tester, nachLinks: true);

      expect(sichtbar(tester), 2);
      expect(find.text('„Hämoglobin“'), findsOneWidget);
      expect(find.text('Quellen'), findsOneWidget);
    });
  });

  testWidgets('laufender Upload übersteht den Wechsel; danach Zeitleiste '
      'und neuer Eintrag', (tester) async {
    final mock = MockApiClient(
      verzoegerung: Duration.zero,
      uploadVerzoegerung: const Duration(seconds: 2),
    );
    final auswahl = FakeDateiAuswahl()..pdf = testPdf('neu.pdf');
    grosserBildschirm(tester); // Eintrag-Ansicht ist länger als 600 px
    await starten(tester, mock, auswahl: auswahl);
    await tester.tap(navigation('Hochladen'));
    await tester.pumpAndSettle();
    await tester.tap(find.text('PDF auswählen'));
    await tester.pumpAndSettle();

    // Upload starten. Solange er läuft, dreht sich der Ladekreis endlos;
    // deshalb hier pump(Dauer) statt pumpAndSettle().
    await tester.tap(find.widgetWithText(FilledButton, 'Hochladen'));
    await tester.pump(const Duration(milliseconds: 100));
    expect(find.textContaining('bis zu einer Minute'), findsOneWidget);

    // Zu „Fragen“ und zurück: Der Upload läuft weiter.
    await tester.tap(navigation('Fragen'));
    await animationAbwarten(tester);
    expect(sichtbar(tester), 2);
    await tester.tap(navigation('Hochladen'));
    await animationAbwarten(tester);
    expect(find.textContaining('bis zu einer Minute'), findsOneWidget);

    // Wieder zu „Fragen“ und dort fertig werden lassen.
    await tester.tap(navigation('Fragen'));
    await tester.pump(const Duration(seconds: 2));
    await tester.pumpAndSettle();

    // Der Rahmen hat über _zeitleiste.currentState den Eintrag geöffnet ...
    expect(find.byType(EintragScreen), findsOneWidget);
    expect(find.text('Blutdruck'), findsOneWidget);

    // ... und ist zur Zeitleiste gewechselt, die neu geladen ist.
    await zurueck(tester);
    expect((sichtbar(tester), markiert(tester)), (0, 0));
    expect(find.text('Mock-Praxis Beispiel'), findsOneWidget);
  });

  testWidgets('Duplikat: „Vorhandenen Eintrag öffnen“ wechselt zur '
      'Zeitleiste', (tester) async {
    final mock = schnellerMock();
    await mock.hochladen(testPdf('doppelt.pdf').bytes, 'doppelt.pdf');
    final auswahl = FakeDateiAuswahl()..pdf = testPdf('doppelt.pdf');
    await starten(tester, mock, auswahl: auswahl);
    await tester.tap(navigation('Hochladen'));
    await tester.pumpAndSettle();
    await tester.tap(find.text('PDF auswählen'));
    await tester.pumpAndSettle();
    await tester.tap(find.widgetWithText(FilledButton, 'Hochladen'));
    await tester.pumpAndSettle();

    await tester.tap(find.text('Vorhandenen Eintrag öffnen'));
    await tester.pumpAndSettle();
    expect(find.byType(EintragScreen), findsOneWidget);

    await zurueck(tester);
    expect((sichtbar(tester), markiert(tester)), (0, 0));
  });
}
