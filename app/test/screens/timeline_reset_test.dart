// After re-sorting and after a new entry the timeline starts at the top.
// Re-sorting builds a new list instead of moving the rows of the old one
// (moving rows with OpenContainer while scrolling crashed on the phone).
import 'package:flutter/material.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:lifelog/api/mock_api_client.dart';
import 'package:lifelog/screens/entry_screen.dart';
import 'package:lifelog/screens/timeline_screen.dart';

import '../helpers.dart';

/// Phone size like the OnePlus 8T, so that the list needs scrolling.
void usePhoneScreen(WidgetTester tester) {
  tester.view.physicalSize = const Size(1080, 2400);
  tester.view.devicePixelRatio = 3;
  addTearDown(tester.view.reset);
}

/// Mock with the sample data plus 12 notes, one per month of 2026.
Future<MockApiClient> longMock({Duration delay = Duration.zero}) async {
  final mock = MockApiClient(
    delay: delay,
    uploadDelay: Duration.zero,
    clock: () => DateTime.utc(2026, 11, 4, 9, 30, 0),
  );
  for (var month = 1; month <= 12; month++) {
    final mm = month.toString().padLeft(2, '0');
    await mock.importText(
      'Praxis $month\nNotiz vom 10.$mm.2026',
      module: 'health',
    );
  }
  return mock;
}

Future<void> openTimeline(WidgetTester tester, MockApiClient mock) async {
  usePhoneScreen(tester);
  await tester.pumpWidget(testApp(mock));
  await tester.pumpAndSettle();
  await openHealth(tester);
}

/// Scroll offset of the timeline list.
double offset(WidgetTester tester) => tester
    .state<ScrollableState>(
      find
          .descendant(
            of: find.byType(TimelineScreen),
            matching: find.byType(Scrollable),
          )
          .first,
    )
    .position
    .pixels;

Future<void> scrollDown(WidgetTester tester) async {
  await tester.drag(find.byType(ListView).first, const Offset(0, -500));
  await tester.pumpAndSettle();
  expect(offset(tester), greaterThan(0));
}

Future<void> toggleOrder(WidgetTester tester) async {
  final down = find.byIcon(Icons.arrow_downward);
  await tester.tap(
    down.evaluate().isNotEmpty ? down : find.byIcon(Icons.arrow_upward),
  );
}

Future<void> chooseSort(WidgetTester tester, String item) async {
  await tester.tap(find.byTooltip('Sortieren nach'));
  await tester.pumpAndSettle();
  await tester.tap(find.text(item), warnIfMissed: false);
}

void main() {
  testWidgets('toggling the order starts the list at the top', (tester) async {
    await openTimeline(tester, await longMock());
    await scrollDown(tester);

    await toggleOrder(tester);
    await tester.pumpAndSettle();
    expect(offset(tester), 0);
    // Oldest first: January is the first month header.
    expect(find.text('Januar 2026'), findsOneWidget);
    expect(tester.takeException(), isNull);
  });

  testWidgets('choosing another sort starts the list at the top', (
    tester,
  ) async {
    await openTimeline(tester, await longMock());
    await scrollDown(tester);

    await chooseSort(tester, 'Nach Importdatum');
    await tester.pumpAndSettle();
    expect(offset(tester), 0);
    expect(find.text('Importdatum · neueste zuerst'), findsOneWidget);
  });

  testWidgets('a reload with the same sort keeps the scroll position', (
    tester,
  ) async {
    await openTimeline(tester, await longMock());
    await scrollDown(tester);
    final before = offset(tester);

    // As after closing an entry or after a correction.
    await tester
        .state<TimelineScreenState>(find.byType(TimelineScreen))
        .reload();
    await tester.pumpAndSettle();
    expect(offset(tester), before);
  });

  testWidgets('after a text import the timeline starts at the top with the '
      'new entry', (tester) async {
    final mock = await longMock();
    await openTimeline(tester, mock);
    await scrollDown(tester);

    await tester.tap(
      find.descendant(
        of: find.byType(NavigationBar),
        matching: find.text('Hochladen'),
      ),
    );
    await tester.pumpAndSettle();
    await tester.tap(find.text('Text eingeben'));
    await tester.pumpAndSettle();
    await tester.enterText(
      find.widgetWithText(TextField, 'Dokumentinhalt'),
      'Zahnarztpraxis Beispiel\nKontrolle vom 03.11.2026',
    );
    await tester.pump();
    await tester.tap(find.widgetWithText(FilledButton, 'Importieren'));
    await tester.pumpAndSettle();
    expect(find.byType(EntryScreen), findsOneWidget);

    await tapBack(tester);
    expect(offset(tester), 0);
    expect(find.text('November 2026'), findsOneWidget);
    expect(find.text('Zahnarztpraxis Beispiel'), findsOneWidget);
  });

  for (final delay in [Duration.zero, const Duration(milliseconds: 300)]) {
    testWidgets('re-sorting while flinging throws nothing '
        '(delay ${delay.inMilliseconds} ms)', (tester) async {
      await openTimeline(tester, await longMock(delay: delay));

      await chooseSort(tester, 'Nach Importdatum');
      await tester.pumpAndSettle();
      await chooseSort(tester, 'Nach Dokumentdatum');
      await tester.pump(const Duration(milliseconds: 100));
      await toggleOrder(tester);
      await tester.pump(const Duration(milliseconds: 50));
      for (var i = 0; i < 3; i++) {
        await tester.fling(
          find.byType(ListView).last,
          const Offset(0, -400),
          3000,
        );
        await tester.pump(const Duration(milliseconds: 60));
      }
      await tester.pumpAndSettle();
      expect(tester.takeException(), isNull);
      expect(offset(tester), greaterThanOrEqualTo(0));
    });
  }
}
