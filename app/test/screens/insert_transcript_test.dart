import 'package:flutter/services.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:lifelog/screens/voice_note_view.dart';

/// A field value with the cursor at [cursor], or no cursor (`null`).
TextEditingValue field(String text, [int? cursor]) => TextEditingValue(
  text: text,
  selection: cursor == null
      ? const TextSelection.collapsed(offset: -1)
      : TextSelection.collapsed(offset: cursor),
);

void main() {
  test('empty field: only the text, cursor behind it', () {
    final value = insertTranscript(field(''), 'Puls 64');
    expect(value.text, 'Puls 64');
    expect(value.selection, const TextSelection.collapsed(offset: 7));
  });

  test('the transcript is trimmed', () {
    expect(insertTranscript(field(''), '  Puls 64 \n').text, 'Puls 64');
  });

  test('cursor at the end of a word: a space before it', () {
    final value = insertTranscript(field('Befund:', 7), 'Puls 64');
    expect(value.text, 'Befund: Puls 64');
    expect(value.selection, const TextSelection.collapsed(offset: 15));
  });

  test('cursor after whitespace: no extra separator', () {
    expect(
      insertTranscript(field('Befund: ', 8), 'Puls 64').text,
      'Befund: Puls 64',
    );
    expect(
      insertTranscript(field('Befund:\n', 8), 'Puls 64').text,
      'Befund:\nPuls 64',
    );
  });

  test('cursor inside the text: spaces on both sides', () {
    final value = insertTranscript(field('Befund:Ende', 7), 'Puls 64');
    expect(value.text, 'Befund: Puls 64 Ende');
    // Behind the inserted text, before the space after it.
    expect(value.selection, const TextSelection.collapsed(offset: 15));
  });

  test('cursor before whitespace: no space after it', () {
    expect(
      insertTranscript(field('Befund: Ende', 7), 'Puls 64').text,
      'Befund: Puls 64 Ende',
    );
  });

  test('cursor before punctuation: no space after it', () {
    expect(insertTranscript(field('Puls .', 5), '64/min').text, 'Puls 64/min.');
    expect(insertTranscript(field('Puls,', 4), '64').text, 'Puls 64,');
  });

  test('cursor at the start: no space before, one after', () {
    expect(insertTranscript(field('Ende', 0), 'Puls 64').text, 'Puls 64 Ende');
  });

  test('a selection is replaced, as typing would', () {
    const value = TextEditingValue(
      text: 'Puls XX/min',
      selection: TextSelection(baseOffset: 5, extentOffset: 7),
    );
    final result = insertTranscript(value, '64');
    expect(result.text, 'Puls 64/min');
    expect(result.selection, const TextSelection.collapsed(offset: 7));
  });

  test('no cursor: at the end, on a new line', () {
    final value = insertTranscript(field('Kopfzeile'), 'Puls 64');
    expect(value.text, 'Kopfzeile\nPuls 64');
    expect(value.selection, const TextSelection.collapsed(offset: 17));
  });

  test('no cursor, text already ends with a line break: no second one', () {
    expect(
      insertTranscript(field('Kopfzeile\n'), 'Puls 64').text,
      'Kopfzeile\nPuls 64',
    );
  });

  test('an empty transcript changes nothing', () {
    final before = field('Befund', 3);
    expect(insertTranscript(before, '  '), before);
  });
}
