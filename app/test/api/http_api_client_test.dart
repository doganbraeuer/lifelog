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
      );

      expect(anfrage.method, 'GET');
      expect(
        anfrage.url.toString(),
        '$basis/api/v1/eintraege?sortierung=importdatum&richtung=aufsteigend',
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
      expect(await api.eintraege(), isEmpty);
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
        await api.hochladen(Uint8List.fromList('%PDF-'.codeUnits), 'a.pdf');
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
    final eintrag = await api.hochladen(bytes, 'befund.pdf');

    expect(anfrage.method, 'POST');
    expect(anfrage.url.toString(), '$basis/api/v1/eintraege');
    expect(anfrage.headers['content-type'], startsWith('multipart/form-data'));
    final body = latin1.decode(anfrage.bodyBytes).toLowerCase();
    expect(body, contains('name="datei"; filename="befund.pdf"'));
    expect(body, contains('content-type: application/pdf'));
    expect(body, contains('%pdf-1.4 inhalt'));
    expect(eintrag.id, laborId);
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
            'nr': 1,
            'eintragId': laborId,
            'typ': 'laborbefund',
            'quelle': 'Labor Muster GmbH, Salzburg',
            'dokumentdatum': '2026-10-14',
            'auszug': 'Hämoglobin 14,2 g/dl (Ref. 13,5-17,5)',
          },
        ],
      });
    });
    final antwort = await api.fragen('Hämoglobin?');

    expect(anfrage.method, 'POST');
    expect(anfrage.url.path, '/api/v1/fragen');
    expect(jsonDecode(anfrage.body), {'frage': 'Hämoglobin?'});
    expect(antwort.gefunden, isTrue);
    expect(antwort.quellen.single.auszug, startsWith('Hämoglobin'));
    expect(antwort.quellen.single.dokumentdatum, Datum(2026, 10, 14));
  });
}
