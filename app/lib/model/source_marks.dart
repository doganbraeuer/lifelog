/// Which lines of a source passage are marked in the full text of its entry.
///
/// The excerpt of a source is a whole chunk. In a short document that is
/// often the whole text, so marking the excerpt would mark everything. Instead
/// only the lines that match the answer sentence citing the source are
/// marked, and rather nothing than too much:
///
/// 1. The anchor is the answer sentence (or sentences) with `[n]` for this
///    source. The question counts with half weight.
/// 2. Terms are numbers, dates, units and words of at least 3 characters
///    ([findTerms]).
/// 3. The text is split into segments: lines, long lines into sentences.
///    Only segments inside the passage can be marked.
/// 4. A term weighs its base weight ([Term.baseWeight]) times its rarity:
///    1 / the number of segments in the whole text that contain it.
/// 5. A segment qualifies only with a number from the anchor or at least two
///    different rare words from the anchor (see [markPassage]).
/// 6. At most [maxMarkedSegments] segments with at least [minScoreShare] of
///    the best score are marked, and never more than [maxMarkedShare] of the
///    text.
library;

import 'answer.dart';
import 'citations.dart';
import 'excerpt_location.dart';
import 'verification.dart';

/// At most this many segments are marked for one source.
const maxMarkedSegments = 3;

/// A segment is marked only with at least this share of the best score.
const minScoreShare = 0.6;

/// Nothing is marked if the marks would cover more than this share of the
/// characters of the whole text.
const maxMarkedShare = 0.35;

/// Lines longer than this (characters) are split into sentences.
const maxSegmentLength = 200;

/// A word with at least this rarity counts as rare.
const minRarity = 0.5;

/// The marks for one source card.
///
/// [current] are the lines of this source (marked strongly), [others] the
/// lines of the other sources from the same entry (marked lightly). Both
/// can be empty. [scrollTo] is the first line of this source, or the whole
/// passage if nothing is marked, so the view scrolls to its start.
typedef SourceMarks = ({
  List<TextLocation> current,
  List<TextLocation> others,
  TextLocation scrollTo,
});

/// The marks of [source] in [fullText], together with the light marks of the
/// other sources in [sameEntry] (this one may be included).
///
/// Returns `null` if the passage of [source] is not found in [fullText].
SourceMarks? markSources({
  required String fullText,
  required Source source,
  required List<Source> sameEntry,
  required String answer,
  required String question,
}) {
  final passage = locateExcerpt(fullText, source.excerpt);
  if (passage == null) return null;
  final current = markPassage(
    fullText: fullText,
    passage: passage,
    answer: answer,
    number: source.number,
    question: question,
  );
  // The 35 % hold for all marks together: another source is only added
  // while the total stays below it.
  final limit = maxMarkedShare * fullText.runes.length;
  var marked = _length(current);
  final others = <TextLocation>[];
  for (final other in sameEntry) {
    if (other.number == source.number) continue;
    final otherPassage = locateExcerpt(fullText, other.excerpt);
    if (otherPassage == null) continue;
    final lines = [
      for (final line in markPassage(
        fullText: fullText,
        passage: otherPassage,
        answer: answer,
        number: other.number,
        question: question,
      ))
        if (![...current, ...others].any((l) => _overlaps(l, line))) line,
    ];
    final length = _length(lines);
    if (marked + length > limit) continue;
    marked += length;
    others.addAll(lines);
  }
  return (
    current: current,
    others: others,
    scrollTo: current.isEmpty ? passage : current.first,
  );
}

/// The segments of [fullText] inside [passage] to mark for source [number],
/// in text order. Empty if nothing should be marked.
///
/// A segment qualifies if it contains a number from the anchor, or at least
/// two different words from the anchor with a rarity of at least
/// [minRarity]. A date alone qualifies a segment only if the anchor has no
/// other number and no rare word, because the date of a document often
/// stands in its header as well. Terms of the [question] only add to the
/// score (half weight); they never qualify a segment.
List<TextLocation> markPassage({
  required String fullText,
  required TextLocation passage,
  required String answer,
  required int number,
  required String question,
}) {
  final anchor = findTerms(anchorText(answer, number));
  if (anchor.isEmpty) return const [];
  final fromQuestion = findTerms(question).difference(anchor);

  final segments = splitSegments(fullText);
  final segmentTerms = [
    for (final (start, end) in segments)
      findTerms(fullText.substring(start, end)),
  ];
  final rarity = <Term, double>{
    for (final term in {...anchor, ...fromQuestion})
      term: _rarity(term, segmentTerms),
  };

  final anchorNumbers = anchor.where((t) => t.kind == TermKind.number).toSet();
  final anchorDates = anchor.where((t) => t.kind == TermKind.date).toSet();
  final rareWords = anchor
      .where((t) => t.kind == TermKind.word && rarity[t]! >= minRarity)
      .toSet();
  final datesQualify = anchorNumbers.isEmpty && rareWords.isEmpty;

  bool qualifies(Set<Term> terms) =>
      terms.any(anchorNumbers.contains) ||
      terms.where(rareWords.contains).length >= 2 ||
      (datesQualify && terms.any(anchorDates.contains));

  double score(Set<Term> terms) {
    var sum = 0.0;
    for (final term in terms) {
      final weight = anchor.contains(term)
          ? 1.0
          : fromQuestion.contains(term)
          ? 0.5
          : 0.0;
      if (weight > 0) sum += term.baseWeight * rarity[term]! * weight;
    }
    return sum;
  }

  final (passageStart, passageEnd) = passage.utf16Range(fullText);
  final candidates = <((int, int), double)>[];
  for (final (start, end) in segments) {
    if (end <= passageStart || start >= passageEnd) continue;
    // A segment that sticks out of the passage is cut at its border.
    final range = _trim(
      fullText,
      start < passageStart ? passageStart : start,
      end > passageEnd ? passageEnd : end,
    );
    if (range == null) continue;
    final terms = findTerms(fullText.substring(range.$1, range.$2));
    if (!qualifies(terms)) continue;
    final points = score(terms);
    if (points > 0) candidates.add((range, points));
  }
  if (candidates.isEmpty) return const [];

  // Best first; with the same score the earlier one.
  candidates.sort((a, b) {
    final byScore = b.$2.compareTo(a.$2);
    return byScore != 0 ? byScore : a.$1.$1.compareTo(b.$1.$1);
  });
  final best = candidates.first.$2;
  final chosen =
      candidates
          .where((c) => c.$2 >= minScoreShare * best)
          .take(maxMarkedSegments)
          .map((c) => c.$1)
          .toList()
        ..sort((a, b) => a.$1.compareTo(b.$1));

  final locations = [
    for (final (start, end) in chosen) _toLocation(fullText, start, end),
  ];
  if (_length(locations) > maxMarkedShare * fullText.runes.length) {
    return const [];
  }
  return locations;
}

/// The answer sentences that cite source [number], without any citations
/// (otherwise the `1` of `[1]` would count as a number). Empty if no
/// sentence cites it.
String anchorText(String answer, int number) {
  final cited = findCitations(
    answer,
  ).where((c) => c.number == number).toList();
  if (cited.isEmpty) return '';
  return [
    for (final (start, end) in splitSentences(answer))
      if (cited.any((c) => c.start >= start && c.start < end))
        answer.substring(start, end).replaceAll(_citationList, ' '),
  ].join('\n');
}

/// Splits [text] into sentences: after `.`, `!` or `?` (and closing quotes)
/// followed by whitespace, and at line breaks. Citations right after the end
/// of a sentence (`… 84 ng/ml. [1]`) belong to it.
///
/// Not split: inside numbers and dates (`2.1`, `14.10.2026`), after common
/// abbreviations (`Dr.`, `z. B.`, `ca.`), before a month name (`14. Oktober`)
/// and before a lowercase letter or digit.
///
/// Returns trimmed, non-empty ranges as UTF-16 indices (for `substring`).
List<(int, int)> splitSentences(String text) {
  final ranges = <(int, int)>[];
  var at = 0;
  void cut(int end, int next) {
    final range = _trim(text, at, end);
    if (range != null) ranges.add(range);
    at = next;
  }

  for (final match in _sentenceEnd.allMatches(text)) {
    if (match.start < at) continue;
    if (match[0] == '\n') {
      cut(match.start, match.end);
      continue;
    }
    if (_keepsSentence(text, match)) continue;
    cut(match.end, match.end);
  }
  cut(text.length, text.length);
  return ranges;
}

/// The segments of [text]: its lines, trimmed and non-empty; a line longer
/// than [maxSegmentLength] split into sentences. UTF-16 indices.
List<(int, int)> splitSegments(String text) {
  final segments = <(int, int)>[];
  var start = 0;
  while (start <= text.length) {
    var end = text.indexOf('\n', start);
    if (end < 0) end = text.length;
    final line = _trim(text, start, end);
    if (line != null) {
      final (from, to) = line;
      if (to - from > maxSegmentLength) {
        for (final (s, e) in splitSentences(text.substring(from, to))) {
          segments.add((from + s, from + e));
        }
      } else {
        segments.add(line);
      }
    }
    start = end + 1;
  }
  return segments;
}

/// Kind of a [Term].
enum TermKind { number, date, unit, word }

/// A term of a text that can match a line: normalized, so that equal
/// values compare equal.
class Term {
  const Term(this.kind, this.text);

  final TermKind kind;

  /// Normalized: lowercase, ä → ae, ö → oe, ü → ue, ß → ss; in numbers `,`
  /// → `.`; a date as `d.m.yyyy`; a unit in one spelling (`µ`, `%`).
  final String text;

  /// Base weight: a number says most, a date less (it is often in the header
  /// too), a word in between, a unit least (often on every line).
  double get baseWeight => switch (kind) {
    TermKind.number => 3,
    TermKind.date => 1.5,
    TermKind.word => 2,
    TermKind.unit => 1,
  };

  @override
  bool operator ==(Object other) =>
      other is Term && other.kind == kind && other.text == text;

  @override
  int get hashCode => Object.hash(kind, text);

  @override
  String toString() => '${kind.name}:$text';
}

/// The terms of [text]: numbers (`2,1` and `2.1` equal; also `130/85`),
/// dates (`02.09.2026`, `2.9.2026`, `2026-09-02` and `2. September 2026`
/// equal), units from a fixed list (`mg/dl`, `mmol/l`, `%`, `mmHg`, …) and
/// words of at least 3 characters that are not filler or general words
/// (`Befund`, `Wert`, `Patient`, …). Case, umlauts and ß do not matter.
Set<Term> findTerms(String text) {
  final terms = <Term>{};
  for (final match in _term.allMatches(text)) {
    if (match[1] != null) {
      final month = _months[_normalize(match[2]!)]!;
      terms.add(
        Term(TermKind.date, '${int.parse(match[1]!)}.$month.${match[3]}'),
      );
    } else if (match[4] != null) {
      terms.add(
        Term(
          TermKind.date,
          '${int.parse(match[6]!)}.${int.parse(match[5]!)}.${match[4]}',
        ),
      );
    } else if (match[7] != null) {
      terms.add(_number(match[7]!));
    } else if (match[8] != null) {
      final unit = match[8]!.toLowerCase().replaceAll('μ', 'µ');
      terms.add(Term(TermKind.unit, _unitAliases[unit] ?? unit));
    } else {
      final word = match[9]!;
      if (word.length < 3) continue;
      final normalized = _normalize(word);
      if (_ignoredWords.contains(normalized)) continue;
      terms.add(Term(TermKind.word, normalized));
    }
  }
  return terms;
}

/// A number, or a date if it has the form `d.m.yyyy` (also `dd.mm.yy`).
Term _number(String digits) {
  final date = _numericDate.firstMatch(digits);
  if (date != null) {
    return Term(
      TermKind.date,
      '${int.parse(date[1]!)}.${int.parse(date[2]!)}.${date[3]}',
    );
  }
  return Term(TermKind.number, digits.replaceAll(',', '.'));
}

double _rarity(Term term, List<Set<Term>> segmentTerms) {
  final count = segmentTerms.where((terms) => terms.contains(term)).length;
  return count == 0 ? 0 : 1 / count;
}

String _normalize(String word) => word
    .toLowerCase()
    .replaceAll('ä', 'ae')
    .replaceAll('ö', 'oe')
    .replaceAll('ü', 'ue')
    .replaceAll('ß', 'ss');

/// [start]–[end] without whitespace at both ends, or `null` if nothing is
/// left.
(int, int)? _trim(String text, int start, int end) {
  while (start < end && _isSpace(text.codeUnitAt(start))) {
    start++;
  }
  while (end > start && _isSpace(text.codeUnitAt(end - 1))) {
    end--;
  }
  return start < end ? (start, end) : null;
}

bool _isSpace(int unit) =>
    unit == 0x20 || (unit >= 0x09 && unit <= 0x0D) || unit == 0xA0;

/// A UTF-16 range of [text] as a [TextLocation] in code points.
TextLocation _toLocation(String text, int start, int end) {
  final from = text.substring(0, start).runes.length;
  return TextLocation(
    start: from,
    end: from + text.substring(start, end).runes.length,
  );
}

int _length(Iterable<TextLocation> locations) =>
    locations.fold(0, (sum, l) => sum + l.end - l.start);

bool _overlaps(TextLocation a, TextLocation b) =>
    a.start < b.end && b.start < a.end;

/// Whether the sentence end [match] is none: after an abbreviation, before a
/// month name, a lowercase letter or a digit.
bool _keepsSentence(String text, RegExpMatch match) {
  final next = _nextWord.matchAsPrefix(text, match.end);
  if (next != null) {
    final word = next[1]!;
    if (_months.containsKey(_normalize(word))) return true;
    final first = word[0];
    if (first != first.toUpperCase() || RegExp(r'\d').hasMatch(first)) {
      return true;
    }
  }
  if (match[0] != '.') return false;
  final before = _wordBefore.firstMatch(text.substring(0, match.start));
  return before != null && _abbreviations.contains(before[1]!.toLowerCase());
}

/// The end of a sentence (punctuation, closing quotes, citations right after
/// it), followed by whitespace or the end; or a line break.
final _sentenceEnd = RegExp(
  r'\n|[.!?]+["“”»«)]*(?:\s*\[\d+(?:\s*,\s*\d+)*\])*(?=\s|$)',
);

final _nextWord = RegExp(r'\s+([\p{L}\p{N}]+)', unicode: true);
final _wordBefore = RegExp(r'(?<![\p{L}])(\p{L}+)$', unicode: true);

/// Words with a dot that end no sentence.
const _abbreviations = {
  'a', 'b', 'bzw', 'ca', 'd', 'dr', 'evtl', 'ggf', 'h', 'inkl', 'mag', //
  'med', 'nr', 'prof', 'ref', 'st', 'str', 'tel', 'u', 'usw', 'vgl', 'z', //
};

/// All citations, also `[1, 3]` (as in `findCitations`).
final _citationList = RegExp(r'\[\d+(?:\s*,\s*\d+)*\]');

/// `dd.mm.yyyy`, `d.m.yyyy` or `dd.mm.yy`.
final _numericDate = RegExp(r'^(\d{1,2})\.(\d{1,2})\.(\d{4}|\d{2})$');

const _months = {
  'januar': 1, 'jaenner': 1, 'februar': 2, 'feber': 2, 'maerz': 3, //
  'april': 4, 'mai': 5, 'juni': 6, 'juli': 7, 'august': 8, //
  'september': 9, 'oktober': 10, 'november': 11, 'dezember': 12,
};

/// Units in their normalized spelling (lowercase, `µ`). Longer ones first,
/// so that `mg/dl` is not read as `mg`.
const _units = [
  'mmol/l', 'µmol/l', 'umol/l', 'nmol/l', 'pmol/l', 'ml/min', //
  'mg/dl', 'µg/dl', 'ug/dl', 'ng/ml', 'pg/ml', 'mg/l', 'µg/l', 'ug/l', //
  'mu/l', 'iu/l', 'ie/l', 'u/l', 'g/dl', 'g/l', 'mmhg', 'prozent', //
  '/min', '°c', 'mg', 'ml', 'µg', 'kg', 'ie', '%',
];

const _unitAliases = {
  'umol/l': 'µmol/l',
  'ug/dl': 'µg/dl',
  'ug/l': 'µg/l',
  'prozent': '%',
};

/// One pass over the text, tried in this order at each position:
/// 1–3 date with a month name, 4–6 ISO date, 7 number, 8 unit, 9 word.
final _term = RegExp(
  r'(?<![\p{L}\p{N}])(?:'
  r'(\d{1,2})\.\s?('
  '$_monthPattern'
  r')\s(\d{4})(?![\p{N}])'
  r'|(\d{4})-(\d{1,2})-(\d{1,2})(?![\p{N}])'
  r'|(\d+(?:[.,:/]\d+)*)'
  r')'
  r'|(?<![\p{L}])('
  '$_unitPattern'
  r')(?![\p{L}\p{N}])'
  r'|(\p{L}[\p{L}\p{N}]*)',
  unicode: true,
  caseSensitive: false,
);

/// The month names, also with `ä` instead of `ae`.
final _monthPattern = _months.keys.join('|').replaceAll('ae', '(?:ae|ä)');

/// The units, `µ` also as the Greek letter μ.
final _unitPattern = _units
    .map((u) => u.replaceAll('/', r'\/').replaceAll('µ', '[µμ]'))
    .join('|');

/// German filler and general words that say nothing about a line. Written
/// normalized (ae, oe, ue, ss).
const _ignoredWords = {
  // Function words.
  'aber', 'alle', 'allem', 'allen', 'aller', 'alles', 'als', 'also', //
  'andere', 'anderen', 'anderer', 'auch', 'auf', 'aus', 'bei', 'beim', //
  'bereits', 'bis', 'bzw', 'circa', 'dabei', 'damit', 'dann', 'darauf', //
  'darin', 'das', 'dass', 'davon', 'dazu', 'dein', 'deine', 'deinem', //
  'deinen', 'deiner', 'deines', 'dem', 'den', 'denn', 'der', 'des', 'die', //
  'dies', 'diese', 'diesem', 'diesen', 'dieser', 'dieses', 'doch', 'dort', //
  'durch', 'ein', 'eine', 'einem', 'einen', 'einer', 'eines', 'etwa', //
  'etwas', 'euch', 'fuer', 'gab', 'gegen', 'gibt', 'habe', 'haben', 'hast', //
  'hat', 'hatte', 'hatten', 'heute', 'hier', 'ihm', 'ihn', 'ihr', 'ihre', //
  'ihrem', 'ihren', 'ihrer', 'immer', 'jede', 'jedem', 'jeden', 'jeder', //
  'jedes', 'jeweils', 'kann', 'kein', 'keine', 'keinem', 'keinen', //
  'keiner', 'koennen', 'konnte', 'lag', 'lagen', 'laut', 'letzte', //
  'letzten', 'letzter', 'liegt', 'liegen', 'man', 'mehr', 'mein', 'meine', //
  'meinem', 'meinen', 'meiner', 'meines', 'mit', 'nach', 'nicht', 'nichts', //
  'noch', 'nur', 'ohne', 'oder', 'schon', 'sehr', 'seit', 'sein', 'seine', //
  'seinem', 'seinen', 'seiner', 'sich', 'sie', 'sind', 'sowie', 'stand', //
  'standen', 'steht', 'stehen', 'ueber', 'uns', 'und', 'unter', 'viel', //
  'viele', 'vom', 'von', 'vor', 'war', 'waren', 'was', 'wann', 'warum', //
  'welche', 'welchem', 'welchen', 'welcher', 'welches', 'wem', 'wen', //
  'wer', 'werden', 'wie', 'wieder', 'wir', 'wird', 'wurde', //
  'wurden', 'zum', 'zur', 'zuletzt', 'betrug', 'betraegt', 'betragen', //
  'finde', 'findet', 'finden', 'gefunden', 'zeigt', 'zeigen', 'bitte', //
  'aktuell', 'aktuelle', 'aktuellen', 'neueste', 'neuesten', 'ich', //
  'mir', 'mich', 'dir', 'dich', 'bin', 'bist', 'ist', 'seid', 'muss', //
  'musst', 'soll', 'sollte', 'worden', 'gewesen', 'waere', 'haette', //
  // General words of documents.
  'angabe', 'angaben', 'arzt', 'aerztin', 'aerzte', 'arztbrief', //
  'arztbriefe', 'arztbriefen', 'befund', 'befunde', 'befunden', 'befundes', //
  'befundbericht', 'datum', 'daten', 'dokument', 'dokumente', 'dokumenten', //
  'dokuments', 'eintrag', 'eintraege', 'eintraegen', 'ergebnis', //
  'ergebnisse', 'ergebnissen', 'information', 'informationen', 'labor', //
  'labors', 'laborbefund', 'laborbefunde', 'laborbefunden', 'laborwert', //
  'laborwerte', 'laborwerten', 'messung', 'messungen', 'normbereich', //
  'normbereiche', 'normwert', 'normwerte', 'patient', 'patientin', //
  'patienten', 'quelle', 'quellen', 'ref', 'referenz', 'referenzbereich', //
  'referenzwert', 'referenzwerte', 'text', 'texte', 'wert', 'werte', //
  'werten', 'wertes',
};
