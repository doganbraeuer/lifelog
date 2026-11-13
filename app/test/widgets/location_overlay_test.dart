import 'dart:convert';
import 'dart:ui' as ui;

import 'package:flutter/material.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:lifelog/model/verification.dart';
import 'package:lifelog/widgets/location_overlay.dart';

/// A real JPEG of 8×4 pixels with EXIF orientation 6 ("rotate by 90°"), as
/// a phone stores a portrait photo. Shown upright it is 4×8.
final rotatedJpeg = base64Decode(
  '/9j/4AAQSkZJRgABAQAAAQABAAD/4QAiRXhpZgAATU0AKgAAAAgAAQESAAMAAAABAAYAAAAAAAD/'
  '2wBDAAMCAgMCAgMDAwMEAwMEBQgFBQQEBQoHBwYIDAoMDAsKCwsNDhIQDQ4RDgsLEBYQERMUFRUV'
  'DA8XGBYUGBIUFRT/2wBDAQMEBAUEBQkFBQkUDQsNFBQUFBQUFBQUFBQUFBQUFBQUFBQUFBQUFBQU'
  'FBQUFBQUFBQUFBQUFBQUFBQUFBQUFBT/wAARCAAEAAgDASIAAhEBAxEB/8QAHwAAAQUBAQEBAQEA'
  'AAAAAAAAAAECAwQFBgcICQoL/8QAtRAAAgEDAwIEAwUFBAQAAAF9AQIDAAQRBRIhMUEGE1FhByJx'
  'FDKBkaEII0KxwRVS0fAkM2JyggkKFhcYGRolJicoKSo0NTY3ODk6Q0RFRkdISUpTVFVWV1hZWmNk'
  'ZWZnaGlqc3R1dnd4eXqDhIWGh4iJipKTlJWWl5iZmqKjpKWmp6ipqrKztLW2t7i5usLDxMXGx8jJ'
  'ytLT1NXW19jZ2uHi4+Tl5ufo6erx8vP09fb3+Pn6/8QAHwEAAwEBAQEBAQEBAQAAAAAAAAECAwQF'
  'BgcICQoL/8QAtREAAgECBAQDBAcFBAQAAQJ3AAECAxEEBSExBhJBUQdhcRMiMoEIFEKRobHBCSMz'
  'UvAVYnLRChYkNOEl8RcYGRomJygpKjU2Nzg5OkNERUZHSElKU1RVVldYWVpjZGVmZ2hpanN0dXZ3'
  'eHl6goOEhYaHiImKkpOUlZaXmJmaoqOkpaanqKmqsrO0tba3uLm6wsPExcbHyMnK0tPU1dbX2Nna'
  '4uPk5ebn6Onq8vP09fb3+Pn6/9oADAMBAAIRAxEAPwD5X8C/8v3/AAD/ANmooopeJP8AyVWL/wC3'
  'P/TcD+oeAP8AkmsL/wBv/wDpyZ//2Q==',
);

const _location = PageLocation(
  page: 1,
  x: 0.25,
  y: 0.5,
  width: 0.5,
  height: 0.1,
);

void main() {
  group('fittedPageRect', () {
    test(
      'portrait page in a portrait box: full width, centered vertically',
      () {
        // 300×400 into 360×640: scale 1.2 → 360×480, 80 above and below.
        expect(
          fittedPageRect(const Size(300, 400), const Size(360, 640)),
          const Rect.fromLTWH(0, 80, 360, 480),
        );
      },
    );

    test('landscape photo in a portrait box: letterbox above and below', () {
      // 400×300 into 360×640: scale 0.9 → 360×270, 185 above and below.
      expect(
        fittedPageRect(const Size(400, 300), const Size(360, 640)),
        const Rect.fromLTWH(0, 185, 360, 270),
      );
    });

    test('portrait page in a landscape box: full height, centered', () {
      // A4 595×842 into 800×400: scale 400/842 → about 282.7×400.
      final rect = fittedPageRect(const Size(595, 842), const Size(800, 400));
      expect(rect.height, 400);
      expect(rect.width, closeTo(282.66, 0.01));
      expect(rect.center.dx, closeTo(400, 0.001));
    });

    test('an empty page or box does not divide by zero', () {
      expect(
        fittedPageRect(Size.zero, const Size(100, 100)),
        const Rect.fromLTWH(0, 0, 100, 100),
      );
    });
  });

  group('locationRect', () {
    test('fractions times the size at which the page is shown', () {
      expect(
        locationRect(_location, const Rect.fromLTWH(0, 80, 360, 480)),
        const Rect.fromLTWH(90, 320, 180, 48),
      );
    });

    test('a rotated photo: the fractions refer to the upright view', () {
      // The file stores 400×300 plus "rotate by 90°"; the backend's
      // fractions refer to the upright 300×400 view. With the upright size
      // the rectangle lands at the right place …
      const viewport = Size(360, 640);
      final upright = locationRect(
        _location,
        fittedPageRect(const Size(300, 400), viewport),
      );
      expect(upright, const Rect.fromLTWH(90, 320, 180, 48));
      // … with the raw size of the file it would be somewhere else.
      final raw = locationRect(
        _location,
        fittedPageRect(const Size(400, 300), viewport),
      );
      expect(raw, isNot(upright));
      expect(raw, const Rect.fromLTWH(90, 320, 180, 27));
    });

    test('corners of the page map to corners of the shown page', () {
      const page = Rect.fromLTWH(10, 20, 100, 200);
      const whole = PageLocation(page: 1, x: 0, y: 0, width: 1, height: 1);
      expect(locationRect(whole, page), page);
    });
  });

  group('focusTransform', () {
    const viewport = Size(360, 640);

    double scaleOf(Matrix4 m) => m.getMaxScaleOnAxis();
    Offset shiftOf(Matrix4 m) {
      final t = m.getTranslation();
      return Offset(t.x, t.y);
    }

    test('a small word is zoomed in to the maximum and centered', () {
      const word = Rect.fromLTWH(150, 300, 30, 10);
      final m = focusTransform(viewport, word);
      expect(scaleOf(m), 3);
      // The center of the word ends up in the center of the viewport.
      final center = MatrixUtils.transformPoint(m, word.center);
      expect(center.dx, closeTo(180, 0.001));
      expect(center.dy, closeTo(320, 0.001));
    });

    test('a wide rectangle is not zoomed in further than fits', () {
      const line = Rect.fromLTWH(30, 300, 240, 12);
      // 80 % of 360 / 240 = 1.2
      expect(scaleOf(focusTransform(viewport, line)), closeTo(1.2, 0.001));
    });

    test('never smaller than 1', () {
      const huge = Rect.fromLTWH(0, 0, 360, 640);
      final m = focusTransform(viewport, huge);
      expect(scaleOf(m), 1);
      expect(shiftOf(m), Offset.zero);
    });

    test('a word at the top left corner: no area beyond the page shows', () {
      const corner = Rect.fromLTWH(0, 0, 20, 10);
      final m = focusTransform(viewport, corner);
      expect(scaleOf(m), 3);
      expect(shiftOf(m), Offset.zero);
    });

    test('a word near the top of a letterboxed page: no grey above it', () {
      // A4 portrait in 360×640: the page fills the width, 80 px of
      // background above and below. The word sits close to the top edge.
      const page = Rect.fromLTWH(0, 80, 360, 480);
      const word = Rect.fromLTWH(150, 90, 30, 10);
      final m = focusTransform(viewport, word, page: page);
      expect(scaleOf(m), 3);
      // The top edge of the page lands exactly at the top of the view
      // instead of 35 px further down.
      expect(MatrixUtils.transformPoint(m, page.topLeft).dy, 0);
      // And the bottom edge is still below the view.
      expect(
        MatrixUtils.transformPoint(m, page.bottomLeft).dy,
        greaterThan(640),
      );
    });

    test('a page narrower than the view stays centered', () {
      // A4 in a wide view at scale 1: 283 px of 800, centered.
      const page = Rect.fromLTWH(258.5, 0, 283, 400);
      final m = focusTransform(
        const Size(800, 400),
        const Rect.fromLTWH(258.5, 0, 283, 400),
        page: page,
      );
      expect(scaleOf(m), 1);
      expect(shiftOf(m).dx, closeTo(0, 0.001));
    });

    test('a word at the bottom right corner: clamped to the far edge', () {
      const corner = Rect.fromLTWH(340, 630, 20, 10);
      final m = focusTransform(viewport, corner);
      // At scale 3 the child is 1080×1920; the shift may go at most to
      // -720 / -1280.
      expect(shiftOf(m), const Offset(-720, -1280));
    });
  });

  testWidgets('a JPEG with EXIF orientation 6 decodes upright (4×8)', (
    tester,
  ) async {
    final size = await tester.runAsync(() async {
      final codec = await ui.instantiateImageCodec(rotatedJpeg);
      final frame = await codec.getNextFrame();
      final size = Size(
        frame.image.width.toDouble(),
        frame.image.height.toDouble(),
      );
      frame.image.dispose();
      codec.dispose();
      return size;
    });
    // Raw the file is 8×4; Flutter applies the orientation while decoding,
    // so the app gets the size of the upright view the locations refer to.
    expect(size, const Size(4, 8));
  });

  testWidgets('the overlay draws all locations of the page over the page', (
    tester,
  ) async {
    const other = PageLocation(
      page: 1,
      x: 0.1,
      y: 0.8,
      width: 0.2,
      height: 0.05,
    );
    await tester.pumpWidget(
      MaterialApp(
        home: Scaffold(
          body: LocationOverlay(
            image: MemoryImage(rotatedJpeg),
            pageSize: const Size(4, 8),
            locations: const [_location, other],
            current: _location,
          ),
        ),
      ),
    );
    await tester.pump();

    final paint = tester.widget<CustomPaint>(
      find.byWidgetPredicate(
        (w) => w is CustomPaint && w.painter is LocationMarksPainter,
      ),
    );
    final painter = paint.painter! as LocationMarksPainter;
    expect(painter.locations, hasLength(2));
    expect(painter.current, same(_location));

    // The marks lie exactly on the shown page: 4×8 in 800×600 is 300×600,
    // centered (before the zoom).
    expect(tester.getSize(find.byWidget(paint)), const Size(300, 600));
    final positioned = tester.widgetList<Positioned>(find.byType(Positioned));
    expect(positioned.map((p) => (p.left, p.top, p.width, p.height)).toSet(), {
      (250.0, 0.0, 300.0, 600.0),
    });

    // After the first frame the view zooms in on the current location.
    final viewer = tester.widget<InteractiveViewer>(
      find.byType(InteractiveViewer),
    );
    expect(viewer.transformationController!.value.getMaxScaleOnAxis(), 3);
  });
}
