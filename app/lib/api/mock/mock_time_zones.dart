import 'package:timezone/data/latest_all.dart' as tz_data;
import 'package:timezone/timezone.dart' as tz;

// Time zones for the mock only. The real backend checks names and computes
// occurrences itself (docs/API.md 3.13, 3.16); the mock needs the IANA
// database to behave the same. `latest_all` holds every zone, also legacy
// names such as Asia/Calcutta and the history (Pacific/Apia in 2011).

var _loaded = false;

/// Loads the zone database once.
void ensureTimeZones() {
  if (_loaded) return;
  tz_data.initializeTimeZones();
  _loaded = true;
}

/// The areas that 3.13 accepts.
final _placeName = RegExp(
  r'^(Africa|America|Antarctica|Arctic|Asia|Atlantic|Australia|Europe|'
  r'Indian|Pacific)(/[A-Z][A-Za-z0-9_-]*){1,2}$',
);

/// The rule of `PATCH /me` for `timeZone` (3.13): `UTC`, `Etc/UTC`, or the
/// name of a place in one of ten areas, at most 64 characters, known to the
/// zone database. Refuses abbreviations, offsets, other spellings and other
/// trees of the database.
bool isAcceptedTimeZone(String name) {
  if (name == 'UTC' || name == 'Etc/UTC') return true;
  if (name.length > 64 || !_placeName.hasMatch(name)) return false;
  return locationOrNull(name) != null;
}

/// The location for an IANA name, or `null` if the database does not know
/// it.
tz.Location? locationOrNull(String name) {
  ensureTimeZones();
  try {
    return tz.getLocation(name);
  } on tz.LocationNotFoundException {
    return null;
  }
}
