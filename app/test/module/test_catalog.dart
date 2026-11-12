import 'package:flutter/material.dart';
import 'package:lifelog/module/module_definition.dart';
import 'package:lifelog/module/module_catalog.dart';

/// Gesundheit wie im Standard, aber mit dem zusätzlichen Typ „impfpass“.
/// Dazu Sport mit dem Typ „training“. Damit lässt sich prüfen, dass die
/// Typen wirklich aus dem Katalog kommen und nur im eigenen Modul gelten.
ModulKatalog testKatalog() {
  final gesundheit = standardModule.first;
  return ModulKatalog([
    ModulDefinition(
      id: gesundheit.id,
      name: gesundheit.name,
      beschreibung: gesundheit.beschreibung,
      symbol: gesundheit.symbol,
      farbe: gesundheit.farbe,
      aktiv: true,
      typen: [
        ...gesundheit.typen,
        const TypDefinition(
          id: 'impfpass',
          name: 'Impfpass',
          symbol: Icons.vaccines_outlined,
        ),
      ],
    ),
    const ModulDefinition(
      id: 'sport',
      name: 'Sport',
      beschreibung: '',
      symbol: Icons.directions_run,
      farbe: Colors.blue,
      aktiv: false,
      typen: [
        TypDefinition(
          id: 'training',
          name: 'Training',
          symbol: Icons.fitness_center,
        ),
      ],
    ),
  ]);
}
