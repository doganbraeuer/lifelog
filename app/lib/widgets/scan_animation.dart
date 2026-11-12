import 'package:flutter/material.dart';

import 'motion.dart';

/// Ruhige Animation während das Backend ein Dokument auswertet: Über ein
/// Dokument-Symbol gleitet eine Linie auf und ab.
///
/// Sie zeigt **keinen Fortschritt** an (den kennt die App nicht), nur dass
/// etwas läuft. Mit „Animationen entfernen“ steht das Symbol still.
class ScanAnimation extends StatefulWidget {
  const ScanAnimation({super.key});

  @override
  State<ScanAnimation> createState() => _ScanAnimationState();
}

class _ScanAnimationState extends State<ScanAnimation>
    with SingleTickerProviderStateMixin {
  late final _verlauf = AnimationController(
    vsync: this,
    duration: Durations.extralong4,
  );

  static const _groesse = 96.0;

  @override
  void didChangeDependencies() {
    super.didChangeDependencies();
    if (bewegungAus(context)) {
      _verlauf.stop();
    } else if (!_verlauf.isAnimating) {
      _verlauf.repeat(reverse: true);
    }
  }

  @override
  void dispose() {
    _verlauf.dispose();
    super.dispose();
  }

  @override
  Widget build(BuildContext context) {
    final farben = Theme.of(context).colorScheme;
    final still = bewegungAus(context);
    final position = CurvedAnimation(parent: _verlauf, curve: Easing.standard);
    return Semantics(
      // Eigener Knoten, damit der Screenreader es getrennt vom Text
      // darunter vorliest.
      container: true,
      label: 'Dokument wird ausgewertet',
      liveRegion: true,
      child: ExcludeSemantics(
        child: SizedBox(
          width: _groesse,
          height: _groesse,
          child: Stack(
            children: [
              Icon(
                Icons.description_outlined,
                size: _groesse,
                color: farben.onSurfaceVariant.withValues(alpha: 0.5),
              ),
              if (!still)
                AnimatedBuilder(
                  animation: position,
                  builder: (context, _) => Positioned(
                    left: _groesse * 0.2,
                    right: _groesse * 0.2,
                    top: _groesse * (0.18 + 0.62 * position.value),
                    child: Container(
                      height: 3,
                      decoration: BoxDecoration(
                        color: farben.primary,
                        borderRadius: BorderRadius.circular(2),
                        boxShadow: [
                          BoxShadow(
                            color: farben.primary.withValues(alpha: 0.4),
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
