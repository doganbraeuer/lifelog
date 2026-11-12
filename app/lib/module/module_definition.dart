import 'package:flutter/widgets.dart';

/// An allowed document type within a module.
class TypeDefinition {
  const TypeDefinition({
    required this.id,
    required this.name,
    required this.icon,
    this.color,
  });

  /// Value of `type` in the API, e.g. `lab_report`.
  final String id;

  /// Display name, e.g. „Laborbefund“.
  final String name;
  final IconData icon;

  /// Color of the dot on the timeline axis. It only marks the type, never a
  /// value. If it is missing, the module's color applies.
  final Color? color;
}

/// An area of Lifelog, e.g. health or sport.
///
/// All modules use the same screens; they only differ in these settings.
/// A new module is therefore just a new item in `defaultModules`
/// (module_catalog.dart).
///
/// Later the modules are meant to come from the backend. An icon cannot be
/// sent as `IconData`; the backend would then send a key like `"heart"`
/// that the app maps to an icon.
class ModuleDefinition {
  const ModuleDefinition({
    required this.id,
    required this.name,
    required this.description,
    required this.icon,
    required this.color,
    required this.types,
    required this.active,
  });

  /// Value of `module` in the API, e.g. `health`.
  final String id;

  /// Display name, e.g. „Gesundheit“.
  final String name;

  /// Short text on the tile in the „Bereiche“ screen.
  final String description;
  final IconData icon;

  /// Base color; the module's color scheme is derived from it.
  final Color color;

  /// Allowed document types. A `type` only ever applies within its module
  /// (docs/API.md, further modules).
  final List<TypeDefinition> types;

  /// `false`: shown as „Bald verfügbar“ and not usable.
  final bool active;

  /// The type with this ID, or `null` if the module does not know it.
  TypeDefinition? typeById(String id) =>
      types.where((t) => t.id == id).firstOrNull;
}
