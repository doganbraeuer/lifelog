import 'package:flutter/material.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:lifelog/api/mock_api_client.dart';
import 'package:lifelog/model/plain_date.dart';
import 'package:lifelog/model/entry_patch.dart';

import '../helpers.dart';

const labIssuer = 'Labor Muster GmbH, Salzburg';
const doctorIssuer = 'Dr. Erika Beispiel, Allgemeinmedizin';

/// Starts the app and opens the entry with this issuer.
Future<void> openEntryNamed(
  WidgetTester tester,
  MockApiClient mock,
  String issuer,
) async {
  useLargeScreen(tester);
  await tester.pumpWidget(testApp(mock));
  await tester.pumpAndSettle();
  await openHealth(tester);
  await tester.tap(find.textContaining(issuer));
  await tester.pumpAndSettle();
}

Future<void> tapAndSettle(WidgetTester tester, Finder finder) async {
  await tester.tap(finder);
  await tester.pumpAndSettle();
}

Finder saveButton() => find.widgetWithText(FilledButton, 'Speichern');

Finder textField(String label) => find.widgetWithText(TextField, label);

void main() {
  testWidgets('shows details, correction hint and PDF file', (tester) async {
    await openEntryNamed(tester, fastMock(), labIssuer);

    expect(find.text('Laborbefund'), findsWidgets);
    expect(find.text('14.10.2026'), findsOneWidget);
    expect(find.text(labIssuer), findsOneWidget);
    expect(find.text('6,8 G/l'), findsOneWidget);
    expect(find.text('Wert korrigiert, erkannt: 6,3'), findsOneWidget);
    expect(find.text('befund_oktober.pdf'), findsOneWidget);
    expect(find.text('180 KB · PDF'), findsOneWidget);
    expect(find.textContaining('nicht korrigiert werden'), findsOneWidget);
    // The import date has no pencil button.
    expect(find.byTooltip('Importiert korrigieren'), findsNothing);
  });

  testWidgets('image as original file is shown', (tester) async {
    await openEntryNamed(tester, fastMock(), doctorIssuer);

    expect(find.byType(Image), findsOneWidget);
    expect(find.textContaining('· Bild'), findsOneWidget);
  });

  group('facts', () {
    testWidgets('correct a value', (tester) async {
      final mock = fastMock();
      await openEntryNamed(tester, mock, labIssuer);

      await tapAndSettle(tester, find.byTooltip('Hämoglobin korrigieren'));
      await tester.enterText(textField('Wert'), '15,0');
      await tapAndSettle(tester, saveButton());

      expect(find.text('15,0 g/dl'), findsOneWidget);
      expect(find.text('Wert korrigiert, erkannt: 14,2'), findsOneWidget);
      expect(find.text('Korrektur gespeichert.'), findsOneWidget);
      expect((await mock.getEntry(labReportId)).facts.first.value, '15,0');
    });

    testWidgets('resetting to the recognized value removes the hint', (
      tester,
    ) async {
      await openEntryNamed(tester, fastMock(), labIssuer);

      await tapAndSettle(tester, find.byTooltip('Leukozyten korrigieren'));
      await tapAndSettle(tester, find.text('Erkannten Wert übernehmen: 6,3'));
      await tapAndSettle(tester, saveButton());

      expect(find.text('6,3 G/l'), findsOneWidget);
      expect(find.textContaining('korrigiert, erkannt'), findsNothing);
    });

    testWidgets('remove the unit', (tester) async {
      await openEntryNamed(tester, fastMock(), labIssuer);

      await tapAndSettle(tester, find.byTooltip('CRP korrigieren'));
      await tapAndSettle(tester, find.byTooltip('Einheit leeren'));
      await tapAndSettle(tester, saveButton());

      expect(find.text('<0,5'), findsOneWidget);
      expect(find.text('Einheit korrigiert, erkannt: mg/dl'), findsOneWidget);
    });

    testWidgets('an empty value is not saved', (tester) async {
      await openEntryNamed(tester, fastMock(), labIssuer);

      await tapAndSettle(tester, find.byTooltip('CRP korrigieren'));
      await tapAndSettle(tester, find.byTooltip('Wert leeren'));
      await tapAndSettle(tester, saveButton());

      expect(find.text('Der Wert darf nicht leer sein.'), findsOneWidget);
      expect(saveButton(), findsOneWidget); // dialog stays open
    });

    testWidgets('no change, no save', (tester) async {
      await openEntryNamed(tester, fastMock(), labIssuer);

      await tapAndSettle(tester, find.byTooltip('CRP korrigieren'));
      await tapAndSettle(tester, saveButton());

      expect(find.text('Korrektur gespeichert.'), findsNothing);
    });
  });

  group('document date', () {
    testWidgets('remove and restore', (tester) async {
      await openEntryNamed(tester, fastMock(), labIssuer);

      await tapAndSettle(tester, find.byTooltip('Dokumentdatum korrigieren'));
      await tapAndSettle(tester, find.text('Datum entfernen'));

      expect(find.text('nicht erkannt'), findsOneWidget);
      expect(find.text('korrigiert, erkannt: 14.10.2026'), findsOneWidget);

      await tapAndSettle(tester, find.byTooltip('Dokumentdatum korrigieren'));
      await tapAndSettle(tester, find.textContaining('Erkanntes Datum'));

      expect(find.text('14.10.2026'), findsOneWidget);
      expect(find.textContaining('erkannt: 14.10.2026'), findsNothing);
    });

    testWidgets('new date via the date picker', (tester) async {
      final mock = fastMock();
      await openEntryNamed(tester, mock, labIssuer);

      await tapAndSettle(tester, find.byTooltip('Dokumentdatum korrigieren'));
      await tapAndSettle(tester, find.text('Datum wählen'));
      // The date picker is German and starts at the current date.
      expect(find.text('Abbrechen'), findsOneWidget);
      await tapAndSettle(tester, find.text('15'));
      await tapAndSettle(tester, find.text('OK'));

      expect(find.text('15.10.2026'), findsOneWidget);
      expect(find.text('korrigiert, erkannt: 14.10.2026'), findsOneWidget);
      expect(
        (await mock.getEntry(labReportId)).documentDate,
        PlainDate(2026, 10, 15),
      );
    });

    testWidgets('wrongly recognized year 1026: picker starts today', (
      tester,
    ) async {
      // Starting point: the date lies outside 1900–2100.
      final mock = fastMock();
      await mock.correct(
        labReportId,
        EntryPatch(documentDate: Value(PlainDate(1026, 10, 14))),
      );
      await openEntryNamed(tester, mock, labIssuer);
      expect(find.text('14.10.1026'), findsOneWidget);

      await tapAndSettle(tester, find.byTooltip('Dokumentdatum korrigieren'));
      await tapAndSettle(tester, find.text('Datum wählen'));

      // No crash: the picker is open, in the current month.
      expect(tester.takeException(), isNull);
      expect(find.text('Abbrechen'), findsOneWidget);
      await tapAndSettle(tester, find.text('15'));
      await tapAndSettle(tester, find.text('OK'));

      final today = DateTime.now();
      final newDate = PlainDate(today.year, today.month, 15);
      expect(find.text(newDate.display), findsOneWidget);
      expect((await mock.getEntry(labReportId)).documentDate, newDate);
    });

    testWidgets('timeline shows the correction after going back', (
      tester,
    ) async {
      await openEntryNamed(tester, fastMock(), labIssuer);

      await tapAndSettle(tester, find.byTooltip('Dokumentdatum korrigieren'));
      await tapAndSettle(tester, find.text('Datum entfernen'));
      await tapBack(tester);

      expect(find.text('ohne Datum'), findsNWidgets(2));
    });
  });

  testWidgets('type via the dropdown', (tester) async {
    await openEntryNamed(tester, fastMock(), labIssuer);

    await tapAndSettle(tester, find.byTooltip('Typ korrigieren'));
    await tapAndSettle(tester, find.byType(DropdownButtonFormField<String>));
    await tapAndSettle(tester, find.text('Arztbrief').last);
    await tapAndSettle(tester, saveButton());

    expect(find.text('Arztbrief'), findsWidgets);
    expect(find.text('korrigiert, erkannt: Laborbefund'), findsOneWidget);
  });

  group('issuer', () {
    testWidgets('change', (tester) async {
      await openEntryNamed(tester, fastMock(), labIssuer);

      await tapAndSettle(tester, find.byTooltip('Quelle korrigieren'));
      await tester.enterText(textField('Quelle'), 'Labor Beispiel');
      await tapAndSettle(tester, saveButton());

      expect(find.text('Labor Beispiel'), findsOneWidget);
      expect(find.text('korrigiert, erkannt: $labIssuer'), findsOneWidget);
    });

    testWidgets('cancel changes nothing', (tester) async {
      await openEntryNamed(tester, fastMock(), labIssuer);

      await tapAndSettle(tester, find.byTooltip('Quelle korrigieren'));
      await tester.enterText(textField('Quelle'), 'Verworfen');
      await tapAndSettle(tester, find.text('Abbrechen'));

      expect(find.text(labIssuer), findsOneWidget);
      expect(find.text('Verworfen'), findsNothing);
    });

    testWidgets('an empty field removes the issuer', (tester) async {
      await openEntryNamed(tester, fastMock(), labIssuer);

      await tapAndSettle(tester, find.byTooltip('Quelle korrigieren'));
      await tapAndSettle(tester, find.byTooltip('Quelle leeren'));
      await tapAndSettle(tester, saveButton());

      expect(find.text('nicht erkannt'), findsOneWidget);
      expect(find.text('korrigiert, erkannt: $labIssuer'), findsOneWidget);
    });
  });

  testWidgets('error while saving: message, old state stays', (tester) async {
    final mock = fastMock();
    await openEntryNamed(tester, mock, labIssuer);

    await tapAndSettle(tester, find.byTooltip('Hämoglobin korrigieren'));
    await tester.enterText(textField('Wert'), '99');
    mock.simulateError = true;
    await tapAndSettle(tester, saveButton());

    expect(find.textContaining('Fehler aufgetreten'), findsOneWidget);
    expect(find.text('14,2 g/dl'), findsOneWidget);
  });
}
