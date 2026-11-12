import 'datum.dart';
import 'json_hilfen.dart';

/// Antwort auf `POST /api/v1/questions` (docs/API.md, Abschnitt 3.6).
class FrageAntwort {
  const FrageAntwort({
    required this.antwort,
    required this.gefunden,
    required this.quellen,
  });

  factory FrageAntwort.fromJson(Map<String, dynamic> json) => FrageAntwort(
    antwort: text(json, 'answer'),
    gefunden: wahrheitswert(json, 'found'),
    quellen: liste(json, 'sources', Quelle.fromJson),
  );

  /// Antworttext. Verweise wie `[1]` zeigen auf [Quelle.nr].
  final String antwort;

  /// `false`, wenn kein passender Abschnitt gefunden wurde.
  final bool gefunden;
  final List<Quelle> quellen;
}

/// Ein Abschnitt, auf den sich die Antwort stützt.
class Quelle {
  const Quelle({
    required this.nr,
    required this.eintragId,
    required this.modul,
    required this.typ,
    required this.quelle,
    required this.dokumentdatum,
    required this.auszug,
  });

  factory Quelle.fromJson(Map<String, dynamic> json) => Quelle(
    nr: ganzzahl(json, 'number'),
    eintragId: text(json, 'entryId'),
    modul: text(json, 'module'),
    typ: text(json, 'type'),
    quelle: textOderNull(json, 'issuer'),
    dokumentdatum: datumOderNull(json, 'documentDate'),
    auszug: text(json, 'excerpt'),
  );

  /// Passt zum Verweis `[nr]` im Antworttext. Nicht unbedingt lückenlos
  /// (z. B. nur 1 und 3), weil nur zitierte Abschnitte geliefert werden.
  final int nr;
  final String eintragId;

  /// Modul des Eintrags. Ein `typ` ist nur zusammen damit eindeutig.
  final String modul;
  final String typ;
  final String? quelle;
  final Datum? dokumentdatum;
  final String auszug;
}
