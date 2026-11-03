import 'package:flutter_test/flutter_test.dart';

import 'package:lifelog/main.dart';

void main() {
  testWidgets('App startet und zeigt den Titel', (tester) async {
    await tester.pumpWidget(const LifelogApp());

    expect(find.text('Lifelog'), findsOneWidget);
  });
}
