import 'package:flutter/material.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:lifelog/api/mock_api_client.dart';

import '../helpers.dart';

const hit = 'Welchen Hämoglobinwert hatte ich im Oktober?';

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

  testWidgets('answer with source, tapping opens the entry', (tester) async {
    await goToQuestions(tester, fastMock());
    await askQuestion(tester, hit);

    expect(find.text('„$hit“'), findsOneWidget);
    expect(find.textContaining('[1]'), findsOneWidget);
    expect(find.text('Quellen'), findsOneWidget);
    expect(find.text('Laborbefund · 14.10.2026'), findsOneWidget);
    expect(find.text('Labor Muster GmbH, Salzburg'), findsOneWidget);
    expect(
      find.text('„Hämoglobin 14,2 g/dl (Ref. 13,5-17,5)“'),
      findsOneWidget,
    );

    await tester.tap(find.text('Laborbefund · 14.10.2026'));
    await tester.pumpAndSettle();
    expect(find.text('Erkannte Angaben'), findsOneWidget);

    // „Zurück“ leads back to the answer, not into the timeline.
    await tapBack(tester);
    expect(find.text('Quellen'), findsOneWidget);
    expect(find.text('„$hit“'), findsOneWidget);
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
