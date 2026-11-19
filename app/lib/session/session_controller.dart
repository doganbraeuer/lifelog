import 'package:flutter/services.dart';
import 'package:flutter/widgets.dart';

import '../api/api_client.dart';
import '../api/api_exception.dart';
import '../model/user.dart';
import 'consent_text.dart';
import 'guarded_api_client.dart';
import 'local_data.dart';
import 'session_store.dart';

/// What the app shows, decided by the session.
enum SessionPhase {
  /// Reading the token and asking `GET /me`.
  loading,

  /// `GET /me` failed without `401` (no network, server error). The token is
  /// kept; the app offers „Erneut versuchen“.
  offline,

  /// No valid token: login screen.
  signedOut,

  /// Start password: „Eigenes Passwort festlegen“.
  mustChangePassword,

  /// The consent notice in its current version is not agreed to yet.
  needsConsent,

  /// Logged in: the modules as usual.
  ready,
}

/// Notice on the login screen after the session ended without a logout.
const reloginNotice = 'Bitte melde dich erneut an.';

/// Holds the login state above the MaterialApp and decides which screen is
/// shown (see [SessionPhase]).
///
/// Screens call the backend through [api], which reports `401` and `403
/// PASSWORD_CHANGE_REQUIRED` back here ([GuardedApiClient]). That is the one
/// place where a session ends; no screen handles it on its own.
class SessionController extends ChangeNotifier {
  SessionController({
    required ApiClient api,
    required this._store,
    required this._localData,
    DateTime Function()? clock,
  }) : _raw = api,
       _clock = clock ?? (() => DateTime.now().toUtc()) {
    this.api = GuardedApiClient(
      api,
      onUnauthorized: handleUnauthorized,
      onPasswordChangeRequired: handlePasswordChangeRequired,
    );
  }

  /// Without the guard: for start, login and logout, which handle their
  /// errors themselves.
  final ApiClient _raw;
  final SessionStore _store;
  final LocalData _localData;
  final DateTime Function() _clock;

  /// For all screens.
  late final ApiClient api;

  SessionPhase get phase => _phase;
  var _phase = SessionPhase.loading;

  /// The logged-in user; `null` while signed out.
  User? get user => _user;
  User? _user;

  /// Why `GET /me` failed in [SessionPhase.offline].
  ApiException? get error => _error;
  ApiException? _error;

  /// Notice for the login screen, e.g. [reloginNotice].
  String? get notice => _notice;
  String? _notice;

  /// Changes whenever the screens of a session must all close (logout,
  /// session ended, password change demanded). The app uses it as the key
  /// of the MaterialApp, which rebuilds the navigator and drops every open
  /// page and its data.
  int get generation => _generation;
  var _generation = 0;

  /// `true` while a session is being ended, so that several `401` at once
  /// end it only once.
  var _ending = false;

  /// Reads the stored token and checks it with `GET /me`.
  Future<void> start() async {
    _setPhase(SessionPhase.loading);
    final stored = await _store.read();
    if (stored == null) {
      _setPhase(SessionPhase.signedOut);
      return;
    }
    if (stored.isExpired(_clock())) {
      await _store.clear();
      _notice = reloginNotice;
      _setPhase(SessionPhase.signedOut);
      return;
    }
    _raw.useToken(stored.token);
    await _loadUser();
  }

  /// „Erneut versuchen“ after [SessionPhase.offline].
  Future<void> retry() async {
    _setPhase(SessionPhase.loading);
    await _loadUser();
  }

  Future<void> _loadUser() async {
    try {
      _user = await _raw.me();
      _error = null;
      _setPhase(_phaseFor(_user!));
    } on ApiException catch (e) {
      if (e.code == ErrorCode.unauthorized) {
        await _endSession(reloginNotice);
      } else {
        // Keep the token: without network the session may still be valid.
        _error = e;
        _setPhase(SessionPhase.offline);
      }
    }
  }

  static SessionPhase _phaseFor(User user) {
    if (user.mustChangePassword) return SessionPhase.mustChangePassword;
    if (user.consentVersion != consentVersion) return SessionPhase.needsConsent;
    return SessionPhase.ready;
  }

  /// `POST /auth/login`. Errors (wrong password, lockout) are thrown for the
  /// login screen. The password is passed on and not kept anywhere.
  Future<void> login(String username, String password) async {
    final result = await _raw.login(username, password);
    try {
      await _store.write(
        StoredSession(token: result.token, expiresAt: result.expiresAt),
      );
    } on PlatformException {
      // The secure storage failed: the login still works, but only until
      // the app is closed.
    }
    _user = result.user;
    _notice = null;
    _error = null;
    _setPhase(_phaseFor(result.user));
  }

  /// `POST /me/password`. On success the start password is replaced and all
  /// other devices are logged out; this one stays logged in.
  Future<void> changePassword(String current, String next) async {
    await api.changePassword(current, next);
    final user = _user;
    if (user == null) return;
    _user = user.copyWith(mustChangePassword: false);
    _setPhase(_phaseFor(_user!));
  }

  /// „Zustimmen“: `PUT /me/consent` with the version of the app's text.
  Future<void> recordConsent() async {
    await api.recordConsent(consentVersion);
    final user = _user;
    if (user == null) return;
    _user = user.copyWith(consentVersion: consentVersion);
    _setPhase(_phaseFor(_user!));
  }

  /// `PATCH /me`.
  Future<void> updateDisplayName(String displayName) async {
    _user = await api.updateDisplayName(displayName);
    notifyListeners();
  }

  /// „Abmelden“ (also „Nicht jetzt“ on the consent notice): ends the token
  /// on the server (errors ignored), then deletes everything local.
  Future<void> logout() async {
    try {
      await _raw.logout();
    } on ApiException {
      // The app logs out in any case (3.11).
    }
    await _endSession(null);
  }

  /// A call answered `401 UNAUTHORIZED`.
  void handleUnauthorized() {
    if (_phase == SessionPhase.signedOut) return;
    _endSession(reloginNotice);
  }

  /// A call answered `403 PASSWORD_CHANGE_REQUIRED` (e.g. an administrator
  /// reset the password while the app was open).
  void handlePasswordChangeRequired() {
    final user = _user;
    if (user == null || _phase == SessionPhase.mustChangePassword) return;
    _user = user.copyWith(mustChangePassword: true);
    _setPhase(SessionPhase.mustChangePassword);
  }

  Future<void> _endSession(String? notice) async {
    if (_ending) return;
    _ending = true;
    try {
      _raw.useToken(null);
      // Delete first, then show the login, so that a quick new login can
      // never be deleted by this one.
      await _store.clear();
      await _localData.clear();
      _user = null;
      _error = null;
      _notice = notice;
      _generation++;
      _setPhase(SessionPhase.signedOut);
    } finally {
      _ending = false;
    }
  }

  void _setPhase(SessionPhase phase) {
    // Leaving the modules: close every page that is open on top.
    if (_phase == SessionPhase.ready && phase != SessionPhase.ready) {
      _generation++;
    }
    _phase = phase;
    notifyListeners();
  }
}

/// Makes the [SessionController] available below the MaterialApp, like
/// `ModuleCatalogScope`. Widgets that use [of] rebuild when it changes.
class SessionScope extends InheritedNotifier<SessionController> {
  const SessionScope({
    super.key,
    required SessionController controller,
    required super.child,
  }) : super(notifier: controller);

  static SessionController of(BuildContext context) {
    final scope = context.dependOnInheritedWidgetOfExactType<SessionScope>();
    assert(scope != null, 'No SessionScope above this widget');
    return scope!.notifier!;
  }
}
