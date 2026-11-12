import 'json_helpers.dart';

/// Result of `POST /transcriptions` (docs/API.md 3.9): the recognized text
/// of a spoken note. Nothing is stored in the backend; the app shows the
/// text for checking and saves it through the normal text import.
class Transcript {
  const Transcript({
    required this.text,
    required this.durationSeconds,
    required this.language,
    required this.engine,
  });

  /// Recognized text. Unchecked machine output, never empty.
  final String text;

  /// Length of the recording in seconds (one decimal).
  final double durationSeconds;

  /// Always `de`.
  final String language;

  /// Program and model, for display and support.
  final String engine;

  factory Transcript.fromJson(Map<String, dynamic> json) {
    final text = readString(json, 'text');
    // No speech is 422 NO_SPEECH, so an empty text breaks the contract.
    if (text.trim().isEmpty) {
      throw const FormatException('Field "text" is empty');
    }
    return Transcript(
      text: text,
      durationSeconds: readDouble(json, 'durationSeconds'),
      language: readString(json, 'language'),
      engine: readString(json, 'engine'),
    );
  }
}
