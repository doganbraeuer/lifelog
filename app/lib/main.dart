import 'dart:isolate';
import 'dart:ui';

import 'package:flutter/material.dart';
import 'package:flutter_localizations/flutter_localizations.dart';

import 'api/api_client.dart';
import 'api/api_client_factory.dart';
import 'module/module_catalog.dart';
import 'platform/device_time_zone.dart';
import 'platform/file_chooser.dart';
import 'platform/reminders.dart';
import 'platform/voice_recorder.dart';
import 'reminders/reminder_action.dart';
import 'reminders/routine_sync.dart';
import 'screens/change_password_screen.dart';
import 'screens/consent_screen.dart';
import 'screens/login_screen.dart';
import 'screens/modules_screen.dart';
import 'session/local_data.dart';
import 'session/session_controller.dart';
import 'session/session_store.dart';
import 'widgets/error_view.dart';

void main() {
  final api = createApiClient();
  runApp(
    LifelogApp(
      api: api,
      fileChooser: PluginFileChooser(),
      voiceRecorder: PluginVoiceRecorder(),
      catalog: ModuleCatalog.defaults(),
      sessionStore: SecureSessionStore(),
      localData: PluginLocalData(),
      deviceTimeZone: PluginDeviceTimeZone(),
      // Reminder buttons in the background need the real backend; the mock
      // lives only in this isolate's memory.
      reminders: PluginReminders(
        backgroundButtons: api.isMock ? null : reminderButtonInBackground,
      ),
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
    required this.deviceTimeZone,
    required this.reminders,
    this.clock,
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

  /// The phone's IANA time zone, sent to the backend for routines. A fake
  /// in tests.
  final DeviceTimeZone deviceTimeZone;

  /// Local reminders of routines. A fake in tests.
  final Reminders reminders;

  /// "Now" for planning reminders; the phone's clock if `null`. Tests pass
  /// the mock's clock.
  final DateTime Function()? clock;

  @override
  State<LifelogApp> createState() => _LifelogAppState();
}

class _LifelogAppState extends State<LifelogApp> {
  late final _session = SessionController(
    api: widget.api,
    store: widget.sessionStore,
    localData: widget.localData,
  );

  late final _sync = RoutineSync(
    session: _session,
    deviceTimeZone: widget.deviceTimeZone,
    reminders: widget.reminders,
    modules: [
      for (final module in widget.catalog.modules)
        if (module.active) module.id,
    ],
    clock: widget.clock,
  );

  /// Back in the foreground: the time zone may have changed (travel), the
  /// day may be a new one, and a reminder button may have logged something.
  late final _lifecycle = AppLifecycleListener(onResume: _refreshIfReady);

  var _wasReady = false;

  @override
  void initState() {
    super.initState();
    _session.addListener(_sessionChanged);
    _lifecycle; // create it
    _initReminders();
    _session.start();
  }

  /// Taps on reminders go to the sync; the one that started the app too.
  /// A reminder button that logged in the background (another isolate)
  /// reports it through a named port, so that the screens reload.
  Future<void> _initReminders() async {
    try {
      IsolateNameServer.removePortNameMapping(reminderPortName);
      IsolateNameServer.registerPortWithName(
        _backgroundLogs.sendPort,
        reminderPortName,
      );
      _backgroundLogs.listen((_) => _sync.loggedInBackground());
      await widget.reminders.initialize(onTap: _sync.handleTap);
      final tap = await widget.reminders.launchTap();
      if (tap != null && mounted) _sync.handleTap(tap);
    } on Exception {
      // Without reminders the app works as before.
    }
  }

  final _backgroundLogs = ReceivePort();

  @override
  void dispose() {
    IsolateNameServer.removePortNameMapping(reminderPortName);
    _backgroundLogs.close();
    _lifecycle.dispose();
    _session.removeListener(_sessionChanged);
    _session.dispose();
    _sync.dispose();
    super.dispose();
  }

  /// At the start of every session (also after a login or the consent) the
  /// sync runs. When a session ends (logout, 401, …) or the app starts
  /// without one, every reminder is removed: none of a previous user stays
  /// on the phone.
  void _sessionChanged() {
    final phase = _session.phase;
    final ready = phase == SessionPhase.ready;
    if (ready && !_wasReady) _sync.refresh();
    if (!ready && (_wasReady || phase == SessionPhase.signedOut)) {
      _sync.clear();
    }
    _wasReady = ready;
  }

  void _refreshIfReady() {
    if (_session.phase == SessionPhase.ready) _sync.refresh();
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
    // Catalog, session and routine sync sit above the MaterialApp, so that
    // every page and every dialog finds them with ModuleCatalog.of,
    // SessionScope.of and RoutineSyncScope.maybeOf.
    return ModuleCatalogScope(
      catalog: widget.catalog,
      child: SessionScope(
        controller: _session,
        child: RoutineSyncScope(
          sync: _sync,
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
      ),
    );
  }
}
