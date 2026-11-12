import 'dart:async';
import 'dart:convert';
import 'dart:typed_data';

import 'package:http/http.dart' as http;
import 'package:http_parser/http_parser.dart';

import '../model/answer.dart';
import '../model/entry.dart';
import '../model/entry_patch.dart';
import '../model/json_helpers.dart';
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

  final String _base;
  final http.Client _client;

  static const _jsonHeaders = {
    'Content-Type': 'application/json; charset=utf-8',
    'Accept': 'application/json',
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
      () => _client.get(uri, headers: _jsonHeaders),
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
      () => _client.get(_uri(_entryPath(id)), headers: _jsonHeaders),
      defaultTimeout,
    );
    return _read(response, Entry.fromJson);
  }

  @override
  Future<FileContent> getFile(String id) async {
    final response = await _send(
      () => _client.get(_uri('${_entryPath(id)}/file')),
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
        headers: _jsonHeaders,
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
      () => _client.delete(_uri(_entryPath(id)), headers: _jsonHeaders),
      defaultTimeout,
    );
  }

  @override
  Future<Answer> ask(String question, {required String module}) async {
    final response = await _send(
      () => _client.post(
        _uri('/questions'),
        headers: _jsonHeaders,
        body: jsonEncode({'question': question, 'module': module}),
      ),
      longTimeout,
    );
    return _read(response, Answer.fromJson);
  }

  // ---------------------------------------------------------------------

  Uri _uri(String path) => Uri.parse('$_base$path');

  String _entryPath(String id) => '/entries/${Uri.encodeComponent(id)}';

  static String _withoutTrailingSlash(String url) => url.endsWith('/')
      ? _withoutTrailingSlash(url.substring(0, url.length - 1))
      : url;

  /// Runs the request and turns network and HTTP errors into an
  /// [ApiException]. Only returns responses with status 2xx.
  Future<http.Response> _send(
    Future<http.Response> Function() request,
    Duration timeout,
  ) async {
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
    if (response.statusCode >= 200 && response.statusCode < 300) {
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
          final details = error['details'];
          return ApiException(
            httpStatus: response.statusCode,
            code: code,
            message: message,
            details: details is Map<String, dynamic> ? details : const {},
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
