import 'package:timezone/timezone.dart' as tz;

import '../../model/json_helpers.dart' show formatTimestamp;
import '../../model/plain_date.dart';
import '../../model/routine.dart';
import '../api_exception.dart';
import 'mock_time_zones.dart';

// Routines in the mock (docs/API.md 3.16–3.27).
//
// In the real app the backend expands schedules and computes status and
// progress; the app never does (3.16, "Decision: the backend computes
// occurrences, status and progress"). The mock has to do it anyway, because
// it stands in for the backend. The rules are ported from the backend
// (backend/internal/core/routine.go, routine_progress.go, routine_range.go,
// schedule.go), with the same messages and the same order of the errors,
// and are checked against backend/internal/core/testdata/
// routine_occurrences.json, the cases of the backend's own test.
//
// Dates are day numbers (days since 1970-01-01) and times minutes after
// midnight, as in the backend; points in time are milliseconds since the
// epoch, UTC.

const _dayMs = 24 * 60 * 60 * 1000;
const _minuteMs = 60 * 1000;

/// Limits of 3.16 "Limits and validation".
const maxRoutinesPerUser = 50;
const _maxNameChars = 60;
const _maxAmount = 100000;
const _maxUnitChars = 20;
const _maxTimesPerDay = 24;
const _maxPauses = 100;
const _maxLogsPerDay = 100;
const _maxOccurrenceDays = 31;
const _maxLogDays = 366;
const _logFutureTolerance = Duration(minutes: 5);
final _minDay = dayNumber(PlainDate(2000, 1, 1));
final _maxDay = dayNumber(PlainDate(2099, 12, 31));

/// Stands for "no end" (a pause until resumed, a routine without endDate).
const _farFuture = 1 << 30;

// Messages (API.md 3.18, 3.20, 3.21, 3.24, 3.26), German like the backend's.
const _msgRoutineBody = 'Bitte die Routine als JSON-Objekt schicken.';
const _msgPatchBody = 'Bitte die Änderung als JSON-Objekt schicken.';
const _msgEmptyPatch = 'Bitte mindestens ein Feld zum Ändern schicken.';
const _msgModule = 'Unbekanntes Modul. Im Prototyp gibt es nur health.';
const _msgName = 'Der Name muss 1 bis 60 Zeichen lang sein.';
const _msgAmount = 'Die Menge muss größer als 0 und höchstens 100.000 sein.';
const _msgUnitNeedsAmount = 'Eine Einheit braucht eine Menge.';
const _msgUnitLength = 'Die Einheit darf höchstens 20 Zeichen lang sein.';
const _msgUnitType = 'Die Einheit muss ein Text sein.';
const _msgSchedule = 'Bitte einen Rhythmus angeben.';
const _msgFrequency = 'Unbekannter Rhythmus.';
const _msgForeignField = 'Dieses Feld passt nicht zum gewählten Rhythmus.';
const _msgTimeFormat = 'Uhrzeiten bitte im Format HH:MM angeben, z. B. 08:00.';
const _msgTimesEmpty = 'Bitte mindestens eine Uhrzeit angeben.';
const _msgTimesMax = 'Höchstens 24 Uhrzeiten pro Tag.';
const _msgTimesDuplicate = 'Jede Uhrzeit nur einmal.';
const _msgWindowOrder =
    'Das Ende des Zeitfensters muss nach dem Beginn liegen.';
const _msgWeekdaysEmpty = 'Bitte mindestens einen Wochentag wählen.';
const _msgWeekdayUnknown = 'Unbekannter Wochentag.';
const _msgWeekdayDuplicate = 'Jeden Wochentag nur einmal.';
const _msgDateFormat = 'Das Datum muss im Format JJJJ-MM-TT angegeben werden.';
const _msgDateRange = 'Das Datum muss zwischen 2000 und 2099 liegen.';
const _msgEndBeforeStart = 'Das Enddatum darf nicht vor dem Startdatum liegen.';
const _msgStartAfterEnd = 'Das Startdatum darf nicht nach dem Enddatum liegen.';
const _msgPauseBody = 'Bitte die Pause als JSON-Objekt schicken.';
const _msgPauseUntil = 'Das Ende der Pause darf nicht vor heute liegen.';
const _msgLogBody = 'Bitte den Verlaufseintrag als JSON-Objekt schicken.';
const _msgLogStatus = 'Unbekannter Status.';
const _msgLogSource = 'Unbekannte Herkunft.';
const _msgInstant = 'Bitte einen gültigen Zeitpunkt angeben.';
const _msgNoOccurrence =
    'Zu diesem Zeitpunkt ist kein Termin der Routine geplant.';
const _msgFutureOccurrence =
    'Termine in der Zukunft können noch nicht eingetragen werden.';
const _msgFutureTakenAt = 'Der Zeitpunkt darf nicht in der Zukunft liegen.';
const _msgSkippedAmount = 'Ausgelassene Termine haben keine Menge.';
const _msgRangeReversed =
    'Das Ende des Zeitraums darf nicht vor dem Anfang liegen.';

const _routineCreateFields = [
  'amount',
  'endDate',
  'module',
  'name',
  'schedule',
  'startDate',
  'unit',
];
const _routinePatchFields = [
  'amount',
  'endDate',
  'name',
  'schedule',
  'startDate',
  'unit',
];
const _routineReadOnlyFields = [
  'createdAt',
  'id',
  'module',
  'pauses',
  'progress',
  'status',
];
const _scheduleFields = [
  'frequency',
  'interval',
  'times',
  'weekdays',
  'windowEnd',
  'windowStart',
];
const _scheduleFieldsOf = {
  'hourly': ['frequency', 'interval', 'windowEnd', 'windowStart'],
  'daily': ['frequency', 'interval', 'times'],
  'weekly': ['frequency', 'interval', 'times', 'weekdays'],
};
const _maxIntervalOf = {'hourly': 23, 'daily': 365, 'weekly': 52};
const _newLogFields = ['amount', 'dueAt', 'source', 'status', 'takenAt'];
final _objectId = RegExp(r'^[0-9a-fA-F]{24}$');
final _controlCharacter = RegExp(r'\p{Cc}', unicode: true);
final _rfc3339 = RegExp(
  r'^\d{4}-\d{2}-\d{2}T\d{2}:\d{2}:\d{2}(\.\d+)?(Z|z|[+-]\d{2}:\d{2})$',
);

// -------------------------------------------------------------------------
// Days, times and points in time
// -------------------------------------------------------------------------

/// Days since 1970-01-01.
int dayNumber(PlainDate date) =>
    DateTime.utc(date.year, date.month, date.day).millisecondsSinceEpoch ~/
    _dayMs;

PlainDate dateOfDay(int day) {
  final t = DateTime.fromMillisecondsSinceEpoch(day * _dayMs, isUtc: true);
  return PlainDate(t.year, t.month, t.day);
}

/// Monday 0 … Sunday 6. 1970-01-01 was a Thursday.
int _weekdayIndex(int day) => (day + 3) % 7;

int _monday(int day) => day - _weekdayIndex(day);

/// `"HH:MM"` with 24 hours, from 00:00 to 23:59, as minutes after midnight.
int? parseClock(Object? value) {
  if (value is! String || value.length != 5 || value[2] != ':') return null;
  final digits = [0, 1, 3, 4].map((i) => value.codeUnitAt(i) - 48).toList();
  if (digits.any((d) => d < 0 || d > 9)) return null;
  final hour = digits[0] * 10 + digits[1];
  final minute = digits[2] * 10 + digits[3];
  if (hour > 23 || minute > 59) return null;
  return hour * 60 + minute;
}

String _clockText(int minutes) =>
    '${(minutes ~/ 60).toString().padLeft(2, '0')}:'
    '${(minutes % 60).toString().padLeft(2, '0')}';

int _offsetAt(int ms, tz.Location loc) =>
    loc.timeZone(ms).offset.inMilliseconds;

/// The local date of the point in time [ms] in [loc].
int localDay(int ms, tz.Location loc) => (ms + _offsetAt(ms, loc)) ~/ _dayMs;

int _localClock(int ms, tz.Location loc) {
  final local = ms + _offsetAt(ms, loc);
  return (local - (local ~/ _dayMs) * _dayMs) ~/ _minuteMs;
}

/// Do the clocks in [loc] show the reading [wallMs] (minutes) at [ms]?
bool _showsWallClock(int ms, tz.Location loc, int wallMs) =>
    (ms + _offsetAt(ms, loc)) ~/ _minuteMs == wallMs ~/ _minuteMs;

/// The point in time at which the clocks in [loc] show [clock] on [day]
/// (schedule.go, wallClockInstant): a time in a gap moves forward by the
/// length of the gap, a time that exists twice takes the earlier point in
/// time. The candidates are the offsets a day before and a day after.
int wallClockInstant(int day, int clock, tz.Location loc) {
  final wall = day * _dayMs + clock * _minuteMs;
  final before = _offsetAt(wall - _dayMs, loc);
  final after = _offsetAt(wall + _dayMs, loc);
  int? best;
  for (final offset in [before, after]) {
    final t = wall - offset;
    if (_showsWallClock(t, loc, wall) && (best == null || t < best)) best = t;
  }
  return best ?? wall - before;
}

// -------------------------------------------------------------------------
// Stored state
// -------------------------------------------------------------------------

class _Schedule {
  _Schedule({
    required this.frequency,
    required this.interval,
    this.times = const [],
    this.weekdays = const [],
    this.windowStart = 0,
    this.windowEnd = 0,
  });

  final String frequency;
  final int interval;
  final List<int> times;

  /// Monday 0 … Sunday 6, sorted.
  final List<int> weekdays;
  final int windowStart;
  final int windowEnd;

  List<int> wallTimes() => frequency == 'hourly'
      ? [for (var t = windowStart; t <= windowEnd; t += interval * 60) t]
      : [...times];

  bool sameAs(_Schedule o) =>
      frequency == o.frequency &&
      interval == o.interval &&
      windowStart == o.windowStart &&
      windowEnd == o.windowEnd &&
      _sameInts(times, o.times) &&
      _sameInts(weekdays, o.weekdays);

  Map<String, dynamic> toJson() => {
    'frequency': frequency,
    'interval': interval,
    if (frequency == 'weekly')
      'weekdays': [for (final d in weekdays) Weekday.values[d].name],
    if (frequency != 'hourly') 'times': [for (final t in times) _clockText(t)],
    if (frequency == 'hourly') ...{
      'windowStart': _clockText(windowStart),
      'windowEnd': _clockText(windowEnd),
    },
  };
}

bool _sameInts(List<int> a, List<int> b) {
  if (a.length != b.length) return false;
  for (var i = 0; i < a.length; i++) {
    if (a[i] != b[i]) return false;
  }
  return true;
}

class _Pause {
  _Pause(this.from, this.until);

  final int from;
  int? until;

  bool includes(int day) => from <= day && (until == null || day <= until!);
}

class _Routine {
  _Routine({
    required this.module,
    required this.name,
    required this.amount,
    required this.unit,
    required this.schedule,
    required this.startDay,
    required this.endDay,
  });

  String id = '';
  final String module;
  String name;
  double? amount;
  String? unit;
  _Schedule schedule;
  int startDay;
  int? endDay;
  List<_Pause> pauses = [];
  DateTime createdAt = DateTime.utc(1970);

  _Routine copy() =>
      _Routine(
          module: module,
          name: name,
          amount: amount,
          unit: unit,
          schedule: schedule,
          startDay: startDay,
          endDay: endDay,
        )
        ..id = id
        ..pauses = [for (final p in pauses) _Pause(p.from, p.until)]
        ..createdAt = createdAt;
}

class _Log {
  _Log({
    required this.id,
    required this.routineId,
    required this.dueMs,
    required this.dueDay,
    required this.dueClock,
    required this.takenAt,
    required this.status,
    required this.amount,
    required this.source,
    required this.timeZone,
    required this.createdAt,
  });

  final String id;
  final String routineId;

  /// The occurrence; `null` without one.
  final int? dueMs;

  /// Local date and time of the occurrence in [timeZone] (the backend's
  /// `dueLocal`). The log belongs to the occurrence with the same local
  /// date and time in the user's current time zone (3.16 "Logs", rule 2).
  final int? dueDay;
  final int? dueClock;
  final DateTime takenAt;
  final String status;
  final double? amount;
  final String source;
  final String timeZone;
  final DateTime createdAt;

  Map<String, dynamic> toJson() => {
    'id': id,
    'routineId': routineId,
    'dueAt': dueMs == null
        ? null
        : formatTimestamp(
            DateTime.fromMillisecondsSinceEpoch(dueMs!, isUtc: true),
          ),
    'takenAt': formatTimestamp(takenAt),
    'status': status,
    'amount': amount,
    'source': source,
    'entryId': null,
    'timeZone': timeZone,
    'createdAt': formatTimestamp(createdAt),
  };
}

class _Occurrence {
  _Occurrence(this.routineId, this.dueMs, this.day, this.clock);

  final String routineId;
  final int dueMs;
  final int day;
  final int clock;
  String status = 'open';
  String? logId;

  Map<String, dynamic> toJson() => {
    'routineId': routineId,
    'dueAt': formatTimestamp(
      DateTime.fromMillisecondsSinceEpoch(dueMs, isUtc: true),
    ),
    'date': dateOfDay(day).iso,
    'time': _clockText(clock),
    'status': status,
    'logId': logId,
  };
}

// -------------------------------------------------------------------------
// Expanding a schedule (schedule.go)
// -------------------------------------------------------------------------

class _Expander {
  _Expander(_Routine r, this.loc)
    : routineId = r.id,
      schedule = r.schedule,
      start = r.startDay,
      end = r.endDay,
      wallTimes = r.schedule.wallTimes();

  final String routineId;
  final _Schedule schedule;
  final int start;
  final int? end;
  final List<int> wallTimes;
  final tz.Location loc;

  /// Does the schedule plan occurrences on day [p]?
  bool planned(int p) {
    if (p < start || (end != null && p > end!)) return false;
    return switch (schedule.frequency) {
      'hourly' => true,
      'daily' => (p - start) % schedule.interval == 0,
      'weekly' =>
        ((_monday(p) - _monday(start)) ~/ 7) % schedule.interval == 0 &&
            schedule.weekdays.contains(_weekdayIndex(p)),
      _ => false,
    };
  }

  /// One offset from two days before to two days after [day]: every
  /// wall-clock time of the day exists exactly once. Near a change of the
  /// offset the slow path below computes each point in time by the rules.
  bool simpleDay(int day) =>
      loc.lookupTimeZone(day * _dayMs - 2 * _dayMs).end >
      (day + 1) * _dayMs + 2 * _dayMs;

  /// The occurrences whose local date is [day], sorted by point in time.
  List<_Occurrence> on(int day) {
    if (simpleDay(day)) {
      if (!planned(day)) return [];
      final offset = _offsetAt(day * _dayMs + _dayMs ~/ 2, loc);
      return [
        for (final c in wallTimes)
          _Occurrence(routineId, day * _dayMs + c * _minuteMs - offset, day, c),
      ];
    }
    // A forward shift can move a time past midnight, so the day before
    // counts too, and a shifted time can meet another one.
    final found = <_Occurrence>[];
    for (final p in [day - 1, day]) {
      if (!planned(p)) continue;
      for (final c in wallTimes) {
        final due = wallClockInstant(p, c, loc);
        if (localDay(due, loc) != day) continue;
        found.add(_Occurrence(routineId, due, day, _localClock(due, loc)));
      }
    }
    found.sort((a, b) => a.dueMs.compareTo(b.dueMs));
    final out = <_Occurrence>[];
    for (final o in found) {
      if (out.isEmpty || out.last.dueMs != o.dueMs) out.add(o);
    }
    return out;
  }

  List<_Occurrence> between(int from, int to) => [
    for (var d = from; d <= to; d++) ...on(d),
  ];
}

/// The occurrences of a schedule on the local dates [from] to [to], without
/// status: for the test against routine_occurrences.json.
List<Occurrence> expandSchedule({
  required RoutineSchedule schedule,
  required PlainDate startDate,
  PlainDate? endDate,
  required PlainDate from,
  required PlainDate to,
  required String timeZone,
  String routineId = '6742c0a1b2c3d4e5f6a7b8c9',
}) {
  final routine = _Routine(
    module: 'health',
    name: 'Testroutine',
    amount: null,
    unit: null,
    schedule: _parseSchedule(schedule.toJson()),
    startDay: dayNumber(startDate),
    endDay: endDate == null ? null : dayNumber(endDate),
  )..id = routineId;
  return [
    for (final o in _Expander(
      routine,
      locationOrNull(timeZone)!,
    ).between(dayNumber(from), dayNumber(to)))
      Occurrence.fromJson(o.toJson()),
  ];
}

// -------------------------------------------------------------------------
// Errors
// -------------------------------------------------------------------------

ApiException _invalid(String message, [String field = '', Object? value]) =>
    ApiException(
      httpStatus: 400,
      code: ErrorCode.invalidRequest,
      message: message,
      details: {'field': ?(field.isEmpty ? null : field), 'value': ?value},
    );

ApiException _parameter(String parameter, String value, String message) =>
    ApiException(
      httpStatus: 400,
      code: ErrorCode.invalidParameter,
      message: message,
      details: {'parameter': parameter, 'value': value},
    );

ApiException _limit(String limit, int max, String message) => ApiException(
  httpStatus: 409,
  code: ErrorCode.routineLimitReached,
  message: message,
  details: {'limit': limit, 'max': max},
);

const _routineNotFound = ApiException(
  httpStatus: 404,
  code: ErrorCode.routineNotFound,
  message: 'Diese Routine gibt es nicht.',
);

void _checkRoutineId(String id) {
  if (!_objectId.hasMatch(id)) {
    throw const ApiException(
      httpStatus: 400,
      code: ErrorCode.invalidId,
      message: 'Diese Routinen-ID ist ungültig.',
    );
  }
}

// -------------------------------------------------------------------------
// Reading requests (routine.go)
// -------------------------------------------------------------------------

/// The first field name (alphabetical) that is not allowed.
void _checkFieldNames(
  Map<String, dynamic> object,
  String prefix,
  List<String> allowed, [
  List<String> readOnly = const [],
]) {
  for (final name in object.keys.toList()..sort()) {
    if (allowed.contains(name)) continue;
    if (readOnly.contains(name)) {
      throw _invalid(
        'Das Feld $prefix$name kann nicht geändert werden.',
        '$prefix$name',
      );
    }
    throw _invalid('Unbekanntes Feld: $prefix$name.', '$prefix$name');
  }
}

/// Whitespace at the edges removed, at most [maxChars] characters, no
/// control characters. Empty is allowed here; `null` if invalid.
String? _trimText(String text, int maxChars) {
  final trimmed = text.trim();
  if (trimmed.runes.length > maxChars || _controlCharacter.hasMatch(trimmed)) {
    return null;
  }
  return trimmed;
}

String _parseName(Object? value) {
  final name = value is String ? _trimText(value, _maxNameChars) : null;
  if (name == null || name.isEmpty) throw _invalid(_msgName, 'name', value);
  return name;
}

double _parseAmount(Object? value) {
  if (value is! num || !(value > 0) || value > _maxAmount) {
    throw _invalid(_msgAmount, 'amount', value);
  }
  return value.toDouble();
}

String? _parseUnit(Object? value) {
  if (value == null) return null;
  if (value is! String) throw _invalid(_msgUnitType, 'unit', value);
  final unit = _trimText(value, _maxUnitChars);
  if (unit == null) throw _invalid(_msgUnitLength, 'unit', value);
  return unit.isEmpty ? null : unit;
}

int _parseDate(Object? value, String field) {
  final date = value is String ? PlainDate.tryParse(value) : null;
  if (date == null) throw _invalid(_msgDateFormat, field, value);
  final day = dayNumber(date);
  if (day < _minDay || day > _maxDay) {
    throw _invalid(_msgDateRange, field, value);
  }
  return day;
}

_Schedule _parseSchedule(Object? value) {
  if (value is! Map<String, dynamic>) {
    throw _invalid(_msgSchedule, 'schedule', value);
  }
  _checkFieldNames(value, 'schedule.', _scheduleFields);
  final frequency = value['frequency'];
  final maxInterval = _maxIntervalOf[frequency];
  if (frequency is! String || maxInterval == null) {
    throw _invalid(_msgFrequency, 'schedule.frequency', value['frequency']);
  }
  for (final field in value.keys.toList()..sort()) {
    if (!_scheduleFieldsOf[frequency]!.contains(field)) {
      throw _invalid(_msgForeignField, 'schedule.$field');
    }
  }
  final interval = value['interval'];
  if (interval is! num ||
      interval < 1 ||
      interval > maxInterval ||
      interval != interval.truncate()) {
    throw _invalid(
      'Das Intervall muss zwischen 1 und $maxInterval liegen.',
      'schedule.interval',
      value['interval'],
    );
  }
  final n = interval.toInt();
  switch (frequency) {
    case 'hourly':
      final start = _parseClockField(value, 'windowStart');
      final end = _parseClockField(value, 'windowEnd');
      if (end <= start) {
        throw _invalid(
          _msgWindowOrder,
          'schedule.windowEnd',
          value['windowEnd'],
        );
      }
      return _Schedule(
        frequency: frequency,
        interval: n,
        windowStart: start,
        windowEnd: end,
      );
    case 'weekly':
      final weekdays = _parseWeekdays(value['weekdays']);
      return _Schedule(
        frequency: frequency,
        interval: n,
        weekdays: weekdays,
        times: _parseTimes(value['times']),
      );
    default:
      return _Schedule(
        frequency: frequency,
        interval: n,
        times: _parseTimes(value['times']),
      );
  }
}

int _parseClockField(Map<String, dynamic> object, String name) {
  final clock = parseClock(object[name]);
  if (clock == null) {
    throw _invalid(_msgTimeFormat, 'schedule.$name', object[name]);
  }
  return clock;
}

List<int> _parseTimes(Object? value) {
  const field = 'schedule.times';
  if (value == null) throw _invalid(_msgTimesEmpty, field);
  if (value is! List) throw _invalid(_msgTimeFormat, field, value);
  if (value.isEmpty) throw _invalid(_msgTimesEmpty, field);
  if (value.length > _maxTimesPerDay) throw _invalid(_msgTimesMax, field);
  final times = <int>[];
  for (final element in value) {
    final clock = parseClock(element);
    if (clock == null) throw _invalid(_msgTimeFormat, field, element);
    if (times.contains(clock)) {
      throw _invalid(_msgTimesDuplicate, field, element);
    }
    times.add(clock);
  }
  return times..sort();
}

List<int> _parseWeekdays(Object? value) {
  const field = 'schedule.weekdays';
  if (value is! List || value.isEmpty) {
    throw _invalid(_msgWeekdaysEmpty, field, value);
  }
  final days = <int>[];
  for (final element in value) {
    final index = Weekday.values.indexWhere((w) => w.name == element);
    if (index < 0) throw _invalid(_msgWeekdayUnknown, field, element);
    if (days.contains(index)) {
      throw _invalid(_msgWeekdayDuplicate, field, element);
    }
    days.add(index);
  }
  return days..sort();
}

/// `POST /routines` (routine.go, ParseNewRoutine). Order: body, unknown
/// field, module, name, amount, unit, schedule, startDate, endDate, endDate
/// before startDate.
_Routine _parseNewRoutine(
  Map<String, dynamic>? body,
  bool Function(String) isKnownModule,
) {
  if (body == null) throw _invalid(_msgRoutineBody);
  _checkFieldNames(body, '', _routineCreateFields);
  final moduleValue = body['module'];
  final String module;
  if (moduleValue == null) {
    module = 'health';
  } else if (moduleValue is String && isKnownModule(moduleValue)) {
    module = moduleValue.isEmpty ? 'health' : moduleValue;
  } else {
    throw _invalid(_msgModule, 'module', moduleValue);
  }
  final name = _parseName(body['name']);
  final amount = body['amount'] == null ? null : _parseAmount(body['amount']);
  final unit = _parseUnit(body['unit']);
  if (unit != null && amount == null) {
    throw _invalid(_msgUnitNeedsAmount, 'unit', body['unit']);
  }
  final schedule = _parseSchedule(body['schedule']);
  final start = _parseDate(body['startDate'], 'startDate');
  final end = body['endDate'] == null
      ? null
      : _parseDate(body['endDate'], 'endDate');
  if (end != null && end < start) {
    throw _invalid(_msgEndBeforeStart, 'endDate', body['endDate']);
  }
  return _Routine(
    module: module,
    name: name,
    amount: amount,
    unit: unit,
    schedule: schedule,
    startDay: start,
    endDay: end,
  );
}

/// `PATCH /routines/{id}` (routine.go, ApplyRoutinePatch): a missing field
/// stays, `null` clears it, `amount: null` also clears the unit. Returns the
/// changed copy.
_Routine _applyPatch(_Routine routine, Map<String, dynamic>? body) {
  if (body == null) throw _invalid(_msgPatchBody);
  if (body.isEmpty) throw _invalid(_msgEmptyPatch);
  _checkFieldNames(body, '', _routinePatchFields, _routineReadOnlyFields);
  final n = routine.copy();
  if (body.containsKey('name')) n.name = _parseName(body['name']);
  if (body.containsKey('amount')) {
    if (body['amount'] == null) {
      n
        ..amount = null
        ..unit = null;
    } else {
      n.amount = _parseAmount(body['amount']);
    }
  }
  if (body.containsKey('unit')) n.unit = _parseUnit(body['unit']);
  if (body.containsKey('schedule')) {
    n.schedule = _parseSchedule(body['schedule']);
  }
  if (body.containsKey('startDate')) {
    n.startDay = _parseDate(body['startDate'], 'startDate');
  }
  if (body.containsKey('endDate')) {
    n.endDay = body['endDate'] == null
        ? null
        : _parseDate(body['endDate'], 'endDate');
  }
  if (n.unit != null && n.amount == null) {
    throw _invalid(_msgUnitNeedsAmount, 'unit', body['unit']);
  }
  if (n.endDay != null && n.endDay! < n.startDay) {
    if (body['endDate'] != null) {
      throw _invalid(_msgEndBeforeStart, 'endDate', body['endDate']);
    }
    throw _invalid(_msgStartAfterEnd, 'startDate', body['startDate']);
  }
  return n;
}

DateTime? _parseInstant(Object? value) {
  if (value is! String || !_rfc3339.hasMatch(value)) return null;
  return DateTime.tryParse(value)?.toUtc();
}

// -------------------------------------------------------------------------
// Status, progress (routine_progress.go)
// -------------------------------------------------------------------------

String _statusOn(_Routine r, int today) {
  if (r.endDay != null && r.endDay! < today) return 'ended';
  if (r.startDay > today) return 'upcoming';
  if (r.pauses.any((p) => p.includes(today))) return 'paused';
  return 'active';
}

/// Rounded to 6 decimal places, so 0.1 + 0.2 gives 0.3.
double _roundAmount(double x) => double.parse(x.toStringAsFixed(6));

class _Placed {
  _Placed(this.day, this.occurrence, this.log);

  final int day;
  final bool occurrence;
  final _Log log;
}

class _ProgressCounter {
  _ProgressCounter(this.r, this.loc)
    : x = _Expander(r, loc),
      lastDate = r.endDay == null ? _farFuture : r.endDay! + 1 {
    wallSet = x.wallTimes.toSet();
    final ranges = [
      for (final p in r.pauses) (from: p.from, to: p.until ?? _farFuture),
    ]..sort((a, b) => a.from.compareTo(b.from));
    for (final range in ranges) {
      if (pauses.isNotEmpty && range.from <= pauses.last.to + 1) {
        final last = pauses.removeLast();
        pauses.add((
          from: last.from,
          to: last.to > range.to ? last.to : range.to,
        ));
      } else {
        pauses.add(range);
      }
    }
  }

  final _Routine r;
  final tz.Location loc;
  final _Expander x;
  late final Set<int> wallSet;
  final pauses = <({int from, int to})>[];

  /// Occurrences end here (endDate + 1 for a time moved past midnight).
  final int lastDate;

  bool occursAt(int day, int clock) => x.simpleDay(day)
      ? x.planned(day) && wallSet.contains(clock)
      : x.on(day).any((o) => o.clock == clock);

  int count(int day) => x.simpleDay(day)
      ? (x.planned(day) ? x.wallTimes.length : 0)
      : x.on(day).length;

  _Placed place(_Log l) {
    if (l.dueDay != null && occursAt(l.dueDay!, l.dueClock!)) {
      return _Placed(l.dueDay!, true, l);
    }
    return _Placed(localDay(l.takenAt.millisecondsSinceEpoch, loc), false, l);
  }

  Map<String, dynamic> range(int a, int b, List<_Placed> placed) {
    var due = 0;
    var done = 0;
    var skipped = 0;
    var pause = 0;
    final first = a > r.startDay ? a : r.startDay;
    final last = b < lastDate ? b : lastDate;
    for (var d = first; d <= last; d++) {
      while (pause < pauses.length && pauses[pause].to < d) {
        pause++;
      }
      if (pause < pauses.length && pauses[pause].from <= d) continue;
      due += count(d);
    }
    var amount = 0.0;
    for (final p in placed) {
      if (p.day < a || p.day > b) continue;
      if (p.occurrence) {
        if (p.log.status == 'done') {
          done++;
        } else {
          skipped++;
        }
        // An occurrence with a log counts, also on a paused day.
        if (r.pauses.any((pause) => pause.includes(p.day))) due++;
      }
      if (p.log.status == 'done') {
        amount += p.log.amount ?? r.amount ?? 0;
      }
    }
    return {
      'done': done,
      'skipped': skipped,
      'due': due,
      'amountDone': r.amount == null ? null : _roundAmount(amount),
      'amountTarget': r.amount == null ? null : _roundAmount(r.amount! * due),
    };
  }
}

Map<String, dynamic> _progress(
  _Routine r,
  List<_Log> logs,
  tz.Location loc,
  DateTime now,
) {
  final today = localDay(now.millisecondsSinceEpoch, loc);
  final counter = _ProgressCounter(r, loc);
  final placed = [for (final l in logs) counter.place(l)];
  final monday = _monday(today);
  Map<String, dynamic>? period;
  if (r.endDay != null) {
    final days = r.endDay! - r.startDay + 1;
    period = {
      'day': today < r.startDay
          ? 0
          : today > r.endDay!
          ? days
          : today - r.startDay + 1,
      'days': days,
      ...counter.range(r.startDay, r.endDay!, placed),
    };
  }
  return {
    'today': counter.range(today, today, placed),
    'week': counter.range(monday, monday + 6, placed),
    'period': period,
  };
}

// -------------------------------------------------------------------------
// The store
// -------------------------------------------------------------------------

/// Routines and logs of the mock. Like the mock's entries they are not
/// separated per user; the user's time zone is passed in with every call.
class MockRoutines {
  MockRoutines({required this._clock, required this._isKnownModule});

  final DateTime Function() _clock;
  final bool Function(String module) _isKnownModule;
  final _routines = <String, _Routine>{};
  final _logs = <String, _Log>{};
  var _counter = 0;

  /// Rising IDs in the style of an ObjectId (seconds, then a counter), so
  /// that sorting by ID follows the order of creation, as with MongoDB.
  String _newId() {
    _counter++;
    final seconds = _clock().millisecondsSinceEpoch ~/ 1000;
    return seconds.toRadixString(16).padLeft(8, '0') +
        _counter.toRadixString(16).padLeft(16, '0');
  }

  tz.Location _location(String timeZone) =>
      locationOrNull(timeZone) ?? locationOrNull('Europe/Vienna')!;

  int _today(tz.Location loc) => localDay(_clock().millisecondsSinceEpoch, loc);

  _Routine _load(String id) {
    _checkRoutineId(id);
    return _routines[id] ?? (throw _routineNotFound);
  }

  List<_Log> _logsOf(String routineId) => [
    for (final l in _logs.values)
      if (l.routineId == routineId) l,
  ];

  Routine _view(_Routine r, String timeZone) {
    final loc = _location(timeZone);
    return Routine.fromJson({
      'id': r.id,
      'module': r.module,
      'name': r.name,
      'amount': r.amount,
      'unit': r.unit,
      'schedule': r.schedule.toJson(),
      'startDate': dateOfDay(r.startDay).iso,
      'endDate': r.endDay == null ? null : dateOfDay(r.endDay!).iso,
      'status': _statusOn(r, _today(loc)),
      'pauses': [
        for (final p in r.pauses)
          {
            'from': dateOfDay(p.from).iso,
            'until': p.until == null ? null : dateOfDay(p.until!).iso,
          },
      ],
      'createdAt': formatTimestamp(r.createdAt),
      'progress': _progress(r, _logsOf(r.id), loc, _clock()),
    });
  }

  String _checkModuleParameter(String module) {
    if (!_isKnownModule(module)) throw _parameter('module', module, _msgModule);
    return module.isEmpty ? 'health' : module;
  }

  // 3.17
  List<Routine> list(String module, String timeZone) {
    final chosen = _checkModuleParameter(module);
    final routines =
        [
          for (final r in _routines.values)
            if (r.module == chosen) r,
        ]..sort((a, b) {
          final byTime = a.createdAt.compareTo(b.createdAt);
          return byTime != 0 ? byTime : a.id.compareTo(b.id);
        });
    return [for (final r in routines) _view(r, timeZone)];
  }

  // 3.18
  Routine create(Map<String, dynamic>? body, String timeZone) {
    final r = _parseNewRoutine(body, _isKnownModule);
    if (_routines.length >= maxRoutinesPerUser) {
      throw _limit(
        'routines',
        maxRoutinesPerUser,
        'Du hast schon 50 Routinen. Lösche eine, bevor du eine neue anlegst.',
      );
    }
    _checkNameFree(r);
    r
      ..id = _newId()
      ..createdAt = _clock();
    _routines[r.id] = r;
    return _view(r, timeZone);
  }

  /// Names are unique per module, ignoring upper and lower case (the name in
  /// lower case; `ß` and `ss` stay different).
  void _checkNameFree(_Routine r) {
    final key = r.name.toLowerCase();
    for (final other in _routines.values) {
      if (other.id != r.id &&
          other.module == r.module &&
          other.name.toLowerCase() == key) {
        throw ApiException(
          httpStatus: 409,
          code: ErrorCode.routineNameTaken,
          message: 'Eine Routine mit diesem Namen gibt es schon.',
          details: {'routineId': other.id},
        );
      }
    }
  }

  // 3.19
  Routine get(String id, String timeZone) => _view(_load(id), timeZone);

  // 3.20
  Routine update(String id, Map<String, dynamic>? body, String timeZone) {
    final changed = _applyPatch(_load(id), body);
    _checkNameFree(changed);
    _routines[id] = changed;
    return _view(changed, timeZone);
  }

  // 3.21
  Routine pause(String id, Map<String, dynamic>? body, String timeZone) {
    final r = _load(id);
    if (body == null) throw _invalid(_msgPauseBody);
    _checkFieldNames(body, '', const ['until']);
    final until = body['until'] == null
        ? null
        : _parseDate(body['until'], 'until');
    final today = _today(_location(timeZone));
    if (until != null && until < today) {
      throw _invalid(_msgPauseUntil, 'until', dateOfDay(until).iso);
    }
    final current = r.pauses.where((p) => p.includes(today)).toList();
    if (current.isNotEmpty) {
      for (final p in current) {
        p.until = until;
      }
    } else {
      if (r.pauses.length >= _maxPauses) {
        throw _limit(
          'pauses',
          _maxPauses,
          'Diese Routine wurde schon zu oft pausiert.',
        );
      }
      r.pauses.add(_Pause(today, until));
    }
    return _view(r, timeZone);
  }

  // 3.22
  Routine resume(String id, String timeZone) {
    final r = _load(id);
    final today = _today(_location(timeZone));
    r.pauses = [
      for (final p in r.pauses)
        if (!p.includes(today))
          p
        else if (p.from != today)
          _Pause(p.from, today - 1),
    ];
    return _view(r, timeZone);
  }

  // 3.23
  void delete(String id) {
    _load(id);
    _logs.removeWhere((_, l) => l.routineId == id);
    _routines.remove(id);
  }

  // 3.24
  OccurrenceList occurrences(Map<String, String> parameters, String timeZone) {
    final chosen = _checkModuleParameter(parameters['module'] ?? '');
    final loc = _location(timeZone);
    final fromText = parameters['from'] ?? '';
    final toText = parameters['to'] ?? '';
    final from = _parameterDate('from', fromText, _today(loc));
    final to = _parameterDate('to', toText, from + 6);
    _checkDayRange(fromText, toText, from, to, _maxOccurrenceDays);
    final list = <_Occurrence>[];
    for (final r in _routines.values.where((r) => r.module == chosen)) {
      final byDue = {
        for (final l in _logsOf(r.id))
          if (l.dueDay != null) (l.dueDay, l.dueClock): l,
      };
      for (final o in _Expander(r, loc).between(from, to)) {
        final log = byDue[(o.day, o.clock)];
        if (log != null) {
          o
            ..logId = log.id
            ..status = log.status;
        } else if (r.pauses.any((p) => p.includes(o.day))) {
          o.status = 'paused';
        }
        list.add(o);
      }
    }
    list.sort((a, b) {
      final byTime = a.dueMs.compareTo(b.dueMs);
      return byTime != 0 ? byTime : a.routineId.compareTo(b.routineId);
    });
    return OccurrenceList.fromJson({
      'from': dateOfDay(from).iso,
      'to': dateOfDay(to).iso,
      'occurrences': [for (final o in list) o.toJson()],
      'count': list.length,
    });
  }

  int _parameterDate(String name, String value, int fallback) {
    if (value.isEmpty) return fallback;
    final date = PlainDate.tryParse(value);
    if (date == null) throw _parameter(name, value, _msgDateFormat);
    return dayNumber(date);
  }

  /// Names the parameter the app sent: `to` if given, otherwise `from`.
  void _checkDayRange(
    String fromText,
    String toText,
    int from,
    int to,
    int maxDays,
  ) {
    final (parameter, value) = toText.isEmpty
        ? ('from', fromText)
        : ('to', toText);
    if (to < from) throw _parameter(parameter, value, _msgRangeReversed);
    if (to - from + 1 > maxDays) {
      throw _parameter(
        parameter,
        value,
        'Der Zeitraum darf höchstens $maxDays Tage umfassen.',
      );
    }
  }

  // 3.25
  List<RoutineLog> logs(
    String id,
    Map<String, String> parameters,
    String timeZone,
  ) {
    _load(id);
    final loc = _location(timeZone);
    final fromText = parameters['from'] ?? '';
    final toText = parameters['to'] ?? '';
    final to = _parameterDate('to', toText, _today(loc));
    final from = _parameterDate('from', fromText, to - 29);
    _checkDayRange(fromText, toText, from, to, _maxLogDays);
    final list =
        [
          for (final l in _logsOf(id))
            if (localDay(l.takenAt.millisecondsSinceEpoch, loc) case final day
                when day >= from && day <= to)
              l,
        ]..sort((a, b) {
          final byTime = b.takenAt.compareTo(a.takenAt);
          return byTime != 0 ? byTime : b.id.compareTo(a.id);
        });
    return [for (final l in list) RoutineLog.fromJson(l.toJson())];
  }

  // 3.26 (core.CheckNewLog and service.AddLog)
  RoutineLog addLog(String id, Map<String, dynamic>? body, String timeZone) {
    final r = _load(id);
    if (body == null) throw _invalid(_msgLogBody);
    _checkFieldNames(body, '', _newLogFields);
    final status = body['status'];
    if (status != 'done' && status != 'skipped') {
      throw _invalid(_msgLogStatus, 'status', status);
    }
    final source = body['source'];
    if (source != 'manual' && source != 'notification') {
      throw _invalid(_msgLogSource, 'source', source);
    }
    DateTime? dueAt;
    if (body['dueAt'] != null) {
      dueAt =
          _parseInstant(body['dueAt']) ??
          (throw _invalid(_msgInstant, 'dueAt', body['dueAt']));
    }
    final now = _clock();
    var takenAt = now;
    if (body['takenAt'] != null) {
      takenAt =
          _parseInstant(body['takenAt']) ??
          (throw _invalid(_msgInstant, 'takenAt', body['takenAt']));
    }
    double? amount;
    if (body['amount'] != null) {
      final value = body['amount'];
      if (value is! num) throw _invalid(_msgAmount, 'amount', value);
      amount = value.toDouble();
    }

    final loc = _location(timeZone);
    _Occurrence? occurrence;
    if (dueAt != null) {
      final ms = dueAt.millisecondsSinceEpoch;
      final day = localDay(ms, loc);
      occurrence = _Expander(
        r,
        loc,
      ).on(day).where((o) => o.dueMs == ms).firstOrNull;
      if (occurrence == null) {
        throw _invalid(_msgNoOccurrence, 'dueAt', body['dueAt']);
      }
      if (occurrence.day > _today(loc)) {
        throw _invalid(_msgFutureOccurrence, 'dueAt', body['dueAt']);
      }
    }
    if (takenAt.isAfter(now.add(_logFutureTolerance))) {
      throw _invalid(_msgFutureTakenAt, 'takenAt', body['takenAt']);
    }
    if (status == 'skipped' && amount != null) {
      throw _invalid(_msgSkippedAmount, 'amount', body['amount']);
    }
    if (amount != null && (!(amount > 0) || amount > _maxAmount)) {
      throw _invalid(_msgAmount, 'amount', body['amount']);
    }
    if (amount == null && status == 'done') amount = r.amount;

    if (occurrence != null) {
      for (final l in _logsOf(id)) {
        if (l.dueDay == occurrence.day && l.dueClock == occurrence.clock) {
          throw ApiException(
            httpStatus: 409,
            code: ErrorCode.routineLogExists,
            message: 'Dieser Termin ist schon eingetragen.',
            details: {'logId': l.id},
          );
        }
      }
    }
    final takenDay = localDay(takenAt.millisecondsSinceEpoch, loc);
    final sameDay = _logsOf(
      id,
    ).where((l) => localDay(l.takenAt.millisecondsSinceEpoch, loc) == takenDay);
    if (sameDay.length >= _maxLogsPerDay) {
      throw _limit(
        'logs',
        _maxLogsPerDay,
        'Für heute sind bei dieser Routine keine weiteren Einträge möglich.',
      );
    }
    final log = _Log(
      id: _newId(),
      routineId: id,
      dueMs: occurrence?.dueMs,
      dueDay: occurrence?.day,
      dueClock: occurrence?.clock,
      takenAt: takenAt,
      status: status as String,
      amount: status == 'skipped' ? null : amount,
      source: source as String,
      timeZone: timeZone,
      createdAt: now,
    );
    _logs[log.id] = log;
    return RoutineLog.fromJson(log.toJson());
  }

  // 3.27
  void deleteLog(String id, String logId) {
    _checkRoutineId(id);
    if (!_objectId.hasMatch(logId)) {
      throw const ApiException(
        httpStatus: 400,
        code: ErrorCode.invalidId,
        message: 'Diese Verlaufseintrag-ID ist ungültig.',
      );
    }
    _load(id);
    final log = _logs[logId];
    if (log == null || log.routineId != id) {
      throw const ApiException(
        httpStatus: 404,
        code: ErrorCode.routineLogNotFound,
        message: 'Diesen Verlaufseintrag gibt es nicht.',
      );
    }
    _logs.remove(logId);
  }

  /// Three synthetic routines for trying the app without a backend,
  /// relative to today in Vienna: „Vitamin D“ (2 Stück at 08:00),
  /// „Blutdruck messen“ (08:00 and 20:00 for 14 days) and „Wasser“ (250 ml
  /// every hour from 08:00 to 20:00). „Vitamin D“ is done today if 08:00
  /// has passed.
  void addSamples() {
    final loc = _location('Europe/Vienna');
    final today = _today(loc);
    String date(int day) => dateOfDay(day).iso;
    final vitamin = create({
      'name': 'Vitamin D',
      'amount': 2,
      'unit': 'Stück',
      'schedule': {
        'frequency': 'daily',
        'interval': 1,
        'times': ['08:00'],
      },
      'startDate': date(today - 10),
    }, 'Europe/Vienna');
    create({
      'name': 'Blutdruck messen',
      'schedule': {
        'frequency': 'daily',
        'interval': 1,
        'times': ['08:00', '20:00'],
      },
      'startDate': date(today - 5),
      'endDate': date(today + 8),
    }, 'Europe/Vienna');
    create({
      'name': 'Wasser',
      'amount': 250,
      'unit': 'ml',
      'schedule': {
        'frequency': 'hourly',
        'interval': 1,
        'windowStart': '08:00',
        'windowEnd': '20:00',
      },
      'startDate': date(today - 2),
    }, 'Europe/Vienna');
    final eight = wallClockInstant(today, 8 * 60, loc);
    if (eight <= _clock().millisecondsSinceEpoch) {
      addLog(vitamin.id, {
        'status': 'done',
        'source': 'manual',
        'dueAt': formatTimestamp(
          DateTime.fromMillisecondsSinceEpoch(eight, isUtc: true),
        ),
      }, 'Europe/Vienna');
    }
  }
}
