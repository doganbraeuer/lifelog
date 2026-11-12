import 'dart:typed_data';

import 'json_helpers.dart';
import 'plain_date.dart';

/// Short form of an entry for the timeline (docs/API.md, section 2).
class EntrySummary {
  const EntrySummary({
    required this.id,
    required this.module,
    required this.type,
    required this.documentDate,
    required this.importedAt,
    required this.issuer,
  });

  factory EntrySummary.fromJson(Map<String, dynamic> json) => EntrySummary(
    id: readString(json, 'id'),
    module: readString(json, 'module'),
    type: readString(json, 'type'),
    documentDate: readDateOrNull(json, 'documentDate'),
    importedAt: readTimestamp(json, 'importedAt'),
    issuer: readStringOrNull(json, 'issuer'),
  );

  final String id;
  final String module;
  final String type;

  /// `null` if the vision LLM recognized no date.
  final PlainDate? documentDate;

  /// Time of the upload in UTC.
  final DateTime importedAt;

  /// Issuing body, `null` if not recognized.
  final String? issuer;
}

/// Full entry with full text, facts, file and corrections.
class Entry extends EntrySummary {
  const Entry({
    required super.id,
    required super.module,
    required super.type,
    required super.documentDate,
    required super.importedAt,
    required super.issuer,
    required this.fullText,
    required this.facts,
    required this.file,
    required this.corrections,
  });

  factory Entry.fromJson(Map<String, dynamic> json) {
    final summary = EntrySummary.fromJson(json);
    return Entry(
      id: summary.id,
      module: summary.module,
      type: summary.type,
      documentDate: summary.documentDate,
      importedAt: summary.importedAt,
      issuer: summary.issuer,
      fullText: readString(json, 'fullText'),
      facts: readList(json, 'facts', Fact.fromJson),
      file: File.fromJson(asObject(json['file'], 'file')),
      corrections: readList(json, 'corrections', Correction.fromJson),
    );
  }

  final String fullText;
  final List<Fact> facts;
  final File file;
  final List<Correction> corrections;

  /// The correction for a field path like `documentDate` or
  /// `facts.f2.value`, or `null` if the field was not corrected.
  Correction? correctionFor(String field) =>
      corrections.where((c) => c.field == field).firstOrNull;

  /// Rule 4 from docs/API.md: a fact counts as corrected if a `field` in
  /// `corrections` starts with `facts.<id>.`.
  bool isFactCorrected(String factId) =>
      corrections.any((c) => c.field.startsWith('facts.$factId.'));
}

/// An extracted fact, e.g. Hämoglobin 14,2 g/dl.
class Fact {
  const Fact({
    required this.id,
    required this.name,
    required this.value,
    this.unit,
  });

  factory Fact.fromJson(Map<String, dynamic> json) => Fact(
    id: readString(json, 'id'),
    name: readString(json, 'name'),
    value: readString(json, 'value'),
    unit: readStringOrNull(json, 'unit'),
  );

  final String id;
  final String name;

  /// Always a string, exactly as on the document (`"14,2"`, `"<0,5"`).
  final String value;
  final String? unit;
}

/// Reference to the original file.
class File {
  const File({
    required this.id,
    required this.fileName,
    required this.mimeType,
    required this.size,
    required this.sha256,
  });

  factory File.fromJson(Map<String, dynamic> json) => File(
    id: readString(json, 'id'),
    fileName: readString(json, 'fileName'),
    mimeType: readString(json, 'mimeType'),
    size: readInt(json, 'size'),
    sha256: readString(json, 'sha256'),
  );

  final String id;
  final String fileName;
  final String mimeType;

  /// Size in bytes.
  final int size;
  final String sha256;

  bool get isPdf => mediaType(mimeType) == 'application/pdf';
  bool get isImage => mediaType(mimeType).startsWith('image/');

  /// Text, e.g. `text.txt` from a text import (`text/plain`, also with
  /// `; charset=utf-8`).
  bool get isText => mediaType(mimeType) == 'text/plain';
}

/// The bare media type without parameters, in lower case:
/// `Text/Plain; charset=UTF-8` → `text/plain`.
String mediaType(String mimeType) =>
    mimeType.split(';').first.trim().toLowerCase();

/// A correction: originally recognized and current value.
class Correction {
  const Correction({
    required this.field,
    required this.recognized,
    required this.corrected,
    required this.correctedAt,
  });

  factory Correction.fromJson(Map<String, dynamic> json) => Correction(
    field: readString(json, 'field'),
    recognized: readStringOrNull(json, 'recognized'),
    corrected: readStringOrNull(json, 'corrected'),
    correctedAt: readTimestamp(json, 'correctedAt'),
  );

  /// Path like `type`, `documentDate`, `issuer`, `facts.f2.value`.
  final String field;
  final String? recognized;
  final String? corrected;
  final DateTime correctedAt;
}

/// Content of the original file as returned by `GET /entries/{id}/file`.
class FileContent {
  const FileContent({required this.bytes, required this.mimeType});

  final Uint8List bytes;
  final String mimeType;

  bool get isPdf => mediaType(mimeType) == 'application/pdf';
  bool get isImage => mediaType(mimeType).startsWith('image/');
  bool get isText => mediaType(mimeType) == 'text/plain';
}
