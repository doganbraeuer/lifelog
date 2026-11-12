import 'package:flutter/material.dart';

/// Kleines Kennzeichen in der AppBar, solange die App mit Mock-Daten läuft.
/// So verwechselt niemand die Beispieldaten mit echten Ergebnissen.
class MockHinweis extends StatelessWidget {
  const MockHinweis({super.key, required this.sichtbar});

  final bool sichtbar;

  @override
  Widget build(BuildContext context) {
    if (!sichtbar) return const SizedBox.shrink();
    final farben = Theme.of(context).colorScheme;
    return Tooltip(
      message: 'Die App zeigt Beispieldaten, kein Backend verbunden.',
      child: Container(
        margin: const EdgeInsets.symmetric(horizontal: 8),
        padding: const EdgeInsets.symmetric(horizontal: 8, vertical: 2),
        decoration: BoxDecoration(
          color: farben.tertiaryContainer,
          borderRadius: BorderRadius.circular(8),
        ),
        child: Text(
          'Mock',
          style: TextStyle(color: farben.onTertiaryContainer, fontSize: 12),
        ),
      ),
    );
  }
}
