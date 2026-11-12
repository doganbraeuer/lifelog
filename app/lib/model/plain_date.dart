/// A plain calendar date without time and without time zone.
///
/// Used for `documentDate`. A report from 14 October should always appear
/// on 14 October. With `DateTime` the date could shift by one day depending
/// on the device's time zone, so only year, month and day are stored.
class PlainDate implements Comparable<PlainDate> {
  const PlainDate._(this.year, this.month, this.day);

  /// Creates a date and checks that it exists in the calendar.
  /// `PlainDate(2026, 2, 30)` throws an [ArgumentError].
  factory PlainDate(int year, int month, int day) {
    if (!isValid(year, month, day)) {
      throw ArgumentError('Not a valid date: $year-$month-$day');
    }
    return PlainDate._(year, month, day);
  }

  /// Takes year, month and day from a [DateTime] as they are, without
  /// converting to another time zone. Meant for the date picker, which
  /// returns a local `DateTime`.
  factory PlainDate.fromDateTime(DateTime time) =>
      PlainDate(time.year, time.month, time.day);

  final int year;
  final int month;
  final int day;

  /// Exactly `YYYY-MM-DD`, as defined in docs/API.md.
  static final format = RegExp(r'^(\d{4})-(\d{2})-(\d{2})$');

  /// Reads `YYYY-MM-DD`. Returns `null` if the format is wrong or the date
  /// does not exist in the calendar (e.g. `2026-02-30`).
  static PlainDate? tryParse(String text) {
    final match = format.firstMatch(text);
    if (match == null) return null;
    final year = int.parse(match.group(1)!);
    final month = int.parse(match.group(2)!);
    final day = int.parse(match.group(3)!);
    if (!isValid(year, month, day)) return null;
    return PlainDate._(year, month, day);
  }

  /// Like [tryParse], but throws a [FormatException].
  static PlainDate parse(String text) =>
      tryParse(text) ??
      (throw FormatException(
        'Not a valid date in the format YYYY-MM-DD',
        text,
      ));

  static bool isValid(int year, int month, int day) {
    if (year < 1 || year > 9999) return false;
    if (month < 1 || month > 12) return false;
    return day >= 1 && day <= daysInMonth(year, month);
  }

  static int daysInMonth(int year, int month) {
    const days = [31, 28, 31, 30, 31, 30, 31, 31, 30, 31, 30, 31];
    if (month == 2 && isLeapYear(year)) return 29;
    return days[month - 1];
  }

  static bool isLeapYear(int year) =>
      (year % 4 == 0 && year % 100 != 0) || year % 400 == 0;

  /// Format for the API: `2026-10-14`.
  String get iso =>
      '${_padded(year, 4)}-${_padded(month, 2)}-'
      '${_padded(day, 2)}';

  /// Format for display: `14.10.2026`.
  String get display =>
      '${_padded(day, 2)}.${_padded(month, 2)}.'
      '${_padded(year, 4)}';

  /// Local midnight of this day. Only meant for widgets like the date picker
  /// that require a `DateTime`. [PlainDate.fromDateTime] converts back.
  DateTime toDateTime() => DateTime(year, month, day);

  static String _padded(int number, int digits) =>
      number.toString().padLeft(digits, '0');

  @override
  int compareTo(PlainDate other) {
    if (year != other.year) return year.compareTo(other.year);
    if (month != other.month) return month.compareTo(other.month);
    return day.compareTo(other.day);
  }

  @override
  bool operator ==(Object other) =>
      other is PlainDate &&
      other.year == year &&
      other.month == month &&
      other.day == day;

  @override
  int get hashCode => Object.hash(year, month, day);

  @override
  String toString() => iso;
}
