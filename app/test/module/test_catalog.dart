import 'package:flutter/material.dart';
import 'package:lifelog/module/module_definition.dart';
import 'package:lifelog/module/module_catalog.dart';

/// Health as in the defaults, but with the additional type „impfpass“.
/// Plus sport with the type „training“. This makes it possible to check
/// that types really come from the catalog and only apply in their module.
ModuleCatalog testCatalog() {
  final health = defaultModules.first;
  return ModuleCatalog([
    ModuleDefinition(
      id: health.id,
      name: health.name,
      description: health.description,
      icon: health.icon,
      color: health.color,
      active: true,
      types: [
        ...health.types,
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
