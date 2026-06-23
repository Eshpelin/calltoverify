import 'package:flutter/foundation.dart';

import 'types.dart';

enum VerificationPhase { choose, starting, instr, verified, expired, error }

typedef StartFn = Future<StartResult> Function(Channel? channel);
typedef StatusFn = Future<StatusResult> Function(String sessionId);

/// Drives a verification from start through polling to a terminal state. It holds
/// no widgets and is a [ChangeNotifier], so the UI rebuilds as the phase changes.
/// It talks to YOUR backend via [start] / [status], not the Coordinator directly.
class VerificationController extends ChangeNotifier {
  VerificationController({
    required this.start,
    required this.status,
    this.channels = const <Channel>[],
  }) : phase = channels.length > 1 ? VerificationPhase.choose : VerificationPhase.starting;

  final StartFn start;
  final StatusFn status;
  final List<Channel> channels;

  VerificationPhase phase;
  Instructions? instructions;
  String? verifiedMsisdn;
  String? _sessionId;

  /// The error that caused the last transition to [VerificationPhase.error], if any.
  Object? lastError;

  String? get sessionId => _sessionId;

  Future<void> begin([Channel? channel]) async {
    instructions = null;
    _sessionId = null;
    lastError = null;
    phase = VerificationPhase.starting;
    notifyListeners();
    try {
      final r = await start(channel);
      _sessionId = r.sessionId;
      instructions = r.instructions;
      phase = VerificationPhase.instr;
    } catch (e) {
      lastError = e;
      phase = VerificationPhase.error;
    }
    notifyListeners();
  }

  Future<VerificationPhase> poll() async {
    if (phase != VerificationPhase.instr || _sessionId == null) return phase;
    // Client-side deadline: once the instructions expire, stop polling and surface
    // expiry even if the backend never reports it.
    final exp = instructions != null ? DateTime.tryParse(instructions!.expiresAt) : null;
    if (exp != null && DateTime.now().isAfter(exp)) {
      phase = VerificationPhase.expired;
      notifyListeners();
      return phase;
    }
    try {
      final s = await status(_sessionId!);
      if (s.status == 'verified') {
        verifiedMsisdn = s.verifiedMsisdn;
        phase = VerificationPhase.verified;
        notifyListeners();
      } else if (s.status == 'expired' || s.status == 'failed') {
        phase = VerificationPhase.expired;
        notifyListeners();
      }
    } catch (_) {
      // Transient: stay pending.
    }
    return phase;
  }

  void reset() {
    phase = channels.length > 1 ? VerificationPhase.choose : VerificationPhase.starting;
    instructions = null;
    _sessionId = null;
    verifiedMsisdn = null;
    notifyListeners();
  }
}
