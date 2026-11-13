// Jump to the original (stage 1): shows where a value of an entry is in the
// original, using `verification.fields[].locations` (docs/API.md, section 2,
// "Verification").
//
// - PDF: only the page of the location, rendered as an image, with the
//   rectangles drawn over it (LocationOverlay).
// - Photo: the photo upright, with the rectangles over it.
// - Text import: `fullText` with the range highlighted, scrolled to it.
//
// Several locations can be stepped through („Fundstelle 1 von 2“).

import 'dart:typed_data';

import 'package:flutter/material.dart';

import '../api/api_client.dart';
import '../api/api_exception.dart';
import '../model/entry.dart';
import '../model/verification.dart';
import '../platform/pdf_viewer.dart';
import '../widgets/error_view.dart';
import '../widgets/location_overlay.dart';
import '../widgets/mock_banner.dart';
import '../widgets/navigation.dart';
import 'original_file_screen.dart';

/// Tooltip of the button that opens the whole original.
const openWholeOriginalLabel = 'Ganzes Original öffnen';

/// Shows the locations of one value of an entry in the original.
class OriginalLocationScreen extends StatefulWidget {
  const OriginalLocationScreen({
    super.key,
    required this.api,
    required this.entry,
    required this.label,
    required this.value,
    required this.locations,
    this.preloaded,
    this.pdfViewer = const PdfxViewer(),
  }) : assert(locations.length > 0);

  final ApiClient api;
  final Entry entry;

  /// Name of the field, e.g. „Dokumentdatum“ or „Hämoglobin“.
  final String label;

  /// The value as the entry shows it, e.g. „14,2 g/dl“.
  final String value;

  /// Where the value is in the original, best match first.
  final List<Location> locations;

  /// Original file that is already loaded (the image preview of the entry).
  final Future<FileContent>? preloaded;

  /// Replaced by a fake viewer in tests.
  final PdfViewer pdfViewer;

  @override
  State<OriginalLocationScreen> createState() => _OriginalLocationScreenState();
}

class _OriginalLocationScreenState extends State<OriginalLocationScreen> {
  var _index = 0;

  /// The original file; only loaded for locations on a page.
  Future<FileContent>? _file;
  FileContent? _loaded;

  /// Rendered PDF pages by page number, so stepping back and forth does not
  /// render a page again.
  final _pages = <int, Future<RenderedPdfPage>>{};

  @override
  void initState() {
    super.initState();
    if (widget.locations.any((l) => l is PageLocation)) {
      _loadFile(widget.preloaded);
    }
  }

  void _loadFile([Future<FileContent>? preloaded]) {
    final future = preloaded ?? widget.api.getFile(widget.entry.id);
    _file = future;
    future.then((file) {
      if (mounted) _loaded = file;
    }, onError: (_) {});
  }

  Future<RenderedPdfPage> _page(Uint8List bytes, int page) =>
      _pages[page] ??= widget.pdfViewer.renderPage(bytes, page);

  void _openWhole() {
    openPage<void>(
      context,
      OriginalFileScreen(
        api: widget.api,
        entryId: widget.entry.id,
        fileName: widget.entry.file.fileName,
        preloaded: _loaded,
        pdfViewer: widget.pdfViewer,
      ),
    );
  }

  @override
  Widget build(BuildContext context) {
    final theme = Theme.of(context);
    final locations = widget.locations;
    final current = locations[_index];
    // Page number in the bar, only for a PDF.
    final pdfPage = widget.entry.file.isPdf && current is PageLocation
        ? current.page
        : null;
    return Scaffold(
      appBar: AppBar(
        title: Column(
          crossAxisAlignment: CrossAxisAlignment.start,
          children: [
            Text(widget.label, overflow: TextOverflow.ellipsis),
            Text(
              widget.value,
              overflow: TextOverflow.ellipsis,
              style: theme.textTheme.bodyMedium?.copyWith(
                color: theme.colorScheme.onSurfaceVariant,
              ),
            ),
          ],
        ),
        actions: [
          MockBanner(visible: widget.api.isMock),
          IconButton(
            icon: const Icon(Icons.open_in_full),
            tooltip: openWholeOriginalLabel,
            onPressed: _openWhole,
          ),
        ],
      ),
      body: Column(
        children: [
          Expanded(
            child: Semantics(
              container: true,
              label: 'Stelle im Original: ${widget.value}',
              child: switch (current) {
                TextLocation() => TextLocationView(
                  fullText: widget.entry.fullText,
                  locations: locations.whereType<TextLocation>().toList(),
                  current: current,
                ),
                PageLocation() => _pageContent(current),
              },
            ),
          ),
          if (locations.length > 1 || pdfPage != null)
            _LocationBar(
              index: _index,
              count: locations.length,
              page: pdfPage,
              onChanged: (index) => setState(() => _index = index),
            ),
        ],
      ),
    );
  }

  Widget _pageContent(PageLocation current) {
    final onThisPage = [
      for (final l in widget.locations)
        if (l is PageLocation && l.page == current.page) l,
    ];
    return FutureBuilder<FileContent>(
      future: _file,
      builder: (context, snapshot) {
        final error = snapshot.error;
        if (error is ApiException) {
          return ErrorView(error: error, onRetry: () => setState(_loadFile));
        }
        if (snapshot.hasError) return _Unavailable(onOpen: _openWhole);
        final file = snapshot.data;
        if (file == null) {
          return const Center(child: CircularProgressIndicator());
        }
        if (file.isPdf) {
          return FutureBuilder<RenderedPdfPage>(
            future: _page(file.bytes, current.page),
            builder: (context, page) {
              if (page.hasError) return _Unavailable(onOpen: _openWhole);
              final rendered = page.data;
              if (rendered == null) {
                return const Center(child: CircularProgressIndicator());
              }
              return LocationOverlay(
                image: MemoryImage(rendered.image),
                pageSize: Size(rendered.width, rendered.height),
                locations: onThisPage,
                current: current,
              );
            },
          );
        }
        if (file.isImage) {
          return PhotoLocationView(
            bytes: file.bytes,
            locations: onThisPage,
            current: current,
            onOpen: _openWhole,
          );
        }
        return _Unavailable(onOpen: _openWhole);
      },
    );
  }
}

/// A photo with the locations marked. The rectangles refer to the photo as
/// shown upright, so the size is taken from the decoded image: Flutter
/// applies the EXIF orientation (and the platform the HEIC rotation) while
/// decoding, so width and height are already those of the upright view.
/// The size in the file header would be swapped for a portrait photo of an
/// iPhone.
class PhotoLocationView extends StatefulWidget {
  const PhotoLocationView({
    super.key,
    required this.bytes,
    required this.locations,
    required this.current,
    required this.onOpen,
  });

  final Uint8List bytes;
  final List<PageLocation> locations;
  final PageLocation current;
  final VoidCallback onOpen;

  @override
  State<PhotoLocationView> createState() => _PhotoLocationViewState();
}

class _PhotoLocationViewState extends State<PhotoLocationView> {
  late final _image = MemoryImage(widget.bytes);
  ImageStream? _stream;
  late final _listener = ImageStreamListener(
    (info, _) {
      final size = Size(
        info.image.width.toDouble(),
        info.image.height.toDouble(),
      );
      if (mounted && size != _size) setState(() => _size = size);
    },
    onError: (_, _) {
      if (mounted) setState(() => _failed = true);
    },
  );

  /// Size of the upright photo in pixels, once decoded.
  Size? _size;
  var _failed = false;

  @override
  void didChangeDependencies() {
    super.didChangeDependencies();
    final stream = _image.resolve(createLocalImageConfiguration(context));
    if (stream.key != _stream?.key) {
      _stream?.removeListener(_listener);
      _stream = stream..addListener(_listener);
    }
  }

  @override
  void dispose() {
    _stream?.removeListener(_listener);
    super.dispose();
  }

  @override
  Widget build(BuildContext context) {
    if (_failed) {
      return _Unavailable(
        text: 'Das Bild kann auf diesem Gerät nicht angezeigt werden.',
        onOpen: widget.onOpen,
      );
    }
    final size = _size;
    if (size == null) return const Center(child: CircularProgressIndicator());
    return LocationOverlay(
      image: _image,
      pageSize: size,
      locations: widget.locations,
      current: widget.current,
    );
  }
}

/// The full text of a text import with the locations highlighted, scrolled
/// so that the current one is in the upper third.
///
/// Locations count Unicode code points (`fullText.runes`); a Dart string and
/// `TextSpan` count UTF-16 code units, so they are converted with
/// [TextLocation.utf16Range].
class TextLocationView extends StatefulWidget {
  const TextLocationView({
    super.key,
    required this.fullText,
    required this.locations,
    required this.current,
  });

  final String fullText;
  final List<TextLocation> locations;
  final TextLocation current;

  @override
  State<TextLocationView> createState() => _TextLocationViewState();
}

class _TextLocationViewState extends State<TextLocationView> {
  static const _padding = 16.0;
  final _scroll = ScrollController();
  (TextLocation, Size)? _scrolledFor;

  @override
  void dispose() {
    _scroll.dispose();
    super.dispose();
  }

  /// Scrolls to the current location after the frame, once per location and
  /// size of the view.
  void _scrollLater(Size viewport, TextStyle style) {
    final key = (widget.current, viewport);
    if (_scrolledFor == key) return;
    _scrolledFor = key;
    final textScaler = MediaQuery.textScalerOf(context);
    WidgetsBinding.instance.addPostFrameCallback((_) {
      if (!mounted || !_scroll.hasClients) return;
      final (start, end) = widget.current.utf16Range(widget.fullText);
      if (start >= end) return;
      // Lay out the text the way the view does, to find the line.
      final painter = TextPainter(
        text: TextSpan(text: widget.fullText, style: style),
        textDirection: TextDirection.ltr,
        textScaler: textScaler,
      )..layout(maxWidth: viewport.width - 2 * _padding);
      final boxes = painter.getBoxesForSelection(
        TextSelection(baseOffset: start, extentOffset: end),
      );
      painter.dispose();
      if (boxes.isEmpty) return;
      final top = _padding + boxes.first.top - viewport.height / 3;
      _scroll.jumpTo(top.clamp(0.0, _scroll.position.maxScrollExtent));
    });
  }

  @override
  Widget build(BuildContext context) {
    final theme = Theme.of(context);
    final style = DefaultTextStyle.of(context).style
        .merge(theme.textTheme.bodyLarge);
    return LayoutBuilder(
      builder: (context, constraints) {
        _scrollLater(constraints.biggest, style);
        return Scrollbar(
          controller: _scroll,
          child: SingleChildScrollView(
            controller: _scroll,
            padding: const EdgeInsets.all(_padding),
            child: SelectableText.rich(
              TextSpan(children: _spans(theme.colorScheme)),
              style: style,
            ),
          ),
        );
      },
    );
  }

  /// The text in pieces: plain, highlighted, plain, ...
  List<TextSpan> _spans(ColorScheme colors) {
    final text = widget.fullText;
    // Overlapping ranges are skipped below. With the same start the current
    // one comes first, so a range with the same text never hides it.
    final ranges =
        [
          for (final location in widget.locations)
            (location.utf16Range(text), identical(location, widget.current)),
        ]..sort((a, b) {
          final byStart = a.$1.$1.compareTo(b.$1.$1);
          if (byStart != 0) return byStart;
          return (b.$2 ? 1 : 0) - (a.$2 ? 1 : 0);
        });
    final spans = <TextSpan>[];
    var at = 0;
    for (final ((start, end), isCurrent) in ranges) {
      if (start < at || start >= end) continue; // overlapping or empty
      if (start > at) spans.add(TextSpan(text: text.substring(at, start)));
      spans.add(
        TextSpan(
          text: text.substring(start, end),
          style: TextStyle(
            backgroundColor: isCurrent
                ? colors.primaryContainer
                : colors.primaryContainer.withValues(alpha: 0.45),
            color: isCurrent ? colors.onPrimaryContainer : null,
          ),
        ),
      );
      at = end;
    }
    if (at < text.length) spans.add(TextSpan(text: text.substring(at)));
    return spans;
  }
}

/// Bar at the bottom: „Fundstelle 1 von 2 · Seite 1“ with buttons to step
/// through the locations.
class _LocationBar extends StatelessWidget {
  const _LocationBar({
    required this.index,
    required this.count,
    required this.page,
    required this.onChanged,
  });

  final int index;
  final int count;

  /// Page of the current location; `null` if it is not a PDF.
  final int? page;
  final ValueChanged<int> onChanged;

  @override
  Widget build(BuildContext context) {
    final colors = Theme.of(context).colorScheme;
    final several = count > 1;
    final text = [
      if (several) 'Fundstelle ${index + 1} von $count',
      if (page != null) 'Seite $page',
    ].join(' · ');
    return Material(
      color: colors.surfaceContainerHigh,
      child: SafeArea(
        top: false,
        child: SizedBox(
          height: 56,
          child: Row(
            children: [
              if (several)
                IconButton(
                  icon: const Icon(Icons.chevron_left),
                  tooltip: 'Vorherige Fundstelle',
                  onPressed: index > 0 ? () => onChanged(index - 1) : null,
                ),
              Expanded(child: Text(text, textAlign: TextAlign.center)),
              if (several)
                IconButton(
                  icon: const Icon(Icons.chevron_right),
                  tooltip: 'Nächste Fundstelle',
                  onPressed: index < count - 1
                      ? () => onChanged(index + 1)
                      : null,
                ),
            ],
          ),
        ),
      ),
    );
  }
}

/// The location cannot be shown (page missing, rendering failed, unknown
/// file type); the whole original can still be opened.
class _Unavailable extends StatelessWidget {
  const _Unavailable({
    this.text = 'Diese Stelle kann nicht angezeigt werden.',
    required this.onOpen,
  });

  final String text;
  final VoidCallback onOpen;

  @override
  Widget build(BuildContext context) {
    return Center(
      child: Padding(
        padding: const EdgeInsets.all(24),
        child: Column(
          mainAxisSize: MainAxisSize.min,
          children: [
            Text(text, textAlign: TextAlign.center),
            const SizedBox(height: 16),
            FilledButton.tonalIcon(
              onPressed: onOpen,
              icon: const Icon(Icons.open_in_full),
              label: const Text(openWholeOriginalLabel),
            ),
          ],
        ),
      ),
    );
  }
}
