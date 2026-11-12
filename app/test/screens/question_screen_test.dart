import 'package:flutter/material.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:lifelog/api/mock_api_client.dart';

import '../helpers.dart';

const treffer = 'Welchen Hämoglobinwert hatte ich im Oktober?';

/// Startet die App und wechselt zum Bereich „Fragen“.
Future<void> zuFragen(WidgetTester tester, MockApiClient mock) async {
  grosserBildschirm(tester);
  await tester.pumpWidget(testApp(mock));
  await tester.pumpAndSettle();
  await zuGesundheit(tester);
  await tester.tap(
    find.descendant(
      of: find.byType(NavigationBar),
      matching: find.text('Fragen'),
    ),
  );
  await tester.pumpAndSettle();
}

Future<void> fragen(WidgetTester tester, String frage) async {
  await tester.enterText(find.byType(TextField), frage);
  await tester.pump();
  await tester.tap(find.byTooltip('Frage senden'));
  await tester.pumpAndSettle();
}

IconButton sendenKnopf(WidgetTester tester) => tester.widget<IconButton>(
  find.ancestor(of: find.byIcon(Icons.send), matching: find.byType(IconButton)),
);

void main() {
  testWidgets('Senden ist erst mit Text möglich', (tester) async {
    await zuFragen(tester, schnellerMock());

    expect(find.text('0/500'), findsOneWidget);
    expect(sendenKnopf(tester).onPressed, isNull);

    await tester.enterText(find.byType(TextField), '   ');
    await tester.pump();
    expect(sendenKnopf(tester).onPressed, isNull);

    await tester.enterText(find.byType(TextField), 'Hämoglobin');
    await tester.pump();
    expect(sendenKnopf(tester).onPressed, isNotNull);
  });

  testWidgets('mehr als 500 Zeichen lassen sich nicht eingeben', (
    tester,
  ) async {
    await zuFragen(tester, schnellerMock());

    await tester.enterText(find.byType(TextField), 'a' * 600);
    await tester.pump();

    expect(find.text('500/500'), findsOneWidget);
  });

  testWidgets('Antwort mit Quelle, Tippen öffnet den Eintrag', (tester) async {
    await zuFragen(tester, schnellerMock());
    await fragen(tester, treffer);

    expect(find.text('„$treffer“'), findsOneWidget);
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

    // „Zurück“ führt wieder zur Antwort, nicht in die Zeitleiste.
    await zurueck(tester);
    expect(find.text('Quellen'), findsOneWidget);
    expect(find.text('„$treffer“'), findsOneWidget);
  });

  testWidgets('ohne Treffer: nur der feste Satz, keine Quellen', (
    tester,
  ) async {
    await zuFragen(tester, schnellerMock());
    await fragen(tester, 'Wie war das Wetter in Paris?');

    expect(
      find.text('Dazu finde ich nichts in deinen Dokumenten.'),
      findsOneWidget,
    );
    expect(find.text('Quellen'), findsNothing);
  });

  testWidgets('Ladeanzeige während der Suche', (tester) async {
    final mock = MockApiClient(
      verzoegerung: const Duration(seconds: 1),
      uploadVerzoegerung: Duration.zero,
    );
    await zuFragen(tester, mock);
    await tester.enterText(find.byType(TextField), treffer);
    await tester.pump();
    await tester.tap(find.byTooltip('Frage senden'));
    await tester.pump(const Duration(milliseconds: 100));

    expect(find.text('Suche in deinen Dokumenten …'), findsOneWidget);
    // Während der Suche lässt sich nicht noch einmal senden.
    expect(sendenKnopf(tester).onPressed, isNull);

    await tester.pump(const Duration(seconds: 1));
    await tester.pumpAndSettle();
    expect(find.text('Quellen'), findsOneWidget);
  });

  testWidgets('Fehler: Meldung und erneuter Versuch', (tester) async {
    final mock = schnellerMock();
    await zuFragen(tester, mock);
    mock.simulierterFehler = true;
    await fragen(tester, treffer);

    expect(find.textContaining('Fehler aufgetreten'), findsOneWidget);

    mock.simulierterFehler = false;
    await tester.tap(find.text('Erneut versuchen'));
    await tester.pumpAndSettle();

    expect(find.textContaining('Fehler aufgetreten'), findsNothing);
    expect(find.text('Quellen'), findsOneWidget);
  });

  testWidgets('Modell nicht erreichbar: Meldung des Backends', (tester) async {
    await zuFragen(tester, schnellerMock());
    await fragen(tester, 'Test modellfehler');

    expect(find.textContaining('nicht erreichbar'), findsOneWidget);
  });

  testWidgets('neue Frage ersetzt die alte Antwort', (tester) async {
    await zuFragen(tester, schnellerMock());
    await fragen(tester, treffer);
    await fragen(tester, 'Wie war das Wetter in Paris?');

    expect(find.text('Quellen'), findsNothing);
    expect(find.text('„$treffer“'), findsNothing);
  });
}
