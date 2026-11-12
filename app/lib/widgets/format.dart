/// File size for display, e.g. `340 KB` or `2,4 MB`.
String formatSize(int bytes) {
  const kb = 1024;
  const mb = 1024 * 1024;
  if (bytes < kb) return '$bytes Bytes';
  if (bytes < mb) return '${(bytes / kb).round()} KB';
  return '${(bytes / mb).toStringAsFixed(1).replaceAll('.', ',')} MB';
}

/// Point in time in the device's local time, e.g. `03.11.2026, 19:42`.
///
/// Only for `importedAt` and correction times. For `documentDate` there is
/// `PlainDate.display`, which is never converted into a time zone.
String formatLocalTimestamp(DateTime timestamp) {
  final local = timestamp.toLocal();
  String two(int n) => n.toString().padLeft(2, '0');
  return '${two(local.day)}.${two(local.month)}.${local.year}, '
      '${two(local.hour)}:${two(local.minute)}';
}
