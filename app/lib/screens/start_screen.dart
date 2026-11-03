import 'package:flutter/material.dart';

import '../api/api_client.dart';
import '../model/eintrag.dart';
import '../plattform/datei_auswahl.dart';
import 'platzhalter_screen.dart';
import 'upload_screen.dart';
import 'zeitleiste_screen.dart';

/// Rahmen der App mit der Navigationsleiste unten.
///
/// `IndexedStack` hält alle drei Bereiche am Leben. Wechselt man den
/// Bereich, bleiben Scroll-Position und Sortierung der Zeitleiste erhalten.
class StartScreen extends StatefulWidget {
  const StartScreen({super.key, required this.api, required this.dateiAuswahl});

  final ApiClient api;
  final DateiAuswahl dateiAuswahl;

  @override
  State<StartScreen> createState() => _StartScreenState();
}

class _StartScreenState extends State<StartScreen> {
  var _index = 0;

  /// Zugriff auf die Zeitleiste, um sie neu zu laden und Einträge zu öffnen.
  final _zeitleiste = GlobalKey<ZeitleisteScreenState>();

  /// Wechselt zur Zeitleiste und öffnet dort den Eintrag. So führt „Zurück“
  /// aus dem Eintrag in die (neu geladene) Zeitleiste.
  void _eintragOeffnen(String eintragId) {
    setState(() => _index = 0);
    _zeitleiste.currentState?.eintragOeffnen(eintragId);
  }

  void _hochgeladen(Eintrag eintrag) {
    _zeitleiste.currentState?.neuLaden();
    _eintragOeffnen(eintrag.id);
  }

  @override
  Widget build(BuildContext context) {
    return Scaffold(
      body: IndexedStack(
        index: _index,
        children: [
          ZeitleisteScreen(key: _zeitleiste, api: widget.api),
          UploadScreen(
            api: widget.api,
            dateiAuswahl: widget.dateiAuswahl,
            hochgeladen: _hochgeladen,
            eintragOeffnen: _eintragOeffnen,
          ),
          const PlatzhalterScreen(titel: 'Fragen'),
        ],
      ),
      bottomNavigationBar: NavigationBar(
        selectedIndex: _index,
        onDestinationSelected: (i) => setState(() => _index = i),
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
