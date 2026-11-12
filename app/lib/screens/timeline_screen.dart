import 'package:flutter/material.dart';

import '../api/api_client.dart';
import '../api/api_exception.dart';
import '../model/entry.dart';
import '../model/timeline_groups.dart';
import '../module/module_catalog.dart';
import '../widgets/motion.dart';
import '../widgets/container_transform.dart';
import '../widgets/error_view.dart';
import '../widgets/format.dart';
import '../widgets/mock_banner.dart';
import '../widgets/navigation.dart';
import '../widgets/skeleton.dart';
import 'entry_screen.dart';

/// All entries of a module as a timeline axis with month sections.
/// Sort and order can be chosen; the backend sorts, the app shows the list
/// in the order it receives and only groups it.
class TimelineScreen extends StatefulWidget {
  const TimelineScreen({
    super.key,
    required this.api,
    required this.moduleId,
    this.onUpload,
  });

  final ApiClient api;

  /// Only entries with this `module` are shown.
  final String moduleId;

  /// Switches to the „Hochladen“ page (button in the empty state).
  final VoidCallback? onUpload;

  @override
  State<TimelineScreen> createState() => TimelineScreenState();
}

class TimelineScreenState extends State<TimelineScreen> {
  var _sort = Sort.documentDate;
  var _order = SortOrder.desc;

  List<EntrySummary>? _entries;
  ApiException? _error;
  var _loading = false;

  /// Counts the requests. If someone changes the sort quickly, responses may
  /// arrive in the wrong order; then only the last one counts.
  var _requestNumber = 0;

  @override
  void initState() {
    super.initState();
    reload();
  }

  /// Reloads the list. Public, so that e.g. the upload can refresh the
  /// timeline after a new entry.
  Future<void> reload() async {
    final number = ++_requestNumber;
    setState(() {
      _loading = true;
      _error = null;
    });
    try {
      final entries = await widget.api.listEntries(
        module: widget.moduleId,
        sort: _sort,
        order: _order,
      );
      if (!mounted || number != _requestNumber) return;
      setState(() {
        // The backend only returns entries of the module (?module=), sorted.
        _entries = entries;
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

  void _chooseSort(Sort sort) {
    if (sort == _sort) return;
    _sort = sort;
    reload();
  }

  void _toggleOrder() {
    _order = _order == SortOrder.desc ? SortOrder.asc : SortOrder.desc;
    reload();
  }

  /// Opens an entry. Public, so that the upload can show the entry right
  /// after uploading (or for a duplicate).
  Future<void> openEntry(String entryId) async {
    await openPage<void>(
      context,
      EntryScreen(api: widget.api, entryId: entryId),
    );
    // After a correction e.g. the date may have changed.
    if (mounted) reload();
  }

  @override
  Widget build(BuildContext context) {
    final descending = _order == SortOrder.desc;
    return Scaffold(
      appBar: AppBar(
        leading: const ModulesButton(),
        title: const Text('Zeitleiste'),
        actions: [
          MockBanner(visible: widget.api.isMock),
          PopupMenuButton<Sort>(
            icon: const Icon(Icons.sort),
            tooltip: 'Sortieren nach',
            onSelected: _chooseSort,
            itemBuilder: (_) => [
              CheckedPopupMenuItem(
                value: Sort.documentDate,
                checked: _sort == Sort.documentDate,
                child: const Text('Nach Dokumentdatum'),
              ),
              CheckedPopupMenuItem(
                value: Sort.importedAt,
                checked: _sort == Sort.importedAt,
                child: const Text('Nach Importdatum'),
              ),
            ],
          ),
          IconButton(
            icon: Icon(descending ? Icons.arrow_downward : Icons.arrow_upward),
            tooltip: descending
                ? 'Neueste zuerst (tippen: älteste zuerst)'
                : 'Älteste zuerst (tippen: neueste zuerst)',
            onPressed: _toggleOrder,
          ),
        ],
        bottom: PreferredSize(
          preferredSize: const Size.fromHeight(28),
          child: Column(
            children: [
              Padding(
                padding: const EdgeInsets.only(bottom: 6),
                child: Text(
                  '${_sort == Sort.documentDate ? 'Dokumentdatum' : 'Importdatum'}'
                  ' · ${descending ? 'neueste zuerst' : 'älteste zuerst'}',
                  style: Theme.of(context).textTheme.bodySmall,
                ),
              ),
              // Progress bar when a list is already there and is being reloaded.
              SizedBox(
                height: 4,
                child: _loading && _entries != null
                    ? const LinearProgressIndicator()
                    : null,
              ),
            ],
          ),
        ),
      ),
      body: _content(),
    );
  }

  /// Soft switch between skeleton, list, empty state and error.
  Widget _content() => FadeThroughSwitcher(child: _state());

  Widget _state() {
    final error = _error;
    if (error != null) {
      return ErrorView(
        key: const ValueKey('error'),
        error: error,
        onRetry: reload,
      );
    }
    final entries = _entries;
    if (entries == null) {
      // First load: skeleton in the form of the timeline axis.
      return const TimelineSkeleton(key: ValueKey('loading'));
    }
    return RefreshIndicator(
      key: ValueKey(entries.isEmpty ? 'leer' : 'liste'),
      onRefresh: reload,
      child: entries.isEmpty
          ? _EmptyTimeline(moduleId: widget.moduleId, onUpload: widget.onUpload)
          : _timelineAxis(entries),
    );
  }

  /// Timeline axis: month headers, on the left a continuous line with one
  /// dot per entry.
  Widget _timelineAxis(List<EntrySummary> entries) {
    final groups = groupTimeline(entries, _sort);
    // The entry view hangs on the top-level navigator; it gets the module's
    // color scheme passed along (as with openPage).
    final themes = InheritedTheme.capture(
      from: context,
      to: Navigator.of(context).context,
    );
    final lines = <Widget>[];
    for (var g = 0; g < groups.length; g++) {
      final group = groups[g];
      lines.add(
        _MonthHeader(
          key: ValueKey('month-${group.title}'),
          title: group.title,
          lineAbove: g > 0,
        ),
      );
      for (var i = 0; i < group.entries.length; i++) {
        final entry = group.entries[i];
        final last = g == groups.length - 1 && i == group.entries.length - 1;
        lines.add(
          // Key per entry: after deleting or re-sorting every row stays tied
          // to its entry (the reverse animation of the container transform
          // also lands in the right place).
          ContainerTransform(
            key: ValueKey('entry-${entry.id}'),
            closedBuilder: (context, onOpen) => _EntryRow(
              entry: entry,
              showImport: _sort == Sort.importedAt,
              lineBelow: !last,
              onTap: onOpen,
            ),
            openBuilder: (_) =>
                themes.wrap(EntryScreen(api: widget.api, entryId: entry.id)),
            // Reload after a correction or a deletion.
            onClosed: () {
              if (mounted) reload();
            },
          ),
        );
      }
    }
    return ListView(
      padding: const EdgeInsets.only(bottom: 24),
      children: lines,
    );
  }
}

/// Width of the left column with line and dots.
const _axisWidth = 64.0;

/// Paints the vertical line of the timeline axis in the left column.
class _AxisPainter extends CustomPainter {
  _AxisPainter({required this.color, required this.top, required this.bottom});

  final Color color;

  /// Paint the line from the top to the middle or from the middle to the bottom.
  final bool top;
  final bool bottom;

  @override
  void paint(Canvas canvas, Size size) {
    final x = size.width / 2;
    final y = size.height / 2;
    final paint = Paint()
      ..color = color
      ..strokeWidth = 2;
    if (top) canvas.drawLine(Offset(x, 0), Offset(x, y), paint);
    if (bottom) canvas.drawLine(Offset(x, y), Offset(x, size.height), paint);
  }

  @override
  bool shouldRepaint(_AxisPainter old) =>
      old.color != color || old.top != top || old.bottom != bottom;
}

class _MonthHeader extends StatelessWidget {
  const _MonthHeader({super.key, required this.title, required this.lineAbove});

  final String title;
  final bool lineAbove;

  @override
  Widget build(BuildContext context) {
    final theme = Theme.of(context);
    return Stack(
      children: [
        Positioned(
          left: 0,
          top: 0,
          bottom: 0,
          width: _axisWidth,
          child: ExcludeSemantics(
            child: CustomPaint(
              painter: _AxisPainter(
                color: theme.colorScheme.outlineVariant,
                top: lineAbove,
                bottom: true,
              ),
              child: Center(
                // Small node where the month starts.
                child: Container(
                  width: 10,
                  height: 10,
                  decoration: BoxDecoration(
                    color: theme.colorScheme.primary,
                    shape: BoxShape.circle,
                  ),
                ),
              ),
            ),
          ),
        ),
        Padding(
          padding: const EdgeInsets.fromLTRB(_axisWidth, 20, 16, 8),
          child: Semantics(
            header: true,
            child: Text(
              title,
              style: theme.textTheme.titleSmall?.copyWith(
                color: theme.colorScheme.primary,
                fontWeight: FontWeight.w600,
              ),
            ),
          ),
        ),
      ],
    );
  }
}

class _EntryRow extends StatelessWidget {
  const _EntryRow({
    required this.entry,
    required this.showImport,
    required this.lineBelow,
    required this.onTap,
  });

  final EntrySummary entry;

  /// When sorting by import date it is shown too, so that the order makes
  /// sense.
  final bool showImport;
  final bool lineBelow;
  final VoidCallback onTap;

  @override
  Widget build(BuildContext context) {
    final theme = Theme.of(context);
    final date = entry.documentDate;
    final issuer = entry.issuer;
    final grey = theme.colorScheme.onSurfaceVariant;
    final catalog = ModuleCatalog.of(context);
    // Soft tones from the type color; it only marks the type.
    final dot = ColorScheme.fromSeed(
      seedColor: catalog.typeColor(entry.module, entry.type),
      brightness: theme.brightness,
    );

    return Stack(
      children: [
        Positioned(
          left: 0,
          top: 0,
          bottom: 0,
          width: _axisWidth,
          child: ExcludeSemantics(
            child: CustomPaint(
              painter: _AxisPainter(
                color: theme.colorScheme.outlineVariant,
                top: true,
                bottom: lineBelow,
              ),
              child: Center(
                child: Container(
                  width: 36,
                  height: 36,
                  decoration: BoxDecoration(
                    color: dot.primaryContainer,
                    shape: BoxShape.circle,
                    // A ring in the background color separates the dot from the line.
                    border: Border.all(
                      color: theme.colorScheme.surface,
                      width: 3,
                    ),
                  ),
                  child: Icon(
                    catalog.typeIcon(entry.module, entry.type),
                    size: 18,
                    color: dot.onPrimaryContainer,
                  ),
                ),
              ),
            ),
          ),
        ),
        Padding(
          padding: const EdgeInsets.only(left: _axisWidth - 8),
          child: ListTile(
            contentPadding: const EdgeInsets.only(left: 8, right: 16),
            title: Text(catalog.typeName(entry.module, entry.type)),
            subtitle: Text(
              [
                issuer ?? 'Quelle unbekannt',
                if (showImport)
                  'Importiert: ${formatLocalTimestamp(entry.importedAt)}',
              ].join('\n'),
              maxLines: showImport ? 3 : 2,
              overflow: TextOverflow.ellipsis,
            ),
            isThreeLine: showImport,
            trailing: date != null
                ? Text(date.display)
                : Text(
                    'ohne Datum',
                    style: TextStyle(fontStyle: FontStyle.italic, color: grey),
                  ),
            onTap: onTap,
          ),
        ),
      ],
    );
  }
}

/// Empty state: large icon, hint and button to upload.
class _EmptyTimeline extends StatelessWidget {
  const _EmptyTimeline({required this.moduleId, required this.onUpload});

  final String moduleId;
  final VoidCallback? onUpload;

  @override
  Widget build(BuildContext context) {
    final theme = Theme.of(context);
    final icon =
        ModuleCatalog.of(context).byId(moduleId)?.icon ?? Icons.inbox_outlined;
    // ListView instead of Column, so that pull-to-refresh works here too.
    return ListView(
      padding: const EdgeInsets.fromLTRB(32, 64, 32, 32),
      children: [
        ExcludeSemantics(
          child: Center(
            child: Container(
              width: 112,
              height: 112,
              decoration: BoxDecoration(
                color: theme.colorScheme.primaryContainer,
                shape: BoxShape.circle,
              ),
              child: Icon(
                icon,
                size: 56,
                color: theme.colorScheme.onPrimaryContainer,
              ),
            ),
          ),
        ),
        const SizedBox(height: 24),
        Text(
          'Noch keine Dokumente.',
          textAlign: TextAlign.center,
          style: theme.textTheme.titleMedium,
        ),
        const SizedBox(height: 8),
        Text(
          'Lade ein Foto, ein PDF oder einen Text hoch. '
          'Es erscheint dann hier in der Zeitleiste.',
          textAlign: TextAlign.center,
          style: theme.textTheme.bodyMedium?.copyWith(
            color: theme.colorScheme.onSurfaceVariant,
          ),
        ),
        if (onUpload != null) ...[
          const SizedBox(height: 24),
          Center(
            child: FilledButton.icon(
              onPressed: onUpload,
              icon: const Icon(Icons.upload_file),
              label: const Text('Dokument hochladen'),
            ),
          ),
        ],
      ],
    );
  }
}
