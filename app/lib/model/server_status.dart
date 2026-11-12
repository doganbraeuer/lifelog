import 'json_helpers.dart';

/// Field `speech` of `GET /status` (docs/API.md 3.8). The names are the
/// values in the JSON.
enum SpeechStatus {
  /// The speech recognition answers.
  ok,

  /// Switched on, but the speech recognition does not answer right now.
  unavailable,

  /// Switched off on this server.
  off,
}

/// Response of `GET /status` (docs/API.md 3.8).
class ServerStatus {
  const ServerStatus({
    required this.status,
    required this.database,
    required this.speech,
  });

  /// `ok` or `error`.
  final String status;

  /// `ok` or `unreachable`.
  final String database;

  final SpeechStatus speech;

  /// `true` only if transcription works right now.
  bool get speechAvailable => speech == SpeechStatus.ok;

  /// A missing or unknown `speech` counts as [SpeechStatus.unavailable]:
  /// a backend without this field (before 2026-11-12) cannot transcribe.
  factory ServerStatus.fromJson(Map<String, dynamic> json) {
    final speech = json['speech'];
    return ServerStatus(
      status: readString(json, 'status'),
      database: readString(json, 'database'),
      speech: SpeechStatus.values.firstWhere(
        (s) => s.name == speech,
        orElse: () => SpeechStatus.unavailable,
      ),
    );
  }
}
