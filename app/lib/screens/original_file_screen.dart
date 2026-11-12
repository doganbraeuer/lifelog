import 'dart:convert';
import 'dart:typed_data';

import 'package:flutter/material.dart';

import '../api/api_client.dart';
import '../api/api_exception.dart';
import '../model/entry.dart';
import '../platform/pdf_viewer.dart';
import '../widgets/error_view.dart';
import '../widgets/mock_banner.dart';

/// Shows the original file of an entry: images zoomable, PDFs with pdfx.
///
/// This makes it possible to check whether the vision LLM read correctly.
class OriginalFileScreen extends StatefulWidget {
  const OriginalFileScreen({
    super.key,
    required this.api,
    required this.entryId,
    required this.fileName,
    this.preloaded,
    this.pdfViewer = const PdfxViewer(),
  });

  final ApiClient api;
  final String entryId;
  final String fileName;

  /// File that is already loaded (e.g. the preview image), saves a download.
  final FileContent? preloaded;

  /// Replaced by a fake viewer in tests.
  final PdfViewer pdfViewer;

  @override
  State<OriginalFileScreen> createState() => _OriginalFileScreenState();
}

class _OriginalFileScreenState extends State<OriginalFileScreen> {
  late FileContent? _file = widget.preloaded;
  ApiException? _error;

  @override
  void initState() {
    super.initState();
    if (_file == null) _load();
  }

  Future<void> _load() async {
    setState(() => _error = null);
    try {
      final file = await widget.api.getFile(widget.entryId);
      if (mounted) setState(() => _file = file);
    } on ApiException catch (error) {
      if (mounted) setState(() => _error = error);
    }
  }

  @override
  Widget build(BuildContext context) {
    return Scaffold(
      appBar: AppBar(
        title: Text(widget.fileName, overflow: TextOverflow.ellipsis),
        actions: [MockBanner(visible: widget.api.isMock)],
      ),
      body: _content(),
    );
  }

  Widget _content() {
    final error = _error;
    if (error != null) return ErrorView(error: error, onRetry: _load);
    final file = _file;
    if (file == null) return const Center(child: CircularProgressIndicator());

    if (file.isPdf) return widget.pdfViewer.buildView(file.bytes);
    if (file.isText) return _TextView(bytes: file.bytes);
    if (file.isImage) {
      // Zoom and pan with two fingers.
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

/// Shows a text file: UTF-8, selectable and scrollable.
class _TextView extends StatelessWidget {
  const _TextView({required this.bytes});

  final Uint8List bytes;

  /// Decodes UTF-8. Invalid bytes become „�“ instead of an exception, so
  /// that a broken file does not crash the view.
  /// A BOM at the start (from some Windows editors) is removed.
  static String decoded(Uint8List bytes) {
    final text = utf8.decode(bytes, allowMalformed: true);
    return text.startsWith('﻿') ? text.substring(1) : text;
  }

  @override
  Widget build(BuildContext context) {
    final text = decoded(bytes);
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
