<?php

declare(strict_types=1);

namespace CallToVerify;

/**
 * A verification event delivered to the developer's webhook URL.
 */
final class WebhookEvent
{
    public function __construct(
        public readonly string $event,
        public readonly string $sessionId,
        public readonly string $verifiedMsisdn,
        public readonly string $channel,
        public readonly string $ts,
    ) {
    }

    /** @param array<string, mixed> $data */
    public static function fromArray(array $data): self
    {
        return new self(
            event: (string) ($data['event'] ?? ''),
            sessionId: (string) ($data['session_id'] ?? ''),
            verifiedMsisdn: (string) ($data['verified_msisdn'] ?? ''),
            channel: (string) ($data['channel'] ?? ''),
            ts: (string) ($data['ts'] ?? ''),
        );
    }
}
