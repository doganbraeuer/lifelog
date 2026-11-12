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
}

/// Real viewer with pdfx: zoomable, pages below each other.
class PdfxViewer implements PdfViewer {
  const PdfxViewer();

  @override
  Widget buildView(Uint8List bytes) => _PdfxView(bytes: bytes);
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
