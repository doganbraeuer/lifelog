import 'package:flutter_test/flutter_test.dart';

import 'package:lifelog/main.dart';

import 'hilfen.dart';

void main() {
  testWidgets('App startet und zeigt Titel und Datenquelle', (tester) async {
    await tester.pumpWidget(LifelogApp(api: schnellerMock()));

    expect(find.text('Lifelog'), findsOneWidget);
    expect(find.text('Datenquelle: Mock'), findsOneWidget);
  });
}
