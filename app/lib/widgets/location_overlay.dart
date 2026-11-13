// Shows a page (a photo or a rendered PDF page) with the locations of a
// value marked on it (docs/API.md, section 2, "Verification").
//
// A location is a rectangle in fractions (0–1) of the upright page. The page
// is fitted into the available space (BoxFit.contain, centered), and the
// rectangles are drawn in exactly the box the page takes up. At the start
// the view zooms in on the current location so that it can be read.

import 'dart:math' as math;

import 'package:flutter/material.dart';

import '../model/verification.dart';

/// Where a page of [pageSize] appears when fitted into [viewport]
/// (`BoxFit.contain`, centered).
Rect fittedPageRect(Size pageSize, Size viewport) {
  if (pageSize.isEmpty || viewport.isEmpty) return Offset.zero & viewport;
  final fitted = applyBoxFit(BoxFit.contain, pageSize, viewport).destination;
  return Alignment.center.inscribe(fitted, Offset.zero & viewport);
}

/// The rectangle of [location] on a page shown in [page] (in the same
/// coordinates as [page]).
Rect locationRect(PageLocation location, Rect page) => Rect.fromLTWH(
  page.left + location.x * page.width,
  page.top + location.y * page.height,
  location.width * page.width,
  location.height * page.height,
);

/// Start transformation for an `InteractiveViewer` whose child fills
/// [viewport]: zooms in on [target] and centers it.
///
/// The scale makes the target take up about 80 % of the width or 40 % of the
/// height, between 1 and [maxScale]. The shift is clamped so that no area
/// outside the child becomes visible.
Matrix4 focusTransform(Size viewport, Rect target, {double maxScale = 3}) {
  final width = math.max(target.width, 1.0);
  final height = math.max(target.height, 1.0);
  final scale = math
      .min(viewport.width * 0.8 / width, viewport.height * 0.4 / height)
      .clamp(1.0, maxScale);
  final dx = (viewport.width / 2 - scale * target.center.dx).clamp(
    viewport.width - scale * viewport.width,
    0.0,
  );
  final dy = (viewport.height / 2 - scale * target.center.dy).clamp(
    viewport.height - scale * viewport.height,
    0.0,
  );
  return Matrix4.diagonal3Values(scale, scale, 1)..setTranslationRaw(dx, dy, 0);
}

/// A page with the locations of a value marked on it; zoomable.
class LocationOverlay extends StatefulWidget {
  const LocationOverlay({
    super.key,
    required this.image,
    required this.pageSize,
    required this.locations,
    required this.current,
  });

  /// The upright page.
  final ImageProvider image;

  /// Size of the upright page (pixels or points; only the ratio counts).
  final Size pageSize;

  /// All locations of the value on this page.
  final List<PageLocation> locations;

  /// The location to zoom in on; marked more strongly than the others.
  final PageLocation current;

  @override
  State<LocationOverlay> createState() => _LocationOverlayState();
}

class _LocationOverlayState extends State<LocationOverlay> {
  final _transform = TransformationController();

  /// Location and viewport the current zoom was computed for.
  (PageLocation, Size)? _focused;

  @override
  void dispose() {
    _transform.dispose();
    super.dispose();
  }

  /// Zooms in on the current location after the frame, if it or the
  /// available space changed. (Setting the controller during the build
  /// would rebuild the viewer in the middle of the build.)
  void _focusLater(Size viewport, Rect target) {
    final key = (widget.current, viewport);
    if (_focused == key) return;
    _focused = key;
    WidgetsBinding.instance.addPostFrameCallback((_) {
      if (mounted) _transform.value = focusTransform(viewport, target);
    });
  }

  @override
  Widget build(BuildContext context) {
    final colors = Theme.of(context).colorScheme;
    return ColoredBox(
      color: colors.surfaceContainer,
      child: LayoutBuilder(
        builder: (context, constraints) {
          final viewport = constraints.biggest;
          final page = fittedPageRect(widget.pageSize, viewport);
          _focusLater(viewport, locationRect(widget.current, page));
          return InteractiveViewer(
            transformationController: _transform,
            maxScale: 8,
            child: SizedBox.fromSize(
              size: viewport,
              child: Stack(
                children: [
                  Positioned.fromRect(
                    rect: page,
                    child: Image(
                      image: widget.image,
                      fit: BoxFit.fill,
                      gaplessPlayback: true,
                    ),
                  ),
                  Positioned.fromRect(
                    rect: page,
                    child: CustomPaint(
                      painter: LocationMarksPainter(
                        locations: widget.locations,
                        current: widget.current,
                        color: colors.primary,
                      ),
                    ),
                  ),
                ],
              ),
            ),
          );
        },
      ),
    );
  }
}

/// Draws the locations over a page that fills the canvas: the current one
/// with a light fill and a frame, the others only lightly filled. Clear but
/// calm, in the color of the module.
class LocationMarksPainter extends CustomPainter {
  const LocationMarksPainter({
    required this.locations,
    required this.current,
    required this.color,
  });

  final List<PageLocation> locations;
  final PageLocation current;
  final Color color;

  @override
  void paint(Canvas canvas, Size size) {
    final page = Offset.zero & size;
    final otherFill = Paint()..color = color.withValues(alpha: 0.12);
    final currentFill = Paint()..color = color.withValues(alpha: 0.22);
    final frame = Paint()
      ..color = color
      ..style = PaintingStyle.stroke
      ..strokeWidth = 1.5;
    for (final location in locations) {
      if (identical(location, current)) continue;
      canvas.drawRRect(_marked(location, page), otherFill);
    }
    final rect = _marked(current, page);
    canvas
      ..drawRRect(rect, currentFill)
      ..drawRRect(rect, frame);
  }

  /// A little larger than the word, so the frame does not cover letters.
  static RRect _marked(PageLocation location, Rect page) {
    final rect = locationRect(location, page);
    final padding = math.max(2.0, rect.height * 0.2);
    return RRect.fromRectAndRadius(
      rect.inflate(padding),
      Radius.circular(padding),
    );
  }

  @override
  bool shouldRepaint(LocationMarksPainter old) =>
      old.locations != locations ||
      old.current != current ||
      old.color != color;
}
