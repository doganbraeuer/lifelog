import 'dart:convert';

import 'package:flutter/gestures.dart';
import 'package:flutter/material.dart';
import 'package:flutter/semantics.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:http/http.dart' as http;
import 'package:http/testing.dart';
import 'package:lifelog/api/http_api_client.dart';
import 'package:lifelog/module/module_catalog.dart';
import 'package:lifelog/screens/original_location_screen.dart';
import 'package:lifelog/screens/question_screen.dart';

import '../helpers.dart';

/// Full text of the lab report as a phone might send it: `\r\n`, a line
/// with spaces around it, and a "blank" line that contains a space.
const labText =
    'Labor Muster GmbH\r\nMusterstraße 1, 5020 Salzburg\r\n'
    'Laborbefund vom 14.10.2026\r\n\r\n'
    '  Hämoglobin 14,2 g/dl (Ref. 13,5-17,5)  \r\n'
    'Leukozyten 6,8 G/l (Ref. 4,0-10,0)\n \n'
    'CRP <0,5 mg/dl (Ref. <0,5)\n'
    'Thrombozyten 245 G/l (Ref. 150-400)\n'
    'Material: Serum, Abnahme nüchtern um 08:15 Uhr\n'
    'Befund vidiert: Dr. Max Muster, Facharzt für Labormedizin';

/// A long lab report, so that the box has to scroll to the passage at the
/// end.
final longLabText = [
  'Labor Muster GmbH',
  for (var i = 1; i <= 30; i++) 'Parameter $i: unauffällig',
  'Vitamin D 32 ng/ml (Ref. 30-100)',
].join('\n');

const doctorLetterText =
    'Dr. Erika Beispiel\n\nArztbrief vom 02.09.2026\n'
    'Blutdruck 128/82 mmHg\nPuls 72/min';

/// Answer of the backend with the sources 1, 3 and 4 (numbers with a gap,
/// as docs/API.md 3.6 allows). 1 and 4 come from the same entry; 4 is a
/// chunk whose whitespace differs from the full text, as
/// `core.SplitIntoChunks` builds it. [7] has no source.
Map<String, dynamic> answerWithGap() => {
  'answer':
      'Hämoglobin 14,2 g/dl [1], Blutdruck 128/82 mmHg [3], '
      'Leukozyten 6,8 G/l und CRP <0,5 mg/dl [1, 4]. '
      'Mehr steht dazu nicht [7].',
  'found': true,
  'sources': [
    source(1, labReportId, 'Hämoglobin 14,2 g/dl (Ref. 13,5-17,5)'),
    {
      ...source(3, doctorLetterId, 'Blutdruck 128/82 mmHg'),
      'type': 'doctor_letter',
      'issuer': 'Dr. Erika Beispiel',
      'documentDate': '2026-09-02',
    },
    source(
      4,
      labReportId,
      'Leukozyten 6,8 G/l (Ref. 4,0-10,0)\n\nCRP <0,5 mg/dl (Ref. <0,5)',
    ),
  ],
};

Map<String, dynamic> source(int number, String entryId, String excerpt) => {
  'number': number,
  'entryId': entryId,
  'module': 'health',
  'type': 'lab_report',
  'issuer': 'Labor Muster GmbH, Salzburg',
  'documentDate': '2026-10-14',
  'excerpt': excerpt,
};

Map<String, dynamic> entryJson(String id, String fullText) => {
  'id': id,
  'module': 'health',
  'type': 'lab_report',
  'documentDate': '2026-10-14',
  'importedAt': '2026-11-03T18:42:07Z',
  'issuer': 'Labor Muster GmbH, Salzburg',
  'fullText': fullText,
  'facts': <Object>[],
  'file': {
    'id': '6727a1f3c2b4e5d6f7a8b9c1',
    'fileName': 'befund.pdf',
    'mimeType': 'application/pdf',
    'size': 1000,
    'sha256': '0' * 64,
  },
  'corrections': <Object>[],
};

/// Fake backend: answers the question with [answerWithGap] and the entries
/// with their full texts. Counts how often each entry is loaded.
class FakeBackend {
  final loads = <String, int>{};

  /// While `true`, loading an entry fails with a server error.
  var failEntries = false;

  /// Full texts by entry ID.
  final texts = {labReportId: labText, doctorLetterId: doctorLetterText};

  /// The answer to the question.
  var answer = answerWithGap();

  http.Response _json(Object body, [int status = 200]) => http.Response.bytes(
    utf8.encode(jsonEncode(body)),
    status,
    headers: {'content-type': 'application/json'},
  );

  Future<http.Response> handle(http.Request request) async {
    final path = request.url.path;
    if (path.endsWith('/questions')) return _json(answer);
    final id = path.split('/').last;
    loads[id] = (loads[id] ?? 0) + 1;
    if (failEntries) {
      return _json({
        'error': {
          'code': 'INTERNAL_ERROR',
          'message': 'Interner Fehler.',
          'details': <String, Object>{},
        },
      }, 500);
    }
    final text = texts[id];
    if (text == null) {
      // Deleted meanwhile, answered like the backend.
      return _json({
        'error': {
          'code': 'ENTRY_NOT_FOUND',
          'message': 'Der Eintrag wurde nicht gefunden.',
          'details': <String, Object>{},
        },
      }, 404);
    }
    return _json(entryJson(id, text));
  }
}

/// The question page alone, with [backend]. Opened entries end up in
/// [opened].
Future<void> showAnswer(
  WidgetTester tester,
  List<String> opened, [
  FakeBackend? backend,
]) async {
  useLargeScreen(tester);
  final fake = backend ?? FakeBackend();
  final api = HttpApiClient(
    baseUrl: 'http://test.local',
    client: MockClient(fake.handle),
  );
  await tester.pumpWidget(
    ModuleCatalogScope(
      catalog: ModuleCatalog.defaults(),
      child: MaterialApp(
        home: QuestionScreen(
          api: api,
          moduleId: 'health',
          openEntry: (id) async => opened.add(id),
        ),
      ),
    ),
  );
  await tester.enterText(find.byType(TextField), 'Werte');
  await tester.pump();
  await tester.tap(find.byTooltip('Frage senden'));
  await tester.pumpAndSettle();
}

/// The card with the number [number] in its avatar.
Finder cardOf(int number) => find.ancestor(
  of: find.descendant(
    of: find.byType(CircleAvatar),
    matching: find.text('$number'),
  ),
  matching: find.byType(Card),
);

/// The arrow button of card [number].
Finder arrowOf(int number) =>
    find.descendant(of: cardOf(number), matching: find.byType(IconButton));

/// The full text view of card [number].
Finder textOf(int number) => find.descendant(
  of: cardOf(number),
  matching: find.byType(TextLocationView),
);

/// The highlighted spans of a full text view: (text, is the current one).
List<(String, bool)> marks(WidgetTester tester, Finder view) {
  final text = tester.widget<SelectableText>(
    find.descendant(of: view, matching: find.byType(SelectableText)),
  );
  final current = Theme.of(tester.element(view)).colorScheme.primaryContainer;
  return [
    for (final span in text.textSpan!.children!.cast<TextSpan>())
      if (span.style?.backgroundColor != null)
        (span.text!, span.style!.backgroundColor == current),
  ];
}

/// The span of the answer text that shows [text].
TextSpan answerSpan(WidgetTester tester, String text) {
  final answer = tester.widget<Text>(
    find.descendant(
      of: find.byType(SelectionArea),
      matching: find.byType(Text),
    ),
  );
  return (answer.textSpan! as TextSpan).children!.cast<TextSpan>().singleWhere(
    (s) => s.text == text,
  );
}

void main() {
  testWidgets('numbers with a gap stay as the backend sent them', (
    tester,
  ) async {
    final opened = <String>[];
    await showAnswer(tester, opened);

    for (final number in [1, 3, 4]) {
      expect(cardOf(number), findsOneWidget);
    }
    expect(cardOf(2), findsNothing);

    // Expanding source 3 shows only its entry.
    await tester.tap(arrowOf(3));
    await tester.pumpAndSettle();
    expect(textOf(3), findsOneWidget);
    expect(marks(tester, textOf(3)), [('Blutdruck 128/82 mmHg', true)]);
    expect(textOf(1), findsNothing);
    expect(opened, isEmpty);

    // Opening source 3 opens its entry, not the one of source 1.
    await tester.tap(find.text('Arztbrief · 02.09.2026'));
    await tester.pumpAndSettle();
    expect(opened, [doctorLetterId]);
  });

  testWidgets('expanded: the full text, the matching lines of the entry '
      'marked, its own ones strongly', (tester) async {
    final backend = FakeBackend();
    await showAnswer(tester, [], backend);
    expect(backend.loads, isEmpty, reason: 'loaded only when expanded');

    await tester.tap(arrowOf(1));
    await tester.pumpAndSettle();
    final view = tester.widget<TextLocationView>(textOf(1));
    expect(view.fullText, labText);
    // Line by line, trimmed. The chunk of source 4 has other whitespace
    // than the full text and is still found.
    expect(marks(tester, textOf(1)), [
      ('Hämoglobin 14,2 g/dl (Ref. 13,5-17,5)', true),
      ('Leukozyten 6,8 G/l (Ref. 4,0-10,0)', false),
      ('CRP <0,5 mg/dl (Ref. <0,5)', false),
    ]);

    // Source 4 from the same entry: the same lines, its own ones strong.
    await tester.tap(arrowOf(4));
    await tester.pumpAndSettle();
    expect(marks(tester, textOf(4)), [
      ('Hämoglobin 14,2 g/dl (Ref. 13,5-17,5)', false),
      ('Leukozyten 6,8 G/l (Ref. 4,0-10,0)', true),
      ('CRP <0,5 mg/dl (Ref. <0,5)', true),
    ]);

    // Loaded once for both cards; collapsing and expanding loads nothing.
    await tester.tap(arrowOf(1));
    await tester.pumpAndSettle();
    await tester.tap(arrowOf(1));
    await tester.pumpAndSettle();
    expect(backend.loads, {labReportId: 1});
  });

  testWidgets('a general answer: nothing marked, the text is still there, '
      'scrolled to the passage', (tester) async {
    const excerpt = 'Vitamin D 32 ng/ml (Ref. 30-100)';
    final backend = FakeBackend()
      ..texts[labReportId] = longLabText
      ..answer = {
        'answer': 'In deinen Befunden stehen Laborwerte [1].',
        'found': true,
        'sources': [source(1, labReportId, excerpt)],
      };
    await showAnswer(tester, [], backend);

    await tester.tap(arrowOf(1));
    await tester.pumpAndSettle();
    expect(textOf(1), findsOneWidget);
    expect(marks(tester, textOf(1)), isEmpty);
    final view = tester.widget<TextLocationView>(textOf(1));
    expect(view.current.textIn(longLabText), excerpt);
    final scroll = tester.state<ScrollableState>(
      find.descendant(of: textOf(1), matching: find.byType(Scrollable)).first,
    );
    expect(scroll.position.pixels, greaterThan(0));
  });

  testWidgets('place not found: only the excerpt, as before', (tester) async {
    final backend = FakeBackend()
      ..texts[doctorLetterId] = 'Ein ganz anderer Text ohne den Wert.';
    await showAnswer(tester, [], backend);

    await tester.tap(arrowOf(3));
    await tester.pumpAndSettle();
    expect(textOf(3), findsNothing);
    expect(find.text('„Blutdruck 128/82 mmHg“'), findsOneWidget);
    expect(find.textContaining('nicht geladen'), findsNothing);
  });

  testWidgets('load error: calm note with the excerpt, retry loads again', (
    tester,
  ) async {
    final backend = FakeBackend()..failEntries = true;
    await showAnswer(tester, [], backend);

    await tester.tap(arrowOf(3));
    await tester.pumpAndSettle();
    expect(
      find.text('Der ganze Text kann gerade nicht geladen werden.'),
      findsOneWidget,
    );
    expect(find.text('„Blutdruck 128/82 mmHg“'), findsOneWidget);
    // Not an error card: the answer stays the main thing.
    expect(find.text('Interner Fehler.'), findsNothing);

    backend.failEntries = false;
    await tester.tap(find.text('Erneut versuchen'));
    await tester.pumpAndSettle();
    expect(textOf(3), findsOneWidget);
    expect(backend.loads, {doctorLetterId: 2});
  });

  testWidgets('a deleted entry: no retry, the excerpt stays', (tester) async {
    final backend = FakeBackend();
    await showAnswer(tester, [], backend);
    backend.texts.remove(doctorLetterId);

    await tester.tap(arrowOf(3));
    await tester.pumpAndSettle();
    expect(find.text('Der Eintrag ist nicht mehr vorhanden.'), findsOneWidget);
    expect(find.text('Erneut versuchen'), findsNothing);
    expect(find.text('„Blutdruck 128/82 mmHg“'), findsOneWidget);
  });

  testWidgets('references with a source are links, [7] stays plain text', (
    tester,
  ) async {
    await showAnswer(tester, []);

    expect(answerSpan(tester, '[1]').recognizer, isNotNull);
    expect(answerSpan(tester, '[3]').recognizer, isNotNull);
    // In a list each number is a link of its own.
    expect(answerSpan(tester, '1').recognizer, isNotNull);
    expect(answerSpan(tester, '4').recognizer, isNotNull);
    // No source 7: plain text, part of the surrounding span.
    final answer = tester.widget<Text>(
      find.descendant(
        of: find.byType(SelectionArea),
        matching: find.byType(Text),
      ),
    );
    final last = (answer.textSpan! as TextSpan).children!.cast<TextSpan>().last;
    expect(last.text, endsWith('Mehr steht dazu nicht [7].'));
    expect(last.recognizer, isNull);
    // The copied text is the answer unchanged; only the screen reader reads
    // „Quelle 1 anzeigen“.
    expect(
      answer.textSpan!.toPlainText(includeSemanticsLabels: false),
      answerWithGap()['answer'] as String,
    );
  });

  testWidgets('tapping [3] expands source 3 and highlights it briefly', (
    tester,
  ) async {
    await showAnswer(tester, []);
    expect(textOf(3), findsNothing);

    final colors = Theme.of(tester.element(cardOf(3))).colorScheme;
    Color? cardColor() => tester.widget<Card>(cardOf(3)).color;

    (answerSpan(tester, '[3]').recognizer! as TapGestureRecognizer).onTap!();
    await tester.pump();
    await tester.pump(const Duration(milliseconds: 200));
    expect(cardColor(), colors.secondaryContainer);

    await tester.pumpAndSettle();
    expect(textOf(3), findsOneWidget);
    expect(textOf(1), findsNothing);
    // Still highlighted, then it fades back.
    expect(cardColor(), colors.secondaryContainer);
    await tester.pump(Durations.extralong4);
    await tester.pumpAndSettle();
    expect(cardColor(), isNot(colors.secondaryContainer));
  });

  testWidgets('tapping a reference scrolls to its card', (tester) async {
    // A small screen, so that the cards are below the answer.
    tester.view.physicalSize = const Size(400, 500);
    tester.view.devicePixelRatio = 1.0;
    addTearDown(tester.view.reset);
    final backend = FakeBackend();
    final api = HttpApiClient(
      baseUrl: 'http://test.local',
      client: MockClient(backend.handle),
    );
    await tester.pumpWidget(
      ModuleCatalogScope(
        catalog: ModuleCatalog.defaults(),
        child: MaterialApp(
          home: QuestionScreen(
            api: api,
            moduleId: 'health',
            openEntry: (_) async {},
          ),
        ),
      ),
    );
    await tester.enterText(find.byType(TextField), 'Werte');
    await tester.pump();
    await tester.tap(find.byTooltip('Frage senden'));
    await tester.pumpAndSettle();

    final scroll = tester.state<ScrollableState>(find.byType(Scrollable).first);
    expect(scroll.position.pixels, 0);
    (answerSpan(tester, '4').recognizer! as TapGestureRecognizer).onTap!();
    await tester.pumpAndSettle();
    expect(scroll.position.pixels, greaterThan(0));
    // Card 4 is on the screen with its text.
    final screen = tester.getRect(find.byType(Scaffold));
    final card = tester.getRect(cardOf(4));
    expect(card.top, greaterThanOrEqualTo(screen.top));
    expect(card.bottom, lessThanOrEqualTo(screen.bottom));
    expect(textOf(4), findsOneWidget);
  });

  testWidgets('screen reader: open, expand and references have own labels', (
    tester,
  ) async {
    final semantics = tester.ensureSemantics();
    final opened = <String>[];
    await showAnswer(tester, opened);

    // Exactly one node per label: the text and the single texts are not
    // merged into it.
    SemanticsNode node(String label) =>
        find.semantics.byLabel(label).evaluate().single;

    final open = node('Quelle 3: Arztbrief, 02.09.2026, Dr. Erika Beispiel');
    expect(open, isSemantics(isButton: true, hasTapAction: true));
    expect(open.hintOverrides?.onTapHint, 'Eintrag öffnen');
    expect(
      node('Textstelle von Quelle 3 anzeigen'),
      isSemantics(
        isButton: true,
        hasTapAction: true,
        hasExpandedState: true,
        isExpanded: false,
      ),
    );

    // The semantic tap actions do what the labels say.
    tester.semantics.tap(
      find.semantics.byLabel('Textstelle von Quelle 3 anzeigen'),
    );
    await tester.pumpAndSettle();
    expect(textOf(3), findsOneWidget);
    expect(
      node('Textstelle von Quelle 3 ausblenden'),
      isSemantics(isButton: true, hasExpandedState: true, isExpanded: true),
    );
    expect(opened, isEmpty);

    // A reference reads „Quelle 1 anzeigen“ and expands source 1.
    expect(find.semantics.byLabel('Quelle 1 anzeigen').evaluate(), isNotEmpty);
    tester.semantics.tap(find.semantics.byLabel('Quelle 4 anzeigen'));
    await tester.pumpAndSettle();
    expect(textOf(4), findsOneWidget);
    expect(opened, isEmpty);

    tester.semantics.tap(
      find.semantics.byLabel(
        'Quelle 1: Laborbefund, 14.10.2026, Labor Muster GmbH, Salzburg',
      ),
    );
    await tester.pumpAndSettle();
    expect(opened, [labReportId]);

    semantics.dispose();
  });
}
