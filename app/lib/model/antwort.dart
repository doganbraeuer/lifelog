import 'datum.dart';
import 'json_hilfen.dart';

/// Antwort auf `POST /api/v1/fragen` (docs/API.md, Abschnitt 3.6).
class FrageAntwort {
  const FrageAntwort({
    required this.antwort,
    required this.gefunden,
    required this.quellen,
  });

  factory FrageAntwort.fromJson(Map<String, dynamic> json) => FrageAntwort(
    antwort: text(json, 'antwort'),
    gefunden: wahrheitswert(json, 'gefunden'),
    quellen: liste(json, 'quellen', Quelle.fromJson),
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
    required this.typ,
    required this.quelle,
    required this.dokumentdatum,
    required this.auszug,
  });

  factory Quelle.fromJson(Map<String, dynamic> json) => Quelle(
    nr: ganzzahl(json, 'nr'),
    eintragId: text(json, 'eintragId'),
    typ: text(json, 'typ'),
    quelle: textOderNull(json, 'quelle'),
    dokumentdatum: datumOderNull(json, 'dokumentdatum'),
    auszug: text(json, 'auszug'),
  );

  final int nr;
  final String eintragId;
  final String typ;
  final String? quelle;
  final Datum? dokumentdatum;
  final String auszug;
}
