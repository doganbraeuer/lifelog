// Dialogs for correcting single fields of an entry.
//
// Every dialog returns `null` if the user cancels. Otherwise it returns
// the new value; the entry screen checks before saving whether anything
// really changed.

import 'package:flutter/material.dart';

import '../model/plain_date.dart';
import '../model/entry.dart';
import '../model/entry_patch.dart';
import '../module/module_catalog.dart';

// ---------------------------------------------------------------------------
// Type
// ---------------------------------------------------------------------------

/// Choose the type from a dropdown. The choices are the types of the
/// module [moduleId] from the module catalog.
Future<String?> showTypeDialog(
  BuildContext context, {
  required String moduleId,
  required String current,
  required Correction? correction,
}) => showDialog<String>(
  context: context,
  builder: (_) => _TypeDialog(
    moduleId: moduleId,
    current: current,
    recognized: correction?.recognized,
  ),
);

class _TypeDialog extends StatefulWidget {
  const _TypeDialog({
    required this.moduleId,
    required this.current,
    required this.recognized,
  });

  final String moduleId;
  final String current;

  /// Originally recognized type, only set if the type was corrected.
  final String? recognized;

  @override
  State<_TypeDialog> createState() => _TypeDialogState();
}

class _TypeDialogState extends State<_TypeDialog> {
  late String _selection = widget.current;

  @override
  Widget build(BuildContext context) {
    final catalog = ModuleCatalog.of(context);
    // The current type stays selectable even if the module does not know it
    // (any more), e.g. for data from a newer backend.
    final types = {
      for (final t in catalog.types(widget.moduleId)) t.id,
      widget.current,
    };
    String name(String type) => catalog.typeName(widget.moduleId, type);
    return AlertDialog(
      title: const Text('Typ korrigieren'),
      content: DropdownButtonFormField<String>(
        initialValue: _selection,
        decoration: const InputDecoration(labelText: 'Typ'),
        items: [
          for (final type in types)
            DropdownMenuItem(
              value: type,
              child: Text(
                type == widget.recognized
                    ? '${name(type)} (erkannt)'
                    : name(type),
              ),
            ),
        ],
        onChanged: (type) => setState(() => _selection = type ?? _selection),
      ),
      actions: [
        TextButton(
          onPressed: () => Navigator.pop(context),
          child: const Text('Abbrechen'),
        ),
        FilledButton(
          onPressed: () => Navigator.pop(context, _selection),
          child: const Text('Speichern'),
        ),
      ],
    );
  }
}

// ---------------------------------------------------------------------------
// Document date
// ---------------------------------------------------------------------------

/// Choice: pick a date, remove the date or restore the recognized date.
/// `Value(null)` means: no date.
Future<Value<PlainDate?>?> showDocumentDateDialog(
  BuildContext context, {
  required PlainDate? current,
  required Correction? correction,
}) async {
  final recognized = correction == null
      ? null
      : Value(
          correction.recognized == null
              ? null
              : PlainDate.tryParse(correction.recognized!),
        );

  final action = await showModalBottomSheet<_DateAction>(
    context: context,
    showDragHandle: true,
    builder: (context) => SafeArea(
      child: Column(
        mainAxisSize: MainAxisSize.min,
        children: [
          ListTile(
            leading: const Icon(Icons.calendar_month_outlined),
            title: const Text('Datum wählen'),
            onTap: () => Navigator.pop(context, _DateAction.pick),
          ),
          if (current != null)
            ListTile(
              leading: const Icon(Icons.event_busy_outlined),
              title: const Text('Datum entfernen'),
              onTap: () => Navigator.pop(context, _DateAction.remove),
            ),
          if (recognized != null)
            ListTile(
              leading: const Icon(Icons.undo),
              title: Text(
                'Erkanntes Datum wiederherstellen: '
                '${recognized.value?.display ?? 'kein Datum'}',
              ),
              onTap: () => Navigator.pop(context, _DateAction.restore),
            ),
        ],
      ),
    ),
  );
  if (action == null || !context.mounted) return null;

  switch (action) {
    case _DateAction.remove:
      return const Value(null);
    case _DateAction.restore:
      return recognized;
    case _DateAction.pick:
      final chosen = await showDatePicker(
        context: context,
        initialDate: initialPickerDate(current, DateTime.now()),
        firstDate: firstPickerDate,
        lastDate: lastPickerDate,
        helpText: 'Dokumentdatum wählen',
      );
      // Only take year, month and day, no time zone (see PlainDate).
      return chosen == null ? null : Value(PlainDate.fromDateTime(chosen));
  }
}

enum _DateAction { pick, remove, restore }

/// Range of the date picker.
final firstPickerDate = DateTime(1900);
final lastPickerDate = DateTime(2100, 12, 31);

/// Initial date of the date picker: the current document date, else today.
///
/// `showDatePicker` fails with an assertion if `initialDate` lies outside
/// `firstDate` to `lastDate`. But a wrongly recognized year (e.g. 1026
/// instead of 2026) is exactly what one wants to correct.
/// If the date lies outside, the picker therefore starts at [today].
DateTime initialPickerDate(PlainDate? current, DateTime today) {
  final date = current?.toDateTime();
  if (date == null ||
      date.isBefore(firstPickerDate) ||
      date.isAfter(lastPickerDate)) {
    return today;
  }
  return date;
}

// ---------------------------------------------------------------------------
// Issuer
// ---------------------------------------------------------------------------

/// Edit the issuer. An empty field removes the issuer (`Value(null)`).
Future<Value<String?>?> showIssuerDialog(
  BuildContext context, {
  required String? current,
  required Correction? correction,
}) => showDialog<Value<String?>>(
  context: context,
  builder: (_) => _IssuerDialog(current: current, correction: correction),
);

class _IssuerDialog extends StatefulWidget {
  const _IssuerDialog({required this.current, required this.correction});

  final String? current;
  final Correction? correction;

  @override
  State<_IssuerDialog> createState() => _IssuerDialogState();
}

class _IssuerDialogState extends State<_IssuerDialog> {
  late final _field = TextEditingController(text: widget.current ?? '');

  @override
  void dispose() {
    _field.dispose();
    super.dispose();
  }

  @override
  Widget build(BuildContext context) {
    final correction = widget.correction;
    return AlertDialog(
      title: const Text('Quelle korrigieren'),
      content: Column(
        mainAxisSize: MainAxisSize.min,
        crossAxisAlignment: CrossAxisAlignment.start,
        children: [
          _DialogTextField(
            controller: _field,
            label: 'Quelle',
            help: 'Leer lassen, um die Quelle zu entfernen.',
          ),
          if (correction != null)
            _UseRecognized(
              recognized: correction.recognized,
              accept: () => _field.text = correction.recognized ?? '',
            ),
        ],
      ),
      actions: [
        TextButton(
          onPressed: () => Navigator.pop(context),
          child: const Text('Abbrechen'),
        ),
        FilledButton(
          onPressed: () {
            final text = _field.text.trim();
            Navigator.pop(context, Value(text.isEmpty ? null : text));
          },
          child: const Text('Speichern'),
        ),
      ],
    );
  }
}

// ---------------------------------------------------------------------------
// Fact (value and unit)
// ---------------------------------------------------------------------------

/// Edit the value and unit of a fact. Returns only the fields that changed,
/// or `null` if nothing changed.
Future<FactPatch?> showFactDialog(
  BuildContext context, {
  required Fact fact,
  required Correction? valueCorrection,
  required Correction? unitCorrection,
}) => showDialog<FactPatch>(
  context: context,
  builder: (_) => _FactDialog(
    fact: fact,
    valueCorrection: valueCorrection,
    unitCorrection: unitCorrection,
  ),
);

class _FactDialog extends StatefulWidget {
  const _FactDialog({
    required this.fact,
    required this.valueCorrection,
    required this.unitCorrection,
  });

  final Fact fact;
  final Correction? valueCorrection;
  final Correction? unitCorrection;

  @override
  State<_FactDialog> createState() => _FactDialogState();
}

class _FactDialogState extends State<_FactDialog> {
  late final _value = TextEditingController(text: widget.fact.value);
  late final _unit = TextEditingController(text: widget.fact.unit ?? '');
  String? _valueError;

  @override
  void dispose() {
    _value.dispose();
    _unit.dispose();
    super.dispose();
  }

  void _save() {
    final value = _value.text.trim();
    if (value.isEmpty) {
      setState(() => _valueError = 'Der Wert darf nicht leer sein.');
      return;
    }
    final unitText = _unit.text.trim();
    final unit = unitText.isEmpty ? null : unitText;

    final valueChanged = value != widget.fact.value;
    final unitChanged = unit != widget.fact.unit;
    Navigator.pop(
      context,
      valueChanged || unitChanged
          ? FactPatch(
              widget.fact.id,
              value: valueChanged ? value : null,
              unit: unitChanged ? Value(unit) : null,
            )
          : null,
    );
  }

  @override
  Widget build(BuildContext context) {
    final valueCorrection = widget.valueCorrection;
    final unitCorrection = widget.unitCorrection;
    return AlertDialog(
      title: Text('${widget.fact.name} korrigieren'),
      content: SingleChildScrollView(
        child: Column(
          mainAxisSize: MainAxisSize.min,
          crossAxisAlignment: CrossAxisAlignment.start,
          children: [
            _DialogTextField(
              controller: _value,
              label: 'Wert',
              error: _valueError,
              help: 'Genau wie auf dem Dokument, z. B. „<0,5“.',
            ),
            if (valueCorrection != null)
              _UseRecognized(
                recognized: valueCorrection.recognized,
                accept: () => _value.text = valueCorrection.recognized ?? '',
              ),
            const SizedBox(height: 16),
            _DialogTextField(
              controller: _unit,
              label: 'Einheit',
              help: 'Leer lassen, wenn es keine Einheit gibt.',
            ),
            if (unitCorrection != null)
              _UseRecognized(
                recognized: unitCorrection.recognized,
                accept: () => _unit.text = unitCorrection.recognized ?? '',
              ),
          ],
        ),
      ),
      actions: [
        TextButton(
          onPressed: () => Navigator.pop(context),
          child: const Text('Abbrechen'),
        ),
        FilledButton(onPressed: _save, child: const Text('Speichern')),
      ],
    );
  }
}

// ---------------------------------------------------------------------------
// Building blocks
// ---------------------------------------------------------------------------

/// Text field with a button to clear it.
class _DialogTextField extends StatelessWidget {
  const _DialogTextField({
    required this.controller,
    required this.label,
    this.help,
    this.error,
  });

  final TextEditingController controller;
  final String label;
  final String? help;
  final String? error;

  @override
  Widget build(BuildContext context) {
    return TextField(
      controller: controller,
      autofocus: false,
      decoration: InputDecoration(
        labelText: label,
        helperText: help,
        helperMaxLines: 2,
        errorText: error,
        suffixIcon: IconButton(
          icon: const Icon(Icons.clear),
          tooltip: '$label leeren',
          onPressed: controller.clear,
        ),
      ),
    );
  }
}

/// Button that puts the originally recognized value into the field.
class _UseRecognized extends StatelessWidget {
  const _UseRecognized({required this.recognized, required this.accept});

  final String? recognized;
  final VoidCallback accept;

  @override
  Widget build(BuildContext context) {
    return TextButton.icon(
      onPressed: accept,
      icon: const Icon(Icons.undo, size: 18),
      label: Text('Erkannten Wert übernehmen: ${recognized ?? 'leer'}'),
    );
  }
}
