import 'dart:typed_data';

import '../model/answer.dart';
import '../model/entry.dart';
import '../model/entry_patch.dart';
import '../model/plain_date.dart';
import '../model/routine.dart';
import '../model/server_status.dart';
import '../model/transcript.dart';
import '../model/user.dart';

/// Sort field of the timeline. The names are the values of the query
/// parameter `sort`.
enum Sort { documentDate, importedAt }

/// Sort order. The names are the values of `order`.
enum SortOrder { desc, asc }

/// Maximum upload size according to docs/API.md: 10 MB.
const maxFileSize = 10 * 1024 * 1024;

/// Maximum length of a text import according to docs/API.md (3.1), in
/// characters.
const maxTextLength = 100000;

/// Maximum length of a recording according to docs/API.md (3.9), in
/// seconds. Longer ones are rejected with `413 AUDIO_TOO_LONG`.
const maxRecordingSeconds = 300;

/// Every call to the backend goes through this interface.
///
/// There are two implementations: [MockApiClient] with sample data and
/// [HttpApiClient] for the real backend. `createApiClient()` picks one based
/// on `--dart-define=API_BASE_URL`.
///
/// Every error is thrown as an [ApiException].
///
/// According to the API the module (`module`, e.g. `health`) is optional
/// and defaults to `health`. The app still always passes it explicitly, so
/// that no area ends up in health by accident.
abstract interface class ApiClient {
  /// `true` for the mock. The app then shows a notice.
  bool get isMock;

  /// `GET /entries`: all entries of a module in the short form, sorted by
  /// the backend.
  Future<List<EntrySummary>> listEntries({
    required String module,
    Sort sort = Sort.documentDate,
    SortOrder order = SortOrder.desc,
  });

  /// `GET /entries/{id}`: one full entry.
  Future<Entry> getEntry(String id);

  /// `GET /entries/{id}/file`: the original file.
  Future<FileContent> getFile(String id);

  /// `POST /entries` with the field `file`: upload a file and have it
  /// extracted.
  Future<Entry> upload(
    Uint8List bytes,
    String fileName, {
    required String module,
  });

  /// `POST /entries` with the field `text`: import document content as
  /// text. The backend stores it as the original file `text.txt`.
  Future<Entry> importText(String text, {required String module});

  /// `PATCH /entries/{id}`: correct recognized values.
  Future<Entry> correct(String id, EntryPatch patch);

  /// `DELETE /entries/{id}`: delete an entry with its original file and
  /// chunks.
  Future<void> deleteEntry(String id);

  /// `POST /questions`: ask a question, answer with sources. Only documents
  /// of the module are searched.
  Future<Answer> ask(String question, {required String module});

  /// `POST /transcriptions` with the field `audio`: turn a recording into
  /// text. Nothing is stored; the app saves the checked text with
  /// [importText]. Can take up to about two minutes.
  Future<Transcript> transcribe(Uint8List bytes, String fileName);

  /// `GET /status`: whether the backend, the database and the speech
  /// recognition work. Calls no model. Needs no token.
  Future<ServerStatus> getStatus();

  // -----------------------------------------------------------------------
  // Accounts and login (docs/API.md section 1 "Login and tokens", 3.10–3.15)
  //
  // Every call except [getStatus] and [login] sends the token as
  // `Authorization: Bearer <token>`. The client only keeps the token in
  // memory; storing it is the job of the session (lib/session/).
  // -----------------------------------------------------------------------

  /// Sets the token for all further calls, e.g. the one restored from secure
  /// storage at start. `null` forgets it.
  void useToken(String? token);

  /// `POST /auth/login`. Needs no token. On success the client uses the new
  /// token right away. The password is only sent, never kept.
  Future<LoginResult> login(String username, String password);

  /// `POST /auth/logout`: ends the token of this device. The client forgets
  /// the token in any case, also if the call fails.
  Future<void> logout();

  /// `GET /me`: the logged-in user.
  Future<User> me();

  /// `PATCH /me`: change the display name (1–50 characters after trimming).
  Future<User> updateDisplayName(String displayName);

  /// `POST /me/password`: replace the password. Ends all other tokens of the
  /// account; the token of this device stays valid.
  Future<void> changePassword(String currentPassword, String newPassword);

  /// `PUT /me/consent`: record that the user agreed to the consent text
  /// with this version.
  Future<void> recordConsent(String version);

  /// `PATCH /me` with `timeZone`: the IANA name of the phone's time zone
  /// (3.13). Routines compute in it; points in time stay UTC.
  Future<User> updateTimeZone(String timeZone);

  // -----------------------------------------------------------------------
  // Routines (docs/API.md 3.16–3.27)
  //
  // The backend expands the schedules, keeps the logs and computes status
  // and progress; the app shows them and plans local reminders from the
  // occurrences. No routine endpoint calls a model.
  // -----------------------------------------------------------------------

  /// `GET /routines`: all routines of the module in every status, sorted by
  /// `createdAt`, then `id`.
  Future<List<Routine>> listRoutines({required String module});

  /// `POST /routines`: create a routine.
  Future<Routine> createRoutine(NewRoutine routine);

  /// `GET /routines/{id}`.
  Future<Routine> getRoutine(String id);

  /// `PATCH /routines/{id}`: only the fields in [patch] change.
  Future<Routine> updateRoutine(String id, RoutinePatch patch);

  /// `POST /routines/{id}/pause`: paused from today until [until]
  /// (inclusive), or until resumed if [until] is `null`.
  Future<Routine> pauseRoutine(String id, {PlainDate? until});

  /// `POST /routines/{id}/resume`: today's occurrences are due again.
  Future<Routine> resumeRoutine(String id);

  /// `DELETE /routines/{id}`: the routine with all its logs.
  Future<void> deleteRoutine(String id);

  /// `GET /routines/occurrences`: the occurrences of all routines of the
  /// module on the days [from] to [to]. Without [from] the backend takes
  /// today in the user's time zone, without [to] six days later.
  Future<OccurrenceList> listOccurrences({
    required String module,
    PlainDate? from,
    PlainDate? to,
  });

  /// `GET /routines/{id}/logs`: newest first. Without [to] the backend
  /// takes today, without [from] 29 days before [to].
  Future<List<RoutineLog>> listRoutineLogs(
    String id, {
    PlainDate? from,
    PlainDate? to,
  });

  /// `POST /routines/{id}/logs`: done or skipped, for an occurrence or
  /// without one. A second log for the same occurrence fails with
  /// `409 ROUTINE_LOG_EXISTS`.
  Future<RoutineLog> addRoutineLog(String id, NewRoutineLog log);

  /// `DELETE /routines/{id}/logs/{logId}`: the occurrence is open again.
  Future<void> deleteRoutineLog(String id, String logId);

  /// Releases resources (e.g. the HTTP connection).
  void close();
}
