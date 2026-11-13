import 'package:flutter/material.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:lifelog/api/mock_api_client.dart';

import '../helpers.dart';

const hit = 'Welchen Hämoglobinwert hatte ich im Oktober?';

/// The excerpt of the source for [hit], as the expanded card shows it.
const hitExcerpt = '„Hämoglobin 14,2 g/dl (Ref. 13,5-17,5)“';

/// Matches a line in the lab report and in the doctor's letter, so the mock
/// answers with two sources.
const twoSources = 'Welcher Patient ist Max Mustermann?';

/// Starts the app and switches to the „Fragen“ page.
Future<void> goToQuestions(WidgetTester tester, MockApiClient mock) async {
  useLargeScreen(tester);
  await tester.pumpWidget(testApp(mock));
  await tester.pumpAndSettle();
  await openHealth(tester);
  await tester.tap(
    find.descendant(
      of: find.byType(NavigationBar),
      matching: find.text('Fragen'),
    ),
  );
  await tester.pumpAndSettle();
}

Future<void> askQuestion(WidgetTester tester, String question) async {
  await tester.enterText(find.byType(TextField), question);
  await tester.pump();
  await tester.tap(find.byTooltip('Frage senden'));
  await tester.pumpAndSettle();
}

IconButton sendButton(WidgetTester tester) => tester.widget<IconButton>(
  find.ancestor(of: find.byIcon(Icons.send), matching: find.byType(IconButton)),
);

void main() {
  testWidgets('sending is only possible with text', (tester) async {
    await goToQuestions(tester, fastMock());

    expect(find.text('0/500'), findsOneWidget);
    expect(sendButton(tester).onPressed, isNull);

    await tester.enterText(find.byType(TextField), '   ');
    await tester.pump();
    expect(sendButton(tester).onPressed, isNull);

    await tester.enterText(find.byType(TextField), 'Hämoglobin');
    await tester.pump();
    expect(sendButton(tester).onPressed, isNotNull);
  });

  testWidgets('more than 500 characters cannot be entered', (tester) async {
    await goToQuestions(tester, fastMock());

    await tester.enterText(find.byType(TextField), 'a' * 600);
    await tester.pump();

    expect(find.text('500/500'), findsOneWidget);
  });

  testWidgets('answer with a collapsed source, tapping opens the entry', (
    tester,
  ) async {
    await goToQuestions(tester, fastMock());
    await askQuestion(tester, hit);

    expect(find.text('„$hit“'), findsOneWidget);
    expect(find.textContaining('[1]'), findsOneWidget);
    expect(find.text('Quellen'), findsOneWidget);
    expect(find.text('Laborbefund · 14.10.2026'), findsOneWidget);
    expect(find.text('Labor Muster GmbH, Salzburg'), findsOneWidget);
    // Collapsed: the excerpt is hidden, only the arrow is there.
    expect(find.text(hitExcerpt), findsNothing);
    expect(find.byTooltip('Auszug anzeigen'), findsOneWidget);

    await tester.tap(find.text('Laborbefund · 14.10.2026'));
    await tester.pumpAndSettle();
    expect(find.text('Erkannte Angaben'), findsOneWidget);

    // „Zurück“ leads back to the answer, not into the timeline.
    await tapBack(tester);
    expect(find.text('Quellen'), findsOneWidget);
    expect(find.text('„$hit“'), findsOneWidget);
  });

  testWidgets('the arrow shows and hides the excerpt, it does not open', (
    tester,
  ) async {
    await goToQuestions(tester, fastMock());
    await askQuestion(tester, hit);

    await tester.tap(find.byTooltip('Auszug anzeigen'));
    await tester.pumpAndSettle();
    expect(find.text(hitExcerpt), findsOneWidget);
    expect(find.byTooltip('Auszug ausblenden'), findsOneWidget);
    // Still on the answer: expanding did not open the entry.
    expect(find.text('Erkannte Angaben'), findsNothing);
    expect(find.text('„$hit“'), findsOneWidget);

    await tester.tap(find.byTooltip('Auszug ausblenden'));
    await tester.pumpAndSettle();
    expect(find.text(hitExcerpt), findsNothing);
    expect(find.byTooltip('Auszug anzeigen'), findsOneWidget);
  });

  testWidgets('several sources expand independently', (tester) async {
    await goToQuestions(tester, fastMock());
    await askQuestion(tester, twoSources);

    expect(find.byTooltip('Auszug anzeigen'), findsNWidgets(2));
    expect(find.text('Laborbefund · 14.10.2026'), findsOneWidget);
    expect(find.text('Arztbrief · 02.09.2026'), findsOneWidget);
    expect(find.textContaining('„Patient: Max Mustermann'), findsNothing);

    await tester.tap(find.byTooltip('Auszug anzeigen').first);
    await tester.pumpAndSettle();
    expect(find.textContaining('„Patient: Max Mustermann'), findsOneWidget);
    expect(find.byTooltip('Auszug anzeigen'), findsOneWidget);
    expect(find.byTooltip('Auszug ausblenden'), findsOneWidget);
  });

  testWidgets('a new answer starts collapsed again', (tester) async {
    await goToQuestions(tester, fastMock());
    await askQuestion(tester, hit);
    await tester.tap(find.byTooltip('Auszug anzeigen'));
    await tester.pumpAndSettle();
    expect(find.text(hitExcerpt), findsOneWidget);

    await askQuestion(tester, 'Hämoglobin');
    expect(find.text(hitExcerpt), findsNothing);
    expect(find.byTooltip('Auszug anzeigen'), findsOneWidget);
  });

  testWidgets('without a hit: only the fixed sentence, no sources', (
    tester,
  ) async {
    await goToQuestions(tester, fastMock());
    await askQuestion(tester, 'Wie war das Wetter in Paris?');

    expect(
      find.text('Dazu finde ich nichts in deinen Dokumenten.'),
      findsOneWidget,
    );
    expect(find.text('Quellen'), findsNothing);
  });

  testWidgets('loading indicator during the search', (tester) async {
    final mock = MockApiClient(
      delay: const Duration(seconds: 1),
      uploadDelay: Duration.zero,
    );
    await goToQuestions(tester, mock);
    await tester.enterText(find.byType(TextField), hit);
    await tester.pump();
    await tester.tap(find.byTooltip('Frage senden'));
    await tester.pump(const Duration(milliseconds: 100));

    expect(find.text('Suche in deinen Dokumenten …'), findsOneWidget);
    // During the search the question cannot be sent again.
    expect(sendButton(tester).onPressed, isNull);

    await tester.pump(const Duration(seconds: 1));
    await tester.pumpAndSettle();
    expect(find.text('Quellen'), findsOneWidget);
  });

  testWidgets('error: message and retry', (tester) async {
    final mock = fastMock();
    await goToQuestions(tester, mock);
    mock.simulateError = true;
    await askQuestion(tester, hit);

    expect(find.textContaining('Fehler aufgetreten'), findsOneWidget);

    mock.simulateError = false;
    await tester.tap(find.text('Erneut versuchen'));
    await tester.pumpAndSettle();

    expect(find.textContaining('Fehler aufgetreten'), findsNothing);
    expect(find.text('Quellen'), findsOneWidget);
  });

  testWidgets('model unavailable: message of the backend', (tester) async {
    await goToQuestions(tester, fastMock());
    await askQuestion(tester, 'Test modelerror');

    expect(find.textContaining('nicht erreichbar'), findsOneWidget);
  });

  testWidgets('a new question replaces the old answer', (tester) async {
    await goToQuestions(tester, fastMock());
    await askQuestion(tester, hit);
    await askQuestion(tester, 'Wie war das Wetter in Paris?');

    expect(find.text('Quellen'), findsNothing);
    expect(find.text('„$hit“'), findsNothing);
  });
}
