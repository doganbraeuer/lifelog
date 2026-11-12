import 'package:flutter/material.dart';

import 'module_definition.dart';

/// The modules of Lifelog. A new module is a new item here.
///
/// The IDs are values of `module` in the API. So far only `health` is fixed
/// (docs/API.md); `nutrition`, `recipes` and `sport` still need to be agreed
/// with Dogan and the backend. Inactive modules have no types yet; they are
/// only defined once the module is built.
const defaultModules = <ModuleDefinition>[
  ModuleDefinition(
    id: 'health',
    name: 'Gesundheit',
    description: 'Befunde, Arztbriefe, Laborwerte',
    icon: Icons.monitor_heart_outlined,
    color: Colors.teal,
    active: true,
    // Exactly the types from docs/API.md, section 2.
    // Colors of the dots on the timeline axis, based on the balls in the
    // app icon (orange, blue). Prescription gets violet instead of red: red
    // looks like a warning in a health timeline. The colors only mark the
    // type, never a value.
    types: [
      TypeDefinition(
        id: 'doctor_letter',
        name: 'Arztbrief',
        icon: Icons.description_outlined,
        color: Color(0xFFF5A623), // orange as in the icon
      ),
      TypeDefinition(
        id: 'lab_report',
        name: 'Laborbefund',
        icon: Icons.science_outlined,
        color: Color(0xFF2F80ED), // blue as in the icon
      ),
      TypeDefinition(
        id: 'prescription',
        name: 'Rezept',
        icon: Icons.medication_outlined,
        color: Color(0xFF7E57C2), // violet
      ),
      TypeDefinition(
        id: 'other',
        name: 'Sonstiges',
        icon: Icons.insert_drive_file_outlined,
        // no color of its own: module color (teal)
      ),
    ],
  ),
  ModuleDefinition(
    id: 'nutrition',
    name: 'Ernährung',
    description: 'Mahlzeiten und Nährwerte',
    icon: Icons.eco_outlined,
    color: Color(0xFF8DC63F), // light apple green, clearly unlike teal
    active: false,
    types: [],
  ),
  ModuleDefinition(
    id: 'recipes',
    name: 'Rezepte',
    description: 'Kochrezepte sammeln',
    icon: Icons.menu_book_outlined,
    color: Colors.orange,
    active: false,
    types: [],
  ),
  ModuleDefinition(
    id: 'sport',
    name: 'Sport',
    description: 'Training und Wettkämpfe',
    icon: Icons.directions_run,
    color: Colors.blue,
    active: false,
    types: [],
  ),
];

/// The list of modules with lookup functions.
///
/// Today the list comes from [defaultModules]. Later it can be loaded from
/// the backend; then only the place that creates the catalog changes.
class ModuleCatalog {
  ModuleCatalog(List<ModuleDefinition> modules)
    : modules = List.unmodifiable(modules) {
    final ids = modules.map((m) => m.id).toSet();
    if (ids.length != modules.length) {
      throw ArgumentError('Module IDs must be unique.');
    }
  }

  factory ModuleCatalog.defaults() => ModuleCatalog(defaultModules);

  final List<ModuleDefinition> modules;

  /// Icon for unknown types.
  static const unknownIcon = Icons.insert_drive_file_outlined;

  /// The catalog from the nearest [ModuleCatalogScope] above.
  static ModuleCatalog of(BuildContext context) {
    final scope = context
        .dependOnInheritedWidgetOfExactType<ModuleCatalogScope>();
    assert(scope != null, 'No ModuleCatalogScope above $context');
    return scope!.catalog;
  }

  ModuleDefinition? byId(String id) =>
      modules.where((m) => m.id == id).firstOrNull;

  /// Allowed types of a module; empty if the module is unknown.
  List<TypeDefinition> types(String moduleId) => byId(moduleId)?.types ?? [];

  /// Display name of a type. Unknown types (e.g. from a newer backend) are
  /// shown unchanged.
  String typeName(String moduleId, String typeId) =>
      byId(moduleId)?.typeById(typeId)?.name ?? typeId;

  IconData typeIcon(String moduleId, String typeId) =>
      byId(moduleId)?.typeById(typeId)?.icon ?? unknownIcon;

  /// Color of a type: its own type color, otherwise the module color,
  /// otherwise neutral.
  Color typeColor(String moduleId, String typeId) {
    final module = byId(moduleId);
    return module?.typeById(typeId)?.color ?? module?.color ?? Colors.blueGrey;
  }
}

/// Provides the [ModuleCatalog] to all pages and dialogs.
///
/// Sits above the `MaterialApp`, so that pages opened with `Navigator.push`
/// find it too.
class ModuleCatalogScope extends InheritedWidget {
  const ModuleCatalogScope({
    super.key,
    required this.catalog,
    required super.child,
  });

  final ModuleCatalog catalog;

  @override
  bool updateShouldNotify(ModuleCatalogScope oldWidget) =>
      catalog != oldWidget.catalog;
}
