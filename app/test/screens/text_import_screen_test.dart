import 'package:flutter/material.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:lifelog/api/mock_api_client.dart';
import 'package:lifelog/screens/entry_screen.dart';
import 'package:lifelog/screens/original_file_screen.dart';
import 'package:lifelog/screens/timeline_screen.dart';

import '../helpers.dart';

const text =
    'Hausarztpraxis Beispiel\n'
    'Überweisung vom 21.10.2026\n'
    'zur Physiotherapie';

Finder textField() => find.widgetWithText(TextField, 'Dokumentinhalt');
Finder importButton() => find.widgetWithText(FilledButton, 'Importieren');

/// Starts the app, opens the upload page and the text field.
Future<void> openTextField(WidgetTester tester, MockApiClient mock) async {
  useLargeScreen(tester);
  await tester.pumpWidget(testApp(mock));
  await tester.pumpAndSettle();
  await openHealth(tester);
  await tester.tap(
    find.descendant(
      of: find.byType(NavigationBar),
      matching: find.text('Hochladen'),
    ),
  );
  await tester.pumpAndSettle();
  await tester.tap(find.text('Text eingeben'));
  await tester.pumpAndSettle();
}

FilledButton button(WidgetTester tester, Finder finder) =>
    tester.widget<FilledButton>(finder);

void main() {
  testWidgets('import is only possible with text', (tester) async {
    await openTextField(tester, fastMock());

    expect(textField(), findsOneWidget);
    expect(find.text('0/100000'), findsOneWidget);
    expect(button(tester, importButton()).onPressed, isNull);

    await tester.enterText(textField(), '   \n ');
    await tester.pump();
    expect(button(tester, importButton()).onPressed, isNull);

    await tester.enterText(textField(), text);
    await tester.pump();
    expect(button(tester, importButton()).onPressed, isNotNull);
  });

  testWidgets('importing text: the entry opens, the original file is the '
      'text', (tester) async {
    final mock = fastMock();
    await openTextField(tester, mock);
    await tester.enterText(textField(), text);
    await tester.pump();
    await tester.tap(importButton());
    await tester.pumpAndSettle();

    expect(find.byType(EntryScreen), findsOneWidget);
    expect(find.text('Text gespeichert.'), findsOneWidget);
    expect(find.text('Hausarztpraxis Beispiel'), findsOneWidget); // issuer
    expect(find.text('21.10.2026'), findsOneWidget);
    expect(find.textContaining('· Text'), findsOneWidget);
    expect(await mock.listEntries(module: 'health'), hasLength(4));

    await tester.tap(find.text('Originaldatei öffnen'));
    await tester.pumpAndSettle();
    expect(find.byType(OriginalFileScreen), findsOneWidget);
    expect(find.textContaining('zur Physiotherapie'), findsWidgets);

    // Back to the timeline; the new entry is there.
    await tapBack(tester);
    await tapBack(tester);
    expect(find.byType(TimelineScreen), findsOneWidget);
    expect(find.text('Hausarztpraxis Beispiel'), findsOneWidget);
  });

  testWidgets('loading indicator, text field locked', (tester) async {
    final mock = MockApiClient(
      delay: Duration.zero,
      uploadDelay: const Duration(seconds: 2),
    );
    await openTextField(tester, mock);
    await tester.enterText(textField(), text);
    await tester.pump();
    await tester.tap(importButton());
    await tester.pump(const Duration(milliseconds: 100));

    expect(find.textContaining('bis zu einer Minute'), findsOneWidget);
    expect(tester.widget<TextField>(textField()).enabled, isFalse);

    await tester.pump(const Duration(seconds: 2));
    await tester.pumpAndSettle();
    expect(find.byType(EntryScreen), findsOneWidget);
  });

  testWidgets('cancel closes the field and discards the text', (tester) async {
    await openTextField(tester, fastMock());
    await tester.enterText(textField(), text);
    await tester.tap(find.text('Abbrechen'));
    await tester.pumpAndSettle();

    expect(textField(), findsNothing);
    expect(find.text('Foto aufnehmen'), findsOneWidget);

    await tester.tap(find.text('Text eingeben'));
    await tester.pumpAndSettle();
    expect(tester.widget<TextField>(textField()).controller!.text, isEmpty);
  });

  testWidgets('duplicate: message, jump possible, text stays', (tester) async {
    final mock = fastMock();
    final preloaded = await mock.importText(text, module: 'health');
    await openTextField(tester, mock);
    await tester.enterText(textField(), text);
    await tester.pump();
    await tester.tap(importButton());
    await tester.pumpAndSettle();

    expect(find.text('Diesen Text hast du schon importiert.'), findsOneWidget);
    expect(tester.widget<TextField>(textField()).controller!.text, text);
    // No point in retrying: the button is still called „Importieren“.
    expect(importButton(), findsOneWidget);

    await tester.tap(find.text('Vorhandenen Eintrag öffnen'));
    await tester.pumpAndSettle();
    expect(find.byType(EntryScreen), findsOneWidget);
    expect(find.textContaining(preloaded.file.fileName), findsWidgets);
  });

  testWidgets('phone size, keyboard open, long text: buttons reachable', (
    tester,
  ) async {
    final mock = fastMock();
    await tester.pumpWidget(testApp(mock));
    await tester.pumpAndSettle();
    await openHealth(tester);

    // Like the OnePlus 8T: 1080 × 2400 px at factor 3, i.e. 360 × 800.
    // Gboard is 897 px high there (measured by screenshot), i.e. 299.
    // Only switch after „Bereiche“: the tiles there overflow in the test
    // font at 360 px width (a separate topic).
    tester.view.physicalSize = const Size(1080, 2400);
    tester.view.devicePixelRatio = 3;
    addTearDown(tester.view.reset);
    await tester.pumpAndSettle();

    await tester.tap(
      find.descendant(
        of: find.byType(NavigationBar),
        matching: find.text('Hochladen'),
      ),
    );
    await tester.pumpAndSettle();
    await tester.tap(find.text('Text eingeben'));
    await tester.pumpAndSettle();

    // Tap the field, the keyboard opens, a long text is entered.
    await tester.tap(textField());
    tester.view.viewInsets = const FakeViewPadding(bottom: 897);
    await tester.enterText(
      textField(),
      [for (var i = 1; i <= 20; i++) 'Zeile $i Laborwert Beispiel'].join('\n'),
    );
    await tester.pumpAndSettle();

    // Only the area between the app bar and the keyboard is visible.
    final top = tester.getRect(find.widgetWithText(AppBar, 'Hochladen')).bottom;
    const bottom = (2400 - 897) / 3;
    for (final button in [importButton(), find.text('Abbrechen')]) {
      final rect = tester.getRect(button);
      expect(rect.top, greaterThanOrEqualTo(top), reason: '$button top');
      expect(rect.bottom, lessThanOrEqualTo(bottom), reason: '$button bottom');
    }

    // And the button does what it should.
    await tester.tap(importButton());
    await tester.pumpAndSettle();
    expect(find.byType(EntryScreen), findsOneWidget);
  });

  testWidgets('retryable error: „Erneut versuchen“, text stays', (
    tester,
  ) async {
    await openTextField(tester, fastMock());
    await tester.enterText(textField(), 'Test modelerror');
    await tester.pump();
    await tester.tap(importButton());
    await tester.pumpAndSettle();

    expect(find.textContaining('nicht erreichbar'), findsOneWidget);
    expect(
      find.widgetWithText(FilledButton, 'Erneut versuchen'),
      findsOneWidget,
    );
    expect(
      tester.widget<TextField>(textField()).controller!.text,
      'Test modelerror',
    );
  });
}
