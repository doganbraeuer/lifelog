import 'package:flutter_test/flutter_test.dart';
import 'package:lifelog/api/api_exception.dart';
import 'package:lifelog/api/mock_api_client.dart';
import 'package:lifelog/model/plain_date.dart';
import 'package:lifelog/model/entry_patch.dart';

import '../helpers.dart';

void main() {
  test('Frage mit Treffer: Antwort mit [1] und Quelle', () async {
    final antwort = await schnellerMock().fragen(
      'Welchen Hämoglobinwert hatte ich im Oktober?',
      modul: 'health',
    );

    expect(antwort.gefunden, isTrue);
    expect(antwort.antwort, contains('[1]'));
    expect(antwort.quellen, hasLength(1));
    final quelle = antwort.quellen.single;
    expect(quelle.nr, 1);
    expect(quelle.eintragId, laborId);
    expect(quelle.typ, 'lab_report');
    expect(quelle.quelle, 'Labor Muster GmbH, Salzburg');
    expect(quelle.dokumentdatum, Datum(2026, 10, 14));
    expect(quelle.auszug, 'Hämoglobin 14,2 g/dl (Ref. 13,5-17,5)');
  });

  test('Quellen zeigen korrigierte Werte', () async {
    final mock = schnellerMock();
    await mock.korrigieren(
      laborId,
      EintragAenderung(dokumentdatum: Wert(Datum(2026, 10, 15))),
    );
    final antwort = await mock.fragen('Hämoglobin', modul: 'health');
    expect(antwort.quellen.single.dokumentdatum, Datum(2026, 10, 15));
  });

  test('ohne Treffer: fester Satz, keine Quellen', () async {
    final antwort = await schnellerMock().fragen(
      'Wie war das Wetter in Paris?',
      modul: 'health',
    );
    expect(antwort.gefunden, isFalse);
    expect(antwort.antwort, 'Dazu finde ich nichts in deinen Dokumenten.');
    expect(antwort.antwort, keinTrefferAntwort);
    expect(antwort.quellen, isEmpty);
  });

  test('leere Frage: 400 QUESTION_MISSING', () async {
    for (final frage in ['', '   ', '\n\t']) {
      await expectLater(
        schnellerMock().fragen(frage, modul: 'health'),
        wirftApiFehler(FehlerCode.frageFehlt, status: 400),
      );
    }
  });

  test('über 500 Zeichen: 400 QUESTION_TOO_LONG, genau 500 ist erlaubt', () async {
    await expectLater(
      schnellerMock().fragen('a' * 501, modul: 'health'),
      wirftApiFehler(FehlerCode.frageZuLang, status: 400),
    );
    final antwort = await schnellerMock().fragen(
      'a' * 500,
      modul: 'health',
    );
    expect(antwort.gefunden, isFalse);
  });

  test('"modellfehler": 503 MODEL_UNAVAILABLE', () async {
    await expectLater(
      schnellerMock().fragen('Test modellfehler', modul: 'health'),
      wirftApiFehler(FehlerCode.modellNichtErreichbar, status: 503),
    );
  });
}
