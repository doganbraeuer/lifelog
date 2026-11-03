import '../model/eintrag.dart';
import 'api_client.dart';

/// Sortiert Einträge nach den Regeln aus docs/API.md, Abschnitt 3.2.
///
/// Die App selbst sortiert nie, das macht das Backend. Diese Funktion
/// braucht nur der Mock, damit er sich genauso verhält wie das Backend.
List<EintragKurz> sortiereZeitleiste(
  Iterable<EintragKurz> eintraege,
  Sortierung sortierung,
  Richtung richtung,
) {
  final liste = eintraege.toList();
  liste.sort((a, b) => vergleicheZeitleiste(a, b, sortierung, richtung));
  return liste;
}

/// Vergleich zweier Einträge:
/// 1. Nach dem gewählten Feld in der gewählten Richtung.
/// 2. Bei `sortierung=dokumentdatum` stehen Einträge ohne Datum immer am
///    Ende, untereinander nach `importdatum` absteigend (bei Gleichstand
///    `id` absteigend).
/// 3. Bei gleichem Wert entscheidet `importdatum` in derselben Richtung,
///    danach `id`.
int vergleicheZeitleiste(
  EintragKurz a,
  EintragKurz b,
  Sortierung sortierung,
  Richtung richtung,
) {
  final faktor = richtung == Richtung.aufsteigend ? 1 : -1;

  if (sortierung == Sortierung.dokumentdatum) {
    final datumA = a.dokumentdatum;
    final datumB = b.dokumentdatum;
    if (datumA == null && datumB == null) {
      final nachImport = b.importdatum.compareTo(a.importdatum);
      return nachImport != 0 ? nachImport : b.id.compareTo(a.id);
    }
    if (datumA == null) return 1;
    if (datumB == null) return -1;
    final nachDatum = datumA.compareTo(datumB);
    if (nachDatum != 0) return faktor * nachDatum;
  }

  final nachImport = a.importdatum.compareTo(b.importdatum);
  if (nachImport != 0) return faktor * nachImport;
  return faktor * a.id.compareTo(b.id);
}
