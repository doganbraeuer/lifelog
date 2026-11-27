import 'entry_patch.dart';
import 'json_helpers.dart';
import 'plain_date.dart';

// Routines (docs/API.md 3.16–3.27, names from docs/GLOSSARY.md "Routines").
//
// A routine is like an alarm clock for something done regularly. The
// backend expands its schedule into occurrences, keeps the logs and computes
// status and progress; the app only shows them and plans local reminders.
//
// Routines and logs are health data: no `toString` here prints a name, an
// amount, a unit or the time of a log, so nothing of it ends up in a log or
// a crash output by accident.

/// `schedule.frequency`: how the occurrences repeat.
enum Frequency { hourly, daily, weekly }

/// `schedule.weekdays`. The order is Monday to Sunday, so
/// `Weekday.values[dateTime.weekday - 1]` is the weekday of a date.
enum Weekday { mon, tue, wed, thu, fri, sat, sun }

/// `status` of a routine, computed by the backend for today in the user's
/// time zone.
enum RoutineStatus { upcoming, active, paused, ended }

/// `status` of an occurrence. `open` means "no log and not paused", before
/// or after `dueAt`; the app sees from `dueAt` whether it is overdue.
enum OccurrenceStatus { open, done, skipped, paused }

/// `status` of a log: done or skipped.
enum LogStatus { done, skipped }

/// `source` of a log: in the app, from a reminder, or (stage 2) from a
/// confirmed suggestion on an entry. Not the `sources` of an answer.
enum LogSource { manual, notification, entry }

/// When the occurrences are (3.16 "Schedule"). Times are `"HH:MM"` in the
/// user's time zone, exactly as the backend sends them.
class RoutineSchedule {
  /// Every day from [windowStart] every [interval] hours up to [windowEnd].
  const RoutineSchedule.hourly({
    required this.interval,
    required String this.windowStart,
    required String this.windowEnd,
  }) : frequency = Frequency.hourly,
       times = const [],
       weekdays = const [];

  /// At every time in [times] on every [interval]-th day from the start.
  const RoutineSchedule.daily({required this.interval, required this.times})
    : frequency = Frequency.daily,
      weekdays = const [],
      windowStart = null,
      windowEnd = null;

  /// At every time in [times] on [weekdays] of every [interval]-th week.
  const RoutineSchedule.weekly({
    required this.interval,
    required this.weekdays,
    required this.times,
  }) : frequency = Frequency.weekly,
       windowStart = null,
       windowEnd = null;

  /// Reads only the fields of the frequency; the backend sends no others.
  factory RoutineSchedule.fromJson(Map<String, dynamic> json) {
    final interval = readInt(json, 'interval');
    return switch (readEnum(json, 'frequency', Frequency.values)) {
      Frequency.hourly => RoutineSchedule.hourly(
        interval: interval,
        windowStart: readString(json, 'windowStart'),
        windowEnd: readString(json, 'windowEnd'),
      ),
      Frequency.daily => RoutineSchedule.daily(
        interval: interval,
        times: readStringList(json, 'times'),
      ),
      Frequency.weekly => RoutineSchedule.weekly(
        interval: interval,
        weekdays: [
          for (final day in readStringList(json, 'weekdays'))
            Weekday.values.firstWhere(
              (w) => w.name == day,
              orElse: () =>
                  throw const FormatException('Unknown value in "weekdays"'),
            ),
        ],
        times: readStringList(json, 'times'),
      ),
    };
  }

  final Frequency frequency;

  /// N in „alle N Stunden“, „jeden N. Tag“, „jede N. Woche“.
  final int interval;

  /// `daily` and `weekly`: 1 to 24 different times, sorted.
  final List<String> times;

  /// `weekly`: 1 to 7 different weekdays, Monday first.
  final List<Weekday> weekdays;

  /// `hourly`: start and end of the window; the window does not cross
  /// midnight.
  final String? windowStart;
  final String? windowEnd;

  /// Only the fields of the frequency: a field of another frequency is an
  /// error in the backend, also with `null`.
  Map<String, dynamic> toJson() => {
    'frequency': frequency.name,
    'interval': interval,
    if (frequency == Frequency.hourly) ...{
      'windowStart': windowStart,
      'windowEnd': windowEnd,
    },
    if (frequency == Frequency.weekly)
      'weekdays': [for (final day in weekdays) day.name],
    if (frequency != Frequency.hourly) 'times': times,
  };

  @override
  bool operator ==(Object other) =>
      other is RoutineSchedule &&
      other.frequency == frequency &&
      other.interval == interval &&
      other.windowStart == windowStart &&
      other.windowEnd == windowEnd &&
      _sameList(other.times, times) &&
      _sameList(other.weekdays, weekdays);

  @override
  int get hashCode => Object.hash(
    frequency,
    interval,
    windowStart,
    windowEnd,
    Object.hashAll(times),
    Object.hashAll(weekdays),
  );

  @override
  String toString() => 'RoutineSchedule(${frequency.name})';
}

bool _sameList<T>(List<T> a, List<T> b) {
  if (a.length != b.length) return false;
  for (var i = 0; i < a.length; i++) {
    if (a[i] != b[i]) return false;
  }
  return true;
}

/// A pause: whole days in the user's time zone, both inclusive.
class RoutinePause {
  const RoutinePause({required this.from, this.until});

  factory RoutinePause.fromJson(Map<String, dynamic> json) => RoutinePause(
    from:
        readDateOrNull(json, 'from') ??
        (throw const FormatException('Field "from" is missing')),
    until: readDateOrNull(json, 'until'),
  );

  final PlainDate from;

  /// `null`: paused until resumed.
  final PlainDate? until;

  /// Whether [day] lies in this pause.
  bool includes(PlainDate day) =>
      from.compareTo(day) <= 0 && (until == null || day.compareTo(until!) <= 0);
}

/// Counters of one range of the progress (`today`, `week` or `period`).
class ProgressCounts {
  const ProgressCounts({
    required this.done,
    required this.skipped,
    required this.due,
    this.amountDone,
    this.amountTarget,
    this.day,
    this.days,
  });

  factory ProgressCounts.fromJson(Map<String, dynamic> json) => ProgressCounts(
    done: readInt(json, 'done'),
    skipped: readInt(json, 'skipped'),
    due: readInt(json, 'due'),
    amountDone: readDoubleOrNull(json, 'amountDone'),
    amountTarget: readDoubleOrNull(json, 'amountTarget'),
    day: readIntOrNull(json, 'day'),
    days: readIntOrNull(json, 'days'),
  );

  /// Occurrences of the range with a log `done` or `skipped`.
  final int done;
  final int skipped;

  /// Occurrences of the range, also later ones today, without paused ones
  /// (unless they have a log). „heute 1 von 2“ is [done] of [due].
  final int due;

  /// Sum of the amounts of all `done` logs, also of logs without an
  /// occurrence; `null` if the routine has no amount.
  final double? amountDone;

  /// Amount × [due]; `null` if the routine has no amount.
  final double? amountTarget;

  /// Only in `period`: day of the period (1 = start day) and the number of
  /// days, „Tag 6 von 14“.
  final int? day;
  final int? days;
}

/// `progress` of a routine, computed at the time of the request.
class RoutineProgress {
  const RoutineProgress({required this.today, required this.week, this.period});

  factory RoutineProgress.fromJson(Map<String, dynamic> json) =>
      RoutineProgress(
        today: ProgressCounts.fromJson(asObject(json['today'], 'today')),
        week: ProgressCounts.fromJson(asObject(json['week'], 'week')),
        period: json['period'] == null
            ? null
            : ProgressCounts.fromJson(asObject(json['period'], 'period')),
      );

  final ProgressCounts today;

  /// The current week, Monday to Sunday.
  final ProgressCounts week;

  /// From the start to the end date; `null` if the routine has no end.
  final ProgressCounts? period;
}

/// A routine as every routine endpoint returns it (3.16 "The routine").
class Routine {
  const Routine({
    required this.id,
    required this.module,
    required this.name,
    required this.amount,
    required this.unit,
    required this.schedule,
    required this.startDate,
    required this.endDate,
    required this.status,
    required this.pauses,
    required this.createdAt,
    required this.progress,
  });

  factory Routine.fromJson(Map<String, dynamic> json) => Routine(
    id: readString(json, 'id'),
    module: readString(json, 'module'),
    name: readString(json, 'name'),
    amount: readDoubleOrNull(json, 'amount'),
    unit: readStringOrNull(json, 'unit'),
    schedule: RoutineSchedule.fromJson(asObject(json['schedule'], 'schedule')),
    startDate:
        readDateOrNull(json, 'startDate') ??
        (throw const FormatException('Field "startDate" is missing')),
    endDate: readDateOrNull(json, 'endDate'),
    status: readEnum(json, 'status', RoutineStatus.values),
    pauses: readList(json, 'pauses', RoutinePause.fromJson),
    createdAt: readTimestamp(json, 'createdAt'),
    progress: RoutineProgress.fromJson(asObject(json['progress'], 'progress')),
  );

  final String id;
  final String module;
  final String name;

  /// Target amount per occurrence, e.g. 2 or 0.5; `null`: no amount.
  final double? amount;

  /// Unit of [amount], e.g. „Stück“; only with an amount, and an amount
  /// without a unit is allowed (20 push-ups).
  final String? unit;
  final RoutineSchedule schedule;
  final PlainDate startDate;

  /// Last day, inclusive; `null`: no end.
  final PlainDate? endDate;
  final RoutineStatus status;

  /// All pauses, oldest first.
  final List<RoutinePause> pauses;
  final DateTime createdAt;
  final RoutineProgress progress;

  bool get hasAmount => amount != null;

  /// The pause that includes [day] and lasts longest, or `null`. After a
  /// change of the time zone two pauses can overlap (3.16 "Pauses").
  RoutinePause? pauseOn(PlainDate day) {
    RoutinePause? found;
    for (final pause in pauses) {
      if (!pause.includes(day)) continue;
      if (found == null ||
          pause.until == null ||
          (found.until != null && pause.until!.compareTo(found.until!) > 0)) {
        found = pause;
      }
    }
    return found;
  }

  @override
  String toString() => 'Routine($id)';
}

/// An occurrence („Termin“) as `GET /routines/occurrences` returns it.
class Occurrence {
  const Occurrence({
    required this.routineId,
    required this.dueAt,
    required this.date,
    required this.time,
    required this.status,
    required this.logId,
  });

  factory Occurrence.fromJson(Map<String, dynamic> json) => Occurrence(
    routineId: readString(json, 'routineId'),
    dueAt: readTimestamp(json, 'dueAt'),
    date:
        readDateOrNull(json, 'date') ??
        (throw const FormatException('Field "date" is missing')),
    time: readString(json, 'time'),
    status: readEnum(json, 'status', OccurrenceStatus.values),
    logId: readStringOrNull(json, 'logId'),
  );

  final String routineId;

  /// Point in time, UTC.
  final DateTime dueAt;

  /// Local date and time of [dueAt] in the user's time zone, so the app
  /// does not convert. After a forward shift [time] is the shifted time.
  final PlainDate date;
  final String time;
  final OccurrenceStatus status;

  /// The log for `done` and `skipped`, otherwise `null`.
  final String? logId;

  bool get isOpen => status == OccurrenceStatus.open;

  /// Open and its point in time has passed.
  bool isOverdue(DateTime now) => isOpen && dueAt.isBefore(now);

  @override
  String toString() => 'Occurrence($routineId)';
}

/// Response of `GET /routines/occurrences`. [from] is "today" in the user's
/// time zone when the request named no `from`.
class OccurrenceList {
  const OccurrenceList({
    required this.from,
    required this.to,
    required this.occurrences,
  });

  factory OccurrenceList.fromJson(Map<String, dynamic> json) => OccurrenceList(
    from:
        readDateOrNull(json, 'from') ??
        (throw const FormatException('Field "from" is missing')),
    to:
        readDateOrNull(json, 'to') ??
        (throw const FormatException('Field "to" is missing')),
    occurrences: readList(json, 'occurrences', Occurrence.fromJson),
  );

  final PlainDate from;
  final PlainDate to;

  /// Sorted by `dueAt`, then `routineId`, in every status.
  final List<Occurrence> occurrences;

  /// The occurrences of one routine, in the order of the list.
  List<Occurrence> of(String routineId) => [
    for (final o in occurrences)
      if (o.routineId == routineId) o,
  ];
}

/// A log („Verlaufseintrag“, never just „Eintrag“: that is a document).
class RoutineLog {
  const RoutineLog({
    required this.id,
    required this.routineId,
    required this.dueAt,
    required this.takenAt,
    required this.status,
    required this.amount,
    required this.source,
    required this.entryId,
    required this.timeZone,
    required this.createdAt,
  });

  factory RoutineLog.fromJson(Map<String, dynamic> json) => RoutineLog(
    id: readString(json, 'id'),
    routineId: readString(json, 'routineId'),
    dueAt: readTimestampOrNull(json, 'dueAt'),
    takenAt: readTimestamp(json, 'takenAt'),
    status: readEnum(json, 'status', LogStatus.values),
    amount: readDoubleOrNull(json, 'amount'),
    source: readEnum(json, 'source', LogSource.values),
    entryId: readStringOrNull(json, 'entryId'),
    timeZone: readString(json, 'timeZone'),
    createdAt: readTimestamp(json, 'createdAt'),
  );

  final String id;
  final String routineId;

  /// The occurrence; `null` without one (an extra glass, a portion).
  final DateTime? dueAt;

  /// When it was done or skipped, UTC.
  final DateTime takenAt;
  final LogStatus status;

  /// Amount taken; always `null` for `skipped`.
  final double? amount;
  final LogSource source;
  final String? entryId;

  /// The user's time zone when the log was created.
  final String timeZone;
  final DateTime createdAt;

  @override
  String toString() => 'RoutineLog($id)';
}

// -------------------------------------------------------------------------
// Requests
// -------------------------------------------------------------------------

/// Body of `POST /routines` (3.18).
class NewRoutine {
  const NewRoutine({
    required this.module,
    required this.name,
    this.amount,
    this.unit,
    required this.schedule,
    required this.startDate,
    this.endDate,
  });

  /// Always sent, so that no area ends up in health by accident.
  final String module;
  final String name;
  final double? amount;
  final String? unit;
  final RoutineSchedule schedule;
  final PlainDate startDate;
  final PlainDate? endDate;

  Map<String, dynamic> toJson() => {
    'module': module,
    'name': name,
    if (amount != null) 'amount': jsonNumber(amount!),
    if (unit != null) 'unit': unit,
    'schedule': schedule.toJson(),
    'startDate': startDate.iso,
    if (endDate != null) 'endDate': endDate!.iso,
  };

  @override
  String toString() => 'NewRoutine(***)';
}

/// Body of `PATCH /routines/{id}` (3.20): only the fields that change. A
/// field that is `null` here is missing from the JSON and stays unchanged;
/// `Value(null)` sends `null` and clears it.
class RoutinePatch {
  const RoutinePatch({
    this.name,
    this.amount,
    this.unit,
    this.schedule,
    this.startDate,
    this.endDate,
  });

  /// The difference between [routine] and the edited form [edited]. Equal
  /// values are left out, so an unchanged form gives an empty patch.
  factory RoutinePatch.between(Routine routine, NewRoutine edited) {
    final amountChanged = edited.amount != routine.amount;
    // `amount: null` removes the unit too (3.20), so it is not sent again.
    final unitChanged =
        edited.unit != routine.unit &&
        !(amountChanged && edited.amount == null);
    return RoutinePatch(
      name: edited.name != routine.name ? edited.name : null,
      amount: amountChanged ? Value(edited.amount) : null,
      unit: unitChanged ? Value(edited.unit) : null,
      schedule: edited.schedule != routine.schedule ? edited.schedule : null,
      startDate: edited.startDate != routine.startDate
          ? edited.startDate
          : null,
      endDate: edited.endDate != routine.endDate ? Value(edited.endDate) : null,
    );
  }

  /// `null` is not allowed for the name, so a plain `String?` is enough.
  final String? name;
  final Value<double?>? amount;
  final Value<String?>? unit;

  /// Replaced as a whole, never merged.
  final RoutineSchedule? schedule;
  final PlainDate? startDate;
  final Value<PlainDate?>? endDate;

  bool get isEmpty =>
      name == null &&
      amount == null &&
      unit == null &&
      schedule == null &&
      startDate == null &&
      endDate == null;

  Map<String, dynamic> toJson() => {
    if (name != null) 'name': name,
    if (amount case final amount?)
      'amount': amount.value == null ? null : jsonNumber(amount.value!),
    if (unit case final unit?) 'unit': unit.value,
    if (schedule != null) 'schedule': schedule!.toJson(),
    if (startDate != null) 'startDate': startDate!.iso,
    if (endDate case final endDate?) 'endDate': endDate.value?.iso,
  };

  /// Only the names of the changed fields, like the backend's log.
  @override
  String toString() => 'RoutinePatch(${toJson().keys.join(', ')})';
}

/// Body of `POST /routines/{id}/logs` (3.26). There is deliberately no
/// `takenAt`: the server sets it; the phone's clock is never sent as "now".
class NewRoutineLog {
  const NewRoutineLog({
    required this.status,
    required this.source,
    this.dueAt,
    this.amount,
  });

  final LogStatus status;

  /// `manual` or `notification`; `entry` only arises through stage 2.
  final LogSource source;

  /// The occurrence as `dueAt` from 3.24; `null`: without an occurrence.
  final DateTime? dueAt;

  /// Only for `done`; `null`: the routine's amount.
  final double? amount;

  Map<String, dynamic> toJson() => {
    'status': status.name,
    'source': source.name,
    if (dueAt != null) 'dueAt': formatTimestamp(dueAt!),
    if (amount != null) 'amount': jsonNumber(amount!),
  };

  @override
  String toString() => 'NewRoutineLog(${status.name}, ${source.name})';
}

/// A whole number as an `int` (`2` instead of `2.0`), anything else as it
/// is. JSON does not distinguish them, but the requests read more naturally
/// and match the examples in docs/API.md.
num jsonNumber(double value) =>
    value == value.roundToDouble() && value.abs() < 1e15
    ? value.toInt()
    : value;
