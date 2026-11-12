import 'package:flutter/material.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:lifelog/module/module_definition.dart';
import 'package:lifelog/module/module_catalog.dart';

void main() {
  final katalog = ModulKatalog.standard();

  group('Standard-Module', () {
    test('IDs wie vorgesehen (noch abzustimmen), in dieser Reihenfolge', () {
      expect(katalog.module.map((m) => m.id), [
        'health',
        'nutrition',
        'recipes',
        'sport',
      ]);
    });

    test('nur Gesundheit ist aktiv', () {
      expect(katalog.module.where((m) => m.aktiv).map((m) => m.id), [
        'health',
      ]);
    });

    test('Gesundheit hat genau die Typen aus docs/API.md', () {
      expect(katalog.typen('health').map((t) => t.id), [
        'doctor_letter',
        'lab_report',
        'prescription',
        'other',
      ]);
    });

    test('inaktive Module haben noch keine Typen', () {
      for (final modul in katalog.module.where((m) => !m.aktiv)) {
        expect(modul.typen, isEmpty, reason: modul.id);
      }
    });

    test('jedes Modul hat eine eigene Farbe', () {
      final farben = katalog.module.map((m) => m.farbe.toARGB32()).toSet();
      expect(farben, hasLength(katalog.module.length));
    });

    test('Typ-IDs sind innerhalb jedes Moduls eindeutig', () {
      for (final modul in katalog.module) {
        final ids = modul.typen.map((t) => t.id).toList();
        expect(ids.toSet(), hasLength(ids.length), reason: modul.id);
      }
    });
  });

  group('Nachschlagen', () {
    test('Modul nach ID', () {
      expect(katalog.nachId('health')?.name, 'Gesundheit');
      expect(katalog.nachId('nutrition')?.name, 'Ernährung');
      expect(katalog.nachId('garten'), isNull);
    });

    test('Typname und Symbol', () {
      expect(katalog.typName('health', 'lab_report'), 'Laborbefund');
      expect(
        katalog.typSymbol('health', 'lab_report'),
        Icons.science_outlined,
      );
    });

    test('unbekannter Typ oder unbekanntes Modul: ID und Ersatz-Symbol', () {
      expect(katalog.typName('health', 'roentgenbild'), 'roentgenbild');
      expect(katalog.typName('garten', 'lab_report'), 'lab_report');
      expect(
        katalog.typSymbol('garten', 'lab_report'),
        ModulKatalog.unbekanntesSymbol,
      );
      expect(katalog.typen('garten'), isEmpty);
    });

    test('ein Typ gilt nur in seinem Modul', () {
      // „rezept“ ist in Gesundheit ein ärztliches Rezept; Sport kennt ihn
      // nicht, also gibt es dort nur den Rohwert.
      expect(katalog.typName('health', 'prescription'), 'Rezept');
      expect(katalog.nachId('sport')!.typ('prescription'), isNull);
    });
  });

  test('doppelte Modul-IDs werden abgelehnt', () {
    const modul = ModulDefinition(
      id: 'doppelt',
      name: 'Doppelt',
      beschreibung: '',
      symbol: Icons.star,
      farbe: Colors.red,
      typen: [],
      aktiv: true,
    );
    expect(() => ModulKatalog([modul, modul]), throwsArgumentError);
  });

  test('ein neues Modul ist nur ein neuer Eintrag', () {
    final mitGarten = ModulKatalog([
      ...standardModule,
      const ModulDefinition(
        id: 'garten',
        name: 'Garten',
        beschreibung: 'Pflanzen und Ernten',
        symbol: Icons.yard_outlined,
        farbe: Colors.lime,
        typen: [
          TypDefinition(id: 'saatgut', name: 'Saatgut', symbol: Icons.grass),
        ],
        aktiv: true,
      ),
    ]);
    expect(mitGarten.typName('garten', 'saatgut'), 'Saatgut');
    expect(mitGarten.module, hasLength(5));
  });
}
