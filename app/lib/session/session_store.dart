import 'package:flutter/services.dart';
import 'package:flutter_secure_storage/flutter_secure_storage.dart';

/// What the app keeps of a login: the token and until when it is valid.
/// Never the password.
class StoredSession {
  const StoredSession({required this.token, required this.expiresAt});

  /// Secret like a password: never logged, never shown. [toString] leaves
  /// it out.
  final String token;

  /// UTC. After this point the app asks for a new login without asking the
  /// server first.
  final DateTime expiresAt;

  bool isExpired(DateTime now) => !now.isBefore(expiresAt);

  @override
  String toString() => 'StoredSession(token: ***, expiresAt: $expiresAt)';
}

/// Where the token lives between app starts. Behind an interface so that
/// tests can use a fake instead of the platform's secure storage.
abstract interface class SessionStore {
  /// The stored session, or `null` if there is none or it cannot be read.
  Future<StoredSession?> read();

  Future<void> write(StoredSession session);

  /// Deletes the token. Afterwards [read] returns `null`.
  Future<void> clear();
}

/// [SessionStore] in the platform's secure storage
/// (package flutter_secure_storage): on Android encrypted with a key from
/// the Android Keystore, on iOS in the Keychain. Nothing goes into normal
/// settings (SharedPreferences).
class SecureSessionStore implements SessionStore {
  SecureSessionStore({FlutterSecureStorage? storage})
    : _storage =
          storage ??
          const FlutterSecureStorage(
            // Default: RSA OAEP + AES-GCM with a key in the Android Keystore.
            aOptions: AndroidOptions(),
            // Only on this device and only after the first unlock: the token
            // never moves to another device or into an iCloud backup.
            iOptions: IOSOptions(
              accessibility: KeychainAccessibility.first_unlock_this_device,
            ),
          );

  final FlutterSecureStorage _storage;

  static const _tokenKey = 'session_token';
  static const _expiresAtKey = 'session_expires_at';

  @override
  Future<StoredSession?> read() async {
    try {
      final token = await _storage.read(key: _tokenKey);
      final expiresAt = DateTime.tryParse(
        await _storage.read(key: _expiresAtKey) ?? '',
      );
      if (token == null || token.isEmpty || expiresAt == null) return null;
      return StoredSession(token: token, expiresAt: expiresAt.toUtc());
    } on PlatformException {
      // E.g. after a restore onto another device the key is missing and the
      // value cannot be decrypted. Then there is no usable token: delete it
      // and ask for a new login.
      await clear();
      return null;
    }
  }

  @override
  Future<void> write(StoredSession session) async {
    await _storage.write(key: _tokenKey, value: session.token);
    await _storage.write(
      key: _expiresAtKey,
      value: session.expiresAt.toUtc().toIso8601String(),
    );
  }

  @override
  Future<void> clear() async {
    try {
      await _storage.delete(key: _tokenKey);
      await _storage.delete(key: _expiresAtKey);
    } on PlatformException {
      // Nothing to delete, or the storage is broken; either way no token.
    }
  }
}
