import 'package:flutter/material.dart';

import '../api/api_client.dart';
import '../api/api_exception.dart';
import '../model/plain_date.dart';
import '../model/routine.dart';
import '../model/routine_texts.dart';
import '../widgets/error_view.dart';
import '../widgets/mock_banner.dart';
import '../widgets/navigation.dart';
import 'occurrence_sheet.dart';
import 'routine_actions.dart';

/// Tab „Routinen“ of a module: today's occurrences with large buttons „Ja“,
/// „Nein“ and „Andere Menge“, then the other routines with their status.
///
/// The backend computes occurrences, status and progress (docs/API.md
/// 3.16); this screen only shows them. "Today" is the `from` of
/// `GET /routines/occurrences` without parameters: today in the user's
/// time zone, from the server, not from the phone's clock.
class RoutinesScreen extends StatefulWidget {
  const RoutinesScreen({
    super.key,
    required this.api,
    required this.moduleId,
    this.onChanged,
  });

  final ApiClient api;
  final String moduleId;

  /// After every change (a log, a new routine, …), e.g. to plan the
  /// reminders again.
  final VoidCallback? onChanged;

  @override
  State<RoutinesScreen> createState() => RoutinesScreenState();
}

class RoutinesScreenState extends State<RoutinesScreen> {
  List<Routine>? _routines;
  OccurrenceList? _occurrences;
  ApiException? _error;
  var _loading = false;

  /// Only the last of several overlapping loads counts.
  var _requestNumber = 0;

  /// Routines with a request under way: their buttons are disabled, so a
  /// double tap cannot send twice.
  final _busy = <String>{};

  @override
  void initState() {
    super.initState();
    reload();
  }

  /// Loads routines and occurrences (today and six days). Public, so that
  /// the shell can refresh it.
  Future<void> reload() async {
    final number = ++_requestNumber;
    setState(() {
      _loading = true;
      _error = null;
    });
    try {
      // One after the other: an error of the first never leaves the second
      // unhandled, and both are quick (no model call).
      final routines = await widget.api.listRoutines(module: widget.moduleId);
      final occurrences = await widget.api.listOccurrences(
        module: widget.moduleId,
      );
      if (!mounted || number != _requestNumber) return;
      setState(() {
        _routines = routines;
        _occurrences = occurrences;
        _loading = false;
      });
    } on ApiException catch (error) {
      if (!mounted || number != _requestNumber) return;
      setState(() {
        _error = error;
        _loading = false;
      });
    }
  }

  PlainDate get _today => _occurrences!.from;

  void _changed() {
    widget.onChanged?.call();
    reload();
  }

  void _show(String message, {SnackBarAction? action}) {
    final messenger = ScaffoldMessenger.of(context);
    messenger
      ..hideCurrentSnackBar()
      ..showSnackBar(SnackBar(content: Text(message), action: action));
  }

  /// „Ja“ or „Nein“ on a card.
  Future<void> _log(
    Routine routine,
    Occurrence occurrence,
    LogStatus status,
  ) async {
    setState(() => _busy.add(routine.id));
    try {
      final result = await logOccurrence(
        widget.api,
        routineId: routine.id,
        dueAt: occurrence.dueAt,
        status: status,
        source: LogSource.manual,
      );
      if (!mounted) return;
      _report(routine, result);
    } on ApiException catch (e) {
      if (mounted) _show(e.message);
    } finally {
      if (mounted) setState(() => _busy.remove(routine.id));
    }
    if (mounted) _changed();
  }

  /// „Andere Menge“: the occurrence in the sheet with the amount field.
  Future<void> _otherAmount(Routine routine, Occurrence occurrence) async {
    final result = await showOccurrenceSheet(
      context,
      api: widget.api,
      routine: routine,
      occurrence: occurrence,
      today: _today,
      focusAmount: true,
    );
    if (result == null || !mounted) return;
    _report(routine, result);
    _changed();
  }

  /// Confirms a log with „Rückgängig“, or shows that it was done already.
  void _report(Routine routine, LogResult result) {
    final log = result.log;
    if (log == null) {
      _show(result.alreadyLoggedMessage!);
      return;
    }
    final what = log.status == LogStatus.done ? 'erledigt' : 'ausgelassen';
    _show(
      '„${routine.name}“ $what.',
      action: SnackBarAction(
        label: 'Rückgängig',
        onPressed: () => _undo(routine, log),
      ),
    );
  }

  Future<void> _undo(Routine routine, RoutineLog log) async {
    try {
      await widget.api.deleteRoutineLog(routine.id, log.id);
    } on ApiException catch (e) {
      if (mounted) _show(e.message);
    }
    if (mounted) _changed();
  }

  @override
  Widget build(BuildContext context) {
    return Scaffold(
      appBar: AppBar(
        leading: const ModulesButton(),
        title: const Text('Routinen'),
        actions: [MockBanner(visible: widget.api.isMock)],
        bottom: PreferredSize(
          preferredSize: const Size.fromHeight(4),
          child: SizedBox(
            height: 4,
            child: _loading && _routines != null
                ? const LinearProgressIndicator()
                : null,
          ),
        ),
      ),
      body: _body(),
    );
  }

  Widget _body() {
    if (_error case final error? when _routines == null) {
      return ErrorView(error: error, onRetry: reload);
    }
    final routines = _routines;
    if (routines == null) {
      return const Center(child: CircularProgressIndicator());
    }
    if (routines.isEmpty) return const _EmptyRoutines();

    final today = _today;
    final occurrences = _occurrences!;
    final todays = <(Routine, List<Occurrence>)>[];
    final others = <Routine>[];
    for (final routine in routines) {
      final list = [
        for (final o in occurrences.of(routine.id))
          if (o.date == today) o,
      ];
      if (routine.status == RoutineStatus.active && list.isNotEmpty) {
        todays.add((routine, list));
      } else {
        others.add(routine);
      }
    }
    // Open ones first, by their next occurrence; finished ones after. At the
    // same time the backend's order stays (List.sort is not stable).
    int nextOpen((Routine, List<Occurrence>) item) {
      final open = item.$2.where((o) => o.isOpen);
      return open.isEmpty ? 1 << 62 : open.first.dueAt.millisecondsSinceEpoch;
    }

    final position = {for (final (i, r) in routines.indexed) r.id: i};
    todays.sort((a, b) {
      final byTime = nextOpen(a).compareTo(nextOpen(b));
      return byTime != 0
          ? byTime
          : position[a.$1.id]!.compareTo(position[b.$1.id]!);
    });

    final theme = Theme.of(context);
    return RefreshIndicator(
      onRefresh: reload,
      child: ListView(
        padding: const EdgeInsets.fromLTRB(16, 16, 16, 96),
        children: [
          // Reloading failed: the last list stays, with the error above it.
          if (_error case final error?)
            Card(
              margin: const EdgeInsets.only(bottom: 16),
              child: ListTile(
                leading: Icon(
                  Icons.error_outline,
                  color: theme.colorScheme.error,
                ),
                title: Text(error.message),
                trailing: TextButton(
                  onPressed: reload,
                  child: const Text('Erneut versuchen'),
                ),
              ),
            ),
          if (todays.isNotEmpty) ...[
            Text('Heute', style: theme.textTheme.titleLarge),
            const SizedBox(height: 8),
            for (final (routine, list) in todays)
              _TodayCard(
                key: ValueKey('today-${routine.id}'),
                routine: routine,
                occurrences: list,
                busy: _busy.contains(routine.id),
                onLog: (o, status) => _log(routine, o, status),
                onOtherAmount: (o) => _otherAmount(routine, o),
              ),
          ],
          if (others.isNotEmpty) ...[
            const SizedBox(height: 16),
            Text('Weitere Routinen', style: theme.textTheme.titleLarge),
            const SizedBox(height: 8),
            for (final routine in others)
              _OtherRoutineTile(
                key: ValueKey('other-${routine.id}'),
                routine: routine,
                today: today,
                next: occurrences
                    .of(routine.id)
                    .where((o) => o.isOpen && o.date != today)
                    .firstOrNull,
              ),
          ],
        ],
      ),
    );
  }
}

class _EmptyRoutines extends StatelessWidget {
  const _EmptyRoutines();

  @override
  Widget build(BuildContext context) {
    final theme = Theme.of(context);
    return Center(
      child: Padding(
        padding: const EdgeInsets.all(32),
        child: Column(
          mainAxisSize: MainAxisSize.min,
          children: [
            Icon(Icons.alarm, size: 64, color: theme.colorScheme.primary),
            const SizedBox(height: 16),
            Text(
              'Noch keine Routinen',
              style: theme.textTheme.titleLarge,
              textAlign: TextAlign.center,
            ),
            const SizedBox(height: 8),
            Text(
              'Eine Routine erinnert dich an etwas, das du regelmäßig tun '
              'möchtest, zum Beispiel Vitamin D jeden Morgen.',
              style: theme.textTheme.bodyLarge,
              textAlign: TextAlign.center,
            ),
          ],
        ),
      ),
    );
  }
}

/// A routine with occurrences today: progress, the next open occurrence and
/// the buttons.
class _TodayCard extends StatelessWidget {
  const _TodayCard({
    super.key,
    required this.routine,
    required this.occurrences,
    required this.busy,
    required this.onLog,
    required this.onOtherAmount,
  });

  final Routine routine;

  /// Today's occurrences of the routine, sorted by point in time.
  final List<Occurrence> occurrences;
  final bool busy;
  final void Function(Occurrence, LogStatus) onLog;
  final void Function(Occurrence) onOtherAmount;

  @override
  Widget build(BuildContext context) {
    final theme = Theme.of(context);
    final today = routine.progress.today;
    final amount = amountText('heute', today, routine.unit);
    final next = occurrences.where((o) => o.isOpen).firstOrNull;
    final overdue = next != null && next.isOverdue(DateTime.now().toUtc());
    final fraction = switch ((today.amountDone, today.amountTarget)) {
      (final done?, final target?) when target > 0 => done / target,
      _ => today.due == 0 ? 0.0 : today.done / today.due,
    };
    return Card(
      margin: const EdgeInsets.only(bottom: 12),
      child: Padding(
        padding: const EdgeInsets.all(16),
        child: Column(
          crossAxisAlignment: CrossAxisAlignment.stretch,
          children: [
            Text(routine.name, style: theme.textTheme.titleLarge),
            const SizedBox(height: 4),
            Text(
              amount ?? countText('heute', today),
              style: theme.textTheme.titleMedium,
            ),
            if (amount != null)
              Text(
                '${countText('', today).trim()} Terminen erledigt',
                style: theme.textTheme.bodyMedium,
              ),
            const SizedBox(height: 8),
            ClipRRect(
              borderRadius: BorderRadius.circular(4),
              child: LinearProgressIndicator(
                value: fraction.clamp(0.0, 1.0),
                minHeight: 8,
              ),
            ),
            const SizedBox(height: 12),
            if (next == null)
              Row(
                children: [
                  Icon(Icons.check_circle, color: theme.colorScheme.primary),
                  const SizedBox(width: 8),
                  Text(
                    'Für heute erledigt',
                    style: theme.textTheme.titleMedium,
                  ),
                ],
              )
            else ...[
              Text(
                overdue
                    ? 'Termin um ${next.time} – noch offen'
                    : 'Nächster Termin: ${next.time}',
                style: theme.textTheme.titleMedium?.copyWith(
                  color: overdue ? theme.colorScheme.error : null,
                ),
              ),
              const SizedBox(height: 12),
              Wrap(
                spacing: 12,
                runSpacing: 12,
                children: [
                  FilledButton(
                    style: largeFilled(context),
                    onPressed: busy ? null : () => onLog(next, LogStatus.done),
                    child: const Text('Ja'),
                  ),
                  OutlinedButton(
                    style: largeOutlined(context),
                    onPressed: busy
                        ? null
                        : () => onLog(next, LogStatus.skipped),
                    child: const Text('Nein'),
                  ),
                  if (routine.hasAmount)
                    OutlinedButton(
                      style: largeOutlined(context),
                      onPressed: busy ? null : () => onOtherAmount(next),
                      child: const Text('Andere Menge'),
                    ),
                ],
              ),
            ],
          ],
        ),
      ),
    );
  }
}

/// A routine without occurrences today: paused, upcoming, ended, or just
/// not due today.
class _OtherRoutineTile extends StatelessWidget {
  const _OtherRoutineTile({
    super.key,
    required this.routine,
    required this.today,
    required this.next,
  });

  final Routine routine;
  final PlainDate today;

  /// The next open occurrence within the loaded days, if any.
  final Occurrence? next;

  String _state() {
    switch (routine.status) {
      case RoutineStatus.paused:
        final until = routine.pauseOn(today)?.until;
        return until == null ? 'pausiert' : 'pausiert bis ${until.display}';
      case RoutineStatus.upcoming:
        return 'beginnt am ${routine.startDate.display}';
      case RoutineStatus.ended:
        return 'beendet am ${routine.endDate?.display ?? ''}';
      case RoutineStatus.active:
        final next = this.next;
        return next == null
            ? 'kein Termin in den nächsten 7 Tagen'
            : 'Nächster Termin: ${occurrenceWhen(next, today)}';
    }
  }

  @override
  Widget build(BuildContext context) {
    final theme = Theme.of(context);
    return Card(
      margin: const EdgeInsets.only(bottom: 12),
      child: ListTile(
        contentPadding: const EdgeInsets.symmetric(horizontal: 16, vertical: 8),
        title: Text(routine.name, style: theme.textTheme.titleLarge),
        subtitle: Text(
          '${_state()}\n${capitalize(describeSchedule(routine.schedule))}',
          style: theme.textTheme.bodyLarge,
        ),
        isThreeLine: true,
      ),
    );
  }
}
