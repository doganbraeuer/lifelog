import 'dart:async';

import 'package:flutter/material.dart';
import 'package:flutter/services.dart';

import '../api/api_client.dart';
import '../api/api_exception.dart';
import '../platform/voice_recorder.dart';
import '../widgets/motion.dart';

/// The recording stops on its own after this time: 2 s before the limit of
/// the backend (5 minutes, docs/API.md 3.9). The file can be a little longer
/// than the clock because the encoder needs a moment to start, and over
/// 300 s the backend rejects it with AUDIO_TOO_LONG.
const autoStopAfter = Duration(seconds: maxRecordingSeconds - 2);

/// For these errors a second attempt with the same recording is
/// worthwhile; the recording is kept until then.
const _retryable = {
  ErrorCode.networkError,
  ErrorCode.internalError,
  ErrorCode.modelUnavailable,
  ErrorCode.unexpectedResponse,
};

enum _Phase { starting, recording, transcribing, failed }

/// Records a spoken note and turns it into text (docs/API.md 3.9).
///
/// Flow: record (with running time) → „Stopp“ → the backend transcribes →
/// [onTranscribed] gets the text. Saving is not done here: the text screen
/// of the upload page inserts the text into its field ([insertTranscript])
/// for checking and saves it through the text import.
///
/// The recording is deleted as soon as it is no longer needed: after the
/// transcription, after an error without a chance of success, on
/// „Abbrechen“ and „Verwerfen“, and when this view goes away.
///
/// Errors go to [onError], so the upload page shows them in its error card.
class VoiceNoteView extends StatefulWidget {
  const VoiceNoteView({
    super.key,
    required this.api,
    required this.recorder,
    required this.onTranscribed,
    required this.onClosed,
    required this.onError,
  });

  final ApiClient api;
  final VoiceRecorder recorder;

  /// Called with the recognized text; the recording is already deleted.
  final void Function(String text) onTranscribed;

  /// Called when the view should close without a text.
  final VoidCallback onClosed;

  /// Shows an error on the upload page, or removes it with `null`.
  final void Function(ApiException? error) onError;

  @override
  State<VoiceNoteView> createState() => _VoiceNoteViewState();
}

class _VoiceNoteViewState extends State<VoiceNoteView>
    with WidgetsBindingObserver {
  var _phase = _Phase.starting;
  var _elapsed = Duration.zero;
  Timer? _ticker;
  StreamSubscription<void>? _systemStops;
  Recording? _recording;

  @override
  void initState() {
    super.initState();
    WidgetsBinding.instance.addObserver(this);
    _systemStops = widget.recorder.stoppedBySystem.listen((_) => _stop());
    _start();
  }

  @override
  void dispose() {
    WidgetsBinding.instance.removeObserver(this);
    _ticker?.cancel();
    _systemStops?.cancel();
    // Whatever happened: no recording stays on the phone.
    widget.recorder.discard();
    super.dispose();
  }

  /// In the background Android no longer hands over the microphone, so
  /// the recording would go on silently. Stop it and transcribe what is
  /// there instead.
  @override
  void didChangeAppLifecycleState(AppLifecycleState state) {
    if (state == AppLifecycleState.hidden ||
        state == AppLifecycleState.paused) {
      _stop();
    }
  }

  Future<void> _start() async {
    try {
      await widget.recorder.start();
    } on RecorderException {
      await widget.recorder.discard();
      _close(
        const ApiException(
          code: ErrorCode.recordingFailed,
          message:
              'Die Aufnahme konnte nicht gestartet werden. '
              'Bitte noch einmal versuchen.',
        ),
      );
      return;
    }
    if (!mounted) {
      // Cancelled while starting: dispose has already run.
      await widget.recorder.discard();
      return;
    }
    HapticFeedback.mediumImpact();
    _ticker = Timer.periodic(const Duration(seconds: 1), (_) => _tick());
    setState(() => _phase = _Phase.recording);
  }

  void _tick() {
    setState(() => _elapsed += const Duration(seconds: 1));
    if (_elapsed >= autoStopAfter) _stop();
  }

  Future<void> _stop() async {
    if (_phase != _Phase.recording) return;
    _ticker?.cancel();
    HapticFeedback.mediumImpact();
    setState(() => _phase = _Phase.transcribing);
    try {
      _recording = await widget.recorder.stop();
    } on RecorderException {
      await widget.recorder.discard();
      _close(
        const ApiException(
          code: ErrorCode.recordingFailed,
          message:
              'Die Aufnahme konnte nicht gespeichert werden. '
              'Bitte noch einmal aufnehmen.',
        ),
      );
      return;
    }
    await _transcribe();
  }

  Future<void> _transcribe() async {
    final recording = _recording;
    if (recording == null || !mounted) return;
    widget.onError(null);
    setState(() => _phase = _Phase.transcribing);
    try {
      final transcript = await widget.api.transcribe(
        recording.bytes,
        recording.fileName,
      );
      // The text is in memory now; the recording is not needed any more.
      await _deleteRecording();
      if (mounted) widget.onTranscribed(transcript.text);
    } on ApiException catch (error) {
      if (!mounted) return;
      if (_retryable.contains(error.code)) {
        // Keep the recording, so the user need not speak again.
        widget.onError(error);
        setState(() => _phase = _Phase.failed);
      } else {
        await _deleteRecording();
        _close(error);
      }
    }
  }

  Future<void> _deleteRecording() async {
    _recording = null;
    await widget.recorder.discard();
  }

  /// „Abbrechen“ while recording, „Verwerfen“ after an error.
  Future<void> _cancel() async {
    _ticker?.cancel();
    await _deleteRecording();
    _close(null);
  }

  void _close(ApiException? error) {
    if (!mounted) return;
    widget.onError(error);
    widget.onClosed();
  }

  @override
  Widget build(BuildContext context) {
    return FadeThroughSwitcher(
      child: switch (_phase) {
        _Phase.starting || _Phase.recording => KeyedSubtree(
          key: const ValueKey('recording'),
          child: _recordingView(context),
        ),
        _Phase.transcribing => const _Transcribing(
          key: ValueKey('transcribing'),
        ),
        _Phase.failed => KeyedSubtree(
          key: const ValueKey('failed'),
          child: _failedView(),
        ),
      },
    );
  }

  Widget _recordingView(BuildContext context) {
    final theme = Theme.of(context);
    final recording = _phase == _Phase.recording;
    return Column(
      crossAxisAlignment: CrossAxisAlignment.stretch,
      children: [
        const SizedBox(height: 8),
        Center(child: _PulsingMicrophone(active: recording)),
        const SizedBox(height: 16),
        Text(
          formatElapsed(_elapsed),
          textAlign: TextAlign.center,
          style: theme.textTheme.displaySmall?.copyWith(
            fontFeatures: const [FontFeature.tabularFigures()],
          ),
        ),
        const SizedBox(height: 4),
        Text(
          recording
              ? 'Aufnahme läuft · höchstens 5 Minuten'
              : 'Aufnahme startet …',
          textAlign: TextAlign.center,
          style: theme.textTheme.bodyMedium?.copyWith(
            color: theme.colorScheme.onSurfaceVariant,
          ),
        ),
        const SizedBox(height: 24),
        FilledButton.icon(
          onPressed: recording ? _stop : null,
          icon: const Icon(Icons.stop),
          label: const Text('Stopp'),
        ),
        const SizedBox(height: 8),
        TextButton(onPressed: _cancel, child: const Text('Abbrechen')),
      ],
    );
  }

  Widget _failedView() => Column(
    crossAxisAlignment: CrossAxisAlignment.stretch,
    children: [
      FilledButton.icon(
        onPressed: _transcribe,
        icon: const Icon(Icons.refresh),
        label: const Text('Erneut versuchen'),
      ),
      const SizedBox(height: 8),
      TextButton(onPressed: _cancel, child: const Text('Verwerfen')),
    ],
  );
}

final _whitespace = RegExp(r'\s');
final _letterOrDigit = RegExp(r'[\p{L}\p{N}]', unicode: true);

/// Puts the recognized [transcript] into the text of a field.
///
/// - With a cursor in the field: at the cursor; a selection is replaced, as
///   typing would do. A space separates it from the text before, so one
///   can dictate the rest of a sentence.
/// - Without a cursor (the field was never tapped): at the end, on a new
///   line, as a new paragraph.
/// - No separator at the start of the text or after whitespace. A space
///   after it only if a letter or digit follows right away, not before
///   punctuation such as `.`, `,` or `/`.
///
/// The cursor ends up behind the inserted text. An empty transcript changes
/// nothing.
TextEditingValue insertTranscript(TextEditingValue value, String transcript) {
  final insert = transcript.trim();
  if (insert.isEmpty) return value;
  final text = value.text;
  final selection = value.selection;
  final hasCursor = selection.isValid && selection.end <= text.length;
  final start = hasCursor ? selection.start : text.length;
  final end = hasCursor ? selection.end : text.length;
  final before = text.substring(0, start);
  final after = text.substring(end);
  final lead = before.isEmpty || _whitespace.hasMatch(before[before.length - 1])
      ? ''
      : hasCursor
      ? ' '
      : '\n';
  final trail =
      after.isNotEmpty &&
          _letterOrDigit.hasMatch(String.fromCharCode(after.runes.first))
      ? ' '
      : '';
  return TextEditingValue(
    text: '$before$lead$insert$trail$after',
    selection: TextSelection.collapsed(
      offset: start + lead.length + insert.length,
    ),
  );
}

/// Running time of a recording, e.g. `0:07` or `4:58`.
String formatElapsed(Duration elapsed) {
  final minutes = elapsed.inMinutes;
  final seconds = elapsed.inSeconds % 60;
  return '$minutes:${seconds.toString().padLeft(2, '0')}';
}

/// Shown while the backend transcribes (up to two minutes).
class _Transcribing extends StatelessWidget {
  const _Transcribing({super.key});

  @override
  Widget build(BuildContext context) => const Padding(
    padding: EdgeInsets.symmetric(vertical: 24),
    child: Column(
      children: [
        CircularProgressIndicator(),
        SizedBox(height: 16),
        Text(
          'Die Sprachnotiz wird in Text umgewandelt.\n'
          'Das kann bis zu zwei Minuten dauern.',
          textAlign: TextAlign.center,
        ),
      ],
    ),
  );
}

/// Microphone in a circle that pulses softly while recording. With
/// „Animationen entfernen“ it stays still.
class _PulsingMicrophone extends StatefulWidget {
  const _PulsingMicrophone({required this.active});

  final bool active;

  @override
  State<_PulsingMicrophone> createState() => _PulsingMicrophoneState();
}

class _PulsingMicrophoneState extends State<_PulsingMicrophone>
    with SingleTickerProviderStateMixin {
  late final _controller = AnimationController(
    vsync: this,
    duration: Durations.extralong2,
  );

  @override
  void didChangeDependencies() {
    super.didChangeDependencies();
    _update();
  }

  @override
  void didUpdateWidget(_PulsingMicrophone oldWidget) {
    super.didUpdateWidget(oldWidget);
    _update();
  }

  void _update() {
    if (widget.active && !motionDisabled(context)) {
      if (!_controller.isAnimating) _controller.repeat(reverse: true);
    } else {
      _controller.stop();
      _controller.value = 0;
    }
  }

  @override
  void dispose() {
    _controller.dispose();
    super.dispose();
  }

  @override
  Widget build(BuildContext context) {
    final colors = Theme.of(context).colorScheme;
    return ScaleTransition(
      scale: Tween(
        begin: 1.0,
        end: 1.12,
      ).animate(CurvedAnimation(parent: _controller, curve: Easing.standard)),
      child: Container(
        width: 96,
        height: 96,
        decoration: BoxDecoration(
          color: widget.active
              ? colors.errorContainer
              : colors.surfaceContainerHighest,
          shape: BoxShape.circle,
        ),
        child: Icon(
          Icons.mic,
          size: 48,
          color: widget.active
              ? colors.onErrorContainer
              : colors.onSurfaceVariant,
          semanticLabel: widget.active ? 'Aufnahme läuft' : null,
        ),
      ),
    );
  }
}
