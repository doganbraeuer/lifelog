/// Ein reines Kalenderdatum ohne Uhrzeit und ohne Zeitzone.
///
/// Wird für `dokumentdatum` verwendet. Ein Befund vom 14.10. soll immer am
/// 14.10. stehen. Mit `DateTime` könnte sich das Datum je nach Zeitzone des
/// Geräts um einen Tag verschieben, deshalb speichern wir nur Jahr, Monat
/// und Tag.
class Datum implements Comparable<Datum> {
  const Datum._(this.jahr, this.monat, this.tag);

  /// Erzeugt ein Datum und prüft, ob es im Kalender existiert.
  /// `Datum(2026, 2, 30)` wirft einen [ArgumentError].
  factory Datum(int jahr, int monat, int tag) {
    if (!istGueltig(jahr, monat, tag)) {
      throw ArgumentError('Kein gültiges Datum: $jahr-$monat-$tag');
    }
    return Datum._(jahr, monat, tag);
  }

  /// Übernimmt Jahr, Monat und Tag aus einem [DateTime], so wie sie dort
  /// stehen, ohne Umrechnung in eine andere Zeitzone. Gedacht für den
  /// DatePicker, der ein lokales `DateTime` liefert.
  factory Datum.aus(DateTime zeit) => Datum(zeit.year, zeit.month, zeit.day);

  final int jahr;
  final int monat;
  final int tag;

  /// Genau `JJJJ-MM-TT`, so wie in docs/API.md festgelegt.
  static final format = RegExp(r'^(\d{4})-(\d{2})-(\d{2})$');

  /// Liest `JJJJ-MM-TT`. Gibt `null` zurück, wenn das Format nicht stimmt
  /// oder das Datum im Kalender nicht existiert (z. B. `2026-02-30`).
  static Datum? tryParse(String text) {
    final treffer = format.firstMatch(text);
    if (treffer == null) return null;
    final jahr = int.parse(treffer.group(1)!);
    final monat = int.parse(treffer.group(2)!);
    final tag = int.parse(treffer.group(3)!);
    if (!istGueltig(jahr, monat, tag)) return null;
    return Datum._(jahr, monat, tag);
  }

  /// Wie [tryParse], wirft aber eine [FormatException].
  static Datum parse(String text) =>
      tryParse(text) ??
      (throw FormatException('Kein gültiges Datum im Format JJJJ-MM-TT', text));

  static bool istGueltig(int jahr, int monat, int tag) {
    if (jahr < 1 || jahr > 9999) return false;
    if (monat < 1 || monat > 12) return false;
    return tag >= 1 && tag <= tageImMonat(jahr, monat);
  }

  static int tageImMonat(int jahr, int monat) {
    const tage = [31, 28, 31, 30, 31, 30, 31, 31, 30, 31, 30, 31];
    if (monat == 2 && istSchaltjahr(jahr)) return 29;
    return tage[monat - 1];
  }

  static bool istSchaltjahr(int jahr) =>
      (jahr % 4 == 0 && jahr % 100 != 0) || jahr % 400 == 0;

  /// Format für die API: `2026-10-14`.
  String get iso =>
      '${_zweistellig(jahr, 4)}-${_zweistellig(monat, 2)}-'
      '${_zweistellig(tag, 2)}';

  /// Format für die Anzeige: `14.10.2026`.
  String get anzeige =>
      '${_zweistellig(tag, 2)}.${_zweistellig(monat, 2)}.'
      '${_zweistellig(jahr, 4)}';

  /// Lokale Mitternacht dieses Tages. Nur für Widgets wie den DatePicker
  /// gedacht, die ein `DateTime` verlangen. Zurück geht es mit [Datum.aus].
  DateTime alsDateTime() => DateTime(jahr, monat, tag);

  static String _zweistellig(int zahl, int stellen) =>
      zahl.toString().padLeft(stellen, '0');

  @override
  int compareTo(Datum other) {
    if (jahr != other.jahr) return jahr.compareTo(other.jahr);
    if (monat != other.monat) return monat.compareTo(other.monat);
    return tag.compareTo(other.tag);
  }

  @override
  bool operator ==(Object other) =>
      other is Datum &&
      other.jahr == jahr &&
      other.monat == monat &&
      other.tag == tag;

  @override
  int get hashCode => Object.hash(jahr, monat, tag);

  @override
  String toString() => iso;
}
