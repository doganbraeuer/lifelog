import 'package:flutter_test/flutter_test.dart';
import 'package:lifelog/model/excerpt_location.dart';

/// The text that [locateExcerpt] finds, or `null`.
String? found(String fullText, String excerpt) =>
    locateExcerpt(fullText, excerpt)?.textIn(fullText);

void main() {
  test('an excerpt contained word for word', () {
    const text = 'Labor\nHämoglobin 14,2 g/dl\nLeukozyten 6,8 G/l';
    final location = locateExcerpt(text, 'Hämoglobin 14,2 g/dl')!;
    expect(location.start, 6);
    expect(location.end, 26);
    expect(location.textIn(text), 'Hämoglobin 14,2 g/dl');
  });

  // The cases below are built the way backend/internal/core/chunks.go
  // builds chunks.
  group('chunks whose whitespace differs from the full text', () {
    test('paragraphs joined with a blank line, trimmed', () {
      // Paragraph separator with spaces in the "blank" line and indented
      // paragraphs: the chunk has "\n\n" and no spaces.
      const text = 'Befund:\n   \n   Blutdruck 128/82 mmHg  \n\t\nPuls 72/min';
      expect(
        found(text, 'Blutdruck 128/82 mmHg\n\nPuls 72/min'),
        'Blutdruck 128/82 mmHg  \n\t\nPuls 72/min',
      );
    });

    test('lines of a long paragraph trimmed and joined with "\\n"', () {
      const text = 'Zeile eins   \n   Zeile zwei\n\n\nRest';
      expect(
        found(text, 'Zeile eins\nZeile zwei'),
        'Zeile eins   \n   Zeile zwei',
      );
    });

    test('words of a long line joined with one space', () {
      const text = 'Diagnose:  Gonarthrose \t rechts';
      expect(
        found(text, 'Diagnose: Gonarthrose rechts'),
        'Diagnose:  Gonarthrose \t rechts',
      );
    });

    test(r'\r\n and \r in the full text', () {
      const text = 'Kopf\r\n\r\nHämoglobin 14,2\r\ng/dl\rEnde';
      expect(
        found(text, 'Hämoglobin 14,2\ng/dl\nEnde'),
        'Hämoglobin 14,2\r\ng/dl\rEnde',
      );
    });

    test('a hard cut inside a long word: the chunk starts in the word', () {
      final word = 'A' * 10 + 'BCDEF';
      final text = 'Vorher $word nachher';
      expect(found(text, 'BCDEF nachher'), 'BCDEF nachher');
    });

    test('characters with a meaning in a regular expression', () {
      const text = 'CRP <0,5 mg/dl (Ref. <0,5) [n.b.] *1 + 2? \$x^';
      expect(
        found(text, '(Ref. <0,5) [n.b.] *1 + 2? \$x^'),
        '(Ref. <0,5) [n.b.] *1 + 2? \$x^',
      );
    });
  });

  test('counted in code points, like TextLocation', () {
    // The emoji is two UTF-16 units but one code point.
    const text = 'Notiz 😀 Puls 72/min';
    final location = locateExcerpt(text, 'Puls 72/min')!;
    expect(location.start, 8);
    expect(location.end, 19);
    expect(location.utf16Range(text), (9, 20));
    expect(location.textIn(text), 'Puls 72/min');
  });

  test('the first match if the excerpt occurs twice', () {
    const text = 'Puls 72\nPuls 72';
    expect(locateExcerpt(text, 'Puls 72')!.start, 0);
  });

  test('not found or empty: null', () {
    expect(locateExcerpt('Blutdruck 128/82 mmHg', 'Puls 72/min'), isNull);
    expect(locateExcerpt('Blutdruck', ''), isNull);
    expect(locateExcerpt('Blutdruck', ' \n '), isNull);
    expect(locateExcerpt('', 'Blutdruck'), isNull);
    // Whitespace between words is required where the excerpt has it.
    expect(locateExcerpt('Blutdruck128', 'Blutdruck 128'), isNull);
  });
}
