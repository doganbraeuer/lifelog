import 'package:flutter/material.dart';
import 'package:flutter_localizations/flutter_localizations.dart';

import 'api/api_client.dart';
import 'api/api_client_auswahl.dart';
import 'plattform/datei_auswahl.dart';
import 'screens/start_screen.dart';

void main() {
  runApp(
    LifelogApp(api: erstelleApiClient(), dateiAuswahl: PluginDateiAuswahl()),
  );
}

class LifelogApp extends StatelessWidget {
  const LifelogApp({super.key, required this.api, required this.dateiAuswahl});

  /// Alle Backend-Aufrufe laufen über diesen Client (Mock oder HTTP).
  final ApiClient api;

  /// Kamera, Galerie und PDF-Auswahl. In Tests ein Fake.
  final DateiAuswahl dateiAuswahl;

  @override
  Widget build(BuildContext context) {
    return MaterialApp(
      title: 'Lifelog',
      theme: ThemeData(colorSchemeSeed: Colors.teal),
      // Die App ist deutsch. Damit sind auch eingebaute Texte wie in der
      // Datumsauswahl („Abbrechen“, Monatsnamen) deutsch.
      localizationsDelegates: GlobalMaterialLocalizations.delegates,
      supportedLocales: const [Locale('de')],
      locale: const Locale('de'),
      home: StartScreen(api: api, dateiAuswahl: dateiAuswahl),
    );
  }
}
