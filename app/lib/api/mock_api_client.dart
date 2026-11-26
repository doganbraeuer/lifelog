import 'dart:convert';
import 'dart:math';

import 'package:crypto/crypto.dart';
import 'package:flutter/foundation.dart';

import '../model/answer.dart';
import '../model/plain_date.dart';
import '../model/entry.dart';
import '../model/entry_patch.dart';
import '../model/json_helpers.dart';
import '../model/routine.dart';
import '../model/server_status.dart';
import '../model/transcript.dart';
import '../model/user.dart';
import '../model/verification.dart';
import '../module/module_catalog.dart';
import '../session/consent_text.dart' as notice;
import 'api_client.dart';
import 'api_exception.dart';
import 'mime_type.dart';
import 'mock/mock_data.dart';
import 'mock/mock_routines.dart';
import 'mock/mock_time_zones.dart';
import 'mock/mock_verification.dart';
import 'timeline_sort.dart';

/// Answer without a hit, literally from docs/API.md.
const notFoundAnswer = 'Dazu finde ich nichts in deinen Dokumenten.';

/// An in-memory backend that behaves like docs/API.md: same sorting, same
/// correction rules, same error codes. That way switching to the real
/// backend brings no surprises.
///
/// Internally the mock stores every entry as a JSON map, exactly as the
/// backend would return it. It checks corrections against the JSON body,
/// so that errors like a wrong date format can be reproduced too.
///
/// `verification` comes from a simplified second reader
/// (mock/mock_verification.dart): uploads and text imports are checked
/// against what the mock "reads", and every correction checks the entry
/// again, as the backend does.
///
/// Errors for trying out by hand:
/// - An upload with `unreadable`, `extractionerror` or `modelerror` in
///   the file name returns 422, 502 or 503.
/// - A question with the word `modelerror` returns 503.
/// - A transcription with `nospeech`, `toolong`, `unreadable` or
///   `modelerror` in the file name returns 422 NO_SPEECH, 413
///   AUDIO_TOO_LONG, 422 UNREADABLE or 503. The app names its recordings
///   itself, so these only work in tests.
/// - With [speechStatus] other than `ok`, `/status` reports it and a
///   transcription returns 503.
/// - `limitreached` in a file name, a text, a question or the file name of
///   a recording returns `429 LIMIT_REACHED` (daily limit).
///
/// Accounts and login (docs/API.md section 1, 3.10–3.15) work as in the
/// backend: every call except `/status` and the login needs a token from
/// [login], otherwise `401 UNAUTHORIZED`; while the start password is
/// unchanged, everything except [me], [changePassword] and [logout] answers
/// `403 PASSWORD_CHANGE_REQUIRED`. Test accounts (synthetic, only in the
/// mock):
///
/// | Username | Password | Note |
/// |---|---|---|
/// | `tester01` | `Startpasswort1` | start password: must change it, no consent yet |
/// | `admin` | `Adminpasswort1` | no password change required, no consent yet |
///
/// After 5 failed attempts for a username (wrong password at login, or a
/// wrong current password when changing it) the next attempt gets
/// `429 TOO_MANY_ATTEMPTS` with `retryAfterSeconds: 60`, measured with the
/// mock's clock. Unlike the backend the mock does not separate entries per
/// user: every account sees the sample data.
///
/// Routines (docs/API.md 3.16–3.27) follow the backend's rules: the mock
/// expands schedules in the user's time zone (daylight saving time
/// included), keeps logs and pauses and computes status and progress, with
/// the backend's messages (mock/mock_routines.dart). With sample data it
/// has three synthetic routines: „Vitamin D“, „Blutdruck messen“ and
/// „Wasser“.
class MockApiClient implements ApiClient {
  MockApiClient({
    bool withSampleData = true,
    this.delay = const Duration(milliseconds: 300),
    this.uploadDelay = const Duration(seconds: 2),
    this.transcriptionDelay = const Duration(seconds: 3),
    DateTime Function()? clock,
    ModuleCatalog? catalog,
  }) : _clock = clock ?? (() => DateTime.now().toUtc()),
       _catalog = catalog ?? ModuleCatalog.defaults() {
    if (withSampleData) {
      for (final e in sampleData()) {
        final id = e.json['id'] as String;
        _entries[id] = e.json;
        _files[id] = e.bytes;
        if (e.original case final original?) _originals[id] = original;
      }
      _routines.addSamples();
    }
  }

  /// Artificial delay per call, so that loading indicators become visible.
  final Duration delay;

  /// Delay for uploads (the real backend needs up to 60 s).
  final Duration uploadDelay;

  /// Delay for transcriptions (the real backend needs up to 120 s).
  final Duration transcriptionDelay;

  /// If `true`, every call fails with `500 INTERNAL_ERROR`.
  bool simulateError = false;

  /// Field `speech` in `/status`. Anything but `ok` makes a transcription
  /// fail with 503, as in the backend.
  SpeechStatus speechStatus = SpeechStatus.ok;

  final DateTime Function() _clock;

  /// Allowed types per module, as the backend checks them too.
  final ModuleCatalog _catalog;
  final _entries = <String, Map<String, dynamic>>{};
  final _files = <String, Uint8List>{};

  /// What the second reader saw at upload, like the backend's collection
  /// `ocr`. Missing for entries that are `unchecked`.
  final _originals = <String, MockOriginal>{};
  final _random = Random();
  var _counter = 0;

  /// Accounts by username; see the table in the class comment.
  final _users = <String, _MockUser>{
    'tester01': _MockUser(
      id: '673b0c1fa2d4e5f6a7b8c9d0',
      username: 'tester01',
      password: mockStartPassword,
      mustChangePassword: true,
    ),
    'admin': _MockUser(
      id: '673b0c1fa2d4e5f6a7b8c9d1',
      username: 'admin',
      password: mockAdminPassword,
      mustChangePassword: false,
    ),
  };

  /// Valid tokens → username. One per login, like the backend's `sessions`.
  final _sessions = <String, String>{};

  /// Token of this client (what HttpApiClient would send).
  String? _token;

  /// Failed attempts per username, and until when it is locked.
  final _failures = <String, int>{};
  final _lockedUntil = <String, DateTime>{};

  /// Passwords of the test accounts in the class comment (synthetic).
  static const mockStartPassword = 'Startpasswort1';
  static const mockAdminPassword = 'Adminpasswort1';

  /// Failed attempts that lock a username, and for how long.
  static const maxFailedAttempts = 5;
  static const lockDuration = Duration(seconds: 60);

  @override
  bool get isMock => true;

  @override
  void close() {}

  // ---------------------------------------------------------------------
  // GET /entries
  // ---------------------------------------------------------------------

  @override
  Future<List<EntrySummary>> listEntries({
    required String module,
    Sort sort = Sort.documentDate,
    SortOrder order = SortOrder.desc,
  }) => listEntriesWithParameters({
    'module': module,
    'sort': sort.name,
    'order': order.name,
  });

  /// Like [listEntries], but with raw query parameters. This makes it
  /// possible to test `400 INVALID_PARAMETER` too.
  @visibleForTesting
  Future<List<EntrySummary>> listEntriesWithParameters(
    Map<String, String> parameter,
  ) async {
    await _wait(delay);
    final module = parameter['module'] ?? '';
    if (!_isKnownModule(module)) {
      throw ApiException(
        httpStatus: 400,
        code: ErrorCode.invalidParameter,
        message: _unknownModule,
        details: {'parameter': 'module', 'value': module},
      );
    }
    final sort = _enumParameter(
      parameter,
      'sort',
      Sort.values,
      Sort.documentDate,
      'Unbekannte Sortierung. Erlaubt sind documentDate und importedAt.',
    );
    final order = _enumParameter(
      parameter,
      'order',
      SortOrder.values,
      SortOrder.desc,
      'Unbekannte Richtung. Erlaubt sind desc und asc.',
    );
    final chosen = _moduleOrDefault(module);
    return sortTimeline(
      _entries.values
          .where((json) => json['module'] == chosen)
          .map(EntrySummary.fromJson),
      sort,
      order,
    );
  }

  // ---------------------------------------------------------------------
  // Modules (docs/API.md, section 1 "Modules")
  // ---------------------------------------------------------------------

  static const _unknownModule = 'Unbekanntes Modul.';

  /// If `module` is missing or empty, `health` applies.
  static String _moduleOrDefault(String module) =>
      module.isEmpty ? 'health' : module;

  /// Every module from the catalog is known. (In the prototype the real
  /// backend only knows `health`; but the app does not request other
  /// modules at all while they are inactive.)
  bool _isKnownModule(String module) =>
      _catalog.byId(_moduleOrDefault(module)) != null;

  /// Checks the module for upload, text import and question: an unknown one
  /// results in `400 INVALID_REQUEST`. Returns the valid module ID.
  String _checkModule(String module) {
    if (!_isKnownModule(module)) {
      throw ApiException(
        httpStatus: 400,
        code: ErrorCode.invalidRequest,
        message: _unknownModule,
        details: {'field': 'module', 'value': module},
      );
    }
    return _moduleOrDefault(module);
  }

  /// Like [_checkModule], plus: nothing can be imported into a module
  /// without types, because there is no allowed `type`.
  String _checkImportModule(String module) {
    final id = _checkModule(module);
    if (_catalog.types(id).isEmpty) {
      throw ApiException(
        httpStatus: 400,
        code: ErrorCode.invalidRequest,
        message: 'In diesem Modul ist noch kein Import möglich.',
        details: {'field': 'module', 'value': module},
      );
    }
    return id;
  }

  /// Preferred type if the module knows it, otherwise the first one.
  String _typeFor(String module, String preferred) {
    final types = _catalog.types(module).map((t) => t.id).toList();
    return types.contains(preferred) ? preferred : types.first;
  }

  T _enumParameter<T extends Enum>(
    Map<String, String> parameter,
    String name,
    List<T> values,
    T defaultValue,
    String message,
  ) {
    final value = parameter[name];
    if (value == null) return defaultValue;
    for (final w in values) {
      if (w.name == value) return w;
    }
    throw ApiException(
      httpStatus: 400,
      code: ErrorCode.invalidParameter,
      message: message,
      details: {'parameter': name, 'value': value},
    );
  }

  // ---------------------------------------------------------------------
  // GET /entries/{id} and /entries/{id}/file
  // ---------------------------------------------------------------------

  @override
  Future<Entry> getEntry(String id) async {
    await _wait(delay);
    return Entry.fromJson(_find(id));
  }

  @override
  Future<FileContent> getFile(String id) async {
    await _wait(delay);
    final json = _find(id);
    final file = asObject(json['file'], 'file');
    return FileContent(
      bytes: _files[json['id']]!,
      mimeType: readString(file, 'mimeType'),
    );
  }

  static final _objectId = RegExp(r'^[0-9a-fA-F]{24}$');

  Map<String, dynamic> _find(String id) {
    if (!_objectId.hasMatch(id)) {
      throw ApiException(
        httpStatus: 400,
        code: ErrorCode.invalidId,
        message: 'Die ID ist ungültig.',
      );
    }
    final json = _entries[id.toLowerCase()];
    if (json == null) {
      throw const ApiException(
        httpStatus: 404,
        code: ErrorCode.entryNotFound,
        message: 'Diesen Eintrag gibt es nicht.',
      );
    }
    return json;
  }

  // ---------------------------------------------------------------------
  // POST /entries
  // ---------------------------------------------------------------------

  @override
  Future<Entry> upload(
    Uint8List bytes,
    String fileName, {
    required String module,
  }) async {
    await _wait(uploadDelay);

    // 1. Check module, size and type.
    final moduleId = _checkImportModule(module);
    if (bytes.isEmpty) {
      throw const ApiException(
        httpStatus: 400,
        code: ErrorCode.fileMissing,
        message: 'Es wurde keine Datei mitgeschickt.',
      );
    }
    if (bytes.length > maxFileSize) {
      throw const ApiException(
        httpStatus: 413,
        code: ErrorCode.fileTooLarge,
        message: 'Die Datei ist größer als 10 MB.',
      );
    }
    final mimeType = detectMimeType(bytes);
    if (mimeType == null) {
      throw const ApiException(
        httpStatus: 415,
        code: ErrorCode.unsupportedFileType,
        message:
            'Dieser Dateityp wird nicht unterstützt. '
            'Erlaubt sind JPEG, PNG, HEIC, WebP und PDF.',
      );
    }

    // 2. Duplicate check via the SHA-256 hash, before the model call.
    final hash = _checkDuplicate(
      bytes,
      'Diese Datei hast du schon hochgeladen.',
    );

    // 3. "Vision LLM": errors can be triggered via the file name.
    _checkTriggers(fileName);

    // 4.–6. Create and store the entry.
    return _store(
      module: moduleId,
      type: _typeFor(moduleId, 'doctor_letter'),
      documentDate: PlainDate.fromDateTime(_clock().toLocal()),
      issuer: 'Mock-Praxis Beispiel',
      fullText:
          'Mock-Praxis Beispiel\n'
          'Dieser Eintrag stammt aus dem Mock, nicht aus einer echten '
          'Auswertung.\n'
          'Dateiname: $fileName\n'
          'Blutdruck 120/80 mmHg',
      facts: [
        {'id': 'f1', 'name': 'Blutdruck', 'value': '120/80', 'unit': 'mmHg'},
      ],
      fileName: fileName,
      mimeType: mimeType,
      bytes: bytes,
      hash: hash,
    );
  }

  /// Date `DD.MM.YYYY` in the text (e.g. „vom 21.10.2026“).
  static final _dateInText = RegExp(r'\b(\d{1,2})\.(\d{1,2})\.(\d{4})\b');

  @override
  Future<Entry> importText(String text, {required String module}) async {
    await _wait(uploadDelay);

    // 1. Check module and text.
    final moduleId = _checkImportModule(module);
    if (text.trim().isEmpty) {
      throw const ApiException(
        httpStatus: 400,
        code: ErrorCode.fileMissing,
        message: 'Bitte einen Text eingeben.',
      );
    }
    if (text.runes.length > maxTextLength) {
      throw const ApiException(
        httpStatus: 413,
        code: ErrorCode.fileTooLarge,
        message: 'Der Text ist länger als 100 000 Zeichen.',
        details: {'maxChars': maxTextLength},
      );
    }

    // 2. The text is stored as text.txt; duplicates via these bytes.
    final bytes = Uint8List.fromList(utf8.encode(text));
    final hash = _checkDuplicate(
      bytes,
      'Diesen Text hast du schon importiert.',
    );

    // 3. "Language model": triggers as for the file name, here in the text.
    _checkTriggers(text);

    // Instead of a model: first date in the text, first line as the issuer.
    // fullText is the entered text, unchanged (docs/API.md 3.1).
    PlainDate? date;
    for (final hits in _dateInText.allMatches(text)) {
      final day = int.parse(hits[1]!);
      final month = int.parse(hits[2]!);
      final year = int.parse(hits[3]!);
      if (PlainDate.isValid(year, month, day)) {
        date = PlainDate(year, month, day);
        break;
      }
    }
    final firstLine = text
        .split('\n')
        .map((z) => z.trim())
        .firstWhere((z) => z.isNotEmpty);

    // Without a date in the text: the day of the import, marked as such
    // (docs/API.md 3.1). The backend takes the day in Europe/Vienna; the
    // mock the device's local day.
    final importDay = date == null;

    return _store(
      module: moduleId,
      type: _typeFor(moduleId, 'other'),
      documentDate: date ?? PlainDate.fromDateTime(_clock().toLocal()),
      importDay: importDay,
      issuer: firstLine,
      fullText: text,
      facts: const [],
      fileName: 'text.txt',
      mimeType: 'text/plain; charset=utf-8',
      bytes: bytes,
      hash: hash,
    );
  }

  /// Throws `409 DUPLICATE` if the hash already exists (across all modules).
  /// Otherwise returns the hash.
  String _checkDuplicate(Uint8List bytes, String message) {
    final hash = sha256.convert(bytes).toString();
    for (final json in _entries.values) {
      if (asObject(json['file'], 'file')['sha256'] == hash) {
        throw ApiException(
          httpStatus: 409,
          code: ErrorCode.duplicate,
          message: message,
          details: {'entryId': json['id']},
        );
      }
    }
    return hash;
  }

  /// Model errors for trying out: `unreadable`, `extractionerror` or
  /// `modelerror` in the file name or in the text.
  void _checkTriggers(String content) {
    final lower = content.toLowerCase();
    // The daily limit is checked before the model call (section 1).
    if (lower.contains('limitreached')) throw _limitReached;
    if (lower.contains('unreadable')) {
      throw const ApiException(
        httpStatus: 422,
        code: ErrorCode.unreadable,
        message:
            'In der Datei ist kein Dokument erkennbar. '
            'Versuche es mit einem schärferen Foto.',
      );
    }
    if (lower.contains('extractionerror')) {
      throw const ApiException(
        httpStatus: 502,
        code: ErrorCode.extractionFailed,
        message:
            'Das Dokument konnte nicht ausgewertet werden. '
            'Bitte später noch einmal versuchen.',
      );
    }
    if (lower.contains('modelerror')) {
      throw _modelUnavailable;
    }
  }

  /// Creates a new entry together with its original file.
  Entry _store({
    required String module,
    required String type,
    required PlainDate? documentDate,
    required String? issuer,
    required String fullText,
    required List<Map<String, dynamic>> facts,
    required String fileName,
    required String mimeType,
    required Uint8List bytes,
    required String hash,
    bool importDay = false,
  }) {
    final id = _newId();
    // The mock "reads" its own full text: a text layer for a PDF, OCR for
    // an image, the entered text for a text import.
    final original = mediaType(mimeType) == 'text/plain'
        ? MockOriginal.text(fullText)
        : MockOriginal.page(
            mediaType(mimeType) == 'application/pdf'
                ? VerificationSource.pdfText
                : VerificationSource.ocr,
            fullText.split('\n'),
          );
    final json = <String, dynamic>{
      'id': id,
      'module': module,
      'type': type,
      'documentDate': documentDate?.iso,
      'documentDateSource': importDay ? 'import' : 'document',
      'importedAt': formatTimestamp(_clock()),
      'issuer': issuer,
      'fullText': fullText,
      'facts': <dynamic>[
        for (final fact in facts) <String, dynamic>{...fact},
      ],
      'file': <String, dynamic>{
        'id': _newId(),
        'fileName': fileName,
        'mimeType': mimeType,
        'size': bytes.length,
        'sha256': hash,
      },
      'corrections': <dynamic>[],
    };
    json['verification'] = checkEntry(
      json,
      original,
      checkedAt: json['importedAt'] as String,
    );
    _entries[id] = json;
    _files[id] = Uint8List.fromList(bytes);
    _originals[id] = original;
    return Entry.fromJson(json);
  }

  // ---------------------------------------------------------------------
  // DELETE /entries/{id}
  // ---------------------------------------------------------------------

  @override
  Future<void> deleteEntry(String id) async {
    await _wait(delay);
    final json = _find(id); // 400 INVALID_ID or 404
    // The mock has no chunks; file and entry disappear.
    // Afterwards the hash is free, the same file can be uploaded again.
    _files.remove(json['id']);
    _originals.remove(json['id']);
    _entries.remove(json['id']);
  }

  /// ID in the style of a MongoDB ObjectId: 24 hex characters.
  String _newId() {
    String id;
    do {
      final seconds = _clock().millisecondsSinceEpoch ~/ 1000;
      final randomPart = [
        for (var i = 0; i < 5; i++)
          _random.nextInt(256).toRadixString(16).padLeft(2, '0'),
      ].join();
      _counter = (_counter + 1) & 0xFFFFFF;
      id =
          seconds.toRadixString(16).padLeft(8, '0') +
          randomPart +
          _counter.toRadixString(16).padLeft(6, '0');
    } while (_entries.containsKey(id));
    return id;
  }

  // ---------------------------------------------------------------------
  // PATCH /entries/{id}
  // ---------------------------------------------------------------------

  @override
  Future<Entry> correct(String id, EntryPatch patch) =>
      correctWithJson(id, patch.toJson());

  /// Like [correct], but with a raw JSON body. This makes it possible to
  /// test invalid requests too that cannot be created with [EntryPatch]
  /// (e.g. `"documentDate": "15.10.2026"`).
  @visibleForTesting
  Future<Entry> correctWithJson(String id, Map<String, dynamic> body) async {
    await _wait(delay);
    final json = _find(id);

    // 1. Check everything. If any part is invalid, nothing is changed.
    final changes = _checkPatch(json, body);

    // 2. and 3. Write the values and maintain `corrections`.
    final timestamp = formatTimestamp(_clock());
    var changed = false;
    // A date set from the import day was never recognized: a request with
    // documentDate starts from null and removes the mark (docs/API.md 3.5).
    if (json['documentDateSource'] == 'import' &&
        changes.any((change) => change.$1 == 'documentDate')) {
      json['documentDate'] = null;
      json['documentDateSource'] = 'document';
      changed = true;
    }
    for (final (field, newValue) in changes) {
      if (_set(json, field, newValue, timestamp)) changed = true;
    }

    // 4. If a value changed, check again against the stored original (no
    // new OCR run). An unchecked entry stays unchecked.
    final original = _originals[json['id']];
    if (changed && original != null) {
      json['verification'] = checkEntry(json, original, checkedAt: timestamp);
    }
    return Entry.fromJson(json);
  }

  static const _correctable = {'type', 'documentDate', 'issuer', 'facts'};
  static const _notCorrectable = {
    'id',
    'module',
    'importedAt',
    'fullText',
    'file',
    'corrections',
  };

  /// Checks the body and returns the changes as a list of
  /// (field path, new value).
  List<(String, String?)> _checkPatch(
    Map<String, dynamic> json,
    Map<String, dynamic> body,
  ) {
    if (body.isEmpty) {
      throw _invalid('Die Anfrage enthält keine Änderung.');
    }
    for (final field in body.keys) {
      if (_notCorrectable.contains(field)) {
        throw _notCorrectableError(field);
      }
      if (!_correctable.contains(field)) {
        throw _invalid('Unbekanntes Feld: $field.', {'field': field});
      }
    }

    final result = <(String, String?)>[];

    if (body.containsKey('type')) {
      final type = body['type'];
      // A type only applies within the module the entry belongs to.
      final allowed = [
        for (final t in _catalog.types(json['module'] as String)) t.id,
      ];
      if (type is! String || !allowed.contains(type)) {
        throw _invalid('Unbekannter Typ. Erlaubt sind ${allowed.join(', ')}.', {
          'field': 'type',
          'value': type,
        });
      }
      result.add(('type', type));
    }

    if (body.containsKey('documentDate')) {
      final value = body['documentDate'];
      if (value == null) {
        result.add(('documentDate', null));
      } else if (value is String && PlainDate.tryParse(value) != null) {
        result.add(('documentDate', value));
      } else if (value is String && PlainDate.format.hasMatch(value)) {
        throw _invalid('Das Dokumentdatum ist kein gültiges Kalenderdatum.', {
          'field': 'documentDate',
          'value': value,
        });
      } else {
        throw _invalid(
          'Das Dokumentdatum muss im Format JJJJ-MM-TT angegeben werden.',
          {'field': 'documentDate', 'value': value},
        );
      }
    }

    if (body.containsKey('issuer')) {
      final value = body['issuer'];
      if (value != null && value is! String) {
        throw _invalid('Die Quelle muss ein Text oder null sein.', {
          'field': 'issuer',
          'value': value,
        });
      }
      // Remove whitespace; if nothing remains, it counts as null (3.5).
      result.add(('issuer', _trimToNull(value as String?)));
    }

    if (body.containsKey('facts')) {
      final facts = body['facts'];
      if (facts is! List) {
        throw _invalid('"facts" muss eine Liste sein.', {'field': 'facts'});
      }
      final existingIds = {
        for (final fact in json['facts'] as List) (fact as Map)['id'],
      };
      final seen = <String>{};
      for (final element in facts) {
        if (element is! Map<String, dynamic>) {
          throw _invalid('Jede Info muss ein Objekt sein.');
        }
        final id = element['id'];
        if (id is! String || id.isEmpty) {
          throw _invalid('Jede Info braucht eine id.', {'field': 'facts'});
        }
        if (!seen.add(id)) {
          throw _invalid('Die Info $id kommt mehrfach vor.', {'factId': id});
        }
        for (final key in element.keys) {
          if (key == 'name') {
            throw _notCorrectableError('facts.$id.name');
          }
          if (!{'id', 'value', 'unit'}.contains(key)) {
            throw _invalid('Unbekanntes Feld: facts.$id.$key.', {
              'field': 'facts.$id.$key',
            });
          }
        }
        if (!element.containsKey('value') && !element.containsKey('unit')) {
          throw _invalid('Die Info $id enthält keine Änderung.', {
            'factId': id,
          });
        }
        if (!existingIds.contains(id)) {
          throw ApiException(
            httpStatus: 404,
            code: ErrorCode.factNotFound,
            message: 'Diese Info gibt es in dem Eintrag nicht.',
            details: {'factId': id},
          );
        }
        if (element.containsKey('value')) {
          final value = element['value'];
          if (value is! String) {
            throw _invalid('Der Wert muss ein Text sein.', {
              'field': 'facts.$id.value',
              'value': value,
            });
          }
          // A fact always has a value: empty is an error (3.5).
          if (value.trim().isEmpty) {
            throw _invalid('Der Wert darf nicht leer sein.', {
              'field': 'facts.$id.value',
              'value': value,
            });
          }
          result.add(('facts.$id.value', value.trim()));
        }
        if (element.containsKey('unit')) {
          final unit = element['unit'];
          if (unit != null && unit is! String) {
            throw _invalid('Die Einheit muss ein Text oder null sein.', {
              'field': 'facts.$id.unit',
              'value': unit,
            });
          }
          result.add(('facts.$id.unit', _trimToNull(unit as String?)));
        }
      }
    }
    return result;
  }

  /// Removes whitespace at the start and end; an empty string becomes `null`.
  static String? _trimToNull(String? text) {
    final trimmed = text?.trim();
    return (trimmed == null || trimmed.isEmpty) ? null : trimmed;
  }

  /// Sets a field and maintains `corrections` according to the rules from
  /// docs/API.md, section 2:
  /// - A field whose value does not change creates no correction.
  /// - At most one correction per field; `recognized` stays the original.
  /// - Back to the recognized value: the correction is removed.
  ///
  /// Returns whether the value changed.
  bool _set(
    Map<String, dynamic> json,
    String field,
    String? newValue,
    String correctedAt,
  ) {
    final old = _read(json, field);
    if (old == newValue) return false;

    final corrections = json['corrections'] as List;
    final index = corrections.indexWhere((k) => (k as Map)['field'] == field);
    // Without a correction yet, the current value is the recognized one.
    final recognized = index >= 0
        ? (corrections[index] as Map)['recognized'] as String?
        : old;

    _write(json, field, newValue);

    if (newValue == recognized) {
      if (index >= 0) corrections.removeAt(index);
    } else if (index >= 0) {
      final correction = corrections[index] as Map<String, dynamic>;
      correction['corrected'] = newValue;
      correction['correctedAt'] = correctedAt;
    } else {
      corrections.add(<String, dynamic>{
        'field': field,
        'recognized': recognized,
        'corrected': newValue,
        'correctedAt': correctedAt,
      });
    }
    return true;
  }

  String? _read(Map<String, dynamic> json, String field) {
    final (fact, key) = _target(json, field);
    return fact[key] as String?;
  }

  void _write(Map<String, dynamic> json, String field, String? value) {
    final (target, key) = _target(json, field);
    // A unit without a value is missing from the JSON entirely ("optional").
    if (value == null && field.startsWith('facts.')) {
      target.remove(key);
    } else {
      target[key] = value;
    }
  }

  /// Returns the map and the key under which a field path is stored.
  (Map<String, dynamic>, String) _target(
    Map<String, dynamic> json,
    String field,
  ) {
    if (!field.startsWith('facts.')) return (json, field);
    final parts = field.split('.'); // facts, <id>, value|unit
    final fact = (json['facts'] as List)
        .cast<Map<String, dynamic>>()
        .firstWhere((i) => i['id'] == parts[1]);
    return (fact, parts[2]);
  }

  ApiException _invalid(String message, [Map<String, dynamic>? details]) =>
      ApiException(
        httpStatus: 400,
        code: ErrorCode.invalidRequest,
        message: message,
        details: details ?? const {},
      );

  ApiException _notCorrectableError(String field) => ApiException(
    httpStatus: 400,
    code: ErrorCode.fieldNotCorrectable,
    message: 'Das Feld $field kann nicht korrigiert werden.',
    details: {'field': field},
  );

  // ---------------------------------------------------------------------
  // POST /questions
  // ---------------------------------------------------------------------

  static const maxQuestionLength = 500;

  @override
  Future<Answer> ask(String question, {required String module}) async {
    await _wait(delay);
    // As in the backend: check the question first, then the module.
    if (question.trim().isEmpty) {
      throw const ApiException(
        httpStatus: 400,
        code: ErrorCode.questionMissing,
        message: 'Bitte gib eine Frage ein.',
      );
    }
    // As in the backend, without details (docs/API.md 3.6).
    if (question.runes.length > maxQuestionLength) {
      throw const ApiException(
        httpStatus: 400,
        code: ErrorCode.questionTooLong,
        message:
            'Die Frage darf höchstens $maxQuestionLength Zeichen lang '
            'sein.',
      );
    }
    final moduleId = _checkModule(module);
    final words = _words(question).where((w) => !_stopWords.contains(w));
    if (words.contains('limitreached')) throw _limitReached;
    if (words.contains('modelerror')) throw _modelUnavailable;

    // Instead of vector search: a simple word search in the lines of the
    // full text. A line scores per question word contained in one of its
    // words (or the other way round, so "Hämoglobinwert" finds "Hämoglobin").
    final searchWords = words.where((w) => w.length >= 4).toSet();
    final hits = <(int, Map<String, dynamic>, String)>[];
    // Only search documents of the chosen module.
    for (final json in _entries.values.where((j) => j['module'] == moduleId)) {
      for (final line in (json['fullText'] as String).split('\n')) {
        final lineWords = _words(line).where((w) => w.length >= 4).toList();
        final score = searchWords
            .where((s) => lineWords.any((z) => z.contains(s) || s.contains(z)))
            .length;
        if (score > 0) hits.add((score, json, line.trim()));
      }
    }
    if (hits.isEmpty) {
      return const Answer(answer: notFoundAnswer, found: false, sources: []);
    }

    // At most two chunks with the best score.
    hits.sort((a, b) => b.$1.compareTo(a.$1));
    final best = hits.where((t) => t.$1 == hits.first.$1).take(2);
    final sources = <Source>[];
    final sentences = <String>[];
    for (final (_, json, excerpt) in best) {
      final summary = EntrySummary.fromJson(json);
      final number = sources.length + 1;
      sources.add(
        Source(
          number: number,
          entryId: summary.id,
          module: summary.module,
          type: summary.type,
          issuer: summary.issuer,
          documentDate: summary.documentDate,
          excerpt: excerpt,
        ),
      );
      final byIssuer = summary.issuer == null ? '' : ' von ${summary.issuer}';
      final onDate = summary.documentDate == null
          ? ''
          : ' vom ${summary.documentDate!.display}';
      sentences.add(
        'Im Dokument „${_catalog.typeName(summary.module, summary.type)}“$byIssuer$onDate '
        'steht: '
        '$excerpt [$number].',
      );
    }
    return Answer(answer: sentences.join(' '), found: true, sources: sources);
  }

  static final _wordPattern = RegExp(r'[\p{L}\p{N}]+', unicode: true);

  static Iterable<String> _words(String text) =>
      _wordPattern.allMatches(text.toLowerCase()).map((m) => m[0]!);

  // German on purpose: the questions are German user input.
  static const _stopWords = {
    'aber',
    'alle',
    'auch',
    'beim',
    'bitte',
    'dass',
    'diese',
    'dieser',
    'dokument',
    'dokumente',
    'dokumenten',
    'eine',
    'einem',
    'einen',
    'einer',
    'gibt',
    'habe',
    'haben',
    'hatte',
    'hatten',
    'meine',
    'meinem',
    'meinen',
    'meiner',
    'nach',
    'nicht',
    'noch',
    'oder',
    'sind',
    'stand',
    'steht',
    'über',
    'unter',
    'wann',
    'warum',
    'welche',
    'welchem',
    'welchen',
    'welcher',
    'welches',
    'wieso',
    'wurde',
    'wurden',
    'werden',
    'zeig',
    'zeige',
  };

  // ---------------------------------------------------------------------
  // POST /transcriptions and GET /status
  // ---------------------------------------------------------------------

  /// Fixed sample text instead of a speech recognition. German on purpose:
  /// it stands for what a user says. It has numbers with units, so that
  /// their marks in the app can be seen (`widgets/quantity_highlight.dart`).
  static const sampleTranscript =
      'Notiz vom 12.11.2026 nach dem Arzttermin. Kniegelenk rechts, '
      'leichter Erguss. Ibuprofen 400 Milligramm dreimal täglich für eine '
      'Woche. TSH 2,5 Milliunits pro Liter, Blutdruck 130/85 mmHg, '
      'Temperatur 37,6 Grad.';

  @override
  Future<Transcript> transcribe(Uint8List bytes, String fileName) async {
    await _wait(transcriptionDelay);

    // 1. Size and format, as in docs/API.md 3.9 (messages from the table).
    if (bytes.isEmpty) {
      throw const ApiException(
        httpStatus: 400,
        code: ErrorCode.fileMissing,
        message: 'Bitte eine Aufnahme mitschicken.',
      );
    }
    if (bytes.length > maxFileSize) {
      throw const ApiException(
        httpStatus: 413,
        code: ErrorCode.fileTooLarge,
        message: 'Die Aufnahme ist größer als 10 MB.',
      );
    }
    if (detectAudioType(bytes) == null) {
      throw const ApiException(
        httpStatus: 415,
        code: ErrorCode.unsupportedFileType,
        message:
            'Dieses Audioformat wird nicht unterstützt. Erlaubt sind M4A, '
            '3GP, AMR, WAV, OGG, WebM, MP3 und FLAC.',
      );
    }

    // 2. "Speech recognition": switched off or failing, or errors
    // triggered via the file name.
    final lower = fileName.toLowerCase();
    if (lower.contains('limitreached')) throw _limitReached;
    if (speechStatus != SpeechStatus.ok || lower.contains('modelerror')) {
      throw _speechUnavailable;
    }
    if (lower.contains('toolong')) {
      throw const ApiException(
        httpStatus: 413,
        code: ErrorCode.audioTooLong,
        message:
            'Die Aufnahme ist länger als 5 Minuten. Bitte kürzer sprechen '
            'oder aufteilen.',
        details: {'maxSeconds': maxRecordingSeconds},
      );
    }
    if (lower.contains('unreadable')) {
      throw const ApiException(
        httpStatus: 422,
        code: ErrorCode.unreadable,
        message: 'Die Aufnahme lässt sich nicht abspielen.',
      );
    }
    if (lower.contains('nospeech')) {
      throw const ApiException(
        httpStatus: 422,
        code: ErrorCode.noSpeech,
        message: 'In der Aufnahme ist keine Sprache zu hören.',
      );
    }

    // 3. Only the text comes back; nothing is stored (3.9).
    return const Transcript(
      text: sampleTranscript,
      durationSeconds: 21.4,
      language: 'de',
      engine: 'Mock',
    );
  }

  @override
  Future<ServerStatus> getStatus() async {
    await _wait(delay, access: _Access.public);
    return ServerStatus(status: 'ok', database: 'ok', speech: speechStatus);
  }

  // ---------------------------------------------------------------------
  // Accounts and login (3.10–3.15)
  // ---------------------------------------------------------------------

  @override
  void useToken(String? token) => _token = token;

  @override
  Future<LoginResult> login(String username, String password) async {
    await _wait(delay, access: _Access.public);
    if (username.trim().isEmpty || password.isEmpty) {
      throw _invalid('Bitte Benutzername und Passwort schicken.');
    }
    // As in the backend: trimmed and lowercase, the password exactly as sent.
    final name = username.trim().toLowerCase();
    _checkLock(name);
    final user = _users[name];
    if (user == null ||
        utf8.encode(password).length > maxPasswordBytes ||
        user.password != password) {
      _countFailure(name);
      throw const ApiException(
        httpStatus: 401,
        code: ErrorCode.invalidCredentials,
        message: 'Benutzername oder Passwort stimmt nicht.',
      );
    }
    _failures.remove(name);
    final token = _newToken();
    _sessions[token] = name;
    _token = token;
    return LoginResult(
      token: token,
      expiresAt: _clock().add(const Duration(days: 30)),
      user: user.toUser(withTimeZone: false),
    );
  }

  @override
  Future<void> logout() async {
    try {
      await _wait(delay, access: _Access.duringPasswordChange);
      _sessions.remove(_token);
    } finally {
      _token = null;
    }
  }

  @override
  Future<User> me() async {
    final user = await _wait(delay, access: _Access.duringPasswordChange);
    return user!.toUser();
  }

  @override
  Future<User> updateDisplayName(String displayName) async {
    final user = (await _wait(delay))!;
    final trimmed = displayName.trim();
    final length = trimmed.runes.length;
    if (length < 1 || length > 50 || _controlCharacter.hasMatch(trimmed)) {
      throw _invalid('Der Anzeigename muss 1 bis 50 Zeichen lang sein.', {
        'field': 'displayName',
      });
    }
    user.displayName = trimmed;
    return user.toUser();
  }

  /// bcrypt's limit in the backend (3.14).
  static const maxPasswordBytes = 72;

  @override
  Future<void> changePassword(
    String currentPassword,
    String newPassword,
  ) async {
    final user = (await _wait(delay, access: _Access.duringPasswordChange))!;
    // Order as in the backend: lock, current password, new password.
    _checkLock(user.username);
    if (currentPassword != user.password) {
      _countFailure(user.username);
      throw const ApiException(
        httpStatus: 400,
        code: ErrorCode.wrongPassword,
        message: 'Das bisherige Passwort stimmt nicht.',
      );
    }
    final weak = newPassword.runes.length < 10
        ? 'Das neue Passwort muss mindestens 10 Zeichen lang sein.'
        : utf8.encode(newPassword).length > maxPasswordBytes
        ? 'Das neue Passwort ist zu lang. Erlaubt sind höchstens 72 Byte, '
              'Umlaute zählen doppelt.'
        : newPassword == currentPassword
        ? 'Das neue Passwort muss sich vom bisherigen unterscheiden.'
        : null;
    if (weak != null) {
      throw ApiException(
        httpStatus: 400,
        code: ErrorCode.weakPassword,
        message: weak,
      );
    }
    user
      ..password = newPassword
      ..mustChangePassword = false;
    // All other tokens of the account end; this one stays valid.
    _sessions.removeWhere((t, name) => name == user.username && t != _token);
  }

  @override
  Future<void> recordConsent(String version) async {
    final user = (await _wait(delay))!;
    final trimmed = version.trim();
    final length = trimmed.runes.length;
    if (length < 1 || length > 100 || _controlCharacter.hasMatch(trimmed)) {
      throw _invalid('Bitte die Version des Hinweises schicken.', {
        'field': 'version',
      });
    }
    user.consentVersion = trimmed;
  }

  static final _controlCharacter = RegExp(r'\p{Cc}', unicode: true);

  @override
  Future<User> updateTimeZone(String timeZone) async {
    final user = (await _wait(delay))!;
    if (!isAcceptedTimeZone(timeZone)) {
      throw _invalid('Unbekannte Zeitzone.', {
        'field': 'timeZone',
        'value': timeZone,
      });
    }
    user.timeZone = timeZone;
    return user.toUser();
  }

  // ---------------------------------------------------------------------
  // Routines (3.16–3.27)
  // ---------------------------------------------------------------------

  /// Routines and logs; occurrences, status and progress computed as in
  /// the backend (mock/mock_routines.dart).
  late final _routines = MockRoutines(
    clock: _clock,
    isKnownModule: _isKnownModule,
  );

  @override
  Future<List<Routine>> listRoutines({required String module}) async {
    final user = (await _wait(delay))!;
    return _routines.list(module, user.timeZone);
  }

  @override
  Future<Routine> createRoutine(NewRoutine routine) =>
      createRoutineFromJson(routine.toJson());

  /// Like [createRoutine], with a raw body (`null`: not a JSON object), to
  /// test the backend's checks with any request.
  @visibleForTesting
  Future<Routine> createRoutineFromJson(Map<String, dynamic>? body) async {
    final user = (await _wait(delay))!;
    return _routines.create(body, user.timeZone);
  }

  @override
  Future<Routine> getRoutine(String id) async {
    final user = (await _wait(delay))!;
    return _routines.get(id, user.timeZone);
  }

  @override
  Future<Routine> updateRoutine(String id, RoutinePatch patch) =>
      updateRoutineFromJson(id, patch.toJson());

  /// Like [updateRoutine], with a raw body.
  @visibleForTesting
  Future<Routine> updateRoutineFromJson(
    String id,
    Map<String, dynamic>? body,
  ) async {
    final user = (await _wait(delay))!;
    return _routines.update(id, body, user.timeZone);
  }

  @override
  Future<Routine> pauseRoutine(String id, {PlainDate? until}) =>
      pauseRoutineFromJson(id, {if (until != null) 'until': until.iso});

  /// Like [pauseRoutine], with a raw body.
  @visibleForTesting
  Future<Routine> pauseRoutineFromJson(
    String id,
    Map<String, dynamic>? body,
  ) async {
    final user = (await _wait(delay))!;
    return _routines.pause(id, body, user.timeZone);
  }

  @override
  Future<Routine> resumeRoutine(String id) async {
    final user = (await _wait(delay))!;
    return _routines.resume(id, user.timeZone);
  }

  @override
  Future<void> deleteRoutine(String id) async {
    await _wait(delay);
    _routines.delete(id);
  }

  @override
  Future<OccurrenceList> listOccurrences({
    required String module,
    PlainDate? from,
    PlainDate? to,
  }) => listOccurrencesWithParameters({
    'module': module,
    if (from != null) 'from': from.iso,
    if (to != null) 'to': to.iso,
  });

  /// Like [listOccurrences], with raw query parameters.
  @visibleForTesting
  Future<OccurrenceList> listOccurrencesWithParameters(
    Map<String, String> parameters,
  ) async {
    final user = (await _wait(delay))!;
    return _routines.occurrences(parameters, user.timeZone);
  }

  @override
  Future<List<RoutineLog>> listRoutineLogs(
    String id, {
    PlainDate? from,
    PlainDate? to,
  }) => listRoutineLogsWithParameters(id, {
    if (from != null) 'from': from.iso,
    if (to != null) 'to': to.iso,
  });

  /// Like [listRoutineLogs], with raw query parameters.
  @visibleForTesting
  Future<List<RoutineLog>> listRoutineLogsWithParameters(
    String id,
    Map<String, String> parameters,
  ) async {
    final user = (await _wait(delay))!;
    return _routines.logs(id, parameters, user.timeZone);
  }

  @override
  Future<RoutineLog> addRoutineLog(String id, NewRoutineLog log) =>
      addRoutineLogFromJson(id, log.toJson());

  /// Like [addRoutineLog], with a raw body, e.g. with `takenAt`.
  @visibleForTesting
  Future<RoutineLog> addRoutineLogFromJson(
    String id,
    Map<String, dynamic>? body,
  ) async {
    final user = (await _wait(delay))!;
    return _routines.addLog(id, body, user.timeZone);
  }

  @override
  Future<void> deleteRoutineLog(String id, String logId) async {
    await _wait(delay);
    _routines.deleteLog(id, logId);
  }

  /// Creates a session for [username] without a login and returns its
  /// token, e.g. to start a test already logged in. With [consent] the user
  /// has agreed to the current notice, without it not at all; the password
  /// change is done.
  @visibleForTesting
  String signInForTest(String username, {bool consent = true}) {
    final user = _users[username]!;
    user.mustChangePassword = false;
    user.consentVersion = consent ? notice.consentVersion : null;
    final token = _newToken();
    _sessions[token] = username;
    return token;
  }

  /// Ends every session, as if all tokens expired. The next call answers
  /// `401 UNAUTHORIZED`.
  @visibleForTesting
  void expireAllSessions() => _sessions.clear();

  /// Sets the consent version stored for a user (e.g. an old one).
  @visibleForTesting
  void setConsentVersion(String username, String? version) =>
      _users[username]!.consentVersion = version;

  /// Demands a password change, as if an administrator reset the password
  /// while the app was open. Calls then answer `403`.
  @visibleForTesting
  void requirePasswordChange(String username) =>
      _users[username]!.mustChangePassword = true;

  /// The user as the mock stores it, for checks in tests.
  @visibleForTesting
  User userForTest(String username) => _users[username]!.toUser();

  /// 429 while [username] is locked after failed attempts.
  void _checkLock(String username) {
    final until = _lockedUntil[username];
    if (until == null) return;
    final remaining = until.difference(_clock());
    if (remaining <= Duration.zero) {
      _lockedUntil.remove(username);
      return;
    }
    final seconds = (remaining.inMilliseconds / 1000).ceil();
    throw ApiException(
      httpStatus: 429,
      code: ErrorCode.tooManyAttempts,
      message:
          'Zu viele Fehlversuche. Bitte versuche es in einigen Minuten '
          'erneut.',
      details: {'retryAfterSeconds': seconds},
    );
  }

  void _countFailure(String username) {
    final count = (_failures[username] ?? 0) + 1;
    if (count >= maxFailedAttempts) {
      _failures.remove(username);
      _lockedUntil[username] = _clock().add(lockDuration);
    } else {
      _failures[username] = count;
    }
  }

  /// 43 characters like the backend's token (32 random bytes, base64url).
  String _newToken() {
    final random = Random.secure();
    final bytes = [for (var i = 0; i < 32; i++) random.nextInt(256)];
    return base64Url.encode(bytes).replaceAll('=', '');
  }

  static const _speechUnavailable = ApiException(
    httpStatus: 503,
    code: ErrorCode.modelUnavailable,
    message:
        'Die Spracherkennung ist gerade nicht erreichbar. Bitte später noch '
        'einmal versuchen.',
  );

  // ---------------------------------------------------------------------
  // Shared
  // ---------------------------------------------------------------------

  static const _modelUnavailable = ApiException(
    httpStatus: 503,
    code: ErrorCode.modelUnavailable,
    message:
        'Der Auswertungsdienst ist gerade nicht erreichbar. '
        'Bitte später noch einmal versuchen.',
  );

  /// Waits, then checks the token like the backend: first `401`, then `403`
  /// for a start password (docs/API.md section 1). Returns the logged-in
  /// user, `null` for public endpoints.
  Future<_MockUser?> _wait(
    Duration duration, {
    _Access access = _Access.user,
  }) async {
    if (duration > Duration.zero) await Future<void>.delayed(duration);
    if (simulateError) {
      throw const ApiException(
        httpStatus: 500,
        code: ErrorCode.internalError,
        message:
            'Im Backend ist ein Fehler aufgetreten. '
            'Bitte später noch einmal versuchen.',
      );
    }
    if (access == _Access.public) return null;
    final user = _users[_sessions[_token]];
    if (user == null) {
      throw const ApiException(
        httpStatus: 401,
        code: ErrorCode.unauthorized,
        message: 'Bitte melde dich an.',
      );
    }
    if (user.mustChangePassword && access != _Access.duringPasswordChange) {
      throw const ApiException(
        httpStatus: 403,
        code: ErrorCode.passwordChangeRequired,
        message: 'Bitte ändere zuerst dein Startpasswort.',
      );
    }
    return user;
  }

  static const _limitReached = ApiException(
    httpStatus: 429,
    code: ErrorCode.limitReached,
    message: 'Du hast dein Tageslimit erreicht. Morgen geht es weiter.',
  );
}

/// Who may call an endpoint of the mock.
enum _Access {
  /// `/status` and the login: no token.
  public,

  /// `GET /me`, `POST /me/password`, `POST /auth/logout`: also with a start
  /// password.
  duringPasswordChange,

  /// Everything else.
  user,
}

/// An account of the mock. Mutable: password, display name and consent
/// change through the API.
class _MockUser {
  _MockUser({
    required this.id,
    required this.username,
    required this.password,
    required this.mustChangePassword,
  }) : displayName = username;

  final String id;
  final String username;
  String password;
  String displayName;
  bool mustChangePassword;
  String? consentVersion;

  /// `Europe/Vienna` until the app sets another one (3.12).
  String timeZone = 'Europe/Vienna';

  /// As `GET /me` returns it. The login response has no `timeZone` (3.13),
  /// so [login] passes `withTimeZone: false`.
  User toUser({bool withTimeZone = true}) => User(
    id: id,
    username: username,
    displayName: displayName,
    mustChangePassword: mustChangePassword,
    consentVersion: consentVersion,
    timeZone: withTimeZone ? timeZone : null,
  );
}
