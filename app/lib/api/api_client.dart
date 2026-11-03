import 'dart:typed_data';

import '../model/antwort.dart';
import '../model/eintrag.dart';
import '../model/eintrag_aenderung.dart';

/// Sortierfeld der Zeitleiste. Die Namen entsprechen den Werten des
/// Query-Parameters `sortierung`.
enum Sortierung { dokumentdatum, importdatum }

/// Sortierrichtung. Die Namen entsprechen den Werten von `richtung`.
enum Richtung { absteigend, aufsteigend }

/// Maximale Upload-Größe laut docs/API.md: 10 MB.
const maxDateiGroesse = 10 * 1024 * 1024;

/// Alle Aufrufe ans Backend laufen über diese Schnittstelle.
///
/// Es gibt zwei Implementierungen: [MockApiClient] mit Beispieldaten und
/// [HttpApiClient] für das echte Backend. Welche verwendet wird, entscheidet
/// `erstelleApiClient()` anhand von `--dart-define=API_BASE_URL`.
///
/// Jeder Fehler wird als [ApiFehler] geworfen.
abstract interface class ApiClient {
  /// `true` für den Mock. Die App zeigt dann einen Hinweis an.
  bool get istMock;

  /// `GET /eintraege`: alle Einträge in der Kurzform, vom Backend sortiert.
  Future<List<EintragKurz>> eintraege({
    Sortierung sortierung = Sortierung.dokumentdatum,
    Richtung richtung = Richtung.absteigend,
  });

  /// `GET /eintraege/{id}`: ein vollständiger Eintrag.
  Future<Eintrag> eintrag(String id);

  /// `GET /eintraege/{id}/datei`: die Originaldatei.
  Future<DateiInhalt> datei(String id);

  /// `POST /eintraege`: Datei hochladen und auslesen lassen.
  Future<Eintrag> hochladen(Uint8List bytes, String dateiname);

  /// `PATCH /eintraege/{id}`: erkannte Werte korrigieren.
  Future<Eintrag> korrigieren(String id, EintragAenderung aenderung);

  /// `POST /fragen`: Frage stellen, Antwort mit Quellenangabe.
  Future<FrageAntwort> fragen(String frage);

  /// Gibt Ressourcen frei (z. B. die HTTP-Verbindung).
  void schliessen();
}
