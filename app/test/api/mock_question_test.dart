import 'package:flutter_test/flutter_test.dart';
import 'package:lifelog/api/api_exception.dart';
import 'package:lifelog/api/mock_api_client.dart';
import 'package:lifelog/model/plain_date.dart';
import 'package:lifelog/model/entry_patch.dart';

import '../helpers.dart';

void main() {
  test('question with a hit: answer with [1] and source', () async {
    final answer = await fastMock().ask(
      'Welchen Hämoglobinwert hatte ich im Oktober?',
      module: 'health',
    );

    expect(answer.found, isTrue);
    expect(answer.answer, contains('[1]'));
    expect(answer.sources, hasLength(1));
    final source = answer.sources.single;
    expect(source.number, 1);
    expect(source.entryId, labReportId);
    expect(source.type, 'lab_report');
    expect(source.issuer, 'Labor Muster GmbH, Salzburg');
    expect(source.documentDate, PlainDate(2026, 10, 14));
    expect(source.excerpt, 'Hämoglobin 14,2 g/dl (Ref. 13,5-17,5)');
  });

  test('sources show corrected values', () async {
    final mock = fastMock();
    await mock.correct(
      labReportId,
      EntryPatch(documentDate: Value(PlainDate(2026, 10, 15))),
    );
    final answer = await mock.ask('Hämoglobin', module: 'health');
    expect(answer.sources.single.documentDate, PlainDate(2026, 10, 15));
  });

  test('without a hit: fixed sentence, no sources', () async {
    final answer = await fastMock().ask(
      'Wie war das Wetter in Paris?',
      module: 'health',
    );
    expect(answer.found, isFalse);
    expect(answer.answer, 'Dazu finde ich nichts in deinen Dokumenten.');
    expect(answer.answer, notFoundAnswer);
    expect(answer.sources, isEmpty);
  });

  test('empty question: 400 QUESTION_MISSING', () async {
    for (final question in ['', '   ', '\n\t']) {
      await expectLater(
        fastMock().ask(question, module: 'health'),
        throwsApiException(ErrorCode.questionMissing, status: 400),
      );
    }
  });

  test(
    'over 500 characters: 400 QUESTION_TOO_LONG, exactly 500 is allowed',
    () async {
      await expectLater(
        fastMock().ask('a' * 501, module: 'health'),
        throwsApiException(ErrorCode.questionTooLong, status: 400),
      );
      final answer = await fastMock().ask('a' * 500, module: 'health');
      expect(answer.found, isFalse);
    },
  );

  test('"modelerror": 503 MODEL_UNAVAILABLE', () async {
    await expectLater(
      fastMock().ask('Test modelerror', module: 'health'),
      throwsApiException(ErrorCode.modelUnavailable, status: 503),
    );
  });
}
