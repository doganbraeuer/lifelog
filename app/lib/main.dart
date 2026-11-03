import 'package:flutter/material.dart';

void main() {
  runApp(const LifelogApp());
}

/// Einstiegspunkt der App. Vorerst nur ein Platzhalter; die Screens
/// (Zeitleiste, Hochladen, Eintrag, Frage) kommen in den nächsten Etappen.
class LifelogApp extends StatelessWidget {
  const LifelogApp({super.key});

  @override
  Widget build(BuildContext context) {
    return MaterialApp(
      title: 'Lifelog',
      theme: ThemeData(colorSchemeSeed: Colors.teal),
      home: const Scaffold(
        body: Center(child: Text('Lifelog')),
      ),
    );
  }
}
