import 'dart:convert';
import 'dart:typed_data';

import 'package:flutter_test/flutter_test.dart';
import 'package:http/http.dart' as http;
import 'package:http/testing.dart';
import 'package:lifelog/api/api_client.dart';
import 'package:lifelog/api/api_fehler.dart';
import 'package:lifelog/api/http_api_client.dart';
import 'package:lifelog/model/datum.dart';
import 'package:lifelog/model/eintrag_aenderung.dart';

import '../hilfen.dart';
import '../model/eintrag_test.dart' show beispielJson;

const basis = 'http://test.local:8080';

/// Antwort mit JSON-Body als UTF-8, aber bewusst ohne `charset` im Header.
http.Response jsonAntwort(Object json, [int status = 200]) =>
    http.Response.bytes(
      utf8.encode(jsonEncode(json)),
      status,
      headers: {'content-type': 'application/json'},
    );

HttpApiClient client(MockClientHandler handler, {String baseUrl = basis}) =>
    HttpApiClient(baseUrl: baseUrl, client: MockClient(handler));

void main() {
  group('eintraege', () {
    test('URL mit Query-Parametern und Antwort wie in 3.2', () async {
      late http.Request anfrage;
      final api = client((request) async {
        anfrage = request;
        return jsonAntwort({
          'eintraege': [
            {
              'id': laborId,
              'modul': 'gesundheit',
              'typ': 'laborbefund',
              'dokumentdatum': '2026-10-14',
              'importdatum': '2026-11-03T18:42:07Z',
              'quelle': 'Labor Muster GmbH, Salzburg',
            },
            {
              'id': sonstigesId,
              'modul': 'gesundheit',
              'typ': 'sonstiges',
              'dokumentdatum': null,
              'importdatum': '2026-11-03T19:01:44Z',
              'quelle': null,
            },
          ],
          'anzahl': 2,
        });
      });

      final liste = await api.eintraege(
        sortierung: Sortierung.importdatum,
        richtung: Richtung.aufsteigend,
        modul: 'gesundheit',
      );

      expect(anfrage.method, 'GET');
      expect(
        anfrage.url.toString(),
        '$basis/api/v1/eintraege'
        '?modul=gesundheit&sortierung=importdatum&richtung=aufsteigend',
      );
      expect(liste, hasLength(2));
      expect(liste.first.dokumentdatum, Datum(2026, 10, 14));
      expect(liste.last.dokumentdatum, isNull);
    });

    test('Schrägstrich am Ende der Basis-URL stört nicht', () async {
      late Uri url;
      final api = client(baseUrl: '$basis/', (request) async {
        url = request.url;
        return jsonAntwort({'eintraege': [], 'anzahl': 0});
      });
      expect(await api.eintraege(modul: 'gesundheit'), isEmpty);
      expect(url.path, '/api/v1/eintraege');
    });
  });

  group('eintrag', () {
    test('Pfad und Umlaute, auch ohne charset im Header', () async {
      late Uri url;
      final api = client((request) async {
        url = request.url;
        return http.Response.bytes(
          utf8.encode(beispielJson),
          200,
          headers: {'content-type': 'application/json'},
        );
      });
      final eintrag = await api.eintrag(laborId);
      expect(url.toString(), '$basis/api/v1/eintraege/$laborId');
      expect(eintrag.infos.first.name, 'Hämoglobin');
    });

    test('Fehler-JSON wird zu ApiFehler', () async {
      final api = client(
        (_) async => jsonAntwort({
          'fehler': {
            'code': 'EINTRAG_NICHT_GEFUNDEN',
            'nachricht': 'Diesen Eintrag gibt es nicht.',
            'details': {},
          },
        }, 404),
      );
      try {
        await api.eintrag('ffffffffffffffffffffffff');
        fail('Fehler erwartet');
      } on ApiFehler catch (f) {
        expect(f.httpStatus, 404);
        expect(f.code, FehlerCode.eintragNichtGefunden);
        expect(f.nachricht, 'Diesen Eintrag gibt es nicht.');
      }
    });

    test('details werden übernommen', () async {
      final api = client(
        (_) async => jsonAntwort({
          'fehler': {
            'code': 'DUPLIKAT',
            'nachricht': 'Diese Datei hast du schon hochgeladen.',
            'details': {'eintragId': laborId},
          },
        }, 409),
      );
      try {
        await api.hochladen(
          Uint8List.fromList('%PDF-'.codeUnits),
          'a.pdf',
          modul: 'gesundheit',
        );
        fail('Fehler erwartet');
      } on ApiFehler catch (f) {
        expect(f.details['eintragId'], laborId);
      }
    });

    test(
      'Fehler ohne JSON (z. B. HTML vom Proxy): UNBEKANNTE_ANTWORT',
      () async {
        final api = client(
          (_) async => http.Response('<html>Bad Gateway</html>', 502),
        );
        await expectLater(
          api.eintrag(laborId),
          wirftApiFehler(FehlerCode.unbekannteAntwort, status: 502),
        );
      },
    );

    test('Erfolg mit unpassendem JSON: UNBEKANNTE_ANTWORT', () async {
      final api = client((_) async => jsonAntwort({'foo': 1}));
      await expectLater(
        api.eintrag(laborId),
        wirftApiFehler(FehlerCode.unbekannteAntwort, status: 200),
      );
    });

    test('keine Verbindung: NETZWERK_FEHLER', () async {
      final api = client((_) async => throw http.ClientException('offline'));
      await expectLater(
        api.eintrag(laborId),
        wirftApiFehler(FehlerCode.netzwerkFehler),
      );
    });
  });

  test('datei liefert Bytes und Typ ohne Parameter', () async {
    late Uri url;
    final api = client((request) async {
      url = request.url;
      return http.Response.bytes(
        [1, 2, 3],
        200,
        headers: {'content-type': 'application/pdf; name=x'},
      );
    });
    final datei = await api.datei(laborId);
    expect(url.path, '/api/v1/eintraege/$laborId/datei');
    expect(datei.bytes, [1, 2, 3]);
    expect(datei.mimeTyp, 'application/pdf');
    expect(datei.istPdf, isTrue);
  });

  test('hochladen schickt multipart mit Feld "datei"', () async {
    late http.Request anfrage;
    final api = client((request) async {
      anfrage = request;
      return http.Response.bytes(
        utf8.encode(beispielJson),
        201,
        headers: {'content-type': 'application/json'},
      );
    });
    final bytes = Uint8List.fromList(utf8.encode('%PDF-1.4 Inhalt'));
    final eintrag = await api.hochladen(
      bytes,
      'befund.pdf',
      modul: 'gesundheit',
    );

    expect(anfrage.method, 'POST');
    expect(anfrage.url.toString(), '$basis/api/v1/eintraege');
    expect(anfrage.headers['content-type'], startsWith('multipart/form-data'));
    final body = latin1.decode(anfrage.bodyBytes).toLowerCase();
    expect(body, contains('name="datei"; filename="befund.pdf"'));
    expect(body, contains('content-type: application/pdf'));
    expect(body, contains('%pdf-1.4 inhalt'));
    expect(body, contains('name="modul"\r\n\r\ngesundheit'));
    expect(body, isNot(contains('name="text"')));
    expect(eintrag.id, laborId);
  });

  test('textImportieren schickt multipart mit "text" und "modul"', () async {
    late http.Request anfrage;
    final api = client((request) async {
      anfrage = request;
      return http.Response.bytes(
        utf8.encode(beispielJson),
        201,
        headers: {'content-type': 'application/json'},
      );
    });
    await api.textImportieren(
      'Überweisung vom 21.10.2026',
      modul: 'gesundheit',
    );

    expect(anfrage.method, 'POST');
    expect(anfrage.url.toString(), '$basis/api/v1/eintraege');
    expect(anfrage.headers['content-type'], startsWith('multipart/form-data'));
    // Formularfelder als UTF-8, damit Umlaute ankommen. Bei Nicht-ASCII
    // schreibt das http-Paket einen eigenen Teil-Header mit charset dazu.
    final body = utf8.decode(anfrage.bodyBytes);
    expect(
      body,
      contains(
        'name="text"\r\n'
        'content-type: text/plain; charset=utf-8\r\n'
        'content-transfer-encoding: binary\r\n'
        '\r\n'
        'Überweisung vom 21.10.2026\r\n',
      ),
    );
    expect(body, contains('name="modul"\r\n\r\ngesundheit'));
    expect(body, isNot(contains('name="datei"')));
  });

  group('loeschen', () {
    test('schickt DELETE, 204 ohne Body ist Erfolg', () async {
      late http.Request anfrage;
      final api = client((request) async {
        anfrage = request;
        return http.Response('', 204);
      });
      await api.loeschen(laborId);

      expect(anfrage.method, 'DELETE');
      expect(anfrage.url.path, '/api/v1/eintraege/$laborId');
    });

    test('404 wird zu ApiFehler', () async {
      final api = client(
        (_) async => jsonAntwort({
          'fehler': {
            'code': 'EINTRAG_NICHT_GEFUNDEN',
            'nachricht': 'Diesen Eintrag gibt es nicht.',
          },
        }, 404),
      );
      await expectLater(
        api.loeschen(laborId),
        wirftApiFehler(FehlerCode.eintragNichtGefunden, status: 404),
      );
    });
  });

  test('Liste null statt [] passt nicht zum Vertrag', () async {
    // docs/API.md: Listen sind immer ein Array, nie null.
    final api = client(
      (_) async => jsonAntwort({'eintraege': null, 'anzahl': 0}),
    );
    await expectLater(
      api.eintraege(modul: 'gesundheit'),
      wirftApiFehler(FehlerCode.unbekannteAntwort, status: 200),
    );
  });

  test('korrigieren schickt PATCH mit null für gelöschte Felder', () async {
    late http.Request anfrage;
    final api = client((request) async {
      anfrage = request;
      return http.Response.bytes(
        utf8.encode(beispielJson),
        200,
        headers: {'content-type': 'application/json'},
      );
    });
    await api.korrigieren(
      laborId,
      const EintragAenderung(
        dokumentdatum: Wert(null),
        infos: [InfoAenderung('i1', einheit: Wert(null))],
      ),
    );

    expect(anfrage.method, 'PATCH');
    expect(anfrage.url.path, '/api/v1/eintraege/$laborId');
    expect(anfrage.headers['content-type'], startsWith('application/json'));
    expect(jsonDecode(anfrage.body), {
      'dokumentdatum': null,
      'infos': [
        {'id': 'i1', 'einheit': null},
      ],
    });
  });

  test('fragen schickt POST und liest Antwort mit Quellen', () async {
    late http.Request anfrage;
    final api = client((request) async {
      anfrage = request;
      return jsonAntwort({
        'antwort': 'Im Laborbefund steht: Hämoglobin 14,2 g/dl [1].',
        'gefunden': true,
        'quellen': [
          {
            'nr': 3,
            'eintragId': laborId,
            'modul': 'gesundheit',
            'typ': 'laborbefund',
            'quelle': 'Labor Muster GmbH, Salzburg',
            'dokumentdatum': '2026-10-14',
            'auszug': 'Hämoglobin 14,2 g/dl (Ref. 13,5-17,5)',
          },
        ],
      });
    });
    final antwort = await api.fragen('Hämoglobin?', modul: 'gesundheit');

    expect(anfrage.method, 'POST');
    expect(anfrage.url.path, '/api/v1/fragen');
    expect(jsonDecode(anfrage.body), {
      'frage': 'Hämoglobin?',
      'modul': 'gesundheit',
    });
    expect(antwort.gefunden, isTrue);
    final quelle = antwort.quellen.single;
    // nr muss nicht bei 1 beginnen (nur zitierte Abschnitte, API.md 3.6).
    expect(quelle.nr, 3);
    expect(quelle.modul, 'gesundheit');
    expect(quelle.auszug, startsWith('Hämoglobin'));
    expect(quelle.dokumentdatum, Datum(2026, 10, 14));
  });

  test('Quelle ohne modul passt nicht zum Vertrag', () async {
    final api = client(
      (_) async => jsonAntwort({
        'antwort': 'x [1].',
        'gefunden': true,
        'quellen': [
          {
            'nr': 1,
            'eintragId': laborId,
            'typ': 'laborbefund',
            'quelle': null,
            'dokumentdatum': null,
            'auszug': 'x',
          },
        ],
      }),
    );
    await expectLater(
      api.fragen('x', modul: 'gesundheit'),
      wirftApiFehler(FehlerCode.unbekannteAntwort),
    );
  });
}
