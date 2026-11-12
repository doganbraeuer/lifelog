// Shared building blocks for motion in the app.
//
// Rules:
// - Durations and curves only come from the Material constants `Durations`
//   and `Easing` (or `Curves.easeInOutCubicEmphasized`), no values of our
//   own.
// - If the device has „Animationen entfernen“ (remove animations) turned on
//   (MediaQuery.disableAnimations), every animation of our own is dropped:
//   content is there at once and static.

import 'package:flutter/material.dart';

/// `true` if the device wants no animations (accessibility settings).
bool motionDisabled(BuildContext context) =>
    MediaQuery.disableAnimationsOf(context);

/// [duration], or `Duration.zero` if animations are off.
Duration motionDuration(BuildContext context, Duration duration) =>
    motionDisabled(context) ? Duration.zero : duration;

/// Soft switch between states in the style of "fade through": the old one
/// fades out quickly, the new one fades in and grows slightly from 92 % to
/// 100 %. The child needs its own `Key` per state, otherwise there is no
/// switch.
class FadeThroughSwitcher extends StatelessWidget {
  const FadeThroughSwitcher({super.key, required this.child});

  final Widget child;

  @override
  Widget build(BuildContext context) {
    return AnimatedSwitcher(
      duration: motionDuration(context, Durations.medium2),
      reverseDuration: motionDuration(context, Durations.short2),
      switchInCurve: Easing.emphasizedDecelerate,
      switchOutCurve: Easing.emphasizedAccelerate,
      transitionBuilder: (child, animation) => FadeTransition(
        opacity: animation,
        child: ScaleTransition(
          scale: Tween(begin: 0.92, end: 1.0).animate(animation),
          child: child,
        ),
      ),
      // Align to the top instead of centering: content does not jump.
      layoutBuilder: (current, previous) => Stack(
        alignment: Alignment.topCenter,
        children: [...previous, ?current],
      ),
      child: child,
    );
  }
}

/// Fades children in one after another (each offset by [Durations.short2]),
/// with a small upward movement. The content is complete at once, only its
/// visibility becomes soft; there is no typewriter effect.
///
/// A new `key` restarts the animation (e.g. per answer).
class StaggeredFadeIn extends StatefulWidget {
  const StaggeredFadeIn({super.key, required this.children});

  final List<Widget> children;

  @override
  State<StaggeredFadeIn> createState() => _StaggeredFadeInState();
}

class _StaggeredFadeInState extends State<StaggeredFadeIn>
    with SingleTickerProviderStateMixin {
  late final AnimationController _controller;
  var _started = false;

  /// Offset between two children and duration per child.
  static const _spacing = Durations.short2;
  static const _durationPerChild = Durations.medium2;

  Duration get _total =>
      _durationPerChild + _spacing * (widget.children.length - 1).clamp(0, 99);

  @override
  void initState() {
    super.initState();
    _controller = AnimationController(vsync: this, duration: _total);
  }

  @override
  void didChangeDependencies() {
    super.didChangeDependencies();
    if (_started) return;
    _started = true;
    if (motionDisabled(context)) {
      _controller.value = 1;
    } else {
      _controller.forward();
    }
  }

  @override
  void dispose() {
    _controller.dispose();
    super.dispose();
  }

  @override
  Widget build(BuildContext context) {
    final total = _total.inMicroseconds;
    return Column(
      crossAxisAlignment: CrossAxisAlignment.stretch,
      children: [
        for (var i = 0; i < widget.children.length; i++)
          Builder(
            builder: (context) {
              final start = (_spacing * i).inMicroseconds / total;
              final end =
                  (_spacing * i + _durationPerChild).inMicroseconds / total;
              final curve = CurvedAnimation(
                parent: _controller,
                curve: Interval(
                  start.clamp(0.0, 1.0),
                  end.clamp(0.0, 1.0),
                  curve: Easing.emphasizedDecelerate,
                ),
              );
              return FadeTransition(
                key: ValueKey('staggered-$i'),
                opacity: curve,
                child: SlideTransition(
                  position: Tween(
                    begin: const Offset(0, 0.06),
                    end: Offset.zero,
                  ).animate(curve),
                  child: widget.children[i],
                ),
              );
            },
          ),
      ],
    );
  }
}
