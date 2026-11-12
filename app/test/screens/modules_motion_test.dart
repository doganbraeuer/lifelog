import 'package:flutter/material.dart';
import 'package:flutter_test/flutter_test.dart';

import '../helpers.dart';

double opacityOf(WidgetTester tester, String moduleId) => tester
    .widget<FadeTransition>(find.byKey(ValueKey('fade-in-$moduleId')))
    .opacity
    .value;

double scaleOf(WidgetTester tester, String moduleId) =>
    tester.widget<AnimatedScale>(find.byKey(ValueKey('press-$moduleId'))).scale;

void main() {
  testWidgets('tiles fade in on start', (tester) async {
    await tester.pumpWidget(testApp(fastMock()));

    // Not fully visible in the first frame yet ...
    expect(opacityOf(tester, 'health'), lessThan(1));
    // ... but after fading in.
    await tester.pumpAndSettle();
    for (final id in ['health', 'nutrition', 'recipes', 'sport']) {
      expect(opacityOf(tester, id), 1, reason: id);
    }
  });

  testWidgets('„Animationen entfernen“: fully visible at once', (tester) async {
    tester.platformDispatcher.accessibilityFeaturesTestValue =
        const FakeAccessibilityFeatures(disableAnimations: true);
    addTearDown(tester.platformDispatcher.clearAccessibilityFeaturesTestValue);

    await tester.pumpWidget(testApp(fastMock()));
    await tester.pump();

    for (final id in ['health', 'nutrition', 'recipes', 'sport']) {
      expect(opacityOf(tester, id), 1, reason: id);
    }
    // No animation is running any more (the frame is still).
    expect(tester.binding.hasScheduledFrame, isFalse);
  });

  testWidgets('press effect: active tile gets smaller while pressed', (
    tester,
  ) async {
    await tester.pumpWidget(testApp(fastMock()));
    await tester.pumpAndSettle();

    final gesture = await tester.startGesture(
      tester.getCenter(find.text('Gesundheit')),
    );
    await tester.pump(Durations.medium2);
    expect(scaleOf(tester, 'health'), 0.97);

    await gesture.cancel();
    await tester.pumpAndSettle();
    expect(scaleOf(tester, 'health'), 1);
  });

  testWidgets('inactive tile: no press effect', (tester) async {
    await tester.pumpWidget(testApp(fastMock()));
    await tester.pumpAndSettle();

    final gesture = await tester.startGesture(
      tester.getCenter(find.text('Sport')),
    );
    await tester.pump(Durations.medium2);
    expect(scaleOf(tester, 'sport'), 1);
    await gesture.cancel();
  });
}
