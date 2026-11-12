import 'package:flutter/material.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:lifelog/api/mock_api_client.dart';
import 'package:lifelog/model/plain_date.dart';
import 'package:lifelog/model/entry_patch.dart';

import '../helpers.dart';

const labor = 'Labor Muster GmbH, Salzburg';
const arzt = 'Dr. Erika Beispiel, Allgemeinmedizin';

/// Startet die App und öffnet den Eintrag mit dieser Quelle.
Future<void> zumEintrag(
  WidgetTester tester,
  MockApiClient mock,
  String issuer,
) async {
  grosserBildschirm(tester);
  await tester.pumpWidget(testApp(mock));
  await tester.pumpAndSettle();
  await zuGesundheit(tester);
  await tester.tap(find.textContaining(issuer));
  await tester.pumpAndSettle();
}

Future<void> tippen(WidgetTester tester, Finder finder) async {
  await tester.tap(finder);
  await tester.pumpAndSettle();
}

Finder speichern() => find.widgetWithText(FilledButton, 'Speichern');

Finder textFeld(String label) => find.widgetWithText(TextField, label);

void main() {
  testWidgets('zeigt Angaben, Korrektur-Hinweis und PDF-Datei', (tester) async {
    await zumEintrag(tester, schnellerMock(), labor);

    expect(find.text('Laborbefund'), findsWidgets);
    expect(find.text('14.10.2026'), findsOneWidget);
    expect(find.text(labor), findsOneWidget);
    expect(find.text('6,8 G/l'), findsOneWidget);
    expect(find.text('Wert korrigiert, erkannt: 6,3'), findsOneWidget);
    expect(find.text('befund_oktober.pdf'), findsOneWidget);
    expect(find.text('180 KB · PDF'), findsOneWidget);
    expect(find.textContaining('nicht korrigiert werden'), findsOneWidget);
    // Das Importdatum hat keinen Stift.
    expect(find.byTooltip('Importiert korrigieren'), findsNothing);
  });

  testWidgets('Bild als Originaldatei wird angezeigt', (tester) async {
    await zumEintrag(tester, schnellerMock(), arzt);

    expect(find.byType(Image), findsOneWidget);
    expect(find.textContaining('· Bild'), findsOneWidget);
  });

  group('Infos', () {
    testWidgets('Wert korrigieren', (tester) async {
      final mock = schnellerMock();
      await zumEintrag(tester, mock, labor);

      await tippen(tester, find.byTooltip('Hämoglobin korrigieren'));
      await tester.enterText(textFeld('Wert'), '15,0');
      await tippen(tester, speichern());

      expect(find.text('15,0 g/dl'), findsOneWidget);
      expect(find.text('Wert korrigiert, erkannt: 14,2'), findsOneWidget);
      expect(find.text('Korrektur gespeichert.'), findsOneWidget);
      expect((await mock.getEntry(laborId)).facts.first.value, '15,0');
    });

    testWidgets('auf erkannten Wert zurücksetzen entfernt den Hinweis', (
      tester,
    ) async {
      await zumEintrag(tester, schnellerMock(), labor);

      await tippen(tester, find.byTooltip('Leukozyten korrigieren'));
      await tippen(tester, find.text('Erkannten Wert übernehmen: 6,3'));
      await tippen(tester, speichern());

      expect(find.text('6,3 G/l'), findsOneWidget);
      expect(find.textContaining('korrigiert, erkannt'), findsNothing);
    });

    testWidgets('Einheit entfernen', (tester) async {
      await zumEintrag(tester, schnellerMock(), labor);

      await tippen(tester, find.byTooltip('CRP korrigieren'));
      await tippen(tester, find.byTooltip('Einheit leeren'));
      await tippen(tester, speichern());

      expect(find.text('<0,5'), findsOneWidget);
      expect(find.text('Einheit korrigiert, erkannt: mg/dl'), findsOneWidget);
    });

    testWidgets('leerer Wert wird nicht gespeichert', (tester) async {
      await zumEintrag(tester, schnellerMock(), labor);

      await tippen(tester, find.byTooltip('CRP korrigieren'));
      await tippen(tester, find.byTooltip('Wert leeren'));
      await tippen(tester, speichern());

      expect(find.text('Der Wert darf nicht leer sein.'), findsOneWidget);
      expect(speichern(), findsOneWidget); // Dialog bleibt offen
    });

    testWidgets('ohne Änderung kein Speichern', (tester) async {
      await zumEintrag(tester, schnellerMock(), labor);

      await tippen(tester, find.byTooltip('CRP korrigieren'));
      await tippen(tester, speichern());

      expect(find.text('Korrektur gespeichert.'), findsNothing);
    });
  });

  group('Dokumentdatum', () {
    testWidgets('entfernen und wiederherstellen', (tester) async {
      await zumEintrag(tester, schnellerMock(), labor);

      await tippen(tester, find.byTooltip('Dokumentdatum korrigieren'));
      await tippen(tester, find.text('Datum entfernen'));

      expect(find.text('nicht erkannt'), findsOneWidget);
      expect(find.text('korrigiert, erkannt: 14.10.2026'), findsOneWidget);

      await tippen(tester, find.byTooltip('Dokumentdatum korrigieren'));
      await tippen(tester, find.textContaining('Erkanntes Datum'));

      expect(find.text('14.10.2026'), findsOneWidget);
      expect(find.textContaining('erkannt: 14.10.2026'), findsNothing);
    });

    testWidgets('neues Datum über die Datumsauswahl', (tester) async {
      final mock = schnellerMock();
      await zumEintrag(tester, mock, labor);

      await tippen(tester, find.byTooltip('Dokumentdatum korrigieren'));
      await tippen(tester, find.text('Datum wählen'));
      // Die Datumsauswahl ist deutsch und startet beim aktuellen Datum.
      expect(find.text('Abbrechen'), findsOneWidget);
      await tippen(tester, find.text('15'));
      await tippen(tester, find.text('OK'));

      expect(find.text('15.10.2026'), findsOneWidget);
      expect(find.text('korrigiert, erkannt: 14.10.2026'), findsOneWidget);
      expect(
        (await mock.getEntry(laborId)).documentDate,
        PlainDate(2026, 10, 15),
      );
    });

    testWidgets('falsch erkanntes Jahr 1026: Auswahl startet heute', (
      tester,
    ) async {
      // Ausgangslage: Das Datum liegt außerhalb von 1900–2100.
      final mock = schnellerMock();
      await mock.correct(
        laborId,
        EntryPatch(documentDate: Value(PlainDate(1026, 10, 14))),
      );
      await zumEintrag(tester, mock, labor);
      expect(find.text('14.10.1026'), findsOneWidget);

      await tippen(tester, find.byTooltip('Dokumentdatum korrigieren'));
      await tippen(tester, find.text('Datum wählen'));

      // Kein Absturz: Die Auswahl ist offen, im aktuellen Monat.
      expect(tester.takeException(), isNull);
      expect(find.text('Abbrechen'), findsOneWidget);
      await tippen(tester, find.text('15'));
      await tippen(tester, find.text('OK'));

      final heute = DateTime.now();
      final neu = PlainDate(heute.year, heute.month, 15);
      expect(find.text(neu.display), findsOneWidget);
      expect((await mock.getEntry(laborId)).documentDate, neu);
    });

    testWidgets('Zeitleiste zeigt die Korrektur nach „Zurück“', (tester) async {
      await zumEintrag(tester, schnellerMock(), labor);

      await tippen(tester, find.byTooltip('Dokumentdatum korrigieren'));
      await tippen(tester, find.text('Datum entfernen'));
      await zurueck(tester);

      expect(find.text('ohne Datum'), findsNWidgets(2));
    });
  });

  testWidgets('Typ über das Dropdown', (tester) async {
    await zumEintrag(tester, schnellerMock(), labor);

    await tippen(tester, find.byTooltip('Typ korrigieren'));
    await tippen(tester, find.byType(DropdownButtonFormField<String>));
    await tippen(tester, find.text('Arztbrief').last);
    await tippen(tester, speichern());

    expect(find.text('Arztbrief'), findsWidgets);
    expect(find.text('korrigiert, erkannt: Laborbefund'), findsOneWidget);
  });

  group('Quelle', () {
    testWidgets('ändern', (tester) async {
      await zumEintrag(tester, schnellerMock(), labor);

      await tippen(tester, find.byTooltip('Quelle korrigieren'));
      await tester.enterText(textFeld('Quelle'), 'Labor Beispiel');
      await tippen(tester, speichern());

      expect(find.text('Labor Beispiel'), findsOneWidget);
      expect(find.text('korrigiert, erkannt: $labor'), findsOneWidget);
    });

    testWidgets('Abbrechen ändert nichts', (tester) async {
      await zumEintrag(tester, schnellerMock(), labor);

      await tippen(tester, find.byTooltip('Quelle korrigieren'));
      await tester.enterText(textFeld('Quelle'), 'Verworfen');
      await tippen(tester, find.text('Abbrechen'));

      expect(find.text(labor), findsOneWidget);
      expect(find.text('Verworfen'), findsNothing);
    });

    testWidgets('leeres Feld entfernt die Quelle', (tester) async {
      await zumEintrag(tester, schnellerMock(), labor);

      await tippen(tester, find.byTooltip('Quelle korrigieren'));
      await tippen(tester, find.byTooltip('Quelle leeren'));
      await tippen(tester, speichern());

      expect(find.text('nicht erkannt'), findsOneWidget);
      expect(find.text('korrigiert, erkannt: $labor'), findsOneWidget);
    });
  });

  testWidgets('Fehler beim Speichern: Meldung, alter Stand bleibt', (
    tester,
  ) async {
    final mock = schnellerMock();
    await zumEintrag(tester, mock, labor);

    await tippen(tester, find.byTooltip('Hämoglobin korrigieren'));
    await tester.enterText(textFeld('Wert'), '99');
    mock.simulateError = true;
    await tippen(tester, speichern());

    expect(find.textContaining('Fehler aufgetreten'), findsOneWidget);
    expect(find.text('14,2 g/dl'), findsOneWidget);
  });
}
