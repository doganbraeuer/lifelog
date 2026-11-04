import 'dart:typed_data';

import 'datum.dart';
import 'json_hilfen.dart';

/// Kurzform eines Eintrags für die Zeitleiste (docs/API.md, Abschnitt 2).
class EintragKurz {
  const EintragKurz({
    required this.id,
    required this.modul,
    required this.typ,
    required this.dokumentdatum,
    required this.importdatum,
    required this.quelle,
  });

  factory EintragKurz.fromJson(Map<String, dynamic> json) => EintragKurz(
    id: text(json, 'id'),
    modul: text(json, 'modul'),
    typ: text(json, 'typ'),
    dokumentdatum: datumOderNull(json, 'dokumentdatum'),
    importdatum: zeitstempel(json, 'importdatum'),
    quelle: textOderNull(json, 'quelle'),
  );

  final String id;
  final String modul;
  final String typ;

  /// `null`, wenn das Vision-LLM kein Datum erkannt hat.
  final Datum? dokumentdatum;

  /// Zeitpunkt des Uploads in UTC.
  final DateTime importdatum;

  /// Ausstellende Stelle, `null` wenn nicht erkannt.
  final String? quelle;
}

/// Vollständiger Eintrag mit Volltext, Infos, Datei und Korrekturen.
class Eintrag extends EintragKurz {
  const Eintrag({
    required super.id,
    required super.modul,
    required super.typ,
    required super.dokumentdatum,
    required super.importdatum,
    required super.quelle,
    required this.volltext,
    required this.infos,
    required this.datei,
    required this.korrekturen,
  });

  factory Eintrag.fromJson(Map<String, dynamic> json) {
    final kurz = EintragKurz.fromJson(json);
    return Eintrag(
      id: kurz.id,
      modul: kurz.modul,
      typ: kurz.typ,
      dokumentdatum: kurz.dokumentdatum,
      importdatum: kurz.importdatum,
      quelle: kurz.quelle,
      volltext: text(json, 'volltext'),
      infos: liste(json, 'infos', Info.fromJson),
      datei: Datei.fromJson(objekt(json['datei'], 'datei')),
      korrekturen: liste(json, 'korrekturen', Korrektur.fromJson),
    );
  }

  final String volltext;
  final List<Info> infos;
  final Datei datei;
  final List<Korrektur> korrekturen;

  /// Die Korrektur zu einem Feldpfad wie `dokumentdatum` oder
  /// `infos.i2.wert`, oder `null`, wenn das Feld nicht korrigiert wurde.
  Korrektur? korrektur(String feld) =>
      korrekturen.where((k) => k.feld == feld).firstOrNull;

  /// Regel 4 aus docs/API.md: Eine Info gilt als korrigiert, wenn ein
  /// `feld` in `korrekturen` mit `infos.<id>.` beginnt.
  bool infoKorrigiert(String infoId) =>
      korrekturen.any((k) => k.feld.startsWith('infos.$infoId.'));
}

/// Eine extrahierte Angabe, z. B. Hämoglobin 14,2 g/dl.
class Info {
  const Info({
    required this.id,
    required this.name,
    required this.wert,
    this.einheit,
  });

  factory Info.fromJson(Map<String, dynamic> json) => Info(
    id: text(json, 'id'),
    name: text(json, 'name'),
    wert: text(json, 'wert'),
    einheit: textOderNull(json, 'einheit'),
  );

  final String id;
  final String name;

  /// Immer ein Text, genau wie auf dem Dokument (`"14,2"`, `"<0,5"`).
  final String wert;
  final String? einheit;
}

/// Referenz auf die Originaldatei.
class Datei {
  const Datei({
    required this.id,
    required this.dateiname,
    required this.mimeTyp,
    required this.groesse,
    required this.sha256,
  });

  factory Datei.fromJson(Map<String, dynamic> json) => Datei(
    id: text(json, 'id'),
    dateiname: text(json, 'dateiname'),
    mimeTyp: text(json, 'mimeTyp'),
    groesse: ganzzahl(json, 'groesse'),
    sha256: text(json, 'sha256'),
  );

  final String id;
  final String dateiname;
  final String mimeTyp;

  /// Größe in Bytes.
  final int groesse;
  final String sha256;

  bool get istPdf => medientyp(mimeTyp) == 'application/pdf';
  bool get istBild => medientyp(mimeTyp).startsWith('image/');

  /// Text, z. B. `text.txt` aus einem Text-Import (`text/plain`, auch mit
  /// `; charset=utf-8`).
  bool get istText => medientyp(mimeTyp) == 'text/plain';
}

/// Der reine Medientyp ohne Parameter, klein geschrieben:
/// `Text/Plain; charset=UTF-8` → `text/plain`.
String medientyp(String mimeTyp) =>
    mimeTyp.split(';').first.trim().toLowerCase();

/// Eine Korrektur: ursprünglich erkannter und aktueller Wert.
class Korrektur {
  const Korrektur({
    required this.feld,
    required this.erkannt,
    required this.korrigiert,
    required this.zeitpunkt,
  });

  factory Korrektur.fromJson(Map<String, dynamic> json) => Korrektur(
    feld: text(json, 'feld'),
    erkannt: textOderNull(json, 'erkannt'),
    korrigiert: textOderNull(json, 'korrigiert'),
    zeitpunkt: zeitstempel(json, 'zeitpunkt'),
  );

  /// Pfad wie `typ`, `dokumentdatum`, `quelle`, `infos.i2.wert`.
  final String feld;
  final String? erkannt;
  final String? korrigiert;
  final DateTime zeitpunkt;
}

/// Inhalt der Originaldatei, wie ihn `GET /eintraege/{id}/datei` liefert.
class DateiInhalt {
  const DateiInhalt({required this.bytes, required this.mimeTyp});

  final Uint8List bytes;
  final String mimeTyp;

  bool get istPdf => medientyp(mimeTyp) == 'application/pdf';
  bool get istBild => medientyp(mimeTyp).startsWith('image/');
  bool get istText => medientyp(mimeTyp) == 'text/plain';
}
