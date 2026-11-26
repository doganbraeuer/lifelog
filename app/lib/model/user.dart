import 'json_helpers.dart';

/// The logged-in user as `GET /me` returns it (docs/API.md 3.12).
class User {
  const User({
    required this.id,
    required this.username,
    required this.displayName,
    required this.mustChangePassword,
    required this.consentVersion,
    this.timeZone,
  });

  factory User.fromJson(Map<String, dynamic> json) => User(
    id: readString(json, 'id'),
    username: readString(json, 'username'),
    displayName: readString(json, 'displayName'),
    mustChangePassword: readBool(json, 'mustChangePassword'),
    consentVersion: readStringOrNull(json, 'consentVersion'),
    timeZone: readStringOrNull(json, 'timeZone'),
  );

  final String id;
  final String username;
  final String displayName;

  /// `true` until the start password has been changed.
  final bool mustChangePassword;

  /// Version of the consent text the user last agreed to; `null` if never.
  final String? consentVersion;

  /// IANA name of the user's time zone, e.g. `Europe/Vienna` (3.12, since
  /// 2026-11-23). Routines compute in it. `null` when the user comes from
  /// the login response, which has no `timeZone` (3.13).
  final String? timeZone;

  User copyWith({
    String? displayName,
    bool? mustChangePassword,
    String? consentVersion,
    String? timeZone,
  }) => User(
    id: id,
    username: username,
    displayName: displayName ?? this.displayName,
    mustChangePassword: mustChangePassword ?? this.mustChangePassword,
    consentVersion: consentVersion ?? this.consentVersion,
    timeZone: timeZone ?? this.timeZone,
  );

  @override
  String toString() => 'User($username)';
}

/// Response of `POST /auth/login` (docs/API.md 3.10).
///
/// [token] is a secret: it is kept like a password, never logged, and
/// [toString] leaves it out.
class LoginResult {
  const LoginResult({
    required this.token,
    required this.expiresAt,
    required this.user,
  });

  /// In the login response `mustChangePassword` sits next to `user`, not in
  /// it; it is moved into [user] here.
  factory LoginResult.fromJson(Map<String, dynamic> json) {
    final user = asObject(json['user'], 'user');
    return LoginResult(
      token: readString(json, 'token'),
      expiresAt: readTimestamp(json, 'expiresAt'),
      user: User.fromJson({
        ...user,
        'mustChangePassword': readBool(json, 'mustChangePassword'),
      }),
    );
  }

  final String token;

  /// 30 days after the login, UTC. Using the token does not extend it.
  final DateTime expiresAt;
  final User user;

  @override
  String toString() => 'LoginResult(${user.username}, token: ***)';
}
