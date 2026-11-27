import 'package:flutter/material.dart';

import '../api/api_client.dart';
import '../api/api_exception.dart';
import '../model/plain_date.dart';
import '../model/routine.dart';
import '../model/routine_texts.dart';
import 'routine_actions.dart';

/// Unit suggestions under the amount (only a help; any unit is allowed).
const unitSuggestions = ['Stück', 'ml', 'g', 'mg', 'Tropfen'];

/// Largest interval per frequency (docs/API.md 3.16 "Schedule").
const maxIntervals = {
  Frequency.hourly: 23,
  Frequency.daily: 365,
  Frequency.weekly: 52,
};

/// „Neue Routine“ or „Routine bearbeiten“: name, optional amount and unit,
/// schedule (hourly in a window, daily, weekly on weekdays, each every N),
/// start and optional end. Large controls, one preview line, and the same
/// checks and texts as the backend before anything is sent; whatever the
/// backend still refuses is shown at the field it names.
///
/// Returns the created or changed routine.
class RoutineFormScreen extends StatefulWidget {
  const RoutineFormScreen({
    super.key,
    required this.api,
    required this.moduleId,
    required this.today,
    this.routine,
  });

  final ApiClient api;
  final String moduleId;

  /// Today in the user's time zone, from the backend: the default start.
  final PlainDate today;

  /// The routine to change; `null` for a new one.
  final Routine? routine;

  @override
  State<RoutineFormScreen> createState() => _RoutineFormScreenState();
}

/// Which part of the form an error belongs to.
enum _Part { name, amount, unit, schedule, start, end, general }

class _RoutineFormScreenState extends State<RoutineFormScreen> {
  late final Routine? _old = widget.routine;
  late final _name = TextEditingController(text: _old?.name ?? '');
  late var _withAmount = _old?.amount != null;
  late final _amount = TextEditingController(
    text: _old?.amount == null ? '' : formatAmount(_old!.amount!),
  );
  late final _unit = TextEditingController(text: _old?.unit ?? '');
  late var _frequency = _old?.schedule.frequency ?? Frequency.daily;
  late var _interval = _old?.schedule.interval ?? 1;
  late var _times = _old == null || _old.schedule.times.isEmpty
      ? ['08:00']
      : [..._old.schedule.times];
  late var _windowStart = _old?.schedule.windowStart ?? '08:00';
  late var _windowEnd = _old?.schedule.windowEnd ?? '20:00';
  late var _weekdays = _old == null || _old.schedule.weekdays.isEmpty
      ? {_weekdayOf(widget.today)}
      : {..._old.schedule.weekdays};
  late var _start = _old?.startDate ?? widget.today;
  late var _end = _old?.endDate;

  final _errors = <_Part, String>{};
  var _busy = false;

  static Weekday _weekdayOf(PlainDate date) =>
      Weekday.values[DateTime.utc(date.year, date.month, date.day).weekday - 1];

  @override
  void dispose() {
    _name.dispose();
    _amount.dispose();
    _unit.dispose();
    super.dispose();
  }

  RoutineSchedule get _schedule => switch (_frequency) {
    Frequency.hourly => RoutineSchedule.hourly(
      interval: _interval,
      windowStart: _windowStart,
      windowEnd: _windowEnd,
    ),
    Frequency.daily => RoutineSchedule.daily(
      interval: _interval,
      times: [..._times]..sort(),
    ),
    Frequency.weekly => RoutineSchedule.weekly(
      interval: _interval,
      weekdays: [
        for (final day in Weekday.values)
          if (_weekdays.contains(day)) day,
      ],
      times: [..._times]..sort(),
    ),
  };

  /// The same rules and texts as the backend (3.16 "Limits and
  /// validation"). Returns the request, or `null` with [_errors] filled.
  NewRoutine? _check() {
    _errors.clear();
    final name = _name.text.trim();
    if (name.isEmpty ||
        name.runes.length > 60 ||
        RegExp(r'\p{Cc}', unicode: true).hasMatch(name)) {
      _errors[_Part.name] = 'Der Name muss 1 bis 60 Zeichen lang sein.';
    }
    double? amount;
    String? unit;
    if (_withAmount) {
      amount = parseAmount(_amount.text);
      if (amount == null) {
        _errors[_Part.amount] =
            'Bitte eine Menge eingeben, z. B. 250 oder 0,5.';
      } else if (!(amount > 0) || amount > 100000) {
        _errors[_Part.amount] =
            'Die Menge muss größer als 0 und höchstens 100.000 sein.';
      }
      final text = _unit.text.trim();
      unit = text.isEmpty ? null : text;
      if (text.runes.length > 20) {
        _errors[_Part.unit] =
            'Die Einheit darf höchstens 20 Zeichen lang sein.';
      }
    }
    if (_frequency == Frequency.weekly && _weekdays.isEmpty) {
      _errors[_Part.schedule] = 'Bitte mindestens einen Wochentag wählen.';
    }
    if (_frequency == Frequency.hourly &&
        _windowEnd.compareTo(_windowStart) <= 0) {
      _errors[_Part.schedule] =
          'Das Ende des Zeitfensters muss nach dem Beginn liegen.';
    }
    if (_end != null && _end!.compareTo(_start) < 0) {
      _errors[_Part.end] = 'Das Enddatum darf nicht vor dem Startdatum liegen.';
    }
    if (_errors.isNotEmpty) return null;
    return NewRoutine(
      module: widget.moduleId,
      name: name,
      amount: amount,
      unit: unit,
      schedule: _schedule,
      startDate: _start,
      endDate: _end,
    );
  }

  Future<void> _save() async {
    final request = _check();
    if (request == null) {
      setState(() {});
      return;
    }
    final old = _old;
    if (old != null && RoutinePatch.between(old, request).isEmpty) {
      // Nothing changed: no request.
      Navigator.pop(context, old);
      return;
    }
    setState(() => _busy = true);
    try {
      final saved = old == null
          ? await widget.api.createRoutine(request)
          : await widget.api.updateRoutine(
              old.id,
              RoutinePatch.between(old, request),
            );
      if (mounted) Navigator.pop(context, saved);
    } on ApiException catch (e) {
      if (!mounted) return;
      setState(() {
        _busy = false;
        _errors[_partOf(e)] = e.message;
      });
    }
  }

  /// The backend names the field in `details.field` (3.18).
  static _Part _partOf(ApiException e) {
    if (e.code == ErrorCode.routineNameTaken) return _Part.name;
    final field = e.details['field'];
    return switch (field) {
      'name' => _Part.name,
      'amount' => _Part.amount,
      'unit' => _Part.unit,
      'startDate' => _Part.start,
      'endDate' => _Part.end,
      String() when field.startsWith('schedule') => _Part.schedule,
      _ => _Part.general,
    };
  }

  Future<String?> _pickTime(String initial) async {
    final parts = initial.split(':');
    final picked = await showTimePicker(
      context: context,
      initialTime: TimeOfDay(
        hour: int.parse(parts[0]),
        minute: int.parse(parts[1]),
      ),
      // Typing two numbers is easier than dragging a dial.
      initialEntryMode: TimePickerEntryMode.input,
      helpText: 'Uhrzeit',
      builder: (context, child) => MediaQuery(
        data: MediaQuery.of(context).copyWith(alwaysUse24HourFormat: true),
        child: child!,
      ),
    );
    if (picked == null) return null;
    String two(int n) => n.toString().padLeft(2, '0');
    return '${two(picked.hour)}:${two(picked.minute)}';
  }

  Future<void> _addTime() async {
    final time = await _pickTime(_times.isEmpty ? '08:00' : _times.last);
    if (time == null || !mounted) return;
    setState(() {
      if (!_times.contains(time)) _times = [..._times, time]..sort();
      _errors.remove(_Part.schedule);
    });
  }

  Future<void> _changeTime(String old) async {
    final time = await _pickTime(old);
    if (time == null || !mounted) return;
    setState(() {
      _times = {..._times.where((t) => t != old), time}.toList()..sort();
    });
  }

  Future<PlainDate?> _pickDate(PlainDate initial) async {
    final picked = await showDatePicker(
      context: context,
      initialDate: initial.toDateTime(),
      firstDate: DateTime(2000),
      lastDate: DateTime(2099, 12, 31),
    );
    return picked == null ? null : PlainDate.fromDateTime(picked);
  }

  void _setFrequency(Frequency frequency) => setState(() {
    _frequency = frequency;
    _interval = _interval.clamp(1, maxIntervals[frequency]!);
    _errors.remove(_Part.schedule);
  });

  @override
  Widget build(BuildContext context) {
    final theme = Theme.of(context);
    final bigText = theme.textTheme.titleMedium;
    Widget heading(String text) => Padding(
      padding: const EdgeInsets.only(top: 24, bottom: 8),
      child: Text(text, style: theme.textTheme.titleLarge),
    );
    Widget? error(_Part part) => _errors[part] == null
        ? null
        : Padding(
            padding: const EdgeInsets.only(top: 8),
            child: Text(
              _errors[part]!,
              style: theme.textTheme.bodyLarge?.copyWith(
                color: theme.colorScheme.error,
              ),
            ),
          );

    return Scaffold(
      appBar: AppBar(
        title: Text(_old == null ? 'Neue Routine' : 'Routine bearbeiten'),
      ),
      body: ListView(
        padding: const EdgeInsets.fromLTRB(16, 8, 16, 32),
        children: [
          TextField(
            key: const ValueKey('routine-name'),
            controller: _name,
            enabled: !_busy,
            style: theme.textTheme.titleLarge,
            maxLength: 60,
            textCapitalization: TextCapitalization.sentences,
            decoration: InputDecoration(
              labelText: 'Name',
              hintText: 'z. B. Vitamin D',
              border: const OutlineInputBorder(),
              errorText: _errors[_Part.name],
              errorMaxLines: 2,
            ),
            onChanged: (_) => setState(() => _errors.remove(_Part.name)),
          ),
          SwitchListTile(
            contentPadding: EdgeInsets.zero,
            title: Text('Mit Menge', style: bigText),
            subtitle: const Text('z. B. 2 Stück oder 250 ml pro Termin'),
            value: _withAmount,
            onChanged: _busy ? null : (v) => setState(() => _withAmount = v),
          ),
          if (_withAmount) ...[
            Row(
              crossAxisAlignment: CrossAxisAlignment.start,
              children: [
                Expanded(
                  flex: 2,
                  child: TextField(
                    key: const ValueKey('routine-amount'),
                    controller: _amount,
                    enabled: !_busy,
                    style: theme.textTheme.titleLarge,
                    keyboardType: const TextInputType.numberWithOptions(
                      decimal: true,
                    ),
                    decoration: InputDecoration(
                      labelText: 'Menge',
                      border: const OutlineInputBorder(),
                      errorText: _errors[_Part.amount],
                      errorMaxLines: 3,
                    ),
                  ),
                ),
                const SizedBox(width: 12),
                Expanded(
                  flex: 3,
                  child: TextField(
                    key: const ValueKey('routine-unit'),
                    controller: _unit,
                    enabled: !_busy,
                    style: theme.textTheme.titleLarge,
                    decoration: InputDecoration(
                      labelText: 'Einheit',
                      hintText: 'z. B. Stück',
                      border: const OutlineInputBorder(),
                      errorText: _errors[_Part.unit],
                      errorMaxLines: 3,
                    ),
                  ),
                ),
              ],
            ),
            const SizedBox(height: 8),
            Wrap(
              spacing: 8,
              runSpacing: 8,
              children: [
                for (final unit in unitSuggestions)
                  ActionChip(
                    label: Text(unit, style: bigText),
                    onPressed: _busy
                        ? null
                        : () => setState(() => _unit.text = unit),
                  ),
              ],
            ),
          ],
          heading('Rhythmus'),
          SegmentedButton<Frequency>(
            style: SegmentedButton.styleFrom(
              minimumSize: largeButtonSize,
              textStyle: bigText,
            ),
            showSelectedIcon: false,
            segments: const [
              ButtonSegment(value: Frequency.hourly, label: Text('stündlich')),
              ButtonSegment(value: Frequency.daily, label: Text('täglich')),
              ButtonSegment(
                value: Frequency.weekly,
                label: Text('wöchentlich'),
              ),
            ],
            selected: {_frequency},
            onSelectionChanged: _busy ? null : (s) => _setFrequency(s.single),
          ),
          const SizedBox(height: 16),
          Row(
            children: [
              IconButton.outlined(
                iconSize: 32,
                tooltip: 'Intervall kleiner',
                icon: const Icon(Icons.remove),
                onPressed: _busy || _interval <= 1
                    ? null
                    : () => setState(() => _interval--),
              ),
              Expanded(
                child: Text(
                  intervalLabel(_frequency, _interval),
                  textAlign: TextAlign.center,
                  style: theme.textTheme.titleLarge,
                ),
              ),
              IconButton.outlined(
                iconSize: 32,
                tooltip: 'Intervall größer',
                icon: const Icon(Icons.add),
                onPressed: _busy || _interval >= maxIntervals[_frequency]!
                    ? null
                    : () => setState(() => _interval++),
              ),
            ],
          ),
          if (_frequency == Frequency.hourly) ...[
            heading('Zeitfenster'),
            Row(
              children: [
                Expanded(
                  child: OutlinedButton(
                    style: largeOutlined(context),
                    onPressed: _busy
                        ? null
                        : () async {
                            final t = await _pickTime(_windowStart);
                            if (t != null && mounted) {
                              setState(() {
                                _windowStart = t;
                                _errors.remove(_Part.schedule);
                              });
                            }
                          },
                    child: Text('von $_windowStart'),
                  ),
                ),
                const SizedBox(width: 12),
                Expanded(
                  child: OutlinedButton(
                    style: largeOutlined(context),
                    onPressed: _busy
                        ? null
                        : () async {
                            final t = await _pickTime(_windowEnd);
                            if (t != null && mounted) {
                              setState(() {
                                _windowEnd = t;
                                _errors.remove(_Part.schedule);
                              });
                            }
                          },
                    child: Text('bis $_windowEnd'),
                  ),
                ),
              ],
            ),
          ] else ...[
            if (_frequency == Frequency.weekly) ...[
              heading('Wochentage'),
              Wrap(
                spacing: 8,
                runSpacing: 8,
                children: [
                  for (final day in Weekday.values)
                    FilterChip(
                      label: Text(weekdayLabels[day]!, style: bigText),
                      selected: _weekdays.contains(day),
                      onSelected: _busy
                          ? null
                          : (on) => setState(() {
                              _weekdays = on
                                  ? {..._weekdays, day}
                                  : ({..._weekdays}..remove(day));
                              _errors.remove(_Part.schedule);
                            }),
                    ),
                ],
              ),
            ],
            heading('Uhrzeiten'),
            Wrap(
              spacing: 8,
              runSpacing: 8,
              children: [
                for (final time in _times)
                  InputChip(
                    label: Text(time, style: theme.textTheme.titleLarge),
                    onPressed: _busy ? null : () => _changeTime(time),
                    onDeleted: _busy || _times.length <= 1
                        ? null
                        : () => setState(
                            () => _times = [..._times]..remove(time),
                          ),
                    deleteButtonTooltipMessage: '$time entfernen',
                  ),
                if (_times.length < 24)
                  ActionChip(
                    avatar: const Icon(Icons.add),
                    label: Text('Uhrzeit hinzufügen', style: bigText),
                    onPressed: _busy ? null : _addTime,
                  ),
              ],
            ),
          ],
          ?error(_Part.schedule),
          heading('Zeitraum'),
          OutlinedButton(
            style: largeOutlined(context),
            onPressed: _busy
                ? null
                : () async {
                    final d = await _pickDate(_start);
                    if (d != null && mounted) {
                      setState(() {
                        _start = d;
                        _errors.remove(_Part.start);
                      });
                    }
                  },
            child: Text('Beginn: ${_start.display}'),
          ),
          ?error(_Part.start),
          const SizedBox(height: 12),
          Row(
            children: [
              Expanded(
                child: OutlinedButton(
                  style: largeOutlined(context),
                  onPressed: _busy
                      ? null
                      : () async {
                          final d = await _pickDate(
                            _end ??
                                (_start.compareTo(widget.today) > 0
                                    ? _start
                                    : widget.today),
                          );
                          if (d != null && mounted) {
                            setState(() {
                              _end = d;
                              _errors.remove(_Part.end);
                            });
                          }
                        },
                  child: Text(
                    _end == null ? 'Ende: ohne' : 'Ende: ${_end!.display}',
                  ),
                ),
              ),
              if (_end != null) ...[
                const SizedBox(width: 8),
                TextButton(
                  onPressed: _busy ? null : () => setState(() => _end = null),
                  child: const Text('Kein Ende'),
                ),
              ],
            ],
          ),
          ?error(_Part.end),
          const SizedBox(height: 24),
          Card(
            child: Padding(
              padding: const EdgeInsets.all(16),
              child: Text(
                describeRoutinePlan(
                  _schedule,
                  _start,
                  _end,
                  today: widget.today,
                ),
                key: const ValueKey('routine-preview'),
                style: bigText,
              ),
            ),
          ),
          ?error(_Part.general),
          const SizedBox(height: 24),
          FilledButton(
            style: largeFilled(context),
            onPressed: _busy ? null : _save,
            child: _busy
                ? const SizedBox(
                    width: 24,
                    height: 24,
                    child: CircularProgressIndicator(strokeWidth: 3),
                  )
                : const Text('Speichern'),
          ),
        ],
      ),
    );
  }
}
