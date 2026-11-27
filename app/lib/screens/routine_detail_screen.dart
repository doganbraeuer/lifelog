import 'package:flutter/material.dart';

import '../api/api_client.dart';
import '../api/api_exception.dart';
import '../model/plain_date.dart';
import '../model/routine.dart';
import '../model/routine_texts.dart';
import '../widgets/error_view.dart';
import '../widgets/format.dart';
import '../widgets/navigation.dart';
import 'occurrence_sheet.dart';
import 'routine_actions.dart';
import 'routine_form_screen.dart';

/// One routine: plan, progress (today, week, period), the occurrences of
/// the next days, the history, and the actions „Pausieren“/„Fortsetzen“,
/// „Bearbeiten“ and „Löschen“.
///
/// Returns `true` if the routine was deleted.
class RoutineDetailScreen extends StatefulWidget {
  const RoutineDetailScreen({
    super.key,
    required this.api,
    required this.routineId,
    required this.moduleId,
    this.onChanged,
  });

  final ApiClient api;
  final String routineId;
  final String moduleId;

  /// After every change, e.g. to plan the reminders again.
  final VoidCallback? onChanged;

  @override
  State<RoutineDetailScreen> createState() => _RoutineDetailScreenState();
}

class _RoutineDetailScreenState extends State<RoutineDetailScreen> {
  Routine? _routine;
  OccurrenceList? _occurrences;
  List<RoutineLog>? _logs;
  ApiException? _error;
  var _busy = false;

  @override
  void initState() {
    super.initState();
    _load();
  }

  Future<void> _load() async {
    setState(() => _error = null);
    try {
      final routine = await widget.api.getRoutine(widget.routineId);
      final occurrences = await widget.api.listOccurrences(
        module: widget.moduleId,
      );
      final logs = await widget.api.listRoutineLogs(widget.routineId);
      if (!mounted) return;
      setState(() {
        _routine = routine;
        _occurrences = occurrences;
        _logs = logs;
      });
    } on ApiException catch (e) {
      if (mounted) setState(() => _error = e);
    }
  }

  PlainDate get _today => _occurrences!.from;

  void _show(String message, {SnackBarAction? action}) =>
      showRoutineSnackBar(context, message, action: action);

  /// Runs an action, then reloads; errors show the backend's message.
  Future<void> _run(Future<void> Function() action) async {
    setState(() => _busy = true);
    try {
      await action();
      widget.onChanged?.call();
    } on ApiException catch (e) {
      if (mounted) _show(e.message);
    }
    if (!mounted) return;
    setState(() => _busy = false);
    await _load();
  }

  Future<bool> _confirm({
    required String title,
    required String content,
    required String action,
  }) async {
    final theme = Theme.of(context);
    final confirmed = await showDialog<bool>(
      context: context,
      builder: (context) => AlertDialog(
        title: Text(title),
        content: Text(content, style: theme.textTheme.bodyLarge),
        actions: [
          TextButton(
            onPressed: () => Navigator.pop(context, false),
            child: const Text('Abbrechen'),
          ),
          FilledButton(
            style: FilledButton.styleFrom(
              backgroundColor: theme.colorScheme.error,
              foregroundColor: theme.colorScheme.onError,
            ),
            onPressed: () => Navigator.pop(context, true),
            child: Text(action),
          ),
        ],
      ),
    );
    return confirmed == true;
  }

  Future<void> _pause(Routine routine) async {
    final today = _today;
    final choice = await showDialog<_PauseChoice>(
      context: context,
      builder: (context) => SimpleDialog(
        title: const Text('Routine pausieren'),
        children: [
          for (final (choice, label) in const [
            (_PauseChoice.untilResumed, 'Bis ich fortsetze'),
            (_PauseChoice.week, 'Für eine Woche'),
            (_PauseChoice.date, 'Bis zu einem Datum …'),
          ])
            SimpleDialogOption(
              padding: const EdgeInsets.symmetric(horizontal: 24, vertical: 16),
              onPressed: () => Navigator.pop(context, choice),
              child: Text(
                label,
                style: Theme.of(context).textTheme.titleMedium,
              ),
            ),
        ],
      ),
    );
    if (choice == null || !mounted) return;
    PlainDate? until;
    switch (choice) {
      case _PauseChoice.untilResumed:
        until = null;
      case _PauseChoice.week:
        // Today and the six days after it.
        until = PlainDate.fromDateTime(
          DateTime.utc(today.year, today.month, today.day + 6),
        );
      case _PauseChoice.date:
        final picked = await showDatePicker(
          context: context,
          helpText: 'Pausieren bis',
          initialDate: today.toDateTime(),
          firstDate: today.toDateTime(),
          lastDate: DateTime(2099, 12, 31),
        );
        if (picked == null || !mounted) return;
        until = PlainDate.fromDateTime(picked);
    }
    await _run(() async {
      await widget.api.pauseRoutine(routine.id, until: until);
      if (mounted) _show('„${routine.name}“ pausiert.');
    });
  }

  Future<void> _resume(Routine routine) => _run(() async {
    await widget.api.resumeRoutine(routine.id);
    if (mounted) _show('„${routine.name}“ läuft wieder.');
  });

  Future<void> _edit(Routine routine) async {
    final saved = await openPage<Routine>(
      context,
      RoutineFormScreen(
        api: widget.api,
        moduleId: widget.moduleId,
        today: _today,
        routine: routine,
      ),
    );
    if (saved == null || !mounted) return;
    widget.onChanged?.call();
    _show('Änderungen gespeichert.');
    await _load();
  }

  Future<void> _delete(Routine routine) async {
    final confirmed = await _confirm(
      title: 'Routine löschen?',
      content:
          '„${routine.name}“ und der ganze Verlauf werden gelöscht. Das lässt '
          'sich nicht rückgängig machen.',
      action: 'Löschen',
    );
    if (!confirmed || !mounted) return;
    setState(() => _busy = true);
    try {
      await widget.api.deleteRoutine(routine.id);
      widget.onChanged?.call();
      if (mounted) Navigator.pop(context, true);
    } on ApiException catch (e) {
      if (!mounted) return;
      setState(() => _busy = false);
      _show(e.message);
    }
  }

  Future<void> _openOccurrence(Routine routine, Occurrence occurrence) async {
    final result = await showOccurrenceSheet(
      context,
      api: widget.api,
      routine: routine,
      occurrence: occurrence,
      today: _today,
    );
    if (result == null || !mounted) return;
    if (result.alreadyLoggedMessage case final message?) _show(message);
    widget.onChanged?.call();
    await _load();
  }

  Future<void> _takeBack(Routine routine, Occurrence occurrence) async {
    final confirmed = await _confirm(
      title: 'Eintrag zurücknehmen?',
      content: 'Der Termin um ${occurrence.time} ist danach wieder offen.',
      action: 'Zurücknehmen',
    );
    if (!confirmed) return;
    await _run(
      () => widget.api.deleteRoutineLog(routine.id, occurrence.logId!),
    );
  }

  Future<void> _deleteLog(Routine routine, RoutineLog log) async {
    final confirmed = await _confirm(
      title: 'Verlaufseintrag löschen?',
      content: log.dueAt == null
          ? 'Der Eintrag wird gelöscht.'
          : 'Der Eintrag wird gelöscht; sein Termin ist danach wieder offen.',
      action: 'Löschen',
    );
    if (!confirmed) return;
    await _run(() => widget.api.deleteRoutineLog(routine.id, log.id));
  }

  Future<void> _extraAmount(Routine routine) async {
    final amount = await showDialog<double>(
      context: context,
      builder: (_) => _ExtraAmountDialog(routine: routine),
    );
    if (amount == null || !mounted) return;
    await _run(() async {
      await widget.api.addRoutineLog(
        routine.id,
        NewRoutineLog(
          status: LogStatus.done,
          source: LogSource.manual,
          amount: amount,
        ),
      );
      if (mounted) {
        _show('${amountWithUnit(amount, routine.unit)} eingetragen.');
      }
    });
  }

  @override
  Widget build(BuildContext context) {
    final routine = _routine;
    return Scaffold(
      appBar: AppBar(title: Text(routine?.name ?? 'Routine')),
      body: switch ((routine, _error)) {
        (null, final ApiException error) => ErrorView(
          error: error,
          onRetry: _load,
        ),
        (null, _) => const Center(child: CircularProgressIndicator()),
        (final Routine routine, _) => _content(routine),
      },
    );
  }

  Widget _content(Routine routine) {
    final theme = Theme.of(context);
    final today = _today;
    final occurrences = _occurrences!.of(routine.id);
    final todays = [
      for (final o in occurrences)
        if (o.date == today) o,
    ];
    final upcoming = occurrences
        .where((o) => o.date.compareTo(today) > 0)
        .take(3)
        .toList();
    final logs = _logs ?? const <RoutineLog>[];
    final progress = routine.progress;
    final period = progress.period;
    final pause = routine.pauseOn(today);

    Widget heading(String text) => Padding(
      padding: const EdgeInsets.only(top: 24, bottom: 8),
      child: Text(text, style: theme.textTheme.titleLarge),
    );

    return RefreshIndicator(
      onRefresh: _load,
      child: ListView(
        padding: const EdgeInsets.fromLTRB(16, 8, 16, 32),
        children: [
          if (_error case final error?)
            Text(
              error.message,
              style: theme.textTheme.bodyLarge?.copyWith(
                color: theme.colorScheme.error,
              ),
            ),
          Card(
            child: Padding(
              padding: const EdgeInsets.all(16),
              child: Column(
                crossAxisAlignment: CrossAxisAlignment.start,
                children: [
                  Chip(
                    label: Text(
                      routine.status == RoutineStatus.paused &&
                              pause?.until != null
                          ? 'pausiert bis ${pause!.until!.display}'
                          : routineStatusLabel(routine.status),
                      style: theme.textTheme.titleMedium,
                    ),
                  ),
                  const SizedBox(height: 8),
                  Text(
                    capitalize(describeSchedule(routine.schedule)),
                    style: theme.textTheme.titleMedium,
                  ),
                  if (routine.amount != null)
                    Text(
                      'Menge: ${amountWithUnit(routine.amount!, routine.unit)}',
                      style: theme.textTheme.bodyLarge,
                    ),
                  Text(
                    'Beginn: ${routine.startDate.display}',
                    style: theme.textTheme.bodyLarge,
                  ),
                  Text(
                    routine.endDate == null
                        ? 'Ende: ohne'
                        : 'Ende: ${routine.endDate!.display}',
                    style: theme.textTheme.bodyLarge,
                  ),
                ],
              ),
            ),
          ),
          heading('Fortschritt'),
          Card(
            child: Padding(
              padding: const EdgeInsets.all(16),
              child: Column(
                crossAxisAlignment: CrossAxisAlignment.start,
                children: [
                  for (final (label, counts) in [
                    ('heute', progress.today),
                    ('Woche', progress.week),
                  ]) ...[
                    Text(
                      amountText(label, counts, routine.unit) ??
                          countText(label, counts),
                      style: theme.textTheme.titleMedium,
                    ),
                    if (counts.amountDone != null)
                      Text(
                        '${counts.done} von ${counts.due} Terminen erledigt',
                        style: theme.textTheme.bodyLarge,
                      ),
                    const SizedBox(height: 8),
                  ],
                  if (period != null) ...[
                    Text(
                      periodDayText(period),
                      style: theme.textTheme.titleMedium,
                    ),
                    Text(
                      '${period.done} von ${period.due} Terminen erledigt'
                      '${period.skipped > 0 ? ', ${period.skipped} ausgelassen' : ''}',
                      style: theme.textTheme.bodyLarge,
                    ),
                  ],
                ],
              ),
            ),
          ),
          // Today's occurrences, then at most three after today: an hourly
          // routine has 91 in seven days, too many to read.
          heading('Termine heute'),
          if (todays.isEmpty)
            Text(
              'Heute gibt es keinen Termin.',
              style: theme.textTheme.bodyLarge,
            )
          else
            Card(
              child: Column(
                children: [
                  for (final o in todays)
                    _OccurrenceRow(
                      occurrence: o,
                      today: today,
                      busy: _busy,
                      onLog: () => _openOccurrence(routine, o),
                      onTakeBack: () => _takeBack(routine, o),
                    ),
                ],
              ),
            ),
          heading('Nächste Termine'),
          if (upcoming.isEmpty)
            Text(
              'In den nächsten 7 Tagen gibt es keinen weiteren Termin.',
              style: theme.textTheme.bodyLarge,
            )
          else
            Card(
              child: Column(
                children: [
                  for (final o in upcoming)
                    _OccurrenceRow(
                      occurrence: o,
                      today: today,
                      busy: _busy,
                      onLog: () => _openOccurrence(routine, o),
                      onTakeBack: () => _takeBack(routine, o),
                    ),
                ],
              ),
            ),
          if (routine.hasAmount) ...[
            const SizedBox(height: 12),
            OutlinedButton.icon(
              style: largeOutlined(context),
              onPressed: _busy ? null : () => _extraAmount(routine),
              icon: const Icon(Icons.add),
              label: const Text('Zusätzlich eintragen'),
            ),
          ],
          heading('Verlauf'),
          if (logs.isEmpty)
            Text(
              'In den letzten 30 Tagen gibt es keinen Verlaufseintrag.',
              style: theme.textTheme.bodyLarge,
            )
          else
            Card(
              child: Column(
                children: [
                  for (final log in logs)
                    ListTile(
                      title: Text(
                        '${formatLocalTimestamp(log.takenAt)} · '
                        '${log.status == LogStatus.done ? 'erledigt' : 'ausgelassen'}'
                        '${log.amount != null && routine.hasAmount ? ' (${amountWithUnit(log.amount!, routine.unit)})' : ''}',
                        style: theme.textTheme.bodyLarge,
                      ),
                      subtitle: Text(switch (log.source) {
                        LogSource.manual =>
                          log.dueAt == null
                              ? 'zusätzlich, in der App'
                              : 'in der App',
                        LogSource.notification => 'über die Erinnerung',
                        LogSource.entry => 'aus einem Dokument',
                      }),
                      trailing: IconButton(
                        tooltip: 'Verlaufseintrag löschen',
                        icon: const Icon(Icons.delete_outline),
                        onPressed: _busy
                            ? null
                            : () => _deleteLog(routine, log),
                      ),
                    ),
                ],
              ),
            ),
          heading('Aktionen'),
          if (routine.status == RoutineStatus.paused)
            FilledButton.icon(
              style: largeFilled(context),
              onPressed: _busy ? null : () => _resume(routine),
              icon: const Icon(Icons.play_arrow),
              label: const Text('Fortsetzen'),
            )
          else if (routine.status != RoutineStatus.ended)
            OutlinedButton.icon(
              style: largeOutlined(context),
              onPressed: _busy ? null : () => _pause(routine),
              icon: const Icon(Icons.pause),
              label: const Text('Pausieren'),
            ),
          const SizedBox(height: 12),
          OutlinedButton.icon(
            style: largeOutlined(context),
            onPressed: _busy ? null : () => _edit(routine),
            icon: const Icon(Icons.edit_outlined),
            label: const Text('Bearbeiten'),
          ),
          const SizedBox(height: 12),
          OutlinedButton.icon(
            style: largeOutlined(context).copyWith(
              foregroundColor: WidgetStatePropertyAll(theme.colorScheme.error),
            ),
            onPressed: _busy ? null : () => _delete(routine),
            icon: const Icon(Icons.delete_outline),
            label: const Text('Löschen'),
          ),
        ],
      ),
    );
  }
}

enum _PauseChoice { untilResumed, week, date }

/// One occurrence in the detail: when, status, and „Eintragen“ (open, up to
/// today) or „Zurücknehmen“ (done or skipped).
class _OccurrenceRow extends StatelessWidget {
  const _OccurrenceRow({
    required this.occurrence,
    required this.today,
    required this.busy,
    required this.onLog,
    required this.onTakeBack,
  });

  final Occurrence occurrence;
  final PlainDate today;
  final bool busy;
  final VoidCallback onLog;
  final VoidCallback onTakeBack;

  @override
  Widget build(BuildContext context) {
    final theme = Theme.of(context);
    final o = occurrence;
    final overdue = o.isOverdue(DateTime.now().toUtc());
    final (icon, color) = switch (o.status) {
      OccurrenceStatus.done => (Icons.check_circle, theme.colorScheme.primary),
      OccurrenceStatus.skipped => (Icons.remove_circle_outline, null),
      OccurrenceStatus.paused => (Icons.pause_circle_outline, null),
      OccurrenceStatus.open when overdue => (
        Icons.error_outline,
        theme.colorScheme.error,
      ),
      OccurrenceStatus.open => (Icons.radio_button_unchecked, null),
    };
    final canLog = o.isOpen && o.date.compareTo(today) <= 0;
    return ListTile(
      leading: Icon(icon, color: color, size: 32),
      title: Text(occurrenceWhen(o, today), style: theme.textTheme.titleMedium),
      subtitle: Text(
        o.isOpen && overdue ? 'noch offen' : occurrenceStatusLabel(o.status),
      ),
      trailing: canLog
          ? FilledButton.tonal(
              onPressed: busy ? null : onLog,
              child: const Text('Eintragen'),
            )
          : o.logId != null
          ? TextButton(
              onPressed: busy ? null : onTakeBack,
              child: const Text('Zurücknehmen'),
            )
          : null,
    );
  }
}

/// „Zusätzlich eintragen“: an amount without an occurrence, e.g. an extra
/// glass of water or a portion (docs/API.md 3.16, „Eiweiß in Portionen“).
class _ExtraAmountDialog extends StatefulWidget {
  const _ExtraAmountDialog({required this.routine});

  final Routine routine;

  @override
  State<_ExtraAmountDialog> createState() => _ExtraAmountDialogState();
}

class _ExtraAmountDialogState extends State<_ExtraAmountDialog> {
  late final _amount = TextEditingController(
    text: formatAmount(widget.routine.amount!),
  );
  String? _error;

  @override
  void dispose() {
    _amount.dispose();
    super.dispose();
  }

  void _submit() {
    final amount = parseAmount(_amount.text);
    if (amount == null) {
      setState(() => _error = 'Bitte eine Menge eingeben, z. B. 250 oder 0,5.');
    } else if (!(amount > 0) || amount > 100000) {
      setState(
        () =>
            _error = 'Die Menge muss größer als 0 und höchstens 100.000 sein.',
      );
    } else {
      Navigator.pop(context, amount);
    }
  }

  @override
  Widget build(BuildContext context) {
    return AlertDialog(
      title: const Text('Zusätzlich eintragen'),
      content: TextField(
        key: const ValueKey('extra-amount'),
        controller: _amount,
        autofocus: true,
        style: Theme.of(context).textTheme.titleLarge,
        keyboardType: const TextInputType.numberWithOptions(decimal: true),
        onSubmitted: (_) => _submit(),
        decoration: InputDecoration(
          labelText: 'Menge',
          suffixText: widget.routine.unit,
          helperText: 'Zählt für die Menge, nicht als Termin.',
          errorText: _error,
          errorMaxLines: 3,
        ),
      ),
      actions: [
        TextButton(
          onPressed: () => Navigator.pop(context),
          child: const Text('Abbrechen'),
        ),
        FilledButton(onPressed: _submit, child: const Text('Eintragen')),
      ],
    );
  }
}
