import 'package:flutter/material.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:lifelog/model/entry.dart';
import 'package:lifelog/model/verification.dart';
import 'package:lifelog/screens/verification_view.dart';

import '../helpers.dart';
import 'entry_screen_test.dart' show labIssuer, openEntryNamed, tapAndSettle;

const pdf = File(
  id: 'x',
  fileName: 'a.pdf',
  mimeType: 'application/pdf',
  size: 1,
  sha256: '',
);
const photo = File(
  id: 'x',
  fileName: 'a.jpg',
  mimeType: 'image/jpeg',
  size: 1,
  sha256: '',
);
const text = File(
  id: 'x',
  fileName: 'text.txt',
  mimeType: 'text/plain; charset=utf-8',
  size: 1,
  sha256: '',
);

Verification checked({
  int confirmed = 5,
  int corrected = 0,
  int total = 6,
  VerificationSource source = VerificationSource.pdfText,
}) => Verification(
  status: VerificationStatus.checked,
  reason: null,
  source: source,
  engine: null,
  checkedAt: null,
  confirmed: confirmed,
  corrected: corrected,
  total: total,
  matchPercent: null,
  fields: const [],
);

Verification unchecked(VerificationReason? reason) => Verification(
  status: VerificationStatus.unchecked,
  reason: reason,
  source: null,
  engine: null,
  checkedAt: null,
  confirmed: 0,
  corrected: 0,
  total: 0,
  matchPercent: null,
  fields: const [],
);

void main() {
  group('texts', () {
    test('summary with singular and plural', () {
      expect(
        verificationTitle(checked()),
        '5 von 6 Werten im Original gefunden',
      );
      expect(
        verificationTitle(checked(confirmed: 1, total: 1)),
        '1 von 1 Wert im Original gefunden',
      );
      expect(
        verificationTitle(checked(confirmed: 0, total: 1)),
        '0 von 1 Wert im Original gefunden',
      );
      expect(
        verificationTitle(checked(confirmed: 0, total: 0)),
        'Keine Werte zum Prüfen',
      );
      expect(
        verificationTitle(unchecked(VerificationReason.noText)),
        'Nicht geprüft',
      );
    });

    test('source in plain German, depending on the file', () {
      expect(
        sourceText(VerificationSource.pdfText, pdf),
        'Abgleich mit der Textebene des PDF',
      );
      expect(
        sourceText(VerificationSource.ocr, photo),
        'Abgleich per Texterkennung im Foto',
      );
      expect(
        sourceText(VerificationSource.ocr, pdf),
        'Abgleich per Texterkennung im gescannten PDF',
      );
      expect(
        sourceText(VerificationSource.text, text),
        'Abgleich mit dem eingegebenen Text',
      );
      expect(
        sourceText(VerificationSource.mixed, pdf),
        'Abgleich mit Textebene und Texterkennung im PDF',
      );
      expect(
        sourceText(VerificationSource.unknown, pdf),
        'Abgleich mit dem Original',
      );
    });

    test('detail names corrected values', () {
      expect(
        verificationDetail(checked(), pdf),
        'Abgleich mit der Textebene des PDF',
      );
      expect(
        verificationDetail(checked(corrected: 1), pdf),
        'Abgleich mit der Textebene des PDF · 1 Wert korrigiert',
      );
      expect(
        verificationDetail(checked(corrected: 2), pdf),
        'Abgleich mit der Textebene des PDF · 2 Werte korrigiert',
      );
    });

    test('one neutral sentence per reason', () {
      final sentences = {
        for (final reason in [...VerificationReason.values, null])
          reason: reasonText(reason),
      };
      expect(
        sentences[VerificationReason.ocrUnavailable],
        'Die Prüfung ist auf diesem Server nicht eingerichtet.',
      );
      expect(
        sentences[VerificationReason.notRun],
        'Dieser Eintrag wurde noch nicht geprüft.',
      );
      expect(
        sentences[VerificationReason.unknown],
        sentences[VerificationReason.notRun],
      );
      expect(sentences[null], sentences[VerificationReason.notRun]);
      // Every known reason has its own sentence.
      final known = VerificationReason.values.where(
        (r) => r != VerificationReason.unknown,
      );
      expect({for (final r in known) sentences[r]}, hasLength(known.length));
      expect(
        verificationDetail(unchecked(VerificationReason.tooManyPages), pdf),
        'Das PDF hat mehr als 10 Seiten und wird deshalb nicht geprüft.',
      );
    });
  });

  group('summary in the entry', () {
    testWidgets('PDF with text layer: count, source and corrections', (
      tester,
    ) async {
      await openEntryNamed(tester, fastMock(), labIssuer);
      expect(find.text('5 von 6 Werten im Original gefunden'), findsOneWidget);
      expect(
        find.text('Abgleich mit der Textebene des PDF · 1 Wert korrigiert'),
        findsOneWidget,
      );
    });

    testWidgets('photo, text import and mixed PDF', (tester) async {
      await openEntryNamed(tester, fastMock(), 'Dr. Erika Beispiel');
      expect(find.text('5 von 6 Werten im Original gefunden'), findsOneWidget);
      expect(find.text('Abgleich per Texterkennung im Foto'), findsOneWidget);
      await tapBack(tester);

      await tester.tap(find.textContaining('Blutdruckmessung zu Hause'));
      await tester.pumpAndSettle();
      expect(find.text('4 von 5 Werten im Original gefunden'), findsOneWidget);
      expect(find.text('Abgleich mit dem eingegebenen Text'), findsOneWidget);
      await tapBack(tester);

      await tester.tap(find.textContaining('Dr. Jonas Muster'));
      await tester.pumpAndSettle();
      expect(find.text('5 von 6 Werten im Original gefunden'), findsOneWidget);
      expect(
        find.text(
          'Abgleich mit Textebene und Texterkennung im PDF · 1 Wert korrigiert',
        ),
        findsOneWidget,
      );
    });

    testWidgets('unchecked: neutral, with the reason', (tester) async {
      await openEntryNamed(tester, fastMock(), 'Radiologie Beispiel');
      expect(find.text('Nicht geprüft'), findsOneWidget);
      expect(
        find.text('Die Prüfung ist auf diesem Server nicht eingerichtet.'),
        findsOneWidget,
      );
      // No error colors: the icon is in the neutral color.
      final icon = tester.widget<Icon>(find.byIcon(Icons.remove_done));
      final colors = Theme.of(tester.element(find.text('Nicht geprüft')))
          .colorScheme;
      expect(icon.color, colors.onSurfaceVariant);
      expect(icon.color, isNot(colors.error));
      await tapBack(tester);

      await tester.tap(find.textContaining('Laborpraxis Nord'));
      await tester.pumpAndSettle();
      expect(
        find.text('Dieser Eintrag wurde noch nicht geprüft.'),
        findsOneWidget,
      );
      await tapBack(tester);

      await tester.tap(find.textContaining('Quelle unbekannt'));
      await tester.pumpAndSettle();
      expect(
        find.text('Im Original wurde kein lesbarer Text gefunden.'),
        findsOneWidget,
      );
    });

    testWidgets('the info button opens the explanation', (tester) async {
      await openEntryNamed(tester, fastMock(), labIssuer);
      expect(find.text('So prüft Lifelog die Werte'), findsNothing);

      await tapAndSettle(tester, find.byTooltip(explanationTooltip));
      expect(find.text('So prüft Lifelog die Werte'), findsOneWidget);
      expect(find.textContaining('nicht, dass er falsch ist'), findsOneWidget);
      expect(find.textContaining('medizinisch'), findsOneWidget);
      expect(
        find.textContaining('Programm: pdftotext 26.11.0'),
        findsOneWidget,
      );
    });

    testWidgets('after a correction the summary shows the new check', (
      tester,
    ) async {
      final mock = fastMock();
      await openEntryNamed(tester, mock, labIssuer);

      await tapAndSettle(tester, find.byTooltip('Hämoglobin korrigieren'));
      await tester.enterText(find.widgetWithText(TextField, 'Wert'), '15,0');
      await tapAndSettle(
        tester,
        find.widgetWithText(FilledButton, 'Speichern'),
      );

      expect(find.text('4 von 6 Werten im Original gefunden'), findsOneWidget);
      expect(
        find.text('Abgleich mit der Textebene des PDF · 2 Werte korrigiert'),
        findsOneWidget,
      );
    });
  });

  group('marks per value', () {
    Finder foundMarks() => find.byTooltip(foundLabel);
    Finder notFoundLines() => find.text(notFoundLabel);

    testWidgets('found: check mark, not found: calm line, corrected: hint', (
      tester,
    ) async {
      await openEntryNamed(tester, fastMock(), labIssuer);

      // Date, issuer, Hämoglobin and Befundart; not type and import date,
      // and not the corrected Leukozyten.
      expect(foundMarks(), findsNWidgets(4));
      // CRP: the text layer says „unter 0,5“.
      expect(notFoundLines(), findsOneWidget);
      expect(
        find.text('Wert korrigiert, erkannt: 6,3 · im Original gefunden'),
        findsOneWidget,
      );

      // No warning colors for „not found“.
      final colors = Theme.of(tester.element(notFoundLines())).colorScheme;
      final icon = tester.widget<Icon>(find.byIcon(Icons.search_off));
      expect(icon.color, colors.onSurfaceVariant);
      final line = tester.widget<Text>(notFoundLines());
      expect(line.style?.color, colors.onSurfaceVariant);
      expect(line.style?.color, isNot(colors.error));
    });

    testWidgets('photo and mixed PDF', (tester) async {
      await openEntryNamed(tester, fastMock(), 'Dr. Erika Beispiel');
      // Puls: OCR read „/mln“.
      expect(notFoundLines(), findsOneWidget);
      expect(foundMarks(), findsNWidgets(5));
      await tapBack(tester);

      await tester.tap(find.textContaining('Dr. Jonas Muster'));
      await tester.pumpAndSettle();
      expect(
        find.text('Wert korrigiert, erkannt: 2 · nicht im Original gefunden'),
        findsOneWidget,
      );
      expect(foundMarks(), findsNWidgets(5));
      expect(notFoundLines(), findsNothing);
    });

    testWidgets('unchecked: no marks at all', (tester) async {
      await openEntryNamed(tester, fastMock(), 'Radiologie Beispiel');
      expect(foundMarks(), findsNothing);
      expect(notFoundLines(), findsNothing);
      expect(find.textContaining('im Original gefunden'), findsNothing);
    });

    testWidgets('a correction shows the new result from the response', (
      tester,
    ) async {
      await openEntryNamed(tester, fastMock(), labIssuer);

      await tapAndSettle(tester, find.byTooltip('Hämoglobin korrigieren'));
      await tester.enterText(find.widgetWithText(TextField, 'Wert'), '15,0');
      await tapAndSettle(
        tester,
        find.widgetWithText(FilledButton, 'Speichern'),
      );

      expect(
        find.text(
          'Wert korrigiert, erkannt: 14,2 · nicht im Original gefunden',
        ),
        findsOneWidget,
      );
      expect(foundMarks(), findsNWidgets(3));
    });

    testWidgets('back to the recognized value: the hint goes, a mark comes', (
      tester,
    ) async {
      await openEntryNamed(tester, fastMock(), labIssuer);

      await tapAndSettle(tester, find.byTooltip('Leukozyten korrigieren'));
      await tapAndSettle(tester, find.text('Erkannten Wert übernehmen: 6,3'));
      await tapAndSettle(
        tester,
        find.widgetWithText(FilledButton, 'Speichern'),
      );

      expect(find.textContaining('korrigiert, erkannt'), findsNothing);
      // 6,3 is not in the PDF: Leukozyten and CRP are not found.
      expect(notFoundLines(), findsNWidgets(2));
      expect(foundMarks(), findsNWidgets(4));
      expect(find.text('4 von 6 Werten im Original gefunden'), findsOneWidget);
    });
  });
}
