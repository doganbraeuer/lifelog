// Creates small sample files for the mock, so that "open original file"
// shows something without a backend. Both formats are put together by hand,
// so that no binary files need to be in the repo.

import 'dart:convert';
import 'dart:io' show zlib;
import 'dart:typed_data';

/// A one-page PDF with the given lines of text (font Helvetica).
Uint8List createMockPdf(List<String> lines) => createMockPdfPages([lines]);

/// A PDF with one page per list of lines (font Helvetica 11 pt, 15 pt line
/// spacing, starting at the top left). The mock verification estimates its
/// rectangles from this layout.
Uint8List createMockPdfPages(List<List<String>> pages) {
  // Objects: 1 catalog, 2 page tree, 3 font, then per page the page and
  // its content stream.
  int pageObject(int index) => 4 + 2 * index;
  final objects = <List<int>>[
    _latin1('<< /Type /Catalog /Pages 2 0 R >>'),
    _latin1(
      '<< /Type /Pages /Kids '
      '[${[for (var i = 0; i < pages.length; i++) '${pageObject(i)} 0 R'].join(' ')}] '
      '/Count ${pages.length} >>',
    ),
    _latin1(
      '<< /Type /Font /Subtype /Type1 /BaseFont /Helvetica '
      '/Encoding /WinAnsiEncoding >>',
    ),
  ];
  for (var i = 0; i < pages.length; i++) {
    final content = StringBuffer('BT\n/F1 11 Tf\n15 TL\n56 790 Td\n');
    for (final line in pages[i]) {
      content.write('(${_pdfText(line)}) Tj T*\n');
    }
    content.write('ET');
    final stream = _latin1(content.toString());
    objects
      ..add(
        _latin1(
          '<< /Type /Page /Parent 2 0 R /MediaBox [0 0 595 842] '
          '/Resources << /Font << /F1 3 0 R >> >> '
          '/Contents ${pageObject(i) + 1} 0 R >>',
        ),
      )
      ..add([
        ..._latin1('<< /Length ${stream.length} >>\nstream\n'),
        ...stream,
        ..._latin1('\nendstream'),
      ]);
  }

  // A PDF ends with a table ("xref") that gives the byte position of every
  // object in the file. That is why the positions are recorded.
  final pdf = BytesBuilder()..add(_latin1('%PDF-1.4\n'));
  final positions = <int>[];
  for (var i = 0; i < objects.length; i++) {
    positions.add(pdf.length);
    pdf
      ..add(_latin1('${i + 1} 0 obj\n'))
      ..add(objects[i])
      ..add(_latin1('\nendobj\n'));
  }
  final xrefPosition = pdf.length;
  pdf.add(_latin1('xref\n0 ${objects.length + 1}\n0000000000 65535 f \n'));
  for (final position in positions) {
    pdf.add(_latin1('${position.toString().padLeft(10, '0')} 00000 n \n'));
  }
  pdf.add(
    _latin1(
      'trailer\n<< /Size ${objects.length + 1} /Root 1 0 R >>\n'
      'startxref\n$xrefPosition\n%%EOF\n',
    ),
  );
  return pdf.toBytes();
}

/// A grey "photo" of a document: a white page with bars as lines of text.
Uint8List createMockPng({int width = 240, int height = 320}) {
  // Raw data: per row one filter byte (0 = none), then one grey value per
  // pixel.
  final raw = BytesBuilder();
  for (var y = 0; y < height; y++) {
    raw.addByte(0);
    for (var x = 0; x < width; x++) {
      raw.addByte(_greyValue(x, y, width, height));
    }
  }

  final header = ByteData(13)
    ..setUint32(0, width)
    ..setUint32(4, height)
    ..setUint8(8, 8) // 8 bits per pixel
    ..setUint8(9, 0) // greyscale
    ..setUint8(10, 0)
    ..setUint8(11, 0)
    ..setUint8(12, 0);

  return (BytesBuilder()
        ..add(const [0x89, 0x50, 0x4E, 0x47, 0x0D, 0x0A, 0x1A, 0x0A])
        ..add(_pngChunk('IHDR', header.buffer.asUint8List()))
        ..add(_pngChunk('IDAT', zlib.encode(raw.toBytes())))
        ..add(_pngChunk('IEND', const [])))
      .toBytes();
}

int _greyValue(int x, int y, int width, int height) {
  const margin = 20;
  if (x < margin || x >= width - margin) return 255;
  // Heading
  if (y >= 24 && y < 34 && x < width * 0.6) return 40;
  // Lines of text: every 18 pixels a line 6 pixels high, of varying length.
  if (y >= 60 && y < height - 30) {
    final line = (y - 60) ~/ 18;
    if ((y - 60) % 18 < 6) {
      final length = width - 2 * margin - (line * 37) % 90;
      if (x < margin + length) return 120;
    }
  }
  return 255;
}

List<int> _pngChunk(String type, List<int> data) {
  final typeBytes = ascii.encode(type);
  final length = ByteData(4)..setUint32(0, data.length);
  final checksum = ByteData(4)..setUint32(0, _crc32([...typeBytes, ...data]));
  return [
    ...length.buffer.asUint8List(),
    ...typeBytes,
    ...data,
    ...checksum.buffer.asUint8List(),
  ];
}

final _crcTable = List<int>.generate(256, (n) {
  var c = n;
  for (var k = 0; k < 8; k++) {
    c = (c & 1) != 0 ? 0xEDB88320 ^ (c >>> 1) : c >>> 1;
  }
  return c;
});

int _crc32(List<int> data) {
  var c = 0xFFFFFFFF;
  for (final b in data) {
    c = _crcTable[(c ^ b) & 0xFF] ^ (c >>> 8);
  }
  return (c ^ 0xFFFFFFFF) & 0xFFFFFFFF;
}

/// Text for PDF: escape parentheses and backslash, replace characters
/// outside Latin-1 with `?`.
String _pdfText(String text) =>
    String.fromCharCodes(text.runes.map((r) => r > 255 ? 0x3F : r))
        .replaceAll(r'\', r'\\')
        .replaceAll('(', r'\(')
        .replaceAll(')', r'\)');

List<int> _latin1(String text) => latin1.encode(text);
