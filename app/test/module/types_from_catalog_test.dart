import 'package:flutter/material.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:lifelog/api/api_exception.dart';
import 'package:lifelog/model/entry_patch.dart';

import '../helpers.dart';
import 'test_catalog.dart';

void main() {
  group('Mock prüft Typen gegen das Modul des Eintrags', () {
    test('Typ aus dem Katalog ist erlaubt', () async {
      final mock = schnellerMock(katalog: testKatalog());
      final eintrag = await mock.korrigieren(
        laborId,
        const EintragAenderung(typ: 'impfpass'),
      );
      expect(eintrag.typ, 'impfpass');
    });

    test('Typ eines anderen Moduls: 400 INVALID_REQUEST', () async {
      final mock = schnellerMock(katalog: testKatalog());
      await expectLater(
        mock.korrigiereMitJson(laborId, {'type': 'training'}),
        wirftApiFehler(FehlerCode.ungueltigeAnfrage, status: 400),
      );
    });

    test('mit Standard-Katalog ist „impfpass“ unbekannt', () async {
      await expectLater(
        schnellerMock().korrigiereMitJson(laborId, {'type': 'impfpass'}),
        wirftApiFehler(FehlerCode.ungueltigeAnfrage, status: 400),
      );
    });
  });

  testWidgets('Typ-Dialog zeigt die Typen aus dem Katalog', (tester) async {
    final katalog = testKatalog();
    grosserBildschirm(tester);
    await tester.pumpWidget(
      testApp(schnellerMock(katalog: katalog), katalog: katalog),
    );
    await tester.pumpAndSettle();
    await zuGesundheit(tester);
    await tester.tap(find.text('Labor Muster GmbH, Salzburg'));
    await tester.pumpAndSettle();

    await tester.tap(find.byTooltip('Typ korrigieren'));
    await tester.pumpAndSettle();
    await tester.tap(find.byType(DropdownButtonFormField<String>));
    await tester.pumpAndSettle();

    // Zusätzlicher Typ aus dem Katalog; „Training“ gehört zu Sport.
    expect(find.text('Impfpass').last, findsOneWidget);
    expect(find.text('Training'), findsNothing);

    await tester.tap(find.text('Impfpass').last);
    await tester.pumpAndSettle();
    await tester.tap(find.widgetWithText(FilledButton, 'Speichern'));
    await tester.pumpAndSettle();

    // Titel und Feld zeigen den neuen Typ, der Hinweis den erkannten.
    expect(find.text('Impfpass'), findsWidgets);
    expect(find.text('korrigiert, erkannt: Laborbefund'), findsOneWidget);
  });
}
