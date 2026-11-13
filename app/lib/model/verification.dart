// Check of the recognized values by a second reader (docs/API.md, section 2,
// "Verification"): the text layer of a PDF, OCR for photos and scans, or the
// entered text of a text import.
//
// Wrong JSON types throw a FormatException, as everywhere in the model.
// Unknown *values* (e.g. a reason the backend adds later) become `unknown`
// instead, so a newer backend never breaks the entry view.

import 'json_helpers.dart';

/// `verification.status`.
enum VerificationStatus {
  checked,
  unchecked;

  static VerificationStatus parse(String value) =>
      value == 'checked' ? checked : unchecked;
}

/// What was compared against: `verification.source` and
/// `verification.fields[].source`. `mixed` only occurs for the entry.
enum VerificationSource {
  pdfText('pdf_text'),
  ocr('ocr'),
  text('text'),
  mixed('mixed'),
  unknown('');

  const VerificationSource(this.code);

  /// The value in the JSON.
  final String code;

  static VerificationSource? parse(String? value) => value == null
      ? null
      : values.firstWhere((s) => s.code == value, orElse: () => unknown);
}

/// Why an entry is `unchecked`: `verification.reason`.
enum VerificationReason {
  ocrUnavailable('ocr_unavailable'),
  ocrFailed('ocr_failed'),
  unsupportedFileType('unsupported_file_type'),
  tooManyPages('too_many_pages'),
  imageTooLarge('image_too_large'),
  noText('no_text'),
  notRun('not_run'),
  unknown('');

  const VerificationReason(this.code);

  /// The value in the JSON.
  final String code;

  static VerificationReason? parse(String? value) => value == null
      ? null
      : values.firstWhere((r) => r.code == value, orElse: () => unknown);
}

/// `verification.fields[].status`.
enum FieldCheckStatus {
  /// Found in the original.
  confirmed('confirmed'),

  /// Not found in the original. This does not mean the value is wrong.
  unconfirmed('unconfirmed'),

  /// Corrected by the user, and the corrected value is in the original.
  correctedConfirmed('corrected_confirmed'),

  /// Corrected by the user, and the corrected value is not in the original.
  correctedUnconfirmed('corrected_unconfirmed'),
  unknown('');

  const FieldCheckStatus(this.code);

  /// The value in the JSON.
  final String code;

  static FieldCheckStatus parse(String value) =>
      values.firstWhere((s) => s.code == value, orElse: () => unknown);

  /// The current value was found in the original.
  bool get isFound => this == confirmed || this == correctedConfirmed;

  /// The current value was searched for and not found.
  bool get isNotFound => this == unconfirmed || this == correctedUnconfirmed;
}

/// Result of the check of an entry.
class Verification {
  const Verification({
    required this.status,
    required this.reason,
    required this.source,
    required this.engine,
    required this.checkedAt,
    required this.confirmed,
    required this.corrected,
    required this.total,
    required this.matchPercent,
    required this.fields,
  });

  /// For an entry without `verification` (backend from before 2026-11-11):
  /// it counts as never checked, like old entries in MongoDB.
  const Verification.notRun()
    : status = VerificationStatus.unchecked,
      reason = VerificationReason.notRun,
      source = null,
      engine = null,
      checkedAt = null,
      confirmed = 0,
      corrected = 0,
      total = 0,
      matchPercent = null,
      fields = const [];

  factory Verification.fromJson(Map<String, dynamic> json) => Verification(
    status: VerificationStatus.parse(readString(json, 'status')),
    reason: VerificationReason.parse(readStringOrNull(json, 'reason')),
    source: VerificationSource.parse(readStringOrNull(json, 'source')),
    engine: readStringOrNull(json, 'engine'),
    checkedAt: readTimestampOrNull(json, 'checkedAt'),
    confirmed: readInt(json, 'confirmed'),
    corrected: readInt(json, 'corrected'),
    total: readInt(json, 'total'),
    matchPercent: readIntOrNull(json, 'matchPercent'),
    fields: readList(json, 'fields', FieldCheck.fromJson),
  );

  final VerificationStatus status;

  /// Only for `unchecked`, otherwise `null`.
  final VerificationReason? reason;

  /// `null` for `unchecked`.
  final VerificationSource? source;

  /// Programs with version, e.g. `tesseract 5.5.4 deu`. `null` for a text
  /// import and for `unchecked`.
  final String? engine;

  /// Time of the last check in UTC, `null` for `unchecked`.
  final DateTime? checkedAt;

  /// Fields whose current value was found (`confirmed` and
  /// `corrected_confirmed`).
  final int confirmed;

  /// Fields the user corrected.
  final int corrected;

  /// Number of checked fields.
  final int total;

  /// `confirmed / total` in percent; `null` for `unchecked` or `total` 0.
  final int? matchPercent;
  final List<FieldCheck> fields;

  bool get isChecked => status == VerificationStatus.checked;

  /// The check of a field: `documentDate`, `issuer` or `facts.<id>`.
  /// `null` if the field was not checked (e.g. it is `null`).
  FieldCheck? checkFor(String field) =>
      fields.where((f) => f.field == field).firstOrNull;
}

/// The check of one field: `verification.fields[]`.
class FieldCheck {
  const FieldCheck({
    required this.field,
    required this.status,
    required this.source,
    required this.confidence,
    required this.locations,
  });

  factory FieldCheck.fromJson(Map<String, dynamic> json) => FieldCheck(
    field: readString(json, 'field'),
    status: FieldCheckStatus.parse(readString(json, 'status')),
    source: VerificationSource.parse(readStringOrNull(json, 'source')),
    confidence: readDoubleOrNull(json, 'confidence'),
    locations: readList(json, 'locations', Location.fromJson),
  );

  /// `documentDate`, `issuer` or `facts.<id>`.
  final String field;
  final FieldCheckStatus status;

  /// How the field was confirmed; `null` if it was not found.
  final VerificationSource? source;

  /// Lowest OCR confidence (0–100) of the matched words; only for `ocr`.
  final double? confidence;

  /// Where the value is in the original, best match first, at most 3.
  final List<Location> locations;
}

/// Where a value is in the original.
sealed class Location {
  const Location();

  /// A location with `start` is a range in a text import, otherwise a
  /// rectangle on a page.
  factory Location.fromJson(Map<String, dynamic> json) =>
      json.containsKey('start')
      ? TextLocation.fromJson(json)
      : PageLocation.fromJson(json);
}

/// A rectangle on a page of a file (`pdf_text` or `ocr`). All values are
/// fractions of the page width and height (0–1, origin top left), so the
/// app multiplies them by the size at which it shows the page.
class PageLocation extends Location {
  const PageLocation({
    required this.page,
    required this.x,
    required this.y,
    required this.width,
    required this.height,
  });

  factory PageLocation.fromJson(Map<String, dynamic> json) => PageLocation(
    page: readInt(json, 'page'),
    x: readDouble(json, 'x'),
    y: readDouble(json, 'y'),
    width: readDouble(json, 'width'),
    height: readDouble(json, 'height'),
  );

  /// Page, starting at 1. A photo has one page (shown upright).
  final int page;
  final double x;
  final double y;
  final double width;
  final double height;
}

/// A range in `fullText` of a text import. Counted in Unicode code points
/// (`fullText.runes`), [start] inclusive, [end] exclusive.
class TextLocation extends Location {
  const TextLocation({required this.start, required this.end});

  factory TextLocation.fromJson(Map<String, dynamic> json) =>
      TextLocation(start: readInt(json, 'start'), end: readInt(json, 'end'));

  final int start;
  final int end;

  /// The text of this range in [fullText].
  String textIn(String fullText) =>
      String.fromCharCodes(fullText.runes.skip(start).take(end - start));

  /// This range as UTF-16 indices into [fullText], for `substring` and
  /// `TextSelection`. Dart strings count UTF-16 code units, so a character
  /// outside the BMP (e.g. an emoji) is one code point but two units.
  /// Offsets beyond the text are clamped to its end.
  (int, int) utf16Range(String fullText) {
    var units = 0;
    var codePoints = 0;
    int? from;
    for (final rune in fullText.runes) {
      if (codePoints == start) from = units;
      if (codePoints == end) break;
      units += rune > 0xFFFF ? 2 : 1;
      codePoints++;
    }
    from ??= units; // start at or beyond the end of the text
    return (from, units < from ? from : units);
  }
}
