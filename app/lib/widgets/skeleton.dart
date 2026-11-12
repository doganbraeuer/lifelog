// Platzhalter (Skeleton) beim ersten Laden: graue Formen in der Gestalt
// des späteren Inhalts, die ruhig pulsieren. Für Screenreader heißt das
// Ganze „Wird geladen“; die Formen selbst sind ausgeblendet.

import 'package:flutter/material.dart';

import 'motion.dart';

/// Pulsiert seine Kinder ruhig in der Deckkraft. Mit „Animationen
/// entfernen“ bleibt es statisch.
class Pulsieren extends StatefulWidget {
  const Pulsieren({super.key, required this.child});

  final Widget child;

  @override
  State<Pulsieren> createState() => _PulsierenState();
}

class _PulsierenState extends State<Pulsieren>
    with SingleTickerProviderStateMixin {
  late final _verlauf = AnimationController(
    vsync: this,
    duration: Durations.extralong4,
  );

  @override
  void didChangeDependencies() {
    super.didChangeDependencies();
    if (bewegungAus(context)) {
      _verlauf
        ..stop()
        ..value = 1;
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
    return FadeTransition(
      opacity: Tween(
        begin: 0.45,
        end: 1.0,
      ).animate(CurvedAnimation(parent: _verlauf, curve: Easing.standard)),
      child: widget.child,
    );
  }
}

/// Ein grauer Balken mit runden Ecken.
class PlatzhalterBalken extends StatelessWidget {
  const PlatzhalterBalken({
    super.key,
    this.breite,
    this.hoehe = 12,
    this.rund = false,
  });

  final double? breite;
  final double hoehe;

  /// Kreis statt Balken (z. B. für einen Punkt der Zeitachse).
  final bool rund;

  @override
  Widget build(BuildContext context) {
    return Container(
      width: rund ? hoehe : breite,
      height: hoehe,
      decoration: BoxDecoration(
        color: Theme.of(context).colorScheme.surfaceContainerHighest,
        borderRadius: rund ? null : BorderRadius.circular(hoehe / 2),
        shape: rund ? BoxShape.circle : BoxShape.rectangle,
      ),
    );
  }
}

/// Rahmen für Platzhalter: meldet „Wird geladen“, blendet die Formen für
/// Screenreader aus und lässt sie pulsieren.
class Platzhalter extends StatelessWidget {
  const Platzhalter({super.key, required this.child});

  final Widget child;

  @override
  Widget build(BuildContext context) {
    return Semantics(
      container: true,
      label: 'Wird geladen',
      liveRegion: true,
      child: ExcludeSemantics(child: Pulsieren(child: child)),
    );
  }
}

/// Platzhalter in der Form der Zeitachse: eine Überschrift und fünf Zeilen.
class ZeitachsenPlatzhalter extends StatelessWidget {
  const ZeitachsenPlatzhalter({super.key});

  @override
  Widget build(BuildContext context) {
    return Platzhalter(
      child: ListView(
        physics: const NeverScrollableScrollPhysics(),
        padding: const EdgeInsets.fromLTRB(0, 20, 16, 0),
        children: [
          const Padding(
            padding: EdgeInsets.fromLTRB(64, 0, 0, 16),
            child: Align(
              alignment: Alignment.centerLeft,
              child: PlatzhalterBalken(breite: 120, hoehe: 14),
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
                      child: PlatzhalterBalken(hoehe: 36, rund: true),
                    ),
                  ),
                  Expanded(
                    child: Column(
                      crossAxisAlignment: CrossAxisAlignment.start,
                      children: [
                        PlatzhalterBalken(breite: 110.0 + (i % 3) * 30),
                        const SizedBox(height: 8),
                        PlatzhalterBalken(
                          breite: 160.0 - (i % 2) * 40,
                          hoehe: 10,
                        ),
                      ],
                    ),
                  ),
                  const PlatzhalterBalken(breite: 72),
                ],
              ),
            ),
        ],
      ),
    );
  }
}

/// Platzhalter für eine Antwort mit zwei Quellen-Karten.
class AntwortPlatzhalter extends StatelessWidget {
  const AntwortPlatzhalter({super.key});

  @override
  Widget build(BuildContext context) {
    Widget karte(List<Widget> kinder) => Card(
      margin: const EdgeInsets.only(bottom: 8),
      child: Padding(
        padding: const EdgeInsets.all(16),
        child: Column(
          crossAxisAlignment: CrossAxisAlignment.start,
          children: kinder,
        ),
      ),
    );

    return Platzhalter(
      child: Column(
        crossAxisAlignment: CrossAxisAlignment.stretch,
        children: [
          karte(const [
            PlatzhalterBalken(),
            SizedBox(height: 10),
            PlatzhalterBalken(),
            SizedBox(height: 10),
            PlatzhalterBalken(breite: 180),
          ]),
          const SizedBox(height: 12),
          for (var i = 0; i < 2; i++)
            karte(const [
              Row(
                children: [
                  PlatzhalterBalken(hoehe: 32, rund: true),
                  SizedBox(width: 12),
                  Expanded(child: PlatzhalterBalken()),
                ],
              ),
              SizedBox(height: 10),
              PlatzhalterBalken(breite: 200, hoehe: 10),
            ]),
        ],
      ),
    );
  }
}
