import 'package:flutter/material.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:lifelog/api/api_exception.dart';
import 'package:lifelog/api/mock_api_client.dart';
import 'package:lifelog/screens/entry_screen.dart';
import 'package:lifelog/screens/question_screen.dart';
import 'package:lifelog/screens/timeline_screen.dart';

import '../helpers.dart';

const labIssuer = 'Labor Muster GmbH, Salzburg';

Future<void> openLabReport(WidgetTester tester, MockApiClient mock) async {
  useLargeScreen(tester);
  await tester.pumpWidget(testApp(mock));
  await tester.pumpAndSettle();
  await openHealth(tester);
  await tester.tap(find.text(labIssuer));
  await tester.pumpAndSettle();
}

Future<void> tapDelete(WidgetTester tester) async {
  await tester.tap(find.byTooltip('Eintrag löschen'));
  await tester.pumpAndSettle();
}

Finder confirmDelete() => find.widgetWithText(FilledButton, 'Löschen');

void main() {
  testWidgets('cancel deletes nothing', (tester) async {
    final mock = fastMock();
    await openLabReport(tester, mock);

    await tapDelete(tester);
    expect(find.text('Eintrag löschen?'), findsOneWidget);
    expect(find.textContaining('endgültig'), findsOneWidget);

    await tester.tap(find.text('Abbrechen'));
    await tester.pumpAndSettle();

    expect(find.byType(EntryScreen), findsOneWidget);
    expect((await mock.getEntry(labReportId)).id, labReportId);
  });

  testWidgets('confirm deletes, back in the timeline without the entry', (
    tester,
  ) async {
    final mock = fastMock();
    await openLabReport(tester, mock);

    await tapDelete(tester);
    await tester.tap(confirmDelete());
    await tester.pumpAndSettle();

    expect(find.byType(EntryScreen), findsNothing);
    expect(find.byType(TimelineScreen), findsOneWidget);
    expect(find.text('Eintrag gelöscht.'), findsOneWidget);
    expect(find.text(labIssuer), findsNothing);
    await expectLater(
      mock.getEntry(labReportId),
      throwsApiException(ErrorCode.entryNotFound),
    );
  });

  testWidgets('error: message, view stays open', (tester) async {
    final mock = fastMock();
    await openLabReport(tester, mock);

    await tapDelete(tester);
    mock.simulateError = true;
    await tester.tap(confirmDelete());
    await tester.pumpAndSettle();

    expect(find.byType(EntryScreen), findsOneWidget);
    expect(find.textContaining('Fehler aufgetreten'), findsOneWidget);
    // The button can be used again.
    final button = tester.widget<IconButton>(
      find.ancestor(
        of: find.byIcon(Icons.delete_outline),
        matching: find.byType(IconButton),
      ),
    );
    expect(button.onPressed, isNotNull);

    mock.simulateError = false;
    expect((await mock.getEntry(labReportId)).id, labReportId);
  });

  testWidgets(
    'deleted from a source: back to the question, timeline reloaded',
    (tester) async {
      final mock = fastMock();
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
      await tester.enterText(find.byType(TextField), 'Hämoglobin');
      await tester.pump();
      await tester.tap(find.byTooltip('Frage senden'));
      await tester.pumpAndSettle();
      await tester.tap(find.text('Laborbefund · 14.10.2026'));
      await tester.pumpAndSettle();

      await tapDelete(tester);
      await tester.tap(confirmDelete());
      await tester.pumpAndSettle();

      expect(find.byType(QuestionScreen), findsOneWidget);
      await tester.tap(
        find.descendant(
          of: find.byType(NavigationBar),
          matching: find.text('Zeitleiste'),
        ),
      );
      await tester.pumpAndSettle();
      expect(find.text(labIssuer), findsNothing);
    },
  );
}
