/// A reference to a source in the answer text, e.g. `[1]`.
class Citation {
  const Citation({
    required this.start,
    required this.end,
    required this.number,
  });

  /// Range in the answer text (UTF-16 indices, for `substring`).
  final int start;
  final int end;

  /// Points to `sources[].number`.
  final int number;

  @override
  bool operator ==(Object other) =>
      other is Citation &&
      other.start == start &&
      other.end == end &&
      other.number == number;

  @override
  int get hashCode => Object.hash(start, end, number);

  @override
  String toString() => 'Citation($start, $end, $number)';
}

/// The same pattern as the backend (`core.ParseCitations`): `[1]`, `[12]`,
/// and also `[1, 3]` or `[1,3]`.
final _citation = RegExp(r'\[(\d+(?:\s*,\s*\d+)*)\]');
final _digits = RegExp(r'\d+');

/// Finds the references in an answer text, in order.
///
/// A single reference like `[1]` covers the brackets as well, so the tap
/// target is larger. In a list like `[1, 3]` each number is a reference of
/// its own; brackets and commas belong to none.
///
/// It does not check whether a source with the number exists; that is up to
/// the caller.
List<Citation> findCitations(String text) {
  final citations = <Citation>[];
  for (final match in _citation.allMatches(text)) {
    final numbers = _digits.allMatches(match[1]!).toList();
    if (numbers.length == 1) {
      // tryParse: a number too large for an int is no reference.
      final number = int.tryParse(numbers.single[0]!);
      if (number != null) {
        citations.add(
          Citation(start: match.start, end: match.end, number: number),
        );
      }
      continue;
    }
    final offset = match.start + 1; // after „[“
    for (final digits in numbers) {
      final number = int.tryParse(digits[0]!);
      if (number == null) continue;
      citations.add(
        Citation(
          start: offset + digits.start,
          end: offset + digits.end,
          number: number,
        ),
      );
    }
  }
  return citations;
}
