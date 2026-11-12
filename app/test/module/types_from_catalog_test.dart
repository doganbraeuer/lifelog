import 'package:flutter/material.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:lifelog/api/api_exception.dart';
import 'package:lifelog/model/entry_patch.dart';

import '../helpers.dart';
import 'test_catalog.dart';

void main() {
  group('mock checks types against the module of the entry', () {
    test('type from the catalog is allowed', () async {
      final mock = fastMock(catalog: testCatalog());
      final entry = await mock.correct(
        labReportId,
        const EntryPatch(type: 'impfpass'),
      );
      expect(entry.type, 'impfpass');
    });

    test('type of another module: 400 INVALID_REQUEST', () async {
      final mock = fastMock(catalog: testCatalog());
      await expectLater(
        mock.correctWithJson(labReportId, {'type': 'training'}),
        throwsApiException(ErrorCode.invalidRequest, status: 400),
      );
    });

    test('with the default catalog „impfpass“ is unknown', () async {
      await expectLater(
        fastMock().correctWithJson(labReportId, {'type': 'impfpass'}),
        throwsApiException(ErrorCode.invalidRequest, status: 400),
      );
    });
  });

  testWidgets('type dialog shows the types from the catalog', (tester) async {
    final catalog = testCatalog();
    useLargeScreen(tester);
    await tester.pumpWidget(
      testApp(fastMock(catalog: catalog), catalog: catalog),
    );
    await tester.pumpAndSettle();
    await openHealth(tester);
    await tester.tap(find.text('Labor Muster GmbH, Salzburg'));
    await tester.pumpAndSettle();

    await tester.tap(find.byTooltip('Typ korrigieren'));
    await tester.pumpAndSettle();
    await tester.tap(find.byType(DropdownButtonFormField<String>));
    await tester.pumpAndSettle();

    // Additional type from the catalog; „Training“ belongs to sport.
    expect(find.text('Impfpass').last, findsOneWidget);
    expect(find.text('Training'), findsNothing);

    await tester.tap(find.text('Impfpass').last);
    await tester.pumpAndSettle();
    await tester.tap(find.widgetWithText(FilledButton, 'Speichern'));
    await tester.pumpAndSettle();

    // Title and field show the new type, the hint shows the recognized one.
    expect(find.text('Impfpass'), findsWidgets);
    expect(find.text('korrigiert, erkannt: Laborbefund'), findsOneWidget);
  });
}
