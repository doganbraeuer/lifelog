import 'package:flutter/material.dart';

import 'motion.dart';

/// Calm animation while the backend processes a document: a line glides up
/// and down over a document icon.
///
/// It shows **no progress** (the app does not know it), only that
/// something is running. With „Animationen entfernen“ the icon stays still.
class ScanAnimation extends StatefulWidget {
  const ScanAnimation({super.key});

  @override
  State<ScanAnimation> createState() => _ScanAnimationState();
}

class _ScanAnimationState extends State<ScanAnimation>
    with SingleTickerProviderStateMixin {
  late final _controller = AnimationController(
    vsync: this,
    duration: Durations.extralong4,
  );

  static const _size = 96.0;

  @override
  void didChangeDependencies() {
    super.didChangeDependencies();
    if (motionDisabled(context)) {
      _controller.stop();
    } else if (!_controller.isAnimating) {
      _controller.repeat(reverse: true);
    }
  }

  @override
  void dispose() {
    _controller.dispose();
    super.dispose();
  }

  @override
  Widget build(BuildContext context) {
    final colors = Theme.of(context).colorScheme;
    final isStatic = motionDisabled(context);
    final position = CurvedAnimation(
      parent: _controller,
      curve: Easing.standard,
    );
    return Semantics(
      // Its own node, so that the screen reader reads it separately from the
      // text below.
      container: true,
      label: 'Dokument wird ausgewertet',
      liveRegion: true,
      child: ExcludeSemantics(
        child: SizedBox(
          width: _size,
          height: _size,
          child: Stack(
            children: [
              Icon(
                Icons.description_outlined,
                size: _size,
                color: colors.onSurfaceVariant.withValues(alpha: 0.5),
              ),
              if (!isStatic)
                AnimatedBuilder(
                  animation: position,
                  builder: (context, _) => Positioned(
                    left: _size * 0.2,
                    right: _size * 0.2,
                    top: _size * (0.18 + 0.62 * position.value),
                    child: Container(
                      height: 3,
                      decoration: BoxDecoration(
                        color: colors.primary,
                        borderRadius: BorderRadius.circular(2),
                        boxShadow: [
                          BoxShadow(
                            color: colors.primary.withValues(alpha: 0.4),
                            blurRadius: 8,
                          ),
                        ],
                      ),
                    ),
                  ),
                ),
            ],
          ),
        ),
      ),
    );
  }
}
