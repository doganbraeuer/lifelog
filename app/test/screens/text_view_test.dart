import 'dart:convert';
import 'dart:typed_data';

import 'package:flutter/material.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:http/http.dart' as http;
import 'package:http/testing.dart';
import 'package:lifelog/api/http_api_client.dart';
import 'package:lifelog/model/entry.dart';
import 'package:lifelog/module/module_catalog.dart';
import 'package:lifelog/screens/entry_screen.dart';
import 'package:lifelog/screens/original_file_screen.dart';

import '../helpers.dart';

/// Zeigt eine schon geladene Datei. Der Mock ist auf Fehler gestellt: Würde
/// der Screen trotzdem laden, erschiene eine Fehlermeldung.
Future<void> zeigeText(WidgetTester tester, List<int> bytes) async {
  await tester.pumpWidget(
    MaterialApp(
      home: OriginalFileScreen(
        api: schnellerMock()..simulateError = true,
        entryId: laborId,
        fileName: 'text.txt',
        preloaded: FileContent(
          bytes: Uint8List.fromList(bytes),
          mimeType: 'text/plain; charset=utf-8',
        ),
      ),
    ),
  );
  await tester.pumpAndSettle();
}

void main() {
  group('Originaldatei als Text', () {
    testWidgets('UTF-8 mit Umlauten, markierbar', (tester) async {
      await zeigeText(
        tester,
        utf8.encode('Hausarztpraxis Beispiel\nÜberweisung: Röntgen'),
      );

      expect(find.byType(SelectableText), findsOneWidget);
      expect(find.textContaining('Überweisung: Röntgen'), findsOneWidget);
      expect(find.textContaining('Fehler'), findsNothing);
    });

    testWidgets('ungültige Bytes: kein Absturz, Ersatzzeichen', (tester) async {
      // 0xFF kommt in UTF-8 nie vor; 0xC3 allein ist ein abgebrochenes „ä“.
      await zeigeText(tester, [0x48, 0xFF, 0x69, 0x20, 0xC3]);

      expect(tester.takeException(), isNull);
      expect(find.textContaining('H�i'), findsOneWidget);
    });

    testWidgets('BOM am Anfang wird entfernt', (tester) async {
      await zeigeText(tester, [0xEF, 0xBB, 0xBF, ...utf8.encode('Text')]);

      final text = tester.widget<SelectableText>(find.byType(SelectableText));
      expect(text.data, 'Text');
    });

    testWidgets('leere Datei', (tester) async {
      await zeigeText(tester, utf8.encode('  \n '));

      expect(find.text('Die Textdatei ist leer.'), findsOneWidget);
    });

    testWidgets('langer Text lässt sich scrollen', (tester) async {
      await zeigeText(
        tester,
        utf8.encode([for (var i = 1; i <= 200; i++) 'Zeile $i'].join('\n')),
      );

      final scrollbar = find.descendant(
        of: find.byType(OriginalFileScreen),
        matching: find.byType(Scrollable),
      );
      await tester.drag(scrollbar.first, const Offset(0, -500));
      await tester.pumpAndSettle();

      expect(
        tester.state<ScrollableState>(scrollbar.first).position.pixels,
        greaterThan(0),
      );
    });
  });

  testWidgets('Eintrag mit text.txt über den HttpApiClient', (tester) async {
    const id = '6727b000c2b4e5d6f7a8b9f0';
    const text = 'Testeintrag\nÜberweisung vom 21.10.2026';
    final entry = {
      'id': id,
      'module': 'health',
      'type': 'other',
      'documentDate': null,
      'importedAt': '2026-11-04T10:00:00Z',
      'issuer': null,
      'fullText': text,
      'facts': <Object>[],
      'file': {
        'id': '6727b000c2b4e5d6f7a8b9f1',
        'fileName': 'text.txt',
        'mimeType': 'text/plain; charset=utf-8',
        'size': utf8.encode(text).length,
        'sha256': '0' * 64,
      },
      'corrections': <Object>[],
    };
    final api = HttpApiClient(
      baseUrl: 'http://test.local',
      client: MockClient((anfrage) async {
        if (anfrage.url.path.endsWith('/file')) {
          return http.Response.bytes(
            utf8.encode(text),
            200,
            headers: {'content-type': 'text/plain; charset=utf-8'},
          );
        }
        return http.Response.bytes(
          utf8.encode(jsonEncode(entry)),
          200,
          headers: {'content-type': 'application/json'},
        );
      }),
    );

    grosserBildschirm(tester);
    await tester.pumpWidget(
      ModuleCatalogScope(
        catalog: ModuleCatalog.defaults(),
        child: MaterialApp(
          home: EntryScreen(api: api, entryId: id),
        ),
      ),
    );
    await tester.pumpAndSettle();

    // Datei-Karte: „Text“ statt „Bild“.
    expect(find.text('text.txt'), findsOneWidget);
    expect(find.textContaining('· Text'), findsOneWidget);
    expect(find.byIcon(Icons.article_outlined), findsOneWidget);

    await tester.tap(find.text('Originaldatei öffnen'));
    await tester.pumpAndSettle();

    expect(find.byType(OriginalFileScreen), findsOneWidget);
    expect(find.byType(SelectableText), findsOneWidget);
    expect(find.textContaining('Überweisung vom 21.10.2026'), findsOneWidget);
  });
}
