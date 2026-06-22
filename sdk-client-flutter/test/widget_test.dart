import 'package:flutter/material.dart';
import 'package:flutter_test/flutter_test.dart';
import 'package:calltoverify/calltoverify.dart';

Widget _wrap(Widget child) => MaterialApp(home: Scaffold(body: Center(child: child)));

void main() {
  testWidgets('renders the chooser for multiple channels', (tester) async {
    await tester.pumpWidget(_wrap(CallToVerify(
      channels: const [Channel.sms, Channel.call],
      start: (ch) async => const StartResult(
        sessionId: 's',
        instructions: Instructions(number: 'n', channel: 'sms', action: 'a', deepLink: 'd', expiresAt: 'e'),
      ),
      status: (id) async => const StatusResult(status: 'pending'),
    )));

    expect(find.text('Verify your number'), findsOneWidget);
    expect(find.text('Text us a code'), findsOneWidget);
    expect(find.text('Give a missed call'), findsOneWidget);
    expect(find.text('Totally free'), findsOneWidget);
  });

  testWidgets('a single channel auto-starts into the SMS instruction', (tester) async {
    await tester.pumpWidget(_wrap(CallToVerify(
      channels: const [Channel.sms],
      start: (ch) async => StartResult(
        sessionId: 's',
        instructions: Instructions(
          number: '+8801700000001',
          code: '482913',
          channel: 'sms',
          action: 'a',
          deepLink: 'sms:+8801700000001',
          expiresAt: DateTime.now().add(const Duration(minutes: 2)).toIso8601String(),
        ),
      ),
      status: (id) async => const StatusResult(status: 'pending'),
    )));

    await tester.pump(); // resolve begin()
    await tester.pump(); // apply state change

    expect(find.text('Open messages'), findsOneWidget);
    expect(find.text('4'), findsWidgets); // a digit chip rendered

    // Dispose the widget so its polling/countdown timers are cancelled before teardown.
    await tester.pumpWidget(const SizedBox());
  });
}
