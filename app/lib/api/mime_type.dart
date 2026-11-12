import 'dart:typed_data';

/// Erkennt den Dateityp an den ersten Bytes ("Magic Bytes"), nicht an der
/// Dateiendung. So macht es laut docs/API.md auch das Backend.
///
/// Gibt `image/jpeg`, `image/png`, `image/heic`, `application/pdf` oder
/// `null` (nicht unterstützt) zurück.
String? erkenneMimeTyp(Uint8List bytes) {
  bool beginntMit(List<int> muster, [int ab = 0]) {
    if (bytes.length < ab + muster.length) return false;
    for (var i = 0; i < muster.length; i++) {
      if (bytes[ab + i] != muster[i]) return false;
    }
    return true;
  }

  if (beginntMit([0xFF, 0xD8, 0xFF])) return 'image/jpeg';
  if (beginntMit([0x89, 0x50, 0x4E, 0x47, 0x0D, 0x0A, 0x1A, 0x0A])) {
    return 'image/png';
  }
  if (beginntMit('%PDF-'.codeUnits)) return 'application/pdf';

  // HEIC: ab Byte 4 steht "ftyp", danach die Marke (z. B. "heic").
  if (beginntMit('ftyp'.codeUnits, 4) && bytes.length >= 12) {
    final marke = String.fromCharCodes(bytes.sublist(8, 12));
    const heicMarken = {'heic', 'heix', 'hevc', 'hevx', 'heim', 'heis', 'mif1'};
    if (heicMarken.contains(marke)) return 'image/heic';
  }
  return null;
}
