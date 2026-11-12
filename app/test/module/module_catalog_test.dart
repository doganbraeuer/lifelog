import 'package:flutter/material.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:lifelog/module/module_definition.dart';
import 'package:lifelog/module/module_catalog.dart';

void main() {
  final catalog = ModuleCatalog.defaults();

  group('default modules', () {
    test('IDs as planned (still to be agreed), in this order', () {
      expect(catalog.modules.map((m) => m.id), [
        'health',
        'nutrition',
        'recipes',
        'sport',
      ]);
    });

    test('only health is active', () {
      expect(catalog.modules.where((m) => m.active).map((m) => m.id), [
        'health',
      ]);
    });

    test('health has exactly the types from docs/API.md', () {
      expect(catalog.types('health').map((t) => t.id), [
        'doctor_letter',
        'lab_report',
        'prescription',
        'other',
      ]);
    });

    test('inactive modules have no types yet', () {
      for (final module in catalog.modules.where((m) => !m.active)) {
        expect(module.types, isEmpty, reason: module.id);
      }
    });

    test('every module has its own color', () {
      final scheme = catalog.modules.map((m) => m.color.toARGB32()).toSet();
      expect(scheme, hasLength(catalog.modules.length));
    });

    test('type IDs are unique within every module', () {
      for (final module in catalog.modules) {
        final ids = module.types.map((t) => t.id).toList();
        expect(ids.toSet(), hasLength(ids.length), reason: module.id);
      }
    });
  });

  group('lookup', () {
    test('module by ID', () {
      expect(catalog.byId('health')?.name, 'Gesundheit');
      expect(catalog.byId('nutrition')?.name, 'Ernährung');
      expect(catalog.byId('garten'), isNull);
    });

    test('type name and icon', () {
      expect(catalog.typeName('health', 'lab_report'), 'Laborbefund');
      expect(catalog.typeIcon('health', 'lab_report'), Icons.science_outlined);
    });

    test('unknown type or unknown module: ID and fallback icon', () {
      expect(catalog.typeName('health', 'roentgenbild'), 'roentgenbild');
      expect(catalog.typeName('garten', 'lab_report'), 'lab_report');
      expect(
        catalog.typeIcon('garten', 'lab_report'),
        ModuleCatalog.unknownIcon,
      );
      expect(catalog.types('garten'), isEmpty);
    });

    test('a type only applies in its module', () {
      // „prescription“ is a medical prescription in health; sport does not
      // know it, so there is only the raw value there.
      expect(catalog.typeName('health', 'prescription'), 'Rezept');
      expect(catalog.byId('sport')!.typeById('prescription'), isNull);
    });
  });

  test('duplicate module IDs are rejected', () {
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

  test('a new module is just a new item', () {
    final withGarden = ModuleCatalog([
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
    expect(withGarden.typeName('garten', 'saatgut'), 'Saatgut');
    expect(withGarden.modules, hasLength(5));
  });
}
