import 'dart:async';
import 'dart:convert';
import 'dart:typed_data';

import 'package:http/http.dart' as http;
import 'package:http_parser/http_parser.dart';

import '../model/antwort.dart';
import '../model/eintrag.dart';
import '../model/eintrag_aenderung.dart';
import '../model/json_hilfen.dart';
import 'api_client.dart';
import 'api_fehler.dart';
import 'mime_typ.dart';

/// Spricht über HTTP mit dem Go-Backend (docs/API.md).
class HttpApiClient implements ApiClient {
  /// [baseUrl] ohne `/api/v1`, z. B. `http://10.0.2.2:8080`.
  /// [client] lässt sich in Tests durch einen `MockClient` ersetzen.
  HttpApiClient({required String baseUrl, http.Client? client})
    : _basis = '${_ohneSchraegstrich(baseUrl.trim())}/api/v1',
      _client = client ?? http.Client();

  /// Für schnelle Aufrufe (Liste, Eintrag, Korrektur).
  static const standardTimeout = Duration(seconds: 20);

  /// Für Frage (Sprachmodell) und Datei (kann groß sein).
  static const langerTimeout = Duration(seconds: 60);

  /// Upload: Das Backend braucht bis zu ca. 60 s, laut API mindestens 90 s.
  static const uploadTimeout = Duration(seconds: 90);

  final String _basis;
  final http.Client _client;

  static const _jsonHeader = {
    'Content-Type': 'application/json; charset=utf-8',
    'Accept': 'application/json',
  };

  @override
  bool get istMock => false;

  @override
  void schliessen() => _client.close();

  @override
  Future<List<EintragKurz>> eintraege({
    required String modul,
    Sortierung sortierung = Sortierung.dokumentdatum,
    Richtung richtung = Richtung.absteigend,
  }) async {
    final uri = _uri('/eintraege').replace(
      queryParameters: {
        'modul': modul,
        'sortierung': sortierung.name,
        'richtung': richtung.name,
      },
    );
    final antwort = await _senden(
      () => _client.get(uri, headers: _jsonHeader),
      standardTimeout,
    );
    return _lesen(
      antwort,
      (json) => liste(json, 'eintraege', EintragKurz.fromJson),
    );
  }

  @override
  Future<Eintrag> eintrag(String id) async {
    final antwort = await _senden(
      () => _client.get(_uri(_eintragPfad(id)), headers: _jsonHeader),
      standardTimeout,
    );
    return _lesen(antwort, Eintrag.fromJson);
  }

  @override
  Future<DateiInhalt> datei(String id) async {
    final antwort = await _senden(
      () => _client.get(_uri('${_eintragPfad(id)}/datei')),
      langerTimeout,
    );
    final contentType = antwort.headers['content-type'];
    final mimeTyp = contentType == null
        ? 'application/octet-stream'
        : contentType.split(';').first.trim().toLowerCase();
    return DateiInhalt(bytes: antwort.bodyBytes, mimeTyp: mimeTyp);
  }

  @override
  Future<Eintrag> hochladen(
    Uint8List bytes,
    String dateiname, {
    required String modul,
  }) async {
    final anfrage = http.MultipartRequest('POST', _uri('/eintraege'))
      ..headers['Accept'] = 'application/json'
      ..fields['modul'] = modul
      ..files.add(
        http.MultipartFile.fromBytes(
          'datei',
          bytes,
          filename: dateiname,
          contentType: MediaType.parse(
            erkenneMimeTyp(bytes) ?? 'application/octet-stream',
          ),
        ),
      );
    return _formularSenden(anfrage);
  }

  @override
  Future<Eintrag> textImportieren(String text, {required String modul}) {
    // Gleicher Endpunkt wie der Upload, nur Feld `text` statt `datei`.
    // MultipartRequest schickt Formularfelder als UTF-8.
    final anfrage = http.MultipartRequest('POST', _uri('/eintraege'))
      ..headers['Accept'] = 'application/json'
      ..fields['text'] = text
      ..fields['modul'] = modul;
    return _formularSenden(anfrage);
  }

  /// Schickt Upload oder Text-Import. Das Backend braucht bis zu ca. 60 s.
  Future<Eintrag> _formularSenden(http.MultipartRequest anfrage) async {
    final antwort = await _senden(
      () async => http.Response.fromStream(await _client.send(anfrage)),
      uploadTimeout,
    );
    return _lesen(antwort, Eintrag.fromJson);
  }

  @override
  Future<Eintrag> korrigieren(String id, EintragAenderung aenderung) async {
    final antwort = await _senden(
      () => _client.patch(
        _uri(_eintragPfad(id)),
        headers: _jsonHeader,
        body: jsonEncode(aenderung.toJson()),
      ),
      standardTimeout,
    );
    return _lesen(antwort, Eintrag.fromJson);
  }

  @override
  Future<void> loeschen(String id) async {
    // Erfolg ist 204 ohne Body; Fehler kommen wie immer als JSON.
    await _senden(
      () => _client.delete(_uri(_eintragPfad(id)), headers: _jsonHeader),
      standardTimeout,
    );
  }

  @override
  Future<FrageAntwort> fragen(String frage, {required String modul}) async {
    final antwort = await _senden(
      () => _client.post(
        _uri('/fragen'),
        headers: _jsonHeader,
        body: jsonEncode({'frage': frage, 'modul': modul}),
      ),
      langerTimeout,
    );
    return _lesen(antwort, FrageAntwort.fromJson);
  }

  // ---------------------------------------------------------------------

  Uri _uri(String pfad) => Uri.parse('$_basis$pfad');

  String _eintragPfad(String id) => '/eintraege/${Uri.encodeComponent(id)}';

  static String _ohneSchraegstrich(String url) => url.endsWith('/')
      ? _ohneSchraegstrich(url.substring(0, url.length - 1))
      : url;

  /// Führt die Anfrage aus und wandelt Netzwerk- und HTTP-Fehler in
  /// [ApiFehler] um. Gibt nur Antworten mit Status 2xx zurück.
  Future<http.Response> _senden(
    Future<http.Response> Function() anfrage,
    Duration timeout,
  ) async {
    final http.Response antwort;
    try {
      antwort = await anfrage().timeout(timeout);
    } on TimeoutException {
      throw const ApiFehler(
        code: FehlerCode.netzwerkFehler,
        nachricht:
            'Der Server antwortet nicht. '
            'Bitte später noch einmal versuchen.',
      );
    } on http.ClientException {
      throw const ApiFehler(
        code: FehlerCode.netzwerkFehler,
        nachricht:
            'Keine Verbindung zum Server. '
            'Bitte prüfe die Internetverbindung.',
      );
    }
    if (antwort.statusCode >= 200 && antwort.statusCode < 300) return antwort;
    throw _fehlerAus(antwort);
  }

  /// Liest das Fehlerformat `{"fehler": {"code", "nachricht", "details"}}`.
  ApiFehler _fehlerAus(http.Response antwort) {
    try {
      final json = jsonDecode(utf8.decode(antwort.bodyBytes));
      if (json case {'fehler': final Map<String, dynamic> fehler}) {
        if (fehler case {
          'code': final String code,
          'nachricht': final String nachricht,
        }) {
          final details = fehler['details'];
          return ApiFehler(
            httpStatus: antwort.statusCode,
            code: code,
            nachricht: nachricht,
            details: details is Map<String, dynamic> ? details : const {},
          );
        }
      }
    } on FormatException {
      // Kein JSON (z. B. HTML-Fehlerseite eines Proxys): unten behandelt.
    }
    return ApiFehler(
      httpStatus: antwort.statusCode,
      code: FehlerCode.unbekannteAntwort,
      nachricht:
          'Unerwartete Antwort vom Server (Status ${antwort.statusCode}).',
    );
  }

  /// Dekodiert den Body als UTF-8-JSON und wandelt ihn um. Passt die
  /// Antwort nicht zum Vertrag, gibt es [FehlerCode.unbekannteAntwort].
  T _lesen<T>(
    http.Response antwort,
    T Function(Map<String, dynamic>) umwandeln,
  ) {
    try {
      // Immer UTF-8 dekodieren: Fehlt `charset` im Header, würde
      // `antwort.body` Latin-1 annehmen und Umlaute verfälschen.
      final json = jsonDecode(utf8.decode(antwort.bodyBytes));
      return umwandeln(objekt(json, 'Antwort'));
    } on FormatException catch (e) {
      throw ApiFehler(
        httpStatus: antwort.statusCode,
        code: FehlerCode.unbekannteAntwort,
        nachricht: 'Die Antwort des Servers ist unvollständig.',
        details: {'grund': e.message},
      );
    }
  }
}
