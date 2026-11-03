import 'package:flutter/material.dart';

import 'api/api_client.dart';
import 'api/api_client_auswahl.dart';
import 'screens/start_screen.dart';

void main() {
  runApp(LifelogApp(api: erstelleApiClient()));
}

class LifelogApp extends StatelessWidget {
  const LifelogApp({super.key, required this.api});

  /// Alle Backend-Aufrufe laufen über diesen Client (Mock oder HTTP).
  final ApiClient api;

  @override
  Widget build(BuildContext context) {
    return MaterialApp(
      title: 'Lifelog',
      theme: ThemeData(colorSchemeSeed: Colors.teal),
      home: StartScreen(api: api),
    );
  }
}
