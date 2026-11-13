// Stage V2: skeletons, scan animation, staggered fade-in and
// „Animationen entfernen“ (remove animations).

import 'package:flutter/material.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:lifelog/api/mock_api_client.dart';
import 'package:lifelog/screens/question_screen.dart';
import 'package:lifelog/screens/upload_screen.dart';
import 'package:lifelog/screens/timeline_screen.dart';

import '../helpers.dart';

/// Mock with a noticeable delay, so that loading states are visible.
MockApiClient slowMock() => MockApiClient(
  delay: Durations.extralong4,
  uploadDelay: Durations.extralong4 * 2,
);

void turnOffAnimations(WidgetTester tester) {
  tester.platformDispatcher.accessibilityFeaturesTestValue =
      const FakeAccessibilityFeatures(disableAnimations: true);
  addTearDown(tester.platformDispatcher.clearAccessibilityFeaturesTestValue);
}

Finder navItem(String label) =>
    find.descendant(of: find.byType(NavigationBar), matching: find.text(label));

/// Opens health without waiting for the timeline to load.
Future<void> openHealthModule(WidgetTester tester, MockApiClient mock) async {
  useLargeScreen(tester);
  await tester.pumpWidget(testApp(mock));
  await tester.pumpAndSettle();
  await tester.tap(find.text('Gesundheit'));
  // Wait for the page transition, but not for loading (takes 1 s).
  for (var i = 0; i < 10; i++) {
    await tester.pump(Durations.short1);
  }
}

void main() {
  group('timeline loading', () {
    testWidgets('skeleton instead of a spinner, then the list', (tester) async {
      final semantics = tester.ensureSemantics();
      await openHealthModule(tester, slowMock());

      final timeline = find.byType(TimelineScreen);
      expect(
        find.descendant(
          of: timeline,
          matching: find.byType(CircularProgressIndicator),
        ),
        findsNothing,
      );
      expect(find.bySemanticsLabel('Wird geladen'), findsOneWidget);

      await tester.pump(Durations.extralong4);
      await tester.pumpAndSettle();
      expect(find.bySemanticsLabel('Wird geladen'), findsNothing);
      expect(find.text('Oktober 2026'), findsOneWidget);
      semantics.dispose();
    });

    testWidgets('„Animationen entfernen“: skeleton stays still', (
      tester,
    ) async {
      turnOffAnimations(tester);
      await openHealthModule(tester, slowMock());

      expect(find.bySemanticsLabel('Wird geladen'), findsOneWidget);
      expect(tester.binding.hasScheduledFrame, isFalse);

      await tester.pump(Durations.extralong4);
      await tester.pumpAndSettle();
    });
  });

  group('question', () {
    Future<void> ask(WidgetTester tester, MockApiClient mock) async {
      useLargeScreen(tester);
      await tester.pumpWidget(testApp(mock));
      await tester.pumpAndSettle();
      await openHealth(tester);
      await tester.tap(navItem('Fragen'));
      await tester.pumpAndSettle();
      await tester.enterText(find.byType(TextField), 'Hämoglobin');
      await tester.pump();
      await tester.tap(find.byTooltip('Frage senden'));
      await tester.pump();
    }

    double opacityOf(WidgetTester tester, int i) => tester
        .widget<FadeTransition>(find.byKey(ValueKey('staggered-$i')))
        .opacity
        .value;

    testWidgets('search shows a skeleton, no spinner', (tester) async {
      final semantics = tester.ensureSemantics();
      await ask(tester, slowMock());
      await tester.pump(Durations.short4);

      expect(find.text('Suche in deinen Dokumenten …'), findsOneWidget);
      expect(find.bySemanticsLabel('Wird geladen'), findsOneWidget);
      expect(
        find.descendant(
          of: find.byType(QuestionScreen),
          matching: find.byType(CircularProgressIndicator),
        ),
        findsNothing,
      );

      await tester.pump(Durations.extralong4);
      await tester.pumpAndSettle();
      expect(find.text('Quellen'), findsOneWidget);
      semantics.dispose();
    });

    testWidgets('answer and sources fade in one after another', (tester) async {
      await ask(tester, fastMock());
      await tester.pump();

      // Shortly after the start: the answer is further along than the sources.
      await tester.pump(Durations.short3);
      final answer = opacityOf(tester, 0);
      final firstSource = opacityOf(tester, 2);
      expect(answer, greaterThan(firstSource));
      // The text is still complete already (no typewriter effect).
      expect(find.textContaining('Hämoglobin 14,2 g/dl'), findsOneWidget);

      await tester.pumpAndSettle();
      expect(opacityOf(tester, 0), 1);
      expect(opacityOf(tester, 2), 1);
    });

    testWidgets('„Animationen entfernen“: fully visible at once', (
      tester,
    ) async {
      turnOffAnimations(tester);
      await ask(tester, fastMock());
      await tester.pump();

      expect(opacityOf(tester, 0), 1);
      expect(opacityOf(tester, 2), 1);
    });
  });

  group('upload', () {
    Future<void> startUpload(WidgetTester tester, MockApiClient mock) async {
      useLargeScreen(tester);
      final chooser = FakeFileChooser()..pdf = testPdf('scan.pdf');
      await tester.pumpWidget(testApp(mock, chooser: chooser));
      await tester.pumpAndSettle();
      await openHealth(tester);
      await tester.tap(navItem('Hochladen'));
      await tester.pumpAndSettle();
      await tester.tap(find.text('PDF auswählen'));
      await tester.pumpAndSettle();
      await tester.tap(find.widgetWithText(FilledButton, 'Hochladen'));
      await tester.pump();
    }

    testWidgets(
      'scan animation instead of a spinner, with text and semantics',
      (tester) async {
        final semantics = tester.ensureSemantics();
        await startUpload(tester, slowMock());
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
        // Runs while processing.
        expect(tester.binding.hasScheduledFrame, isTrue);

        await tester.pump(Durations.extralong4 * 2);
        await tester.pumpAndSettle();
        semantics.dispose();
      },
    );

    testWidgets('„Animationen entfernen“: scan stays still', (tester) async {
      turnOffAnimations(tester);
      await startUpload(tester, slowMock());
      await tester.pump();

      expect(find.textContaining('bis zu einer Minute'), findsOneWidget);
      expect(tester.binding.hasScheduledFrame, isFalse);

      await tester.pump(Durations.extralong4 * 2);
      await tester.pumpAndSettle();
    });

    testWidgets('error card appears softly and disappears again', (
      tester,
    ) async {
      useLargeScreen(tester);
      final chooser = FakeFileChooser()..pdf = testPdf('modelerror.pdf');
      await tester.pumpWidget(testApp(fastMock(), chooser: chooser));
      await tester.pumpAndSettle();
      await openHealth(tester);
      await tester.tap(navItem('Hochladen'));
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

  group('phone size (1080 × 2400, factor 3)', () {
    testWidgets('skeleton, scan and answer do not overflow', (tester) async {
      tester.view.physicalSize = const Size(1080, 2400);
      tester.view.devicePixelRatio = 3;
      addTearDown(tester.view.reset);
      final chooser = FakeFileChooser()..pdf = testPdf('handy.pdf');
      final mock = slowMock();

      await tester.pumpWidget(testApp(mock, chooser: chooser));
      await tester.pumpAndSettle();
      await tester.tap(find.text('Gesundheit'));
      for (var i = 0; i < 10; i++) {
        await tester.pump(Durations.short1);
      }
      expect(tester.takeException(), isNull); // skeleton
      await tester.pump(Durations.extralong4);
      await tester.pumpAndSettle();

      await tester.tap(navItem('Hochladen'));
      await tester.pumpAndSettle();
      await tester.tap(find.text('PDF auswählen'));
      await tester.pumpAndSettle();
      await tester.tap(find.widgetWithText(FilledButton, 'Hochladen'));
      await tester.pump(Durations.medium2);
      expect(tester.takeException(), isNull); // scan
      await tester.pump(Durations.extralong4 * 2);
      await tester.pumpAndSettle();
      expect(tester.takeException(), isNull);
    });
  });
}
