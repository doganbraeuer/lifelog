// Daily limit (429 LIMIT_REACHED) and an overloaded model (503): the
// existing error displays show the server's message (docs/API.md section 1
// "Daily limits").
import 'package:flutter/material.dart';
import 'package:flutter_test/flutter_test.dart';

import '../helpers.dart';

const limitMessage = 'Du hast dein Tageslimit erreicht. Morgen geht es weiter.';

Future<void> goTo(
  WidgetTester tester,
  String page, {
  FakeFileChooser? chooser,
  FakeVoiceRecorder? recorder,
}) async {
  useLargeScreen(tester);
  await tester.pumpWidget(
    testApp(fastMock(), chooser: chooser, recorder: recorder),
  );
  await tester.pumpAndSettle();
  await openHealth(tester);
  await tester.tap(
    find.descendant(of: find.byType(NavigationBar), matching: find.text(page)),
  );
  await tester.pumpAndSettle();
}

void main() {
  testWidgets('upload: the limit message, no „Erneut versuchen“', (
    tester,
  ) async {
    final chooser = FakeFileChooser()..pdf = testPdf('limitreached.pdf');
    await goTo(tester, 'Hochladen', chooser: chooser);
    await tester.tap(find.text('PDF auswählen'));
    await tester.pumpAndSettle();
    await tester.tap(find.widgetWithText(FilledButton, 'Hochladen'));
    await tester.pumpAndSettle();

    expect(find.text(limitMessage), findsOneWidget);
    // Trying again the same day does not help.
    expect(find.text('Erneut versuchen'), findsNothing);
  });

  testWidgets('upload: 503 shows its message with „Erneut versuchen“', (
    tester,
  ) async {
    final chooser = FakeFileChooser()..pdf = testPdf('modelerror.pdf');
    await goTo(tester, 'Hochladen', chooser: chooser);
    await tester.tap(find.text('PDF auswählen'));
    await tester.pumpAndSettle();
    await tester.tap(find.widgetWithText(FilledButton, 'Hochladen'));
    await tester.pumpAndSettle();

    expect(
      find.text(
        'Der Auswertungsdienst ist gerade nicht erreichbar. '
        'Bitte später noch einmal versuchen.',
      ),
      findsOneWidget,
    );
    expect(find.text('Erneut versuchen'), findsOneWidget);
  });

  testWidgets('question: the limit message', (tester) async {
    await goTo(tester, 'Fragen');
    await tester.enterText(find.byType(TextField), 'Wert limitreached?');
    await tester.pump();
    await tester.tap(find.byTooltip('Frage senden'));
    await tester.pumpAndSettle();

    expect(find.text(limitMessage), findsOneWidget);
  });

  testWidgets('voice note: the limit message', (tester) async {
    final recorder = FakeVoiceRecorder()..fileName = 'limitreached.m4a';
    await goTo(tester, 'Hochladen', recorder: recorder);
    await tester.tap(find.text('Text eingeben'));
    await tester.pumpAndSettle();

    await tester.tap(find.widgetWithIcon(IconButton, Icons.mic_none));
    await tester.pump();
    await tester.pump(const Duration(milliseconds: 500));
    await tester.tap(find.widgetWithText(FilledButton, 'Stopp'));
    await tester.pumpAndSettle();

    expect(find.text(limitMessage), findsOneWidget);
    expect(recorder.hasFile, isFalse);
  });
}
