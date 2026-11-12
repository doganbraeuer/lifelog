import 'package:flutter/material.dart';

import '../api/api_client.dart';
import '../api/api_exception.dart';
import '../model/entry.dart';
import '../model/timeline_groups.dart';
import '../module/module_catalog.dart';
import '../widgets/motion.dart';
import '../widgets/container_transform.dart';
import '../widgets/error_view.dart';
import '../widgets/format.dart';
import '../widgets/mock_banner.dart';
import '../widgets/navigation.dart';
import '../widgets/skeleton.dart';
import 'entry_screen.dart';

/// Alle Einträge eines Moduls als Zeitachse mit Monatsabschnitten.
/// Sortierung und Richtung sind wählbar; sortiert wird im Backend, die App
/// zeigt die Liste in der gelieferten Reihenfolge und gruppiert sie nur.
class ZeitleisteScreen extends StatefulWidget {
  const ZeitleisteScreen({
    super.key,
    required this.api,
    required this.moduleId,
    this.zumHochladen,
  });

  final ApiClient api;

  /// Nur Einträge mit diesem `modul` werden gezeigt.
  final String moduleId;

  /// Wechselt zur Seite „Hochladen“ (Knopf im leeren Zustand).
  final VoidCallback? zumHochladen;

  @override
  State<ZeitleisteScreen> createState() => ZeitleisteScreenState();
}

class ZeitleisteScreenState extends State<ZeitleisteScreen> {
  var _sortierung = Sort.documentDate;
  var _richtung = SortOrder.desc;

  List<EntrySummary>? _eintraege;
  ApiException? _fehler;
  var _laedt = false;

  /// Zählt die Anfragen. Wechselt jemand schnell die Sortierung, kommen
  /// Antworten evtl. in falscher Reihenfolge an; dann zählt nur die letzte.
  var _anfrageNr = 0;

  @override
  void initState() {
    super.initState();
    neuLaden();
  }

  /// Lädt die Liste neu. Öffentlich, damit z. B. der Upload die Zeitleiste
  /// nach einem neuen Eintrag aktualisieren kann.
  Future<void> neuLaden() async {
    final nr = ++_anfrageNr;
    setState(() {
      _laedt = true;
      _fehler = null;
    });
    try {
      final entries = await widget.api.listEntries(
        module: widget.moduleId,
        sort: _sortierung,
        order: _richtung,
      );
      if (!mounted || nr != _anfrageNr) return;
      setState(() {
        // Das Backend liefert nur Einträge des Moduls (?modul=), sortiert.
        _eintraege = entries;
        _laedt = false;
      });
    } on ApiException catch (fehler) {
      if (!mounted || nr != _anfrageNr) return;
      setState(() {
        _fehler = fehler;
        _laedt = false;
      });
    }
  }

  void _sortierungWaehlen(Sort sort) {
    if (sort == _sortierung) return;
    _sortierung = sort;
    neuLaden();
  }

  void _richtungUmschalten() {
    _richtung = _richtung == SortOrder.desc ? SortOrder.asc : SortOrder.desc;
    neuLaden();
  }

  /// Öffnet einen Eintrag. Öffentlich, damit der Upload nach dem Hochladen
  /// (oder bei einem Duplikat) direkt den Eintrag zeigen kann.
  Future<void> eintragOeffnen(String entryId) async {
    await oeffneSeite<void>(
      context,
      EintragScreen(api: widget.api, entryId: entryId),
    );
    // Nach einer Korrektur kann sich z. B. das Datum geändert haben.
    if (mounted) neuLaden();
  }

  @override
  Widget build(BuildContext context) {
    final absteigend = _richtung == SortOrder.desc;
    return Scaffold(
      appBar: AppBar(
        leading: const BereicheKnopf(),
        title: const Text('Zeitleiste'),
        actions: [
          MockHinweis(sichtbar: widget.api.isMock),
          PopupMenuButton<Sort>(
            icon: const Icon(Icons.sort),
            tooltip: 'Sortieren nach',
            onSelected: _sortierungWaehlen,
            itemBuilder: (_) => [
              CheckedPopupMenuItem(
                value: Sort.documentDate,
                checked: _sortierung == Sort.documentDate,
                child: const Text('Nach Dokumentdatum'),
              ),
              CheckedPopupMenuItem(
                value: Sort.importedAt,
                checked: _sortierung == Sort.importedAt,
                child: const Text('Nach Importdatum'),
              ),
            ],
          ),
          IconButton(
            icon: Icon(absteigend ? Icons.arrow_downward : Icons.arrow_upward),
            tooltip: absteigend
                ? 'Neueste zuerst (tippen: älteste zuerst)'
                : 'Älteste zuerst (tippen: neueste zuerst)',
            onPressed: _richtungUmschalten,
          ),
        ],
        bottom: PreferredSize(
          preferredSize: const Size.fromHeight(28),
          child: Column(
            children: [
              Padding(
                padding: const EdgeInsets.only(bottom: 6),
                child: Text(
                  '${_sortierung == Sort.documentDate ? 'Dokumentdatum' : 'Importdatum'}'
                  ' · ${absteigend ? 'neueste zuerst' : 'älteste zuerst'}',
                  style: Theme.of(context).textTheme.bodySmall,
                ),
              ),
              // Ladebalken, wenn schon eine Liste da ist und neu geladen wird.
              SizedBox(
                height: 4,
                child: _laedt && _eintraege != null
                    ? const LinearProgressIndicator()
                    : null,
              ),
            ],
          ),
        ),
      ),
      body: _inhalt(),
    );
  }

  /// Weicher Wechsel zwischen Platzhalter, Liste, leerem Zustand und Fehler.
  Widget _inhalt() => Uebergang(child: _zustand());

  Widget _zustand() {
    final fehler = _fehler;
    if (fehler != null) {
      return FehlerAnzeige(
        key: const ValueKey('fehler'),
        fehler: fehler,
        erneut: neuLaden,
      );
    }
    final entries = _eintraege;
    if (entries == null) {
      // Erstes Laden: Platzhalter in der Form der Zeitachse.
      return const ZeitachsenPlatzhalter(key: ValueKey('laedt'));
    }
    return RefreshIndicator(
      key: ValueKey(entries.isEmpty ? 'leer' : 'liste'),
      onRefresh: neuLaden,
      child: entries.isEmpty
          ? _LeereZeitleiste(
              moduleId: widget.moduleId,
              zumHochladen: widget.zumHochladen,
            )
          : _zeitachse(entries),
    );
  }

  /// Zeitachse: Monatsüberschriften, links eine durchgehende Linie mit
  /// einem Punkt pro Eintrag.
  Widget _zeitachse(List<EntrySummary> entries) {
    final gruppen = groupTimeline(entries, _sortierung);
    // Die Eintrag-Ansicht hängt am obersten Navigator; sie bekommt das
    // Farbschema des Moduls mit (wie bei oeffneSeite).
    final themes = InheritedTheme.capture(
      from: context,
      to: Navigator.of(context).context,
    );
    final zeilen = <Widget>[];
    for (var g = 0; g < gruppen.length; g++) {
      final gruppe = gruppen[g];
      zeilen.add(
        _MonatsUeberschrift(
          key: ValueKey('monat-${gruppe.title}'),
          title: gruppe.title,
          linieOben: g > 0,
        ),
      );
      for (var i = 0; i < gruppe.entries.length; i++) {
        final entry = gruppe.entries[i];
        final letzter =
            g == gruppen.length - 1 && i == gruppe.entries.length - 1;
        zeilen.add(
          // Key nach Eintrag: Nach Löschen oder Umsortieren bleibt jede
          // Zeile mit ihrem Eintrag verbunden (auch der Rücklauf des
          // Container Transform landet am richtigen Platz).
          ContainerTransform(
            key: ValueKey('eintrag-${entry.id}'),
            geschlossen: (context, oeffnen) => _EintragZeile(
              entry: entry,
              zeigeImport: _sortierung == Sort.importedAt,
              linieUnten: !letzter,
              onTap: oeffnen,
            ),
            offen: (_) =>
                themes.wrap(EintragScreen(api: widget.api, entryId: entry.id)),
            // Nach einer Korrektur oder dem Löschen neu laden.
            beimSchliessen: () {
              if (mounted) neuLaden();
            },
          ),
        );
      }
    }
    return ListView(
      padding: const EdgeInsets.only(bottom: 24),
      children: zeilen,
    );
  }
}

/// Breite der linken Spalte mit Linie und Punkten.
const _achsenBreite = 64.0;

/// Zeichnet die senkrechte Linie der Zeitachse in der linken Spalte.
class _AchsenMaler extends CustomPainter {
  _AchsenMaler({required this.color, required this.oben, required this.unten});

  final Color color;

  /// Linie von oben bis zur Mitte bzw. von der Mitte bis unten zeichnen.
  final bool oben;
  final bool unten;

  @override
  void paint(Canvas canvas, Size size) {
    final x = size.width / 2;
    final y = size.height / 2;
    final stift = Paint()
      ..color = color
      ..strokeWidth = 2;
    if (oben) canvas.drawLine(Offset(x, 0), Offset(x, y), stift);
    if (unten) canvas.drawLine(Offset(x, y), Offset(x, size.height), stift);
  }

  @override
  bool shouldRepaint(_AchsenMaler alt) =>
      alt.color != color || alt.oben != oben || alt.unten != unten;
}

class _MonatsUeberschrift extends StatelessWidget {
  const _MonatsUeberschrift({
    super.key,
    required this.title,
    required this.linieOben,
  });

  final String title;
  final bool linieOben;

  @override
  Widget build(BuildContext context) {
    final theme = Theme.of(context);
    return Stack(
      children: [
        Positioned(
          left: 0,
          top: 0,
          bottom: 0,
          width: _achsenBreite,
          child: ExcludeSemantics(
            child: CustomPaint(
              painter: _AchsenMaler(
                color: theme.colorScheme.outlineVariant,
                oben: linieOben,
                unten: true,
              ),
              child: Center(
                // Kleiner Knoten, an dem der Monat beginnt.
                child: Container(
                  width: 10,
                  height: 10,
                  decoration: BoxDecoration(
                    color: theme.colorScheme.primary,
                    shape: BoxShape.circle,
                  ),
                ),
              ),
            ),
          ),
        ),
        Padding(
          padding: const EdgeInsets.fromLTRB(_achsenBreite, 20, 16, 8),
          child: Semantics(
            header: true,
            child: Text(
              title,
              style: theme.textTheme.titleSmall?.copyWith(
                color: theme.colorScheme.primary,
                fontWeight: FontWeight.w600,
              ),
            ),
          ),
        ),
      ],
    );
  }
}

class _EintragZeile extends StatelessWidget {
  const _EintragZeile({
    required this.entry,
    required this.zeigeImport,
    required this.linieUnten,
    required this.onTap,
  });

  final EntrySummary entry;

  /// Bei Sortierung nach Importdatum wird es mit angezeigt, damit die
  /// Reihenfolge nachvollziehbar ist.
  final bool zeigeImport;
  final bool linieUnten;
  final VoidCallback onTap;

  @override
  Widget build(BuildContext context) {
    final theme = Theme.of(context);
    final datum = entry.documentDate;
    final issuer = entry.issuer;
    final grau = theme.colorScheme.onSurfaceVariant;
    final catalog = ModuleCatalog.of(context);
    // Sanfte Töne aus der Typfarbe; sie kennzeichnet nur den Typ.
    final punkt = ColorScheme.fromSeed(
      seedColor: catalog.typeColor(entry.module, entry.type),
      brightness: theme.brightness,
    );

    return Stack(
      children: [
        Positioned(
          left: 0,
          top: 0,
          bottom: 0,
          width: _achsenBreite,
          child: ExcludeSemantics(
            child: CustomPaint(
              painter: _AchsenMaler(
                color: theme.colorScheme.outlineVariant,
                oben: true,
                unten: linieUnten,
              ),
              child: Center(
                child: Container(
                  width: 36,
                  height: 36,
                  decoration: BoxDecoration(
                    color: punkt.primaryContainer,
                    shape: BoxShape.circle,
                    // Ring in Hintergrundfarbe trennt den Punkt von der Linie.
                    border: Border.all(
                      color: theme.colorScheme.surface,
                      width: 3,
                    ),
                  ),
                  child: Icon(
                    catalog.typeIcon(entry.module, entry.type),
                    size: 18,
                    color: punkt.onPrimaryContainer,
                  ),
                ),
              ),
            ),
          ),
        ),
        Padding(
          padding: const EdgeInsets.only(left: _achsenBreite - 8),
          child: ListTile(
            contentPadding: const EdgeInsets.only(left: 8, right: 16),
            title: Text(catalog.typeName(entry.module, entry.type)),
            subtitle: Text(
              [
                issuer ?? 'Quelle unbekannt',
                if (zeigeImport)
                  'Importiert: ${formatZeitpunktLokal(entry.importedAt)}',
              ].join('\n'),
              maxLines: zeigeImport ? 3 : 2,
              overflow: TextOverflow.ellipsis,
            ),
            isThreeLine: zeigeImport,
            trailing: datum != null
                ? Text(datum.display)
                : Text(
                    'ohne Datum',
                    style: TextStyle(fontStyle: FontStyle.italic, color: grau),
                  ),
            onTap: onTap,
          ),
        ),
      ],
    );
  }
}

/// Leerer Zustand: großes Symbol, Hinweis und Knopf zum Hochladen.
class _LeereZeitleiste extends StatelessWidget {
  const _LeereZeitleiste({required this.moduleId, required this.zumHochladen});

  final String moduleId;
  final VoidCallback? zumHochladen;

  @override
  Widget build(BuildContext context) {
    final theme = Theme.of(context);
    final icon =
        ModuleCatalog.of(context).byId(moduleId)?.icon ?? Icons.inbox_outlined;
    // ListView statt Column, damit Pull-to-Refresh auch hier geht.
    return ListView(
      padding: const EdgeInsets.fromLTRB(32, 64, 32, 32),
      children: [
        ExcludeSemantics(
          child: Center(
            child: Container(
              width: 112,
              height: 112,
              decoration: BoxDecoration(
                color: theme.colorScheme.primaryContainer,
                shape: BoxShape.circle,
              ),
              child: Icon(
                icon,
                size: 56,
                color: theme.colorScheme.onPrimaryContainer,
              ),
            ),
          ),
        ),
        const SizedBox(height: 24),
        Text(
          'Noch keine Dokumente.',
          textAlign: TextAlign.center,
          style: theme.textTheme.titleMedium,
        ),
        const SizedBox(height: 8),
        Text(
          'Lade ein Foto, ein PDF oder einen Text hoch. '
          'Es erscheint dann hier in der Zeitleiste.',
          textAlign: TextAlign.center,
          style: theme.textTheme.bodyMedium?.copyWith(
            color: theme.colorScheme.onSurfaceVariant,
          ),
        ),
        if (zumHochladen != null) ...[
          const SizedBox(height: 24),
          Center(
            child: FilledButton.icon(
              onPressed: zumHochladen,
              icon: const Icon(Icons.upload_file),
              label: const Text('Dokument hochladen'),
            ),
          ),
        ],
      ],
    );
  }
}
