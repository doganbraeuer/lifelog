import 'verification.dart';

/// Finds a source excerpt (`sources[].excerpt`) in the full text of its
/// entry.
///
/// The excerpt is a chunk of the backend (`core.SplitIntoChunks`): paragraphs
/// and lines are trimmed and joined again with `\n\n`, `\n` or a space, and
/// `\r\n` becomes `\n`. So it is not always contained word for word. The
/// search therefore compares only the words in order and accepts any
/// whitespace between them. It needs no word boundaries, so a chunk that
/// starts or ends inside a very long word (hard cut) is found as well.
///
/// Returns the first match as a range in code points, as [TextLocation]
/// counts, or `null` if the excerpt is empty or not found.
TextLocation? locateExcerpt(String fullText, String excerpt) {
  final words = excerpt.trim().split(_whitespace);
  if (words.length == 1 && words.single.isEmpty) return null;
  final pattern = RegExp(words.map(RegExp.escape).join(r'\s+'));
  final match = pattern.firstMatch(fullText);
  if (match == null) return null;
  final start = fullText.substring(0, match.start).runes.length;
  final length = fullText.substring(match.start, match.end).runes.length;
  return TextLocation(start: start, end: start + length);
}

final _whitespace = RegExp(r'\s+');
