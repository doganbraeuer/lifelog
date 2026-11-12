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
      api: createApiClient(),
      fileChooser: PluginFileChooser(),
      catalog: ModuleCatalog.defaults(),
    ),
  );
}

class LifelogApp extends StatelessWidget {
  const LifelogApp({
    super.key,
    required this.api,
    required this.fileChooser,
    required this.catalog,
  });

  /// Every backend call goes through this client (mock or HTTP).
  final ApiClient api;

  /// Camera, gallery and PDF picker. A fake in tests.
  final FileChooser fileChooser;

  /// The modules with their types. Tests may pass their own catalog.
  final ModuleCatalog catalog;

  @override
  Widget build(BuildContext context) {
    // The catalog sits above the MaterialApp, so that every page and every
    // dialog finds it with ModuleCatalog.of(context).
    return ModuleCatalogScope(
      catalog: catalog,
      child: MaterialApp(
        title: 'Lifelog',
        // Neutral base scheme for the „Bereiche“ screen. Every module gets its
        // own scheme from its color when it is opened.
        theme: ThemeData(colorSchemeSeed: Colors.blueGrey),
        // The app is German. This makes built-in texts such as in the date
        // picker („Abbrechen“, month names) German too.
        localizationsDelegates: GlobalMaterialLocalizations.delegates,
        supportedLocales: const [Locale('de')],
        locale: const Locale('de'),
        home: ModulesScreen(api: api, fileChooser: fileChooser),
      ),
    );
  }
}
