<?php

declare(strict_types=1);

namespace CallToVerify\Http;

/**
 * Minimal HTTP response value object returned by an {@see HttpClient}.
 */
final class HttpResponse
{
    public function __construct(
        public readonly int $statusCode,
        public readonly string $body,
    ) {
    }
}
