import 'dart:async';
import 'dart:convert';
import 'dart:typed_data';

import 'package:http/http.dart' as http;
import 'package:http_parser/http_parser.dart';

import '../model/answer.dart';
import '../model/entry.dart';
import '../model/entry_patch.dart';
import '../model/json_helpers.dart';
import '../model/plain_date.dart';
import '../model/routine.dart';
import '../model/server_status.dart';
import '../model/transcript.dart';
import '../model/user.dart';
import 'api_client.dart';
import 'api_exception.dart';
import 'mime_type.dart';

/// Talks to the Go backend over HTTP (docs/API.md).
class HttpApiClient implements ApiClient {
  /// [baseUrl] without `/api/v1`, e.g. `http://10.0.2.2:8080`.
  /// In tests [client] can be replaced by a `MockClient`.
  HttpApiClient({required String baseUrl, http.Client? client})
    : _base = '${_withoutTrailingSlash(baseUrl.trim())}/api/v1',
      _client = client ?? http.Client();

  /// For quick calls (list, entry, correction).
  static const defaultTimeout = Duration(seconds: 20);

  /// For questions (language model) and files (can be large).
  static const longTimeout = Duration(seconds: 60);

  /// Upload: the backend needs up to about 60 s, the API asks for at least
  /// 90 s.
  static const uploadTimeout = Duration(seconds: 90);

  /// Transcription: the backend has a time limit of about 120 s, the API
  /// asks for at least 130 s (3.9).
  static const transcriptionTimeout = Duration(seconds: 130);

  final String _base;
  final http.Client _client;

  /// Token for `Authorization: Bearer`. Only in memory; it is never logged
  /// and never put into an [ApiException].
  String? _token;

  /// For the two public endpoints `/status` and the login: without token.
  static const _jsonHeaders = {
    'Content-Type': 'application/json; charset=utf-8',
    'Accept': 'application/json',
  };

  /// [_jsonHeaders] plus the token. Every endpoint except `/status` and the
  /// login needs it (docs/API.md section 1).
  Map<String, String> get _authJsonHeaders => {..._jsonHeaders, ..._authHeader};

  /// Only the token header (original file and multipart requests).
  Map<String, String> get _authHeader => {
    if (_token case final token?) 'Authorization': 'Bearer $token',
  };

  @override
  bool get isMock => false;

  @override
  void close() => _client.close();

  @override
  Future<List<EntrySummary>> listEntries({
    required String module,
    Sort sort = Sort.documentDate,
    SortOrder order = SortOrder.desc,
  }) async {
    final uri = _uri('/entries').replace(
      queryParameters: {
        'module': module,
        'sort': sort.name,
        'order': order.name,
      },
    );
    final response = await _send(
      () => _client.get(uri, headers: _authJsonHeaders),
      defaultTimeout,
    );
    return _read(
      response,
      (json) => readList(json, 'entries', EntrySummary.fromJson),
    );
  }

  @override
  Future<Entry> getEntry(String id) async {
    final response = await _send(
      () => _client.get(_uri(_entryPath(id)), headers: _authJsonHeaders),
      defaultTimeout,
    );
    return _read(response, Entry.fromJson);
  }

  @override
  Future<FileContent> getFile(String id) async {
    final response = await _send(
      () => _client.get(_uri('${_entryPath(id)}/file'), headers: _authHeader),
      longTimeout,
    );
    final contentType = response.headers['content-type'];
    final mimeType = contentType == null
        ? 'application/octet-stream'
        : contentType.split(';').first.trim().toLowerCase();
    return FileContent(bytes: response.bodyBytes, mimeType: mimeType);
  }

  @override
  Future<Entry> upload(
    Uint8List bytes,
    String fileName, {
    required String module,
  }) async {
    final request = http.MultipartRequest('POST', _uri('/entries'))
      ..headers['Accept'] = 'application/json'
      ..headers.addAll(_authHeader)
      ..fields['module'] = module
      ..files.add(
        http.MultipartFile.fromBytes(
          'file',
          bytes,
          filename: fileName,
          contentType: MediaType.parse(
            detectMimeType(bytes) ?? 'application/octet-stream',
          ),
        ),
      );
    return _sendForm(request);
  }

  @override
  Future<Entry> importText(String text, {required String module}) {
    // Same endpoint as the upload, only the field `text` instead of `file`.
    // MultipartRequest sends form fields as UTF-8.
    final request = http.MultipartRequest('POST', _uri('/entries'))
      ..headers['Accept'] = 'application/json'
      ..headers.addAll(_authHeader)
      ..fields['text'] = text
      ..fields['module'] = module;
    return _sendForm(request);
  }

  /// Sends an upload or a text import. The backend needs up to about 60 s.
  Future<Entry> _sendForm(http.MultipartRequest request) async {
    final response = await _send(
      () async => http.Response.fromStream(await _client.send(request)),
      uploadTimeout,
    );
    return _read(response, Entry.fromJson);
  }

  @override
  Future<Entry> correct(String id, EntryPatch patch) async {
    final response = await _send(
      () => _client.patch(
        _uri(_entryPath(id)),
        headers: _authJsonHeaders,
        body: jsonEncode(patch.toJson()),
      ),
      defaultTimeout,
    );
    return _read(response, Entry.fromJson);
  }

  @override
  Future<void> deleteEntry(String id) async {
    // Success is 204 without a body; errors come as JSON as always.
    await _send(
      () => _client.delete(_uri(_entryPath(id)), headers: _authJsonHeaders),
      defaultTimeout,
    );
  }

  @override
  Future<Answer> ask(String question, {required String module}) async {
    final response = await _send(
      () => _client.post(
        _uri('/questions'),
        headers: _authJsonHeaders,
        body: jsonEncode({'question': question, 'module': module}),
      ),
      longTimeout,
    );
    return _read(response, Answer.fromJson);
  }

  @override
  Future<Transcript> transcribe(Uint8List bytes, String fileName) async {
    final request = http.MultipartRequest('POST', _uri('/transcriptions'))
      ..headers['Accept'] = 'application/json'
      ..headers.addAll(_authHeader)
      ..files.add(
        http.MultipartFile.fromBytes(
          'audio',
          bytes,
          filename: fileName,
          contentType: MediaType.parse(
            detectAudioType(bytes) ?? 'application/octet-stream',
          ),
        ),
      );
    final response = await _send(
      () async => http.Response.fromStream(await _client.send(request)),
      transcriptionTimeout,
    );
    return _read(response, Transcript.fromJson);
  }

  @override
  Future<ServerStatus> getStatus() async {
    // 503 (database unreachable) comes with the normal status body, not in
    // the error format (3.8). It still tells whether speech works.
    final response = await _send(
      () => _client.get(_uri('/status'), headers: _jsonHeaders),
      defaultTimeout,
      alsoAccept: const {503},
    );
    return _read(response, ServerStatus.fromJson);
  }

  // ---------------------------------------------------------------------
  // Accounts and login (3.10–3.15)
  // ---------------------------------------------------------------------

  @override
  void useToken(String? token) => _token = token;

  @override
  Future<LoginResult> login(String username, String password) async {
    // Public endpoint: deliberately without the token (_jsonHeaders).
    final response = await _send(
      () => _client.post(
        _uri('/auth/login'),
        headers: _jsonHeaders,
        body: jsonEncode({'username': username, 'password': password}),
      ),
      defaultTimeout,
    );
    final result = _read(response, LoginResult.fromJson);
    _token = result.token;
    return result;
  }

  @override
  Future<void> logout() async {
    try {
      // 204 without a body.
      await _send(
        () => _client.post(_uri('/auth/logout'), headers: _authJsonHeaders),
        defaultTimeout,
      );
    } finally {
      // The app logs out in any case, also if the call fails (3.11).
      _token = null;
    }
  }

  @override
  Future<User> me() async {
    final response = await _send(
      () => _client.get(_uri('/me'), headers: _authJsonHeaders),
      defaultTimeout,
    );
    return _read(response, User.fromJson);
  }

  @override
  Future<User> updateDisplayName(String displayName) async {
    final response = await _send(
      () => _client.patch(
        _uri('/me'),
        headers: _authJsonHeaders,
        body: jsonEncode({'displayName': displayName}),
      ),
      defaultTimeout,
    );
    return _read(response, User.fromJson);
  }

  @override
  Future<void> changePassword(
    String currentPassword,
    String newPassword,
  ) async {
    // 204 without a body. bcrypt makes the backend slower than usual, the
    // default timeout is still plenty.
    await _send(
      () => _client.post(
        _uri('/me/password'),
        headers: _authJsonHeaders,
        body: jsonEncode({
          'currentPassword': currentPassword,
          'newPassword': newPassword,
        }),
      ),
      defaultTimeout,
    );
  }

  @override
  Future<void> recordConsent(String version) async {
    await _send(
      () => _client.put(
        _uri('/me/consent'),
        headers: _authJsonHeaders,
        body: jsonEncode({'version': version}),
      ),
      defaultTimeout,
    );
  }

  @override
  Future<User> updateTimeZone(String timeZone) async {
    final response = await _send(
      () => _client.patch(
        _uri('/me'),
        headers: _authJsonHeaders,
        body: jsonEncode({'timeZone': timeZone}),
      ),
      defaultTimeout,
    );
    return _read(response, User.fromJson);
  }

  // ---------------------------------------------------------------------
  // Routines (3.16–3.27)
  // ---------------------------------------------------------------------

  @override
  Future<List<Routine>> listRoutines({required String module}) async {
    final uri = _uri('/routines').replace(queryParameters: {'module': module});
    final response = await _send(
      () => _client.get(uri, headers: _authJsonHeaders),
      defaultTimeout,
    );
    return _read(
      response,
      (json) => readList(json, 'routines', Routine.fromJson),
    );
  }

  @override
  Future<Routine> createRoutine(NewRoutine routine) =>
      _sendRoutine('POST', '/routines', routine.toJson());

  @override
  Future<Routine> getRoutine(String id) async {
    final response = await _send(
      () => _client.get(_uri(_routinePath(id)), headers: _authJsonHeaders),
      defaultTimeout,
    );
    return _read(response, Routine.fromJson);
  }

  @override
  Future<Routine> updateRoutine(String id, RoutinePatch patch) =>
      _sendRoutine('PATCH', _routinePath(id), patch.toJson());

  @override
  Future<Routine> pauseRoutine(String id, {PlainDate? until}) =>
      // Without `until`: paused until resumed (3.21).
      _sendRoutine('POST', '${_routinePath(id)}/pause', {
        if (until != null) 'until': until.iso,
      });

  @override
  Future<Routine> resumeRoutine(String id) async {
    // No body (3.22).
    final response = await _send(
      () => _client.post(
        _uri('${_routinePath(id)}/resume'),
        headers: _authJsonHeaders,
      ),
      defaultTimeout,
    );
    return _read(response, Routine.fromJson);
  }

  @override
  Future<void> deleteRoutine(String id) async {
    // 204 without a body.
    await _send(
      () => _client.delete(_uri(_routinePath(id)), headers: _authJsonHeaders),
      defaultTimeout,
    );
  }

  @override
  Future<OccurrenceList> listOccurrences({
    required String module,
    PlainDate? from,
    PlainDate? to,
  }) async {
    final uri = _uri('/routines/occurrences').replace(
      queryParameters: {
        'module': module,
        if (from != null) 'from': from.iso,
        if (to != null) 'to': to.iso,
      },
    );
    final response = await _send(
      () => _client.get(uri, headers: _authJsonHeaders),
      defaultTimeout,
    );
    return _read(response, OccurrenceList.fromJson);
  }

  @override
  Future<List<RoutineLog>> listRoutineLogs(
    String id, {
    PlainDate? from,
    PlainDate? to,
  }) async {
    final parameters = {
      if (from != null) 'from': from.iso,
      if (to != null) 'to': to.iso,
    };
    // Without parameters no `?` at the end of the URL.
    final path = _uri('${_routinePath(id)}/logs');
    final uri = parameters.isEmpty
        ? path
        : path.replace(queryParameters: parameters);
    final response = await _send(
      () => _client.get(uri, headers: _authJsonHeaders),
      defaultTimeout,
    );
    return _read(
      response,
      (json) => readList(json, 'logs', RoutineLog.fromJson),
    );
  }

  @override
  Future<RoutineLog> addRoutineLog(String id, NewRoutineLog log) async {
    final response = await _send(
      () => _client.post(
        _uri('${_routinePath(id)}/logs'),
        headers: _authJsonHeaders,
        body: jsonEncode(log.toJson()),
      ),
      defaultTimeout,
    );
    return _read(response, RoutineLog.fromJson);
  }

  @override
  Future<void> deleteRoutineLog(String id, String logId) async {
    await _send(
      () => _client.delete(
        _uri('${_routinePath(id)}/logs/${Uri.encodeComponent(logId)}'),
        headers: _authJsonHeaders,
      ),
      defaultTimeout,
    );
  }

  /// POST or PATCH with a JSON body; the response is a routine.
  Future<Routine> _sendRoutine(
    String method,
    String path,
    Map<String, dynamic> body,
  ) async {
    final uri = _uri(path);
    final encoded = jsonEncode(body);
    final response = await _send(
      () => method == 'PATCH'
          ? _client.patch(uri, headers: _authJsonHeaders, body: encoded)
          : _client.post(uri, headers: _authJsonHeaders, body: encoded),
      defaultTimeout,
    );
    return _read(response, Routine.fromJson);
  }

  // ---------------------------------------------------------------------

  Uri _uri(String path) => Uri.parse('$_base$path');

  String _entryPath(String id) => '/entries/${Uri.encodeComponent(id)}';

  String _routinePath(String id) => '/routines/${Uri.encodeComponent(id)}';

  static String _withoutTrailingSlash(String url) => url.endsWith('/')
      ? _withoutTrailingSlash(url.substring(0, url.length - 1))
      : url;

  /// Runs the request and turns network and HTTP errors into an
  /// [ApiException]. Only returns responses with status 2xx, or with a
  /// status from [alsoAccept].
  Future<http.Response> _send(
    Future<http.Response> Function() request,
    Duration timeout, {
    Set<int> alsoAccept = const {},
  }) async {
    final http.Response response;
    try {
      response = await request().timeout(timeout);
    } on TimeoutException {
      throw const ApiException(
        code: ErrorCode.networkError,
        message:
            'Der Server antwortet nicht. '
            'Bitte später noch einmal versuchen.',
      );
    } on http.ClientException {
      throw const ApiException(
        code: ErrorCode.networkError,
        message:
            'Keine Verbindung zum Server. '
            'Bitte prüfe die Internetverbindung.',
      );
    }
    if ((response.statusCode >= 200 && response.statusCode < 300) ||
        alsoAccept.contains(response.statusCode)) {
      return response;
    }
    throw _errorFrom(response);
  }

  /// Reads the error format `{"error": {"code", "message", "details"}}`.
  ApiException _errorFrom(http.Response response) {
    try {
      final json = jsonDecode(utf8.decode(response.bodyBytes));
      if (json case {'error': final Map<String, dynamic> error}) {
        if (error case {
          'code': final String code,
          'message': final String message,
        }) {
          final raw = error['details'];
          final details = raw is Map<String, dynamic>
              ? raw
              : const <String, dynamic>{};
          return ApiException(
            httpStatus: response.statusCode,
            code: code,
            message: message,
            details: _withRetryAfter(code, details, response.headers),
          );
        }
      }
    } on FormatException {
      // Not JSON (e.g. an HTML error page of a proxy): handled below.
    }
    return ApiException(
      httpStatus: response.statusCode,
      code: ErrorCode.unexpectedResponse,
      message:
          'Unerwartete Antwort vom Server (Status ${response.statusCode}).',
    );
  }

  /// For `429 TOO_MANY_ATTEMPTS` the wait comes in `details.retryAfterSeconds`
  /// and in the header `Retry-After`, both with the same value (3.10). If
  /// the field is missing (e.g. a proxy changed the body), the header fills
  /// it in.
  static Map<String, dynamic> _withRetryAfter(
    String code,
    Map<String, dynamic> details,
    Map<String, String> headers,
  ) {
    if (code != ErrorCode.tooManyAttempts ||
        details['retryAfterSeconds'] is num) {
      return details;
    }
    final seconds = int.tryParse(headers['retry-after']?.trim() ?? '');
    if (seconds == null) return details;
    return {...details, 'retryAfterSeconds': seconds};
  }

  /// Decodes the body as UTF-8 JSON and converts it. If the response does
  /// not match the contract, the result is [ErrorCode.unexpectedResponse].
  T _read<T>(http.Response response, T Function(Map<String, dynamic>) convert) {
    try {
      // Always decode as UTF-8: without `charset` in the header,
      // `response.body` would assume Latin-1 and garble umlauts.
      final json = jsonDecode(utf8.decode(response.bodyBytes));
      return convert(asObject(json, 'response'));
    } on FormatException catch (e) {
      throw ApiException(
        httpStatus: response.statusCode,
        code: ErrorCode.unexpectedResponse,
        message: 'Die Antwort des Servers ist unvollständig.',
        details: {'reason': e.message},
      );
    }
  }
}
