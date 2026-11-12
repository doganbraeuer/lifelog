import '../api/api_client.dart';
import 'entry.dart';

/// German month names for display (without `intl`).
const monthNames = [
  'Januar',
  'Februar',
  'März',
  'April',
  'Mai',
  'Juni',
  'Juli',
  'August',
  'September',
  'Oktober',
  'November',
  'Dezember',
];

/// Title of the group for entries without a document date.
const noDateTitle = 'Ohne Datum';

/// A section of the timeline axis, e.g. „Oktober 2026“.
class TimelineGroup {
  const TimelineGroup(this.title, this.entries);

  final String title;
  final List<EntrySummary> entries;
}

/// Splits the entries into month groups for the timeline axis.
///
/// - The order from the backend is kept; consecutive entries with the same
///   month form a group.
/// - The month comes from the chosen sort field: for the document date from
///   the plain date (without time zone), for the import date from local
///   time (as it is also displayed).
/// - Entries without a document date go into their own group
///   [noDateTitle] at the end.
List<TimelineGroup> groupTimeline(List<EntrySummary> entries, Sort sort) {
  final groups = <(String, List<EntrySummary>)>[];
  final withoutDate = <EntrySummary>[];
  for (final entry in entries) {
    final title = _monthTitle(entry, sort);
    if (title == null) {
      withoutDate.add(entry);
    } else if (groups.isNotEmpty && groups.last.$1 == title) {
      groups.last.$2.add(entry);
    } else {
      groups.add((title, [entry]));
    }
  }
  return [
    for (final (title, list) in groups)
      TimelineGroup(title, List.unmodifiable(list)),
    if (withoutDate.isNotEmpty)
      TimelineGroup(noDateTitle, List.unmodifiable(withoutDate)),
  ];
}

String? _monthTitle(EntrySummary entry, Sort sort) {
  switch (sort) {
    case Sort.documentDate:
      final date = entry.documentDate;
      if (date == null) return null;
      return '${monthNames[date.month - 1]} ${date.year}';
    case Sort.importedAt:
      final local = entry.importedAt.toLocal();
      return '${monthNames[local.month - 1]} ${local.year}';
  }
}
