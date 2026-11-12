import 'package:flutter/material.dart';

import '../api/api_client.dart';
import '../module/module_definition.dart';
import '../module/module_catalog.dart';
import '../platform/file_chooser.dart';
import '../widgets/motion.dart';
import '../widgets/container_transform.dart';
import '../widgets/mock_banner.dart';
import '../widgets/navigation.dart';
import 'module_shell.dart';

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
  final FileChooser dateiAuswahl;

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

  /// Aktive Kachel: Container Transform von der Kachel zum Modul (auf iOS
  /// der normale Übergang). Inaktive Kachel: nur die Kachel.
  Widget _kachel(ModuleDefinition module) {
    if (!module.active) return _ModulKachel(module: module, oeffnen: null);
    final theme = modulTheme(module);
    return ContainerTransform(
      form: RoundedRectangleBorder(borderRadius: BorderRadius.circular(12)),
      offenFarbe: theme.colorScheme.surface,
      geschlossen: (context, oeffnen) =>
          _ModulKachel(module: module, oeffnen: oeffnen),
      // Die Seite hängt am obersten Navigator: Farbschema mitgeben.
      offen: (_) => Theme(
        data: theme,
        child: ModulRahmen(
          module: module,
          api: widget.api,
          dateiAuswahl: widget.dateiAuswahl,
        ),
      ),
    );
  }

  @override
  Widget build(BuildContext context) {
    final modules = ModuleCatalog.of(context).modules;
    return Scaffold(
      appBar: AppBar(
        title: const Text('Lifelog'),
        actions: [MockHinweis(sichtbar: widget.api.isMock)],
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
        itemCount: modules.length,
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
            key: ValueKey('einblenden-${modules[i].id}'),
            opacity: verlauf,
            child: SlideTransition(
              position: Tween(
                begin: const Offset(0, 0.08),
                end: Offset.zero,
              ).animate(verlauf),
              child: _kachel(modules[i]),
            ),
          );
        },
      ),
    );
  }
}

class _ModulKachel extends StatefulWidget {
  const _ModulKachel({required this.module, required this.oeffnen});

  final ModuleDefinition module;

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
    final module = widget.module;
    // Jede Kachel in den Farben ihres eigenen Moduls.
    final farben = modulTheme(module).colorScheme;
    final textTheme = Theme.of(context).textTheme;
    final active = module.active;
    final vordergrund = active
        ? farben.onPrimaryContainer
        : farben.onSurfaceVariant.withValues(alpha: 0.6);

    return Semantics(
      button: true,
      enabled: active,
      label: active ? module.name : '${module.name}, bald verfügbar',
      excludeSemantics: true,
      child: AnimatedScale(
        key: ValueKey('druck-${module.id}'),
        scale: _gedrueckt ? 0.97 : 1,
        duration: bewegungsDauer(context, Durations.short2),
        curve: Easing.standard,
        child: Card(
          clipBehavior: Clip.antiAlias,
          color: active ? null : farben.surfaceContainerHigh,
          child: Ink(
            // Aktiv: mehr Modulfarbe als leichter Verlauf.
            decoration: active
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
              onHighlightChanged: active
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
                        color: active
                            ? farben.primary
                            : farben.surfaceContainerHighest,
                        shape: BoxShape.circle,
                      ),
                      child: Icon(
                        module.icon,
                        size: 28,
                        color: active ? farben.onPrimary : vordergrund,
                      ),
                    ),
                    const Spacer(),
                    Text(
                      module.name,
                      style: textTheme.titleMedium?.copyWith(
                        color: vordergrund,
                        fontWeight: active ? FontWeight.w600 : null,
                      ),
                      maxLines: 1,
                      overflow: TextOverflow.ellipsis,
                    ),
                    const SizedBox(height: 4),
                    Text(
                      module.description,
                      style: textTheme.bodySmall?.copyWith(color: vordergrund),
                      maxLines: 2,
                      overflow: TextOverflow.ellipsis,
                    ),
                    if (!active) ...[
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
