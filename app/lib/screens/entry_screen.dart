import 'package:flutter/material.dart';
import 'package:flutter/services.dart';

import '../api/api_client.dart';
import '../api/api_exception.dart';
import '../model/plain_date.dart';
import '../model/entry.dart';
import '../model/entry_patch.dart';
import '../module/module_catalog.dart';
import '../widgets/error_view.dart';
import '../widgets/format.dart';
import '../widgets/mock_banner.dart';
import '../widgets/navigation.dart';
import 'correction_dialogs.dart';
import 'original_file_screen.dart';

/// Zeigt einen Eintrag und erlaubt, falsch erkannte Werte zu korrigieren.
///
/// Jede Korrektur ist ein eigener PATCH. Angezeigt wird danach immer die
/// Antwort des Backends; die App rechnet nichts selbst nach.
class EintragScreen extends StatefulWidget {
  const EintragScreen({super.key, required this.api, required this.entryId});

  final ApiClient api;
  final String entryId;

  @override
  State<EintragScreen> createState() => _EintragScreenState();
}

class _EintragScreenState extends State<EintragScreen> {
  Entry? _eintrag;
  ApiException? _fehler;
  var _speichert = false;

  /// Originaldatei (nur bei Bildern geladen). Wird einmal angelegt, damit
  /// das Bild nicht bei jeder Korrektur neu geladen wird.
  Future<FileContent>? _bild;

  @override
  void initState() {
    super.initState();
    _laden();
  }

  Future<void> _laden() async {
    setState(() => _fehler = null);
    try {
      final entry = await widget.api.getEntry(widget.entryId);
      if (!mounted) return;
      setState(() {
        _eintrag = entry;
        if (entry.file.isImage) _bild = widget.api.getFile(entry.id);
      });
    } on ApiException catch (fehler) {
      if (mounted) setState(() => _fehler = fehler);
    }
  }

  // -------------------------------------------------------------------------
  // Korrigieren
  // -------------------------------------------------------------------------

  Future<void> _speichern(EntryPatch aenderung) async {
    if (aenderung.isEmpty) return;
    setState(() => _speichert = true);
    final message = ScaffoldMessenger.of(context);
    try {
      final entry = await widget.api.correct(widget.entryId, aenderung);
      if (!mounted) return;
      setState(() => _eintrag = entry);
      HapticFeedback.lightImpact(); // Erfolg spürbar machen
      message.showSnackBar(
        const SnackBar(content: Text('Korrektur gespeichert.')),
      );
    } on ApiException catch (fehler) {
      // Der alte Stand bleibt angezeigt; das Backend hat nichts geändert.
      message.showSnackBar(SnackBar(content: Text(fehler.message)));
    } finally {
      if (mounted) setState(() => _speichert = false);
    }
  }

  Future<void> _typKorrigieren(Entry entry) async {
    final type = await zeigeTypDialog(
      context,
      // Erlaubt sind die Typen des Moduls, zu dem der Eintrag gehört.
      moduleId: entry.module,
      aktuell: entry.type,
      correction: entry.correctionFor('type'),
    );
    if (type != null && type != entry.type) {
      await _speichern(EntryPatch(type: type));
    }
  }

  Future<void> _datumKorrigieren(Entry entry) async {
    final datum = await zeigeDatumAuswahl(
      context,
      aktuell: entry.documentDate,
      correction: entry.correctionFor('documentDate'),
    );
    if (datum != null && datum.value != entry.documentDate) {
      await _speichern(EntryPatch(documentDate: datum));
    }
  }

  Future<void> _quelleKorrigieren(Entry entry) async {
    final issuer = await zeigeQuelleDialog(
      context,
      aktuell: entry.issuer,
      correction: entry.correctionFor('issuer'),
    );
    if (issuer != null && issuer.value != entry.issuer) {
      await _speichern(EntryPatch(issuer: issuer));
    }
  }

  Future<void> _infoKorrigieren(Entry entry, Fact fact) async {
    final aenderung = await zeigeInfoDialog(
      context,
      fact: fact,
      wertKorrektur: entry.correctionFor('facts.${fact.id}.value'),
      einheitKorrektur: entry.correctionFor('facts.${fact.id}.unit'),
    );
    if (aenderung != null) {
      await _speichern(EntryPatch(facts: [aenderung]));
    }
  }

  void _originalOeffnen(Entry entry, FileContent? vorhanden) {
    oeffneSeite<void>(
      context,
      OriginaldateiScreen(
        api: widget.api,
        entryId: entry.id,
        fileName: entry.file.fileName,
        vorhanden: vorhanden,
      ),
    );
  }

  // -------------------------------------------------------------------------
  // Löschen
  // -------------------------------------------------------------------------

  /// Fragt nach und löscht dann Eintrag, Originaldatei und Suchabschnitte.
  /// Bei Erfolg schließt sich die Ansicht; die Zeitleiste lädt danach neu.
  Future<void> _loeschen() async {
    final bestaetigt = await showDialog<bool>(
      context: context,
      builder: (dialog) {
        final farben = Theme.of(dialog).colorScheme;
        return AlertDialog(
          title: const Text('Eintrag löschen?'),
          content: const Text(
            'Eintrag, Originaldatei und Suchabschnitte werden endgültig '
            'gelöscht.',
          ),
          actions: [
            TextButton(
              onPressed: () => Navigator.pop(dialog, false),
              child: const Text('Abbrechen'),
            ),
            FilledButton(
              style: FilledButton.styleFrom(
                backgroundColor: farben.error,
                foregroundColor: farben.onError,
              ),
              onPressed: () => Navigator.pop(dialog, true),
              child: const Text('Löschen'),
            ),
          ],
        );
      },
    );
    if (bestaetigt != true || !mounted) return;

    final navigator = Navigator.of(context);
    final message = ScaffoldMessenger.of(context);
    setState(() => _speichert = true);
    try {
      await widget.api.deleteEntry(widget.entryId);
      navigator.pop();
      HapticFeedback.lightImpact();
      message.showSnackBar(const SnackBar(content: Text('Eintrag gelöscht.')));
    } on ApiException catch (fehler) {
      // Nichts gelöscht (bzw. der Eintrag ist noch sichtbar): Ansicht bleibt.
      message.showSnackBar(SnackBar(content: Text(fehler.message)));
      if (mounted) setState(() => _speichert = false);
    }
  }

  // -------------------------------------------------------------------------
  // Anzeige
  // -------------------------------------------------------------------------

  @override
  Widget build(BuildContext context) {
    final entry = _eintrag;
    return Scaffold(
      appBar: AppBar(
        title: Text(
          entry == null
              ? 'Eintrag'
              : ModuleCatalog.of(context).typeName(entry.module, entry.type),
        ),
        actions: [
          MockHinweis(sichtbar: widget.api.isMock),
          IconButton(
            icon: const Icon(Icons.delete_outline),
            tooltip: 'Eintrag löschen',
            onPressed: entry == null || _speichert ? null : _loeschen,
          ),
        ],
        bottom: PreferredSize(
          preferredSize: const Size.fromHeight(4),
          child: SizedBox(
            height: 4,
            child: _speichert ? const LinearProgressIndicator() : null,
          ),
        ),
      ),
      body: _inhalt(context),
    );
  }

  Widget _inhalt(BuildContext context) {
    final fehler = _fehler;
    if (fehler != null) return FehlerAnzeige(fehler: fehler, erneut: _laden);
    final entry = _eintrag;
    if (entry == null) {
      return const Center(child: CircularProgressIndicator());
    }

    // Während gespeichert wird, sind die Stifte gesperrt.
    VoidCallback? bearbeiten(Future<void> Function() aktion) =>
        _speichert ? null : aktion;

    // Typnamen kommen aus dem Modul, zu dem der Eintrag gehört.
    final catalog = ModuleCatalog.of(context);
    String typeName(String type) => catalog.typeName(entry.module, type);

    return ListView(
      padding: const EdgeInsets.only(bottom: 24),
      children: [
        _Originaldatei(
          file: entry.file,
          bild: _bild,
          oeffnen: (vorhanden) => _originalOeffnen(entry, vorhanden),
        ),
        const _Abschnitt('Angaben'),
        _FeldZeile(
          label: 'Typ',
          value: typeName(entry.type),
          correction: _hinweis(entry.correctionFor('type'), typeName),
          bearbeiten: bearbeiten(() => _typKorrigieren(entry)),
        ),
        _FeldZeile(
          label: 'Dokumentdatum',
          value: entry.documentDate?.display ?? 'nicht erkannt',
          correction: _hinweis(
            entry.correctionFor('documentDate'),
            (iso) => PlainDate.tryParse(iso)?.display ?? iso,
          ),
          bearbeiten: bearbeiten(() => _datumKorrigieren(entry)),
        ),
        _FeldZeile(
          label: 'Quelle',
          value: entry.issuer ?? 'nicht erkannt',
          correction: _hinweis(entry.correctionFor('issuer')),
          bearbeiten: bearbeiten(() => _quelleKorrigieren(entry)),
        ),
        _FeldZeile(
          label: 'Importiert',
          value: formatZeitpunktLokal(entry.importedAt),
          korrigierbar: false,
        ),
        const _Abschnitt('Erkannte Angaben'),
        if (entry.facts.isEmpty)
          const ListTile(title: Text('Keine Angaben erkannt.')),
        for (final fact in entry.facts)
          _FeldZeile(
            label: fact.name,
            value: [fact.value, ?fact.unit].join(' '),
            correction: [
              ?_hinweis(
                entry.correctionFor('facts.${fact.id}.value'),
                null,
                'Wert',
              ),
              ?_hinweis(
                entry.correctionFor('facts.${fact.id}.unit'),
                null,
                'Einheit',
              ),
            ].join('\n').nullWennLeer,
            bearbeiten: bearbeiten(() => _infoKorrigieren(entry, fact)),
          ),
        const _Abschnitt('Volltext'),
        Padding(
          padding: const EdgeInsets.symmetric(horizontal: 16),
          child: SelectableText(entry.fullText),
        ),
        Padding(
          padding: const EdgeInsets.fromLTRB(16, 8, 16, 0),
          child: Text(
            'Der Volltext kann im Prototyp nicht korrigiert werden.',
            style: Theme.of(context).textTheme.bodySmall,
          ),
        ),
      ],
    );
  }

  /// Text für korrigierte Felder, z. B. „korrigiert, erkannt: 6,3“.
  /// [anzeige] wandelt den erkannten Rohwert für die Anzeige um.
  String? _hinweis(
    Correction? correction, [
    String Function(String)? display,
    String? feldname,
  ]) {
    if (correction == null) return null;
    final recognized = correction.recognized;
    final text = recognized == null
        ? 'nichts'
        : (display == null ? recognized : display(recognized));
    final praefix = feldname == null ? '' : '$feldname ';
    return '${praefix}korrigiert, erkannt: $text';
  }
}

extension on String {
  String? get nullWennLeer => isEmpty ? null : this;
}

/// Überschrift eines Abschnitts.
class _Abschnitt extends StatelessWidget {
  const _Abschnitt(this.title);

  final String title;

  @override
  Widget build(BuildContext context) {
    return Padding(
      padding: const EdgeInsets.fromLTRB(16, 20, 16, 4),
      child: Text(
        title,
        style: Theme.of(context).textTheme.titleSmall
            ?.copyWith(color: Theme.of(context).colorScheme.primary),
      ),
    );
  }
}

/// Eine Zeile mit Bezeichnung, Wert, ggf. Korrektur-Hinweis und Stift.
class _FeldZeile extends StatelessWidget {
  const _FeldZeile({
    required this.label,
    required this.value,
    this.correction,
    this.bearbeiten,
    this.korrigierbar = true,
  });

  final String label;
  final String value;

  /// Hinweis wie „korrigiert, erkannt: 6,3“, sonst `null`.
  final String? correction;

  /// `null` sperrt den Stift (z. B. während gespeichert wird).
  final VoidCallback? bearbeiten;

  /// `false`: gar kein Stift (z. B. beim Importdatum).
  final bool korrigierbar;

  @override
  Widget build(BuildContext context) {
    final theme = Theme.of(context);
    final correction = this.correction;
    return ListTile(
      title: Text(
        label,
        style: theme.textTheme.labelMedium?.copyWith(
          color: theme.colorScheme.onSurfaceVariant,
        ),
      ),
      subtitle: Column(
        crossAxisAlignment: CrossAxisAlignment.start,
        children: [
          Text(value, style: theme.textTheme.bodyLarge),
          if (correction != null)
            Padding(
              padding: const EdgeInsets.only(top: 2),
              child: Row(
                crossAxisAlignment: CrossAxisAlignment.start,
                children: [
                  Icon(
                    Icons.edit_note,
                    size: 16,
                    color: theme.colorScheme.tertiary,
                  ),
                  const SizedBox(width: 4),
                  Expanded(
                    child: Text(
                      correction,
                      style: theme.textTheme.bodySmall?.copyWith(
                        color: theme.colorScheme.tertiary,
                      ),
                    ),
                  ),
                ],
              ),
            ),
        ],
      ),
      trailing: korrigierbar
          ? IconButton(
              icon: const Icon(Icons.edit_outlined),
              tooltip: '$label korrigieren',
              onPressed: bearbeiten,
            )
          : null,
    );
  }
}

/// Originaldatei: Bilder als Vorschau, bei PDFs Name und Größe. Darunter
/// der Knopf zum Öffnen; ein Tipp auf das Vorschaubild öffnet ebenfalls.
class _Originaldatei extends StatelessWidget {
  const _Originaldatei({
    required this.file,
    required this.bild,
    required this.oeffnen,
  });

  final File file;
  final Future<FileContent>? bild;

  /// Öffnet die Datei; [vorhanden] ist das schon geladene Bild, falls da.
  final void Function(FileContent? vorhanden) oeffnen;

  @override
  Widget build(BuildContext context) {
    final bild = this.bild;
    final (icon, art) = file.isPdf
        ? (Icons.picture_as_pdf_outlined, 'PDF')
        : file.isText
        ? (Icons.article_outlined, 'Text')
        : (Icons.image_outlined, 'Bild');
    return Card(
      margin: const EdgeInsets.all(16),
      clipBehavior: Clip.antiAlias,
      child: Column(
        children: [
          if (bild != null)
            FutureBuilder<FileContent>(
              future: bild,
              builder: (context, snapshot) {
                if (snapshot.hasError) {
                  return const _BildPlatzhalter(
                    kind: Text('Originaldatei konnte nicht geladen werden.'),
                  );
                }
                final inhalt = snapshot.data;
                if (inhalt == null) {
                  return const _BildPlatzhalter(
                    kind: CircularProgressIndicator(),
                  );
                }
                return InkWell(
                  onTap: () => oeffnen(inhalt),
                  child: Image.memory(
                    inhalt.bytes,
                    // Volle Breite: Die ganze Fläche ist antippbar, auch
                    // bevor das Bild dekodiert ist.
                    width: double.infinity,
                    height: 220,
                    fit: BoxFit.contain,
                    errorBuilder: (_, _, _) => const _BildPlatzhalter(
                      kind: Icon(Icons.broken_image_outlined, size: 48),
                    ),
                  ),
                );
              },
            ),
          ListTile(
            leading: Icon(icon),
            title: Text(file.fileName),
            subtitle: Text('${formatGroesse(file.size)} · $art'),
          ),
          Padding(
            padding: const EdgeInsets.fromLTRB(16, 0, 16, 12),
            child: SizedBox(
              width: double.infinity,
              child: FilledButton.tonalIcon(
                onPressed: () => oeffnen(null),
                icon: const Icon(Icons.open_in_full),
                label: const Text('Originaldatei öffnen'),
              ),
            ),
          ),
        ],
      ),
    );
  }
}

class _BildPlatzhalter extends StatelessWidget {
  const _BildPlatzhalter({required this.kind});

  final Widget kind;

  @override
  Widget build(BuildContext context) =>
      SizedBox(height: 220, child: Center(child: kind));
}
