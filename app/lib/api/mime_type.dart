import 'dart:typed_data';

/// Detects the file type from the first bytes ("magic bytes"), not from the
/// file extension. According to docs/API.md the backend does the same.
///
/// Returns `image/jpeg`, `image/png`, `image/heic`, `image/webp`,
/// `application/pdf` or `null` (not supported).
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

  // WebP: a RIFF container with the form type "WEBP" at byte 8 (lossy,
  // lossless and extended alike). Allowed by the backend since 2026-11-11.
  if (startsWith('RIFF'.codeUnits) && startsWith('WEBP'.codeUnits, 8)) {
    return 'image/webp';
  }

  // HEIC: "ftyp" from byte 4, followed by the brand (e.g. "heic").
  if (startsWith('ftyp'.codeUnits, 4) && bytes.length >= 12) {
    final brand = String.fromCharCodes(bytes.sublist(8, 12));
    if (_heicBrands.contains(brand)) return 'image/heic';
  }
  return null;
}

/// HEIC brands after "ftyp"; a file with one of them is an image, not audio.
const _heicBrands = {'heic', 'heix', 'hevc', 'hevx', 'heim', 'heis', 'mif1'};

/// Detects the audio format of a recording from its first bytes, like
/// [detectMimeType] for uploads. According to docs/API.md (3.9) the backend
/// also decides by content.
///
/// Returns one of the MIME types allowed for `POST /transcriptions`
/// (`audio/mp4`, `audio/3gpp`, `audio/amr`, `audio/wav`, `audio/ogg`,
/// `audio/webm`, `audio/mpeg`, `audio/flac`) or `null`.
String? detectAudioType(Uint8List bytes) {
  bool startsWith(List<int> pattern, [int offset = 0]) {
    if (bytes.length < offset + pattern.length) return false;
    for (var i = 0; i < pattern.length; i++) {
      if (bytes[offset + i] != pattern[i]) return false;
    }
    return true;
  }

  // MP4 family: "ftyp" from byte 4, followed by the brand. M4A from the
  // package record has e.g. "M4A " or "mp42"; 3GP has "3gp4", "3gp5", ….
  if (startsWith('ftyp'.codeUnits, 4) && bytes.length >= 12) {
    final brand = String.fromCharCodes(bytes.sublist(8, 12));
    if (_heicBrands.contains(brand)) return null;
    return brand.startsWith('3g') ? 'audio/3gpp' : 'audio/mp4';
  }
  if (startsWith('#!AMR'.codeUnits)) return 'audio/amr';
  if (startsWith('RIFF'.codeUnits) && startsWith('WAVE'.codeUnits, 8)) {
    return 'audio/wav';
  }
  if (startsWith('OggS'.codeUnits)) return 'audio/ogg';
  if (startsWith([0x1A, 0x45, 0xDF, 0xA3])) return 'audio/webm';
  if (startsWith('fLaC'.codeUnits)) return 'audio/flac';
  if (startsWith('ID3'.codeUnits)) return 'audio/mpeg';
  // MPEG audio frame without an ID3 tag: 11 sync bits and a layer other
  // than 00 (00 would be AAC in ADTS, which is not allowed).
  if (bytes.length >= 2 &&
      bytes[0] == 0xFF &&
      (bytes[1] & 0xE0) == 0xE0 &&
      (bytes[1] >> 1) & 0x03 != 0) {
    return 'audio/mpeg';
  }
  return null;
}
