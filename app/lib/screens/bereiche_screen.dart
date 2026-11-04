import 'package:flutter/material.dart';

import '../api/api_client.dart';
import '../module/modul_definition.dart';
import '../module/modul_katalog.dart';
import '../plattform/datei_auswahl.dart';
import '../widgets/mock_hinweis.dart';
import '../widgets/navigation.dart';
import 'modul_rahmen.dart';

/// Startscreen: eine Kachel pro Modul aus dem Modul-Katalog.
///
/// Aktive Module öffnen ihren Rahmen (Zeitleiste, Hochladen, Fragen) in
/// ihrem eigenen Farbschema. Inaktive zeigen „Bald verfügbar“.
class BereicheScreen extends StatelessWidget {
  const BereicheScreen({
    super.key,
    required this.api,
    required this.dateiAuswahl,
  });

  final ApiClient api;
  final DateiAuswahl dateiAuswahl;

  void _oeffnen(BuildContext context, ModulDefinition modul) {
    Navigator.of(context).push(
      MaterialPageRoute<void>(
        builder: (_) => Theme(
          data: modulTheme(modul),
          child: ModulRahmen(
            modul: modul,
            api: api,
            dateiAuswahl: dateiAuswahl,
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
        actions: [MockHinweis(sichtbar: api.istMock)],
      ),
      body: GridView.builder(
        padding: const EdgeInsets.all(16),
        gridDelegate: const SliverGridDelegateWithMaxCrossAxisExtent(
          // Auf dem Handy zwei Spalten, auf breiten Geräten mehr.
          maxCrossAxisExtent: 240,
          mainAxisSpacing: 12,
          crossAxisSpacing: 12,
          childAspectRatio: 0.9,
        ),
        itemCount: module.length,
        itemBuilder: (context, i) => _ModulKachel(
          modul: module[i],
          oeffnen: module[i].aktiv ? () => _oeffnen(context, module[i]) : null,
        ),
      ),
    );
  }
}

class _ModulKachel extends StatelessWidget {
  const _ModulKachel({required this.modul, required this.oeffnen});

  final ModulDefinition modul;

  /// `null` bei inaktiven Modulen: Die Kachel reagiert nicht.
  final VoidCallback? oeffnen;

  @override
  Widget build(BuildContext context) {
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
      child: Card(
        color: aktiv ? farben.primaryContainer : farben.surfaceContainerHigh,
        clipBehavior: Clip.antiAlias,
        child: InkWell(
          onTap: oeffnen,
          child: Padding(
            padding: const EdgeInsets.all(16),
            child: Column(
              crossAxisAlignment: CrossAxisAlignment.start,
              children: [
                Icon(modul.symbol, size: 40, color: vordergrund),
                const Spacer(),
                Text(
                  modul.name,
                  style: textTheme.titleMedium?.copyWith(color: vordergrund),
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
    );
  }
}
