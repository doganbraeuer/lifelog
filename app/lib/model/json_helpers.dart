// Small helpers for reading JSON safely.
//
// Instead of `json['id'] as String` (which throws a TypeError for a wrong
// type), these functions check the type and throw an understandable
// FormatException. The HttpApiClient catches it and reports an unexpected
// response from the server.

import 'plain_date.dart';

Map<String, dynamic> asObject(Object? value, String field) {
  if (value is Map<String, dynamic>) return value;
  throw FormatException('"$field" is not a JSON object');
}

String readString(Map<String, dynamic> json, String field) {
  final value = json[field];
  if (value is String) return value;
  throw FormatException('Field "$field" is missing or not a string');
}

String? readStringOrNull(Map<String, dynamic> json, String field) {
  final value = json[field];
  if (value == null || value is String) return value as String?;
  throw FormatException('Field "$field" is neither a string nor null');
}

int readInt(Map<String, dynamic> json, String field) {
  final value = json[field];
  if (value is int) return value;
  throw FormatException('Field "$field" is missing or not an integer');
}

bool readBool(Map<String, dynamic> json, String field) {
  final value = json[field];
  if (value is bool) return value;
  throw FormatException('Field "$field" is missing or not a boolean');
}

List<T> readList<T>(
  Map<String, dynamic> json,
  String field,
  T Function(Map<String, dynamic>) convert,
) {
  final value = json[field];
  if (value is! List) {
    throw FormatException('Field "$field" is missing or not a list');
  }
  return [for (final element in value) convert(asObject(element, field))];
}

/// Reads a point in time in RFC 3339 format (e.g. `2026-11-03T18:42:07Z`)
/// and returns it in UTC.
DateTime readTimestamp(Map<String, dynamic> json, String field) {
  final value = DateTime.tryParse(readString(json, field));
  if (value == null) {
    throw FormatException('Field "$field" is not a valid point in time');
  }
  return value.toUtc();
}

/// Reads a plain date `YYYY-MM-DD` or `null`.
PlainDate? readDateOrNull(Map<String, dynamic> json, String field) {
  final value = readStringOrNull(json, field);
  if (value == null) return null;
  return PlainDate.tryParse(value) ??
      (throw FormatException('Field "$field" is not a date YYYY-MM-DD', value));
}

/// Writes a point in time as RFC 3339 in UTC without milliseconds,
/// e.g. `2026-11-03T18:42:07Z`, as the backend returns it.
String formatTimestamp(DateTime time) {
  final utc = time.toUtc();
  String two(int n) => n.toString().padLeft(2, '0');
  return '${utc.year.toString().padLeft(4, '0')}-${two(utc.month)}-'
      '${two(utc.day)}T${two(utc.hour)}:${two(utc.minute)}:'
      '${two(utc.second)}Z';
}
