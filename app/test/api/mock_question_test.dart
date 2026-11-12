import 'package:flutter_test/flutter_test.dart';
import 'package:lifelog/api/api_exception.dart';
import 'package:lifelog/api/mock_api_client.dart';
import 'package:lifelog/model/plain_date.dart';
import 'package:lifelog/model/entry_patch.dart';

import '../helpers.dart';

void main() {
  test('Frage mit Treffer: Antwort mit [1] und Quelle', () async {
    final answer = await schnellerMock().ask(
      'Welchen Hämoglobinwert hatte ich im Oktober?',
      module: 'health',
    );

    expect(answer.found, isTrue);
    expect(answer.answer, contains('[1]'));
    expect(answer.sources, hasLength(1));
    final source = answer.sources.single;
    expect(source.number, 1);
    expect(source.entryId, laborId);
    expect(source.type, 'lab_report');
    expect(source.issuer, 'Labor Muster GmbH, Salzburg');
    expect(source.documentDate, PlainDate(2026, 10, 14));
    expect(source.excerpt, 'Hämoglobin 14,2 g/dl (Ref. 13,5-17,5)');
  });

  test('Quellen zeigen korrigierte Werte', () async {
    final mock = schnellerMock();
    await mock.correct(
      laborId,
      EntryPatch(documentDate: Value(PlainDate(2026, 10, 15))),
    );
    final answer = await mock.ask('Hämoglobin', module: 'health');
    expect(answer.sources.single.documentDate, PlainDate(2026, 10, 15));
  });

  test('ohne Treffer: fester Satz, keine Quellen', () async {
    final answer = await schnellerMock().ask(
      'Wie war das Wetter in Paris?',
      module: 'health',
    );
    expect(answer.found, isFalse);
    expect(answer.answer, 'Dazu finde ich nichts in deinen Dokumenten.');
    expect(answer.answer, notFoundAnswer);
    expect(answer.sources, isEmpty);
  });

  test('leere Frage: 400 QUESTION_MISSING', () async {
    for (final question in ['', '   ', '\n\t']) {
      await expectLater(
        schnellerMock().ask(question, module: 'health'),
        wirftApiFehler(ErrorCode.questionMissing, status: 400),
      );
    }
  });

  test(
    'über 500 Zeichen: 400 QUESTION_TOO_LONG, genau 500 ist erlaubt',
    () async {
      await expectLater(
        schnellerMock().ask('a' * 501, module: 'health'),
        wirftApiFehler(ErrorCode.questionTooLong, status: 400),
      );
      final answer = await schnellerMock().ask('a' * 500, module: 'health');
      expect(answer.found, isFalse);
    },
  );

  test('"modelerror": 503 MODEL_UNAVAILABLE', () async {
    await expectLater(
      schnellerMock().ask('Test modelerror', module: 'health'),
      wirftApiFehler(ErrorCode.modelUnavailable, status: 503),
    );
  });
}
