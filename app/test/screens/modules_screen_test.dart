import 'package:flutter/material.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:lifelog/module/module_definition.dart';
import 'package:lifelog/module/module_catalog.dart';
import 'package:lifelog/screens/entry_screen.dart';
import 'package:lifelog/screens/original_file_screen.dart';
import 'package:lifelog/screens/timeline_screen.dart';

import '../helpers.dart';

/// Primary color derived from a base color (as in the module theme).
Color primaryFrom(Color color) =>
    ColorScheme.fromSeed(seedColor: color).primary;

/// Primary color that applies at the place of the found widget.
Color primaryAt(WidgetTester tester, Finder finder) =>
    Theme.of(tester.element(finder)).colorScheme.primary;

/// Copy of a module with another color or another active state.
ModuleDefinition modified(ModuleDefinition m, {Color? color, bool? active}) =>
    ModuleDefinition(
      id: m.id,
      name: m.name,
      description: m.description,
      icon: m.icon,
      color: color ?? m.color,
      types: m.types,
      active: active ?? m.active,
    );

Future<void> startApp(WidgetTester tester, {ModuleCatalog? catalog}) async {
  useLargeScreen(tester);
  await tester.pumpWidget(testApp(fastMock(), catalog: catalog));
  await tester.pumpAndSettle();
}

void main() {
  group('tiles', () {
    testWidgets('one tile per module, three „Bald verfügbar“', (tester) async {
      await startApp(tester);

      expect(find.text('Lifelog'), findsOneWidget);
      expect(find.text('Mock'), findsOneWidget);
      for (final name in ['Gesundheit', 'Ernährung', 'Rezepte', 'Sport']) {
        expect(find.text(name), findsOneWidget, reason: name);
      }
      expect(find.text('Bald verfügbar'), findsNWidgets(3));
      // The modules screen itself loads nothing and shows no timeline.
      expect(find.byType(TimelineScreen), findsNothing);
    });

    testWidgets('inactive tile does not react', (tester) async {
      await startApp(tester);

      await tester.tap(find.text('Ernährung'));
      await tester.pumpAndSettle();

      expect(find.byType(TimelineScreen), findsNothing);
      expect(find.text('Lifelog'), findsOneWidget);
    });

    testWidgets('screen reader: inactive tile is disabled', (tester) async {
      final semantics = tester.ensureSemantics();
      await startApp(tester);

      expect(
        tester.getSemantics(find.bySemanticsLabel('Sport, bald verfügbar')),
        isSemantics(isButton: true, isEnabled: false),
      );
      expect(
        tester.getSemantics(find.bySemanticsLabel('Gesundheit')),
        isSemantics(isButton: true, isEnabled: true),
      );
      semantics.dispose();
    });
  });

  group('navigation', () {
    testWidgets('health opens the shell, „Bereiche“ leads back', (
      tester,
    ) async {
      await startApp(tester);
      await openHealth(tester);

      expect(find.byType(TimelineScreen), findsOneWidget);
      expect(find.text('Labor Muster GmbH, Salzburg'), findsOneWidget);

      await tester.tap(find.byTooltip('Bereiche'));
      await tester.pumpAndSettle();

      expect(find.byType(TimelineScreen), findsNothing);
      expect(find.text('Bald verfügbar'), findsNWidgets(3));
    });

    testWidgets('„Bereiche“ also exists in upload and questions', (
      tester,
    ) async {
      await startApp(tester);
      for (final tab in ['Hochladen', 'Fragen']) {
        await openHealth(tester);
        await tester.tap(
          find.descendant(
            of: find.byType(NavigationBar),
            matching: find.text(tab),
          ),
        );
        await tester.pumpAndSettle();

        await tester.tap(find.byTooltip('Bereiche'));
        await tester.pumpAndSettle();
        expect(find.text('Bald verfügbar'), findsNWidgets(3), reason: tab);
      }
    });

    testWidgets('the system back button leads to the modules screen', (
      tester,
    ) async {
      await startApp(tester);
      await openHealth(tester);

      await tester.binding.handlePopRoute();
      await tester.pumpAndSettle();

      expect(find.byType(TimelineScreen), findsNothing);
      expect(find.text('Bald verfügbar'), findsNWidgets(3));
    });
  });

  group('color scheme', () {
    testWidgets('module in its color, modules screen neutral', (tester) async {
      await startApp(tester);
      final modulesPrimary = primaryAt(tester, find.text('Lifelog'));

      await openHealth(tester);
      final timelinePrimary = primaryAt(tester, find.byType(TimelineScreen));

      expect(timelinePrimary, primaryFrom(Colors.teal));
      expect(modulesPrimary, isNot(timelinePrimary));
    });

    testWidgets('color comes from the catalog', (tester) async {
      final violetCatalog = ModuleCatalog([
        modified(defaultModules.first, color: Colors.purple),
      ]);
      await startApp(tester, catalog: violetCatalog);
      await openHealth(tester);

      expect(
        primaryAt(tester, find.byType(TimelineScreen)),
        primaryFrom(Colors.purple),
      );
    });

    testWidgets('is kept in the entry, dialog and original file', (
      tester,
    ) async {
      final teal = primaryFrom(Colors.teal);
      await startApp(tester);
      await openHealth(tester);

      // Entry view (opened via Navigator.push)
      await tester.tap(find.text('Dr. Erika Beispiel, Allgemeinmedizin'));
      await tester.pumpAndSettle();
      expect(primaryAt(tester, find.byType(EntryScreen)), teal);

      // Dialog
      await tester.tap(find.byTooltip('Quelle korrigieren'));
      await tester.pumpAndSettle();
      expect(primaryAt(tester, find.byType(AlertDialog)), teal);
      await tester.tap(find.text('Abbrechen'));
      await tester.pumpAndSettle();

      // Original file (opened from the entry view)
      await tester.tap(find.text('Originaldatei öffnen'));
      await tester.pumpAndSettle();
      expect(primaryAt(tester, find.byType(OriginalFileScreen)), teal);
    });

    testWidgets('tiles show the color of their module', (tester) async {
      await startApp(tester);

      // Active tiles: gradient from the module's container colors.
      for (final module in defaultModules.where((m) => m.active)) {
        final ink = tester.widget<Ink>(
          find.ancestor(of: find.text(module.name), matching: find.byType(Ink)),
        );
        final gradient =
            (ink.decoration! as BoxDecoration).gradient! as LinearGradient;
        final scheme = ColorScheme.fromSeed(seedColor: module.color);
        expect(gradient.colors, [
          scheme.primaryContainer,
          scheme.secondaryContainer,
        ], reason: module.id);
      }
    });
  });

  testWidgets('timeline only shows entries of its own module', (tester) async {
    // Sport is active here; all mock entries belong to health.
    final catalog = ModuleCatalog([
      defaultModules.first,
      modified(defaultModules.last, active: true),
    ]);
    await startApp(tester, catalog: catalog);

    await tester.tap(find.text('Sport'));
    await tester.pumpAndSettle();
    expect(find.byType(TimelineScreen), findsOneWidget);
    expect(find.textContaining('Noch keine Dokumente'), findsOneWidget);

    await tester.tap(find.byTooltip('Bereiche'));
    await tester.pumpAndSettle();
    await openHealth(tester);
    expect(find.text('Labor Muster GmbH, Salzburg'), findsOneWidget);
  });
}
