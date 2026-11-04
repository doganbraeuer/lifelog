import 'package:flutter/material.dart';

import '../api/api_client.dart';
import '../api/api_fehler.dart';
import '../model/eintrag.dart';
import '../module/modul_katalog.dart';
import '../widgets/fehler_anzeige.dart';
import '../widgets/format.dart';
import '../widgets/mock_hinweis.dart';
import '../widgets/navigation.dart';
import 'eintrag_screen.dart';

/// Alle Einträge eines Moduls als Liste. Sortierung und Richtung sind
/// wählbar; sortiert wird im Backend, die App zeigt die Liste in der
/// gelieferten Reihenfolge.
class ZeitleisteScreen extends StatefulWidget {
  const ZeitleisteScreen({super.key, required this.api, required this.modulId});

  final ApiClient api;

  /// Nur Einträge mit diesem `modul` werden gezeigt.
  final String modulId;

  @override
  State<ZeitleisteScreen> createState() => ZeitleisteScreenState();
}

class ZeitleisteScreenState extends State<ZeitleisteScreen> {
  var _sortierung = Sortierung.dokumentdatum;
  var _richtung = Richtung.absteigend;

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
        // Die API kennt (noch) keinen Filter nach Modul, deshalb filtert die
        // App selbst. Die Reihenfolge des Backends bleibt dabei erhalten.
        _eintraege = [
          for (final e in eintraege)
            if (e.modul == widget.modulId) e,
        ];
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
    _richtung = _richtung == Richtung.absteigend
        ? Richtung.aufsteigend
        : Richtung.absteigend;
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
    final absteigend = _richtung == Richtung.absteigend;
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
                value: Sortierung.dokumentdatum,
                checked: _sortierung == Sortierung.dokumentdatum,
                child: const Text('Nach Dokumentdatum'),
              ),
              CheckedPopupMenuItem(
                value: Sortierung.importdatum,
                checked: _sortierung == Sortierung.importdatum,
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
                  '${_sortierung == Sortierung.dokumentdatum ? 'Dokumentdatum' : 'Importdatum'}'
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

  Widget _inhalt() {
    final fehler = _fehler;
    if (fehler != null) {
      return FehlerAnzeige(fehler: fehler, erneut: neuLaden);
    }
    final eintraege = _eintraege;
    if (eintraege == null) {
      return const Center(child: CircularProgressIndicator());
    }
    return RefreshIndicator(
      onRefresh: neuLaden,
      child: eintraege.isEmpty
          ? ListView(
              // ListView statt Text, damit Pull-to-Refresh auch hier geht.
              children: const [
                Padding(
                  padding: EdgeInsets.all(32),
                  child: Text(
                    'Noch keine Dokumente.\n'
                    'Unter „Hochladen“ kannst du das erste hinzufügen.',
                    textAlign: TextAlign.center,
                  ),
                ),
              ],
            )
          : ListView.separated(
              itemCount: eintraege.length,
              separatorBuilder: (_, _) => const Divider(height: 1),
              itemBuilder: (_, i) => _EintragKachel(
                eintrag: eintraege[i],
                zeigeImport: _sortierung == Sortierung.importdatum,
                onTap: () => eintragOeffnen(eintraege[i].id),
              ),
            ),
    );
  }
}

class _EintragKachel extends StatelessWidget {
  const _EintragKachel({
    required this.eintrag,
    required this.zeigeImport,
    required this.onTap,
  });

  final EintragKurz eintrag;

  /// Bei Sortierung nach Importdatum wird es mit angezeigt, damit die
  /// Reihenfolge nachvollziehbar ist.
  final bool zeigeImport;
  final VoidCallback onTap;

  @override
  Widget build(BuildContext context) {
    final datum = eintrag.dokumentdatum;
    final quelle = eintrag.quelle;
    final grau = Theme.of(context).colorScheme.onSurfaceVariant;
    final katalog = ModulKatalog.von(context);
    return ListTile(
      leading: Icon(katalog.typSymbol(eintrag.modul, eintrag.typ)),
      title: Text(katalog.typName(eintrag.modul, eintrag.typ)),
      subtitle: Text(
        [
          quelle ?? 'Quelle unbekannt',
          if (zeigeImport)
            'Importiert: ${formatZeitpunktLokal(eintrag.importdatum)}',
        ].join('\n'),
      ),
      isThreeLine: zeigeImport,
      trailing: datum != null
          ? Text(datum.anzeige)
          : Text(
              'ohne Datum',
              style: TextStyle(fontStyle: FontStyle.italic, color: grau),
            ),
      onTap: onTap,
    );
  }
}
