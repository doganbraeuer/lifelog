// Skeletons for the first load: grey shapes in the form of the later
// content that pulse calmly. For screen readers the whole thing is called
// „Wird geladen“ (loading); the shapes themselves are hidden.

import 'package:flutter/material.dart';

import 'motion.dart';

/// Pulses its child calmly in opacity. With „Animationen entfernen“ it
/// stays static.
class Pulse extends StatefulWidget {
  const Pulse({super.key, required this.child});

  final Widget child;

  @override
  State<Pulse> createState() => _PulseState();
}

class _PulseState extends State<Pulse> with SingleTickerProviderStateMixin {
  late final _controller = AnimationController(
    vsync: this,
    duration: Durations.extralong4,
  );

  @override
  void didChangeDependencies() {
    super.didChangeDependencies();
    if (motionDisabled(context)) {
      _controller
        ..stop()
        ..value = 1;
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
    return FadeTransition(
      opacity: Tween(
        begin: 0.45,
        end: 1.0,
      ).animate(CurvedAnimation(parent: _controller, curve: Easing.standard)),
      child: widget.child,
    );
  }
}

/// A grey bar with rounded corners.
class SkeletonBar extends StatelessWidget {
  const SkeletonBar({
    super.key,
    this.width,
    this.height = 12,
    this.circular = false,
  });

  final double? width;
  final double height;

  /// Circle instead of a bar (e.g. for a dot on the timeline axis).
  final bool circular;

  @override
  Widget build(BuildContext context) {
    return Container(
      width: circular ? height : width,
      height: height,
      decoration: BoxDecoration(
        color: Theme.of(context).colorScheme.surfaceContainerHighest,
        borderRadius: circular ? null : BorderRadius.circular(height / 2),
        shape: circular ? BoxShape.circle : BoxShape.rectangle,
      ),
    );
  }
}

/// Frame for skeletons: announces „Wird geladen“, hides the shapes from
/// screen readers and lets them pulse.
class Skeleton extends StatelessWidget {
  const Skeleton({super.key, required this.child});

  final Widget child;

  @override
  Widget build(BuildContext context) {
    return Semantics(
      container: true,
      label: 'Wird geladen',
      liveRegion: true,
      child: ExcludeSemantics(child: Pulse(child: child)),
    );
  }
}

/// Skeleton in the form of the timeline axis: a header and five rows.
class TimelineSkeleton extends StatelessWidget {
  const TimelineSkeleton({super.key});

  @override
  Widget build(BuildContext context) {
    return Skeleton(
      child: ListView(
        physics: const NeverScrollableScrollPhysics(),
        padding: const EdgeInsets.fromLTRB(0, 20, 16, 0),
        children: [
          const Padding(
            padding: EdgeInsets.fromLTRB(64, 0, 0, 16),
            child: Align(
              alignment: Alignment.centerLeft,
              child: SkeletonBar(width: 120, height: 14),
            ),
          ),
          for (var i = 0; i < 5; i++)
            Padding(
              padding: const EdgeInsets.symmetric(vertical: 14),
              child: Row(
                children: [
                  const SizedBox(
                    width: 64,
                    child: Center(
                      child: SkeletonBar(height: 36, circular: true),
                    ),
                  ),
                  Expanded(
                    child: Column(
                      crossAxisAlignment: CrossAxisAlignment.start,
                      children: [
                        SkeletonBar(width: 110.0 + (i % 3) * 30),
                        const SizedBox(height: 8),
                        SkeletonBar(width: 160.0 - (i % 2) * 40, height: 10),
                      ],
                    ),
                  ),
                  const SkeletonBar(width: 72),
                ],
              ),
            ),
        ],
      ),
    );
  }
}

/// Skeleton for an answer with two source cards.
class AnswerSkeleton extends StatelessWidget {
  const AnswerSkeleton({super.key});

  @override
  Widget build(BuildContext context) {
    Widget card(List<Widget> children) => Card(
      margin: const EdgeInsets.only(bottom: 8),
      child: Padding(
        padding: const EdgeInsets.all(16),
        child: Column(
          crossAxisAlignment: CrossAxisAlignment.start,
          children: children,
        ),
      ),
    );

    return Skeleton(
      child: Column(
        crossAxisAlignment: CrossAxisAlignment.stretch,
        children: [
          card(const [
            SkeletonBar(),
            SizedBox(height: 10),
            SkeletonBar(),
            SizedBox(height: 10),
            SkeletonBar(width: 180),
          ]),
          const SizedBox(height: 12),
          for (var i = 0; i < 2; i++)
            card(const [
              Row(
                children: [
                  SkeletonBar(height: 32, circular: true),
                  SizedBox(width: 12),
                  Expanded(child: SkeletonBar()),
                ],
              ),
              SizedBox(height: 10),
              SkeletonBar(width: 200, height: 10),
            ]),
        ],
      ),
    );
  }
}
