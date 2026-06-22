<?php

declare(strict_types=1);

namespace CallToVerify\Laravel;

use CallToVerify\CallToVerify;
use CallToVerify\Verification;
use CallToVerify\VerificationStatus;
use CallToVerify\WebhookEvent;
use Illuminate\Support\Facades\Facade;

/**
 * Laravel facade for the CallToVerify client.
 *
 * @method static Verification startVerification(array $params = [])
 * @method static VerificationStatus checkStatus(string $sessionId)
 * @method static WebhookEvent verifyWebhook(string $rawBody, string $signature)
 *
 * @see CallToVerify
 */
final class CallToVerifyFacade extends Facade
{
    protected static function getFacadeAccessor(): string
    {
        return CallToVerify::class;
    }
}
