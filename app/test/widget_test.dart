import 'package:flutter_test/flutter_test.dart';

import 'helpers.dart';

void main() {
  testWidgets('app starts with timeline and navigation bar', (tester) async {
    await tester.pumpWidget(testApp(fastMock()));
    await tester.pumpAndSettle();
    await openHealth(tester);

    // "Zeitleiste" is in the app bar and in the navigation bar.
    expect(find.text('Zeitleiste'), findsNWidgets(2));
    expect(find.text('Hochladen'), findsOneWidget);
    expect(find.text('Fragen'), findsOneWidget);
  });
}
