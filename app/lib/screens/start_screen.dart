import 'package:flutter/material.dart';

import '../api/api_client.dart';
import 'platzhalter_screen.dart';
import 'zeitleiste_screen.dart';

/// Rahmen der App mit der Navigationsleiste unten.
///
/// `IndexedStack` hält alle drei Bereiche am Leben. Wechselt man den
/// Bereich, bleiben Scroll-Position und Sortierung der Zeitleiste erhalten.
class StartScreen extends StatefulWidget {
  const StartScreen({super.key, required this.api});

  final ApiClient api;

  @override
  State<StartScreen> createState() => _StartScreenState();
}

class _StartScreenState extends State<StartScreen> {
  var _index = 0;

  @override
  Widget build(BuildContext context) {
    return Scaffold(
      body: IndexedStack(
        index: _index,
        children: [
          ZeitleisteScreen(api: widget.api),
          const PlatzhalterScreen(titel: 'Hochladen'),
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
