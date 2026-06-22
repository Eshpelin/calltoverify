<?php

declare(strict_types=1);

namespace CallToVerify;

/**
 * Result of starting a verification.
 */
final class Verification
{
    public function __construct(
        public readonly string $sessionId,
        public readonly string $status,
        public readonly Instructions $instructions,
    ) {
    }

    /** @param array<string, mixed> $data */
    public static function fromArray(array $data): self
    {
        /** @var array<string, mixed> $instructions */
        $instructions = $data['instructions'] ?? [];

        return new self(
            sessionId: (string) ($data['session_id'] ?? ''),
            status: (string) ($data['status'] ?? ''),
            instructions: Instructions::fromArray($instructions),
        );
    }
}
