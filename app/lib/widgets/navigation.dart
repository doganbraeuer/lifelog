import 'package:flutter/material.dart';

import '../module/module_definition.dart';

/// Color scheme of a module, derived from its base color.
ThemeData moduleTheme(ModuleDefinition module) =>
    ThemeData(colorSchemeSeed: module.color);

/// Opens [page] with `Navigator.push` and passes the color scheme of the
/// caller along.
///
/// Why this is needed: new pages hang on the app's top-level navigator,
/// outside the module shell, and would otherwise get the app's base scheme.
/// Dialogs and the date picker get this right on their own.
Future<T?> openPage<T>(BuildContext context, Widget page) {
  final navigator = Navigator.of(context);
  final themes = InheritedTheme.capture(from: context, to: navigator.context);
  return navigator.push(
    MaterialPageRoute<T>(builder: (_) => themes.wrap(page)),
  );
}

/// Button on the left in a module's app bar: back to the „Bereiche“ screen.
class ModulesButton extends StatelessWidget {
  const ModulesButton({super.key});

  @override
  Widget build(BuildContext context) {
    return IconButton(
      icon: const Icon(Icons.apps),
      tooltip: 'Bereiche',
      onPressed: () => Navigator.of(context).maybePop(),
    );
  }
}
