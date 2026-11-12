import 'dart:convert';
import 'dart:math';

import 'package:crypto/crypto.dart';
import 'package:flutter/foundation.dart';

import '../model/answer.dart';
import '../model/plain_date.dart';
import '../model/entry.dart';
import '../model/entry_patch.dart';
import '../model/json_helpers.dart';
import '../module/module_catalog.dart';
import 'api_client.dart';
import 'api_exception.dart';
import 'mime_type.dart';
import 'mock/mock_data.dart';
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
/// Errors for trying out by hand:
/// - An upload with `unreadable`, `extractionerror` or `modelerror` in
///   the file name returns 422, 502 or 503.
/// - A question with the word `modelerror` returns 503.
class MockApiClient implements ApiClient {
  MockApiClient({
    bool withSampleData = true,
    this.delay = const Duration(milliseconds: 300),
    this.uploadDelay = const Duration(seconds: 2),
    DateTime Function()? clock,
    ModuleCatalog? catalog,
  }) : _clock = clock ?? (() => DateTime.now().toUtc()),
       _catalog = catalog ?? ModuleCatalog.defaults() {
    if (withSampleData) {
      for (final e in sampleData()) {
        final id = e.json['id'] as String;
        _entries[id] = e.json;
        _files[id] = e.bytes;
      }
    }
  }

  /// Artificial delay per call, so that loading indicators become visible.
  final Duration delay;

  /// Delay for uploads (the real backend needs up to 60 s).
  final Duration uploadDelay;

  /// If `true`, every call fails with `500 INTERNAL_ERROR`.
  bool simulateError = false;

  final DateTime Function() _clock;

  /// Allowed types per module, as the backend checks them too.
  final ModuleCatalog _catalog;
  final _entries = <String, Map<String, dynamic>>{};
  final _files = <String, Uint8List>{};
  final _random = Random();
  var _counter = 0;

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
            'Erlaubt sind JPEG, PNG, HEIC und PDF.',
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

    return _store(
      module: moduleId,
      type: _typeFor(moduleId, 'other'),
      documentDate: date,
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
  }) {
    final id = _newId();
    final json = <String, dynamic>{
      'id': id,
      'module': module,
      'type': type,
      'documentDate': documentDate?.iso,
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
    _entries[id] = json;
    _files[id] = Uint8List.fromList(bytes);
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
    for (final (field, newValue) in changes) {
      _set(json, field, newValue, timestamp);
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
  void _set(
    Map<String, dynamic> json,
    String field,
    String? newValue,
    String correctedAt,
  ) {
    final old = _read(json, field);
    if (old == newValue) return;

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
  // Shared
  // ---------------------------------------------------------------------

  static const _modelUnavailable = ApiException(
    httpStatus: 503,
    code: ErrorCode.modelUnavailable,
    message:
        'Der Auswertungsdienst ist gerade nicht erreichbar. '
        'Bitte später noch einmal versuchen.',
  );

  Future<void> _wait(Duration duration) async {
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
  }
}
