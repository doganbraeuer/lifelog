import 'dart:convert';
import 'dart:math';

import 'package:crypto/crypto.dart';
import 'package:flutter/foundation.dart';

import '../model/answer.dart';
import '../model/plain_date.dart';
import '../model/entry.dart';
import '../model/entry_patch.dart';
import '../model/json_helpers.dart';
import '../module/module_catalog.dart';
import 'api_client.dart';
import 'api_exception.dart';
import 'mime_type.dart';
import 'mock/mock_data.dart';
import 'timeline_sort.dart';

/// Antwort ohne Treffer, wörtlich aus docs/API.md.
const keinTrefferAntwort = 'Dazu finde ich nichts in deinen Dokumenten.';

/// Ein Backend im Speicher, das sich wie docs/API.md verhält: gleiche
/// Sortierung, gleiche Korrekturregeln, gleiche Fehlercodes. So bringt der
/// Wechsel zum echten Backend keine Überraschungen.
///
/// Intern speichert der Mock jeden Eintrag als JSON-Map, genau wie das
/// Backend ihn liefern würde. Korrekturen prüft er am JSON-Body, so dass
/// auch Fehler wie ein falsches Datumsformat nachgestellt werden können.
///
/// Fehler zum Ausprobieren von Hand:
/// - Upload mit `unlesbar`, `auslesefehler` oder `modellfehler` im
///   Dateinamen liefert 422, 502 bzw. 503.
/// - Eine Frage mit dem Wort `modellfehler` liefert 503.
class MockApiClient implements ApiClient {
  MockApiClient({
    bool mitBeispieldaten = true,
    this.verzoegerung = const Duration(milliseconds: 300),
    this.uploadVerzoegerung = const Duration(seconds: 2),
    DateTime Function()? uhr,
    ModulKatalog? katalog,
  }) : _uhr = uhr ?? (() => DateTime.now().toUtc()),
       _katalog = katalog ?? ModulKatalog.standard() {
    if (mitBeispieldaten) {
      for (final e in beispieldaten()) {
        final id = e.json['id'] as String;
        _eintraege[id] = e.json;
        _dateien[id] = e.bytes;
      }
    }
  }

  /// Künstliche Wartezeit pro Aufruf, damit Ladeanzeigen sichtbar werden.
  final Duration verzoegerung;

  /// Wartezeit beim Upload (das echte Backend braucht bis zu 60 s).
  final Duration uploadVerzoegerung;

  /// Wenn `true`, scheitert jeder Aufruf mit `500 INTERNAL_ERROR`.
  bool simulierterFehler = false;

  final DateTime Function() _uhr;

  /// Erlaubte Typen pro Modul, wie sie auch das Backend prüft.
  final ModulKatalog _katalog;
  final _eintraege = <String, Map<String, dynamic>>{};
  final _dateien = <String, Uint8List>{};
  final _zufall = Random();
  var _zaehler = 0;

  @override
  bool get istMock => true;

  @override
  void schliessen() {}

  // ---------------------------------------------------------------------
  // GET /entries
  // ---------------------------------------------------------------------

  @override
  Future<List<EintragKurz>> eintraege({
    required String modul,
    Sortierung sortierung = Sortierung.documentDate,
    Richtung richtung = Richtung.desc,
  }) => eintraegeMitParametern({
    'module': modul,
    'sort': sortierung.name,
    'order': richtung.name,
  });

  /// Wie [eintraege], aber mit rohen Query-Parametern. So lässt sich auch
  /// `400 INVALID_PARAMETER` testen.
  @visibleForTesting
  Future<List<EintragKurz>> eintraegeMitParametern(
    Map<String, String> parameter,
  ) async {
    await _warte(verzoegerung);
    final modul = parameter['module'] ?? '';
    if (!_modulBekannt(modul)) {
      throw ApiFehler(
        httpStatus: 400,
        code: FehlerCode.ungueltigerParameter,
        nachricht: _unbekanntesModul,
        details: {'parameter': 'module', 'value': modul},
      );
    }
    final sortierung = _enumParameter(
      parameter,
      'sort',
      Sortierung.values,
      Sortierung.documentDate,
      'Unbekannte Sortierung. Erlaubt sind documentDate und importedAt.',
    );
    final richtung = _enumParameter(
      parameter,
      'order',
      Richtung.values,
      Richtung.desc,
      'Unbekannte Richtung. Erlaubt sind desc und asc.',
    );
    final gewaehlt = _modulOderStandard(modul);
    return sortiereZeitleiste(
      _eintraege.values
          .where((json) => json['module'] == gewaehlt)
          .map(EintragKurz.fromJson),
      sortierung,
      richtung,
    );
  }

  // ---------------------------------------------------------------------
  // Module (docs/API.md, Abschnitt 1 „Module“)
  // ---------------------------------------------------------------------

  static const _unbekanntesModul = 'Unbekanntes Modul.';

  /// Fehlt `modul` oder ist es leer, gilt `gesundheit`.
  static String _modulOderStandard(String modul) =>
      modul.isEmpty ? 'health' : modul;

  /// Bekannt ist jedes Modul aus dem Katalog. (Das echte Backend kennt im
  /// Prototyp nur `gesundheit`; die App fragt andere Module aber gar nicht
  /// an, solange sie inaktiv sind.)
  bool _modulBekannt(String modul) =>
      _katalog.nachId(_modulOderStandard(modul)) != null;

  /// Prüft das Modul für Upload, Text-Import und Frage: unbekannt ergibt
  /// `400 INVALID_REQUEST`. Gibt die gültige Modul-ID zurück.
  String _pruefeModul(String modul) {
    if (!_modulBekannt(modul)) {
      throw ApiFehler(
        httpStatus: 400,
        code: FehlerCode.ungueltigeAnfrage,
        nachricht: _unbekanntesModul,
        details: {'field': 'module', 'value': modul},
      );
    }
    return _modulOderStandard(modul);
  }

  /// Wie [_pruefeModul], dazu: In ein Modul ohne Typen kann nichts
  /// importiert werden, weil es keinen erlaubten `typ` gibt.
  String _pruefeImportModul(String modul) {
    final id = _pruefeModul(modul);
    if (_katalog.typen(id).isEmpty) {
      throw ApiFehler(
        httpStatus: 400,
        code: FehlerCode.ungueltigeAnfrage,
        nachricht: 'In diesem Modul ist noch kein Import möglich.',
        details: {'field': 'module', 'value': modul},
      );
    }
    return id;
  }

  /// Bevorzugter Typ, falls das Modul ihn kennt, sonst der erste.
  String _typFuer(String modul, String bevorzugt) {
    final typen = _katalog.typen(modul).map((t) => t.id).toList();
    return typen.contains(bevorzugt) ? bevorzugt : typen.first;
  }

  T _enumParameter<T extends Enum>(
    Map<String, String> parameter,
    String name,
    List<T> werte,
    T standard,
    String nachricht,
  ) {
    final wert = parameter[name];
    if (wert == null) return standard;
    for (final w in werte) {
      if (w.name == wert) return w;
    }
    throw ApiFehler(
      httpStatus: 400,
      code: FehlerCode.ungueltigerParameter,
      nachricht: nachricht,
      details: {'parameter': name, 'value': wert},
    );
  }

  // ---------------------------------------------------------------------
  // GET /entries/{id} und /entries/{id}/file
  // ---------------------------------------------------------------------

  @override
  Future<Eintrag> eintrag(String id) async {
    await _warte(verzoegerung);
    return Eintrag.fromJson(_finde(id));
  }

  @override
  Future<DateiInhalt> datei(String id) async {
    await _warte(verzoegerung);
    final json = _finde(id);
    final datei = objekt(json['file'], 'file');
    return DateiInhalt(
      bytes: _dateien[json['id']]!,
      mimeTyp: text(datei, 'mimeType'),
    );
  }

  static final _objectId = RegExp(r'^[0-9a-fA-F]{24}$');

  Map<String, dynamic> _finde(String id) {
    if (!_objectId.hasMatch(id)) {
      throw ApiFehler(
        httpStatus: 400,
        code: FehlerCode.ungueltigeId,
        nachricht: 'Die ID ist ungültig.',
      );
    }
    final json = _eintraege[id.toLowerCase()];
    if (json == null) {
      throw const ApiFehler(
        httpStatus: 404,
        code: FehlerCode.eintragNichtGefunden,
        nachricht: 'Diesen Eintrag gibt es nicht.',
      );
    }
    return json;
  }

  // ---------------------------------------------------------------------
  // POST /entries
  // ---------------------------------------------------------------------

  @override
  Future<Eintrag> hochladen(
    Uint8List bytes,
    String dateiname, {
    required String modul,
  }) async {
    await _warte(uploadVerzoegerung);

    // 1. Modul, Größe und Typ prüfen.
    final modulId = _pruefeImportModul(modul);
    if (bytes.isEmpty) {
      throw const ApiFehler(
        httpStatus: 400,
        code: FehlerCode.dateiFehlt,
        nachricht: 'Es wurde keine Datei mitgeschickt.',
      );
    }
    if (bytes.length > maxDateiGroesse) {
      throw const ApiFehler(
        httpStatus: 413,
        code: FehlerCode.dateiZuGross,
        nachricht: 'Die Datei ist größer als 10 MB.',
      );
    }
    final mimeTyp = erkenneMimeTyp(bytes);
    if (mimeTyp == null) {
      throw const ApiFehler(
        httpStatus: 415,
        code: FehlerCode.dateitypNichtUnterstuetzt,
        nachricht:
            'Dieser Dateityp wird nicht unterstützt. '
            'Erlaubt sind JPEG, PNG, HEIC und PDF.',
      );
    }

    // 2. Duplikat-Prüfung über den SHA-256-Hash, vor dem Modellaufruf.
    final hash = _pruefeDuplikat(
      bytes,
      'Diese Datei hast du schon hochgeladen.',
    );

    // 3. "Vision-LLM": Fehler lassen sich über den Dateinamen auslösen.
    _pruefeAusloeser(dateiname);

    // 4.–6. Eintrag anlegen und speichern.
    return _speichern(
      modul: modulId,
      typ: _typFuer(modulId, 'doctor_letter'),
      dokumentdatum: Datum.aus(_uhr().toLocal()),
      quelle: 'Mock-Praxis Beispiel',
      volltext:
          'Mock-Praxis Beispiel\n'
          'Dieser Eintrag stammt aus dem Mock, nicht aus einer echten '
          'Auswertung.\n'
          'Dateiname: $dateiname\n'
          'Blutdruck 120/80 mmHg',
      infos: [
        {'id': 'f1', 'name': 'Blutdruck', 'value': '120/80', 'unit': 'mmHg'},
      ],
      dateiname: dateiname,
      mimeTyp: mimeTyp,
      bytes: bytes,
      hash: hash,
    );
  }

  /// Datum `TT.MM.JJJJ` im Text (z. B. „vom 21.10.2026“).
  static final _datumImText = RegExp(r'\b(\d{1,2})\.(\d{1,2})\.(\d{4})\b');

  @override
  Future<Eintrag> textImportieren(String text, {required String modul}) async {
    await _warte(uploadVerzoegerung);

    // 1. Modul und Text prüfen.
    final modulId = _pruefeImportModul(modul);
    if (text.trim().isEmpty) {
      throw const ApiFehler(
        httpStatus: 400,
        code: FehlerCode.dateiFehlt,
        nachricht: 'Bitte einen Text eingeben.',
      );
    }
    if (text.runes.length > maxTextLaenge) {
      throw const ApiFehler(
        httpStatus: 413,
        code: FehlerCode.dateiZuGross,
        nachricht: 'Der Text ist länger als 100 000 Zeichen.',
        details: {'maxChars': maxTextLaenge},
      );
    }

    // 2. Der Text wird als text.txt gespeichert; Duplikat über diese Bytes.
    final bytes = Uint8List.fromList(utf8.encode(text));
    final hash = _pruefeDuplikat(
      bytes,
      'Diesen Text hast du schon importiert.',
    );

    // 3. "Sprachmodell": Auslöser wie beim Dateinamen, hier im Text.
    _pruefeAusloeser(text);

    // Statt eines Modells: erstes Datum im Text, erste Zeile als Quelle.
    // volltext ist der eingegebene Text, unverändert (docs/API.md 3.1).
    Datum? datum;
    for (final treffer in _datumImText.allMatches(text)) {
      final tag = int.parse(treffer[1]!);
      final monat = int.parse(treffer[2]!);
      final jahr = int.parse(treffer[3]!);
      if (Datum.istGueltig(jahr, monat, tag)) {
        datum = Datum(jahr, monat, tag);
        break;
      }
    }
    final ersteZeile = text
        .split('\n')
        .map((z) => z.trim())
        .firstWhere((z) => z.isNotEmpty);

    return _speichern(
      modul: modulId,
      typ: _typFuer(modulId, 'other'),
      dokumentdatum: datum,
      quelle: ersteZeile,
      volltext: text,
      infos: const [],
      dateiname: 'text.txt',
      mimeTyp: 'text/plain; charset=utf-8',
      bytes: bytes,
      hash: hash,
    );
  }

  /// Wirft `409 DUPLICATE`, wenn es den Hash schon gibt (über alle Module).
  /// Gibt sonst den Hash zurück.
  String _pruefeDuplikat(Uint8List bytes, String nachricht) {
    final hash = sha256.convert(bytes).toString();
    for (final json in _eintraege.values) {
      if (objekt(json['file'], 'file')['sha256'] == hash) {
        throw ApiFehler(
          httpStatus: 409,
          code: FehlerCode.duplikat,
          nachricht: nachricht,
          details: {'entryId': json['id']},
        );
      }
    }
    return hash;
  }

  /// Fehler des Modells zum Ausprobieren: `unlesbar`, `auslesefehler`
  /// oder `modellfehler` im Dateinamen bzw. im Text.
  void _pruefeAusloeser(String inhalt) {
    final klein = inhalt.toLowerCase();
    if (klein.contains('unlesbar')) {
      throw const ApiFehler(
        httpStatus: 422,
        code: FehlerCode.nichtLesbar,
        nachricht:
            'In der Datei ist kein Dokument erkennbar. '
            'Versuche es mit einem schärferen Foto.',
      );
    }
    if (klein.contains('auslesefehler')) {
      throw const ApiFehler(
        httpStatus: 502,
        code: FehlerCode.auslesenFehlgeschlagen,
        nachricht:
            'Das Dokument konnte nicht ausgewertet werden. '
            'Bitte später noch einmal versuchen.',
      );
    }
    if (klein.contains('modellfehler')) {
      throw _modellNichtErreichbar;
    }
  }

  /// Legt einen neuen Eintrag samt Originaldatei an.
  Eintrag _speichern({
    required String modul,
    required String typ,
    required Datum? dokumentdatum,
    required String? quelle,
    required String volltext,
    required List<Map<String, dynamic>> infos,
    required String dateiname,
    required String mimeTyp,
    required Uint8List bytes,
    required String hash,
  }) {
    final id = _neueId();
    final json = <String, dynamic>{
      'id': id,
      'module': modul,
      'type': typ,
      'documentDate': dokumentdatum?.iso,
      'importedAt': formatZeitpunkt(_uhr()),
      'issuer': quelle,
      'fullText': volltext,
      'facts': <dynamic>[
        for (final info in infos) <String, dynamic>{...info},
      ],
      'file': <String, dynamic>{
        'id': _neueId(),
        'fileName': dateiname,
        'mimeType': mimeTyp,
        'size': bytes.length,
        'sha256': hash,
      },
      'corrections': <dynamic>[],
    };
    _eintraege[id] = json;
    _dateien[id] = Uint8List.fromList(bytes);
    return Eintrag.fromJson(json);
  }

  // ---------------------------------------------------------------------
  // DELETE /entries/{id}
  // ---------------------------------------------------------------------

  @override
  Future<void> loeschen(String id) async {
    await _warte(verzoegerung);
    final json = _finde(id); // 400 INVALID_ID oder 404
    // Im Mock gibt es keine Abschnitte; Datei und Eintrag verschwinden.
    // Danach ist der Hash frei, dieselbe Datei lässt sich neu hochladen.
    _dateien.remove(json['id']);
    _eintraege.remove(json['id']);
  }

  /// ID im Stil einer MongoDB-ObjectId: 24 Hex-Zeichen.
  String _neueId() {
    String id;
    do {
      final sekunden = _uhr().millisecondsSinceEpoch ~/ 1000;
      final zufall = [
        for (var i = 0; i < 5; i++)
          _zufall.nextInt(256).toRadixString(16).padLeft(2, '0'),
      ].join();
      _zaehler = (_zaehler + 1) & 0xFFFFFF;
      id =
          sekunden.toRadixString(16).padLeft(8, '0') +
          zufall +
          _zaehler.toRadixString(16).padLeft(6, '0');
    } while (_eintraege.containsKey(id));
    return id;
  }

  // ---------------------------------------------------------------------
  // PATCH /entries/{id}
  // ---------------------------------------------------------------------

  @override
  Future<Eintrag> korrigieren(String id, EintragAenderung aenderung) =>
      korrigiereMitJson(id, aenderung.toJson());

  /// Wie [korrigieren], aber mit rohem JSON-Body. So lassen sich auch
  /// ungültige Anfragen testen, die mit [EintragAenderung] gar nicht
  /// entstehen können (z. B. `"documentDate": "15.10.2026"`).
  @visibleForTesting
  Future<Eintrag> korrigiereMitJson(
    String id,
    Map<String, dynamic> body,
  ) async {
    await _warte(verzoegerung);
    final json = _finde(id);

    // 1. Alles prüfen. Ist ein Teil ungültig, wird nichts geändert.
    final aenderungen = _pruefeKorrektur(json, body);

    // 2. und 3. Werte schreiben und `korrekturen` pflegen.
    final jetzt = formatZeitpunkt(_uhr());
    for (final (feld, neu) in aenderungen) {
      _setze(json, feld, neu, jetzt);
    }
    return Eintrag.fromJson(json);
  }

  static const _korrigierbar = {'type', 'documentDate', 'issuer', 'facts'};
  static const _nichtKorrigierbar = {
    'id',
    'module',
    'importedAt',
    'fullText',
    'file',
    'corrections',
  };

  /// Prüft den Body und gibt die Änderungen als Liste von
  /// (Feldpfad, neuer Wert) zurück.
  List<(String, String?)> _pruefeKorrektur(
    Map<String, dynamic> json,
    Map<String, dynamic> body,
  ) {
    if (body.isEmpty) {
      throw _ungueltig('Die Anfrage enthält keine Änderung.');
    }
    for (final feld in body.keys) {
      if (_nichtKorrigierbar.contains(feld)) {
        throw _nichtKorrigierbarFehler(feld);
      }
      if (!_korrigierbar.contains(feld)) {
        throw _ungueltig('Unbekanntes Feld: $feld.', {'field': feld});
      }
    }

    final ergebnis = <(String, String?)>[];

    if (body.containsKey('type')) {
      final typ = body['type'];
      // Ein Typ gilt nur innerhalb des Moduls, zu dem der Eintrag gehört.
      final erlaubt = [
        for (final t in _katalog.typen(json['module'] as String)) t.id,
      ];
      if (typ is! String || !erlaubt.contains(typ)) {
        throw _ungueltig(
          'Unbekannter Typ. Erlaubt sind ${erlaubt.join(', ')}.',
          {'field': 'type', 'value': typ},
        );
      }
      ergebnis.add(('type', typ));
    }

    if (body.containsKey('documentDate')) {
      final wert = body['documentDate'];
      if (wert == null) {
        ergebnis.add(('documentDate', null));
      } else if (wert is String && Datum.tryParse(wert) != null) {
        ergebnis.add(('documentDate', wert));
      } else if (wert is String && Datum.format.hasMatch(wert)) {
        throw _ungueltig('Das Dokumentdatum ist kein gültiges Kalenderdatum.', {
          'field': 'documentDate',
          'value': wert,
        });
      } else {
        throw _ungueltig(
          'Das Dokumentdatum muss im Format JJJJ-MM-TT angegeben werden.',
          {'field': 'documentDate', 'value': wert},
        );
      }
    }

    if (body.containsKey('issuer')) {
      final wert = body['issuer'];
      if (wert != null && wert is! String) {
        throw _ungueltig('Die Quelle muss ein Text oder null sein.', {
          'field': 'issuer',
          'value': wert,
        });
      }
      // Leerraum entfernen; bleibt nichts übrig, gilt das wie null (3.5).
      ergebnis.add(('issuer', _ohneLeerraum(wert as String?)));
    }

    if (body.containsKey('facts')) {
      final infos = body['facts'];
      if (infos is! List) {
        throw _ungueltig('"facts" muss eine Liste sein.', {'field': 'facts'});
      }
      final vorhandeneIds = {
        for (final info in json['facts'] as List) (info as Map)['id'],
      };
      final gesehen = <String>{};
      for (final element in infos) {
        if (element is! Map<String, dynamic>) {
          throw _ungueltig('Jede Info muss ein Objekt sein.');
        }
        final id = element['id'];
        if (id is! String || id.isEmpty) {
          throw _ungueltig('Jede Info braucht eine id.', {'field': 'facts'});
        }
        if (!gesehen.add(id)) {
          throw _ungueltig('Die Info $id kommt mehrfach vor.', {'factId': id});
        }
        for (final schluessel in element.keys) {
          if (schluessel == 'name') {
            throw _nichtKorrigierbarFehler('facts.$id.name');
          }
          if (!{'id', 'value', 'unit'}.contains(schluessel)) {
            throw _ungueltig('Unbekanntes Feld: facts.$id.$schluessel.', {
              'field': 'facts.$id.$schluessel',
            });
          }
        }
        if (!element.containsKey('value') && !element.containsKey('unit')) {
          throw _ungueltig('Die Info $id enthält keine Änderung.', {
            'factId': id,
          });
        }
        if (!vorhandeneIds.contains(id)) {
          throw ApiFehler(
            httpStatus: 404,
            code: FehlerCode.infoNichtGefunden,
            nachricht: 'Diese Info gibt es in dem Eintrag nicht.',
            details: {'factId': id},
          );
        }
        if (element.containsKey('value')) {
          final wert = element['value'];
          if (wert is! String) {
            throw _ungueltig('Der Wert muss ein Text sein.', {
              'field': 'facts.$id.value',
              'value': wert,
            });
          }
          // Eine Info hat immer einen Wert: leer ist ein Fehler (3.5).
          if (wert.trim().isEmpty) {
            throw _ungueltig('Der Wert darf nicht leer sein.', {
              'field': 'facts.$id.value',
              'value': wert,
            });
          }
          ergebnis.add(('facts.$id.value', wert.trim()));
        }
        if (element.containsKey('unit')) {
          final einheit = element['unit'];
          if (einheit != null && einheit is! String) {
            throw _ungueltig('Die Einheit muss ein Text oder null sein.', {
              'field': 'facts.$id.unit',
              'value': einheit,
            });
          }
          ergebnis.add((
            'facts.$id.unit',
            _ohneLeerraum(einheit as String?),
          ));
        }
      }
    }
    return ergebnis;
  }

  /// Entfernt Leerraum am Anfang und Ende; ein leerer Text wird zu `null`.
  static String? _ohneLeerraum(String? text) {
    final ohne = text?.trim();
    return (ohne == null || ohne.isEmpty) ? null : ohne;
  }

  /// Setzt ein Feld und pflegt `korrekturen` nach den Regeln aus
  /// docs/API.md, Abschnitt 2:
  /// - Ein Feld, dessen Wert sich nicht ändert, erzeugt keine Korrektur.
  /// - Pro Feld höchstens eine Korrektur; `erkannt` bleibt der Originalwert.
  /// - Zurück auf den erkannten Wert: Die Korrektur wird entfernt.
  void _setze(
    Map<String, dynamic> json,
    String feld,
    String? neu,
    String zeitpunkt,
  ) {
    final alt = _lies(json, feld);
    if (alt == neu) return;

    final korrekturen = json['corrections'] as List;
    final index = korrekturen.indexWhere((k) => (k as Map)['field'] == feld);
    // Gibt es noch keine Korrektur, ist der aktuelle Wert der erkannte.
    final erkannt = index >= 0
        ? (korrekturen[index] as Map)['recognized'] as String?
        : alt;

    _schreibe(json, feld, neu);

    if (neu == erkannt) {
      if (index >= 0) korrekturen.removeAt(index);
    } else if (index >= 0) {
      final korrektur = korrekturen[index] as Map<String, dynamic>;
      korrektur['corrected'] = neu;
      korrektur['correctedAt'] = zeitpunkt;
    } else {
      korrekturen.add(<String, dynamic>{
        'field': feld,
        'recognized': erkannt,
        'corrected': neu,
        'correctedAt': zeitpunkt,
      });
    }
  }

  String? _lies(Map<String, dynamic> json, String feld) {
    final (info, schluessel) = _ziel(json, feld);
    return info[schluessel] as String?;
  }

  void _schreibe(Map<String, dynamic> json, String feld, String? wert) {
    final (ziel, schluessel) = _ziel(json, feld);
    // Eine Einheit ohne Wert fehlt im JSON ganz (laut API "optional").
    if (wert == null && feld.startsWith('facts.')) {
      ziel.remove(schluessel);
    } else {
      ziel[schluessel] = wert;
    }
  }

  /// Liefert die Map und den Schlüssel, unter dem ein Feldpfad steht.
  (Map<String, dynamic>, String) _ziel(Map<String, dynamic> json, String feld) {
    if (!feld.startsWith('facts.')) return (json, feld);
    final teile = feld.split('.'); // infos, <id>, wert|einheit
    final info = (json['facts'] as List)
        .cast<Map<String, dynamic>>()
        .firstWhere((i) => i['id'] == teile[1]);
    return (info, teile[2]);
  }

  ApiFehler _ungueltig(String nachricht, [Map<String, dynamic>? details]) =>
      ApiFehler(
        httpStatus: 400,
        code: FehlerCode.ungueltigeAnfrage,
        nachricht: nachricht,
        details: details ?? const {},
      );

  ApiFehler _nichtKorrigierbarFehler(String feld) => ApiFehler(
    httpStatus: 400,
    code: FehlerCode.feldNichtKorrigierbar,
    nachricht: 'Das Feld $feld kann nicht korrigiert werden.',
    details: {'field': feld},
  );

  // ---------------------------------------------------------------------
  // POST /questions
  // ---------------------------------------------------------------------

  static const maxFrageLaenge = 500;

  @override
  Future<FrageAntwort> fragen(String frage, {required String modul}) async {
    await _warte(verzoegerung);
    // Wie im Backend: erst die Frage prüfen, dann das Modul.
    if (frage.trim().isEmpty) {
      throw const ApiFehler(
        httpStatus: 400,
        code: FehlerCode.frageFehlt,
        nachricht: 'Bitte gib eine Frage ein.',
      );
    }
    // Wie im Backend ohne details (docs/API.md 3.6).
    if (frage.runes.length > maxFrageLaenge) {
      throw const ApiFehler(
        httpStatus: 400,
        code: FehlerCode.frageZuLang,
        nachricht:
            'Die Frage darf höchstens $maxFrageLaenge Zeichen lang '
            'sein.',
      );
    }
    final modulId = _pruefeModul(modul);
    final woerter = _woerter(frage).where((w) => !_stoppwoerter.contains(w));
    if (woerter.contains('modellfehler')) throw _modellNichtErreichbar;

    // Statt Vektorsuche: einfache Wortsuche in den Zeilen des Volltexts.
    // Eine Zeile zählt pro Fragewort, das in einem ihrer Wörter steckt
    // (oder umgekehrt, damit "Hämoglobinwert" auch "Hämoglobin" findet).
    final suchwoerter = woerter.where((w) => w.length >= 4).toSet();
    final treffer = <(int, Map<String, dynamic>, String)>[];
    // Nur Dokumente des gewählten Moduls durchsuchen.
    for (final json in _eintraege.values.where((j) => j['module'] == modulId)) {
      for (final zeile in (json['fullText'] as String).split('\n')) {
        final zeilenWoerter = _woerter(zeile)
            .where((w) => w.length >= 4)
            .toList();
        final punkte = suchwoerter
            .where(
              (s) => zeilenWoerter.any((z) => z.contains(s) || s.contains(z)),
            )
            .length;
        if (punkte > 0) treffer.add((punkte, json, zeile.trim()));
      }
    }
    if (treffer.isEmpty) {
      return const FrageAntwort(
        antwort: keinTrefferAntwort,
        gefunden: false,
        quellen: [],
      );
    }

    // Höchstens zwei Abschnitte mit der besten Punktzahl.
    treffer.sort((a, b) => b.$1.compareTo(a.$1));
    final beste = treffer.where((t) => t.$1 == treffer.first.$1).take(2);
    final quellen = <Quelle>[];
    final saetze = <String>[];
    for (final (_, json, auszug) in beste) {
      final kurz = EintragKurz.fromJson(json);
      final nr = quellen.length + 1;
      quellen.add(
        Quelle(
          nr: nr,
          eintragId: kurz.id,
          modul: kurz.modul,
          typ: kurz.typ,
          quelle: kurz.quelle,
          dokumentdatum: kurz.dokumentdatum,
          auszug: auszug,
        ),
      );
      final von = kurz.quelle == null ? '' : ' von ${kurz.quelle}';
      final vom = kurz.dokumentdatum == null
          ? ''
          : ' vom ${kurz.dokumentdatum!.anzeige}';
      saetze.add(
        'Im Dokument „${_katalog.typName(kurz.modul, kurz.typ)}“$von$vom '
        'steht: '
        '$auszug [$nr].',
      );
    }
    return FrageAntwort(
      antwort: saetze.join(' '),
      gefunden: true,
      quellen: quellen,
    );
  }

  static final _wortMuster = RegExp(r'[\p{L}\p{N}]+', unicode: true);

  static Iterable<String> _woerter(String text) =>
      _wortMuster.allMatches(text.toLowerCase()).map((m) => m[0]!);

  static const _stoppwoerter = {
    'aber',
    'alle',
    'auch',
    'beim',
    'bitte',
    'dass',
    'diese',
    'dieser',
    'dokument',
    'dokumente',
    'dokumenten',
    'eine',
    'einem',
    'einen',
    'einer',
    'gibt',
    'habe',
    'haben',
    'hatte',
    'hatten',
    'meine',
    'meinem',
    'meinen',
    'meiner',
    'nach',
    'nicht',
    'noch',
    'oder',
    'sind',
    'stand',
    'steht',
    'über',
    'unter',
    'wann',
    'warum',
    'welche',
    'welchem',
    'welchen',
    'welcher',
    'welches',
    'wieso',
    'wurde',
    'wurden',
    'werden',
    'zeig',
    'zeige',
  };

  // ---------------------------------------------------------------------
  // Gemeinsames
  // ---------------------------------------------------------------------

  static const _modellNichtErreichbar = ApiFehler(
    httpStatus: 503,
    code: FehlerCode.modellNichtErreichbar,
    nachricht:
        'Der Auswertungsdienst ist gerade nicht erreichbar. '
        'Bitte später noch einmal versuchen.',
  );

  Future<void> _warte(Duration dauer) async {
    if (dauer > Duration.zero) await Future<void>.delayed(dauer);
    if (simulierterFehler) {
      throw const ApiFehler(
        httpStatus: 500,
        code: FehlerCode.internerFehler,
        nachricht:
            'Im Backend ist ein Fehler aufgetreten. '
            'Bitte später noch einmal versuchen.',
      );
    }
  }
}
