import 'package:flutter/material.dart';

import '../api/api_client.dart';
import '../api/api_fehler.dart';
import '../model/eintrag.dart';
import '../model/zeitleisten_gruppen.dart';
import '../module/modul_katalog.dart';
import '../widgets/bewegung.dart';
import '../widgets/container_transform.dart';
import '../widgets/fehler_anzeige.dart';
import '../widgets/format.dart';
import '../widgets/mock_hinweis.dart';
import '../widgets/navigation.dart';
import '../widgets/platzhalter.dart';
import 'eintrag_screen.dart';

/// Alle Einträge eines Moduls als Zeitachse mit Monatsabschnitten.
/// Sortierung und Richtung sind wählbar; sortiert wird im Backend, die App
/// zeigt die Liste in der gelieferten Reihenfolge und gruppiert sie nur.
class ZeitleisteScreen extends StatefulWidget {
  const ZeitleisteScreen({
    super.key,
    required this.api,
    required this.modulId,
    this.zumHochladen,
  });

  final ApiClient api;

  /// Nur Einträge mit diesem `modul` werden gezeigt.
  final String modulId;

  /// Wechselt zur Seite „Hochladen“ (Knopf im leeren Zustand).
  final VoidCallback? zumHochladen;

  @override
  State<ZeitleisteScreen> createState() => ZeitleisteScreenState();
}

class ZeitleisteScreenState extends State<ZeitleisteScreen> {
  var _sortierung = Sortierung.documentDate;
  var _richtung = Richtung.desc;

  List<EintragKurz>? _eintraege;
  ApiFehler? _fehler;
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
      final eintraege = await widget.api.eintraege(
        modul: widget.modulId,
        sortierung: _sortierung,
        richtung: _richtung,
      );
      if (!mounted || nr != _anfrageNr) return;
      setState(() {
        // Das Backend liefert nur Einträge des Moduls (?modul=), sortiert.
        _eintraege = eintraege;
        _laedt = false;
      });
    } on ApiFehler catch (fehler) {
      if (!mounted || nr != _anfrageNr) return;
      setState(() {
        _fehler = fehler;
        _laedt = false;
      });
    }
  }

  void _sortierungWaehlen(Sortierung sortierung) {
    if (sortierung == _sortierung) return;
    _sortierung = sortierung;
    neuLaden();
  }

  void _richtungUmschalten() {
    _richtung = _richtung == Richtung.desc
        ? Richtung.asc
        : Richtung.desc;
    neuLaden();
  }

  /// Öffnet einen Eintrag. Öffentlich, damit der Upload nach dem Hochladen
  /// (oder bei einem Duplikat) direkt den Eintrag zeigen kann.
  Future<void> eintragOeffnen(String eintragId) async {
    await oeffneSeite<void>(
      context,
      EintragScreen(api: widget.api, eintragId: eintragId),
    );
    // Nach einer Korrektur kann sich z. B. das Datum geändert haben.
    if (mounted) neuLaden();
  }

  @override
  Widget build(BuildContext context) {
    final absteigend = _richtung == Richtung.desc;
    return Scaffold(
      appBar: AppBar(
        leading: const BereicheKnopf(),
        title: const Text('Zeitleiste'),
        actions: [
          MockHinweis(sichtbar: widget.api.istMock),
          PopupMenuButton<Sortierung>(
            icon: const Icon(Icons.sort),
            tooltip: 'Sortieren nach',
            onSelected: _sortierungWaehlen,
            itemBuilder: (_) => [
              CheckedPopupMenuItem(
                value: Sortierung.documentDate,
                checked: _sortierung == Sortierung.documentDate,
                child: const Text('Nach Dokumentdatum'),
              ),
              CheckedPopupMenuItem(
                value: Sortierung.importedAt,
                checked: _sortierung == Sortierung.importedAt,
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
                  '${_sortierung == Sortierung.documentDate ? 'Dokumentdatum' : 'Importdatum'}'
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
    final eintraege = _eintraege;
    if (eintraege == null) {
      // Erstes Laden: Platzhalter in der Form der Zeitachse.
      return const ZeitachsenPlatzhalter(key: ValueKey('laedt'));
    }
    return RefreshIndicator(
      key: ValueKey(eintraege.isEmpty ? 'leer' : 'liste'),
      onRefresh: neuLaden,
      child: eintraege.isEmpty
          ? _LeereZeitleiste(
              modulId: widget.modulId,
              zumHochladen: widget.zumHochladen,
            )
          : _zeitachse(eintraege),
    );
  }

  /// Zeitachse: Monatsüberschriften, links eine durchgehende Linie mit
  /// einem Punkt pro Eintrag.
  Widget _zeitachse(List<EintragKurz> eintraege) {
    final gruppen = gruppiereZeitleiste(eintraege, _sortierung);
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
          key: ValueKey('monat-${gruppe.titel}'),
          titel: gruppe.titel,
          linieOben: g > 0,
        ),
      );
      for (var i = 0; i < gruppe.eintraege.length; i++) {
        final eintrag = gruppe.eintraege[i];
        final letzter =
            g == gruppen.length - 1 && i == gruppe.eintraege.length - 1;
        zeilen.add(
          // Key nach Eintrag: Nach Löschen oder Umsortieren bleibt jede
          // Zeile mit ihrem Eintrag verbunden (auch der Rücklauf des
          // Container Transform landet am richtigen Platz).
          ContainerTransform(
            key: ValueKey('eintrag-${eintrag.id}'),
            geschlossen: (context, oeffnen) => _EintragZeile(
              eintrag: eintrag,
              zeigeImport: _sortierung == Sortierung.importedAt,
              linieUnten: !letzter,
              onTap: oeffnen,
            ),
            offen: (_) => themes.wrap(
              EintragScreen(api: widget.api, eintragId: eintrag.id),
            ),
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
  _AchsenMaler({required this.farbe, required this.oben, required this.unten});

  final Color farbe;

  /// Linie von oben bis zur Mitte bzw. von der Mitte bis unten zeichnen.
  final bool oben;
  final bool unten;

  @override
  void paint(Canvas canvas, Size size) {
    final x = size.width / 2;
    final y = size.height / 2;
    final stift = Paint()
      ..color = farbe
      ..strokeWidth = 2;
    if (oben) canvas.drawLine(Offset(x, 0), Offset(x, y), stift);
    if (unten) canvas.drawLine(Offset(x, y), Offset(x, size.height), stift);
  }

  @override
  bool shouldRepaint(_AchsenMaler alt) =>
      alt.farbe != farbe || alt.oben != oben || alt.unten != unten;
}

class _MonatsUeberschrift extends StatelessWidget {
  const _MonatsUeberschrift({
    super.key,
    required this.titel,
    required this.linieOben,
  });

  final String titel;
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
                farbe: theme.colorScheme.outlineVariant,
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
              titel,
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
    required this.eintrag,
    required this.zeigeImport,
    required this.linieUnten,
    required this.onTap,
  });

  final EintragKurz eintrag;

  /// Bei Sortierung nach Importdatum wird es mit angezeigt, damit die
  /// Reihenfolge nachvollziehbar ist.
  final bool zeigeImport;
  final bool linieUnten;
  final VoidCallback onTap;

  @override
  Widget build(BuildContext context) {
    final theme = Theme.of(context);
    final datum = eintrag.dokumentdatum;
    final quelle = eintrag.quelle;
    final grau = theme.colorScheme.onSurfaceVariant;
    final katalog = ModulKatalog.von(context);
    // Sanfte Töne aus der Typfarbe; sie kennzeichnet nur den Typ.
    final punkt = ColorScheme.fromSeed(
      seedColor: katalog.typFarbe(eintrag.modul, eintrag.typ),
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
                farbe: theme.colorScheme.outlineVariant,
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
                    katalog.typSymbol(eintrag.modul, eintrag.typ),
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
            title: Text(katalog.typName(eintrag.modul, eintrag.typ)),
            subtitle: Text(
              [
                quelle ?? 'Quelle unbekannt',
                if (zeigeImport)
                  'Importiert: ${formatZeitpunktLokal(eintrag.importdatum)}',
              ].join('\n'),
              maxLines: zeigeImport ? 3 : 2,
              overflow: TextOverflow.ellipsis,
            ),
            isThreeLine: zeigeImport,
            trailing: datum != null
                ? Text(datum.anzeige)
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
  const _LeereZeitleiste({required this.modulId, required this.zumHochladen});

  final String modulId;
  final VoidCallback? zumHochladen;

  @override
  Widget build(BuildContext context) {
    final theme = Theme.of(context);
    final symbol =
        ModulKatalog.von(context).nachId(modulId)?.symbol ??
        Icons.inbox_outlined;
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
                symbol,
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
