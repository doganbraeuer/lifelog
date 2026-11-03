import 'datum.dart';

/// Hülle für einen Wert, der auch `null` sein darf.
///
/// Damit unterscheidet ein PATCH drei Fälle:
/// - Feld ist `null` (keine Hülle): Das Feld fehlt im JSON und bleibt
///   unverändert.
/// - `Wert(null)`: Im JSON steht `"feld": null`, der Wert wird gelöscht.
/// - `Wert(x)`: Im JSON steht `"feld": x`.
class Wert<T> {
  const Wert(this.wert);

  final T wert;
}

/// Änderung an einer Info. `id` ist Pflicht.
class InfoAenderung {
  const InfoAenderung(this.id, {this.wert, this.einheit});

  final String id;

  /// Neuer Wert. `null` heißt: unverändert. (Ein Wert kann nicht gelöscht
  /// werden, laut API ist er immer ein Text.)
  final String? wert;

  /// `null`: unverändert. `Wert(null)`: Einheit entfernen.
  final Wert<String?>? einheit;

  Map<String, dynamic> toJson() => {
    'id': id,
    if (wert != null) 'wert': wert,
    if (einheit != null) 'einheit': einheit?.wert,
  };
}

/// Body für `PATCH /api/v1/eintraege/{id}` (docs/API.md, Abschnitt 3.5).
/// Er enthält nur die Felder, die sich ändern sollen.
class EintragAenderung {
  const EintragAenderung({
    this.typ,
    this.dokumentdatum,
    this.quelle,
    this.infos = const [],
  });

  /// `null`: unverändert.
  final String? typ;

  /// `null`: unverändert. `Wert(null)`: Datum entfernen.
  final Wert<Datum?>? dokumentdatum;

  /// `null`: unverändert. `Wert(null)`: Quelle entfernen.
  final Wert<String?>? quelle;

  final List<InfoAenderung> infos;

  bool get istLeer =>
      typ == null && dokumentdatum == null && quelle == null && infos.isEmpty;

  Map<String, dynamic> toJson() => {
    if (typ != null) 'typ': typ,
    if (dokumentdatum != null) 'dokumentdatum': dokumentdatum?.wert?.iso,
    if (quelle != null) 'quelle': quelle?.wert,
    if (infos.isNotEmpty) 'infos': [for (final i in infos) i.toJson()],
  };
}
