// Etappe V3: Container Transform, iOS-Zurück-Geste und Haptik.

import 'package:animations/animations.dart';
import 'package:flutter/foundation.dart';
import 'package:flutter/material.dart';
import 'package:flutter/services.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:lifelog/api/mock_api_client.dart';
import 'package:lifelog/screens/entry_screen.dart';
import 'package:lifelog/screens/timeline_screen.dart';

import '../helpers.dart';

const labor = 'Labor Muster GmbH, Salzburg';
const arzt = 'Dr. Erika Beispiel, Allgemeinmedizin';

double oben(WidgetTester tester, String text) =>
    tester.getTopLeft(find.textContaining(text).first).dy;

Finder navigation(String label) =>
    find.descendant(of: find.byType(NavigationBar), matching: find.text(label));

Future<void> zurZeitleiste(WidgetTester tester, MockApiClient mock) async {
  grosserBildschirm(tester);
  await tester.pumpWidget(testApp(mock));
  await tester.pumpAndSettle();
  await zuGesundheit(tester);
}

/// Schreibt mit, welche Haptik die App auslöst.
List<String> haptikMitschreiben(WidgetTester tester) {
  final aufrufe = <String>[];
  final bote = tester.binding.defaultBinaryMessenger;
  bote.setMockMethodCallHandler(SystemChannels.platform, (aufruf) async {
    if (aufruf.method == 'HapticFeedback.vibrate') {
      aufrufe.add(aufruf.arguments as String);
    }
    return null;
  });
  addTearDown(
    () => bote.setMockMethodCallHandler(SystemChannels.platform, null),
  );
  return aufrufe;
}

const leicht = 'HapticFeedbackType.lightImpact';
const auswahl = 'HapticFeedbackType.selectionClick';

void main() {
  group('Container Transform', () {
    testWidgets('an der aktiven Kachel und an jedem Eintrag', (tester) async {
      grosserBildschirm(tester);
      await tester.pumpWidget(testApp(schnellerMock()));
      await tester.pumpAndSettle();

      // Nur Gesundheit ist aktiv; inaktive Kacheln öffnen nichts.
      expect(find.byType(OpenContainer<void>), findsOneWidget);
      expect(
        find.ancestor(
          of: find.text('Gesundheit'),
          matching: find.byType(OpenContainer<void>),
        ),
        findsOneWidget,
      );

      await zuGesundheit(tester);
      expect(
        find.descendant(
          of: find.byType(TimelineScreen),
          matching: find.byType(OpenContainer<void>),
        ),
        findsNWidgets(3),
      );
    });

    testWidgets('Eintrag öffnen und schließen, Zeitleiste bleibt', (
      tester,
    ) async {
      await zurZeitleiste(tester, schnellerMock());
      await tester.tap(find.text(labor));
      await tester.pumpAndSettle();
      expect(find.byType(EntryScreen), findsOneWidget);

      await zurueck(tester);
      expect(find.byType(EntryScreen), findsNothing);
      expect(find.text(labor), findsOneWidget);
      expect(tester.takeException(), isNull);
    });

    for (final (name, mock) in [
      ('ohne Verzögerung', schnellerMock),
      // Neuladen fällt mitten in die Rücklauf-Animation.
      (
        'Neuladen während des Schließens',
        () =>
            MockApiClient(delay: Durations.short2, uploadDelay: Duration.zero),
      ),
    ]) {
      testWidgets('Löschen aus dem geöffneten Eintrag ($name): schließt ohne '
          'Exception, Eintrag fehlt danach', (tester) async {
        await zurZeitleiste(tester, mock());
        await tester.tap(find.text(labor));
        await tester.pumpAndSettle();

        await tester.tap(find.byTooltip('Eintrag löschen'));
        await tester.pumpAndSettle();
        await tester.tap(find.widgetWithText(FilledButton, 'Löschen'));
        await tester.pumpAndSettle();

        expect(tester.takeException(), isNull);
        expect(find.byType(EntryScreen), findsNothing);
        expect(find.byType(TimelineScreen), findsOneWidget);
        expect(find.text(labor), findsNothing);
        expect(find.text('Oktober 2026'), findsNothing); // Gruppe leer
        expect(find.text(arzt), findsOneWidget);
      });
    }

    testWidgets('Datumskorrektur: nach dem Schließen in der richtigen '
        'Monatsgruppe', (tester) async {
      await zurZeitleiste(tester, schnellerMock());
      // Ausgangslage: Laborbefund 14.10.2026 unter „Oktober 2026“.
      expect(oben(tester, 'Oktober 2026'), lessThan(oben(tester, labor)));

      await tester.tap(find.text(labor));
      await tester.pumpAndSettle();
      await tester.tap(find.byTooltip('Dokumentdatum korrigieren'));
      await tester.pumpAndSettle();
      await tester.tap(find.text('Datum wählen'));
      await tester.pumpAndSettle();
      // Einen Monat zurück, den 15. wählen: 15.09.2026.
      await tester.tap(find.byIcon(Icons.chevron_left));
      await tester.pumpAndSettle();
      await tester.tap(find.text('15'));
      await tester.tap(find.text('OK'));
      await tester.pumpAndSettle();
      expect(find.text('15.09.2026'), findsOneWidget);

      await zurueck(tester);
      expect(tester.takeException(), isNull);
      // „Oktober 2026“ ist leer und verschwunden; der Laborbefund steht
      // unter „September 2026“, vor dem Arztbrief vom 02.09.
      expect(find.text('Oktober 2026'), findsNothing);
      expect(oben(tester, 'September 2026'), lessThan(oben(tester, labor)));
      expect(oben(tester, labor), lessThan(oben(tester, arzt)));
      expect(oben(tester, arzt), lessThan(oben(tester, 'Ohne Datum')));
    });

    testWidgets('„Animationen entfernen“: öffnet ohne Übergang', (
      tester,
    ) async {
      tester.platformDispatcher.accessibilityFeaturesTestValue =
          const FakeAccessibilityFeatures(disableAnimations: true);
      addTearDown(
        tester.platformDispatcher.clearAccessibilityFeaturesTestValue,
      );
      await zurZeitleiste(tester, schnellerMock());

      await tester.tap(find.text(labor));
      await tester.pump();
      await tester.pump();
      expect(find.byType(EntryScreen), findsOneWidget);
      await tester.pumpAndSettle();
    });
  });

  testWidgets('iOS: kein Container Transform, Wisch-zurück-Geste bleibt', (
    tester,
  ) async {
    debugDefaultTargetPlatformOverride = TargetPlatform.iOS;
    try {
      await zurZeitleiste(tester, schnellerMock());
      expect(find.byType(OpenContainer<void>), findsNothing);
      expect(find.byType(TimelineScreen), findsOneWidget);

      // Eintrag öffnen und per Wischen vom linken Rand zurück.
      await tester.tap(find.text(labor));
      await tester.pumpAndSettle();
      expect(find.byType(EntryScreen), findsOneWidget);
      await tester.dragFrom(const Offset(2, 400), const Offset(500, 0));
      await tester.pumpAndSettle();
      expect(find.byType(EntryScreen), findsNothing);

      // Auch zurück zu den Bereichen per Wischen.
      await tester.dragFrom(const Offset(2, 400), const Offset(500, 0));
      await tester.pumpAndSettle();
      expect(find.byType(TimelineScreen), findsNothing);
      expect(find.text('Bald verfügbar'), findsNWidgets(3));
    } finally {
      debugDefaultTargetPlatformOverride = null;
    }
  });

  group('Haptik', () {
    testWidgets('Tab-Wechsel per Tipp und Wischen: je einmal', (tester) async {
      final haptik = haptikMitschreiben(tester);
      await zurZeitleiste(tester, schnellerMock());

      await tester.tap(navigation('Hochladen'));
      await tester.pumpAndSettle();
      expect(haptik, [auswahl]);

      // Derselbe Tab noch einmal: kein Wechsel, keine Haptik.
      await tester.tap(navigation('Hochladen'));
      await tester.pumpAndSettle();
      expect(haptik, [auswahl]);

      // Sprung über eine Seite: trotzdem nur einmal.
      await tester.tap(navigation('Zeitleiste'));
      await tester.pumpAndSettle();
      await tester.tap(navigation('Fragen'));
      await tester.pumpAndSettle();
      expect(haptik, [auswahl, auswahl, auswahl]);

      await tester.fling(find.byType(PageView), const Offset(300, 0), 1000);
      await tester.pumpAndSettle();
      expect(haptik, [auswahl, auswahl, auswahl, auswahl]);
    });

    testWidgets('Upload-Erfolg: leichter Impuls, kein Tab-Klick beim '
        'automatischen Wechsel', (tester) async {
      final haptik = haptikMitschreiben(tester);
      grosserBildschirm(tester);
      final dateien = FakeDateiAuswahl()..pdf = testPdf('haptik.pdf');
      await tester.pumpWidget(testApp(schnellerMock(), auswahl: dateien));
      await tester.pumpAndSettle();
      await zuGesundheit(tester);
      await tester.tap(navigation('Hochladen'));
      await tester.pumpAndSettle();
      haptik.clear();

      await tester.tap(find.text('PDF auswählen'));
      await tester.pumpAndSettle();
      await tester.tap(find.widgetWithText(FilledButton, 'Hochladen'));
      await tester.pumpAndSettle();

      expect(find.byType(EntryScreen), findsOneWidget);
      expect(haptik, [leicht]);
    });

    testWidgets('Korrektur gespeichert und Eintrag gelöscht: je ein Impuls', (
      tester,
    ) async {
      final haptik = haptikMitschreiben(tester);
      await zurZeitleiste(tester, schnellerMock());
      await tester.tap(find.text(labor));
      await tester.pumpAndSettle();

      await tester.tap(find.byTooltip('Quelle korrigieren'));
      await tester.pumpAndSettle();
      await tester.enterText(
        find.widgetWithText(TextField, 'Quelle'),
        'Labor Beispiel',
      );
      await tester.tap(find.widgetWithText(FilledButton, 'Speichern'));
      await tester.pumpAndSettle();
      expect(haptik, [leicht]);

      await tester.tap(find.byTooltip('Eintrag löschen'));
      await tester.pumpAndSettle();
      await tester.tap(find.widgetWithText(FilledButton, 'Löschen'));
      await tester.pumpAndSettle();
      expect(haptik, [leicht, leicht]);
    });

    testWidgets('Fehler beim Speichern: keine Erfolgs-Haptik', (tester) async {
      final haptik = haptikMitschreiben(tester);
      final mock = schnellerMock();
      await zurZeitleiste(tester, mock);
      await tester.tap(find.text(labor));
      await tester.pumpAndSettle();

      await tester.tap(find.byTooltip('Quelle korrigieren'));
      await tester.pumpAndSettle();
      await tester.enterText(find.widgetWithText(TextField, 'Quelle'), 'X');
      mock.simulateError = true;
      await tester.tap(find.widgetWithText(FilledButton, 'Speichern'));
      await tester.pumpAndSettle();

      expect(haptik, isEmpty);
    });
  });
}
