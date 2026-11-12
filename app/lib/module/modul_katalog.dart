import 'package:flutter/material.dart';

import 'modul_definition.dart';

/// Die Module von Lifelog. Ein neues Modul ist ein neuer Eintrag hier.
///
/// Die IDs sind Werte von `modul` in der API. Festgelegt ist bisher nur
/// `gesundheit` (docs/API.md); `ernaehrung`, `rezepte` und `sport` sind
/// noch mit Dogan bzw. dem Backend abzustimmen. Inaktive Module haben noch
/// keine Typen; die werden erst festgelegt, wenn das Modul gebaut wird.
const standardModule = <ModulDefinition>[
  ModulDefinition(
    id: 'health',
    name: 'Gesundheit',
    beschreibung: 'Befunde, Arztbriefe, Laborwerte',
    symbol: Icons.monitor_heart_outlined,
    farbe: Colors.teal,
    aktiv: true,
    // Genau die Typen aus docs/API.md, Abschnitt 2.
    // Farben der Punkte in der Zeitachse, angelehnt an die Kugeln im
    // App-Icon (orange, blau). Rezept bekommt Violett statt Rot: Rot wirkt
    // in einer Gesundheits-Zeitleiste wie eine Warnung. Die Farben
    // kennzeichnen nur den Typ, nie einen Wert.
    typen: [
      TypDefinition(
        id: 'doctor_letter',
        name: 'Arztbrief',
        symbol: Icons.description_outlined,
        farbe: Color(0xFFF5A623), // Orange wie im Icon
      ),
      TypDefinition(
        id: 'lab_report',
        name: 'Laborbefund',
        symbol: Icons.science_outlined,
        farbe: Color(0xFF2F80ED), // Blau wie im Icon
      ),
      TypDefinition(
        id: 'prescription',
        name: 'Rezept',
        symbol: Icons.medication_outlined,
        farbe: Color(0xFF7E57C2), // Violett
      ),
      TypDefinition(
        id: 'other',
        name: 'Sonstiges',
        symbol: Icons.insert_drive_file_outlined,
        // keine eigene Farbe: Modulfarbe (Teal)
      ),
    ],
  ),
  ModulDefinition(
    id: 'nutrition',
    name: 'Ernährung',
    beschreibung: 'Mahlzeiten und Nährwerte',
    symbol: Icons.eco_outlined,
    farbe: Color(0xFF8DC63F), // helles Apfelgrün, klar anders als Teal
    aktiv: false,
    typen: [],
  ),
  ModulDefinition(
    id: 'recipes',
    name: 'Rezepte',
    beschreibung: 'Kochrezepte sammeln',
    symbol: Icons.menu_book_outlined,
    farbe: Colors.orange,
    aktiv: false,
    typen: [],
  ),
  ModulDefinition(
    id: 'sport',
    name: 'Sport',
    beschreibung: 'Training und Wettkämpfe',
    symbol: Icons.directions_run,
    farbe: Colors.blue,
    aktiv: false,
    typen: [],
  ),
];

/// Die Liste der Module mit Funktionen zum Nachschlagen.
///
/// Heute kommt die Liste aus [standardModule]. Später kann sie vom Backend
/// geladen werden; dann wird nur die Stelle ausgetauscht, die den Katalog
/// erzeugt.
class ModulKatalog {
  ModulKatalog(List<ModulDefinition> module)
    : module = List.unmodifiable(module) {
    final ids = module.map((m) => m.id).toSet();
    if (ids.length != module.length) {
      throw ArgumentError('Modul-IDs müssen eindeutig sein.');
    }
  }

  factory ModulKatalog.standard() => ModulKatalog(standardModule);

  final List<ModulDefinition> module;

  /// Symbol für unbekannte Typen.
  static const unbekanntesSymbol = Icons.insert_drive_file_outlined;

  /// Der Katalog aus dem nächsten [ModulKatalogScope] oberhalb.
  static ModulKatalog von(BuildContext context) {
    final scope = context
        .dependOnInheritedWidgetOfExactType<ModulKatalogScope>();
    assert(scope != null, 'Kein ModulKatalogScope oberhalb von $context');
    return scope!.katalog;
  }

  ModulDefinition? nachId(String id) =>
      module.where((m) => m.id == id).firstOrNull;

  /// Erlaubte Typen eines Moduls; leer, wenn das Modul unbekannt ist.
  List<TypDefinition> typen(String modulId) => nachId(modulId)?.typen ?? [];

  /// Anzeigename eines Typs. Unbekannte Typen (z. B. aus einem neueren
  /// Backend) werden unverändert angezeigt.
  String typName(String modulId, String typId) =>
      nachId(modulId)?.typ(typId)?.name ?? typId;

  IconData typSymbol(String modulId, String typId) =>
      nachId(modulId)?.typ(typId)?.symbol ?? unbekanntesSymbol;

  /// Farbe eines Typs: eigene Typfarbe, sonst Modulfarbe, sonst neutral.
  Color typFarbe(String modulId, String typId) {
    final modul = nachId(modulId);
    return modul?.typ(typId)?.farbe ?? modul?.farbe ?? Colors.blueGrey;
  }
}

/// Stellt den [ModulKatalog] allen Seiten und Dialogen bereit.
///
/// Liegt oberhalb der `MaterialApp`, damit auch Seiten, die mit
/// `Navigator.push` geöffnet werden, ihn finden.
class ModulKatalogScope extends InheritedWidget {
  const ModulKatalogScope({
    super.key,
    required this.katalog,
    required super.child,
  });

  final ModulKatalog katalog;

  @override
  bool updateShouldNotify(ModulKatalogScope oldWidget) =>
      katalog != oldWidget.katalog;
}
