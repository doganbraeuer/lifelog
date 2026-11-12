import 'dart:io';

import 'package:flutter_test/flutter_test.dart';
import 'package:lifelog/platform/voice_recorder.dart';

void main() {
  late Directory temp;
  late RecordingFiles files;

  setUp(() async {
    temp = await Directory.systemTemp.createTemp('lifelog_test_');
    files = RecordingFiles(
      Directory('${temp.path}${Platform.pathSeparator}voice_notes'),
      clock: () => DateTime.utc(2026, 11, 12, 10),
    );
  });

  tearDown(() async {
    if (await temp.exists()) await temp.delete(recursive: true);
  });

  test('newPath: M4A file in the recording folder, folder exists', () async {
    final path = await files.newPath();
    expect(path, startsWith(files.directory.path));
    expect(path, endsWith('.m4a'));
    expect(path, contains('note_'));
    expect(await files.directory.exists(), isTrue);
  });

  test('deleteAll removes the recording', () async {
    final path = await files.newPath();
    await File(path).writeAsBytes([1, 2, 3]);
    expect(await files.list(), hasLength(1));

    await files.deleteAll();

    expect(await File(path).exists(), isFalse);
    expect(await files.list(), isEmpty);
  });

  test('a new recording deletes leftovers (e.g. after a crash)', () async {
    await files.directory.create(recursive: true);
    final leftover = File(
      '${files.directory.path}${Platform.pathSeparator}note_1.m4a',
    );
    await leftover.writeAsBytes([1]);

    final path = await files.newPath();

    expect(await leftover.exists(), isFalse);
    expect(await files.list(), isEmpty); // the new file is not written yet
    expect(path, isNot(leftover.path));
  });

  test('deleteAll without a folder is fine', () async {
    await files.deleteAll();
    expect(await files.list(), isEmpty);
  });

  test('deleteAll only touches the recording folder', () async {
    final other = File('${temp.path}${Platform.pathSeparator}other.txt');
    await other.writeAsString('bleibt');
    await File(await files.newPath()).writeAsBytes([1]);

    await files.deleteAll();

    expect(await other.exists(), isTrue);
  });
}
