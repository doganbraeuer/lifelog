import 'package:flutter/material.dart';

import '../api/api_client.dart';
import '../module/modul_definition.dart';
import '../module/modul_katalog.dart';
import '../plattform/datei_auswahl.dart';
import '../widgets/bewegung.dart';
import '../widgets/mock_hinweis.dart';
import '../widgets/navigation.dart';
import 'modul_rahmen.dart';

/// Startscreen: eine Kachel pro Modul aus dem Modul-Katalog.
///
/// Aktive Module öffnen ihren Rahmen (Zeitleiste, Hochladen, Fragen) in
/// ihrem eigenen Farbschema. Inaktive zeigen „Bald verfügbar“.
/// Beim ersten Aufbau blenden die Kacheln nacheinander ein.
class BereicheScreen extends StatefulWidget {
  const BereicheScreen({
    super.key,
    required this.api,
    required this.dateiAuswahl,
  });

  final ApiClient api;
  final DateiAuswahl dateiAuswahl;

  @override
  State<BereicheScreen> createState() => _BereicheScreenState();
}

class _BereicheScreenState extends State<BereicheScreen>
    with SingleTickerProviderStateMixin {
  /// Ein Controller für das Einblenden aller Kacheln, versetzt per Interval.
  late final _einblenden = AnimationController(
    vsync: this,
    duration: Durations.long4,
  );
  var _gestartet = false;

  @override
  void didChangeDependencies() {
    super.didChangeDependencies();
    // Nur beim ersten Aufbau; beim Zurückkehren bleibt der Zustand erhalten.
    if (_gestartet) return;
    _gestartet = true;
    if (bewegungAus(context)) {
      _einblenden.value = 1;
    } else {
      _einblenden.forward();
    }
  }

  @override
  void dispose() {
    _einblenden.dispose();
    super.dispose();
  }

  void _oeffnen(BuildContext context, ModulDefinition modul) {
    Navigator.of(context).push(
      MaterialPageRoute<void>(
        builder: (_) => Theme(
          data: modulTheme(modul),
          child: ModulRahmen(
            modul: modul,
            api: widget.api,
            dateiAuswahl: widget.dateiAuswahl,
          ),
        ),
      ),
    );
  }

  @override
  Widget build(BuildContext context) {
    final module = ModulKatalog.von(context).module;
    return Scaffold(
      appBar: AppBar(
        title: const Text('Lifelog'),
        actions: [MockHinweis(sichtbar: widget.api.istMock)],
      ),
      body: GridView.builder(
        padding: const EdgeInsets.all(16),
        gridDelegate: const SliverGridDelegateWithMaxCrossAxisExtent(
          // Auf dem Handy zwei Spalten, auf breiten Geräten mehr. Feste Höhe
          // statt Seitenverhältnis: Auf schmalen Bildschirmen würden die
          // Kacheln sonst zu niedrig für ihren Inhalt.
          maxCrossAxisExtent: 240,
          mainAxisSpacing: 12,
          crossAxisSpacing: 12,
          mainAxisExtent: 200,
        ),
        itemCount: module.length,
        itemBuilder: (context, i) {
          // Versetzt: Jede Kachel startet etwas später als die vorige.
          final start = (i * 0.12).clamp(0.0, 0.5);
          final verlauf = CurvedAnimation(
            parent: _einblenden,
            curve: Interval(
              start,
              start + 0.5,
              curve: Easing.emphasizedDecelerate,
            ),
          );
          return FadeTransition(
            key: ValueKey('einblenden-${module[i].id}'),
            opacity: verlauf,
            child: SlideTransition(
              position: Tween(
                begin: const Offset(0, 0.08),
                end: Offset.zero,
              ).animate(verlauf),
              child: _ModulKachel(
                modul: module[i],
                oeffnen: module[i].aktiv
                    ? () => _oeffnen(context, module[i])
                    : null,
              ),
            ),
          );
        },
      ),
    );
  }
}

class _ModulKachel extends StatefulWidget {
  const _ModulKachel({required this.modul, required this.oeffnen});

  final ModulDefinition modul;

  /// `null` bei inaktiven Modulen: Die Kachel reagiert nicht.
  final VoidCallback? oeffnen;

  @override
  State<_ModulKachel> createState() => _ModulKachelState();
}

class _ModulKachelState extends State<_ModulKachel> {
  /// Während der Finger auf der Kachel liegt: leicht verkleinert.
  var _gedrueckt = false;

  @override
  Widget build(BuildContext context) {
    final modul = widget.modul;
    // Jede Kachel in den Farben ihres eigenen Moduls.
    final farben = modulTheme(modul).colorScheme;
    final textTheme = Theme.of(context).textTheme;
    final aktiv = modul.aktiv;
    final vordergrund = aktiv
        ? farben.onPrimaryContainer
        : farben.onSurfaceVariant.withValues(alpha: 0.6);

    return Semantics(
      button: true,
      enabled: aktiv,
      label: aktiv ? modul.name : '${modul.name}, bald verfügbar',
      excludeSemantics: true,
      child: AnimatedScale(
        key: ValueKey('druck-${modul.id}'),
        scale: _gedrueckt ? 0.97 : 1,
        duration: bewegungsDauer(context, Durations.short2),
        curve: Easing.standard,
        child: Card(
          clipBehavior: Clip.antiAlias,
          color: aktiv ? null : farben.surfaceContainerHigh,
          child: Ink(
            // Aktiv: mehr Modulfarbe als leichter Verlauf.
            decoration: aktiv
                ? BoxDecoration(
                    gradient: LinearGradient(
                      begin: Alignment.topLeft,
                      end: Alignment.bottomRight,
                      colors: [
                        farben.primaryContainer,
                        farben.secondaryContainer,
                      ],
                    ),
                  )
                : null,
            child: InkWell(
              onTap: widget.oeffnen,
              onHighlightChanged: aktiv
                  ? (gedrueckt) => setState(() => _gedrueckt = gedrueckt)
                  : null,
              child: Padding(
                padding: const EdgeInsets.all(16),
                child: Column(
                  crossAxisAlignment: CrossAxisAlignment.start,
                  children: [
                    Container(
                      width: 48,
                      height: 48,
                      decoration: BoxDecoration(
                        color: aktiv
                            ? farben.primary
                            : farben.surfaceContainerHighest,
                        shape: BoxShape.circle,
                      ),
                      child: Icon(
                        modul.symbol,
                        size: 28,
                        color: aktiv ? farben.onPrimary : vordergrund,
                      ),
                    ),
                    const Spacer(),
                    Text(
                      modul.name,
                      style: textTheme.titleMedium?.copyWith(
                        color: vordergrund,
                        fontWeight: aktiv ? FontWeight.w600 : null,
                      ),
                      maxLines: 1,
                      overflow: TextOverflow.ellipsis,
                    ),
                    const SizedBox(height: 4),
                    Text(
                      modul.beschreibung,
                      style: textTheme.bodySmall?.copyWith(color: vordergrund),
                      maxLines: 2,
                      overflow: TextOverflow.ellipsis,
                    ),
                    if (!aktiv) ...[
                      const SizedBox(height: 8),
                      Container(
                        padding: const EdgeInsets.symmetric(
                          horizontal: 8,
                          vertical: 2,
                        ),
                        decoration: BoxDecoration(
                          color: farben.surfaceContainerHighest,
                          borderRadius: BorderRadius.circular(8),
                        ),
                        child: Text(
                          'Bald verfügbar',
                          // Eine Zeile, damit die Kachel auch bei großer
                          // Schrift oder schmalen Geräten nicht überläuft.
                          maxLines: 1,
                          overflow: TextOverflow.ellipsis,
                          style: textTheme.labelSmall?.copyWith(
                            color: farben.onSurfaceVariant,
                          ),
                        ),
                      ),
                    ],
                  ],
                ),
              ),
            ),
          ),
        ),
      ),
    );
  }
}
