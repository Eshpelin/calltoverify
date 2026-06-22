<?php

declare(strict_types=1);

namespace CallToVerify;

/**
 * User-facing instructions for completing a verification.
 */
final class Instructions
{
    public function __construct(
        public readonly string $number,
        public readonly string $channel,
        public readonly string $action,
        public readonly string $deepLink,
        public readonly string $expiresAt,
        public readonly ?string $code = null,
    ) {
    }

    /** @param array<string, mixed> $data */
    public static function fromArray(array $data): self
    {
        $code = $data['code'] ?? null;

        return new self(
            number: (string) ($data['number'] ?? ''),
            channel: (string) ($data['channel'] ?? ''),
            action: (string) ($data['action'] ?? ''),
            deepLink: (string) ($data['deep_link'] ?? ''),
            expiresAt: (string) ($data['expires_at'] ?? ''),
            code: ($code === null || $code === '') ? null : (string) $code,
        );
    }
}
