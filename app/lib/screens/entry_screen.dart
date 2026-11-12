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

/// Shows an entry and allows correcting wrongly recognized values.
///
/// Every correction is its own PATCH. Afterwards the screen always shows
/// the backend's response; the app does not recompute anything itself.
class EntryScreen extends StatefulWidget {
  const EntryScreen({super.key, required this.api, required this.entryId});

  final ApiClient api;
  final String entryId;

  @override
  State<EntryScreen> createState() => _EntryScreenState();
}

class _EntryScreenState extends State<EntryScreen> {
  Entry? _entry;
  ApiException? _error;
  var _saving = false;

  /// Original file (only loaded for images). Created once, so that the
  /// image is not loaded again on every correction.
  Future<FileContent>? _image;

  @override
  void initState() {
    super.initState();
    _load();
  }

  Future<void> _load() async {
    setState(() => _error = null);
    try {
      final entry = await widget.api.getEntry(widget.entryId);
      if (!mounted) return;
      setState(() {
        _entry = entry;
        if (entry.file.isImage) _image = widget.api.getFile(entry.id);
      });
    } on ApiException catch (error) {
      if (mounted) setState(() => _error = error);
    }
  }

  // -------------------------------------------------------------------------
  // Correcting
  // -------------------------------------------------------------------------

  Future<void> _save(EntryPatch patch) async {
    if (patch.isEmpty) return;
    setState(() => _saving = true);
    final message = ScaffoldMessenger.of(context);
    try {
      final entry = await widget.api.correct(widget.entryId, patch);
      if (!mounted) return;
      setState(() => _entry = entry);
      HapticFeedback.lightImpact(); // Make success noticeable
      message.showSnackBar(
        const SnackBar(content: Text('Korrektur gespeichert.')),
      );
    } on ApiException catch (error) {
      // The old state stays on screen; the backend changed nothing.
      message.showSnackBar(SnackBar(content: Text(error.message)));
    } finally {
      if (mounted) setState(() => _saving = false);
    }
  }

  Future<void> _correctType(Entry entry) async {
    final type = await showTypeDialog(
      context,
      // Allowed are the types of the module the entry belongs to.
      moduleId: entry.module,
      current: entry.type,
      correction: entry.correctionFor('type'),
    );
    if (type != null && type != entry.type) {
      await _save(EntryPatch(type: type));
    }
  }

  Future<void> _correctDate(Entry entry) async {
    final date = await showDocumentDateDialog(
      context,
      current: entry.documentDate,
      correction: entry.correctionFor('documentDate'),
    );
    if (date != null && date.value != entry.documentDate) {
      await _save(EntryPatch(documentDate: date));
    }
  }

  Future<void> _correctIssuer(Entry entry) async {
    final issuer = await showIssuerDialog(
      context,
      current: entry.issuer,
      correction: entry.correctionFor('issuer'),
    );
    if (issuer != null && issuer.value != entry.issuer) {
      await _save(EntryPatch(issuer: issuer));
    }
  }

  Future<void> _correctFact(Entry entry, Fact fact) async {
    final patch = await showFactDialog(
      context,
      fact: fact,
      valueCorrection: entry.correctionFor('facts.${fact.id}.value'),
      unitCorrection: entry.correctionFor('facts.${fact.id}.unit'),
    );
    if (patch != null) {
      await _save(EntryPatch(facts: [patch]));
    }
  }

  void _openOriginal(Entry entry, FileContent? preloaded) {
    openPage<void>(
      context,
      OriginalFileScreen(
        api: widget.api,
        entryId: entry.id,
        fileName: entry.file.fileName,
        preloaded: preloaded,
      ),
    );
  }

  // -------------------------------------------------------------------------
  // Deleting
  // -------------------------------------------------------------------------

  /// Asks for confirmation, then deletes the entry, original file and chunks.
  /// On success the view closes; the timeline reloads afterwards.
  Future<void> _delete() async {
    final confirmed = await showDialog<bool>(
      context: context,
      builder: (dialog) {
        final colors = Theme.of(dialog).colorScheme;
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
                backgroundColor: colors.error,
                foregroundColor: colors.onError,
              ),
              onPressed: () => Navigator.pop(dialog, true),
              child: const Text('Löschen'),
            ),
          ],
        );
      },
    );
    if (confirmed != true || !mounted) return;

    final navigator = Navigator.of(context);
    final message = ScaffoldMessenger.of(context);
    setState(() => _saving = true);
    try {
      await widget.api.deleteEntry(widget.entryId);
      navigator.pop();
      HapticFeedback.lightImpact();
      message.showSnackBar(const SnackBar(content: Text('Eintrag gelöscht.')));
    } on ApiException catch (error) {
      // Nothing deleted (or the entry is still visible): the view stays.
      message.showSnackBar(SnackBar(content: Text(error.message)));
      if (mounted) setState(() => _saving = false);
    }
  }

  // -------------------------------------------------------------------------
  // Display
  // -------------------------------------------------------------------------

  @override
  Widget build(BuildContext context) {
    final entry = _entry;
    return Scaffold(
      appBar: AppBar(
        title: Text(
          entry == null
              ? 'Eintrag'
              : ModuleCatalog.of(context).typeName(entry.module, entry.type),
        ),
        actions: [
          MockBanner(visible: widget.api.isMock),
          IconButton(
            icon: const Icon(Icons.delete_outline),
            tooltip: 'Eintrag löschen',
            onPressed: entry == null || _saving ? null : _delete,
          ),
        ],
        bottom: PreferredSize(
          preferredSize: const Size.fromHeight(4),
          child: SizedBox(
            height: 4,
            child: _saving ? const LinearProgressIndicator() : null,
          ),
        ),
      ),
      body: _content(context),
    );
  }

  Widget _content(BuildContext context) {
    final error = _error;
    if (error != null) return ErrorView(error: error, onRetry: _load);
    final entry = _entry;
    if (entry == null) {
      return const Center(child: CircularProgressIndicator());
    }

    // While saving, the pencil buttons are disabled.
    VoidCallback? edit(Future<void> Function() action) =>
        _saving ? null : action;

    // Type names come from the module the entry belongs to.
    final catalog = ModuleCatalog.of(context);
    String typeName(String type) => catalog.typeName(entry.module, type);

    return ListView(
      padding: const EdgeInsets.only(bottom: 24),
      children: [
        _OriginalFileCard(
          file: entry.file,
          image: _image,
          onOpen: (preloaded) => _openOriginal(entry, preloaded),
        ),
        const _Section('Angaben'),
        _FieldRow(
          label: 'Typ',
          value: typeName(entry.type),
          correction: _correctionHint(entry.correctionFor('type'), typeName),
          edit: edit(() => _correctType(entry)),
        ),
        _FieldRow(
          label: 'Dokumentdatum',
          value: entry.documentDate?.display ?? 'nicht erkannt',
          correction: _correctionHint(
            entry.correctionFor('documentDate'),
            (iso) => PlainDate.tryParse(iso)?.display ?? iso,
          ),
          edit: edit(() => _correctDate(entry)),
        ),
        _FieldRow(
          label: 'Quelle',
          value: entry.issuer ?? 'nicht erkannt',
          correction: _correctionHint(entry.correctionFor('issuer')),
          edit: edit(() => _correctIssuer(entry)),
        ),
        _FieldRow(
          label: 'Importiert',
          value: formatLocalTimestamp(entry.importedAt),
          correctable: false,
        ),
        const _Section('Erkannte Angaben'),
        if (entry.facts.isEmpty)
          const ListTile(title: Text('Keine Angaben erkannt.')),
        for (final fact in entry.facts)
          _FieldRow(
            label: fact.name,
            value: [fact.value, ?fact.unit].join(' '),
            correction: [
              ?_correctionHint(
                entry.correctionFor('facts.${fact.id}.value'),
                null,
                'Wert',
              ),
              ?_correctionHint(
                entry.correctionFor('facts.${fact.id}.unit'),
                null,
                'Einheit',
              ),
            ].join('\n').nullIfEmpty,
            edit: edit(() => _correctFact(entry, fact)),
          ),
        const _Section('Volltext'),
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

  /// Text for corrected fields, e.g. „korrigiert, erkannt: 6,3“.
  /// [display] converts the recognized raw value for display.
  String? _correctionHint(
    Correction? correction, [
    String Function(String)? display,
    String? fieldName,
  ]) {
    if (correction == null) return null;
    final recognized = correction.recognized;
    final text = recognized == null
        ? 'nichts'
        : (display == null ? recognized : display(recognized));
    final prefix = fieldName == null ? '' : '$fieldName ';
    return '${prefix}korrigiert, erkannt: $text';
  }
}

extension on String {
  String? get nullIfEmpty => isEmpty ? null : this;
}

/// Header of a section.
class _Section extends StatelessWidget {
  const _Section(this.title);

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

/// A row with label, value, correction hint if any, and pencil button.
class _FieldRow extends StatelessWidget {
  const _FieldRow({
    required this.label,
    required this.value,
    this.correction,
    this.edit,
    this.correctable = true,
  });

  final String label;
  final String value;

  /// Hint like „korrigiert, erkannt: 6,3“, otherwise `null`.
  final String? correction;

  /// `null` disables the pencil button (e.g. while saving).
  final VoidCallback? edit;

  /// `false`: no pencil button at all (e.g. for the import date).
  final bool correctable;

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
      trailing: correctable
          ? IconButton(
              icon: const Icon(Icons.edit_outlined),
              tooltip: '$label korrigieren',
              onPressed: edit,
            )
          : null,
    );
  }
}

/// Original file: images as a preview, for PDFs name and size. Below it the
/// button to open it; a tap on the preview image opens it as well.
class _OriginalFileCard extends StatelessWidget {
  const _OriginalFileCard({
    required this.file,
    required this.image,
    required this.onOpen,
  });

  final File file;
  final Future<FileContent>? image;

  /// Opens the file; `preloaded` is the image already loaded, if any.
  final void Function(FileContent? preloaded) onOpen;

  @override
  Widget build(BuildContext context) {
    final image = this.image;
    final (icon, kind) = file.isPdf
        ? (Icons.picture_as_pdf_outlined, 'PDF')
        : file.isText
        ? (Icons.article_outlined, 'Text')
        : (Icons.image_outlined, 'Bild');
    return Card(
      margin: const EdgeInsets.all(16),
      clipBehavior: Clip.antiAlias,
      child: Column(
        children: [
          if (image != null)
            FutureBuilder<FileContent>(
              future: image,
              builder: (context, snapshot) {
                if (snapshot.hasError) {
                  return const _ImageFallback(
                    child: Text('Originaldatei konnte nicht geladen werden.'),
                  );
                }
                final content = snapshot.data;
                if (content == null) {
                  return const _ImageFallback(
                    child: CircularProgressIndicator(),
                  );
                }
                return InkWell(
                  onTap: () => onOpen(content),
                  child: Image.memory(
                    content.bytes,
                    // Full width: the whole area can be tapped, even
                    // before the image is decoded.
                    width: double.infinity,
                    height: 220,
                    fit: BoxFit.contain,
                    errorBuilder: (_, _, _) => const _ImageFallback(
                      child: Icon(Icons.broken_image_outlined, size: 48),
                    ),
                  ),
                );
              },
            ),
          ListTile(
            leading: Icon(icon),
            title: Text(file.fileName),
            subtitle: Text('${formatSize(file.size)} · $kind'),
          ),
          Padding(
            padding: const EdgeInsets.fromLTRB(16, 0, 16, 12),
            child: SizedBox(
              width: double.infinity,
              child: FilledButton.tonalIcon(
                onPressed: () => onOpen(null),
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

class _ImageFallback extends StatelessWidget {
  const _ImageFallback({required this.child});

  final Widget child;

  @override
  Widget build(BuildContext context) =>
      SizedBox(height: 220, child: Center(child: child));
}
