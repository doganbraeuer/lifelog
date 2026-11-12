// Etappe V2: Platzhalter, Scan-Animation, gestaffeltes Einblenden und
// „Animationen entfernen“.

import 'package:flutter/material.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:lifelog/api/mock_api_client.dart';
import 'package:lifelog/screens/question_screen.dart';
import 'package:lifelog/screens/upload_screen.dart';
import 'package:lifelog/screens/timeline_screen.dart';

import '../helpers.dart';

/// Mock mit spürbarer Wartezeit, damit Ladezustände sichtbar sind.
MockApiClient langsamerMock() => MockApiClient(
  verzoegerung: Durations.extralong4,
  uploadVerzoegerung: Durations.extralong4 * 2,
);

void animationenAus(WidgetTester tester) {
  tester.platformDispatcher.accessibilityFeaturesTestValue =
      const FakeAccessibilityFeatures(disableAnimations: true);
  addTearDown(tester.platformDispatcher.clearAccessibilityFeaturesTestValue);
}

Finder navigation(String label) =>
    find.descendant(of: find.byType(NavigationBar), matching: find.text(label));

/// Öffnet Gesundheit, ohne auf das Laden der Zeitleiste zu warten.
Future<void> gesundheitOeffnen(WidgetTester tester, MockApiClient mock) async {
  grosserBildschirm(tester);
  await tester.pumpWidget(testApp(mock));
  await tester.pumpAndSettle();
  await tester.tap(find.text('Gesundheit'));
  // Seitenübergang abwarten, aber nicht das Laden (dauert 1 s).
  for (var i = 0; i < 10; i++) {
    await tester.pump(Durations.short1);
  }
}

void main() {
  group('Zeitleiste lädt', () {
    testWidgets('Platzhalter statt Spinner, dann die Liste', (tester) async {
      final semantik = tester.ensureSemantics();
      await gesundheitOeffnen(tester, langsamerMock());

      final zeitleiste = find.byType(ZeitleisteScreen);
      expect(
        find.descendant(
          of: zeitleiste,
          matching: find.byType(CircularProgressIndicator),
        ),
        findsNothing,
      );
      expect(find.bySemanticsLabel('Wird geladen'), findsOneWidget);

      await tester.pump(Durations.extralong4);
      await tester.pumpAndSettle();
      expect(find.bySemanticsLabel('Wird geladen'), findsNothing);
      expect(find.text('Oktober 2026'), findsOneWidget);
      semantik.dispose();
    });

    testWidgets('„Animationen entfernen“: Platzhalter steht still', (
      tester,
    ) async {
      animationenAus(tester);
      await gesundheitOeffnen(tester, langsamerMock());

      expect(find.bySemanticsLabel('Wird geladen'), findsOneWidget);
      expect(tester.binding.hasScheduledFrame, isFalse);

      await tester.pump(Durations.extralong4);
      await tester.pumpAndSettle();
    });
  });

  group('Frage', () {
    Future<void> fragen(WidgetTester tester, MockApiClient mock) async {
      grosserBildschirm(tester);
      await tester.pumpWidget(testApp(mock));
      await tester.pumpAndSettle();
      await zuGesundheit(tester);
      await tester.tap(navigation('Fragen'));
      await tester.pumpAndSettle();
      await tester.enterText(find.byType(TextField), 'Hämoglobin');
      await tester.pump();
      await tester.tap(find.byTooltip('Frage senden'));
      await tester.pump();
    }

    double deckkraft(WidgetTester tester, int i) => tester
        .widget<FadeTransition>(find.byKey(ValueKey('gestaffelt-$i')))
        .opacity
        .value;

    testWidgets('Suche zeigt Platzhalter, kein Spinner', (tester) async {
      final semantik = tester.ensureSemantics();
      await fragen(tester, langsamerMock());
      await tester.pump(Durations.short4);

      expect(find.text('Suche in deinen Dokumenten …'), findsOneWidget);
      expect(find.bySemanticsLabel('Wird geladen'), findsOneWidget);
      expect(
        find.descendant(
          of: find.byType(FrageScreen),
          matching: find.byType(CircularProgressIndicator),
        ),
        findsNothing,
      );

      await tester.pump(Durations.extralong4);
      await tester.pumpAndSettle();
      expect(find.text('Quellen'), findsOneWidget);
      semantik.dispose();
    });

    testWidgets('Antwort und Quellen blenden nacheinander ein', (tester) async {
      await fragen(tester, schnellerMock());
      await tester.pump();

      // Kurz nach dem Start: Die Antwort ist weiter als die Quellen.
      await tester.pump(Durations.short3);
      final antwort = deckkraft(tester, 0);
      final ersteQuelle = deckkraft(tester, 2);
      expect(antwort, greaterThan(ersteQuelle));
      // Der Text ist trotzdem schon vollständig da (kein Tipp-Effekt).
      expect(find.textContaining('[1]'), findsOneWidget);

      await tester.pumpAndSettle();
      expect(deckkraft(tester, 0), 1);
      expect(deckkraft(tester, 2), 1);
    });

    testWidgets('„Animationen entfernen“: sofort vollständig sichtbar', (
      tester,
    ) async {
      animationenAus(tester);
      await fragen(tester, schnellerMock());
      await tester.pump();

      expect(deckkraft(tester, 0), 1);
      expect(deckkraft(tester, 2), 1);
    });
  });

  group('Upload', () {
    Future<void> hochladenStarten(
      WidgetTester tester,
      MockApiClient mock,
    ) async {
      grosserBildschirm(tester);
      final auswahl = FakeDateiAuswahl()..pdf = testPdf('scan.pdf');
      await tester.pumpWidget(testApp(mock, auswahl: auswahl));
      await tester.pumpAndSettle();
      await zuGesundheit(tester);
      await tester.tap(navigation('Hochladen'));
      await tester.pumpAndSettle();
      await tester.tap(find.text('PDF auswählen'));
      await tester.pumpAndSettle();
      await tester.tap(find.widgetWithText(FilledButton, 'Hochladen'));
      await tester.pump();
    }

    testWidgets('Scan-Animation statt Spinner, mit Text und Semantik', (
      tester,
    ) async {
      final semantik = tester.ensureSemantics();
      await hochladenStarten(tester, langsamerMock());
      await tester.pump(Durations.medium2);

      expect(
        find.bySemanticsLabel('Dokument wird ausgewertet'),
        findsOneWidget,
      );
      expect(find.textContaining('bis zu einer Minute'), findsOneWidget);
      expect(
        find.descendant(
          of: find.byType(UploadScreen),
          matching: find.byType(CircularProgressIndicator),
        ),
        findsNothing,
      );
      // Läuft, solange ausgewertet wird.
      expect(tester.binding.hasScheduledFrame, isTrue);

      await tester.pump(Durations.extralong4 * 2);
      await tester.pumpAndSettle();
      semantik.dispose();
    });

    testWidgets('„Animationen entfernen“: Scan steht still', (tester) async {
      animationenAus(tester);
      await hochladenStarten(tester, langsamerMock());
      await tester.pump();

      expect(find.textContaining('bis zu einer Minute'), findsOneWidget);
      expect(tester.binding.hasScheduledFrame, isFalse);

      await tester.pump(Durations.extralong4 * 2);
      await tester.pumpAndSettle();
    });

    testWidgets('Fehlerkarte erscheint weich und verschwindet wieder', (
      tester,
    ) async {
      grosserBildschirm(tester);
      final auswahl = FakeDateiAuswahl()..pdf = testPdf('modellfehler.pdf');
      await tester.pumpWidget(testApp(schnellerMock(), auswahl: auswahl));
      await tester.pumpAndSettle();
      await zuGesundheit(tester);
      await tester.tap(navigation('Hochladen'));
      await tester.pumpAndSettle();
      await tester.tap(find.text('PDF auswählen'));
      await tester.pumpAndSettle();
      await tester.tap(find.widgetWithText(FilledButton, 'Hochladen'));
      await tester.pumpAndSettle();
      expect(find.textContaining('nicht erreichbar'), findsOneWidget);

      await tester.tap(find.text('Andere Datei wählen'));
      await tester.pumpAndSettle();
      expect(find.textContaining('nicht erreichbar'), findsNothing);
      expect(find.text('PDF auswählen'), findsOneWidget);
    });
  });

  group('Handygröße (1080 × 2400, Faktor 3)', () {
    testWidgets('Platzhalter, Scan und Antwort laufen nicht über', (
      tester,
    ) async {
      tester.view.physicalSize = const Size(1080, 2400);
      tester.view.devicePixelRatio = 3;
      addTearDown(tester.view.reset);
      final auswahl = FakeDateiAuswahl()..pdf = testPdf('handy.pdf');
      final mock = langsamerMock();

      await tester.pumpWidget(testApp(mock, auswahl: auswahl));
      await tester.pumpAndSettle();
      await tester.tap(find.text('Gesundheit'));
      for (var i = 0; i < 10; i++) {
        await tester.pump(Durations.short1);
      }
      expect(tester.takeException(), isNull); // Platzhalter
      await tester.pump(Durations.extralong4);
      await tester.pumpAndSettle();

      await tester.tap(navigation('Hochladen'));
      await tester.pumpAndSettle();
      await tester.tap(find.text('PDF auswählen'));
      await tester.pumpAndSettle();
      await tester.tap(find.widgetWithText(FilledButton, 'Hochladen'));
      await tester.pump(Durations.medium2);
      expect(tester.takeException(), isNull); // Scan
      await tester.pump(Durations.extralong4 * 2);
      await tester.pumpAndSettle();
      expect(tester.takeException(), isNull);
    });
  });
}
