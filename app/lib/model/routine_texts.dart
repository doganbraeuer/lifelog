import 'plain_date.dart';
import 'routine.dart';

// German display texts for routines, as pure functions (docs/GLOSSARY.md,
// "Routines": „täglich“, „jeden 2. Tag“, „heute 1 von 2“, „Tag 6 von 14“,
// „heute 1.250 von 3.250 ml“). Times and dates come from the backend in the
// user's time zone; nothing here converts a time zone.

/// „Mo“ … „So“.
const weekdayLabels = {
  Weekday.mon: 'Mo',
  Weekday.tue: 'Di',
  Weekday.wed: 'Mi',
  Weekday.thu: 'Do',
  Weekday.fri: 'Fr',
  Weekday.sat: 'Sa',
  Weekday.sun: 'So',
};

/// Status of a routine: „geplant“, „aktiv“, „pausiert“, „beendet“.
String routineStatusLabel(RoutineStatus status) => switch (status) {
  RoutineStatus.upcoming => 'geplant',
  RoutineStatus.active => 'aktiv',
  RoutineStatus.paused => 'pausiert',
  RoutineStatus.ended => 'beendet',
};

/// Status of an occurrence: „offen“, „erledigt“, „ausgelassen“, „pausiert“.
String occurrenceStatusLabel(OccurrenceStatus status) => switch (status) {
  OccurrenceStatus.open => 'offen',
  OccurrenceStatus.done => 'erledigt',
  OccurrenceStatus.skipped => 'ausgelassen',
  OccurrenceStatus.paused => 'pausiert',
};

/// „a“, „a und b“, „a, b und c“.
String germanList(List<String> items) => switch (items.length) {
  0 => '',
  1 => items.single,
  _ => '${items.sublist(0, items.length - 1).join(', ')} und ${items.last}',
};

/// The interval in words, for the form: „jede Stunde“, „alle 2 Stunden“,
/// „jeden Tag“, „jeden 2. Tag“, „jede Woche“, „jede 2. Woche“.
String intervalLabel(Frequency frequency, int interval) => switch (frequency) {
  Frequency.hourly => interval == 1 ? 'jede Stunde' : 'alle $interval Stunden',
  Frequency.daily => interval == 1 ? 'jeden Tag' : 'jeden $interval. Tag',
  Frequency.weekly => interval == 1 ? 'jede Woche' : 'jede $interval. Woche',
};

/// The schedule in one line (3.16: „the app can explain it in one line“):
/// „täglich um 08:00 und 20:00“, „jeden 2. Tag um 08:00“, „stündlich von
/// 08:00 bis 20:00“, „alle 3 Stunden von 08:00 bis 20:00“, „wöchentlich am Di
/// und Fr um 07:00“, „jede 2. Woche am Di und Fr um 07:00“.
String describeSchedule(RoutineSchedule schedule) {
  final n = schedule.interval;
  final times = 'um ${germanList(schedule.times)}';
  return switch (schedule.frequency) {
    Frequency.hourly =>
      '${n == 1 ? 'stündlich' : 'alle $n Stunden'} '
          'von ${schedule.windowStart} bis ${schedule.windowEnd}',
    Frequency.daily => '${n == 1 ? 'täglich' : 'jeden $n. Tag'} $times',
    Frequency.weekly =>
      '${n == 1 ? 'wöchentlich' : 'jede $n. Woche'} am '
          '${germanList([for (final d in schedule.weekdays) weekdayLabels[d]!])}'
          ' $times',
  };
}

/// The schedule with start and end for the preview in the form, e.g.
/// „Täglich um 08:00 und 20:00, bis 01.12.2026“. The start is named only if
/// it is not [today].
String describeRoutinePlan(
  RoutineSchedule schedule,
  PlainDate start,
  PlainDate? end, {
  required PlainDate today,
}) {
  final parts = [
    capitalize(describeSchedule(schedule)),
    if (start != today) 'ab ${start.display}',
    if (end != null) 'bis ${end.display}',
  ];
  return parts.join(', ');
}

String capitalize(String text) =>
    text.isEmpty ? text : text[0].toUpperCase() + text.substring(1);

/// An amount in German notation: 1250 → „1.250“, 0.5 → „0,5“, 2 → „2“. At
/// most 6 decimal places, like the backend's rounding.
String formatAmount(double value) {
  final fixed = value.abs().toStringAsFixed(6);
  final point = fixed.indexOf('.');
  final whole = fixed.substring(0, point);
  final fraction = fixed.substring(point + 1).replaceFirst(RegExp(r'0+$'), '');
  final grouped = StringBuffer();
  for (var i = 0; i < whole.length; i++) {
    if (i > 0 && (whole.length - i) % 3 == 0) grouped.write('.');
    grouped.write(whole[i]);
  }
  final sign = value < 0 && (whole != '0' || fraction.isNotEmpty) ? '-' : '';
  return fraction.isEmpty ? '$sign$grouped' : '$sign$grouped,$fraction';
}

/// The amount with its unit: „2 Stück“, „250 ml“, „20“ without a unit.
String amountWithUnit(double amount, String? unit) =>
    unit == null ? formatAmount(amount) : '${formatAmount(amount)} $unit';

final _plainNumber = RegExp(r'^\d+([.,]\d+)?$');
final _germanGrouped = RegExp(r'^[1-9]\d{0,2}(\.\d{3})+(,\d+)?$');

/// Reads an amount as typed in German or English: „2“, „0,5“, „0.5“,
/// „1.250“ (thousands), „1.250,5“. `null` if it is not a number.
///
/// A point followed by exactly three digits after a first group without a
/// leading zero is a thousands separator, as written in German („1.250 ml“);
/// otherwise a point is a decimal point („0.5“, „1.25“).
double? parseAmount(String text) {
  final value = text.trim().replaceAll(' ', '');
  if (_germanGrouped.hasMatch(value)) {
    return double.parse(value.replaceAll('.', '').replaceAll(',', '.'));
  }
  if (_plainNumber.hasMatch(value)) {
    return double.parse(value.replaceAll(',', '.'));
  }
  return null;
}

/// „heute 1 von 2“, „Woche 5 von 7“.
String countText(String label, ProgressCounts counts) =>
    '$label ${counts.done} von ${counts.due}';

/// „heute 1.250 von 3.250 ml“; `null` if the routine has no amount.
String? amountText(String label, ProgressCounts counts, String? unit) {
  final done = counts.amountDone;
  final target = counts.amountTarget;
  if (done == null || target == null) return null;
  final suffix = unit == null ? '' : ' $unit';
  return '$label ${formatAmount(done)} von ${formatAmount(target)}$suffix';
}

/// „Tag 6 von 14“ for the period.
String periodDayText(ProgressCounts period) =>
    'Tag ${period.day ?? 0} von ${period.days ?? 0}';

/// A date with its weekday, „Fr, 27.11.“. The weekday of a plain date does
/// not depend on a time zone.
String weekdayDate(PlainDate date) {
  final weekday =
      Weekday.values[DateTime.utc(date.year, date.month, date.day).weekday - 1];
  final day = date.day.toString().padLeft(2, '0');
  final month = date.month.toString().padLeft(2, '0');
  return '${weekdayLabels[weekday]}, $day.$month.';
}

/// When an occurrence is, relative to [today]: „08:00“ today, „morgen,
/// 08:00“, otherwise „Fr, 27.11., 08:00“.
String occurrenceWhen(Occurrence occurrence, PlainDate today) {
  // DateTime.utc normalizes day 32 to the next month; no time zone involved.
  final tomorrow = PlainDate.fromDateTime(
    DateTime.utc(today.year, today.month, today.day + 1),
  );
  if (occurrence.date == today) return occurrence.time;
  if (occurrence.date == tomorrow) return 'morgen, ${occurrence.time}';
  return '${weekdayDate(occurrence.date)}, ${occurrence.time}';
}
