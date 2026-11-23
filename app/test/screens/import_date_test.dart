// Date for text imports and voice notes (docs/API.md 3.1, section 2
// documentDateSource): a text without a date gets the day of its import,
// shown as „(Importdatum)“ with the time, and is never marked red.
import 'package:flutter/material.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:lifelog/model/entry.dart';
import 'package:lifelog/model/entry_patch.dart';
import 'package:lifelog/model/plain_date.dart';
import 'package:lifelog/screens/verification_view.dart';
import 'package:lifelog/widgets/format.dart';

import '../helpers.dart';

/// Fixed clock of the tests; day and time are shown in local time, so the
/// expectations are computed the same way (independent of the machine).
final clock = DateTime.utc(2026, 11, 23, 9, 46);
final today = PlainDate.fromDateTime(clock.toLocal());
final time = formatLocalTime(clock);

const voiceNote = 'Blutdruck 130 zu 85, Puls 64';

/// Phone size as in timeline_axis_test.dart.
void usePhoneScreen(WidgetTester tester) {
  tester.view.physicalSize = const Size(1080, 2400);
  tester.view.devicePixelRatio = 3;
  addTearDown(tester.view.reset);
}

void main() {
  group('model', () {
    Map<String, dynamic> summary(Object? source) => {
      'id': '6727a1f3c2b4e5d6f7a8b9c0',
      'module': 'health',
      'type': 'other',
      'documentDate': '2026-11-23',
      'importedAt': '2026-11-23T09:46:00Z',
      'issuer': null,
      'documentDateSource': ?source,
    };

    test('"import" is the import day, anything else the document', () {
      expect(
        EntrySummary.fromJson(summary('import')).documentDateSource,
        DocumentDateSource.importDay,
      );
      expect(EntrySummary.fromJson(summary('import')).hasImportDate, isTrue);
      for (final other in ['document', 'later', null]) {
        final e = EntrySummary.fromJson(summary(other));
        expect(
          e.documentDateSource,
          DocumentDateSource.document,
          reason: '$other',
        );
        expect(e.hasImportDate, isFalse);
      }
    });
  });

  group('mock', () {
    test('a text without a date gets the import day, not checked', () async {
      final mock = fastMock(clock: () => clock);
      final e = await mock.importText(voiceNote, module: 'health');

      expect(e.documentDate, today);
      expect(e.documentDateSource, DocumentDateSource.importDay);
      expect(e.verification.checkFor('documentDate'), isNull);
      // The short form of the timeline carries the mark too.
      final list = await mock.listEntries(module: 'health');
      expect(
        list.firstWhere((s) => s.id == e.id).documentDateSource,
        DocumentDateSource.importDay,
      );
    });

    test('a date in the text stays the document date', () async {
      final mock = fastMock(clock: () => clock);
      final e = await mock.importText(
        'Notiz vom 20.11.2026\n$voiceNote',
        module: 'health',
      );
      expect(e.documentDate, PlainDate(2026, 11, 20));
      expect(e.documentDateSource, DocumentDateSource.document);
    });

    test('correcting the date removes the mark; recognized is null', () async {
      final mock = fastMock(clock: () => clock);
      final e = await mock.importText(voiceNote, module: 'health');

      final corrected = await mock.correct(
        e.id,
        EntryPatch(documentDate: Value(PlainDate(2026, 11, 20))),
      );
      expect(corrected.documentDate, PlainDate(2026, 11, 20));
      expect(corrected.documentDateSource, DocumentDateSource.document);
      final correction = corrected.correctionFor('documentDate')!;
      expect(correction.recognized, isNull);
      expect(correction.corrected, '2026-11-20');
    });

    test('removing the date: no date, no correction', () async {
      final mock = fastMock(clock: () => clock);
      final e = await mock.importText(voiceNote, module: 'health');

      final corrected = await mock.correct(
        e.id,
        const EntryPatch(documentDate: Value(null)),
      );
      expect(corrected.documentDate, isNull);
      expect(corrected.documentDateSource, DocumentDateSource.document);
      expect(corrected.corrections, isEmpty);
    });
  });

  group('screens', () {
    testWidgets('entry: „(Importdatum)“ with the time, never red', (
      tester,
    ) async {
      final mock = fastMock(clock: () => clock);
      await mock.importText(voiceNote, module: 'health');
      useLargeScreen(tester);
      await tester.pumpWidget(testApp(mock));
      await tester.pumpAndSettle();
      await openHealth(tester);
      await tester.tap(find.text(voiceNote));
      await tester.pumpAndSettle();

      expect(find.text('${today.display} (Importdatum)'), findsOneWidget);
      expect(
        find.text('Im Text stand kein Datum. Importiert um $time Uhr.'),
        findsOneWidget,
      );
      expect(find.text(notFoundLabel), findsNothing);
      expect(find.text('nicht erkannt'), findsNothing);
      // It can still be corrected.
      expect(find.byTooltip('Dokumentdatum korrigieren'), findsOneWidget);
    });

    testWidgets('timeline: date and time of the import, nothing overflows', (
      tester,
    ) async {
      final mock = fastMock(clock: () => clock);
      await mock.importText(voiceNote, module: 'health');
      usePhoneScreen(tester);
      await tester.pumpWidget(testApp(mock));
      await tester.pumpAndSettle();
      await openHealth(tester);

      expect(find.text('${today.display}, $time'), findsOneWidget);
      expect(tester.takeException(), isNull);
    });
  });
}
