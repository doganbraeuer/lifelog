/// Error codes from docs/API.md (section 1) plus local codes of the app.
abstract final class ErrorCode {
  static const invalidRequest = 'INVALID_REQUEST';
  static const invalidParameter = 'INVALID_PARAMETER';
  static const invalidId = 'INVALID_ID';
  static const fileMissing = 'FILE_MISSING';
  static const fieldNotCorrectable = 'FIELD_NOT_CORRECTABLE';
  static const questionMissing = 'QUESTION_MISSING';
  static const questionTooLong = 'QUESTION_TOO_LONG';
  static const entryNotFound = 'ENTRY_NOT_FOUND';
  static const factNotFound = 'FACT_NOT_FOUND';
  static const duplicate = 'DUPLICATE';
  static const fileTooLarge = 'FILE_TOO_LARGE';
  static const unsupportedFileType = 'UNSUPPORTED_FILE_TYPE';
  static const unreadable = 'UNREADABLE';
  static const internalError = 'INTERNAL_ERROR';
  static const extractionFailed = 'EXTRACTION_FAILED';
  static const modelUnavailable = 'MODEL_UNAVAILABLE';

  /// Transcription (3.9): the recording is longer than 5 minutes.
  static const audioTooLong = 'AUDIO_TOO_LONG';

  /// Transcription (3.9): no speech in the recording (silence, noise, or
  /// shorter than 0.5 s).
  static const noSpeech = 'NO_SPEECH';

  // Only in the app, never sent by the backend:
  /// No connection or timeout.
  static const networkError = 'NETWORK_ERROR';

  /// The server returned something that does not match the contract.
  static const unexpectedResponse = 'UNEXPECTED_RESPONSE';

  /// Recording a spoken note failed on the device: microphone access
  /// denied, or the recorder did not start.
  static const recordingFailed = 'RECORDING_FAILED';

  /// Choosing a photo or file failed on the device (camera, photo library
  /// or file picker), e.g. access denied.
  static const pickFailed = 'PICK_FAILED';
}

/// Every error of an [ApiClient] call. The app evaluates [code] and may show
/// [message] directly.
class ApiException implements Exception {
  const ApiException({
    this.httpStatus,
    required this.code,
    required this.message,
    this.details = const {},
  });

  /// HTTP status, `null` for local errors (e.g. no connection).
  final int? httpStatus;
  final String code;
  final String message;
  final Map<String, dynamic> details;

  @override
  String toString() => 'ApiException($httpStatus $code: $message)';
}
