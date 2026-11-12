import 'package:flutter/material.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:lifelog/api/mock_api_client.dart';
import 'package:lifelog/widgets/quantity_highlight.dart';

/// The marked parts of [text], as the reader sees them.
List<String> marked(String text) =>
    findQuantities(text).map((range) => range.textInside(text)).toList();

const markedStyle = TextStyle(fontWeight: FontWeight.w600);

void main() {
  group('findQuantities', () {
    test('the mock transcript: numbers with their units', () {
      expect(marked(MockApiClient.sampleTranscript), [
        '12.11.2026',
        '400 Milligramm',
        '2,5 Milliunits pro Liter',
        '130/85 mmHg',
        '37,6 Grad',
      ]);
    });

    test('a real but wrong unit is marked too (the Whisper case)', () {
      expect(marked('TSH 2,5 mL pro Liter'), ['2,5 mL pro Liter']);
    });

    test('unknown or misspelled units are marked: no list of units', () {
      expect(marked('2 Milli-Units, 3 Milliunitz'), [
        '2 Milli-Units',
        '3 Milliunitz',
      ]);
    });

    test('decimal comma, decimal point, thousands, time', () {
      expect(marked('0,5 l und 1.5 l, 1.000 Schritte, 8:30 Uhr'), [
        '0,5 l',
        '1.5 l',
        '1.000 Schritte',
        '8:30 Uhr',
      ]);
    });

    test('units without a space, with signs and with a slash', () {
      expect(marked('400mg, 5 %, 38,5 °C, 37,6°, 0,9 mg/dl, 1,73 m²'), [
        '400mg',
        '5 %',
        '38,5 °C',
        '37,6°',
        '0,9 mg/dl',
        '1,73 m²',
      ]);
    });

    test('a function word after a number is no unit', () {
      expect(marked('3 bis 5 Tabletten'), ['3', '5 Tabletten']);
      expect(marked('Blutdruck 130 zu 85'), ['130', '85']);
      expect(marked('am 12.11.2026 Nach dem Termin'), ['12.11.2026']);
    });

    test('„pro“ joins a unit, also right after the number', () {
      expect(marked('3 pro Tag'), ['3 pro Tag']);
      expect(marked('5 mg pro Tag'), ['5 mg pro Tag']);
      expect(marked('5 mg je Kilogramm'), ['5 mg je Kilogramm']);
      // A number after „pro“ is a quantity of its own.
      expect(marked('5 mg pro 24 Stunden'), ['5 mg', '24 Stunden']);
    });

    test('digits inside a word are no number', () {
      expect(marked('HbA1c 6,5 Prozent, Vitamin B12'), ['6,5 Prozent']);
    });

    test('the dot at the end of a sentence is not part of the unit', () {
      expect(marked('Gewicht 80 kg. Puls 72. Danach'), ['80 kg', '72']);
    });

    test('a unit on the next line does not belong to the number', () {
      expect(marked('Dosis 5\nTabletten'), ['5']);
    });

    test('non-breaking space between number and unit', () {
      expect(marked('5 mg'), ['5 mg']);
    });

    test('no numbers, empty text', () {
      expect(marked('Kniegelenk rechts, leichter Erguss.'), isEmpty);
      expect(marked(''), isEmpty);
    });
  });

  group('quantitySpans', () {
    String joined(List<TextSpan> spans) => spans.map((s) => s.text).join();

    test('gives back the text unchanged, marks only the quantities', () {
      const text = 'Ibuprofen 400 mg täglich';
      final spans = quantitySpans(
        text,
        findQuantities(text),
        marked: markedStyle,
      );

      expect(joined(spans), text);
      expect(spans.map((s) => (s.text, s.style)), [
        ('Ibuprofen ', null),
        ('400 mg', markedStyle),
        (' täglich', null),
      ]);
    });

    test('underlines the word the keyboard is still working on', () {
      const text = 'Ibuprofen 400 mg';
      final spans = quantitySpans(
        text,
        findQuantities(text),
        composing: const TextRange(start: 7, end: 12),
        marked: markedStyle,
      );

      expect(joined(spans), text);
      expect(spans.map((s) => s.text), ['Ibuprof', 'en ', '40', '0 mg']);
      expect(spans[0].style, isNull);
      expect(spans[1].style!.decoration, TextDecoration.underline);
      expect(spans[2].style!.decoration, TextDecoration.underline);
      expect(spans[2].style!.fontWeight, FontWeight.w600);
      expect(spans[3].style, markedStyle);
    });

    test('text without quantities is one plain span', () {
      final spans = quantitySpans('Erguss', const [], marked: markedStyle);
      expect(spans.map((s) => (s.text, s.style)), [('Erguss', null)]);
    });
  });

  group('QuantityHighlightController', () {
    Future<BuildContext> pumpContext(WidgetTester tester) async {
      late BuildContext context;
      await tester.pumpWidget(
        MaterialApp(
          home: Builder(
            builder: (c) {
              context = c;
              return const SizedBox.shrink();
            },
          ),
        ),
      );
      return context;
    }

    testWidgets('off: plain text as with a normal controller', (tester) async {
      final context = await pumpContext(tester);
      final controller = QuantityHighlightController(text: 'Ibuprofen 400 mg');
      addTearDown(controller.dispose);

      final span = controller.buildTextSpan(
        context: context,
        withComposing: false,
      );

      expect(span.text, 'Ibuprofen 400 mg');
      expect(span.children, isNull);
    });

    testWidgets('on: quantities in the colors of the theme', (tester) async {
      final context = await pumpContext(tester);
      final colors = Theme.of(context).colorScheme;
      final controller = QuantityHighlightController(text: 'Ibuprofen 400 mg')
        ..highlightQuantities = true;
      addTearDown(controller.dispose);

      final span = controller.buildTextSpan(
        context: context,
        withComposing: false,
      );

      final children = span.children!.cast<TextSpan>();
      expect(children.map((s) => s.text), ['Ibuprofen ', '400 mg']);
      expect(children[1].style!.backgroundColor, colors.tertiaryContainer);
      expect(children[1].style!.fontWeight, FontWeight.w600);
      // The color comes from the field, so a disabled field greys it out.
      expect(children[1].style!.color, isNull);
      // The text itself has no marks.
      expect(controller.text, 'Ibuprofen 400 mg');
    });

    test('switching tells the field to draw again, only on a change', () {
      final controller = QuantityHighlightController();
      addTearDown(controller.dispose);
      var notified = 0;
      controller.addListener(() => notified++);

      controller.highlightQuantities = true;
      controller.highlightQuantities = true;
      expect(notified, 1);

      controller.highlightQuantities = false;
      expect(notified, 2);
    });
  });
}
