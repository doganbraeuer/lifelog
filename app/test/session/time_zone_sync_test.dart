// The app sends the phone's time zone (docs/API.md 3.13): at the start of a
// session and when it comes back to the foreground, only if it differs, and
// a zone the backend refuses is not sent again.
import 'package:flutter/widgets.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:lifelog/api/mock_api_client.dart';
import 'package:lifelog/model/user.dart';
import 'package:lifelog/session/guarded_api_client.dart';

import '../helpers.dart';

/// Counts `PATCH /me` with a time zone.
class CountingApi extends GuardedApiClient {
  CountingApi(super.inner)
    : super(onUnauthorized: () {}, onPasswordChangeRequired: () {});

  final sent = <String>[];

  @override
  Future<User> updateTimeZone(String timeZone) {
    sent.add(timeZone);
    return super.updateTimeZone(timeZone);
  }
}

Future<CountingApi> start(
  WidgetTester tester,
  MockApiClient mock,
  FakeDeviceTimeZone phone,
) async {
  final api = CountingApi(mock);
  await tester.pumpWidget(
    testApp(api, store: signedInStore(mock), deviceTimeZone: phone),
  );
  await tester.pumpAndSettle();
  return api;
}

Future<void> backToForeground(WidgetTester tester) async {
  for (final state in [
    AppLifecycleState.inactive,
    AppLifecycleState.hidden,
    AppLifecycleState.paused,
    AppLifecycleState.hidden,
    AppLifecycleState.inactive,
    AppLifecycleState.resumed,
  ]) {
    tester.binding.handleAppLifecycleStateChanged(state);
  }
  await tester.pumpAndSettle();
}

void main() {
  testWidgets('a different zone is sent once at the start', (tester) async {
    final mock = fastMock(signedIn: false);
    final api = await start(tester, mock, FakeDeviceTimeZone('Europe/Lisbon'));
    expect(api.sent, ['Europe/Lisbon']);
    expect(mock.userForTest('admin').timeZone, 'Europe/Lisbon');

    // Back in the foreground with the same zone: nothing more.
    await backToForeground(tester);
    expect(api.sent, ['Europe/Lisbon']);
  });

  testWidgets('the same zone: no request', (tester) async {
    final mock = fastMock(signedIn: false);
    final phone = FakeDeviceTimeZone('Europe/Vienna');
    final api = await start(tester, mock, phone);
    expect(phone.calls, 1);
    expect(api.sent, isEmpty);
  });

  testWidgets('travelling: the new zone is sent when the app comes back', (
    tester,
  ) async {
    final mock = fastMock(signedIn: false);
    final phone = FakeDeviceTimeZone('Europe/Vienna');
    final api = await start(tester, mock, phone);
    phone.zone = 'America/New_York';
    await backToForeground(tester);
    expect(api.sent, ['America/New_York']);
    expect(mock.userForTest('admin').timeZone, 'America/New_York');
  });

  testWidgets('a refused zone is not sent again; the app keeps working', (
    tester,
  ) async {
    final mock = fastMock(signedIn: false);
    final api = await start(tester, mock, FakeDeviceTimeZone('GMT'));
    expect(api.sent, ['GMT']);
    expect(mock.userForTest('admin').timeZone, 'Europe/Vienna');
    await backToForeground(tester);
    expect(api.sent, ['GMT']);
    expect(find.text('Gesundheit'), findsOneWidget);
  });

  testWidgets('no zone from the phone: nothing happens', (tester) async {
    final mock = fastMock(signedIn: false);
    final api = await start(tester, mock, FakeDeviceTimeZone(null));
    expect(api.sent, isEmpty);
  });

  testWidgets('after the login too', (tester) async {
    final mock = fastMock(signedIn: false)..setConsentVersion('admin', null);
    final phone = FakeDeviceTimeZone('Europe/Berlin');
    final store = FakeSessionStore();
    await tester.pumpWidget(testApp(mock, store: store, deviceTimeZone: phone));
    await tester.pumpAndSettle();
    expect(phone.calls, 0, reason: 'not before the login');
    await logIn(tester, 'admin', MockApiClient.mockAdminPassword);
    expect(phone.calls, 0, reason: 'not before the consent');
    await tester.tap(find.text('Zustimmen'));
    await tester.pumpAndSettle();
    expect(mock.userForTest('admin').timeZone, 'Europe/Berlin');
  });
}
