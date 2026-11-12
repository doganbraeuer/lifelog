import 'dart:convert';
import 'dart:typed_data';

import 'package:flutter/material.dart';

import '../api/api_client.dart';
import '../api/api_exception.dart';
import '../model/entry.dart';
import '../platform/pdf_viewer.dart';
import '../widgets/error_view.dart';
import '../widgets/mock_banner.dart';

/// Zeigt die Originaldatei eines Eintrags: Bilder zoombar, PDFs mit pdfx.
///
/// So lässt sich prüfen, ob das Vision-LLM richtig gelesen hat.
class OriginaldateiScreen extends StatefulWidget {
  const OriginaldateiScreen({
    super.key,
    required this.api,
    required this.entryId,
    required this.fileName,
    this.vorhanden,
    this.pdfViewer = const PdfxViewer(),
  });

  final ApiClient api;
  final String entryId;
  final String fileName;

  /// Schon geladene Datei (z. B. das Vorschaubild), spart einen Download.
  final FileContent? vorhanden;

  /// In Tests durch eine Fake-Anzeige ersetzt.
  final PdfViewer pdfViewer;

  @override
  State<OriginaldateiScreen> createState() => _OriginaldateiScreenState();
}

class _OriginaldateiScreenState extends State<OriginaldateiScreen> {
  late FileContent? _datei = widget.vorhanden;
  ApiException? _fehler;

  @override
  void initState() {
    super.initState();
    if (_datei == null) _laden();
  }

  Future<void> _laden() async {
    setState(() => _fehler = null);
    try {
      final file = await widget.api.getFile(widget.entryId);
      if (mounted) setState(() => _datei = file);
    } on ApiException catch (fehler) {
      if (mounted) setState(() => _fehler = fehler);
    }
  }

  @override
  Widget build(BuildContext context) {
    return Scaffold(
      appBar: AppBar(
        title: Text(widget.fileName, overflow: TextOverflow.ellipsis),
        actions: [MockHinweis(sichtbar: widget.api.isMock)],
      ),
      body: _inhalt(),
    );
  }

  Widget _inhalt() {
    final fehler = _fehler;
    if (fehler != null) return FehlerAnzeige(fehler: fehler, erneut: _laden);
    final file = _datei;
    if (file == null) return const Center(child: CircularProgressIndicator());

    if (file.isPdf) return widget.pdfViewer.buildView(file.bytes);
    if (file.isText) return _TextAnsicht(bytes: file.bytes);
    if (file.isImage) {
      // Mit zwei Fingern zoomen und verschieben.
      return InteractiveViewer(
        maxScale: 8,
        child: Center(
          child: Image.memory(
            file.bytes,
            fit: BoxFit.contain,
            errorBuilder: (_, _, _) => const Padding(
              padding: EdgeInsets.all(24),
              child: Text(
                'Das Bild kann auf diesem Gerät nicht angezeigt werden.',
                textAlign: TextAlign.center,
              ),
            ),
          ),
        ),
      );
    }
    return Center(
      child: Padding(
        padding: const EdgeInsets.all(24),
        child: Text(
          'Dieser Dateityp (${file.mimeType}) kann nicht angezeigt werden.',
          textAlign: TextAlign.center,
        ),
      ),
    );
  }
}

/// Zeigt eine Textdatei: UTF-8, markierbar und scrollbar.
class _TextAnsicht extends StatelessWidget {
  const _TextAnsicht({required this.bytes});

  final Uint8List bytes;

  /// Dekodiert UTF-8. Ungültige Bytes werden zu „�“ statt zu einer
  /// Exception, damit eine kaputte Datei die Ansicht nicht abstürzen lässt.
  /// Ein BOM am Anfang (von manchen Windows-Editoren) wird entfernt.
  static String dekodieren(Uint8List bytes) {
    final text = utf8.decode(bytes, allowMalformed: true);
    return text.startsWith('﻿') ? text.substring(1) : text;
  }

  @override
  Widget build(BuildContext context) {
    final text = dekodieren(bytes);
    if (text.trim().isEmpty) {
      return const Center(child: Text('Die Textdatei ist leer.'));
    }
    return Scrollbar(
      child: SingleChildScrollView(
        padding: const EdgeInsets.all(16),
        child: SelectableText(
          text,
          style: Theme.of(context).textTheme.bodyLarge,
        ),
      ),
    );
  }
}
