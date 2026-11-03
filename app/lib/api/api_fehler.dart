/// Fehlercodes aus docs/API.md (Abschnitt 1) plus zwei lokale Codes der App.
abstract final class FehlerCode {
  static const ungueltigeAnfrage = 'UNGUELTIGE_ANFRAGE';
  static const ungueltigerParameter = 'UNGUELTIGER_PARAMETER';
  static const ungueltigeId = 'UNGUELTIGE_ID';
  static const dateiFehlt = 'DATEI_FEHLT';
  static const feldNichtKorrigierbar = 'FELD_NICHT_KORRIGIERBAR';
  static const frageFehlt = 'FRAGE_FEHLT';
  static const frageZuLang = 'FRAGE_ZU_LANG';
  static const eintragNichtGefunden = 'EINTRAG_NICHT_GEFUNDEN';
  static const infoNichtGefunden = 'INFO_NICHT_GEFUNDEN';
  static const duplikat = 'DUPLIKAT';
  static const dateiZuGross = 'DATEI_ZU_GROSS';
  static const dateitypNichtUnterstuetzt = 'DATEITYP_NICHT_UNTERSTUETZT';
  static const nichtLesbar = 'NICHT_LESBAR';
  static const internerFehler = 'INTERNER_FEHLER';
  static const auslesenFehlgeschlagen = 'AUSLESEN_FEHLGESCHLAGEN';
  static const modellNichtErreichbar = 'MODELL_NICHT_ERREICHBAR';

  // Nur in der App, kommt nie vom Backend:
  /// Keine Verbindung oder Zeitüberschreitung.
  static const netzwerkFehler = 'NETZWERK_FEHLER';

  /// Der Server hat etwas geliefert, das nicht zum Vertrag passt.
  static const unbekannteAntwort = 'UNBEKANNTE_ANTWORT';
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
