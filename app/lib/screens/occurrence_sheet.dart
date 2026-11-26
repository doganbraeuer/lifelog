import 'package:flutter/material.dart';

import '../api/api_client.dart';
import '../api/api_exception.dart';
import '../model/plain_date.dart';
import '../model/routine.dart';
import '../model/routine_texts.dart';
import 'routine_actions.dart';

/// One occurrence („Termin“) in a bottom sheet with large buttons: „Ja,
/// erledigt“, „Nein, ausgelassen“ and, for a routine with an amount, the
/// amount actually taken. Opened by „Andere Menge“, by a reminder and from
/// the routine's detail.
///
/// Returns a [LogResult] once something was logged (or the occurrence
/// turned out to be logged already), `null` if the sheet was closed.
Future<LogResult?> showOccurrenceSheet(
  BuildContext context, {
  required ApiClient api,
  required Routine routine,
  required Occurrence occurrence,
  required PlainDate today,
  LogSource source = LogSource.manual,
  bool focusAmount = false,
}) => showModalBottomSheet<LogResult>(
  context: context,
  isScrollControlled: true,
  useSafeArea: true,
  showDragHandle: true,
  builder: (_) => OccurrenceSheet(
    api: api,
    routine: routine,
    occurrence: occurrence,
    today: today,
    source: source,
    focusAmount: focusAmount,
  ),
);

class OccurrenceSheet extends StatefulWidget {
  const OccurrenceSheet({
    super.key,
    required this.api,
    required this.routine,
    required this.occurrence,
    required this.today,
    this.source = LogSource.manual,
    this.focusAmount = false,
  });

  final ApiClient api;
  final Routine routine;
  final Occurrence occurrence;

  /// Today in the user's time zone, from the backend.
  final PlainDate today;

  /// `notification` when a reminder opened the sheet (3.16 "Logs").
  final LogSource source;

  /// Put the cursor into the amount field („Andere Menge“).
  final bool focusAmount;

  @override
  State<OccurrenceSheet> createState() => _OccurrenceSheetState();
}

class _OccurrenceSheetState extends State<OccurrenceSheet> {
  late final _amount = TextEditingController(
    text: widget.routine.amount == null
        ? ''
        : formatAmount(widget.routine.amount!),
  );
  var _busy = false;
  String? _error;

  @override
  void dispose() {
    _amount.dispose();
    super.dispose();
  }

  Future<void> _send(LogStatus status, {double? amount}) async {
    setState(() {
      _busy = true;
      _error = null;
    });
    try {
      final result = await logOccurrence(
        widget.api,
        routineId: widget.routine.id,
        dueAt: widget.occurrence.dueAt,
        status: status,
        source: widget.source,
        amount: amount,
      );
      if (mounted) Navigator.pop(context, result);
    } on ApiException catch (e) {
      if (!mounted) return;
      setState(() {
        _busy = false;
        _error = e.message;
      });
    }
  }

  void _saveAmount() {
    final amount = parseAmount(_amount.text);
    if (amount == null) {
      setState(() => _error = 'Bitte eine Menge eingeben, z. B. 250 oder 0,5.');
      return;
    }
    if (!(amount > 0) || amount > 100000) {
      // The same rule and text as the backend (3.16).
      setState(
        () =>
            _error = 'Die Menge muss größer als 0 und höchstens 100.000 sein.',
      );
      return;
    }
    _send(LogStatus.done, amount: amount);
  }

  @override
  Widget build(BuildContext context) {
    final theme = Theme.of(context);
    final routine = widget.routine;
    final occurrence = widget.occurrence;
    final open = occurrence.isOpen;
    final overdue = occurrence.isOverdue(DateTime.now().toUtc());
    return Padding(
      padding: EdgeInsets.fromLTRB(
        24,
        0,
        24,
        24 + MediaQuery.viewInsetsOf(context).bottom,
      ),
      child: Column(
        mainAxisSize: MainAxisSize.min,
        crossAxisAlignment: CrossAxisAlignment.stretch,
        children: [
          Text(routine.name, style: theme.textTheme.headlineSmall),
          const SizedBox(height: 4),
          Text(
            'Termin: ${occurrenceWhen(occurrence, widget.today)}'
            '${open && overdue ? ' (noch offen)' : ''}',
            style: theme.textTheme.titleMedium,
          ),
          if (routine.amount != null)
            Text(
              'Menge: ${amountWithUnit(routine.amount!, routine.unit)}',
              style: theme.textTheme.bodyLarge,
            ),
          const SizedBox(height: 20),
          if (!open) ...[
            Text(
              'Schon eingetragen: '
              '${occurrenceStatusLabel(occurrence.status)}.',
              style: theme.textTheme.titleMedium,
            ),
            const SizedBox(height: 16),
            OutlinedButton(
              style: largeOutlined(context),
              onPressed: () => Navigator.pop(context),
              child: const Text('Schließen'),
            ),
          ] else ...[
            if (routine.amount != null) ...[
              TextField(
                key: const ValueKey('occurrence-amount'),
                controller: _amount,
                autofocus: widget.focusAmount,
                enabled: !_busy,
                style: theme.textTheme.titleLarge,
                keyboardType: const TextInputType.numberWithOptions(
                  decimal: true,
                ),
                textInputAction: TextInputAction.done,
                onSubmitted: (_) => _saveAmount(),
                decoration: InputDecoration(
                  labelText: 'Andere Menge',
                  suffixText: routine.unit,
                  border: const OutlineInputBorder(),
                ),
              ),
              const SizedBox(height: 12),
              FilledButton.tonal(
                style: FilledButton.styleFrom(
                  minimumSize: largeButtonSize,
                  textStyle: theme.textTheme.titleMedium,
                ),
                onPressed: _busy ? null : _saveAmount,
                child: const Text('Menge speichern'),
              ),
              const SizedBox(height: 20),
            ],
            FilledButton(
              style: largeFilled(context),
              onPressed: _busy ? null : () => _send(LogStatus.done),
              child: const Text('Ja, erledigt'),
            ),
            const SizedBox(height: 12),
            OutlinedButton(
              style: largeOutlined(context),
              onPressed: _busy ? null : () => _send(LogStatus.skipped),
              child: const Text('Nein, ausgelassen'),
            ),
          ],
          if (_error case final error?) ...[
            const SizedBox(height: 12),
            Text(
              error,
              style: theme.textTheme.bodyLarge?.copyWith(
                color: theme.colorScheme.error,
              ),
            ),
          ],
        ],
      ),
    );
  }
}
