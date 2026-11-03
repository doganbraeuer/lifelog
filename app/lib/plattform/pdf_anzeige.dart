import 'package:flutter/material.dart';
import 'package:flutter/services.dart';
import 'package:pdfx/pdfx.dart';

/// Zeigt ein PDF an.
///
/// pdfx nutzt die PDF-Funktionen von Android (`PdfRenderer`) und iOS
/// (`CGPDF`). Die gibt es in Widget-Tests nicht, deshalb liegt die Anzeige
/// hinter dieser Schnittstelle; Tests geben eine eigene an.
abstract interface class PdfAnzeige {
  Widget erstellen(Uint8List bytes);
}

/// Echte Anzeige mit pdfx: zoombar, Seiten untereinander.
class PdfxAnzeige implements PdfAnzeige {
  const PdfxAnzeige();

  @override
  Widget erstellen(Uint8List bytes) => _PdfxAnsicht(bytes: bytes);
}

class _PdfxAnsicht extends StatefulWidget {
  const _PdfxAnsicht({required this.bytes});

  final Uint8List bytes;

  @override
  State<_PdfxAnsicht> createState() => _PdfxAnsichtState();
}

class _PdfxAnsichtState extends State<_PdfxAnsicht> {
  // Der Controller lädt das Dokument direkt aus den Bytes, ohne Datei.
  late final _controller = PdfControllerPinch(
    document: PdfDocument.openData(widget.bytes),
  );
  var _seite = 1;
  var _seiten = 0;

  @override
  void dispose() {
    _controller.dispose();
    super.dispose();
  }

  @override
  Widget build(BuildContext context) {
    final farben = Theme.of(context).colorScheme;
    return Stack(
      children: [
        PdfViewPinch(
          controller: _controller,
          backgroundDecoration: BoxDecoration(color: farben.surfaceContainer),
          onDocumentLoaded: (dokument) =>
              setState(() => _seiten = dokument.pagesCount),
          onPageChanged: (seite) => setState(() => _seite = seite),
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
        if (_seiten > 0)
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
                  color: farben.inverseSurface.withValues(alpha: 0.85),
                  borderRadius: BorderRadius.circular(16),
                ),
                child: Text(
                  'Seite $_seite von $_seiten',
                  style: TextStyle(color: farben.onInverseSurface),
                ),
              ),
            ),
          ),
      ],
    );
  }
}
