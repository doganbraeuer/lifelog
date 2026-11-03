import 'package:flutter/material.dart';

import 'api/api_client.dart';
import 'api/api_client_auswahl.dart';

void main() {
  runApp(LifelogApp(api: erstelleApiClient()));
}

/// Einstiegspunkt der App. Vorerst nur ein Platzhalter; die Screens
/// (Zeitleiste, Hochladen, Eintrag, Frage) kommen in den nächsten Etappen.
class LifelogApp extends StatelessWidget {
  const LifelogApp({super.key, required this.api});

  /// Alle Backend-Aufrufe laufen über diesen Client (Mock oder HTTP).
  final ApiClient api;

  @override
  Widget build(BuildContext context) {
    return MaterialApp(
      title: 'Lifelog',
      theme: ThemeData(colorSchemeSeed: Colors.teal),
      home: Scaffold(
        body: Center(
          child: Column(
            mainAxisSize: MainAxisSize.min,
            children: [
              const Text('Lifelog'),
              const SizedBox(height: 8),
              Text(
                api.istMock
                    ? 'Datenquelle: Mock'
                    : 'Datenquelle: Backend $apiBaseUrl',
              ),
            ],
          ),
        ),
      ),
    );
  }
}
