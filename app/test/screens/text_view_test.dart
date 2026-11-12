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

/// Shows a file that is already loaded. The mock is set to fail: if the
/// screen loaded anyway, an error message would appear.
Future<void> showText(WidgetTester tester, List<int> bytes) async {
  await tester.pumpWidget(
    MaterialApp(
      home: OriginalFileScreen(
        api: fastMock()..simulateError = true,
        entryId: labReportId,
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
  group('original file as text', () {
    testWidgets('UTF-8 with umlauts, selectable', (tester) async {
      await showText(
        tester,
        utf8.encode('Hausarztpraxis Beispiel\nÜberweisung: Röntgen'),
      );

      expect(find.byType(SelectableText), findsOneWidget);
      expect(find.textContaining('Überweisung: Röntgen'), findsOneWidget);
      expect(find.textContaining('Fehler'), findsNothing);
    });

    testWidgets('invalid bytes: no crash, replacement character', (
      tester,
    ) async {
      // 0xFF never occurs in UTF-8; 0xC3 alone is a truncated „ä“.
      await showText(tester, [0x48, 0xFF, 0x69, 0x20, 0xC3]);

      expect(tester.takeException(), isNull);
      expect(find.textContaining('H�i'), findsOneWidget);
    });

    testWidgets('BOM at the start is removed', (tester) async {
      await showText(tester, [0xEF, 0xBB, 0xBF, ...utf8.encode('Text')]);

      final text = tester.widget<SelectableText>(find.byType(SelectableText));
      expect(text.data, 'Text');
    });

    testWidgets('empty file', (tester) async {
      await showText(tester, utf8.encode('  \n '));

      expect(find.text('Die Textdatei ist leer.'), findsOneWidget);
    });

    testWidgets('long text can be scrolled', (tester) async {
      await showText(
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

  testWidgets('entry with text.txt via the HttpApiClient', (tester) async {
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
      client: MockClient((request) async {
        if (request.url.path.endsWith('/file')) {
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

    useLargeScreen(tester);
    await tester.pumpWidget(
      ModuleCatalogScope(
        catalog: ModuleCatalog.defaults(),
        child: MaterialApp(
          home: EntryScreen(api: api, entryId: id),
        ),
      ),
    );
    await tester.pumpAndSettle();

    // File card: „Text“ instead of „Bild“.
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
