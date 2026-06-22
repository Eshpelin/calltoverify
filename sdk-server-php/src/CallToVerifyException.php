<?php

declare(strict_types=1);

namespace CallToVerify;

use RuntimeException;
use Throwable;

/**
 * Thrown for non-2xx Coordinator responses and webhook signature failures.
 *
 * Use {@see getStatusCode()} for the HTTP status, {@see getErrorCode()} for the
 * machine-readable error code returned by the Coordinator (the JSON `error`
 * field), and {@see getMessage()} for the human-readable detail.
 */
final class CallToVerifyException extends RuntimeException
{
    private int $statusCode;
    private string $errorCode;

    public function __construct(int $statusCode, string $errorCode, string $detail, ?Throwable $previous = null)
    {
        parent::__construct($detail, $statusCode, $previous);
        $this->statusCode = $statusCode;
        $this->errorCode = $errorCode;
    }

    /** HTTP status code (or 401 for signature mismatches). */
    public function getStatusCode(): int
    {
        return $this->statusCode;
    }

    /** Machine-readable error code (the Coordinator's JSON `error` field). */
    public function getErrorCode(): string
    {
        return $this->errorCode;
    }
}
