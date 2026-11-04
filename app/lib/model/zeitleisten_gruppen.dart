import '../api/api_client.dart';
import 'eintrag.dart';

/// Deutsche Monatsnamen (ohne `intl`).
const monatsnamen = [
  'Januar',
  'Februar',
  'März',
  'April',
  'Mai',
  'Juni',
  'Juli',
  'August',
  'September',
  'Oktober',
  'November',
  'Dezember',
];

/// Titel der Gruppe für Einträge ohne Dokumentdatum.
const ohneDatumTitel = 'Ohne Datum';

/// Ein Abschnitt der Zeitachse, z. B. „Oktober 2026“.
class ZeitleistenGruppe {
  const ZeitleistenGruppe(this.titel, this.eintraege);

  final String titel;
  final List<EintragKurz> eintraege;
}

/// Teilt die Einträge in Monatsgruppen für die Zeitachse.
///
/// - Die Reihenfolge des Backends bleibt erhalten; aufeinanderfolgende
///   Einträge mit gleichem Monat bilden eine Gruppe.
/// - Der Monat kommt aus dem gewählten Sortierfeld: beim Dokumentdatum aus
///   dem reinen Datum (ohne Zeitzone), beim Importdatum aus der lokalen Zeit
///   (wie es auch angezeigt wird).
/// - Einträge ohne Dokumentdatum kommen in eine eigene Gruppe
///   [ohneDatumTitel] am Ende.
List<ZeitleistenGruppe> gruppiereZeitleiste(
  List<EintragKurz> eintraege,
  Sortierung sortierung,
) {
  final gruppen = <(String, List<EintragKurz>)>[];
  final ohneDatum = <EintragKurz>[];
  for (final eintrag in eintraege) {
    final titel = _monatstitel(eintrag, sortierung);
    if (titel == null) {
      ohneDatum.add(eintrag);
    } else if (gruppen.isNotEmpty && gruppen.last.$1 == titel) {
      gruppen.last.$2.add(eintrag);
    } else {
      gruppen.add((titel, [eintrag]));
    }
  }
  return [
    for (final (titel, liste) in gruppen)
      ZeitleistenGruppe(titel, List.unmodifiable(liste)),
    if (ohneDatum.isNotEmpty)
      ZeitleistenGruppe(ohneDatumTitel, List.unmodifiable(ohneDatum)),
  ];
}

String? _monatstitel(EintragKurz eintrag, Sortierung sortierung) {
  switch (sortierung) {
    case Sortierung.dokumentdatum:
      final datum = eintrag.dokumentdatum;
      if (datum == null) return null;
      return '${monatsnamen[datum.monat - 1]} ${datum.jahr}';
    case Sortierung.importdatum:
      final lokal = eintrag.importdatum.toLocal();
      return '${monatsnamen[lokal.month - 1]} ${lokal.year}';
  }
}
