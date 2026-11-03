import 'package:file_picker/file_picker.dart';
import 'package:flutter/foundation.dart';
import 'package:image_picker/image_picker.dart';

import '../api/api_client.dart';

/// Eine vom Nutzer gewählte Datei.
class AusgewaehlteDatei {
  const AusgewaehlteDatei({required this.bytes, required this.name});

  final Uint8List bytes;
  final String name;
}

/// Die gewählte Datei ist größer als erlaubt. Wird geworfen, bevor die
/// Datei eingelesen wird, wenn die Größe vorher bekannt ist.
class DateiZuGross implements Exception {
  const DateiZuGross(this.groesse);

  final int groesse;
}

/// Kamera, Galerie und Dateiauswahl hinter einer Schnittstelle.
///
/// Die Plugins öffnen Fenster des Betriebssystems, die sich in Tests nicht
/// bedienen lassen. Tests geben deshalb eine eigene Implementierung an.
///
/// Alle Methoden geben `null` zurück, wenn der Nutzer abbricht.
abstract interface class DateiAuswahl {
  Future<AusgewaehlteDatei?> fotoAufnehmen();

  Future<AusgewaehlteDatei?> bildAuswaehlen();

  Future<AusgewaehlteDatei?> pdfAuswaehlen();

  /// Android kann die App beenden, während die Kamera-App offen ist (wenig
  /// Speicher). Das Foto lässt sich nach dem Neustart hier abholen.
  Future<AusgewaehlteDatei?> verloreneDatei();
}

/// Echte Auswahl über `image_picker` (Kamera, Galerie) und `file_picker`
/// (PDF).
class PluginDateiAuswahl implements DateiAuswahl {
  PluginDateiAuswahl();

  /// Längste Seite eines Fotos in Pixeln. 3000 px halten auch kleine
  /// Schrift lesbar und bleiben als JPEG deutlich unter 10 MB.
  static const maxSeite = 3000.0;

  /// JPEG-Qualität (0–100).
  static const jpegQualitaet = 85;

  final _bildAuswahl = ImagePicker();

  @override
  Future<AusgewaehlteDatei?> fotoAufnehmen() => _bild(ImageSource.camera);

  @override
  Future<AusgewaehlteDatei?> bildAuswaehlen() => _bild(ImageSource.gallery);

  Future<AusgewaehlteDatei?> _bild(ImageSource quelle) async {
    final datei = await _bildAuswahl.pickImage(
      source: quelle,
      maxWidth: maxSeite,
      maxHeight: maxSeite,
      imageQuality: jpegQualitaet,
      // Ohne volle Metadaten braucht iOS keine Berechtigung für die ganze
      // Fotomediathek; die Auswahl läuft über den System-Dialog.
      requestFullMetadata: false,
    );
    if (datei == null) return null;
    return AusgewaehlteDatei(
      bytes: await datei.readAsBytes(),
      name: datei.name,
    );
  }

  @override
  Future<AusgewaehlteDatei?> pdfAuswaehlen() async {
    final datei = await FilePicker.pickFile(
      type: FileType.custom,
      allowedExtensions: const ['pdf'],
    );
    if (datei == null) return null;
    final groesse = datei.lengthSync();
    if (groesse != null && groesse > maxDateiGroesse) {
      throw DateiZuGross(groesse);
    }
    return AusgewaehlteDatei(
      bytes: await datei.readAsBytes(),
      name: datei.name,
    );
  }

  @override
  Future<AusgewaehlteDatei?> verloreneDatei() async {
    if (kIsWeb || defaultTargetPlatform != TargetPlatform.android) return null;
    final antwort = await _bildAuswahl.retrieveLostData();
    final datei = antwort.file;
    if (antwort.isEmpty || datei == null) return null;
    return AusgewaehlteDatei(
      bytes: await datei.readAsBytes(),
      name: datei.name,
    );
  }
}
