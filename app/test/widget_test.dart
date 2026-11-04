import 'package:flutter_test/flutter_test.dart';

import 'hilfen.dart';

void main() {
  testWidgets('App startet mit Zeitleiste und Navigationsleiste', (
    tester,
  ) async {
    await tester.pumpWidget(testApp(schnellerMock()));
    await tester.pumpAndSettle();
    await zuGesundheit(tester);

    // "Zeitleiste" steht in der AppBar und in der Navigationsleiste.
    expect(find.text('Zeitleiste'), findsNWidgets(2));
    expect(find.text('Hochladen'), findsOneWidget);
    expect(find.text('Fragen'), findsOneWidget);
  });
}
