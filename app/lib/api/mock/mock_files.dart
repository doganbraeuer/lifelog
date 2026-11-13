// Creates small sample files for the mock, so that "open original file"
// shows something without a backend. Both formats are put together by hand,
// so that no binary files need to be in the repo.

import 'dart:convert';
import 'dart:io' show zlib;
import 'dart:typed_data';

/// Where the lines of a mock original sit on its page. The mock verification
/// estimates its rectangles from this, so they land on the values.
class MockLayout {
  const MockLayout({
    required this.pageWidth,
    required this.pageHeight,
    required this.left,
    required this.firstTop,
    required this.lineHeight,
    required this.charWidth,
    required this.textHeight,
    this.helveticaSize,
  });

  /// The mock PDF (createMockPdfPages): A4 in points, Helvetica 11 pt,
  /// 15 pt line spacing, first line at the top left (baseline at 52 pt from
  /// the top, capitals 7.9 pt above it, descenders 2.3 pt below).
  static const pdfA4 = MockLayout(
    pageWidth: 595,
    pageHeight: 842,
    left: 56,
    firstTop: 44,
    lineHeight: 15,
    charWidth: 5.5,
    textHeight: 12,
    helveticaSize: 11,
  );

  /// The mock photo (createMockPhoto), in pixels.
  static const photo = MockLayout(
    pageWidth: 600,
    pageHeight: 800,
    left: 40,
    firstTop: 48,
    lineHeight: 36,
    charWidth: 12,
    textHeight: 20,
  );

  final double pageWidth;
  final double pageHeight;

  /// Left edge of every line.
  final double left;

  /// Top edge of the text of the first line.
  final double firstTop;
  final double lineHeight;

  /// Width of every character, unless [helveticaSize] is set.
  final double charWidth;

  /// Height of the text of a line (from the top of capitals to below the
  /// descenders).
  final double textHeight;

  /// Font size of a page written in Helvetica (the mock PDF); characters then
  /// have their real Helvetica widths instead of [charWidth].
  final double? helveticaSize;

  /// Width of [runes] in a line, in the units of the page.
  double widthOf(Iterable<int> runes) {
    final size = helveticaSize;
    if (size == null) return runes.length * charWidth;
    var units = 0;
    for (final rune in runes) {
      // The PDF writes characters outside Latin-1 as „?“ (see _pdfText).
      units += _helveticaWidths[rune > 255 ? 0x3F : rune] ?? 556;
    }
    return units * size / 1000;
  }
}

/// Widths of Helvetica (standard font metrics, in 1/1000 em) for the
/// characters the mock data uses; other characters count as 556, the width
/// of a digit.
const _helveticaWidths = <int, int>{
  0x20: 278, 0x21: 278, 0x22: 355, 0x25: 889, 0x26: 667, 0x27: 191, //
  0x28: 333, 0x29: 333, 0x2A: 389, 0x2B: 584, 0x2C: 278, 0x2D: 333, //
  0x2E: 278, 0x2F: 278, 0x3A: 278, 0x3B: 278, 0x3C: 584, 0x3D: 584, //
  0x3E: 584, 0x40: 1015, //
  // A–Z
  0x41: 667, 0x42: 667, 0x43: 722, 0x44: 722, 0x45: 667, 0x46: 611, //
  0x47: 778, 0x48: 722, 0x49: 278, 0x4A: 500, 0x4B: 667, 0x4C: 556, //
  0x4D: 833, 0x4E: 722, 0x4F: 778, 0x50: 667, 0x51: 778, 0x52: 722, //
  0x53: 667, 0x54: 611, 0x55: 722, 0x56: 667, 0x57: 944, 0x58: 667, //
  0x59: 667, 0x5A: 611, //
  0x5B: 278, 0x5C: 278, 0x5D: 278, 0x5F: 556, //
  // a–z
  0x61: 556, 0x62: 556, 0x63: 500, 0x64: 556, 0x65: 556, 0x66: 278, //
  0x67: 556, 0x68: 556, 0x69: 222, 0x6A: 222, 0x6B: 500, 0x6C: 222, //
  0x6D: 833, 0x6E: 556, 0x6F: 556, 0x70: 556, 0x71: 556, 0x72: 333, //
  0x73: 500, 0x74: 278, 0x75: 556, 0x76: 500, 0x77: 722, 0x78: 500, //
  0x79: 500, 0x7A: 500, //
  // Ä Ö Ü ä ö ü ß
  0xC4: 667, 0xD6: 778, 0xDC: 722, 0xE4: 556, 0xF6: 556, 0xFC: 556, //
  0xDF: 611,
};

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
Uint8List createMockPng({int width = 240, int height = 320}) =>
    _greyPng(width, height, (x, y) => _greyValue(x, y, width, height));

/// A grey "photo" of a sheet of paper on a table, with one dark block per
/// character of [lines] (gaps for spaces), laid out as in
/// [MockLayout.photo]. The mock verification uses the same layout, so its
/// rectangles lie exactly on the "words".
Uint8List createMockPhoto(List<String> lines) {
  const layout = MockLayout.photo;
  final width = layout.pageWidth.toInt();
  final height = layout.pageHeight.toInt();
  final runes = [for (final line in lines) line.runes.toList()];
  const sheet = 16; // table visible around the sheet
  return _greyPng(width, height, (x, y) {
    if (x < sheet || x >= width - sheet || y < sheet || y >= height - sheet) {
      return 110;
    }
    final top = y - layout.firstTop;
    if (top < 0) return 255;
    final line = top ~/ layout.lineHeight;
    if (line >= runes.length || top % layout.lineHeight >= layout.textHeight) {
      return 255;
    }
    final left = x - layout.left;
    if (left < 0) return 255;
    final column = left ~/ layout.charWidth;
    final chars = runes[line];
    if (column >= chars.length) return 255;
    // A thin gap between characters, none at all for a space.
    if (String.fromCharCode(chars[column]).trim().isEmpty) return 255;
    return left % layout.charWidth < layout.charWidth - 2 ? 70 : 255;
  });
}

/// A greyscale PNG; [grey] gives the value (0–255) of every pixel.
Uint8List _greyPng(int width, int height, int Function(int x, int y) grey) {
  // Raw data: per row one filter byte (0 = none), then one grey value per
  // pixel.
  final raw = BytesBuilder();
  for (var y = 0; y < height; y++) {
    raw.addByte(0);
    for (var x = 0; x < width; x++) {
      raw.addByte(grey(x, y));
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
