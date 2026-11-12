import '../model/entry.dart';
import 'api_client.dart';

/// Sorts entries by the rules from docs/API.md, section 3.2.
///
/// The app itself never sorts; the backend does. Only the mock needs this
/// function, so that it behaves exactly like the backend.
List<EntrySummary> sortTimeline(
  Iterable<EntrySummary> entries,
  Sort sort,
  SortOrder order,
) {
  final list = entries.toList();
  list.sort((a, b) => compareTimeline(a, b, sort, order));
  return list;
}

/// Compares two entries:
/// 1. By the chosen field in the chosen order.
/// 2. With `sort=documentDate`, entries without a date are always at the
///    end, among themselves by `importedAt` descending (on a tie by `id`
///    descending).
/// 3. For equal values `importedAt` decides in the same order, then `id`.
int compareTimeline(
  EntrySummary a,
  EntrySummary b,
  Sort sort,
  SortOrder order,
) {
  final factor = order == SortOrder.asc ? 1 : -1;

  if (sort == Sort.documentDate) {
    final dateA = a.documentDate;
    final dateB = b.documentDate;
    if (dateA == null && dateB == null) {
      final byImport = b.importedAt.compareTo(a.importedAt);
      return byImport != 0 ? byImport : b.id.compareTo(a.id);
    }
    if (dateA == null) return 1;
    if (dateB == null) return -1;
    final byDate = dateA.compareTo(dateB);
    if (byDate != 0) return factor * byDate;
  }

  final byImport = a.importedAt.compareTo(b.importedAt);
  if (byImport != 0) return factor * byImport;
  return factor * a.id.compareTo(b.id);
}
