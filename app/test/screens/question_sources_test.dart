import 'dart:convert';

import 'package:flutter/material.dart';
import 'package:flutter/semantics.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:http/http.dart' as http;
import 'package:http/testing.dart';
import 'package:lifelog/api/http_api_client.dart';
import 'package:lifelog/module/module_catalog.dart';
import 'package:lifelog/screens/question_screen.dart';

import '../helpers.dart';

/// Answer of the backend with the sources 1 and 3 (numbers with a gap, as
/// docs/API.md 3.6 allows).
Map<String, dynamic> answerWithGap() => {
  'answer': 'Hämoglobin 14,2 g/dl [1], Blutdruck 128/82 mmHg [3].',
  'found': true,
  'sources': [
    {
      'number': 1,
      'entryId': labReportId,
      'module': 'health',
      'type': 'lab_report',
      'issuer': 'Labor Muster GmbH, Salzburg',
      'documentDate': '2026-10-14',
      'excerpt': 'Hämoglobin 14,2 g/dl (Ref. 13,5-17,5)',
    },
    {
      'number': 3,
      'entryId': doctorLetterId,
      'module': 'health',
      'type': 'doctor_letter',
      'issuer': 'Dr. Erika Beispiel',
      'documentDate': '2026-09-02',
      'excerpt': 'Blutdruck 128/82 mmHg',
    },
  ],
};

/// The question page alone, with a backend that always gives [answerWithGap].
/// Opened entries end up in [opened].
Future<void> showAnswer(WidgetTester tester, List<String> opened) async {
  useLargeScreen(tester);
  final api = HttpApiClient(
    baseUrl: 'http://test.local',
    client: MockClient(
      (_) async => http.Response.bytes(
        utf8.encode(jsonEncode(answerWithGap())),
        200,
        headers: {'content-type': 'application/json'},
      ),
    ),
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

/// The arrow button of the card that shows [header].
Finder arrowOf(String header) => find.descendant(
  of: find.ancestor(of: find.text(header), matching: find.byType(Card)),
  matching: find.byType(IconButton),
);

void main() {
  testWidgets('numbers with a gap stay as the backend sent them', (
    tester,
  ) async {
    final opened = <String>[];
    await showAnswer(tester, opened);

    expect(
      find.descendant(of: find.byType(CircleAvatar), matching: find.text('1')),
      findsOneWidget,
    );
    expect(
      find.descendant(of: find.byType(CircleAvatar), matching: find.text('3')),
      findsOneWidget,
    );
    expect(
      find.descendant(of: find.byType(CircleAvatar), matching: find.text('2')),
      findsNothing,
    );

    // Expanding source 3 shows only its excerpt.
    await tester.tap(arrowOf('Arztbrief · 02.09.2026'));
    await tester.pumpAndSettle();
    expect(find.text('„Blutdruck 128/82 mmHg“'), findsOneWidget);
    expect(find.text('„Hämoglobin 14,2 g/dl (Ref. 13,5-17,5)“'), findsNothing);
    expect(opened, isEmpty);

    // Opening source 3 opens its entry, not the one of source 1.
    await tester.tap(find.text('Arztbrief · 02.09.2026'));
    await tester.pumpAndSettle();
    expect(opened, [doctorLetterId]);
  });

  testWidgets('screen reader: open and expand have their own labels', (
    tester,
  ) async {
    final semantics = tester.ensureSemantics();
    final opened = <String>[];
    await showAnswer(tester, opened);

    // Exactly one node per label: the excerpt and the single texts are not
    // merged into it.
    SemanticsNode node(String label) =>
        find.semantics.byLabel(label).evaluate().single;

    final open = node('Quelle 3: Arztbrief, 02.09.2026, Dr. Erika Beispiel');
    expect(open, isSemantics(isButton: true, hasTapAction: true));
    expect(open.hintOverrides?.onTapHint, 'Eintrag öffnen');
    expect(
      node('Auszug von Quelle 3 anzeigen'),
      isSemantics(
        isButton: true,
        hasTapAction: true,
        hasExpandedState: true,
        isExpanded: false,
      ),
    );

    // The semantic tap actions do what the labels say.
    tester.semantics.tap(
      find.semantics.byLabel('Auszug von Quelle 3 anzeigen'),
    );
    await tester.pumpAndSettle();
    expect(find.text('„Blutdruck 128/82 mmHg“'), findsOneWidget);
    expect(
      node('Auszug von Quelle 3 ausblenden'),
      isSemantics(isButton: true, hasExpandedState: true, isExpanded: true),
    );
    expect(node('„Blutdruck 128/82 mmHg“'), isNotNull);
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
