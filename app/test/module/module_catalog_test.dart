import 'package:flutter/material.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:lifelog/module/module_definition.dart';
import 'package:lifelog/module/module_catalog.dart';

void main() {
  final catalog = ModuleCatalog.defaults();

  group('Standard-Module', () {
    test('IDs wie vorgesehen (noch abzustimmen), in dieser Reihenfolge', () {
      expect(catalog.modules.map((m) => m.id), [
        'health',
        'nutrition',
        'recipes',
        'sport',
      ]);
    });

    test('nur Gesundheit ist aktiv', () {
      expect(catalog.modules.where((m) => m.active).map((m) => m.id), [
        'health',
      ]);
    });

    test('Gesundheit hat genau die Typen aus docs/API.md', () {
      expect(catalog.types('health').map((t) => t.id), [
        'doctor_letter',
        'lab_report',
        'prescription',
        'other',
      ]);
    });

    test('inaktive Module haben noch keine Typen', () {
      for (final module in catalog.modules.where((m) => !m.active)) {
        expect(module.types, isEmpty, reason: module.id);
      }
    });

    test('jedes Modul hat eine eigene Farbe', () {
      final farben = catalog.modules.map((m) => m.color.toARGB32()).toSet();
      expect(farben, hasLength(catalog.modules.length));
    });

    test('Typ-IDs sind innerhalb jedes Moduls eindeutig', () {
      for (final module in catalog.modules) {
        final ids = module.types.map((t) => t.id).toList();
        expect(ids.toSet(), hasLength(ids.length), reason: module.id);
      }
    });
  });

  group('Nachschlagen', () {
    test('Modul nach ID', () {
      expect(catalog.byId('health')?.name, 'Gesundheit');
      expect(catalog.byId('nutrition')?.name, 'Ernährung');
      expect(catalog.byId('garten'), isNull);
    });

    test('Typname und Symbol', () {
      expect(catalog.typeName('health', 'lab_report'), 'Laborbefund');
      expect(catalog.typeIcon('health', 'lab_report'), Icons.science_outlined);
    });

    test('unbekannter Typ oder unbekanntes Modul: ID und Ersatz-Symbol', () {
      expect(catalog.typeName('health', 'roentgenbild'), 'roentgenbild');
      expect(catalog.typeName('garten', 'lab_report'), 'lab_report');
      expect(
        catalog.typeIcon('garten', 'lab_report'),
        ModuleCatalog.unknownIcon,
      );
      expect(catalog.types('garten'), isEmpty);
    });

    test('ein Typ gilt nur in seinem Modul', () {
      // „rezept“ ist in Gesundheit ein ärztliches Rezept; Sport kennt ihn
      // nicht, also gibt es dort nur den Rohwert.
      expect(catalog.typeName('health', 'prescription'), 'Rezept');
      expect(catalog.byId('sport')!.typeById('prescription'), isNull);
    });
  });

  test('doppelte Modul-IDs werden abgelehnt', () {
    const module = ModuleDefinition(
      id: 'doppelt',
      name: 'Doppelt',
      description: '',
      icon: Icons.star,
      color: Colors.red,
      types: [],
      active: true,
    );
    expect(() => ModuleCatalog([module, module]), throwsArgumentError);
  });

  test('ein neues Modul ist nur ein neuer Eintrag', () {
    final mitGarten = ModuleCatalog([
      ...defaultModules,
      const ModuleDefinition(
        id: 'garten',
        name: 'Garten',
        description: 'Pflanzen und Ernten',
        icon: Icons.yard_outlined,
        color: Colors.lime,
        types: [
          TypeDefinition(id: 'saatgut', name: 'Saatgut', icon: Icons.grass),
        ],
        active: true,
      ),
    ]);
    expect(mitGarten.typeName('garten', 'saatgut'), 'Saatgut');
    expect(mitGarten.modules, hasLength(5));
  });
}
