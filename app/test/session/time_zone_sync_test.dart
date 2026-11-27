// The app sends the phone's time zone (docs/API.md 3.13): at the start of a
// session and when it comes back to the foreground, only if it differs, and
// a zone the backend refuses is not sent again.
import 'dart:async';

import 'package:flutter/widgets.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:lifelog/api/mock_api_client.dart';
import 'package:lifelog/model/user.dart';
import 'package:lifelog/session/guarded_api_client.dart';

import '../helpers.dart';
import '../screens/routines_screen_test.dart' show inCard, navItem;

/// Counts `PATCH /me` with a time zone.
class CountingApi extends GuardedApiClient {
  CountingApi(super.inner)
    : super(onUnauthorized: () {}, onPasswordChangeRequired: () {});

  final sent = <String>[];

  /// While set, `PATCH /me` waits for it, so that a screen loads first.
  Completer<void>? hold;

  @override
  Future<User> updateTimeZone(String timeZone) async {
    sent.add(timeZone);
    await hold?.future;
    return super.updateTimeZone(timeZone);
  }
}

Future<CountingApi> start(
  WidgetTester tester,
  MockApiClient mock,
  FakeDeviceTimeZone phone, {
  DateTime Function()? clock,
}) async {
  final api = CountingApi(mock);
  await tester.pumpWidget(
    testApp(
      api,
      store: signedInStore(mock),
      deviceTimeZone: phone,
      clock: clock,
    ),
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

  testWidgets('travelling with the routines tab open: it loads again once '
      'the new zone is stored', (tester) async {
    // Found on the phone: the tab loaded while PATCH /me ran and showed the
    // progress of the old zone's day.
    // 23:30 in Lisbon on 4 November, already 5 November in Vienna.
    DateTime late() => DateTime.utc(2026, 11, 4, 23, 30);
    final mock = fastMock(signedIn: false, clock: late);
    final phone = FakeDeviceTimeZone('Europe/Lisbon');
    useLargeScreen(tester);
    final api = await start(tester, mock, phone, clock: late);
    await openHealth(tester);
    await tester.tap(navItem('Routinen'));
    await tester.pumpAndSettle();
    await tester.tap(inCard('Vitamin D', find.text('Ja')));
    await tester.pumpAndSettle();
    expect(inCard('Vitamin D', find.text('heute 2 von 2 Stück')), findsOne);

    // Back in Vienna; the tab loads before the zone is stored.
    api.hold = Completer<void>();
    phone.zone = 'Europe/Vienna';
    await backToForeground(tester);
    expect(inCard('Vitamin D', find.text('heute 2 von 2 Stück')), findsOne);

    api.hold!.complete();
    await tester.pumpAndSettle();
    expect(api.sent, ['Europe/Lisbon', 'Europe/Vienna']);
    // 5 November in Vienna: nothing logged yet.
    expect(inCard('Vitamin D', find.text('heute 0 von 2 Stück')), findsOne);
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
