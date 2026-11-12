import 'package:flutter/material.dart';
import 'package:flutter_localizations/flutter_localizations.dart';

import 'api/api_client.dart';
import 'api/api_client_factory.dart';
import 'module/module_catalog.dart';
import 'platform/file_chooser.dart';
import 'screens/modules_screen.dart';

void main() {
  runApp(
    LifelogApp(
      api: erstelleApiClient(),
      dateiAuswahl: PluginDateiAuswahl(),
      katalog: ModulKatalog.standard(),
    ),
  );
}

class LifelogApp extends StatelessWidget {
  const LifelogApp({
    super.key,
    required this.api,
    required this.dateiAuswahl,
    required this.katalog,
  });

  /// Alle Backend-Aufrufe laufen über diesen Client (Mock oder HTTP).
  final ApiClient api;

  /// Kamera, Galerie und PDF-Auswahl. In Tests ein Fake.
  final DateiAuswahl dateiAuswahl;

  /// Die Module mit ihren Typen. In Tests evtl. ein eigener Katalog.
  final ModulKatalog katalog;

  @override
  Widget build(BuildContext context) {
    // Der Katalog liegt über der MaterialApp, damit jede Seite und jeder
    // Dialog ihn mit ModulKatalog.von(context) findet.
    return ModulKatalogScope(
      katalog: katalog,
      child: MaterialApp(
        title: 'Lifelog',
        // Neutrales Grundschema für den Screen „Bereiche“. Jedes Modul
        // bekommt beim Öffnen sein eigenes Schema aus seiner Farbe.
        theme: ThemeData(colorSchemeSeed: Colors.blueGrey),
        // Die App ist deutsch. Damit sind auch eingebaute Texte wie in der
        // Datumsauswahl („Abbrechen“, Monatsnamen) deutsch.
        localizationsDelegates: GlobalMaterialLocalizations.delegates,
        supportedLocales: const [Locale('de')],
        locale: const Locale('de'),
        home: BereicheScreen(api: api, dateiAuswahl: dateiAuswahl),
      ),
    );
  }
}
