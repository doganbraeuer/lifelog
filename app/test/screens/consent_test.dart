// Consent notice (docs/API.md 3.15).
import 'package:flutter/material.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:lifelog/api/mock_api_client.dart';
import 'package:lifelog/session/consent_text.dart';

import '../helpers.dart';

void main() {
  testWidgets('shows the whole text, scrollable, with the version', (
    tester,
  ) async {
    final mock = fastMock();
    await tester.pumpWidget(
      testApp(mock, store: signedInStore(mock, consent: false)),
    );
    await tester.pumpAndSettle();

    expect(find.text(consentTitle), findsOneWidget);
    expect(find.byType(Scrollable), findsWidgets);
    await tester.scrollUntilVisible(find.text('Version $consentVersion'), 200);
    expect(find.textContaining('dylan.mueller@stud.plus.ac.at'), findsOne);
  });

  testWidgets('„Zustimmen“ sends the version and opens the app', (
    tester,
  ) async {
    final mock = fastMock();
    await tester.pumpWidget(
      testApp(mock, store: signedInStore(mock, consent: false)),
    );
    await tester.pumpAndSettle();

    await tester.tap(find.widgetWithText(FilledButton, 'Zustimmen'));
    await tester.pumpAndSettle();

    expect(mock.userForTest('admin').consentVersion, consentVersion);
    expect(find.text('Gesundheit'), findsOneWidget);
  });

  testWidgets('„Nicht jetzt“ logs out', (tester) async {
    final mock = fastMock(signedIn: false);
    final store = await startSignedOut(tester, mock);
    await logIn(tester, 'admin', MockApiClient.mockAdminPassword);

    await tester.tap(find.widgetWithText(OutlinedButton, 'Nicht jetzt'));
    await tester.pumpAndSettle();

    expect(store.session, isNull);
    expect(usernameField(), findsOneWidget);
    expect(mock.userForTest('admin').consentVersion, isNull);
  });

  testWidgets('an old version shows the notice again', (tester) async {
    final mock = fastMock();
    final store = signedInStore(mock);
    mock.setConsentVersion('admin', '2026-01-01');
    await tester.pumpWidget(testApp(mock, store: store));
    await tester.pumpAndSettle();

    expect(find.text(consentTitle), findsOneWidget);
    expect(find.text('Gesundheit'), findsNothing);
  });
}
