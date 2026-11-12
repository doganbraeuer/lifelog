import 'dart:typed_data';

/// Detects the file type from the first bytes ("magic bytes"), not from the
/// file extension. According to docs/API.md the backend does the same.
///
/// Returns `image/jpeg`, `image/png`, `image/heic`, `application/pdf` or
/// `null` (not supported).
String? detectMimeType(Uint8List bytes) {
  bool startsWith(List<int> pattern, [int offset = 0]) {
    if (bytes.length < offset + pattern.length) return false;
    for (var i = 0; i < pattern.length; i++) {
      if (bytes[offset + i] != pattern[i]) return false;
    }
    return true;
  }

  if (startsWith([0xFF, 0xD8, 0xFF])) return 'image/jpeg';
  if (startsWith([0x89, 0x50, 0x4E, 0x47, 0x0D, 0x0A, 0x1A, 0x0A])) {
    return 'image/png';
  }
  if (startsWith('%PDF-'.codeUnits)) return 'application/pdf';

  // HEIC: "ftyp" from byte 4, followed by the brand (e.g. "heic").
  if (startsWith('ftyp'.codeUnits, 4) && bytes.length >= 12) {
    final brand = String.fromCharCodes(bytes.sublist(8, 12));
    const heicBrands = {'heic', 'heix', 'hevc', 'hevx', 'heim', 'heis', 'mif1'};
    if (heicBrands.contains(brand)) return 'image/heic';
  }
  return null;
}
