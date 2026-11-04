import 'package:flutter/widgets.dart';

/// Ein erlaubter Dokumenttyp innerhalb eines Moduls.
class TypDefinition {
  const TypDefinition({
    required this.id,
    required this.name,
    required this.symbol,
  });

  /// Wert von `typ` in der API, z. B. `laborbefund`.
  final String id;

  /// Anzeigename, z. B. „Laborbefund“.
  final String name;
  final IconData symbol;
}

/// Ein Bereich von Lifelog, z. B. Gesundheit oder Sport.
///
/// Alle Module nutzen dieselben Screens; sie unterscheiden sich nur durch
/// diese Angaben. Ein neues Modul ist deshalb nur ein neuer Eintrag in
/// `standardModule` (modul_katalog.dart).
///
/// Später sollen die Module vom Backend kommen. Ein Symbol lässt sich nicht
/// als `IconData` übertragen; das Backend würde dann einen Schlüssel wie
/// `"herz"` schicken, den die App auf ein Symbol abbildet.
class ModulDefinition {
  const ModulDefinition({
    required this.id,
    required this.name,
    required this.beschreibung,
    required this.symbol,
    required this.farbe,
    required this.typen,
    required this.aktiv,
  });

  /// Wert von `modul` in der API, z. B. `gesundheit`.
  final String id;

  /// Anzeigename, z. B. „Gesundheit“.
  final String name;

  /// Kurzer Text auf der Kachel im Screen „Bereiche“.
  final String beschreibung;
  final IconData symbol;

  /// Grundfarbe; daraus entsteht das Farbschema des Moduls.
  final Color farbe;

  /// Erlaubte Dokumenttypen. Ein `typ` gilt immer nur innerhalb seines
  /// Moduls (docs/API.md, Weitere Module).
  final List<TypDefinition> typen;

  /// `false`: wird als „Bald verfügbar“ angezeigt und ist nicht nutzbar.
  final bool aktiv;

  /// Der Typ mit dieser ID oder `null`, wenn das Modul ihn nicht kennt.
  TypDefinition? typ(String id) => typen.where((t) => t.id == id).firstOrNull;
}
