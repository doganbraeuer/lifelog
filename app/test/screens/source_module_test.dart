import 'dart:convert';

import 'package:flutter/material.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:http/http.dart' as http;
import 'package:http/testing.dart';
import 'package:lifelog/api/http_api_client.dart';
import 'package:lifelog/module/module_catalog.dart';
import 'package:lifelog/screens/question_screen.dart';

import '../helpers.dart';
import '../module/test_catalog.dart';

void main() {
  testWidgets('source card takes the type name from source.module', (
    tester,
  ) async {
    // The question is asked in health, but the source belongs to sport. The
    // type „training“ is only known in sport (test catalog).
    final api = HttpApiClient(
      baseUrl: 'http://test.local',
      client: MockClient(
        (_) async => http.Response.bytes(
          utf8.encode(
            jsonEncode({
              'answer': 'Laut Trainingsplan [1].',
              'found': true,
              'sources': [
                {
                  'number': 1,
                  'entryId': labReportId,
                  'module': 'sport',
                  'type': 'training',
                  'issuer': 'Laufverein Beispiel',
                  'documentDate': null,
                  'excerpt': '10 km locker',
                },
              ],
            }),
          ),
          200,
          headers: {'content-type': 'application/json'},
        ),
      ),
    );

    await tester.pumpWidget(
      ModuleCatalogScope(
        catalog: testCatalog(),
        child: MaterialApp(
          home: QuestionScreen(
            api: api,
            moduleId: 'health',
            openEntry: (_) async {},
          ),
        ),
      ),
    );
    await tester.enterText(find.byType(TextField), 'Training');
    await tester.pump();
    await tester.tap(find.byTooltip('Frage senden'));
    await tester.pumpAndSettle();

    expect(find.text('Training · ohne Datum'), findsOneWidget);
    expect(find.byIcon(Icons.fitness_center), findsOneWidget);
  });
}
