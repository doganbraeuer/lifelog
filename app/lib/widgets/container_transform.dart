import 'package:animations/animations.dart';
import 'package:flutter/material.dart';

import 'bewegung.dart';

/// Container Transform (Material Motion): Die angetippte Fläche wächst zur
/// neuen Seite und schrumpft beim Schließen zurück.
///
/// Auf iOS gibt es stattdessen den normalen Übergang: Die Route von
/// `OpenContainer` hat keine Wisch-zurück-Geste, und die soll dort bleiben.
/// Mit „Animationen entfernen“ dauert der Übergang 0.
class ContainerTransform extends StatelessWidget {
  const ContainerTransform({
    super.key,
    required this.geschlossen,
    required this.offen,
    this.beimSchliessen,
    this.form = const RoundedRectangleBorder(),
    this.offenFarbe,
  });

  /// Die Fläche im geschlossenen Zustand; [oeffnen] öffnet die Seite.
  final Widget Function(BuildContext context, VoidCallback oeffnen) geschlossen;

  /// Die geöffnete Seite. Sie hängt am obersten Navigator, muss also ihr
  /// Farbschema selbst mitbringen (z. B. mit `InheritedTheme.capture`).
  final WidgetBuilder offen;

  /// Nach dem Schließen (bei `OpenContainer` zu Beginn der Rücklauf-
  /// Animation, auf iOS nach dem Zurückgehen).
  final VoidCallback? beimSchliessen;

  /// Form der geschlossenen Fläche, z. B. abgerundet wie eine Karte.
  final ShapeBorder form;

  /// Hintergrund der geöffneten Seite während des Übergangs.
  final Color? offenFarbe;

  @override
  Widget build(BuildContext context) {
    final theme = Theme.of(context);
    if (theme.platform == TargetPlatform.iOS) {
      return geschlossen(context, () async {
        await Navigator.of(context)
            .push(MaterialPageRoute<void>(builder: offen));
        beimSchliessen?.call();
      });
    }
    return OpenContainer<void>(
      // Antippen übernimmt die Fläche selbst (InkWell, Semantik bleibt).
      tappable: false,
      closedElevation: 0,
      openElevation: 0,
      closedColor: Colors.transparent,
      openColor: offenFarbe ?? theme.colorScheme.surface,
      closedShape: form,
      transitionType: ContainerTransitionType.fadeThrough,
      transitionDuration: bewegungsDauer(context, Durations.medium4),
      onClosed: (_) => beimSchliessen?.call(),
      closedBuilder: geschlossen,
      openBuilder: (context, _) => offen(context),
    );
  }
}
