import 'dart:async';

import 'package:flutter/material.dart';

import 'controller.dart';
import 'types.dart';

const Color _brand = Color(0xFF4F46E5);
const Color _brandStrong = Color(0xFF4338CA);
const Color _success = Color(0xFF15A34A);
const Color _muted = Color(0xFF6B6B82);

/// A drop-in verification widget. Renders the multi-channel CallToVerify UX and
/// drives it via your [start] / [status] callbacks (which call your backend).
class CallToVerify extends StatefulWidget {
  const CallToVerify({
    super.key,
    required this.start,
    required this.status,
    this.channels = const <Channel>[Channel.sms],
    this.onVerified,
    this.onExpired,
    this.onError,
    this.onOpenLink,
    this.pollInterval = const Duration(milliseconds: 2500),
  });

  final StartFn start;
  final StatusFn status;

  /// Channels in priority order: the first is the primary (shown first), the rest
  /// are alternatives. With more than one, a chooser is shown first.
  final List<Channel> channels;
  final void Function(String? verifiedMsisdn)? onVerified;
  final VoidCallback? onExpired;

  /// Called when starting a verification fails, with the underlying error so the
  /// host app can log it or show its own message.
  final void Function(Object? error)? onError;

  /// Called with a `sms:`/`tel:` deep link when the user taps the primary action.
  /// Wire this to `url_launcher`'s `launchUrl` in your app.
  final void Function(String url)? onOpenLink;

  final Duration pollInterval;

  @override
  State<CallToVerify> createState() => _CallToVerifyState();
}

class _CallToVerifyState extends State<CallToVerify> {
  late final VerificationController _c;
  Timer? _poll;
  Timer? _count;
  int _remaining = 0;
  bool _polling = false; // true while a poll() is in flight, so polls don't stack

  @override
  void initState() {
    super.initState();
    _c = VerificationController(start: widget.start, status: widget.status, channels: widget.channels);
    _c.addListener(_onChange);
    if (widget.channels.length <= 1) {
      _c.begin(widget.channels.isEmpty ? null : widget.channels.first);
    }
  }

  void _onChange() {
    _poll?.cancel();
    _count?.cancel();
    _poll = null;
    _count = null;
    switch (_c.phase) {
      case VerificationPhase.instr:
        _poll = Timer.periodic(widget.pollInterval, (_) async {
          if (_polling) return; // don't stack overlapping polls
          _polling = true;
          try {
            await _c.poll();
          } finally {
            _polling = false;
          }
        });
        _count = Timer.periodic(const Duration(seconds: 1), (_) => _tick());
        _tick();
        break;
      case VerificationPhase.verified:
        widget.onVerified?.call(_c.verifiedMsisdn);
        break;
      case VerificationPhase.expired:
        widget.onExpired?.call();
        break;
      case VerificationPhase.error:
        widget.onError?.call(_c.lastError);
        break;
      default:
        break;
    }
    if (mounted) setState(() {});
  }

  void _tick() {
    final i = _c.instructions;
    if (i == null) return;
    final exp = DateTime.tryParse(i.expiresAt);
    final r = exp == null ? 0 : exp.difference(DateTime.now()).inSeconds;
    setState(() => _remaining = r < 0 ? 0 : r);
  }

  @override
  void dispose() {
    _poll?.cancel();
    _count?.cancel();
    _c.removeListener(_onChange);
    _c.dispose();
    super.dispose();
  }

  @override
  Widget build(BuildContext context) {
    return Container(
      constraints: const BoxConstraints(maxWidth: 380),
      padding: const EdgeInsets.all(22),
      decoration: BoxDecoration(
        color: Theme.of(context).colorScheme.surface,
        borderRadius: BorderRadius.circular(18),
        border: Border.all(color: Colors.black.withValues(alpha: 0.10)),
      ),
      child: switch (_c.phase) {
        VerificationPhase.choose => _chooser(),
        VerificationPhase.starting => const Text('Starting…', style: TextStyle(color: _muted)),
        VerificationPhase.instr => _instr(_c.instructions!),
        VerificationPhase.verified => _final(true, "You're verified", _c.verifiedMsisdn, null),
        VerificationPhase.expired => _final(false, 'That code expired', null, 'Try again'),
        VerificationPhase.error => _final(false, 'Something went wrong', null, 'Retry'),
      },
    );
  }

  Widget _chooser() {
    final meta = <Channel, (IconData, String, String, bool)>{
      Channel.sms: (Icons.sms_outlined, 'Text us a code', 'Send one SMS', false),
      Channel.call: (Icons.phone_outlined, 'Give a missed call', 'Ring once and hang up', true),
      Channel.dtmf: (Icons.dialpad_outlined, 'Call and enter a code', 'Type it on the keypad', false),
    };
    return Column(
      mainAxisSize: MainAxisSize.min,
      crossAxisAlignment: CrossAxisAlignment.stretch,
      children: [
        const Text('Verify your number', style: TextStyle(fontSize: 19, fontWeight: FontWeight.w700)),
        const SizedBox(height: 4),
        const Text("We don't send you a code. You contact us from your number instead, and we verify it.",
            style: TextStyle(fontSize: 13.5, color: _muted)),
        const SizedBox(height: 18),
        for (final ch in widget.channels)
          Padding(
            padding: const EdgeInsets.only(bottom: 10),
            child: OutlinedButton(
              onPressed: () => _c.begin(ch),
              style: OutlinedButton.styleFrom(
                padding: const EdgeInsets.symmetric(vertical: 13, horizontal: 15),
                alignment: Alignment.centerLeft,
                shape: RoundedRectangleBorder(borderRadius: BorderRadius.circular(11)),
              ),
              child: Row(children: [
                Icon(meta[ch]!.$1, color: _brand),
                const SizedBox(width: 13),
                Column(crossAxisAlignment: CrossAxisAlignment.start, children: [
                  Text(meta[ch]!.$2, style: const TextStyle(fontSize: 14.5, fontWeight: FontWeight.w600)),
                  Text(meta[ch]!.$3,
                      style: TextStyle(fontSize: 12, color: meta[ch]!.$4 ? _success : _muted)),
                ]),
              ]),
            ),
          ),
      ],
    );
  }

  Widget _instr(Instructions i) {
    final children = <Widget>[];
    if (i.channel == 'sms' || i.channel == 'dtmf') {
      children.addAll([
        Text(
            i.channel == 'sms'
                ? 'Send the code below to ${i.number}'
                : 'Call ${i.number}, then enter this code on the keypad',
            style: const TextStyle(color: _muted)),
        const SizedBox(height: 14),
        Row(
          mainAxisAlignment: MainAxisAlignment.center,
          children: [
            for (final d in (i.code ?? '').split(''))
              Container(
                margin: const EdgeInsets.symmetric(horizontal: 4),
                width: 46,
                height: 56,
                alignment: Alignment.center,
                decoration: BoxDecoration(color: _brand.withValues(alpha: 0.08), borderRadius: BorderRadius.circular(12)),
                child: Text(d, style: const TextStyle(fontSize: 26, fontWeight: FontWeight.w700, color: _brandStrong)),
              ),
          ],
        ),
      ]);
    } else {
      children.addAll([
        const Text('Give a quick missed call to', style: TextStyle(color: _muted)),
        const SizedBox(height: 6),
        Text(i.number, textAlign: TextAlign.center, style: const TextStyle(fontSize: 21, fontWeight: FontWeight.w700)),
      ]);
    }
    children.addAll([
      const SizedBox(height: 16),
      _primaryButton(i.channel == 'sms' ? 'Open messages' : 'Call now', () => widget.onOpenLink?.call(i.deepLink)),
      const SizedBox(height: 16),
      Row(children: [
        const SizedBox(
            width: 15, height: 15, child: CircularProgressIndicator(strokeWidth: 2, color: _brand)),
        const SizedBox(width: 9),
        const Expanded(child: Text('Waiting for you. We detect it automatically.', style: TextStyle(fontSize: 12.5, color: _muted))),
      ]),
      const SizedBox(height: 8),
      Row(mainAxisAlignment: MainAxisAlignment.spaceBetween, children: [
        Text('Expires in ${_remaining}s', style: const TextStyle(fontSize: 11.5, color: _muted)),
        if (widget.channels.length > 1)
          TextButton(onPressed: _c.reset, child: const Text('Try another way')),
      ]),
    ]);
    return Column(mainAxisSize: MainAxisSize.min, crossAxisAlignment: CrossAxisAlignment.stretch, children: children);
  }

  Widget _final(bool ok, String title, String? number, String? retry) {
    return Column(mainAxisSize: MainAxisSize.min, children: [
      Container(
        width: 60,
        height: 60,
        decoration: BoxDecoration(
            color: (ok ? _success : Colors.red).withValues(alpha: 0.12), shape: BoxShape.circle),
        child: Icon(ok ? Icons.check : Icons.error_outline, color: ok ? _success : Colors.red, size: 32),
      ),
      const SizedBox(height: 14),
      Text(title, style: const TextStyle(fontSize: 18, fontWeight: FontWeight.w700)),
      if (number != null) ...[
        const SizedBox(height: 4),
        Text(number, style: const TextStyle(fontSize: 15, fontWeight: FontWeight.w600)),
      ],
      if (retry != null) ...[
        const SizedBox(height: 16),
        _primaryButton(retry, _c.reset),
      ],
    ]);
  }

  Widget _primaryButton(String label, VoidCallback onPressed) {
    return ElevatedButton(
      onPressed: onPressed,
      style: ElevatedButton.styleFrom(
        backgroundColor: _brand,
        foregroundColor: Colors.white,
        minimumSize: const Size.fromHeight(48),
        shape: RoundedRectangleBorder(borderRadius: BorderRadius.circular(11)),
      ),
      child: Text(label, style: const TextStyle(fontSize: 15, fontWeight: FontWeight.w600)),
    );
  }
}
