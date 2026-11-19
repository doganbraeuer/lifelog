// A session that ends while the app is in use (docs/API.md section 1):
// one central place logs out, closes every page and shows the login.
import 'package:flutter_test/flutter_test.dart';
import 'package:lifelog/screens/module_shell.dart';
import 'package:lifelog/session/session_controller.dart';

import '../helpers.dart';

void main() {
  testWidgets('401 while loading the timeline: login with notice', (
    tester,
  ) async {
    final mock = fastMock();
    final store = signedInStore(mock);
    final localData = FakeLocalData();
    await tester.pumpWidget(testApp(mock, store: store, localData: localData));
    await tester.pumpAndSettle();

    // E.g. the token expired or an administrator revoked it.
    mock.expireAllSessions();
    await openHealth(tester);

    expect(usernameField(), findsOneWidget);
    expect(find.text(reloginNotice), findsOneWidget);
    expect(store.session, isNull);
    expect(localData.clears, 1);
    // The module page is closed, not just covered.
    expect(find.byType(ModuleShell), findsNothing);
  });

  testWidgets('401 deep inside (entry open): every page closes', (
    tester,
  ) async {
    useLargeScreen(tester);
    final mock = fastMock();
    final store = signedInStore(mock);
    await tester.pumpWidget(testApp(mock, store: store));
    await tester.pumpAndSettle();
    await openHealth(tester);

    mock.expireAllSessions();
    await tester.tap(find.text('Labor Muster GmbH, Salzburg').first);
    await tester.pumpAndSettle();

    expect(usernameField(), findsOneWidget);
    expect(find.text(reloginNotice), findsOneWidget);
    expect(store.session, isNull);
    expect(find.byTooltip('Zurück'), findsNothing);
  });

  testWidgets('403 PASSWORD_CHANGE_REQUIRED: to the password screen', (
    tester,
  ) async {
    final mock = fastMock();
    final store = signedInStore(mock);
    await tester.pumpWidget(testApp(mock, store: store));
    await tester.pumpAndSettle();

    mock.requirePasswordChange('admin');
    await openHealth(tester);

    expect(find.text('Eigenes Passwort festlegen'), findsOneWidget);
    expect(find.byType(ModuleShell), findsNothing);
    // Still logged in: the token stays, only the password must change.
    expect(store.session, isNotNull);
  });
}
