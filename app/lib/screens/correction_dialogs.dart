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
  required String modulId,
  required String aktuell,
  required Korrektur? korrektur,
}) => showDialog<String>(
  context: context,
  builder: (_) => _TypDialog(
    modulId: modulId,
    aktuell: aktuell,
    erkannt: korrektur?.erkannt,
  ),
);

class _TypDialog extends StatefulWidget {
  const _TypDialog({
    required this.modulId,
    required this.aktuell,
    required this.erkannt,
  });

  final String modulId;
  final String aktuell;

  /// Ursprünglich erkannter Typ, nur gesetzt, wenn der Typ korrigiert wurde.
  final String? erkannt;

  @override
  State<_TypDialog> createState() => _TypDialogState();
}

class _TypDialogState extends State<_TypDialog> {
  late String _auswahl = widget.aktuell;

  @override
  Widget build(BuildContext context) {
    final katalog = ModulKatalog.von(context);
    // Der aktuelle Typ bleibt wählbar, auch wenn das Modul ihn nicht (mehr)
    // kennt, z. B. bei Daten von einem neueren Backend.
    final typen = {
      for (final t in katalog.typen(widget.modulId)) t.id,
      widget.aktuell,
    };
    String name(String typ) => katalog.typName(widget.modulId, typ);
    return AlertDialog(
      title: const Text('Typ korrigieren'),
      content: DropdownButtonFormField<String>(
        initialValue: _auswahl,
        decoration: const InputDecoration(labelText: 'Typ'),
        items: [
          for (final typ in typen)
            DropdownMenuItem(
              value: typ,
              child: Text(
                typ == widget.erkannt ? '${name(typ)} (erkannt)' : name(typ),
              ),
            ),
        ],
        onChanged: (typ) => setState(() => _auswahl = typ ?? _auswahl),
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
Future<Wert<Datum?>?> zeigeDatumAuswahl(
  BuildContext context, {
  required Datum? aktuell,
  required Korrektur? korrektur,
}) async {
  final erkannt = korrektur == null
      ? null
      : Wert(
          korrektur.erkannt == null ? null : Datum.tryParse(korrektur.erkannt!),
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
          if (erkannt != null)
            ListTile(
              leading: const Icon(Icons.undo),
              title: Text(
                'Erkanntes Datum wiederherstellen: '
                '${erkannt.wert?.anzeige ?? 'kein Datum'}',
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
      return const Wert(null);
    case _DatumAktion.wiederherstellen:
      return erkannt;
    case _DatumAktion.waehlen:
      final gewaehlt = await showDatePicker(
        context: context,
        initialDate: startdatumFuerAuswahl(aktuell, DateTime.now()),
        firstDate: fruehestesDatum,
        lastDate: spaetestesDatum,
        helpText: 'Dokumentdatum wählen',
      );
      // Nur Jahr, Monat und Tag übernehmen, keine Zeitzone (siehe Datum).
      return gewaehlt == null ? null : Wert(Datum.aus(gewaehlt));
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
DateTime startdatumFuerAuswahl(Datum? aktuell, DateTime heute) {
  final datum = aktuell?.alsDateTime();
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
Future<Wert<String?>?> zeigeQuelleDialog(
  BuildContext context, {
  required String? aktuell,
  required Korrektur? korrektur,
}) => showDialog<Wert<String?>>(
  context: context,
  builder: (_) => _QuelleDialog(aktuell: aktuell, korrektur: korrektur),
);

class _QuelleDialog extends StatefulWidget {
  const _QuelleDialog({required this.aktuell, required this.korrektur});

  final String? aktuell;
  final Korrektur? korrektur;

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
    final korrektur = widget.korrektur;
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
          if (korrektur != null)
            _ErkanntUebernehmen(
              erkannt: korrektur.erkannt,
              uebernehmen: () => _feld.text = korrektur.erkannt ?? '',
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
            Navigator.pop(context, Wert(text.isEmpty ? null : text));
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
Future<InfoAenderung?> zeigeInfoDialog(
  BuildContext context, {
  required Info info,
  required Korrektur? wertKorrektur,
  required Korrektur? einheitKorrektur,
}) => showDialog<InfoAenderung>(
  context: context,
  builder: (_) => _InfoDialog(
    info: info,
    wertKorrektur: wertKorrektur,
    einheitKorrektur: einheitKorrektur,
  ),
);

class _InfoDialog extends StatefulWidget {
  const _InfoDialog({
    required this.info,
    required this.wertKorrektur,
    required this.einheitKorrektur,
  });

  final Info info;
  final Korrektur? wertKorrektur;
  final Korrektur? einheitKorrektur;

  @override
  State<_InfoDialog> createState() => _InfoDialogState();
}

class _InfoDialogState extends State<_InfoDialog> {
  late final _wert = TextEditingController(text: widget.info.wert);
  late final _einheit = TextEditingController(text: widget.info.einheit ?? '');
  String? _wertFehler;

  @override
  void dispose() {
    _wert.dispose();
    _einheit.dispose();
    super.dispose();
  }

  void _speichern() {
    final wert = _wert.text.trim();
    if (wert.isEmpty) {
      setState(() => _wertFehler = 'Der Wert darf nicht leer sein.');
      return;
    }
    final einheitText = _einheit.text.trim();
    final einheit = einheitText.isEmpty ? null : einheitText;

    final wertGeaendert = wert != widget.info.wert;
    final einheitGeaendert = einheit != widget.info.einheit;
    Navigator.pop(
      context,
      wertGeaendert || einheitGeaendert
          ? InfoAenderung(
              widget.info.id,
              wert: wertGeaendert ? wert : null,
              einheit: einheitGeaendert ? Wert(einheit) : null,
            )
          : null,
    );
  }

  @override
  Widget build(BuildContext context) {
    final wertKorrektur = widget.wertKorrektur;
    final einheitKorrektur = widget.einheitKorrektur;
    return AlertDialog(
      title: Text('${widget.info.name} korrigieren'),
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
                erkannt: wertKorrektur.erkannt,
                uebernehmen: () => _wert.text = wertKorrektur.erkannt ?? '',
              ),
            const SizedBox(height: 16),
            _TextFeld(
              controller: _einheit,
              label: 'Einheit',
              hilfe: 'Leer lassen, wenn es keine Einheit gibt.',
            ),
            if (einheitKorrektur != null)
              _ErkanntUebernehmen(
                erkannt: einheitKorrektur.erkannt,
                uebernehmen: () =>
                    _einheit.text = einheitKorrektur.erkannt ?? '',
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
  const _ErkanntUebernehmen({required this.erkannt, required this.uebernehmen});

  final String? erkannt;
  final VoidCallback uebernehmen;

  @override
  Widget build(BuildContext context) {
    return TextButton.icon(
      onPressed: uebernehmen,
      icon: const Icon(Icons.undo, size: 18),
      label: Text('Erkannten Wert übernehmen: ${erkannt ?? 'leer'}'),
    );
  }
}
