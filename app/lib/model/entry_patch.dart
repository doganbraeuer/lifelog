import 'plain_date.dart';

/// Wrapper for a value that may also be `null`.
///
/// This lets a PATCH tell three cases apart:
/// - The field is `null` (no wrapper): the field is missing from the JSON
///   and stays unchanged.
/// - `Value(null)`: the JSON contains `"field": null`, the value is cleared.
/// - `Value(x)`: the JSON contains `"field": x`.
class Value<T> {
  const Value(this.value);

  final T value;
}

/// Change to a fact. `id` is required.
class FactPatch {
  const FactPatch(this.id, {this.value, this.unit});

  final String id;

  /// New value. `null` means unchanged. (A value cannot be cleared; according
  /// to the API it is always a string.)
  final String? value;

  /// `null`: unchanged. `Value(null)`: remove the unit.
  final Value<String?>? unit;

  Map<String, dynamic> toJson() => {
    'id': id,
    if (value != null) 'value': value,
    if (unit != null) 'unit': unit?.value,
  };
}

/// Body for `PATCH /api/v1/entries/{id}` (docs/API.md, section 3.5).
/// It only contains the fields that should change.
class EntryPatch {
  const EntryPatch({
    this.type,
    this.documentDate,
    this.issuer,
    this.facts = const [],
  });

  /// `null`: unchanged.
  final String? type;

  /// `null`: unchanged. `Value(null)`: remove the date.
  final Value<PlainDate?>? documentDate;

  /// `null`: unchanged. `Value(null)`: remove the issuer.
  final Value<String?>? issuer;

  final List<FactPatch> facts;

  bool get isEmpty =>
      type == null && documentDate == null && issuer == null && facts.isEmpty;

  Map<String, dynamic> toJson() => {
    if (type != null) 'type': type,
    if (documentDate != null) 'documentDate': documentDate?.value?.iso,
    if (issuer != null) 'issuer': issuer?.value,
    if (facts.isNotEmpty) 'facts': [for (final f in facts) f.toJson()],
  };
}
