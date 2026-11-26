// „Neue Routine“ and „Routine bearbeiten“. The mock's clock is 2026-11-04
// 09:30 UTC, a Wednesday.
import 'package:flutter/material.dart';
import 'package:flutter_localizations/flutter_localizations.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:lifelog/api/mock_api_client.dart';
import 'package:lifelog/model/plain_date.dart';
import 'package:lifelog/model/routine.dart';
import 'package:lifelog/screens/routine_form_screen.dart';
import 'package:lifelog/session/guarded_api_client.dart';

import '../helpers.dart';
import 'routines_screen_test.dart' show card, openRoutines;

final today = PlainDate(2026, 11, 4);

/// Records the patches the form sends.
class RecordingApi extends GuardedApiClient {
  RecordingApi(super.inner)
    : super(onUnauthorized: () {}, onPasswordChangeRequired: () {});

  final patches = <RoutinePatch>[];

  @override
  Future<Routine> updateRoutine(String id, RoutinePatch patch) {
    patches.add(patch);
    return super.updateRoutine(id, patch);
  }
}

Future<void> openForm(WidgetTester tester, MockApiClient mock) async {
  await openRoutines(tester, mock);
  await tester.tap(find.text('Neue Routine').last);
  await tester.pumpAndSettle();
}

/// Types a time into the time picker (input mode) and confirms it.
Future<void> enterTime(WidgetTester tester, String hour, String minute) async {
  final fields = find.descendant(
    of: find.byType(Dialog),
    matching: find.byType(TextField),
  );
  await tester.enterText(fields.at(0), hour);
  await tester.enterText(fields.at(1), minute);
  await tester.tap(find.text('OK'));
  await tester.pumpAndSettle();
}

Future<void> save(WidgetTester tester) async {
  await tester.ensureVisible(find.text('Speichern'));
  await tester.tap(find.text('Speichern'));
  await tester.pumpAndSettle();
}

String preview(WidgetTester tester) =>
    tester.widget<Text>(find.byKey(const ValueKey('routine-preview'))).data!;

void main() {
  testWidgets('daily with an amount and two times', (tester) async {
    final mock = fastMock(withSampleData: false);
    await openForm(tester, mock);
    expect(find.text('Neue Routine'), findsOneWidget);

    await tester.enterText(
      find.byKey(const ValueKey('routine-name')),
      'Vitamin D',
    );
    await tester.tap(find.text('Mit Menge'));
    await tester.pumpAndSettle();
    await tester.enterText(find.byKey(const ValueKey('routine-amount')), '2');
    await tester.tap(find.widgetWithText(ActionChip, 'Stück'));
    await tester.pumpAndSettle();

    await tester.tap(find.text('Uhrzeit hinzufügen'));
    await tester.pumpAndSettle();
    await enterTime(tester, '20', '00');
    expect(preview(tester), 'Täglich um 08:00 und 20:00');

    await save(tester);
    expect(find.text('„Vitamin D“ angelegt.'), findsOneWidget);
    expect(card('Vitamin D'), findsOne);

    final routine = (await mock.listRoutines(module: 'health')).single;
    expect(routine.amount, 2);
    expect(routine.unit, 'Stück');
    expect(routine.schedule.times, ['08:00', '20:00']);
    expect(routine.startDate, today);
    expect(routine.endDate, isNull);
  });

  testWidgets('hourly in a window, every 2 hours, with an end', (tester) async {
    final mock = fastMock(withSampleData: false);
    await openForm(tester, mock);
    await tester.enterText(
      find.byKey(const ValueKey('routine-name')),
      'Wasser',
    );
    await tester.tap(find.text('stündlich'));
    await tester.pumpAndSettle();
    expect(preview(tester), 'Stündlich von 08:00 bis 20:00');

    await tester.tap(find.byTooltip('Intervall größer'));
    await tester.pumpAndSettle();
    expect(find.text('alle 2 Stunden'), findsOneWidget);
    expect(preview(tester), 'Alle 2 Stunden von 08:00 bis 20:00');

    await tester.tap(find.text('bis 20:00'));
    await tester.pumpAndSettle();
    await enterTime(tester, '18', '00');

    await tester.ensureVisible(find.text('Ende: ohne'));
    await tester.tap(find.text('Ende: ohne'));
    await tester.pumpAndSettle();
    await tester.tap(find.text('18'));
    await tester.tap(find.text('OK'));
    await tester.pumpAndSettle();
    expect(
      preview(tester),
      'Alle 2 Stunden von 08:00 bis 18:00, bis 18.11.2026',
    );

    await save(tester);
    final routine = (await mock.listRoutines(module: 'health')).single;
    expect(
      routine.schedule,
      const RoutineSchedule.hourly(
        interval: 2,
        windowStart: '08:00',
        windowEnd: '18:00',
      ),
    );
    expect(routine.endDate, PlainDate(2026, 11, 18));
  });

  testWidgets('weekly: today is preselected, another day added', (
    tester,
  ) async {
    final mock = fastMock(withSampleData: false);
    await openForm(tester, mock);
    await tester.enterText(
      find.byKey(const ValueKey('routine-name')),
      'Eisbaden',
    );
    await tester.tap(find.text('wöchentlich'));
    await tester.pumpAndSettle();
    expect(preview(tester), 'Wöchentlich am Mi um 08:00');
    await tester.tap(find.widgetWithText(FilterChip, 'Fr'));
    await tester.pumpAndSettle();
    expect(preview(tester), 'Wöchentlich am Mi und Fr um 08:00');

    // Without any weekday: the backend's text, nothing is sent.
    await tester.tap(find.widgetWithText(FilterChip, 'Mi'));
    await tester.tap(find.widgetWithText(FilterChip, 'Fr'));
    await tester.pumpAndSettle();
    await save(tester);
    expect(find.text('Bitte mindestens einen Wochentag wählen.'), findsOne);
    expect(await mock.listRoutines(module: 'health'), isEmpty);
  });

  testWidgets('checks before sending: name and amount', (tester) async {
    final mock = fastMock(withSampleData: false);
    await openForm(tester, mock);
    await tester.tap(find.text('Mit Menge'));
    await tester.pumpAndSettle();
    await tester.enterText(find.byKey(const ValueKey('routine-amount')), '0');
    await save(tester);
    expect(find.text('Der Name muss 1 bis 60 Zeichen lang sein.'), findsOne);
    expect(
      find.text('Die Menge muss größer als 0 und höchstens 100.000 sein.'),
      findsOne,
    );
    expect(await mock.listRoutines(module: 'health'), isEmpty);
  });

  testWidgets('the backend refuses a taken name: shown at the name', (
    tester,
  ) async {
    final mock = fastMock();
    await openForm(tester, mock);
    await tester.enterText(
      find.byKey(const ValueKey('routine-name')),
      'vitamin d',
    );
    await save(tester);
    expect(
      find.text('Eine Routine mit diesem Namen gibt es schon.'),
      findsOneWidget,
    );
    expect(find.text('Neue Routine'), findsOneWidget, reason: 'still open');
  });

  group('editing', () {
    Future<(RecordingApi, Routine)> openEdit(WidgetTester tester) async {
      final mock = fastMock();
      final api = RecordingApi(mock);
      final water = (await mock.listRoutines(module: 'health'))
          .firstWhere((r) => r.name == 'Wasser');
      useLargeScreen(tester);
      await tester.pumpWidget(
        MaterialApp(
          localizationsDelegates: GlobalMaterialLocalizations.delegates,
          supportedLocales: const [Locale('de')],
          locale: const Locale('de'),
          home: Builder(
            builder: (context) => Scaffold(
              body: TextButton(
                onPressed: () => Navigator.push(
                  context,
                  MaterialPageRoute<Routine>(
                    builder: (_) => RoutineFormScreen(
                      api: api,
                      moduleId: 'health',
                      today: today,
                      routine: water,
                    ),
                  ),
                ),
                child: const Text('öffnen'),
              ),
            ),
          ),
        ),
      );
      await tester.tap(find.text('öffnen'));
      await tester.pumpAndSettle();
      return (api, water);
    }

    testWidgets('the form starts with the routine', (tester) async {
      await openEdit(tester);
      expect(find.text('Routine bearbeiten'), findsOneWidget);
      expect(find.text('Wasser'), findsOneWidget);
      expect(find.text('250'), findsOneWidget);
      expect(find.text('ml'), findsWidgets);
      expect(preview(tester), 'Stündlich von 08:00 bis 20:00, ab 02.11.2026');
    });

    testWidgets('only the changed fields are sent', (tester) async {
      final (api, water) = await openEdit(tester);
      await tester.enterText(
        find.byKey(const ValueKey('routine-name')),
        'Wasser trinken',
      );
      await save(tester);
      expect(api.patches.single.toJson(), {'name': 'Wasser trinken'});
      expect(find.text('öffnen'), findsOneWidget, reason: 'closed');
      expect((await api.getRoutine(water.id)).name, 'Wasser trinken');
    });

    testWidgets('nothing changed: no request', (tester) async {
      final (api, _) = await openEdit(tester);
      await save(tester);
      expect(api.patches, isEmpty);
      expect(find.text('öffnen'), findsOneWidget);
    });

    testWidgets('removing the amount clears it', (tester) async {
      final (api, _) = await openEdit(tester);
      await tester.tap(find.text('Mit Menge'));
      await tester.pumpAndSettle();
      await save(tester);
      expect(api.patches.single.toJson(), {'amount': null});
    });
  });
}
