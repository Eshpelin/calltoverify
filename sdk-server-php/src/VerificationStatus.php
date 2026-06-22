<?php

declare(strict_types=1);

namespace CallToVerify;

/**
 * Current status of a verification session.
 */
final class VerificationStatus
{
    public function __construct(
        public readonly string $sessionId,
        public readonly string $status,
        public readonly string $channel,
        public readonly string $expiresAt,
        public readonly ?string $verifiedMsisdn = null,
    ) {
    }

    /** @param array<string, mixed> $data */
    public static function fromArray(array $data): self
    {
        $verified = $data['verified_msisdn'] ?? null;

        return new self(
            sessionId: (string) ($data['session_id'] ?? ''),
            status: (string) ($data['status'] ?? ''),
            channel: (string) ($data['channel'] ?? ''),
            expiresAt: (string) ($data['expires_at'] ?? ''),
            verifiedMsisdn: ($verified === null || $verified === '') ? null : (string) $verified,
        );
    }
}
