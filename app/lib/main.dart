import 'package:flutter/material.dart';
import 'package:flutter_localizations/flutter_localizations.dart';

import 'api/api_client.dart';
import 'api/api_client_factory.dart';
import 'module/module_catalog.dart';
import 'platform/file_chooser.dart';
import 'platform/voice_recorder.dart';
import 'screens/change_password_screen.dart';
import 'screens/consent_screen.dart';
import 'screens/login_screen.dart';
import 'screens/modules_screen.dart';
import 'session/local_data.dart';
import 'session/session_controller.dart';
import 'session/session_store.dart';
import 'widgets/error_view.dart';

void main() {
  runApp(
    LifelogApp(
      api: createApiClient(),
      fileChooser: PluginFileChooser(),
      voiceRecorder: PluginVoiceRecorder(),
      catalog: ModuleCatalog.defaults(),
      sessionStore: SecureSessionStore(),
      localData: PluginLocalData(),
    ),
  );
}

class LifelogApp extends StatefulWidget {
  const LifelogApp({
    super.key,
    required this.api,
    required this.fileChooser,
    required this.voiceRecorder,
    required this.catalog,
    required this.sessionStore,
    required this.localData,
  });

  /// Every backend call goes through this client (mock or HTTP).
  final ApiClient api;

  /// Camera, gallery and PDF picker. A fake in tests.
  final FileChooser fileChooser;

  /// Microphone for spoken notes. A fake in tests.
  final VoiceRecorder voiceRecorder;

  /// The modules with their types. Tests may pass their own catalog.
  final ModuleCatalog catalog;

  /// Where the token is kept between starts. A fake in tests.
  final SessionStore sessionStore;

  /// Temporary files and image cache, deleted on logout. A fake in tests.
  final LocalData localData;

  @override
  State<LifelogApp> createState() => _LifelogAppState();
}

class _LifelogAppState extends State<LifelogApp> {
  late final _session = SessionController(
    api: widget.api,
    store: widget.sessionStore,
    localData: widget.localData,
  );

  @override
  void initState() {
    super.initState();
    _session.start();
  }

  @override
  void dispose() {
    _session.dispose();
    super.dispose();
  }

  /// The screen for the current state of the session.
  Widget _home() => switch (_session.phase) {
    SessionPhase.loading => const Scaffold(
      body: Center(child: CircularProgressIndicator()),
    ),
    SessionPhase.offline => Scaffold(
      appBar: AppBar(title: const Text('Lifelog')),
      body: ErrorView(error: _session.error!, onRetry: _session.retry),
    ),
    SessionPhase.signedOut => const LoginScreen(),
    SessionPhase.mustChangePassword => const ChangePasswordScreen(
      mandatory: true,
    ),
    SessionPhase.needsConsent => const ConsentScreen(),
    SessionPhase.ready => ModulesScreen(
      api: _session.api,
      fileChooser: widget.fileChooser,
      voiceRecorder: widget.voiceRecorder,
    ),
  };

  @override
  Widget build(BuildContext context) {
    // Catalog and session sit above the MaterialApp, so that every page and
    // every dialog finds them with ModuleCatalog.of / SessionScope.of.
    return ModuleCatalogScope(
      catalog: widget.catalog,
      child: SessionScope(
        controller: _session,
        child: ListenableBuilder(
          listenable: _session,
          builder: (context, _) => MaterialApp(
            // A new key after logout or an ended session: the navigator is
            // rebuilt, every open page closes and no data of the previous
            // session stays in memory.
            key: ValueKey(_session.generation),
            title: 'Lifelog',
            // Neutral base scheme for the „Bereiche“ screen. Every module
            // gets its own scheme from its color when it is opened.
            theme: ThemeData(colorSchemeSeed: Colors.blueGrey),
            // The app is German. This makes built-in texts such as in the
            // date picker („Abbrechen“, month names) German too.
            localizationsDelegates: GlobalMaterialLocalizations.delegates,
            supportedLocales: const [Locale('de')],
            locale: const Locale('de'),
            home: _home(),
          ),
        ),
      ),
    );
  }
}
