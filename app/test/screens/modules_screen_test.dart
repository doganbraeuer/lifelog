import 'package:flutter/material.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:lifelog/module/module_definition.dart';
import 'package:lifelog/module/module_catalog.dart';
import 'package:lifelog/screens/entry_screen.dart';
import 'package:lifelog/screens/original_file_screen.dart';
import 'package:lifelog/screens/timeline_screen.dart';

import '../helpers.dart';

/// Primärfarbe, die aus einer Grundfarbe entsteht (so wie im Modul-Theme).
Color primaerAus(Color farbe) => ColorScheme.fromSeed(seedColor: farbe).primary;

/// Primärfarbe, die an der Stelle des gefundenen Widgets gilt.
Color primaerBei(WidgetTester tester, Finder finder) =>
    Theme.of(tester.element(finder)).colorScheme.primary;

/// Kopie eines Moduls mit anderer Farbe bzw. anderem Aktiv-Status.
ModulDefinition abgewandelt(ModulDefinition m, {Color? farbe, bool? aktiv}) =>
    ModulDefinition(
      id: m.id,
      name: m.name,
      beschreibung: m.beschreibung,
      symbol: m.symbol,
      farbe: farbe ?? m.farbe,
      typen: m.typen,
      aktiv: aktiv ?? m.aktiv,
    );

Future<void> starten(WidgetTester tester, {ModulKatalog? katalog}) async {
  grosserBildschirm(tester);
  await tester.pumpWidget(testApp(schnellerMock(), katalog: katalog));
  await tester.pumpAndSettle();
}

void main() {
  group('Kacheln', () {
    testWidgets('eine Kachel pro Modul, drei „Bald verfügbar“', (tester) async {
      await starten(tester);

      expect(find.text('Lifelog'), findsOneWidget);
      expect(find.text('Mock'), findsOneWidget);
      for (final name in ['Gesundheit', 'Ernährung', 'Rezepte', 'Sport']) {
        expect(find.text(name), findsOneWidget, reason: name);
      }
      expect(find.text('Bald verfügbar'), findsNWidgets(3));
      // Bereiche selbst lädt nichts und zeigt keine Zeitleiste.
      expect(find.byType(ZeitleisteScreen), findsNothing);
    });

    testWidgets('inaktive Kachel reagiert nicht', (tester) async {
      await starten(tester);

      await tester.tap(find.text('Ernährung'));
      await tester.pumpAndSettle();

      expect(find.byType(ZeitleisteScreen), findsNothing);
      expect(find.text('Lifelog'), findsOneWidget);
    });

    testWidgets('Screenreader: inaktive Kachel ist deaktiviert', (
      tester,
    ) async {
      final semantik = tester.ensureSemantics();
      await starten(tester);

      expect(
        tester.getSemantics(find.bySemanticsLabel('Sport, bald verfügbar')),
        isSemantics(isButton: true, isEnabled: false),
      );
      expect(
        tester.getSemantics(find.bySemanticsLabel('Gesundheit')),
        isSemantics(isButton: true, isEnabled: true),
      );
      semantik.dispose();
    });
  });

  group('Navigation', () {
    testWidgets('Gesundheit öffnet den Rahmen, „Bereiche“ führt zurück', (
      tester,
    ) async {
      await starten(tester);
      await zuGesundheit(tester);

      expect(find.byType(ZeitleisteScreen), findsOneWidget);
      expect(find.text('Labor Muster GmbH, Salzburg'), findsOneWidget);

      await tester.tap(find.byTooltip('Bereiche'));
      await tester.pumpAndSettle();

      expect(find.byType(ZeitleisteScreen), findsNothing);
      expect(find.text('Bald verfügbar'), findsNWidgets(3));
    });

    testWidgets('„Bereiche“ gibt es auch in Hochladen und Fragen', (
      tester,
    ) async {
      await starten(tester);
      for (final bereich in ['Hochladen', 'Fragen']) {
        await zuGesundheit(tester);
        await tester.tap(
          find.descendant(
            of: find.byType(NavigationBar),
            matching: find.text(bereich),
          ),
        );
        await tester.pumpAndSettle();

        await tester.tap(find.byTooltip('Bereiche'));
        await tester.pumpAndSettle();
        expect(find.text('Bald verfügbar'), findsNWidgets(3), reason: bereich);
      }
    });

    testWidgets('Zurück-Taste des Systems führt zu den Bereichen', (
      tester,
    ) async {
      await starten(tester);
      await zuGesundheit(tester);

      await tester.binding.handlePopRoute();
      await tester.pumpAndSettle();

      expect(find.byType(ZeitleisteScreen), findsNothing);
      expect(find.text('Bald verfügbar'), findsNWidgets(3));
    });
  });

  group('Farbschema', () {
    testWidgets('Modul in seiner Farbe, Bereiche neutral', (tester) async {
      await starten(tester);
      final bereiche = primaerBei(tester, find.text('Lifelog'));

      await zuGesundheit(tester);
      final zeitleiste = primaerBei(tester, find.byType(ZeitleisteScreen));

      expect(zeitleiste, primaerAus(Colors.teal));
      expect(bereiche, isNot(zeitleiste));
    });

    testWidgets('Farbe kommt aus dem Katalog', (tester) async {
      final lila = ModulKatalog([
        abgewandelt(standardModule.first, farbe: Colors.purple),
      ]);
      await starten(tester, katalog: lila);
      await zuGesundheit(tester);

      expect(
        primaerBei(tester, find.byType(ZeitleisteScreen)),
        primaerAus(Colors.purple),
      );
    });

    testWidgets('bleibt in Eintrag, Dialog und Originaldatei erhalten', (
      tester,
    ) async {
      final teal = primaerAus(Colors.teal);
      await starten(tester);
      await zuGesundheit(tester);

      // Eintrag-Ansicht (über Navigator.push geöffnet)
      await tester.tap(find.text('Dr. Erika Beispiel, Allgemeinmedizin'));
      await tester.pumpAndSettle();
      expect(primaerBei(tester, find.byType(EintragScreen)), teal);

      // Dialog
      await tester.tap(find.byTooltip('Quelle korrigieren'));
      await tester.pumpAndSettle();
      expect(primaerBei(tester, find.byType(AlertDialog)), teal);
      await tester.tap(find.text('Abbrechen'));
      await tester.pumpAndSettle();

      // Originaldatei (aus der Eintrag-Ansicht geöffnet)
      await tester.tap(find.text('Originaldatei öffnen'));
      await tester.pumpAndSettle();
      expect(primaerBei(tester, find.byType(OriginaldateiScreen)), teal);
    });

    testWidgets('Kacheln zeigen die Farbe ihres Moduls', (tester) async {
      await starten(tester);

      // Aktive Kacheln: Verlauf aus den Containerfarben des Moduls.
      for (final modul in standardModule.where((m) => m.aktiv)) {
        final ink = tester.widget<Ink>(
          find.ancestor(of: find.text(modul.name), matching: find.byType(Ink)),
        );
        final verlauf =
            (ink.decoration! as BoxDecoration).gradient! as LinearGradient;
        final farben = ColorScheme.fromSeed(seedColor: modul.farbe);
        expect(verlauf.colors, [
          farben.primaryContainer,
          farben.secondaryContainer,
        ], reason: modul.id);
      }
    });
  });

  testWidgets('Zeitleiste zeigt nur Einträge des eigenen Moduls', (
    tester,
  ) async {
    // Sport ist hier aktiv; alle Mock-Einträge gehören zu Gesundheit.
    final katalog = ModulKatalog([
      standardModule.first,
      abgewandelt(standardModule.last, aktiv: true),
    ]);
    await starten(tester, katalog: katalog);

    await tester.tap(find.text('Sport'));
    await tester.pumpAndSettle();
    expect(find.byType(ZeitleisteScreen), findsOneWidget);
    expect(find.textContaining('Noch keine Dokumente'), findsOneWidget);

    await tester.tap(find.byTooltip('Bereiche'));
    await tester.pumpAndSettle();
    await zuGesundheit(tester);
    expect(find.text('Labor Muster GmbH, Salzburg'), findsOneWidget);
  });
}
