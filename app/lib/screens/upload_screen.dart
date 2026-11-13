import 'package:flutter/material.dart';
import 'package:flutter/services.dart';

import '../api/api_client.dart';
import '../api/api_exception.dart';
import '../api/mime_type.dart';
import '../model/entry.dart';
import '../model/server_status.dart';
import '../platform/file_chooser.dart';
import '../platform/voice_recorder.dart';
import '../widgets/motion.dart';
import '../widgets/format.dart';
import '../widgets/mock_banner.dart';
import '../widgets/navigation.dart';
import '../widgets/quantity_highlight.dart';
import '../widgets/scan_animation.dart';
import 'voice_note_view.dart';

/// Privacy notice, always at the top of the screen.
const cloudNotice =
    'Dokumente werden zur Auswertung an einen Cloud-Dienst gesendet. '
    'Bitte nur Testdokumente verwenden.';

/// Take a photo, choose an image or PDF and upload it, or enter the
/// document content as text.
///
/// Flow: choose a file → check the preview → „Hochladen“. The preview
/// prevents the wrong file from going to the cloud by accident.
/// For text: type or paste → „Importieren“.
class UploadScreen extends StatefulWidget {
  const UploadScreen({
    super.key,
    required this.api,
    required this.fileChooser,
    required this.voiceRecorder,
    required this.moduleId,
    required this.uploaded,
    required this.openEntry,
  });

  final ApiClient api;
  final FileChooser fileChooser;

  /// Microphone for spoken notes.
  final VoiceRecorder voiceRecorder;

  /// Module to upload into (field `module` of the upload).
  final String moduleId;

  /// Called with the new entry after a successful upload.
  final void Function(Entry entry) uploaded;

  /// Opens an existing entry (for a duplicate).
  final void Function(String entryId) openEntry;

  @override
  State<UploadScreen> createState() => _UploadScreenState();
}

class _UploadScreenState extends State<UploadScreen> {
  ChosenFile? _file;
  ApiException? _error;
  var _uploading = false;

  /// `true` while the text field for the text import is open.
  var _textMode = false;
  final _text = QuantityHighlightController();

  /// `true` while recording or transcribing a spoken note.
  var _voiceMode = false;

  /// `true` if the text in the field comes from a spoken note: it is
  /// unchecked machine output, so the page asks to check it and marks
  /// numbers and units in it. Kept in the controller, so that the marks
  /// and this flag can never disagree.
  bool get _textFromSpeech => _text.highlightQuantities;
  set _textFromSpeech(bool value) => _text.highlightQuantities = value;

  /// Field `speech` from `/status`; `null` while it is loading. A failed
  /// request counts as `unavailable`.
  SpeechStatus? _speech;

  @override
  void dispose() {
    _text.dispose();
    super.dispose();
  }

  /// For these errors a second attempt with the same file is worthwhile.
  static const _retryable = {
    ErrorCode.networkError,
    ErrorCode.internalError,
    ErrorCode.extractionFailed,
    ErrorCode.modelUnavailable,
    ErrorCode.unexpectedResponse,
  };

  @override
  void initState() {
    super.initState();
    _retrieveLostFile();
    _loadSpeechStatus();
    // So that „Importieren“ becomes enabled or disabled while typing.
    _text.addListener(() => setState(() {}));
  }

  /// Asks `/status` whether spoken notes work right now (docs/API.md 3.8).
  Future<void> _loadSpeechStatus() async {
    SpeechStatus speech;
    try {
      speech = (await widget.api.getStatus()).speech;
    } on ApiException {
      speech = SpeechStatus.unavailable;
    }
    if (mounted) setState(() => _speech = speech);
  }

  void _checkSpeechAgain() {
    setState(() => _speech = null);
    _loadSpeechStatus();
  }

  Future<void> _retrieveLostFile() async {
    try {
      final file = await widget.fileChooser.retrieveLostFile();
      if (file != null && mounted) _accept(file);
    } on PlatformException {
      // Nothing to restore; no reason for a message.
    }
  }

  Future<void> _choose(Future<ChosenFile?> Function() chooser) async {
    try {
      final file = await chooser();
      if (file != null && mounted) _accept(file);
    } on FileTooLargeException {
      _showLocalError();
    } on PlatformException catch (e) {
      if (!mounted) return;
      setState(() {
        _error = ApiException(
          code: ErrorCode.pickFailed,
          message: e.code.contains('access_denied')
              ? 'Kein Zugriff auf Kamera oder Fotos. Bitte in den '
                    'Einstellungen des Geräts erlauben.'
              : 'Die Auswahl hat nicht geklappt. Bitte noch einmal versuchen.',
        );
      });
    }
  }

  void _accept(ChosenFile file) {
    // Check the size here already, so that nothing is uploaded needlessly.
    // The backend still checks and reports 413 if needed.
    if (file.bytes.length > maxFileSize) {
      _showLocalError();
      return;
    }
    setState(() {
      _file = file;
      _error = null;
    });
  }

  void _showLocalError() {
    if (!mounted) return;
    setState(() {
      _file = null;
      _error = const ApiException(
        code: ErrorCode.fileTooLarge,
        message:
            'Die Datei ist größer als 10 MB. Bitte eine kleinere Datei wählen.',
      );
    });
  }

  Future<void> _upload() async {
    final file = _file;
    if (file == null) return;
    setState(() {
      _uploading = true;
      _error = null;
    });
    try {
      final entry = await widget.api.upload(
        file.bytes,
        file.name,
        module: widget.moduleId,
      );
      if (!mounted) return;
      setState(() {
        _uploading = false;
        _file = null;
      });
      ScaffoldMessenger.of(context)
          .showSnackBar(const SnackBar(content: Text('Dokument gespeichert.')));
      HapticFeedback.lightImpact(); // Make success noticeable
      widget.uploaded(entry);
    } on ApiException catch (error) {
      if (!mounted) return;
      setState(() {
        _uploading = false;
        _error = error;
        // Without a chance of success discard the file, so that another one can
        // be chosen right away.
        if (!_retryable.contains(error.code)) _file = null;
      });
    }
  }

  void _discard() => setState(() {
    _file = null;
    _error = null;
  });

  // -------------------------------------------------------------------------
  // Text import
  // -------------------------------------------------------------------------

  void _openText() => setState(() {
    _textMode = true;
    _textFromSpeech = false;
    _error = null;
  });

  void _cancelText() => setState(() {
    _textMode = false;
    _textFromSpeech = false;
    _text.clear();
    _error = null;
  });

  Future<void> _importText() async {
    final text = _text.text;
    if (text.trim().isEmpty) return;
    // Close the keyboard. Unfocus the field itself, not the scope: a scope
    // would remember the field and give the focus back later.
    FocusManager.instance.primaryFocus?.unfocus();
    setState(() {
      _uploading = true;
      _error = null;
    });
    try {
      final entry = await widget.api.importText(text, module: widget.moduleId);
      if (!mounted) return;
      setState(() {
        _uploading = false;
        _textMode = false;
        _textFromSpeech = false;
        _text.clear();
      });
      ScaffoldMessenger.of(context)
          .showSnackBar(const SnackBar(content: Text('Text gespeichert.')));
      HapticFeedback.lightImpact(); // Make success noticeable
      widget.uploaded(entry);
    } on ApiException catch (error) {
      if (!mounted) return;
      // The text stays after every error, so that it can be changed
      // (e.g. shortened) or sent again.
      setState(() {
        _uploading = false;
        _error = error;
      });
    }
  }

  // -------------------------------------------------------------------------
  // Spoken note (docs/API.md 3.9)
  // -------------------------------------------------------------------------

  /// Asks for the microphone, then opens the recording view.
  Future<void> _openVoice() async {
    setState(() => _error = null);
    final allowed = await widget.voiceRecorder.requestPermission();
    if (!mounted) return;
    setState(() {
      if (allowed) {
        _voiceMode = true;
      } else {
        _error = const ApiException(
          code: ErrorCode.recordingFailed,
          message:
              'Kein Zugriff auf das Mikrofon. Bitte in den Einstellungen '
              'des Geräts erlauben.',
        );
      }
    });
  }

  /// The backend recognized text: show it in the text field for checking.
  /// It is only saved when the user confirms („Speichern“).
  void _showTranscript(String text) {
    _text.text = text;
    setState(() {
      _voiceMode = false;
      _textMode = true;
      _textFromSpeech = true;
      _error = null;
    });
  }

  void _closeVoice() => setState(() => _voiceMode = false);

  void _showVoiceError(ApiException? error) => setState(() => _error = error);

  @override
  Widget build(BuildContext context) {
    return Scaffold(
      appBar: AppBar(
        leading: const ModulesButton(),
        title: const Text('Hochladen'),
        actions: [MockBanner(visible: widget.api.isMock)],
      ),
      body: Column(
        children: [
          Expanded(
            child: ListView(
              padding: const EdgeInsets.all(16),
              children: [
                const _Notice(),
                const SizedBox(height: 16),
                // The error card appears and disappears softly; the height
                // glides along instead of the content below jumping.
                AnimatedSize(
                  duration: motionDuration(context, Durations.medium2),
                  curve: Easing.emphasizedDecelerate,
                  alignment: Alignment.topCenter,
                  child: FadeThroughSwitcher(
                    child: switch (_error) {
                      final error? => Padding(
                        key: ObjectKey(error),
                        padding: const EdgeInsets.only(bottom: 16),
                        child: _ErrorCard(
                          error: error,
                          openEntry: widget.openEntry,
                        ),
                      ),
                      null => const SizedBox.shrink(key: ValueKey('ok')),
                    },
                  ),
                ),
                // Soft switch between choice, preview, text field and
                // recording.
                FadeThroughSwitcher(
                  child: switch ((_file, _textMode, _voiceMode)) {
                    (final file?, _, _) => KeyedSubtree(
                      key: const ValueKey('preview'),
                      child: _preview(file),
                    ),
                    (null, _, true) => KeyedSubtree(
                      key: const ValueKey('voice'),
                      child: VoiceNoteView(
                        api: widget.api,
                        recorder: widget.voiceRecorder,
                        onTranscribed: _showTranscript,
                        onClosed: _closeVoice,
                        onError: _showVoiceError,
                      ),
                    ),
                    (null, true, false) => KeyedSubtree(
                      key: const ValueKey('text'),
                      child: _textInput(),
                    ),
                    (null, false, false) => KeyedSubtree(
                      key: const ValueKey('choose'),
                      child: _chooserButtons(),
                    ),
                  },
                ),
              ],
            ),
          ),
          // Fixed bar below the list: it does not scroll and sits right above
          // the open keyboard (the shell shrinks the screen). That way the
          // buttons stay reachable even with long text.
          if (_textMode && _file == null && !_uploading) _textActions(),
        ],
      ),
    );
  }

  Widget _textActions() {
    final error = _error;
    final retry = error != null && _retryable.contains(error.code);
    return Material(
      color: Theme.of(context).colorScheme.surfaceContainer,
      child: SafeArea(
        top: false,
        child: Padding(
          padding: const EdgeInsets.fromLTRB(16, 8, 16, 8),
          // Both buttons share the width; labels that are too long are
          // shortened instead of overflowing (narrow phones, large text).
          child: Row(
            children: [
              Expanded(
                child: TextButton(
                  onPressed: _cancelText,
                  child: const Text(
                    'Abbrechen',
                    maxLines: 1,
                    overflow: TextOverflow.ellipsis,
                  ),
                ),
              ),
              const SizedBox(width: 8),
              Expanded(
                flex: 2,
                child: FilledButton.icon(
                  onPressed: _text.text.trim().isEmpty ? null : _importText,
                  icon: Icon(retry ? Icons.refresh : Icons.cloud_upload),
                  label: Text(
                    retry
                        ? 'Erneut versuchen'
                        : _textFromSpeech
                        ? 'Speichern'
                        : 'Importieren',
                    maxLines: 1,
                    overflow: TextOverflow.ellipsis,
                  ),
                ),
              ),
            ],
          ),
        ),
      ),
    );
  }

  Widget _chooserButtons() {
    final chooser = widget.fileChooser;
    return Column(
      crossAxisAlignment: CrossAxisAlignment.stretch,
      children: [
        FilledButton.icon(
          onPressed: () => _choose(chooser.takePhoto),
          icon: const Icon(Icons.photo_camera_outlined),
          label: const Text('Foto aufnehmen'),
        ),
        const SizedBox(height: 8),
        OutlinedButton.icon(
          onPressed: () => _choose(chooser.pickImage),
          icon: const Icon(Icons.photo_library_outlined),
          label: const Text('Bild auswählen'),
        ),
        const SizedBox(height: 8),
        OutlinedButton.icon(
          onPressed: () => _choose(chooser.pickPdf),
          icon: const Icon(Icons.picture_as_pdf_outlined),
          label: const Text('PDF auswählen'),
        ),
        const SizedBox(height: 8),
        OutlinedButton.icon(
          onPressed: _openText,
          icon: const Icon(Icons.notes),
          label: const Text('Text eingeben'),
        ),
        // Switched off on the server: no button at all. Otherwise only
        // usable when /status says the speech recognition answers.
        if (_speech != SpeechStatus.off) ...[
          const SizedBox(height: 8),
          OutlinedButton.icon(
            onPressed: _speech == SpeechStatus.ok ? _openVoice : null,
            icon: const Icon(Icons.mic_none),
            label: const Text('Sprachnotiz'),
          ),
        ],
        if (_speech == SpeechStatus.unavailable) _speechUnavailable(),
        const SizedBox(height: 12),
        Text(
          // Only promise spoken notes when they work right now.
          _speech == SpeechStatus.ok
              ? 'Erlaubt sind JPEG, PNG, HEIC, WebP und PDF bis 10 MB, '
                    'Text bis 100 000 Zeichen oder eine Sprachnotiz bis '
                    '5 Minuten.'
              : 'Erlaubt sind JPEG, PNG, HEIC, WebP und PDF bis 10 MB '
                    'oder Text bis 100 000 Zeichen.',
          textAlign: TextAlign.center,
          style: Theme.of(context).textTheme.bodySmall,
        ),
      ],
    );
  }

  Widget _speechUnavailable() {
    final theme = Theme.of(context);
    return Padding(
      padding: const EdgeInsets.only(top: 4),
      child: Row(
        children: [
          Expanded(
            child: Text(
              'Sprachnotizen sind gerade nicht verfügbar.',
              style: theme.textTheme.bodySmall?.copyWith(
                color: theme.colorScheme.onSurfaceVariant,
              ),
            ),
          ),
          TextButton(
            onPressed: _checkSpeechAgain,
            child: const Text('Erneut prüfen'),
          ),
        ],
      ),
    );
  }

  /// Text field; the buttons are in the fixed bar ([_textActions]).
  Widget _textInput() {
    return Column(
      crossAxisAlignment: CrossAxisAlignment.stretch,
      children: [
        if (_textFromSpeech) ...[
          const _TranscriptNotice(),
          const SizedBox(height: 12),
        ],
        TextField(
          controller: _text,
          enabled: !_uploading,
          minLines: 6,
          maxLines: 14,
          maxLength: maxTextLength,
          keyboardType: TextInputType.multiline,
          decoration: const InputDecoration(
            labelText: 'Dokumentinhalt',
            hintText: 'Text des Dokuments eingeben oder einfügen',
            alignLabelWithHint: true,
            border: OutlineInputBorder(),
          ),
        ),
        FadeThroughSwitcher(
          child: _uploading
              ? const Padding(
                  key: ValueKey('loading'),
                  padding: EdgeInsets.only(top: 16),
                  child: _Processing(),
                )
              : const SizedBox.shrink(key: ValueKey('ready')),
        ),
      ],
    );
  }

  Widget _preview(ChosenFile file) {
    final mimeType = detectMimeType(file.bytes);
    final isImage = mimeType != null && mimeType.startsWith('image/');
    final retry = _error != null;
    return Column(
      crossAxisAlignment: CrossAxisAlignment.stretch,
      children: [
        Card(
          clipBehavior: Clip.antiAlias,
          // The stack puts the X icon over the preview, top right.
          child: Stack(
            children: [
              Column(
                children: [
                  if (isImage)
                    Image.memory(
                      file.bytes,
                      height: 260,
                      fit: BoxFit.contain,
                      // Android may not be able to show HEIC; then an icon.
                      errorBuilder: (_, _, _) => const _FileIcon(Icons.image),
                    )
                  else
                    const _FileIcon(Icons.picture_as_pdf_outlined),
                  ListTile(
                    title: Text(file.name),
                    subtitle: Text(formatSize(file.bytes.length)),
                  ),
                ],
              ),
              // Hidden during the upload: the file is already on its way then
              // and should not disappear in the middle of the process.
              if (!_uploading)
                Positioned(
                  top: 4,
                  right: 4,
                  child: IconButton.filledTonal(
                    onPressed: _discard,
                    icon: const Icon(Icons.close),
                    tooltip: 'Datei verwerfen',
                  ),
                ),
            ],
          ),
        ),
        const SizedBox(height: 16),
        FadeThroughSwitcher(
          child: _uploading
              ? const _Processing(key: ValueKey('loading'))
              : Column(
                  key: const ValueKey('buttons'),
                  crossAxisAlignment: CrossAxisAlignment.stretch,
                  children: [
                    FilledButton.icon(
                      onPressed: _upload,
                      icon: Icon(retry ? Icons.refresh : Icons.cloud_upload),
                      label: Text(retry ? 'Erneut versuchen' : 'Hochladen'),
                    ),
                    const SizedBox(height: 8),
                    TextButton(
                      onPressed: _discard,
                      child: const Text('Andere Datei wählen'),
                    ),
                  ],
                ),
        ),
      ],
    );
  }
}

/// Shown while the backend processes (up to a minute): a calm scan
/// animation without made-up progress.
class _Processing extends StatelessWidget {
  const _Processing({super.key});

  @override
  Widget build(BuildContext context) => const Column(
    children: [
      ScanAnimation(),
      SizedBox(height: 12),
      Text(
        'Das Dokument wird ausgewertet.\n'
        'Das kann bis zu einer Minute dauern.',
        textAlign: TextAlign.center,
      ),
    ],
  );
}

class _Notice extends StatelessWidget {
  const _Notice();

  @override
  Widget build(BuildContext context) {
    final colors = Theme.of(context).colorScheme;
    return Card(
      color: colors.secondaryContainer,
      child: Padding(
        padding: const EdgeInsets.all(12),
        child: Row(
          children: [
            Icon(Icons.info_outline, color: colors.onSecondaryContainer),
            const SizedBox(width: 12),
            Expanded(
              child: Text(
                cloudNotice,
                style: TextStyle(color: colors.onSecondaryContainer),
              ),
            ),
          ],
        ),
      ),
    );
  }
}

/// Above a text that comes from a spoken note: it is unchecked machine
/// output (docs/API.md 3.9), so ask to check it before saving. The card has
/// the color of the marks in the field ([QuantityHighlightController]).
class _TranscriptNotice extends StatelessWidget {
  const _TranscriptNotice();

  @override
  Widget build(BuildContext context) {
    final colors = Theme.of(context).colorScheme;
    final style = TextStyle(color: colors.onTertiaryContainer);
    return Card(
      color: colors.tertiaryContainer,
      child: Padding(
        padding: const EdgeInsets.all(12),
        child: Row(
          children: [
            Icon(Icons.mic_none, color: colors.onTertiaryContainer),
            const SizedBox(width: 12),
            Expanded(
              child: Column(
                crossAxisAlignment: CrossAxisAlignment.start,
                children: [
                  Text(
                    'Erkannter Text aus deiner Sprachnotiz. Bitte prüfen und '
                    'bei Bedarf korrigieren, bevor du speicherst.',
                    style: style,
                  ),
                  const SizedBox(height: 8),
                  Text(
                    'Zahlen und Einheiten sind markiert. Bitte besonders '
                    'genau prüfen.',
                    style: style.copyWith(fontWeight: FontWeight.w600),
                  ),
                ],
              ),
            ),
          ],
        ),
      ),
    );
  }
}

class _ErrorCard extends StatelessWidget {
  const _ErrorCard({required this.error, required this.openEntry});

  final ApiException error;
  final void Function(String entryId) openEntry;

  @override
  Widget build(BuildContext context) {
    final colors = Theme.of(context).colorScheme;
    final existingId = error.code == ErrorCode.duplicate
        ? error.details['entryId']
        : null;
    return Card(
      color: colors.errorContainer,
      child: Padding(
        padding: const EdgeInsets.all(12),
        child: Column(
          crossAxisAlignment: CrossAxisAlignment.start,
          children: [
            Row(
              children: [
                Icon(Icons.error_outline, color: colors.onErrorContainer),
                const SizedBox(width: 12),
                Expanded(
                  child: Text(
                    error.message,
                    style: TextStyle(color: colors.onErrorContainer),
                  ),
                ),
              ],
            ),
            if (existingId is String)
              Align(
                alignment: Alignment.centerRight,
                child: TextButton(
                  onPressed: () => openEntry(existingId),
                  child: const Text('Vorhandenen Eintrag öffnen'),
                ),
              ),
          ],
        ),
      ),
    );
  }
}

class _FileIcon extends StatelessWidget {
  const _FileIcon(this.icon);

  final IconData icon;

  @override
  Widget build(BuildContext context) => SizedBox(
    height: 120,
    child: Center(
      child: Icon(
        icon,
        size: 64,
        color: Theme.of(context).colorScheme.onSurfaceVariant,
      ),
    ),
  );
}
