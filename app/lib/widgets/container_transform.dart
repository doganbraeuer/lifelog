import 'package:animations/animations.dart';
import 'package:flutter/material.dart';

import 'motion.dart';

/// Container transform (Material motion): the tapped surface grows into the
/// new page and shrinks back when it closes.
///
/// On iOS there is the normal transition instead: the route of
/// `OpenContainer` has no swipe-back gesture, and that should stay there.
/// With „Animationen entfernen“ (remove animations) the transition takes 0.
class ContainerTransform extends StatelessWidget {
  const ContainerTransform({
    super.key,
    required this.closedBuilder,
    required this.openBuilder,
    this.onClosed,
    this.shape = const RoundedRectangleBorder(),
    this.openColor,
  });

  /// The surface in the closed state; `onOpen` opens the page.
  final Widget Function(BuildContext context, VoidCallback onOpen)
  closedBuilder;

  /// The opened page. It hangs on the top-level navigator, so it has to bring
  /// its own color scheme (e.g. with `InheritedTheme.capture`).
  final WidgetBuilder openBuilder;

  /// After closing (with `OpenContainer` at the start of the reverse
  /// animation, on iOS after going back).
  final VoidCallback? onClosed;

  /// Shape of the closed surface, e.g. rounded like a card.
  final ShapeBorder shape;

  /// Background of the opened page during the transition.
  final Color? openColor;

  @override
  Widget build(BuildContext context) {
    final theme = Theme.of(context);
    if (theme.platform == TargetPlatform.iOS) {
      return closedBuilder(context, () async {
        await Navigator.of(context)
            .push(MaterialPageRoute<void>(builder: openBuilder));
        onClosed?.call();
      });
    }
    return OpenContainer<void>(
      // The surface handles taps itself (InkWell, semantics stay).
      tappable: false,
      closedElevation: 0,
      openElevation: 0,
      closedColor: Colors.transparent,
      openColor: openColor ?? theme.colorScheme.surface,
      closedShape: shape,
      transitionType: ContainerTransitionType.fadeThrough,
      transitionDuration: motionDuration(context, Durations.medium4),
      onClosed: (_) => onClosed?.call(),
      closedBuilder: closedBuilder,
      openBuilder: (context, _) => openBuilder(context),
    );
  }
}
