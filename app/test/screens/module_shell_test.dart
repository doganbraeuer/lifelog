import 'dart:convert';
import 'dart:typed_data';

import 'package:flutter/material.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:lifelog/api/api_client.dart';
import 'package:lifelog/api/mock_api_client.dart';
import 'package:lifelog/screens/entry_screen.dart';
import 'package:lifelog/screens/question_screen.dart';
import 'package:lifelog/screens/upload_screen.dart';
import 'package:lifelog/screens/timeline_screen.dart';

import '../helpers.dart';

/// Which page the navigation bar marks as selected.
int selectedTab(WidgetTester tester) =>
    tester.widget<NavigationBar>(find.byType(NavigationBar)).selectedIndex;

/// Which page is visible. Invisible pages (kept alive) do not count for
/// the finders.
int visiblePage(WidgetTester tester) {
  final pages = [
    find.byType(TimelineScreen),
    find.byType(UploadScreen),
    find.byType(QuestionScreen),
  ];
  final hit = [
    for (var i = 0; i < pages.length; i++)
      if (pages[i].evaluate().isNotEmpty) i,
  ];
  expect(hit, hasLength(1), reason: 'exactly one page visible');
  return hit.single;
}

/// Swipe: to the left pages forward, to the right back.
Future<void> swipe(WidgetTester tester, {required bool toLeft}) async {
  await tester.fling(
    find.byType(PageView),
    Offset(toLeft ? -300 : 300, 0),
    1000,
  );
  await tester.pumpAndSettle();
}

/// Runs a page animation frame by frame (0.6 s). Needed where
/// pumpAndSettle() does not work: a single pump(400 ms) only produces one
/// frame in which the animation is only just starting.
Future<void> waitForAnimation(WidgetTester tester) async {
  for (var i = 0; i < 30; i++) {
    await tester.pump(const Duration(milliseconds: 20));
  }
}

Finder navItem(String label) =>
    find.descendant(of: find.byType(NavigationBar), matching: find.text(label));

Future<void> startApp(
  WidgetTester tester,
  MockApiClient mock, {
  FakeFileChooser? chooser,
}) async {
  await tester.pumpWidget(testApp(mock, chooser: chooser));
  await tester.pumpAndSettle();
  await openHealth(tester);
}

void main() {
  group('swiping and navigation bar', () {
    testWidgets('swiping changes the page, the bar follows', (tester) async {
      await startApp(tester, fastMock());
      expect((visiblePage(tester), selectedTab(tester)), (0, 0));

      await swipe(tester, toLeft: true);
      expect((visiblePage(tester), selectedTab(tester)), (1, 1));

      await swipe(tester, toLeft: true);
      expect((visiblePage(tester), selectedTab(tester)), (2, 2));

      // At the end it goes no further.
      await swipe(tester, toLeft: true);
      expect((visiblePage(tester), selectedTab(tester)), (2, 2));

      await swipe(tester, toLeft: false);
      expect((visiblePage(tester), selectedTab(tester)), (1, 1));
    });

    testWidgets('tapping in the bar pages to the page', (tester) async {
      await startApp(tester, fastMock());

      await tester.tap(navItem('Hochladen'));
      await tester.pumpAndSettle();
      expect((visiblePage(tester), selectedTab(tester)), (1, 1));

      await tester.tap(navItem('Zeitleiste'));
      await tester.pumpAndSettle();
      expect((visiblePage(tester), selectedTab(tester)), (0, 0));
    });

    testWidgets('jump over a page: the bar does not flicker', (tester) async {
      await startApp(tester, fastMock());

      await tester.tap(navItem('Fragen'));
      // During the animation the PageView passes „Hochladen“;
      // the bar should still show „Fragen“ throughout.
      for (var i = 0; i < 25; i++) {
        await tester.pump(const Duration(milliseconds: 16));
        expect(selectedTab(tester), 2, reason: 'Frame $i');
      }
      await tester.pumpAndSettle();
      expect(visiblePage(tester), 2);
    });
  });

  group('state is kept when switching', () {
    testWidgets('sort of the timeline', (tester) async {
      await startApp(tester, fastMock());

      await tester.tap(find.byIcon(Icons.sort));
      await tester.pumpAndSettle();
      await tester.tap(
        find.widgetWithText(CheckedPopupMenuItem<Sort>, 'Nach Importdatum'),
      );
      await tester.pumpAndSettle();
      await tester.tap(find.byIcon(Icons.arrow_downward));
      await tester.pumpAndSettle();

      await swipe(tester, toLeft: true);
      await swipe(tester, toLeft: true);
      await tester.tap(navItem('Zeitleiste'));
      await tester.pumpAndSettle();

      expect(find.text('Importdatum · älteste zuerst'), findsOneWidget);
    });

    testWidgets('scroll position of the timeline', (tester) async {
      // Enough entries so that the list scrolls.
      final mock = fastMock();
      for (var i = 0; i < 20; i++) {
        await mock.upload(
          Uint8List.fromList(utf8.encode('%PDF-1.4 Nr. $i')),
          'dokument_$i.pdf',
          module: 'health',
        );
      }
      await startApp(tester, mock);

      Finder readList() => find
          .descendant(
            of: find.byType(TimelineScreen),
            matching: find.byType(Scrollable),
          )
          .first;
      double position() =>
          tester.state<ScrollableState>(readList()).position.pixels;

      await tester.drag(readList(), const Offset(0, -400));
      await tester.pumpAndSettle();
      final before = position();
      expect(before, greaterThan(0));

      await swipe(tester, toLeft: true);
      await swipe(tester, toLeft: true);
      await swipe(tester, toLeft: false);
      await swipe(tester, toLeft: false);

      expect(visiblePage(tester), 0);
      expect(position(), before);
    });

    testWidgets('chosen file on „Hochladen“', (tester) async {
      final chooser = FakeFileChooser()..pdf = testPdf('befund.pdf');
      await startApp(tester, fastMock(), chooser: chooser);
      await tester.tap(navItem('Hochladen'));
      await tester.pumpAndSettle();
      await tester.tap(find.text('PDF auswählen'));
      await tester.pumpAndSettle();

      await swipe(tester, toLeft: false); // to the timeline
      await swipe(tester, toLeft: true); // back

      expect(find.text('befund.pdf'), findsOneWidget);
      expect(find.widgetWithText(FilledButton, 'Hochladen'), findsOneWidget);
    });

    testWidgets('last answer on „Fragen“', (tester) async {
      await startApp(tester, fastMock());
      await tester.tap(navItem('Fragen'));
      await tester.pumpAndSettle();
      await tester.enterText(find.byType(TextField), 'Hämoglobin');
      await tester.pump();
      await tester.tap(find.byTooltip('Frage senden'));
      await tester.pumpAndSettle();
      expect(find.text('Quellen'), findsOneWidget);

      await tester.tap(navItem('Zeitleiste'));
      await tester.pumpAndSettle();
      await swipe(tester, toLeft: true);
      await swipe(tester, toLeft: true);

      expect(visiblePage(tester), 2);
      expect(find.text('„Hämoglobin“'), findsOneWidget);
      expect(find.text('Quellen'), findsOneWidget);
    });
  });

  testWidgets('a running upload survives the switch; then timeline '
      'and new entry', (tester) async {
    final mock = MockApiClient(
      delay: Duration.zero,
      uploadDelay: const Duration(seconds: 2),
    );
    final chooser = FakeFileChooser()..pdf = testPdf('neu.pdf');
    useLargeScreen(tester); // the entry view is longer than 600 px
    await startApp(tester, mock, chooser: chooser);
    await tester.tap(navItem('Hochladen'));
    await tester.pumpAndSettle();
    await tester.tap(find.text('PDF auswählen'));
    await tester.pumpAndSettle();

    // Start the upload. While it runs, the progress indicator spins endlessly;
    // that is why pump(duration) is used here instead of pumpAndSettle().
    await tester.tap(find.widgetWithText(FilledButton, 'Hochladen'));
    await tester.pump(const Duration(milliseconds: 100));
    expect(find.textContaining('bis zu einer Minute'), findsOneWidget);

    // To „Fragen“ and back: the upload keeps running.
    await tester.tap(navItem('Fragen'));
    await waitForAnimation(tester);
    expect(visiblePage(tester), 2);
    await tester.tap(navItem('Hochladen'));
    await waitForAnimation(tester);
    expect(find.textContaining('bis zu einer Minute'), findsOneWidget);

    // To „Fragen“ again and let it finish there.
    await tester.tap(navItem('Fragen'));
    await tester.pump(const Duration(seconds: 2));
    await tester.pumpAndSettle();

    // The shell opened the entry via _timeline.currentState ...
    expect(find.byType(EntryScreen), findsOneWidget);
    expect(find.text('Blutdruck'), findsOneWidget);

    // ... and switched to the timeline, which has been reloaded.
    await tapBack(tester);
    expect((visiblePage(tester), selectedTab(tester)), (0, 0));
    expect(find.text('Mock-Praxis Beispiel'), findsOneWidget);
  });

  testWidgets('duplicate: „Vorhandenen Eintrag öffnen“ switches to the '
      'timeline', (tester) async {
    final mock = fastMock();
    await mock.upload(
      testPdf('doppelt.pdf').bytes,
      'doppelt.pdf',
      module: 'health',
    );
    final chooser = FakeFileChooser()..pdf = testPdf('doppelt.pdf');
    await startApp(tester, mock, chooser: chooser);
    await tester.tap(navItem('Hochladen'));
    await tester.pumpAndSettle();
    await tester.tap(find.text('PDF auswählen'));
    await tester.pumpAndSettle();
    await tester.tap(find.widgetWithText(FilledButton, 'Hochladen'));
    await tester.pumpAndSettle();

    await tester.tap(find.text('Vorhandenen Eintrag öffnen'));
    await tester.pumpAndSettle();
    expect(find.byType(EntryScreen), findsOneWidget);

    await tapBack(tester);
    expect((visiblePage(tester), selectedTab(tester)), (0, 0));
  });
}
