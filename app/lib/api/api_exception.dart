/// Fehlercodes aus docs/API.md (Abschnitt 1) plus zwei lokale Codes der App.
abstract final class FehlerCode {
  static const ungueltigeAnfrage = 'INVALID_REQUEST';
  static const ungueltigerParameter = 'INVALID_PARAMETER';
  static const ungueltigeId = 'INVALID_ID';
  static const dateiFehlt = 'FILE_MISSING';
  static const feldNichtKorrigierbar = 'FIELD_NOT_CORRECTABLE';
  static const frageFehlt = 'QUESTION_MISSING';
  static const frageZuLang = 'QUESTION_TOO_LONG';
  static const eintragNichtGefunden = 'ENTRY_NOT_FOUND';
  static const infoNichtGefunden = 'FACT_NOT_FOUND';
  static const duplikat = 'DUPLICATE';
  static const dateiZuGross = 'FILE_TOO_LARGE';
  static const dateitypNichtUnterstuetzt = 'UNSUPPORTED_FILE_TYPE';
  static const nichtLesbar = 'UNREADABLE';
  static const internerFehler = 'INTERNAL_ERROR';
  static const auslesenFehlgeschlagen = 'EXTRACTION_FAILED';
  static const modellNichtErreichbar = 'MODEL_UNAVAILABLE';

  // Nur in der App, kommt nie vom Backend:
  /// Keine Verbindung oder Zeitüberschreitung.
  static const netzwerkFehler = 'NETWORK_ERROR';

  /// Der Server hat etwas geliefert, das nicht zum Vertrag passt.
  static const unbekannteAntwort = 'UNEXPECTED_RESPONSE';
}

/// Jeder Fehler eines [ApiClient]-Aufrufs. Die App wertet [code] aus und
/// darf [nachricht] direkt anzeigen.
class ApiFehler implements Exception {
  const ApiFehler({
    this.httpStatus,
    required this.code,
    required this.nachricht,
    this.details = const {},
  });

  /// HTTP-Status, `null` bei lokalen Fehlern (z. B. keine Verbindung).
  final int? httpStatus;
  final String code;
  final String nachricht;
  final Map<String, dynamic> details;

  @override
  String toString() => 'ApiFehler($httpStatus $code: $nachricht)';
}
