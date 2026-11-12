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
  testWidgets('Quellen-Karte nimmt den Typnamen aus quelle.modul', (
    tester,
  ) async {
    // Gefragt wird in Gesundheit, die Quelle gehört aber zu Sport. Der
    // Typ „training“ ist nur in Sport bekannt (Test-Katalog).
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
                  'entryId': laborId,
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
        catalog: testKatalog(),
        child: MaterialApp(
          home: FrageScreen(
            api: api,
            moduleId: 'health',
            eintragOeffnen: (_) async {},
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
