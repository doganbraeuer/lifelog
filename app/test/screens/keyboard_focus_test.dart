// The keyboard of a text field on a hidden page must not come back over
// another page (found on the phone: it covered the navigation bar).
import 'package:flutter/material.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:lifelog/screens/entry_screen.dart';
import 'package:lifelog/screens/timeline_screen.dart';

import '../helpers.dart';

Finder navItem(String label) =>
    find.descendant(of: find.byType(NavigationBar), matching: find.text(label));

Future<void> goTo(WidgetTester tester, String label) async {
  await tester.tap(navItem(label));
  await tester.pumpAndSettle();
}

Future<void> askQuestion(WidgetTester tester, String question) async {
  await tester.tap(find.widgetWithText(TextField, 'Deine Frage'));
  await tester.enterText(
    find.widgetWithText(TextField, 'Deine Frage'),
    question,
  );
  await tester.tap(find.byTooltip('Frage senden'));
  await tester.pumpAndSettle();
}

Future<void> importText(WidgetTester tester, String text) async {
  await tester.tap(find.text('Text eingeben'));
  await tester.pumpAndSettle();
  final field = find.widgetWithText(TextField, 'Dokumentinhalt');
  await tester.tap(field);
  await tester.enterText(field, text);
  await tester.pump();
  await tester.tap(find.widgetWithText(FilledButton, 'Importieren'));
  await tester.pumpAndSettle();
}

/// `true` if no text field has focus, i.e. no keyboard is requested.
bool noTextFieldFocused() {
  final context = FocusManager.instance.primaryFocus?.context;
  return context?.findAncestorWidgetOfExactType<EditableText>() == null;
}

void main() {
  testWidgets('after a text import and back from the entry: no keyboard', (
    tester,
  ) async {
    useLargeScreen(tester);
    await tester.pumpWidget(testApp(fastMock()));
    await tester.pumpAndSettle();
    await openHealth(tester);

    await goTo(tester, 'Fragen');
    await askQuestion(tester, 'Hämoglobin');
    await goTo(tester, 'Hochladen');
    await importText(tester, 'Notiz 12.11.2026: Zahnarzt-Kontrolle.');
    expect(find.byType(EntryScreen), findsOneWidget);

    await tapBack(tester);
    expect(find.byType(TimelineScreen), findsOneWidget);
    expect(noTextFieldFocused(), isTrue);
    expect(tester.testTextInput.isVisible, isFalse);
  });

  testWidgets('duplicate: jumping to the existing entry and back leaves no '
      'keyboard', (tester) async {
    useLargeScreen(tester);
    final mock = fastMock();
    await mock.importText('Notiz: doppelt', module: 'health');
    await tester.pumpWidget(testApp(mock));
    await tester.pumpAndSettle();
    await openHealth(tester);

    await goTo(tester, 'Hochladen');
    await importText(tester, 'Notiz: doppelt');
    expect(find.text('Diesen Text hast du schon importiert.'), findsOneWidget);
    await tester.tap(find.text('Vorhandenen Eintrag öffnen'));
    await tester.pumpAndSettle();
    expect(find.byType(EntryScreen), findsOneWidget);

    await tapBack(tester);
    expect(noTextFieldFocused(), isTrue);
    expect(tester.testTextInput.isVisible, isFalse);
  });

  testWidgets('switching pages drops the focus of the hidden page', (
    tester,
  ) async {
    useLargeScreen(tester);
    await tester.pumpWidget(testApp(fastMock()));
    await tester.pumpAndSettle();
    await openHealth(tester);

    await goTo(tester, 'Fragen');
    await tester.tap(find.widgetWithText(TextField, 'Deine Frage'));
    await tester.pump();
    expect(noTextFieldFocused(), isFalse);

    await goTo(tester, 'Zeitleiste');
    expect(noTextFieldFocused(), isTrue);
    expect(tester.testTextInput.isVisible, isFalse);
  });
}
