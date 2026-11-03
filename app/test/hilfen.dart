import 'package:flutter_test/flutter_test.dart';
import 'package:lifelog/api/api_fehler.dart';
import 'package:lifelog/api/mock_api_client.dart';

/// Erwartet, dass ein Future mit einem [ApiFehler] mit diesem Code scheitert.
Matcher wirftApiFehler(String code, {int? status}) => throwsA(
  isA<ApiFehler>()
      .having((f) => f.code, 'code', code)
      .having((f) => f.httpStatus, 'httpStatus', status ?? anything),
);

/// Mock ohne künstliche Wartezeit und mit fester Uhr.
MockApiClient schnellerMock({
  bool mitBeispieldaten = true,
  DateTime Function()? uhr,
}) => MockApiClient(
  mitBeispieldaten: mitBeispieldaten,
  verzoegerung: Duration.zero,
  uploadVerzoegerung: Duration.zero,
  uhr: uhr ?? () => DateTime.utc(2026, 11, 4, 9, 30, 0),
);

const laborId = '6727a1f3c2b4e5d6f7a8b9c0';
const arztbriefId = '6727a20bc2b4e5d6f7a8b9d4';
const sonstigesId = '6727a2a9c2b4e5d6f7a8b9e7';
