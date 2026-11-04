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

/// Weicher Wechsel zwischen Zuständen im Stil von „Fade Through“: Das Alte
/// blendet schnell aus, das Neue blendet ein und wächst leicht von 92 %
/// auf 100 %. Das Kind braucht einen eigenen `Key` pro Zustand, sonst gibt
/// es keinen Wechsel.
class Uebergang extends StatelessWidget {
  const Uebergang({super.key, required this.child});

  final Widget child;

  @override
  Widget build(BuildContext context) {
    return AnimatedSwitcher(
      duration: bewegungsDauer(context, Durations.medium2),
      reverseDuration: bewegungsDauer(context, Durations.short2),
      switchInCurve: Easing.emphasizedDecelerate,
      switchOutCurve: Easing.emphasizedAccelerate,
      transitionBuilder: (kind, verlauf) => FadeTransition(
        opacity: verlauf,
        child: ScaleTransition(
          scale: Tween(begin: 0.92, end: 1.0).animate(verlauf),
          child: kind,
        ),
      ),
      // Oben ausrichten statt zentrieren: Inhalte springen nicht.
      layoutBuilder: (aktuell, vorige) => Stack(
        alignment: Alignment.topCenter,
        children: [...vorige, ?aktuell],
      ),
      child: child,
    );
  }
}

/// Blendet Kinder nacheinander ein (je [Durations.short2] versetzt), mit
/// einer kleinen Bewegung nach oben. Der Inhalt ist sofort vollständig da,
/// nur seine Sichtbarkeit wird weich; es gibt keinen Tipp-Effekt.
///
/// Ein neuer `key` startet die Animation neu (z. B. pro Antwort).
class GestaffeltEinblenden extends StatefulWidget {
  const GestaffeltEinblenden({super.key, required this.kinder});

  final List<Widget> kinder;

  @override
  State<GestaffeltEinblenden> createState() => _GestaffeltEinblendenState();
}

class _GestaffeltEinblendenState extends State<GestaffeltEinblenden>
    with SingleTickerProviderStateMixin {
  late final AnimationController _verlauf;
  var _gestartet = false;

  /// Abstand zwischen zwei Kindern und Dauer pro Kind.
  static const _abstand = Durations.short2;
  static const _dauerProKind = Durations.medium2;

  Duration get _gesamt =>
      _dauerProKind + _abstand * (widget.kinder.length - 1).clamp(0, 99);

  @override
  void initState() {
    super.initState();
    _verlauf = AnimationController(vsync: this, duration: _gesamt);
  }

  @override
  void didChangeDependencies() {
    super.didChangeDependencies();
    if (_gestartet) return;
    _gestartet = true;
    if (bewegungAus(context)) {
      _verlauf.value = 1;
    } else {
      _verlauf.forward();
    }
  }

  @override
  void dispose() {
    _verlauf.dispose();
    super.dispose();
  }

  @override
  Widget build(BuildContext context) {
    final gesamt = _gesamt.inMicroseconds;
    return Column(
      crossAxisAlignment: CrossAxisAlignment.stretch,
      children: [
        for (var i = 0; i < widget.kinder.length; i++)
          Builder(
            builder: (context) {
              final start = (_abstand * i).inMicroseconds / gesamt;
              final ende =
                  (_abstand * i + _dauerProKind).inMicroseconds / gesamt;
              final kurve = CurvedAnimation(
                parent: _verlauf,
                curve: Interval(
                  start.clamp(0.0, 1.0),
                  ende.clamp(0.0, 1.0),
                  curve: Easing.emphasizedDecelerate,
                ),
              );
              return FadeTransition(
                key: ValueKey('gestaffelt-$i'),
                opacity: kurve,
                child: SlideTransition(
                  position: Tween(
                    begin: const Offset(0, 0.06),
                    end: Offset.zero,
                  ).animate(kurve),
                  child: widget.kinder[i],
                ),
              );
            },
          ),
      ],
    );
  }
}
