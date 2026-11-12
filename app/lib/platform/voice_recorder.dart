import 'dart:async';
import 'dart:io';

import 'package:flutter/foundation.dart';
import 'package:path_provider/path_provider.dart';
import 'package:record/record.dart';

/// A finished recording of a spoken note.
class Recording {
  const Recording({required this.bytes, required this.fileName});

  final Uint8List bytes;
  final String fileName;
}

/// The recorder could not start, or the recording could not be read.
/// [reason] is for logs and tests; the app shows its own German text.
class RecorderException implements Exception {
  const RecorderException(this.reason);

  final String reason;

  @override
  String toString() => 'RecorderException($reason)';
}

/// Microphone recording behind one interface.
///
/// The plugin needs a real device and microphone. Tests therefore pass
/// their own implementation, like [FileChooser].
///
/// A recording lives as a file in the app's temporary folder until
/// [discard]. Recordings must not stay on the phone (docs/API.md 3.9), so
/// the app calls [discard] as soon as it no longer needs the file.
abstract interface class VoiceRecorder {
  /// Asks for microphone access if it was not decided yet. `false` if the
  /// user denies it, now or earlier.
  Future<bool> requestPermission();

  /// Starts a new recording into a new file. Leftovers of earlier
  /// recordings are deleted first. Throws [RecorderException].
  Future<void> start();

  /// Stops and returns the recording. The file stays until [discard], so it
  /// can be sent again after a network error. Throws [RecorderException].
  Future<Recording> stop();

  /// Stops if needed and deletes every recording file.
  Future<void> discard();

  /// Fires when the system pauses or ends the recording on its own (phone
  /// call, error). The app then handles it like „Stopp“.
  Stream<void> get stoppedBySystem;
}

/// The folder with the recordings. Pure Dart, so deleting can be tested
/// with a real folder.
class RecordingFiles {
  RecordingFiles(this.directory, {DateTime Function()? clock})
    : _clock = clock ?? DateTime.now;

  /// Folder that only holds recordings; it is deleted as a whole.
  final Directory directory;
  final DateTime Function() _clock;

  /// Deletes leftovers and returns the path for a new recording.
  Future<String> newPath() async {
    await deleteAll();
    await directory.create(recursive: true);
    final name = 'note_${_clock().millisecondsSinceEpoch}.m4a';
    return '${directory.path}${Platform.pathSeparator}$name';
  }

  /// Deletes every recording, also leftovers after a crash.
  Future<void> deleteAll() async {
    if (await directory.exists()) await directory.delete(recursive: true);
  }

  /// The recording files that exist right now.
  Future<List<File>> list() async {
    if (!await directory.exists()) return const [];
    return directory.list().where((e) => e is File).cast<File>().toList();
  }
}

/// Real recording with the package `record`, as recommended in
/// docs/API.md (3.9): AAC in M4A, mono, 16 kHz, about 32 kbit/s. A minute is
/// then about 240 KB.
class PluginVoiceRecorder implements VoiceRecorder {
  PluginVoiceRecorder();

  static const config = RecordConfig(
    encoder: AudioEncoder.aacLc,
    sampleRate: 16000,
    numChannels: 1,
    bitRate: 32000,
    // A headset that goes away: carry on with the phone's microphone
    // instead of stopping.
    audioRouteChange: AudioRouteChangeMode.follow,
  );

  final _recorder = AudioRecorder();
  final _stoppedBySystem = StreamController<void>.broadcast();
  StreamSubscription<RecordState>? _states;
  StreamSubscription<RecordStop>? _stops;
  RecordingFiles? _files;
  String? _path;

  /// `true` between [start] and [stop]/[discard]. A pause or end in this
  /// time comes from the system, because the app itself never pauses.
  var _recording = false;

  @override
  Stream<void> get stoppedBySystem => _stoppedBySystem.stream;

  Future<RecordingFiles> _folder() async {
    final temp = await getTemporaryDirectory();
    return _files ??= RecordingFiles(
      Directory('${temp.path}${Platform.pathSeparator}voice_notes'),
    );
  }

  @override
  Future<bool> requestPermission() async {
    try {
      return await _recorder.hasPermission();
    } on Exception {
      return false;
    }
  }

  @override
  Future<void> start() async {
    _states ??= _recorder.onStateChanged().listen((state) {
      // A phone call pauses the recording (AudioInterruptionMode.pause).
      if (_recording && state == RecordState.pause) _systemStop();
    });
    _stops ??= _recorder.onStopped().listen((stop) {
      if (_recording && stop.reason != RecordStopReason.stop) _systemStop();
    });
    try {
      final path = await (await _folder()).newPath();
      _path = path;
      await _recorder.start(config, path: path);
      _recording = true;
    } on Exception catch (e) {
      _path = null;
      throw RecorderException('$e');
    }
  }

  void _systemStop() {
    if (!_stoppedBySystem.isClosed) _stoppedBySystem.add(null);
  }

  @override
  Future<Recording> stop() async {
    final path = _path;
    _recording = false;
    try {
      await _recorder.stop();
    } on Exception {
      // The take may already have ended (system stop); the file is there.
    }
    if (path == null) throw const RecorderException('no recording');
    try {
      final bytes = await File(path).readAsBytes();
      if (bytes.isEmpty) throw const RecorderException('empty recording');
      return Recording(
        bytes: bytes,
        fileName: path.split(Platform.pathSeparator).last,
      );
    } on FileSystemException catch (e) {
      throw RecorderException(e.message);
    }
  }

  @override
  Future<void> discard() async {
    _recording = false;
    _path = null;
    try {
      await _recorder.cancel(); // stops and deletes a running take
    } on Exception {
      // Nothing was recording.
    }
    await (await _folder()).deleteAll();
  }
}
