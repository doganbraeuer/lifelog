import 'package:flutter_test/flutter_test.dart';
import 'package:lifelog/session/password_rules.dart';

String? check(
  String next, {
  String current = 'dummy-start-1',
  String? repeat,
}) => checkNewPassword(current: current, next: next, repeat: repeat ?? next);

void main() {
  group('checkNewPassword', () {
    test('a valid password passes', () {
      expect(check('Sonnenblume am Berg'), isNull);
    });

    test('current password missing', () {
      expect(check('Sonnenblume am Berg', current: ''), contains('bisheriges'));
    });

    test('9 characters are too short, 10 are enough', () {
      expect(check('abcdefghi'), contains('mindestens 10 Zeichen'));
      expect(check('abcdefghij'), isNull);
    });

    test('characters, not bytes: 10 umlauts are 10 characters', () {
      expect(check('ääääääääää'), isNull);
    });

    test('umlauts over 72 bytes, though fewer than 72 characters', () {
      // 37 × „ä“ = 37 characters, but 74 bytes in UTF-8.
      final umlauts = 'ä' * 37;
      expect(umlauts.length, lessThan(72));
      expect(check(umlauts), contains('höchstens 72 Byte'));
      // 36 × „ä“ = 72 bytes: still allowed.
      expect(check('ä' * 36), isNull);
    });

    test('an emoji counts as one character, four bytes', () {
      expect(check('😀' * 10), isNull);
      expect(check('😀' * 19), contains('72 Byte'));
    });

    test('equal to the current one', () {
      expect(
        check('dummy-start-1', current: 'dummy-start-1'),
        contains('unterscheiden'),
      );
    });

    test('repetition differs', () {
      expect(
        check('Sonnenblume am Berg', repeat: 'Sonnenblume am Berk'),
        'Die beiden neuen Passwörter stimmen nicht überein.',
      );
    });
  });

  group('waitHint', () {
    test('singular for one minute, plural otherwise, rounded up', () {
      expect(waitHint(1), 'Bitte warte etwa 1 Minute.');
      expect(waitHint(60), 'Bitte warte etwa 1 Minute.');
      expect(waitHint(61), 'Bitte warte etwa 2 Minuten.');
      expect(waitHint(180), 'Bitte warte etwa 3 Minuten.');
      expect(waitHint(900), 'Bitte warte etwa 15 Minuten.');
    });

    test('0 seconds still says one minute', () {
      expect(waitHint(0), 'Bitte warte etwa 1 Minute.');
    });
  });
}
