import 'package:flutter_test/flutter_test.dart';
import 'package:lifelog/model/citations.dart';

/// The references as (text in the answer, number).
List<(String, int)> refs(String text) => [
  for (final c in findCitations(text))
    (text.substring(c.start, c.end), c.number),
];

void main() {
  test('a single reference covers the brackets', () {
    expect(refs('Hämoglobin 14,2 g/dl [1].'), [('[1]', 1)]);
    expect(findCitations('a [1]'), [
      const Citation(start: 2, end: 5, number: 1),
    ]);
  });

  test('several in a row and two-digit numbers', () {
    expect(refs('Wert [1][3], anderer [12]'), [
      ('[1]', 1),
      ('[3]', 3),
      ('[12]', 12),
    ]);
  });

  test('a list: each number on its own, brackets and comma are not part', () {
    expect(refs('Werte [1, 3] und [2,4]'), [
      ('1', 1),
      ('3', 3),
      ('2', 2),
      ('4', 4),
    ]);
  });

  test('no references', () {
    expect(findCitations('Dazu finde ich nichts in deinen Dokumenten.'), []);
    expect(findCitations('Text [a] [] [1a] [ 1] [1,] 1'), []);
  });

  test('a number too large for an int is no reference', () {
    expect(refs('[99999999999999999999999] [2]'), [('[2]', 2)]);
  });
}
