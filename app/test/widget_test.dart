import 'package:flutter_test/flutter_test.dart';

import 'package:lifelog/main.dart';

import 'hilfen.dart';

void main() {
  testWidgets('App startet mit Zeitleiste und Navigationsleiste', (
    tester,
  ) async {
    await tester.pumpWidget(LifelogApp(api: schnellerMock()));
    await tester.pumpAndSettle();

    // "Zeitleiste" steht in der AppBar und in der Navigationsleiste.
    expect(find.text('Zeitleiste'), findsNWidgets(2));
    expect(find.text('Hochladen'), findsOneWidget);
    expect(find.text('Fragen'), findsOneWidget);
  });
}
