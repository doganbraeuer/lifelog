import 'package:flutter/material.dart';
import 'package:lifelog/module/module_definition.dart';
import 'package:lifelog/module/module_catalog.dart';

/// Gesundheit wie im Standard, aber mit dem zusätzlichen Typ „impfpass“.
/// Dazu Sport mit dem Typ „training“. Damit lässt sich prüfen, dass die
/// Typen wirklich aus dem Katalog kommen und nur im eigenen Modul gelten.
ModuleCatalog testKatalog() {
  final gesundheit = defaultModules.first;
  return ModuleCatalog([
    ModuleDefinition(
      id: gesundheit.id,
      name: gesundheit.name,
      description: gesundheit.description,
      icon: gesundheit.icon,
      color: gesundheit.color,
      active: true,
      types: [
        ...gesundheit.types,
        const TypeDefinition(
          id: 'impfpass',
          name: 'Impfpass',
          icon: Icons.vaccines_outlined,
        ),
      ],
    ),
    const ModuleDefinition(
      id: 'sport',
      name: 'Sport',
      description: '',
      icon: Icons.directions_run,
      color: Colors.blue,
      active: false,
      types: [
        TypeDefinition(
          id: 'training',
          name: 'Training',
          icon: Icons.fitness_center,
        ),
      ],
    ),
  ]);
}
