import 'package:flutter/material.dart';

import '../module/module_definition.dart';

/// Farbschema eines Moduls, erzeugt aus seiner Grundfarbe.
ThemeData modulTheme(ModuleDefinition module) =>
    ThemeData(colorSchemeSeed: module.color);

/// Öffnet [seite] mit `Navigator.push` und gibt ihr das Farbschema der
/// aufrufenden Stelle mit.
///
/// Warum nötig: Neue Seiten hängen am obersten Navigator der App, also
/// außerhalb des Modul-Rahmens, und bekämen sonst das Grundschema der App.
/// Dialoge und die Datumsauswahl machen das von sich aus richtig.
Future<T?> oeffneSeite<T>(BuildContext context, Widget seite) {
  final navigator = Navigator.of(context);
  final themes = InheritedTheme.capture(from: context, to: navigator.context);
  return navigator.push(
    MaterialPageRoute<T>(builder: (_) => themes.wrap(seite)),
  );
}

/// Knopf links in der AppBar eines Moduls: zurück zum Screen „Bereiche“.
class BereicheKnopf extends StatelessWidget {
  const BereicheKnopf({super.key});

  @override
  Widget build(BuildContext context) {
    return IconButton(
      icon: const Icon(Icons.apps),
      tooltip: 'Bereiche',
      onPressed: () => Navigator.of(context).maybePop(),
    );
  }
}
