import 'package:flutter/material.dart';
import 'package:flutter/services.dart';
import 'package:pdfx/pdfx.dart';

/// Shows a PDF.
///
/// pdfx uses the PDF functions of Android (`PdfRenderer`) and iOS (`CGPDF`).
/// They do not exist in widget tests, so the viewer sits behind this
/// interface; tests pass their own.
abstract interface class PdfViewer {
  Widget buildView(Uint8List bytes);

  /// Renders page [page] (starting at 1) as an image, for showing a
  /// location on it. Throws a [RangeError] if the PDF has no such page.
  Future<RenderedPdfPage> renderPage(Uint8List bytes, int page);
}

/// One page of a PDF rendered as an image.
class RenderedPdfPage {
  const RenderedPdfPage({
    required this.image,
    required this.width,
    required this.height,
    required this.pageCount,
  });

  /// The page as a PNG.
  final Uint8List image;

  /// Size of [image] in pixels; its aspect ratio is that of the page.
  final double width;
  final double height;

  /// Number of pages of the whole PDF.
  final int pageCount;
}

/// Real viewer with pdfx: zoomable, pages below each other.
class PdfxViewer implements PdfViewer {
  const PdfxViewer();

  /// Width of a rendered page in pixels: sharp enough for a zoom of about
  /// 3× on a phone, and an A4 page stays at about 23 MB in memory.
  static const renderWidth = 2000.0;

  @override
  Widget buildView(Uint8List bytes) => _PdfxView(bytes: bytes);

  @override
  Future<RenderedPdfPage> renderPage(Uint8List bytes, int page) async {
    final document = await PdfDocument.openData(bytes);
    try {
      final count = document.pagesCount;
      if (page < 1 || page > count) {
        throw RangeError.range(page, 1, count, 'page');
      }
      final pdfPage = await document.getPage(page);
      try {
        final height = renderWidth * pdfPage.height / pdfPage.width;
        final image = await pdfPage.render(
          width: renderWidth,
          height: height,
          format: PdfPageImageFormat.png,
          backgroundColor: '#FFFFFF',
        );
        if (image == null) throw StateError('page $page was not rendered');
        return RenderedPdfPage(
          image: image.bytes,
          width: (image.width ?? renderWidth).toDouble(),
          height: (image.height ?? height).toDouble(),
          pageCount: count,
        );
      } finally {
        await pdfPage.close();
      }
    } finally {
      await document.close();
    }
  }
}

class _PdfxView extends StatefulWidget {
  const _PdfxView({required this.bytes});

  final Uint8List bytes;

  @override
  State<_PdfxView> createState() => _PdfxViewState();
}

class _PdfxViewState extends State<_PdfxView> {
  // The controller loads the document straight from the bytes, without a
  // file.
  late final _controller = PdfControllerPinch(
    document: PdfDocument.openData(widget.bytes),
  );
  var _page = 1;
  var _pages = 0;

  @override
  void dispose() {
    _controller.dispose();
    super.dispose();
  }

  @override
  Widget build(BuildContext context) {
    final colors = Theme.of(context).colorScheme;
    return Stack(
      children: [
        PdfViewPinch(
          controller: _controller,
          backgroundDecoration: BoxDecoration(color: colors.surfaceContainer),
          onDocumentLoaded: (document) =>
              setState(() => _pages = document.pagesCount),
          onPageChanged: (page) => setState(() => _page = page),
          builders: PdfViewPinchBuilders<DefaultBuilderOptions>(
            options: const DefaultBuilderOptions(),
            documentLoaderBuilder: (_) =>
                const Center(child: CircularProgressIndicator()),
            pageLoaderBuilder: (_) =>
                const Center(child: CircularProgressIndicator()),
            errorBuilder: (_, _) => const Center(
              child: Padding(
                padding: EdgeInsets.all(24),
                child: Text(
                  'Das PDF kann nicht angezeigt werden. '
                  'Möglicherweise ist die Datei beschädigt oder geschützt.',
                  textAlign: TextAlign.center,
                ),
              ),
            ),
          ),
        ),
        if (_pages > 0)
          Positioned(
            bottom: 16,
            left: 0,
            right: 0,
            child: Center(
              child: Container(
                padding: const EdgeInsets.symmetric(
                  horizontal: 12,
                  vertical: 6,
                ),
                decoration: BoxDecoration(
                  color: colors.inverseSurface.withValues(alpha: 0.85),
                  borderRadius: BorderRadius.circular(16),
                ),
                child: Text(
                  'Seite $_page von $_pages',
                  style: TextStyle(color: colors.onInverseSurface),
                ),
              ),
            ),
          ),
      ],
    );
  }
}
