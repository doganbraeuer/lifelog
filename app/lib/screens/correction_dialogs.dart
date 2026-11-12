// Dialoge zum Korrigieren einzelner Felder eines Eintrags.
//
// Jeder Dialog gibt `null` zurück, wenn der Nutzer abbricht. Sonst gibt er
// den neuen Wert zurück; ob sich wirklich etwas geändert hat, prüft der
// Eintrag-Screen vor dem Speichern.

import 'package:flutter/material.dart';

import '../model/plain_date.dart';
import '../model/entry.dart';
import '../model/entry_patch.dart';
import '../module/module_catalog.dart';

// ---------------------------------------------------------------------------
// Typ
// ---------------------------------------------------------------------------

/// Typ über ein Dropdown wählen. Zur Auswahl stehen die Typen des Moduls
/// [modulId] aus dem Modul-Katalog.
Future<String?> zeigeTypDialog(
  BuildContext context, {
  required String moduleId,
  required String aktuell,
  required Correction? correction,
}) => showDialog<String>(
  context: context,
  builder: (_) => _TypDialog(
    moduleId: moduleId,
    aktuell: aktuell,
    recognized: correction?.recognized,
  ),
);

class _TypDialog extends StatefulWidget {
  const _TypDialog({
    required this.moduleId,
    required this.aktuell,
    required this.recognized,
  });

  final String moduleId;
  final String aktuell;

  /// Ursprünglich erkannter Typ, nur gesetzt, wenn der Typ korrigiert wurde.
  final String? recognized;

  @override
  State<_TypDialog> createState() => _TypDialogState();
}

class _TypDialogState extends State<_TypDialog> {
  late String _auswahl = widget.aktuell;

  @override
  Widget build(BuildContext context) {
    final catalog = ModuleCatalog.of(context);
    // Der aktuelle Typ bleibt wählbar, auch wenn das Modul ihn nicht (mehr)
    // kennt, z. B. bei Daten von einem neueren Backend.
    final types = {
      for (final t in catalog.types(widget.moduleId)) t.id,
      widget.aktuell,
    };
    String name(String type) => catalog.typeName(widget.moduleId, type);
    return AlertDialog(
      title: const Text('Typ korrigieren'),
      content: DropdownButtonFormField<String>(
        initialValue: _auswahl,
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
        onChanged: (type) => setState(() => _auswahl = type ?? _auswahl),
      ),
      actions: [
        TextButton(
          onPressed: () => Navigator.pop(context),
          child: const Text('Abbrechen'),
        ),
        FilledButton(
          onPressed: () => Navigator.pop(context, _auswahl),
          child: const Text('Speichern'),
        ),
      ],
    );
  }
}

// ---------------------------------------------------------------------------
// Dokumentdatum
// ---------------------------------------------------------------------------

/// Auswahl: Datum wählen, Datum entfernen oder erkanntes Datum
/// wiederherstellen. `Wert(null)` heißt: kein Datum.
Future<Value<PlainDate?>?> zeigeDatumAuswahl(
  BuildContext context, {
  required PlainDate? aktuell,
  required Correction? correction,
}) async {
  final recognized = correction == null
      ? null
      : Value(
          correction.recognized == null
              ? null
              : PlainDate.tryParse(correction.recognized!),
        );

  final aktion = await showModalBottomSheet<_DatumAktion>(
    context: context,
    showDragHandle: true,
    builder: (context) => SafeArea(
      child: Column(
        mainAxisSize: MainAxisSize.min,
        children: [
          ListTile(
            leading: const Icon(Icons.calendar_month_outlined),
            title: const Text('Datum wählen'),
            onTap: () => Navigator.pop(context, _DatumAktion.waehlen),
          ),
          if (aktuell != null)
            ListTile(
              leading: const Icon(Icons.event_busy_outlined),
              title: const Text('Datum entfernen'),
              onTap: () => Navigator.pop(context, _DatumAktion.entfernen),
            ),
          if (recognized != null)
            ListTile(
              leading: const Icon(Icons.undo),
              title: Text(
                'Erkanntes Datum wiederherstellen: '
                '${recognized.value?.display ?? 'kein Datum'}',
              ),
              onTap: () =>
                  Navigator.pop(context, _DatumAktion.wiederherstellen),
            ),
        ],
      ),
    ),
  );
  if (aktion == null || !context.mounted) return null;

  switch (aktion) {
    case _DatumAktion.entfernen:
      return const Value(null);
    case _DatumAktion.wiederherstellen:
      return recognized;
    case _DatumAktion.waehlen:
      final gewaehlt = await showDatePicker(
        context: context,
        initialDate: startdatumFuerAuswahl(aktuell, DateTime.now()),
        firstDate: fruehestesDatum,
        lastDate: spaetestesDatum,
        helpText: 'Dokumentdatum wählen',
      );
      // Nur Jahr, Monat und Tag übernehmen, keine Zeitzone (siehe Datum).
      return gewaehlt == null ? null : Value(PlainDate.fromDateTime(gewaehlt));
  }
}

enum _DatumAktion { waehlen, entfernen, wiederherstellen }

/// Bereich der Datumsauswahl.
final fruehestesDatum = DateTime(1900);
final spaetestesDatum = DateTime(2100, 12, 31);

/// Startdatum der Datumsauswahl: das aktuelle Dokumentdatum, sonst heute.
///
/// `showDatePicker` bricht mit einer Assertion ab, wenn `initialDate`
/// außerhalb von `firstDate` bis `lastDate` liegt. Genau ein falsch
/// erkanntes Jahr (z. B. 1026 statt 2026) will man aber korrigieren.
/// Liegt das Datum außerhalb, startet die Auswahl deshalb bei [heute].
DateTime startdatumFuerAuswahl(PlainDate? aktuell, DateTime heute) {
  final datum = aktuell?.toDateTime();
  if (datum == null ||
      datum.isBefore(fruehestesDatum) ||
      datum.isAfter(spaetestesDatum)) {
    return heute;
  }
  return datum;
}

// ---------------------------------------------------------------------------
// Quelle
// ---------------------------------------------------------------------------

/// Quelle bearbeiten. Ein leeres Feld entfernt die Quelle (`Wert(null)`).
Future<Value<String?>?> zeigeQuelleDialog(
  BuildContext context, {
  required String? aktuell,
  required Correction? correction,
}) => showDialog<Value<String?>>(
  context: context,
  builder: (_) => _QuelleDialog(aktuell: aktuell, correction: correction),
);

class _QuelleDialog extends StatefulWidget {
  const _QuelleDialog({required this.aktuell, required this.correction});

  final String? aktuell;
  final Correction? correction;

  @override
  State<_QuelleDialog> createState() => _QuelleDialogState();
}

class _QuelleDialogState extends State<_QuelleDialog> {
  late final _feld = TextEditingController(text: widget.aktuell ?? '');

  @override
  void dispose() {
    _feld.dispose();
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
          _TextFeld(
            controller: _feld,
            label: 'Quelle',
            hilfe: 'Leer lassen, um die Quelle zu entfernen.',
          ),
          if (correction != null)
            _ErkanntUebernehmen(
              recognized: correction.recognized,
              uebernehmen: () => _feld.text = correction.recognized ?? '',
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
            final text = _feld.text.trim();
            Navigator.pop(context, Value(text.isEmpty ? null : text));
          },
          child: const Text('Speichern'),
        ),
      ],
    );
  }
}

// ---------------------------------------------------------------------------
// Info (Wert und Einheit)
// ---------------------------------------------------------------------------

/// Wert und Einheit einer Info bearbeiten. Gibt nur die Felder zurück, die
/// sich geändert haben, oder `null`, wenn sich nichts geändert hat.
Future<FactPatch?> zeigeInfoDialog(
  BuildContext context, {
  required Fact fact,
  required Correction? wertKorrektur,
  required Correction? einheitKorrektur,
}) => showDialog<FactPatch>(
  context: context,
  builder: (_) => _InfoDialog(
    fact: fact,
    wertKorrektur: wertKorrektur,
    einheitKorrektur: einheitKorrektur,
  ),
);

class _InfoDialog extends StatefulWidget {
  const _InfoDialog({
    required this.fact,
    required this.wertKorrektur,
    required this.einheitKorrektur,
  });

  final Fact fact;
  final Correction? wertKorrektur;
  final Correction? einheitKorrektur;

  @override
  State<_InfoDialog> createState() => _InfoDialogState();
}

class _InfoDialogState extends State<_InfoDialog> {
  late final _wert = TextEditingController(text: widget.fact.value);
  late final _einheit = TextEditingController(text: widget.fact.unit ?? '');
  String? _wertFehler;

  @override
  void dispose() {
    _wert.dispose();
    _einheit.dispose();
    super.dispose();
  }

  void _speichern() {
    final value = _wert.text.trim();
    if (value.isEmpty) {
      setState(() => _wertFehler = 'Der Wert darf nicht leer sein.');
      return;
    }
    final einheitText = _einheit.text.trim();
    final unit = einheitText.isEmpty ? null : einheitText;

    final wertGeaendert = value != widget.fact.value;
    final einheitGeaendert = unit != widget.fact.unit;
    Navigator.pop(
      context,
      wertGeaendert || einheitGeaendert
          ? FactPatch(
              widget.fact.id,
              value: wertGeaendert ? value : null,
              unit: einheitGeaendert ? Value(unit) : null,
            )
          : null,
    );
  }

  @override
  Widget build(BuildContext context) {
    final wertKorrektur = widget.wertKorrektur;
    final einheitKorrektur = widget.einheitKorrektur;
    return AlertDialog(
      title: Text('${widget.fact.name} korrigieren'),
      content: SingleChildScrollView(
        child: Column(
          mainAxisSize: MainAxisSize.min,
          crossAxisAlignment: CrossAxisAlignment.start,
          children: [
            _TextFeld(
              controller: _wert,
              label: 'Wert',
              fehler: _wertFehler,
              hilfe: 'Genau wie auf dem Dokument, z. B. „<0,5“.',
            ),
            if (wertKorrektur != null)
              _ErkanntUebernehmen(
                recognized: wertKorrektur.recognized,
                uebernehmen: () => _wert.text = wertKorrektur.recognized ?? '',
              ),
            const SizedBox(height: 16),
            _TextFeld(
              controller: _einheit,
              label: 'Einheit',
              hilfe: 'Leer lassen, wenn es keine Einheit gibt.',
            ),
            if (einheitKorrektur != null)
              _ErkanntUebernehmen(
                recognized: einheitKorrektur.recognized,
                uebernehmen: () =>
                    _einheit.text = einheitKorrektur.recognized ?? '',
              ),
          ],
        ),
      ),
      actions: [
        TextButton(
          onPressed: () => Navigator.pop(context),
          child: const Text('Abbrechen'),
        ),
        FilledButton(onPressed: _speichern, child: const Text('Speichern')),
      ],
    );
  }
}

// ---------------------------------------------------------------------------
// Bausteine
// ---------------------------------------------------------------------------

/// Textfeld mit Knopf zum Leeren.
class _TextFeld extends StatelessWidget {
  const _TextFeld({
    required this.controller,
    required this.label,
    this.hilfe,
    this.fehler,
  });

  final TextEditingController controller;
  final String label;
  final String? hilfe;
  final String? fehler;

  @override
  Widget build(BuildContext context) {
    return TextField(
      controller: controller,
      autofocus: false,
      decoration: InputDecoration(
        labelText: label,
        helperText: hilfe,
        helperMaxLines: 2,
        errorText: fehler,
        suffixIcon: IconButton(
          icon: const Icon(Icons.clear),
          tooltip: '$label leeren',
          onPressed: controller.clear,
        ),
      ),
    );
  }
}

/// Knopf, der den ursprünglich erkannten Wert ins Feld übernimmt.
class _ErkanntUebernehmen extends StatelessWidget {
  const _ErkanntUebernehmen({
    required this.recognized,
    required this.uebernehmen,
  });

  final String? recognized;
  final VoidCallback uebernehmen;

  @override
  Widget build(BuildContext context) {
    return TextButton.icon(
      onPressed: uebernehmen,
      icon: const Icon(Icons.undo, size: 18),
      label: Text('Erkannten Wert übernehmen: ${recognized ?? 'leer'}'),
    );
  }
}
