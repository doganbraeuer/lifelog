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

/// Start screen: one tile per module from the module catalog.
///
/// Active modules open their shell (timeline, upload, questions) in their
/// own color scheme. Inactive ones show „Bald verfügbar“ (coming soon).
/// On the first build the tiles fade in one after another.
class ModulesScreen extends StatefulWidget {
  const ModulesScreen({
    super.key,
    required this.api,
    required this.fileChooser,
  });

  final ApiClient api;
  final FileChooser fileChooser;

  @override
  State<ModulesScreen> createState() => _ModulesScreenState();
}

class _ModulesScreenState extends State<ModulesScreen>
    with SingleTickerProviderStateMixin {
  /// One controller for fading in all tiles, staggered with an Interval.
  late final _fadeIn = AnimationController(
    vsync: this,
    duration: Durations.long4,
  );
  var _started = false;

  @override
  void didChangeDependencies() {
    super.didChangeDependencies();
    // Only on the first build; when coming back the state is kept.
    if (_started) return;
    _started = true;
    if (motionDisabled(context)) {
      _fadeIn.value = 1;
    } else {
      _fadeIn.forward();
    }
  }

  @override
  void dispose() {
    _fadeIn.dispose();
    super.dispose();
  }

  /// Active tile: container transform from the tile to the module (on iOS
  /// the normal transition). Inactive tile: just the tile.
  Widget _tile(ModuleDefinition module) {
    if (!module.active) return _ModuleTile(module: module, onOpen: null);
    final theme = moduleTheme(module);
    return ContainerTransform(
      shape: RoundedRectangleBorder(borderRadius: BorderRadius.circular(12)),
      openColor: theme.colorScheme.surface,
      closedBuilder: (context, onOpen) =>
          _ModuleTile(module: module, onOpen: onOpen),
      // The page hangs on the top-level navigator: pass the color scheme along.
      openBuilder: (_) => Theme(
        data: theme,
        child: ModuleShell(
          module: module,
          api: widget.api,
          fileChooser: widget.fileChooser,
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
        actions: [MockBanner(visible: widget.api.isMock)],
      ),
      body: GridView.builder(
        padding: const EdgeInsets.all(16),
        gridDelegate: const SliverGridDelegateWithMaxCrossAxisExtent(
          // Two columns on a phone, more on wide devices. A fixed height instead
          // of an aspect ratio: on narrow screens the tiles would otherwise be
          // too low for their content.
          maxCrossAxisExtent: 240,
          mainAxisSpacing: 12,
          crossAxisSpacing: 12,
          mainAxisExtent: 200,
        ),
        itemCount: modules.length,
        itemBuilder: (context, i) {
          // Staggered: every tile starts a little later than the previous one.
          final start = (i * 0.12).clamp(0.0, 0.5);
          final animation = CurvedAnimation(
            parent: _fadeIn,
            curve: Interval(
              start,
              start + 0.5,
              curve: Easing.emphasizedDecelerate,
            ),
          );
          return FadeTransition(
            key: ValueKey('fade-in-${modules[i].id}'),
            opacity: animation,
            child: SlideTransition(
              position: Tween(
                begin: const Offset(0, 0.08),
                end: Offset.zero,
              ).animate(animation),
              child: _tile(modules[i]),
            ),
          );
        },
      ),
    );
  }
}

class _ModuleTile extends StatefulWidget {
  const _ModuleTile({required this.module, required this.onOpen});

  final ModuleDefinition module;

  /// `null` for inactive modules: the tile does not react.
  final VoidCallback? onOpen;

  @override
  State<_ModuleTile> createState() => _ModuleTileState();
}

class _ModuleTileState extends State<_ModuleTile> {
  /// While the finger rests on the tile: slightly scaled down.
  var _pressed = false;

  @override
  Widget build(BuildContext context) {
    final module = widget.module;
    // Every tile in the colors of its own module.
    final colors = moduleTheme(module).colorScheme;
    final textTheme = Theme.of(context).textTheme;
    final active = module.active;
    final foreground = active
        ? colors.onPrimaryContainer
        : colors.onSurfaceVariant.withValues(alpha: 0.6);

    return Semantics(
      button: true,
      enabled: active,
      label: active ? module.name : '${module.name}, bald verfügbar',
      excludeSemantics: true,
      child: AnimatedScale(
        key: ValueKey('press-${module.id}'),
        scale: _pressed ? 0.97 : 1,
        duration: motionDuration(context, Durations.short2),
        curve: Easing.standard,
        child: Card(
          clipBehavior: Clip.antiAlias,
          color: active ? null : colors.surfaceContainerHigh,
          child: Ink(
            // Active: more module color as a light gradient.
            decoration: active
                ? BoxDecoration(
                    gradient: LinearGradient(
                      begin: Alignment.topLeft,
                      end: Alignment.bottomRight,
                      colors: [
                        colors.primaryContainer,
                        colors.secondaryContainer,
                      ],
                    ),
                  )
                : null,
            child: InkWell(
              onTap: widget.onOpen,
              onHighlightChanged: active
                  ? (pressed) => setState(() => _pressed = pressed)
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
                            ? colors.primary
                            : colors.surfaceContainerHighest,
                        shape: BoxShape.circle,
                      ),
                      child: Icon(
                        module.icon,
                        size: 28,
                        color: active ? colors.onPrimary : foreground,
                      ),
                    ),
                    const Spacer(),
                    Text(
                      module.name,
                      style: textTheme.titleMedium?.copyWith(
                        color: foreground,
                        fontWeight: active ? FontWeight.w600 : null,
                      ),
                      maxLines: 1,
                      overflow: TextOverflow.ellipsis,
                    ),
                    const SizedBox(height: 4),
                    Text(
                      module.description,
                      style: textTheme.bodySmall?.copyWith(color: foreground),
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
                          color: colors.surfaceContainerHighest,
                          borderRadius: BorderRadius.circular(8),
                        ),
                        child: Text(
                          'Bald verfügbar',
                          // One line, so that the tile does not overflow even with
                          // large text or on narrow devices.
                          maxLines: 1,
                          overflow: TextOverflow.ellipsis,
                          style: textTheme.labelSmall?.copyWith(
                            color: colors.onSurfaceVariant,
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
