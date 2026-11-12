// Stage V3: container transform, iOS back gesture and haptics.

import 'package:animations/animations.dart';
import 'package:flutter/foundation.dart';
import 'package:flutter/material.dart';
import 'package:flutter/services.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:lifelog/api/mock_api_client.dart';
import 'package:lifelog/screens/entry_screen.dart';
import 'package:lifelog/screens/timeline_screen.dart';

import '../helpers.dart';

const labIssuer = 'Labor Muster GmbH, Salzburg';
const doctorIssuer = 'Dr. Erika Beispiel, Allgemeinmedizin';

double topOf(WidgetTester tester, String text) =>
    tester.getTopLeft(find.textContaining(text).first).dy;

Finder navItem(String label) =>
    find.descendant(of: find.byType(NavigationBar), matching: find.text(label));

Future<void> goToTimeline(WidgetTester tester, MockApiClient mock) async {
  useLargeScreen(tester);
  await tester.pumpWidget(testApp(mock));
  await tester.pumpAndSettle();
  await openHealth(tester);
}

/// Records which haptic feedback the app triggers.
List<String> recordHaptics(WidgetTester tester) {
  final calls = <String>[];
  final messenger = tester.binding.defaultBinaryMessenger;
  messenger.setMockMethodCallHandler(SystemChannels.platform, (
    methodCall,
  ) async {
    if (methodCall.method == 'HapticFeedback.vibrate') {
      calls.add(methodCall.arguments as String);
    }
    return null;
  });
  addTearDown(
    () => messenger.setMockMethodCallHandler(SystemChannels.platform, null),
  );
  return calls;
}

const light = 'HapticFeedbackType.lightImpact';
const chooser = 'HapticFeedbackType.selectionClick';

void main() {
  group('container transform', () {
    testWidgets('on the active tile and on every entry', (tester) async {
      useLargeScreen(tester);
      await tester.pumpWidget(testApp(fastMock()));
      await tester.pumpAndSettle();

      // Only health is active; inactive tiles open nothing.
      expect(find.byType(OpenContainer<void>), findsOneWidget);
      expect(
        find.ancestor(
          of: find.text('Gesundheit'),
          matching: find.byType(OpenContainer<void>),
        ),
        findsOneWidget,
      );

      await openHealth(tester);
      expect(
        find.descendant(
          of: find.byType(TimelineScreen),
          matching: find.byType(OpenContainer<void>),
        ),
        findsNWidgets(3),
      );
    });

    testWidgets('open and close an entry, the timeline stays', (tester) async {
      await goToTimeline(tester, fastMock());
      await tester.tap(find.text(labIssuer));
      await tester.pumpAndSettle();
      expect(find.byType(EntryScreen), findsOneWidget);

      await tapBack(tester);
      expect(find.byType(EntryScreen), findsNothing);
      expect(find.text(labIssuer), findsOneWidget);
      expect(tester.takeException(), isNull);
    });

    for (final (name, mock) in [
      ('ohne Verzögerung', fastMock),
      // The reload falls in the middle of the reverse animation.
      (
        'Neuladen während des Schließens',
        () =>
            MockApiClient(delay: Durations.short2, uploadDelay: Duration.zero),
      ),
    ]) {
      testWidgets('delete from the opened entry ($name): closes without an '
          'exception, the entry is gone afterwards', (tester) async {
        await goToTimeline(tester, mock());
        await tester.tap(find.text(labIssuer));
        await tester.pumpAndSettle();

        await tester.tap(find.byTooltip('Eintrag löschen'));
        await tester.pumpAndSettle();
        await tester.tap(find.widgetWithText(FilledButton, 'Löschen'));
        await tester.pumpAndSettle();

        expect(tester.takeException(), isNull);
        expect(find.byType(EntryScreen), findsNothing);
        expect(find.byType(TimelineScreen), findsOneWidget);
        expect(find.text(labIssuer), findsNothing);
        expect(find.text('Oktober 2026'), findsNothing); // group empty
        expect(find.text(doctorIssuer), findsOneWidget);
      });
    }

    testWidgets('date correction: after closing in the right '
        'month group', (tester) async {
      await goToTimeline(tester, fastMock());
      // Starting point: lab report 14.10.2026 under „Oktober 2026“.
      expect(topOf(tester, 'Oktober 2026'), lessThan(topOf(tester, labIssuer)));

      await tester.tap(find.text(labIssuer));
      await tester.pumpAndSettle();
      await tester.tap(find.byTooltip('Dokumentdatum korrigieren'));
      await tester.pumpAndSettle();
      await tester.tap(find.text('Datum wählen'));
      await tester.pumpAndSettle();
      // One month back, pick the 15th: 15.09.2026.
      await tester.tap(find.byIcon(Icons.chevron_left));
      await tester.pumpAndSettle();
      await tester.tap(find.text('15'));
      await tester.tap(find.text('OK'));
      await tester.pumpAndSettle();
      expect(find.text('15.09.2026'), findsOneWidget);

      await tapBack(tester);
      expect(tester.takeException(), isNull);
      // „Oktober 2026“ is empty and gone; the lab report is under
      // „September 2026“, before the doctor letter from 02.09.
      expect(find.text('Oktober 2026'), findsNothing);
      expect(
        topOf(tester, 'September 2026'),
        lessThan(topOf(tester, labIssuer)),
      );
      expect(topOf(tester, labIssuer), lessThan(topOf(tester, doctorIssuer)));
      expect(
        topOf(tester, doctorIssuer),
        lessThan(topOf(tester, 'Ohne Datum')),
      );
    });

    testWidgets('„Animationen entfernen“: opens without a transition', (
      tester,
    ) async {
      tester.platformDispatcher.accessibilityFeaturesTestValue =
          const FakeAccessibilityFeatures(disableAnimations: true);
      addTearDown(
        tester.platformDispatcher.clearAccessibilityFeaturesTestValue,
      );
      await goToTimeline(tester, fastMock());

      await tester.tap(find.text(labIssuer));
      await tester.pump();
      await tester.pump();
      expect(find.byType(EntryScreen), findsOneWidget);
      await tester.pumpAndSettle();
    });
  });

  testWidgets('iOS: no container transform, the swipe-back gesture stays', (
    tester,
  ) async {
    debugDefaultTargetPlatformOverride = TargetPlatform.iOS;
    try {
      await goToTimeline(tester, fastMock());
      expect(find.byType(OpenContainer<void>), findsNothing);
      expect(find.byType(TimelineScreen), findsOneWidget);

      // Open an entry and go back by swiping from the left edge.
      await tester.tap(find.text(labIssuer));
      await tester.pumpAndSettle();
      expect(find.byType(EntryScreen), findsOneWidget);
      await tester.dragFrom(const Offset(2, 400), const Offset(500, 0));
      await tester.pumpAndSettle();
      expect(find.byType(EntryScreen), findsNothing);

      // Back to the modules screen by swiping too.
      await tester.dragFrom(const Offset(2, 400), const Offset(500, 0));
      await tester.pumpAndSettle();
      expect(find.byType(TimelineScreen), findsNothing);
      expect(find.text('Bald verfügbar'), findsNWidgets(3));
    } finally {
      debugDefaultTargetPlatformOverride = null;
    }
  });

  group('haptics', () {
    testWidgets('tab change by tap and swipe: once each', (tester) async {
      final haptics = recordHaptics(tester);
      await goToTimeline(tester, fastMock());

      await tester.tap(navItem('Hochladen'));
      await tester.pumpAndSettle();
      expect(haptics, [chooser]);

      // The same tab again: no change, no haptics.
      await tester.tap(navItem('Hochladen'));
      await tester.pumpAndSettle();
      expect(haptics, [chooser]);

      // Jump over a page: still only once.
      await tester.tap(navItem('Zeitleiste'));
      await tester.pumpAndSettle();
      await tester.tap(navItem('Fragen'));
      await tester.pumpAndSettle();
      expect(haptics, [chooser, chooser, chooser]);

      await tester.fling(find.byType(PageView), const Offset(300, 0), 1000);
      await tester.pumpAndSettle();
      expect(haptics, [chooser, chooser, chooser, chooser]);
    });

    testWidgets('upload success: light impact, no tab click on the '
        'automatic switch', (tester) async {
      final haptics = recordHaptics(tester);
      useLargeScreen(tester);
      final chooser = FakeFileChooser()..pdf = testPdf('haptik.pdf');
      await tester.pumpWidget(testApp(fastMock(), chooser: chooser));
      await tester.pumpAndSettle();
      await openHealth(tester);
      await tester.tap(navItem('Hochladen'));
      await tester.pumpAndSettle();
      haptics.clear();

      await tester.tap(find.text('PDF auswählen'));
      await tester.pumpAndSettle();
      await tester.tap(find.widgetWithText(FilledButton, 'Hochladen'));
      await tester.pumpAndSettle();

      expect(find.byType(EntryScreen), findsOneWidget);
      expect(haptics, [light]);
    });

    testWidgets('correction saved and entry deleted: one impact each', (
      tester,
    ) async {
      final haptics = recordHaptics(tester);
      await goToTimeline(tester, fastMock());
      await tester.tap(find.text(labIssuer));
      await tester.pumpAndSettle();

      await tester.tap(find.byTooltip('Quelle korrigieren'));
      await tester.pumpAndSettle();
      await tester.enterText(
        find.widgetWithText(TextField, 'Quelle'),
        'Labor Beispiel',
      );
      await tester.tap(find.widgetWithText(FilledButton, 'Speichern'));
      await tester.pumpAndSettle();
      expect(haptics, [light]);

      await tester.tap(find.byTooltip('Eintrag löschen'));
      await tester.pumpAndSettle();
      await tester.tap(find.widgetWithText(FilledButton, 'Löschen'));
      await tester.pumpAndSettle();
      expect(haptics, [light, light]);
    });

    testWidgets('error while saving: no success haptics', (tester) async {
      final haptics = recordHaptics(tester);
      final mock = fastMock();
      await goToTimeline(tester, mock);
      await tester.tap(find.text(labIssuer));
      await tester.pumpAndSettle();

      await tester.tap(find.byTooltip('Quelle korrigieren'));
      await tester.pumpAndSettle();
      await tester.enterText(find.widgetWithText(TextField, 'Quelle'), 'X');
      mock.simulateError = true;
      await tester.tap(find.widgetWithText(FilledButton, 'Speichern'));
      await tester.pumpAndSettle();

      expect(haptics, isEmpty);
    });
  });
}
