import 'package:flutter/material.dart';

import '../api/api_client.dart';
import '../model/eintrag.dart';
import '../module/modul_definition.dart';
import '../plattform/datei_auswahl.dart';
import '../widgets/navigation.dart';
import 'eintrag_screen.dart';
import 'frage_screen.dart';
import 'upload_screen.dart';
import 'zeitleiste_screen.dart';

/// Rahmen eines Moduls mit der Navigationsleiste unten (Zeitleiste,
/// Hochladen, Fragen). Alle Module nutzen diesen Rahmen; das Farbschema
/// setzt der Screen „Bereiche“ beim Öffnen.
///
/// Die drei Seiten liegen in einer `PageView` (Wischen oder Navigationsleiste)
/// und bleiben dabei am Leben, damit ihr Zustand erhalten bleibt.
class ModulRahmen extends StatefulWidget {
  const ModulRahmen({
    super.key,
    required this.modul,
    required this.api,
    required this.dateiAuswahl,
  });

  final ModulDefinition modul;
  final ApiClient api;
  final DateiAuswahl dateiAuswahl;

  @override
  State<ModulRahmen> createState() => _ModulRahmenState();
}

class _ModulRahmenState extends State<ModulRahmen> {
  /// Seite, die in der Navigationsleiste markiert ist.
  var _index = 0;

  final _seiten = PageController();

  /// Ziel, solange nach einem Tipp in der Navigationsleiste animiert wird.
  /// Beim Sprung von Seite 0 zu 2 meldet die PageView unterwegs auch
  /// Seite 1; ohne diese Sperre würde die Leiste kurz „Hochladen“ zeigen.
  int? _animiertZu;

  /// Zugriff auf die Zeitleiste, um sie neu zu laden und Einträge zu öffnen.
  /// Funktioniert auch, wenn gerade eine andere Seite sichtbar ist, weil
  /// alle Seiten am Leben bleiben (siehe [_AmLeben]).
  final _zeitleiste = GlobalKey<ZeitleisteScreenState>();

  @override
  void dispose() {
    _seiten.dispose();
    super.dispose();
  }

  /// Tipp in der Navigationsleiste: Leiste sofort umstellen, Seite animiert
  /// hinterher.
  Future<void> _zeigeSeite(int index) async {
    if (index == _index && _animiertZu == null) return;
    setState(() => _index = index);
    _animiertZu = index;
    await _seiten.animateToPage(
      index,
      duration: const Duration(milliseconds: 300),
      curve: Curves.easeOutCubic,
    );
    if (_animiertZu == index) _animiertZu = null;
  }

  /// Seitenwechsel durch Wischen (oder am Ende einer Animation).
  void _seiteGewechselt(int index) {
    if (_animiertZu != null && index != _animiertZu) return;
    if (index != _index) setState(() => _index = index);
  }

  /// Wechselt zur Zeitleiste und öffnet dort den Eintrag. So führt „Zurück“
  /// aus dem Eintrag in die (neu geladene) Zeitleiste.
  void _eintragOeffnen(String eintragId) {
    _zeigeSeite(0);
    _zeitleiste.currentState?.eintragOeffnen(eintragId);
  }

  void _hochgeladen(Eintrag eintrag) {
    _zeitleiste.currentState?.neuLaden();
    _eintragOeffnen(eintrag.id);
  }

  /// Öffnet einen Eintrag über dem aktuellen Bereich, ohne zur Zeitleiste
  /// zu wechseln. So führt „Zurück“ z. B. wieder zur Antwort auf die Frage.
  Future<void> _eintragDarueberOeffnen(String eintragId) async {
    await oeffneSeite<void>(
      context,
      EintragScreen(api: widget.api, eintragId: eintragId),
    );
    // Falls dort etwas korrigiert wurde.
    _zeitleiste.currentState?.neuLaden();
  }

  @override
  Widget build(BuildContext context) {
    return Scaffold(
      // Wischen nach links/rechts wechselt die Seite.
      body: PageView(
        controller: _seiten,
        onPageChanged: _seiteGewechselt,
        children: [
          _AmLeben(
            child: ZeitleisteScreen(
              key: _zeitleiste,
              api: widget.api,
              modulId: widget.modul.id,
            ),
          ),
          _AmLeben(
            child: UploadScreen(
              api: widget.api,
              dateiAuswahl: widget.dateiAuswahl,
              hochgeladen: _hochgeladen,
              eintragOeffnen: _eintragOeffnen,
            ),
          ),
          _AmLeben(
            child: FrageScreen(
              api: widget.api,
              modulId: widget.modul.id,
              eintragOeffnen: _eintragDarueberOeffnen,
            ),
          ),
        ],
      ),
      bottomNavigationBar: NavigationBar(
        selectedIndex: _index,
        onDestinationSelected: _zeigeSeite,
        destinations: const [
          NavigationDestination(
            icon: Icon(Icons.timeline_outlined),
            selectedIcon: Icon(Icons.timeline),
            label: 'Zeitleiste',
          ),
          NavigationDestination(
            icon: Icon(Icons.upload_file_outlined),
            selectedIcon: Icon(Icons.upload_file),
            label: 'Hochladen',
          ),
          NavigationDestination(
            icon: Icon(Icons.question_answer_outlined),
            selectedIcon: Icon(Icons.question_answer),
            label: 'Fragen',
          ),
        ],
      ),
    );
  }
}

/// Hält eine Seite der PageView am Leben, auch wenn sie nicht sichtbar ist.
///
/// Ohne das würde die PageView unsichtbare Seiten abbauen: Sortierung,
/// Scroll-Position, gewählte Datei, laufender Upload und letzte Antwort
/// gingen verloren, und `_zeitleiste.currentState` wäre `null`.
class _AmLeben extends StatefulWidget {
  const _AmLeben({required this.child});

  final Widget child;

  @override
  State<_AmLeben> createState() => _AmLebenState();
}

class _AmLebenState extends State<_AmLeben> with AutomaticKeepAliveClientMixin {
  @override
  bool get wantKeepAlive => true;

  @override
  Widget build(BuildContext context) {
    super.build(context); // Pflicht beim Mixin: meldet „am Leben halten“.
    return widget.child;
  }
}
