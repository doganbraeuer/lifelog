// A simplified second reader for the mock: it checks date, issuer and facts
// of an entry against the "original" the way docs/API.md, section 2
// ("Verification") describes it, so that switching to the real backend
// brings no surprises.
//
// Simplifications compared to the backend (on purpose, the mock only needs
// plausible results):
// - No Unicode NFKC and no visual rows from positions: a line is a line of
//   the mock original.
// - The unit only has to follow the value in the same line (no limit of
//   three words), and the issuer is not searched across line breaks.
// - Rectangles are estimated from line and column in the layout of the page
//   (`MockLayout`: the mock PDF or the mock photo), not measured.

import '../../model/plain_date.dart';
import '../../model/verification.dart';
import 'mock_files.dart';

/// One page of the original as the second reader sees it.
class MockPage {
  const MockPage(this.source, this.lines, {this.layout = MockLayout.pdfA4});

  /// `pdfText`, `ocr` or `text`.
  final VerificationSource source;
  final List<String> lines;

  /// Where the lines are on the page; for rectangles.
  final MockLayout layout;
}

/// What the second reader sees of a document. It may differ from `fullText`
/// (the reading of the vision LLM), e.g. a unit OCR read wrongly.
class MockOriginal {
  const MockOriginal(this.pages);

  /// One page of a file read by text layer or OCR.
  MockOriginal.page(
    VerificationSource source,
    List<String> lines, {
    MockLayout layout = MockLayout.pdfA4,
  }) : pages = [MockPage(source, lines, layout: layout)];

  /// A text import: the entered text itself.
  MockOriginal.text(String fullText)
    : pages = [MockPage(VerificationSource.text, fullText.split('\n'))];

  final List<MockPage> pages;

  /// `mixed` if the pages used both text layer and OCR.
  VerificationSource get source {
    final sources = {for (final page in pages) page.source};
    return sources.length == 1 ? sources.single : VerificationSource.mixed;
  }

  /// Programs with version, both separated by a comma for `mixed`, `null`
  /// for a text import.
  String? get engine {
    final names = [
      for (final source in [VerificationSource.pdfText, VerificationSource.ocr])
        if (pages.any((p) => p.source == source)) _engines[source]!,
    ];
    return names.isEmpty ? null : names.join(', ');
  }

  static const _engines = {
    VerificationSource.pdfText: 'pdftotext 26.11.0',
    VerificationSource.ocr: 'tesseract 5.5.4 deu',
  };
}

/// `verification` of an entry that could not be checked.
Map<String, dynamic> uncheckedVerification(VerificationReason reason) => {
  'status': 'unchecked',
  'reason': reason.code,
  'source': null,
  'engine': null,
  'checkedAt': null,
  'confirmed': 0,
  'corrected': 0,
  'total': 0,
  'matchPercent': null,
  'fields': <dynamic>[],
};

/// Checks the current values of [entry] (JSON as from the backend) against
/// [original] and returns `verification` as JSON.
///
/// A field counts as corrected while `corrections` holds an item for it;
/// its status is then `corrected_confirmed` or `corrected_unconfirmed`.
/// Fields that are `null` are not checked.
Map<String, dynamic> checkEntry(
  Map<String, dynamic> entry,
  MockOriginal original, {
  required String checkedAt,
}) {
  final corrected = [
    for (final c in entry['corrections'] as List) (c as Map)['field'] as String,
  ];
  final reader = _Reader(original);
  final fields = <Map<String, dynamic>>[];

  void add(String field, List<_Match> matches, bool isCorrected) {
    final found = matches.isNotEmpty;
    final status = switch ((isCorrected, found)) {
      (false, true) => FieldCheckStatus.confirmed,
      (false, false) => FieldCheckStatus.unconfirmed,
      (true, true) => FieldCheckStatus.correctedConfirmed,
      (true, false) => FieldCheckStatus.correctedUnconfirmed,
    };
    final best = matches.take(3).toList();
    final source = found ? reader.pageSource(best.first) : null;
    fields.add({
      'field': field,
      'status': status.code,
      'source': source?.code,
      'confidence': source == VerificationSource.ocr
          ? reader.confidence(best.first)
          : null,
      'locations': [for (final m in best) reader.location(m)],
    });
  }

  final date = PlainDate.tryParse(entry['documentDate'] as String? ?? '');
  if (date != null) {
    add(
      'documentDate',
      reader.findDate(date),
      corrected.contains('documentDate'),
    );
  }
  final issuer = entry['issuer'] as String?;
  if (issuer != null) {
    add('issuer', reader.findIssuer(issuer), corrected.contains('issuer'));
  }
  for (final fact in (entry['facts'] as List).cast<Map<String, dynamic>>()) {
    final id = fact['id'] as String;
    add(
      'facts.$id',
      reader.findFact(
        fact['name'] as String,
        fact['value'] as String,
        fact['unit'] as String?,
      ),
      corrected.any((field) => field.startsWith('facts.$id.')),
    );
  }

  final total = fields.length;
  final confirmed = fields
      .where((f) => FieldCheckStatus.parse(f['status'] as String).isFound)
      .length;
  final correctedCount = fields
      .where((f) => (f['status'] as String).startsWith('corrected_'))
      .length;
  return {
    'status': 'checked',
    'reason': null,
    'source': original.source.code,
    'engine': original.engine,
    'checkedAt': checkedAt,
    'confirmed': confirmed,
    'corrected': correctedCount,
    'total': total,
    'matchPercent': total == 0 ? null : (confirmed * 100 / total).round(),
    'fields': fields,
  };
}

/// A hit: page and line index, range in code points of the line.
class _Match {
  const _Match(this.page, this.line, this.start, this.end);

  final int page;
  final int line;
  final int start;

  /// Exclusive.
  final int end;

  int compareTo(_Match other) => page != other.page
      ? page - other.page
      : line != other.line
      ? line - other.line
      : start - other.start;
}

/// A line without whitespace, in lower case, with a decimal comma between
/// digits as a point and uniform dashes and quotation marks. [origin] maps
/// every code unit of [text] back to the code point in the line.
class _Normalized {
  _Normalized(String line) : runes = line.runes.toList() {
    final buffer = StringBuffer();
    for (var i = 0; i < runes.length; i++) {
      final char = String.fromCharCode(runes[i]);
      if (char.trim().isEmpty) continue;
      var normal = _replacements[char] ?? char.toLowerCase();
      if (char == ',' && _digitAt(i - 1) && _digitAt(i + 1)) normal = '.';
      for (final unit in normal.codeUnits) {
        buffer.writeCharCode(unit);
        origin.add(i);
      }
    }
    text = buffer.toString();
  }

  final List<int> runes;
  final origin = <int>[];
  late final String text;

  static const _replacements = {
    '–': '-',
    '—': '-',
    '‑': '-',
    '−': '-',
    '„': '"',
    '“': '"',
    '”': '"',
    '‚': "'",
    '‘': "'",
    '’': "'",
  };

  static final _wordChar = RegExp(r'[\p{L}\p{N}]', unicode: true);

  bool _digitAt(int i) =>
      i >= 0 &&
      i < runes.length &&
      runes[i] >= 0x30 && // '0'
      runes[i] <= 0x39; // '9'

  /// Part of a word: a letter or digit, or a decimal separator between
  /// digits (so `2` is not found inside `14,2`).
  bool _wordAt(int i) {
    if (i < 0 || i >= runes.length) return false;
    final char = String.fromCharCode(runes[i]);
    if (_wordChar.hasMatch(char)) return true;
    return (char == ',' || char == '.') && _digitAt(i - 1) && _digitAt(i + 1);
  }

  /// All hits of [needle] (already normalized), as ranges of code points,
  /// without looking at word boundaries.
  Iterable<(int, int)> findAll(String needle) sync* {
    if (needle.isEmpty) return;
    var from = 0;
    while (true) {
      final index = text.indexOf(needle, from);
      if (index < 0) return;
      yield (origin[index], origin[index + needle.length - 1] + 1);
      from = index + 1;
    }
  }

  /// A word boundary lies before code point [start].
  bool boundaryBefore(int start) => !_wordAt(start - 1);

  /// A word boundary lies after a range ending (exclusively) at [end].
  bool boundaryAfter(int end) => !_wordAt(end);

  /// All hits of [needle] that start and end at word boundaries.
  Iterable<(int, int)> find(String needle) =>
      findAll(needle)
          .where((hit) => boundaryBefore(hit.$1) && boundaryAfter(hit.$2));
}

class _Reader {
  _Reader(this.original)
    : _lines = [
        for (final page in original.pages)
          [for (final line in page.lines) _Normalized(line)],
      ];

  final MockOriginal original;
  final List<List<_Normalized>> _lines;

  static String _normalize(String phrase) => _Normalized(phrase).text;

  /// All hits of [phrase] in document order.
  List<_Match> find(String phrase) {
    final needle = _normalize(phrase);
    return [
      for (var p = 0; p < _lines.length; p++)
        for (var l = 0; l < _lines[p].length; l++)
          for (final (start, end) in _lines[p][l].find(needle))
            _Match(p, l, start, end),
    ];
  }

  /// The date in common German forms (docs/API.md, rule 2).
  List<_Match> findDate(PlainDate date) {
    String two(int n) => n.toString().padLeft(2, '0');
    final d = date.day, m = date.month, y = date.year;
    final shortYear = two(y % 100);
    final forms = {
      '${two(d)}.${two(m)}.$y',
      '$d.$m.$y',
      '${two(d)}.${two(m)}.$shortYear',
      '$d.$m.$shortYear',
      date.iso,
      '${two(d)}/${two(m)}/$y',
      '$d. ${_months[m - 1]} $y',
      '${two(d)}. ${_months[m - 1]} $y',
      if (m == 1) '$d. Jänner $y',
    };
    final hits = <_Match>[];
    for (final form in forms) {
      for (final hit in find(form)) {
        if (!hits.any((h) => h.compareTo(hit) == 0)) hits.add(hit);
      }
    }
    return hits..sort((a, b) => a.compareTo(b));
  }

  static const _months = [
    'Januar',
    'Februar',
    'März',
    'April',
    'Mai',
    'Juni',
    'Juli',
    'August',
    'September',
    'Oktober',
    'November',
    'Dezember',
  ];

  /// The issuer as a whole, or each comma-separated part somewhere in the
  /// document; then one location per part (rule 3).
  List<_Match> findIssuer(String issuer) {
    final whole = find(issuer);
    if (whole.isNotEmpty) return whole;
    final parts = [
      for (final part in issuer.split(','))
        if (part.trim().isNotEmpty) part.trim(),
    ];
    if (parts.length < 2) return const [];
    final firstHits = <_Match>[];
    for (final part in parts) {
      final hits = find(part);
      if (hits.isEmpty) return const [];
      firstHits.add(hits.first);
    }
    return firstHits;
  }

  /// The value as a whole; the unit after it in the same line. A short value
  /// without a unit only counts in the line of the first word of the name.
  /// Hits in the line of the name come first (rule 4).
  List<_Match> findFact(String name, String value, String? unit) {
    final nameWord = _normalize(name.trim().split(RegExp(r'\s+')).first);
    bool inNameLine(_Match m) => _lines[m.page][m.line].text.contains(nameWord);

    final hits = <_Match>[];
    if (unit == null) {
      for (final hit in find(value)) {
        if (value.trim().runes.length <= 3 && !inNameLine(hit)) continue;
        hits.add(hit);
      }
    } else {
      // Whitespace does not count, so the unit may follow the value
      // directly (`64/min`, `14,2g/dl`); otherwise both need boundaries.
      final valueNeedle = _normalize(value);
      final unitNeedle = _normalize(unit);
      for (var p = 0; p < _lines.length; p++) {
        for (var l = 0; l < _lines[p].length; l++) {
          final line = _lines[p][l];
          for (final (start, end) in line.findAll(valueNeedle)) {
            if (!line.boundaryBefore(start)) continue;
            final unitHit = line
                .findAll(unitNeedle)
                .where(
                  (u) =>
                      u.$1 >= end &&
                      line.boundaryAfter(u.$2) &&
                      (u.$1 == end ||
                          (line.boundaryAfter(end) &&
                              line.boundaryBefore(u.$1))),
                )
                .firstOrNull;
            if (unitHit != null) hits.add(_Match(p, l, start, unitHit.$2));
          }
        }
      }
    }
    final inLine = hits.where(inNameLine);
    return [...inLine, ...hits.where((h) => !inNameLine(h))];
  }

  VerificationSource pageSource(_Match m) => original.pages[m.page].source;

  /// A made-up but stable OCR confidence between 80 and 96.
  int confidence(_Match m) {
    final text = String.fromCharCodes(
      _lines[m.page][m.line].runes.sublist(m.start, m.end),
    );
    return 80 + text.codeUnits.fold(0, (a, b) => a + b) % 17;
  }

  static double _fraction(double value) =>
      double.parse(value.toStringAsFixed(4));

  /// A range in `fullText` for a text import, otherwise a rectangle.
  Map<String, dynamic> location(_Match m) {
    final page = original.pages[m.page];
    if (page.source == VerificationSource.text) {
      // Code points of the lines before, plus one per line break.
      var offset = 0;
      for (var l = 0; l < m.line; l++) {
        offset += page.lines[l].runes.length + 1;
      }
      return {'start': offset + m.start, 'end': offset + m.end};
    }
    final layout = page.layout;
    return {
      'page': m.page + 1,
      'x': _fraction(
        (layout.left + m.start * layout.charWidth) / layout.pageWidth,
      ),
      'y': _fraction(
        (layout.firstTop + m.line * layout.lineHeight) / layout.pageHeight,
      ),
      'width': _fraction(
        (m.end - m.start) * layout.charWidth / layout.pageWidth,
      ),
      'height': _fraction(layout.textHeight / layout.pageHeight),
    };
  }
}
