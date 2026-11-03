// Erzeugt kleine Beispieldateien für den Mock, damit "Originaldatei
// öffnen" auch ohne Backend etwas zeigt. Beide Formate werden von Hand
// zusammengesetzt, damit keine Binärdateien im Repo liegen müssen.

import 'dart:convert';
import 'dart:io' show zlib;
import 'dart:typed_data';

/// Ein einseitiges PDF mit den angegebenen Textzeilen (Schrift Helvetica).
Uint8List erzeugeMockPdf(List<String> zeilen) {
  final inhalt = StringBuffer('BT\n/F1 11 Tf\n15 TL\n56 790 Td\n');
  for (final zeile in zeilen) {
    inhalt.write('(${_pdfText(zeile)}) Tj T*\n');
  }
  inhalt.write('ET');
  final stream = _latin1(inhalt.toString());

  final objekte = <List<int>>[
    _latin1('<< /Type /Catalog /Pages 2 0 R >>'),
    _latin1('<< /Type /Pages /Kids [3 0 R] /Count 1 >>'),
    _latin1(
      '<< /Type /Page /Parent 2 0 R /MediaBox [0 0 595 842] '
      '/Resources << /Font << /F1 4 0 R >> >> /Contents 5 0 R >>',
    ),
    _latin1(
      '<< /Type /Font /Subtype /Type1 /BaseFont /Helvetica '
      '/Encoding /WinAnsiEncoding >>',
    ),
    [
      ..._latin1('<< /Length ${stream.length} >>\nstream\n'),
      ...stream,
      ..._latin1('\nendstream'),
    ],
  ];

  // Ein PDF endet mit einer Tabelle ("xref"), die für jedes Objekt die
  // Byte-Position in der Datei angibt. Deshalb merken wir uns die Positionen.
  final pdf = BytesBuilder()..add(_latin1('%PDF-1.4\n'));
  final positionen = <int>[];
  for (var i = 0; i < objekte.length; i++) {
    positionen.add(pdf.length);
    pdf
      ..add(_latin1('${i + 1} 0 obj\n'))
      ..add(objekte[i])
      ..add(_latin1('\nendobj\n'));
  }
  final xrefPosition = pdf.length;
  pdf.add(_latin1('xref\n0 ${objekte.length + 1}\n0000000000 65535 f \n'));
  for (final position in positionen) {
    pdf.add(_latin1('${position.toString().padLeft(10, '0')} 00000 n \n'));
  }
  pdf.add(
    _latin1(
      'trailer\n<< /Size ${objekte.length + 1} /Root 1 0 R >>\n'
      'startxref\n$xrefPosition\n%%EOF\n',
    ),
  );
  return pdf.toBytes();
}

/// Ein graues "Foto" eines Dokuments: weiße Seite mit Balken als Textzeilen.
Uint8List erzeugeMockPng({int breite = 240, int hoehe = 320}) {
  // Rohdaten: pro Zeile ein Filter-Byte (0 = keiner), dann ein Grauwert
  // pro Pixel.
  final roh = BytesBuilder();
  for (var y = 0; y < hoehe; y++) {
    roh.addByte(0);
    for (var x = 0; x < breite; x++) {
      roh.addByte(_grauwert(x, y, breite, hoehe));
    }
  }

  final kopf = ByteData(13)
    ..setUint32(0, breite)
    ..setUint32(4, hoehe)
    ..setUint8(8, 8) // 8 Bit pro Pixel
    ..setUint8(9, 0) // Graustufen
    ..setUint8(10, 0)
    ..setUint8(11, 0)
    ..setUint8(12, 0);

  return (BytesBuilder()
        ..add(const [0x89, 0x50, 0x4E, 0x47, 0x0D, 0x0A, 0x1A, 0x0A])
        ..add(_pngBlock('IHDR', kopf.buffer.asUint8List()))
        ..add(_pngBlock('IDAT', zlib.encode(roh.toBytes())))
        ..add(_pngBlock('IEND', const [])))
      .toBytes();
}

int _grauwert(int x, int y, int breite, int hoehe) {
  const rand = 20;
  if (x < rand || x >= breite - rand) return 255;
  // Überschrift
  if (y >= 24 && y < 34 && x < breite * 0.6) return 40;
  // Textzeilen: alle 18 Pixel eine 6 Pixel hohe Zeile, unterschiedlich lang.
  if (y >= 60 && y < hoehe - 30) {
    final zeile = (y - 60) ~/ 18;
    if ((y - 60) % 18 < 6) {
      final laenge = breite - 2 * rand - (zeile * 37) % 90;
      if (x < rand + laenge) return 120;
    }
  }
  return 255;
}

List<int> _pngBlock(String typ, List<int> daten) {
  final typBytes = ascii.encode(typ);
  final laenge = ByteData(4)..setUint32(0, daten.length);
  final pruefsumme = ByteData(4)..setUint32(0, _crc32([...typBytes, ...daten]));
  return [
    ...laenge.buffer.asUint8List(),
    ...typBytes,
    ...daten,
    ...pruefsumme.buffer.asUint8List(),
  ];
}

final _crcTabelle = List<int>.generate(256, (n) {
  var c = n;
  for (var k = 0; k < 8; k++) {
    c = (c & 1) != 0 ? 0xEDB88320 ^ (c >>> 1) : c >>> 1;
  }
  return c;
});

int _crc32(List<int> daten) {
  var c = 0xFFFFFFFF;
  for (final b in daten) {
    c = _crcTabelle[(c ^ b) & 0xFF] ^ (c >>> 8);
  }
  return (c ^ 0xFFFFFFFF) & 0xFFFFFFFF;
}

/// Text für PDF: Klammern und Backslash maskieren, Zeichen außerhalb von
/// Latin-1 durch `?` ersetzen.
String _pdfText(String text) =>
    String.fromCharCodes(text.runes.map((r) => r > 255 ? 0x3F : r))
        .replaceAll(r'\', r'\\')
        .replaceAll('(', r'\(')
        .replaceAll(')', r'\)');

List<int> _latin1(String text) => latin1.encode(text);
