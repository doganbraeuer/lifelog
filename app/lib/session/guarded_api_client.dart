import 'dart:typed_data';

import '../api/api_client.dart';
import '../api/api_exception.dart';
import '../model/answer.dart';
import '../model/entry.dart';
import '../model/entry_patch.dart';
import '../model/server_status.dart';
import '../model/transcript.dart';
import '../model/user.dart';

/// Wraps an [ApiClient] (mock or HTTP) and reports session errors in one
/// place, so that no screen has to handle them itself:
///
/// - `401 UNAUTHORIZED` (token expired, revoked or account disabled) calls
///   [onUnauthorized]; the session then logs out and shows the login.
/// - `403 PASSWORD_CHANGE_REQUIRED` calls [onPasswordChangeRequired].
///
/// The error is still thrown, so the calling screen ends its loading state
/// as usual; the session replaces the screen right after.
///
/// [login], [logout] and [getStatus] are passed through unguarded: the
/// login answers wrong passwords with `INVALID_CREDENTIALS`, and a failed
/// logout ends the session anyway.
class GuardedApiClient implements ApiClient {
  GuardedApiClient(
    this.inner, {
    required this.onUnauthorized,
    required this.onPasswordChangeRequired,
  });

  final ApiClient inner;
  final void Function() onUnauthorized;
  final void Function() onPasswordChangeRequired;

  Future<T> _guard<T>(Future<T> Function() call) async {
    try {
      return await call();
    } on ApiException catch (e) {
      if (e.code == ErrorCode.unauthorized) {
        onUnauthorized();
      } else if (e.code == ErrorCode.passwordChangeRequired) {
        onPasswordChangeRequired();
      }
      rethrow;
    }
  }

  @override
  bool get isMock => inner.isMock;

  @override
  void close() => inner.close();

  @override
  void useToken(String? token) => inner.useToken(token);

  @override
  Future<List<EntrySummary>> listEntries({
    required String module,
    Sort sort = Sort.documentDate,
    SortOrder order = SortOrder.desc,
  }) =>
      _guard(() => inner.listEntries(module: module, sort: sort, order: order));

  @override
  Future<Entry> getEntry(String id) => _guard(() => inner.getEntry(id));

  @override
  Future<FileContent> getFile(String id) => _guard(() => inner.getFile(id));

  @override
  Future<Entry> upload(
    Uint8List bytes,
    String fileName, {
    required String module,
  }) => _guard(() => inner.upload(bytes, fileName, module: module));

  @override
  Future<Entry> importText(String text, {required String module}) =>
      _guard(() => inner.importText(text, module: module));

  @override
  Future<Entry> correct(String id, EntryPatch patch) =>
      _guard(() => inner.correct(id, patch));

  @override
  Future<void> deleteEntry(String id) => _guard(() => inner.deleteEntry(id));

  @override
  Future<Answer> ask(String question, {required String module}) =>
      _guard(() => inner.ask(question, module: module));

  @override
  Future<Transcript> transcribe(Uint8List bytes, String fileName) =>
      _guard(() => inner.transcribe(bytes, fileName));

  @override
  Future<ServerStatus> getStatus() => inner.getStatus();

  @override
  Future<LoginResult> login(String username, String password) =>
      inner.login(username, password);

  @override
  Future<void> logout() => inner.logout();

  @override
  Future<User> me() => _guard(inner.me);

  @override
  Future<User> updateDisplayName(String displayName) =>
      _guard(() => inner.updateDisplayName(displayName));

  @override
  Future<void> changePassword(String currentPassword, String newPassword) =>
      _guard(() => inner.changePassword(currentPassword, newPassword));

  @override
  Future<void> recordConsent(String version) =>
      _guard(() => inner.recordConsent(version));
}
