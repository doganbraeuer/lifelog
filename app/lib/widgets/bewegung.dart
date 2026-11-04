// Gemeinsame Bausteine für Bewegung in der App.
//
// Regeln:
// - Zeiten und Kurven kommen nur aus den Material-Konstanten `Durations`
//   und `Easing` (bzw. `Curves.easeInOutCubicEmphasized`), keine eigenen
//   Werte.
// - Hat das Gerät „Animationen entfernen“ eingeschaltet
//   (MediaQuery.disableAnimations), entfällt jede eigene Animation: Inhalte
//   stehen sofort und statisch da.

import 'package:flutter/material.dart';

/// `true`, wenn das Gerät keine Animationen möchte (Bedienungshilfen).
bool bewegungAus(BuildContext context) =>
    MediaQuery.disableAnimationsOf(context);

/// [dauer], oder `Duration.zero`, wenn Animationen aus sind.
Duration bewegungsDauer(BuildContext context, Duration dauer) =>
    bewegungAus(context) ? Duration.zero : dauer;
