import 'package:flutter/material.dart';
import 'package:flutter_test/flutter_test.dart';

import '../hilfen.dart';

double deckkraft(WidgetTester tester, String modulId) => tester
    .widget<FadeTransition>(find.byKey(ValueKey('einblenden-$modulId')))
    .opacity
    .value;

double skalierung(WidgetTester tester, String modulId) =>
    tester.widget<AnimatedScale>(find.byKey(ValueKey('druck-$modulId'))).scale;

void main() {
  testWidgets('Kacheln blenden beim Start ein', (tester) async {
    await tester.pumpWidget(testApp(schnellerMock()));

    // Im ersten Bild noch nicht voll sichtbar ...
    expect(deckkraft(tester, 'gesundheit'), lessThan(1));
    // ... nach dem Einblenden schon.
    await tester.pumpAndSettle();
    for (final id in ['gesundheit', 'ernaehrung', 'rezepte', 'sport']) {
      expect(deckkraft(tester, id), 1, reason: id);
    }
  });

  testWidgets('„Animationen entfernen“: sofort vollständig sichtbar', (
    tester,
  ) async {
    tester.platformDispatcher.accessibilityFeaturesTestValue =
        const FakeAccessibilityFeatures(disableAnimations: true);
    addTearDown(tester.platformDispatcher.clearAccessibilityFeaturesTestValue);

    await tester.pumpWidget(testApp(schnellerMock()));
    await tester.pump();

    for (final id in ['gesundheit', 'ernaehrung', 'rezepte', 'sport']) {
      expect(deckkraft(tester, id), 1, reason: id);
    }
    // Es läuft keine Animation mehr (Bild steht).
    expect(tester.binding.hasScheduledFrame, isFalse);
  });

  testWidgets('Druck-Effekt: aktive Kachel wird beim Drücken kleiner', (
    tester,
  ) async {
    await tester.pumpWidget(testApp(schnellerMock()));
    await tester.pumpAndSettle();

    final finger = await tester.startGesture(
      tester.getCenter(find.text('Gesundheit')),
    );
    await tester.pump(Durations.medium2);
    expect(skalierung(tester, 'gesundheit'), 0.97);

    await finger.cancel();
    await tester.pumpAndSettle();
    expect(skalierung(tester, 'gesundheit'), 1);
  });

  testWidgets('inaktive Kachel: kein Druck-Effekt', (tester) async {
    await tester.pumpWidget(testApp(schnellerMock()));
    await tester.pumpAndSettle();

    final finger = await tester.startGesture(
      tester.getCenter(find.text('Sport')),
    );
    await tester.pump(Durations.medium2);
    expect(skalierung(tester, 'sport'), 1);
    await finger.cancel();
  });
}
