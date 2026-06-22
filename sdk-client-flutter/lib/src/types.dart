/// The verification channels CallToVerify supports.
enum Channel { sms, call, dtmf }

extension ChannelWire on Channel {
  String get wire => switch (this) {
        Channel.sms => 'sms',
        Channel.call => 'call',
        Channel.dtmf => 'dtmf',
      };
}

/// Instructions to show the end user.
class Instructions {
  const Instructions({
    required this.number,
    required this.channel,
    required this.action,
    required this.deepLink,
    required this.expiresAt,
    this.code,
  });

  final String number;
  final String? code;
  final String channel;
  final String action;
  final String deepLink;
  final String expiresAt;

  factory Instructions.fromJson(Map<String, dynamic> j) => Instructions(
        number: (j['number'] ?? '') as String,
        code: j['code'] as String?,
        channel: (j['channel'] ?? '') as String,
        action: (j['action'] ?? '') as String,
        deepLink: (j['deep_link'] ?? j['deepLink'] ?? '') as String,
        expiresAt: (j['expires_at'] ?? j['expiresAt'] ?? '') as String,
      );
}

/// Returned by your `start` callback.
class StartResult {
  const StartResult({required this.sessionId, required this.instructions});

  final String sessionId;
  final Instructions instructions;

  factory StartResult.fromJson(Map<String, dynamic> j) => StartResult(
        sessionId: (j['session_id'] ?? j['sessionId'] ?? '') as String,
        instructions: Instructions.fromJson((j['instructions'] ?? const {}) as Map<String, dynamic>),
      );
}

/// Returned by your `status` callback.
class StatusResult {
  const StatusResult({required this.status, this.verifiedMsisdn});

  final String status;
  final String? verifiedMsisdn;

  factory StatusResult.fromJson(Map<String, dynamic> j) => StatusResult(
        status: (j['status'] ?? '') as String,
        verifiedMsisdn: (j['verified_msisdn'] ?? j['verifiedMsisdn']) as String?,
      );
}
