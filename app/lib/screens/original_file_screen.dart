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
    required this.eintragId,
    required this.dateiname,
    this.vorhanden,
    this.pdfAnzeige = const PdfxAnzeige(),
  });

  final ApiClient api;
  final String eintragId;
  final String dateiname;

  /// Schon geladene Datei (z. B. das Vorschaubild), spart einen Download.
  final DateiInhalt? vorhanden;

  /// In Tests durch eine Fake-Anzeige ersetzt.
  final PdfAnzeige pdfAnzeige;

  @override
  State<OriginaldateiScreen> createState() => _OriginaldateiScreenState();
}

class _OriginaldateiScreenState extends State<OriginaldateiScreen> {
  late DateiInhalt? _datei = widget.vorhanden;
  ApiFehler? _fehler;

  @override
  void initState() {
    super.initState();
    if (_datei == null) _laden();
  }

  Future<void> _laden() async {
    setState(() => _fehler = null);
    try {
      final datei = await widget.api.datei(widget.eintragId);
      if (mounted) setState(() => _datei = datei);
    } on ApiFehler catch (fehler) {
      if (mounted) setState(() => _fehler = fehler);
    }
  }

  @override
  Widget build(BuildContext context) {
    return Scaffold(
      appBar: AppBar(
        title: Text(widget.dateiname, overflow: TextOverflow.ellipsis),
        actions: [MockHinweis(sichtbar: widget.api.istMock)],
      ),
      body: _inhalt(),
    );
  }

  Widget _inhalt() {
    final fehler = _fehler;
    if (fehler != null) return FehlerAnzeige(fehler: fehler, erneut: _laden);
    final datei = _datei;
    if (datei == null) return const Center(child: CircularProgressIndicator());

    if (datei.istPdf) return widget.pdfAnzeige.erstellen(datei.bytes);
    if (datei.istText) return _TextAnsicht(bytes: datei.bytes);
    if (datei.istBild) {
      // Mit zwei Fingern zoomen und verschieben.
      return InteractiveViewer(
        maxScale: 8,
        child: Center(
          child: Image.memory(
            datei.bytes,
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
          'Dieser Dateityp (${datei.mimeTyp}) kann nicht angezeigt werden.',
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
