import 'package:flutter_test/flutter_test.dart';
import 'package:calltoverify/calltoverify.dart';

const _instr = Instructions(
  number: '+8801700000001',
  code: '123456',
  channel: 'sms',
  action: 'Send 123456 to +8801700000001',
  deepLink: 'sms:+8801700000001?body=123456',
  expiresAt: '2099-01-01T00:00:00Z',
);

const _expiredInstr = Instructions(
  number: '+8801700000001',
  code: '123456',
  channel: 'sms',
  action: 'Send 123456 to +8801700000001',
  deepLink: 'sms:+8801700000001?body=123456',
  expiresAt: '2000-01-01T00:00:00Z',
);

void main() {
  test('begin moves to instr and stores the session', () async {
    final c = VerificationController(
      start: (ch) async => const StartResult(sessionId: 's1', instructions: _instr),
      status: (id) async => const StatusResult(status: 'pending'),
    );
    await c.begin(Channel.sms);
    expect(c.phase, VerificationPhase.instr);
    expect(c.sessionId, 's1');
    expect(c.instructions?.code, '123456');
  });

  test('poll transitions to verified and stores the number', () async {
    var n = 0;
    final c = VerificationController(
      start: (ch) async => const StartResult(sessionId: 's1', instructions: _instr),
      status: (id) async {
        n++;
        return n < 2
            ? const StatusResult(status: 'pending')
            : const StatusResult(status: 'verified', verifiedMsisdn: '+8801711111111');
      },
    );
    await c.begin(Channel.sms);
    expect(await c.poll(), VerificationPhase.instr);
    expect(await c.poll(), VerificationPhase.verified);
    expect(c.verifiedMsisdn, '+8801711111111');
  });

  test('poll surfaces expiry once the client-side deadline passes', () async {
    final c = VerificationController(
      start: (ch) async => const StartResult(sessionId: 's1', instructions: _expiredInstr),
      status: (id) async => const StatusResult(status: 'pending'), // backend never reports expiry
    );
    await c.begin(Channel.sms);
    expect(await c.poll(), VerificationPhase.expired);
  });

  test('begin retains the error on failure', () async {
    final c = VerificationController(
      start: (ch) async => throw StateError('boom'),
      status: (id) async => const StatusResult(status: 'pending'),
    );
    await c.begin(Channel.sms);
    expect(c.phase, VerificationPhase.error);
    expect(c.lastError, isA<StateError>());
  });

  test('reset returns to the chooser when multiple channels', () async {
    final c = VerificationController(
      start: (ch) async => const StartResult(sessionId: 's1', instructions: _instr),
      status: (id) async => const StatusResult(status: 'pending'),
      channels: const [Channel.sms, Channel.call],
    );
    await c.begin(Channel.sms);
    expect(c.phase, VerificationPhase.instr);
    c.reset();
    expect(c.phase, VerificationPhase.choose);
    expect(c.sessionId, isNull);
  });

  test('fromJson parses snake_case wire shapes', () {
    final r = StartResult.fromJson({
      'session_id': 's',
      'instructions': {
        'number': 'n',
        'channel': 'sms',
        'action': 'a',
        'deep_link': 'd',
        'expires_at': 'e',
        'code': '123456',
      },
    });
    expect(r.sessionId, 's');
    expect(r.instructions.deepLink, 'd');
    expect(r.instructions.code, '123456');

    final s = StatusResult.fromJson({'status': 'verified', 'verified_msisdn': '+880'});
    expect(s.status, 'verified');
    expect(s.verifiedMsisdn, '+880');
  });
}
