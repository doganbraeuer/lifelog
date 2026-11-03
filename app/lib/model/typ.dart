/// Erlaubte Dokumenttypen im Modul `gesundheit` (docs/API.md, Abschnitt 2).
const gesundheitTypen = ['arztbrief', 'laborbefund', 'rezept', 'sonstiges'];

/// Anzeigename für einen Typ. Unbekannte Typen (z. B. aus einem späteren
/// Modul) werden unverändert angezeigt.
String typAnzeige(String typ) => switch (typ) {
  'arztbrief' => 'Arztbrief',
  'laborbefund' => 'Laborbefund',
  'rezept' => 'Rezept',
  'sonstiges' => 'Sonstiges',
  _ => typ,
};
