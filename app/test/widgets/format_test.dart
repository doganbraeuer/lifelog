import 'package:flutter_test/flutter_test.dart';
import 'package:lifelog/widgets/format.dart';

void main() {
  test('file size', () {
    expect(formatSize(512), '512 Bytes');
    expect(formatSize(340 * 1024), '340 KB');
    expect(formatSize((2.4 * 1024 * 1024).round()), '2,4 MB');
    expect(formatSize(10 * 1024 * 1024), '10,0 MB');
  });

  test('point in time is shown in local time', () {
    // Independent of the machine's time zone: create 19:42 local time,
    // pass it as UTC, and 19:42 must come out again.
    final local = DateTime(2026, 11, 3, 19, 42);
    expect(formatLocalTimestamp(local.toUtc()), '03.11.2026, 19:42');
  });
}
