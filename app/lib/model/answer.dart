import 'json_helpers.dart';
import 'plain_date.dart';

/// Response to `POST /api/v1/questions` (docs/API.md, section 3.6).
class Answer {
  const Answer({
    required this.answer,
    required this.found,
    required this.sources,
  });

  factory Answer.fromJson(Map<String, dynamic> json) => Answer(
    answer: readString(json, 'answer'),
    found: readBool(json, 'found'),
    sources: readList(json, 'sources', Source.fromJson),
  );

  /// Answer text. References like `[1]` point to [Source.number].
  final String answer;

  /// `false` if no matching chunk was found.
  final bool found;
  final List<Source> sources;
}

/// A chunk the answer is based on.
class Source {
  const Source({
    required this.number,
    required this.entryId,
    required this.module,
    required this.type,
    required this.issuer,
    required this.documentDate,
    required this.excerpt,
  });

  factory Source.fromJson(Map<String, dynamic> json) => Source(
    number: readInt(json, 'number'),
    entryId: readString(json, 'entryId'),
    module: readString(json, 'module'),
    type: readString(json, 'type'),
    issuer: readStringOrNull(json, 'issuer'),
    documentDate: readDateOrNull(json, 'documentDate'),
    excerpt: readString(json, 'excerpt'),
  );

  /// Matches the reference `[number]` in the answer text. Not necessarily
  /// without gaps (e.g. only 1 and 3), because only cited chunks are
  /// returned.
  final int number;
  final String entryId;

  /// Module of the entry. A `type` is only unique together with it.
  final String module;
  final String type;
  final String? issuer;
  final PlainDate? documentDate;
  final String excerpt;
}
