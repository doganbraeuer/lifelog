import 'package:flutter/services.dart';
import 'package:flutter_timezone/flutter_timezone.dart';

/// The phone's time zone as an IANA name, e.g. `Europe/Vienna`. Behind an
/// interface so that tests can use a fake.
///
/// `DateTime.timeZoneName` only gives abbreviations such as `CET`, which
/// the backend refuses (docs/API.md 3.13); the package flutter_timezone
/// asks the operating system for the IANA name.
abstract interface class DeviceTimeZone {
  /// `null` if the phone does not tell.
  Future<String?> name();
}

class PluginDeviceTimeZone implements DeviceTimeZone {
  @override
  Future<String?> name() async {
    try {
      final info = await FlutterTimezone.getLocalTimezone();
      return info.identifier;
    } on PlatformException {
      return null;
    } on ArgumentError {
      return null;
    }
  }
}
