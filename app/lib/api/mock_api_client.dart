import 'dart:math';

import 'package:crypto/crypto.dart';
import 'package:flutter/foundation.dart';

import '../model/antwort.dart';
import '../model/datum.dart';
import '../model/eintrag.dart';
import '../model/eintrag_aenderung.dart';
import '../model/json_hilfen.dart';
import '../model/typ.dart';
import 'api_client.dart';
import 'api_fehler.dart';
import 'mime_typ.dart';
import 'mock/mock_daten.dart';
import 'zeitleisten_sortierung.dart';

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
  }) : _uhr = uhr ?? (() => DateTime.now().toUtc()) {
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

  /// Wenn `true`, scheitert jeder Aufruf mit `500 INTERNER_FEHLER`.
  bool simulierterFehler = false;

  final DateTime Function() _uhr;
  final _eintraege = <String, Map<String, dynamic>>{};
  final _dateien = <String, Uint8List>{};
  final _zufall = Random();
  var _zaehler = 0;

  @override
  bool get istMock => true;

  @override
  void schliessen() {}

  // ---------------------------------------------------------------------
  // GET /eintraege
  // ---------------------------------------------------------------------

  @override
  Future<List<EintragKurz>> eintraege({
    Sortierung sortierung = Sortierung.dokumentdatum,
    Richtung richtung = Richtung.absteigend,
  }) => eintraegeMitParametern({
    'sortierung': sortierung.name,
    'richtung': richtung.name,
  });

  /// Wie [eintraege], aber mit rohen Query-Parametern. So lässt sich auch
  /// `400 UNGUELTIGER_PARAMETER` testen.
  @visibleForTesting
  Future<List<EintragKurz>> eintraegeMitParametern(
    Map<String, String> parameter,
  ) async {
    await _warte(verzoegerung);
    final sortierung = _enumParameter(
      parameter,
      'sortierung',
      Sortierung.values,
      Sortierung.dokumentdatum,
      'Unbekannte Sortierung. Erlaubt sind dokumentdatum und importdatum.',
    );
    final richtung = _enumParameter(
      parameter,
      'richtung',
      Richtung.values,
      Richtung.absteigend,
      'Unbekannte Richtung. Erlaubt sind absteigend und aufsteigend.',
    );
    return sortiereZeitleiste(
      _eintraege.values.map(EintragKurz.fromJson),
      sortierung,
      richtung,
    );
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
      details: {'parameter': name, 'wert': wert},
    );
  }

  // ---------------------------------------------------------------------
  // GET /eintraege/{id} und /eintraege/{id}/datei
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
    final datei = objekt(json['datei'], 'datei');
    return DateiInhalt(
      bytes: _dateien[json['id']]!,
      mimeTyp: text(datei, 'mimeTyp'),
    );
  }

  static final _objectId = RegExp(r'^[0-9a-fA-F]{24}$');

  Map<String, dynamic> _finde(String id) {
    if (!_objectId.hasMatch(id)) {
      throw ApiFehler(
        httpStatus: 400,
        code: FehlerCode.ungueltigeId,
        nachricht: 'Die ID ist ungültig.',
        details: {'id': id},
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
  // POST /eintraege
  // ---------------------------------------------------------------------

  @override
  Future<Eintrag> hochladen(Uint8List bytes, String dateiname) async {
    await _warte(uploadVerzoegerung);

    // 1. Größe und Typ prüfen.
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
        details: {'maxBytes': maxDateiGroesse},
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
    final hash = sha256.convert(bytes).toString();
    for (final json in _eintraege.values) {
      if (objekt(json['datei'], 'datei')['sha256'] == hash) {
        throw ApiFehler(
          httpStatus: 409,
          code: FehlerCode.duplikat,
          nachricht: 'Diese Datei hast du schon hochgeladen.',
          details: {'eintragId': json['id']},
        );
      }
    }

    // 3. "Vision-LLM": Fehler lassen sich über den Dateinamen auslösen.
    final name = dateiname.toLowerCase();
    if (name.contains('unlesbar')) {
      throw const ApiFehler(
        httpStatus: 422,
        code: FehlerCode.nichtLesbar,
        nachricht:
            'In der Datei ist kein Dokument erkennbar. '
            'Versuche es mit einem schärferen Foto.',
      );
    }
    if (name.contains('auslesefehler')) {
      throw const ApiFehler(
        httpStatus: 502,
        code: FehlerCode.auslesenFehlgeschlagen,
        nachricht:
            'Das Dokument konnte nicht ausgewertet werden. '
            'Bitte später noch einmal versuchen.',
      );
    }
    if (name.contains('modellfehler')) {
      throw _modellNichtErreichbar;
    }

    // 4.–6. Eintrag anlegen und speichern.
    final jetzt = _uhr();
    final id = _neueId();
    final json = <String, dynamic>{
      'id': id,
      'modul': 'gesundheit',
      'typ': 'arztbrief',
      'dokumentdatum': Datum.aus(jetzt.toLocal()).iso,
      'importdatum': formatZeitpunkt(jetzt),
      'quelle': 'Mock-Praxis Beispiel',
      'volltext':
          'Mock-Praxis Beispiel\n'
          'Dieser Eintrag stammt aus dem Mock, nicht aus einer echten '
          'Auswertung.\n'
          'Dateiname: $dateiname\n'
          'Blutdruck 120/80 mmHg',
      'infos': <dynamic>[
        <String, dynamic>{
          'id': 'i1',
          'name': 'Blutdruck',
          'wert': '120/80',
          'einheit': 'mmHg',
        },
      ],
      'datei': <String, dynamic>{
        'id': _neueId(),
        'dateiname': dateiname,
        'mimeTyp': mimeTyp,
        'groesse': bytes.length,
        'sha256': hash,
      },
      'korrekturen': <dynamic>[],
    };
    _eintraege[id] = json;
    _dateien[id] = Uint8List.fromList(bytes);
    return Eintrag.fromJson(json);
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
  // PATCH /eintraege/{id}
  // ---------------------------------------------------------------------

  @override
  Future<Eintrag> korrigieren(String id, EintragAenderung aenderung) =>
      korrigiereMitJson(id, aenderung.toJson());

  /// Wie [korrigieren], aber mit rohem JSON-Body. So lassen sich auch
  /// ungültige Anfragen testen, die mit [EintragAenderung] gar nicht
  /// entstehen können (z. B. `"dokumentdatum": "15.10.2026"`).
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

  static const _korrigierbar = {'typ', 'dokumentdatum', 'quelle', 'infos'};
  static const _nichtKorrigierbar = {
    'id',
    'modul',
    'importdatum',
    'volltext',
    'datei',
    'korrekturen',
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
        throw _ungueltig('Unbekanntes Feld: $feld.', {'feld': feld});
      }
    }

    final ergebnis = <(String, String?)>[];

    if (body.containsKey('typ')) {
      final typ = body['typ'];
      if (typ is! String || !gesundheitTypen.contains(typ)) {
        throw _ungueltig(
          'Unbekannter Typ. Erlaubt sind ${gesundheitTypen.join(', ')}.',
          {'feld': 'typ', 'wert': typ},
        );
      }
      ergebnis.add(('typ', typ));
    }

    if (body.containsKey('dokumentdatum')) {
      final wert = body['dokumentdatum'];
      if (wert == null) {
        ergebnis.add(('dokumentdatum', null));
      } else if (wert is String && Datum.tryParse(wert) != null) {
        ergebnis.add(('dokumentdatum', wert));
      } else if (wert is String && Datum.format.hasMatch(wert)) {
        throw _ungueltig('Das Dokumentdatum ist kein gültiges Kalenderdatum.', {
          'feld': 'dokumentdatum',
          'wert': wert,
        });
      } else {
        throw _ungueltig(
          'Das Dokumentdatum muss im Format JJJJ-MM-TT angegeben werden.',
          {'feld': 'dokumentdatum', 'wert': wert},
        );
      }
    }

    if (body.containsKey('quelle')) {
      final wert = body['quelle'];
      if (wert != null && wert is! String) {
        throw _ungueltig('Die Quelle muss ein Text oder null sein.', {
          'feld': 'quelle',
          'wert': wert,
        });
      }
      ergebnis.add(('quelle', wert as String?));
    }

    if (body.containsKey('infos')) {
      final infos = body['infos'];
      if (infos is! List) {
        throw _ungueltig('"infos" muss eine Liste sein.', {'feld': 'infos'});
      }
      final vorhandeneIds = {
        for (final info in json['infos'] as List) (info as Map)['id'],
      };
      final gesehen = <String>{};
      for (final element in infos) {
        if (element is! Map<String, dynamic>) {
          throw _ungueltig('Jede Info muss ein Objekt sein.');
        }
        final id = element['id'];
        if (id is! String || id.isEmpty) {
          throw _ungueltig('Jede Info braucht eine id.', {'feld': 'infos'});
        }
        if (!gesehen.add(id)) {
          throw _ungueltig('Die Info $id kommt mehrfach vor.', {'infoId': id});
        }
        for (final schluessel in element.keys) {
          if (schluessel == 'name') {
            throw _nichtKorrigierbarFehler('infos.$id.name');
          }
          if (!{'id', 'wert', 'einheit'}.contains(schluessel)) {
            throw _ungueltig('Unbekanntes Feld: infos.$id.$schluessel.', {
              'feld': 'infos.$id.$schluessel',
            });
          }
        }
        if (!element.containsKey('wert') && !element.containsKey('einheit')) {
          throw _ungueltig('Die Info $id enthält keine Änderung.', {
            'infoId': id,
          });
        }
        if (!vorhandeneIds.contains(id)) {
          throw ApiFehler(
            httpStatus: 404,
            code: FehlerCode.infoNichtGefunden,
            nachricht: 'Diese Info gibt es in dem Eintrag nicht.',
            details: {'infoId': id},
          );
        }
        if (element.containsKey('wert')) {
          final wert = element['wert'];
          if (wert is! String) {
            throw _ungueltig('Der Wert muss ein Text sein.', {
              'feld': 'infos.$id.wert',
              'wert': wert,
            });
          }
          ergebnis.add(('infos.$id.wert', wert));
        }
        if (element.containsKey('einheit')) {
          final einheit = element['einheit'];
          if (einheit != null && einheit is! String) {
            throw _ungueltig('Die Einheit muss ein Text oder null sein.', {
              'feld': 'infos.$id.einheit',
              'wert': einheit,
            });
          }
          ergebnis.add(('infos.$id.einheit', einheit as String?));
        }
      }
    }
    return ergebnis;
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

    final korrekturen = json['korrekturen'] as List;
    final index = korrekturen.indexWhere((k) => (k as Map)['feld'] == feld);
    // Gibt es noch keine Korrektur, ist der aktuelle Wert der erkannte.
    final erkannt = index >= 0
        ? (korrekturen[index] as Map)['erkannt'] as String?
        : alt;

    _schreibe(json, feld, neu);

    if (neu == erkannt) {
      if (index >= 0) korrekturen.removeAt(index);
    } else if (index >= 0) {
      final korrektur = korrekturen[index] as Map<String, dynamic>;
      korrektur['korrigiert'] = neu;
      korrektur['zeitpunkt'] = zeitpunkt;
    } else {
      korrekturen.add(<String, dynamic>{
        'feld': feld,
        'erkannt': erkannt,
        'korrigiert': neu,
        'zeitpunkt': zeitpunkt,
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
    if (wert == null && feld.startsWith('infos.')) {
      ziel.remove(schluessel);
    } else {
      ziel[schluessel] = wert;
    }
  }

  /// Liefert die Map und den Schlüssel, unter dem ein Feldpfad steht.
  (Map<String, dynamic>, String) _ziel(Map<String, dynamic> json, String feld) {
    if (!feld.startsWith('infos.')) return (json, feld);
    final teile = feld.split('.'); // infos, <id>, wert|einheit
    final info = (json['infos'] as List)
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
    details: {'feld': feld},
  );

  // ---------------------------------------------------------------------
  // POST /fragen
  // ---------------------------------------------------------------------

  static const maxFrageLaenge = 500;

  @override
  Future<FrageAntwort> fragen(String frage) async {
    await _warte(verzoegerung);
    if (frage.trim().isEmpty) {
      throw const ApiFehler(
        httpStatus: 400,
        code: FehlerCode.frageFehlt,
        nachricht: 'Bitte gib eine Frage ein.',
      );
    }
    final laenge = frage.runes.length;
    if (laenge > maxFrageLaenge) {
      throw ApiFehler(
        httpStatus: 400,
        code: FehlerCode.frageZuLang,
        nachricht:
            'Die Frage darf höchstens $maxFrageLaenge Zeichen lang '
            'sein.',
        details: {'maxZeichen': maxFrageLaenge, 'laenge': laenge},
      );
    }
    final woerter = _woerter(frage).where((w) => !_stoppwoerter.contains(w));
    if (woerter.contains('modellfehler')) throw _modellNichtErreichbar;

    // Statt Vektorsuche: einfache Wortsuche in den Zeilen des Volltexts.
    // Eine Zeile zählt pro Fragewort, das in einem ihrer Wörter steckt
    // (oder umgekehrt, damit "Hämoglobinwert" auch "Hämoglobin" findet).
    final suchwoerter = woerter.where((w) => w.length >= 4).toSet();
    final treffer = <(int, Map<String, dynamic>, String)>[];
    for (final json in _eintraege.values) {
      for (final zeile in (json['volltext'] as String).split('\n')) {
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
        'Im Dokument „${typAnzeige(kurz.typ)}“$von$vom steht: '
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
