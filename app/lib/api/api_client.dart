import 'dart:typed_data';

import '../model/answer.dart';
import '../model/entry.dart';
import '../model/entry_patch.dart';
import '../model/server_status.dart';
import '../model/transcript.dart';

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
  /// recognition work. Calls no model.
  Future<ServerStatus> getStatus();

  /// Releases resources (e.g. the HTTP connection).
  void close();
}
