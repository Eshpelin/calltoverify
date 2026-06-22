<?php

declare(strict_types=1);

namespace CallToVerify\Tests;

use CallToVerify\CallToVerify;
use CallToVerify\CallToVerifyException;
use InvalidArgumentException;
use PHPUnit\Framework\TestCase;

final class CallToVerifyTest extends TestCase
{
    private function client(FakeHttpClient $http, ?string $webhookSecret = null): CallToVerify
    {
        return new CallToVerify('https://verify.example.com', 'k', $webhookSecret, ['http' => $http]);
    }

    public function testStartVerificationSendsAuthAndMapsResponse(): void
    {
        $http = new FakeHttpClient();
        $http->on('POST', '/v1/verifications', 201, [
            'session_id' => 'sess1',
            'status' => 'pending',
            'instructions' => [
                'number' => '+8801700000001',
                'code' => '123456',
                'channel' => 'sms',
                'action' => 'Send 123456 to +8801700000001',
                'deep_link' => 'sms:+8801700000001?body=123456',
                'expires_at' => '2026-01-01T00:00:00Z',
            ],
        ]);

        $v = $this->client($http)->startVerification(['channel' => 'sms', 'bindingMode' => 'derive']);

        self::assertSame('Bearer k', $http->lastRequest['headers']['Authorization']);
        self::assertSame('POST', $http->lastRequest['method']);
        self::assertSame('/v1/verifications', $http->lastRequest['path']);

        $sentBody = json_decode((string) $http->lastRequest['body'], true);
        self::assertSame('sms', $sentBody['channel']);
        self::assertSame('derive', $sentBody['binding_mode']);

        self::assertSame('sess1', $v->sessionId);
        self::assertSame('pending', $v->status);
        self::assertSame('123456', $v->instructions->code);
        self::assertSame('sms:+8801700000001?body=123456', $v->instructions->deepLink);
        self::assertSame('+8801700000001', $v->instructions->number);
    }

    public function testStartVerificationAcceptsSnakeCaseKeys(): void
    {
        $http = new FakeHttpClient();
        $http->on('POST', '/v1/verifications', 201, [
            'session_id' => 's',
            'status' => 'pending',
            'instructions' => [
                'number' => 'n', 'channel' => 'sms', 'action' => 'a',
                'deep_link' => 'd', 'expires_at' => 'e',
            ],
        ]);

        $this->client($http)->startVerification([
            'channel' => 'sms',
            'binding_mode' => 'claim',
            'claimed_msisdn' => '+8801712345678',
        ]);

        $sentBody = json_decode((string) $http->lastRequest['body'], true);
        self::assertSame('claim', $sentBody['binding_mode']);
        self::assertSame('+8801712345678', $sentBody['claimed_msisdn']);
    }

    public function testStartOmitsNullFields(): void
    {
        $http = new FakeHttpClient();
        $http->on('POST', '/v1/verifications', 201, [
            'session_id' => 's',
            'status' => 'pending',
            'instructions' => [
                'number' => 'n', 'channel' => 'sms', 'action' => 'a',
                'deep_link' => 'd', 'expires_at' => 'e',
            ],
        ]);

        $this->client($http)->startVerification(['channel' => 'sms']);

        $sentBody = json_decode((string) $http->lastRequest['body'], true);
        self::assertArrayHasKey('channel', $sentBody);
        self::assertArrayNotHasKey('binding_mode', $sentBody);
        self::assertArrayNotHasKey('claimed_msisdn', $sentBody);
    }

    public function testMissingInstructionCodeMapsToNull(): void
    {
        $http = new FakeHttpClient();
        $http->on('POST', '/v1/verifications', 201, [
            'session_id' => 's',
            'status' => 'pending',
            'instructions' => [
                'number' => 'n', 'channel' => 'call', 'action' => 'a',
                'deep_link' => 'd', 'expires_at' => 'e',
            ],
        ]);

        $v = $this->client($http)->startVerification(['channel' => 'call']);
        self::assertNull($v->instructions->code);
    }

    public function testCheckStatusMapsVerifiedSession(): void
    {
        $http = new FakeHttpClient();
        $http->on('GET', '/v1/verifications/sess1', 200, [
            'session_id' => 'sess1',
            'status' => 'verified',
            'channel' => 'sms',
            'verified_msisdn' => '+8801712345678',
            'expires_at' => '2026-01-01T00:00:00Z',
        ]);

        $s = $this->client($http)->checkStatus('sess1');

        self::assertSame('GET', $http->lastRequest['method']);
        self::assertSame('/v1/verifications/sess1', $http->lastRequest['path']);
        self::assertSame('verified', $s->status);
        self::assertSame('+8801712345678', $s->verifiedMsisdn);
        self::assertSame('sms', $s->channel);
    }

    public function testCheckStatusPendingHasNullMsisdn(): void
    {
        $http = new FakeHttpClient();
        $http->on('GET', '/v1/verifications/sess2', 200, [
            'session_id' => 'sess2',
            'status' => 'pending',
            'channel' => 'sms',
            'expires_at' => '2026-01-01T00:00:00Z',
        ]);

        $s = $this->client($http)->checkStatus('sess2');
        self::assertNull($s->verifiedMsisdn);
    }

    public function testNonTwoxxMapsToException(): void
    {
        $http = new FakeHttpClient();
        $http->on('POST', '/v1/verifications', 401, [
            'error' => 'unauthorized',
            'detail' => 'invalid API key',
        ]);

        try {
            $this->client($http)->startVerification();
            self::fail('expected CallToVerifyException');
        } catch (CallToVerifyException $e) {
            self::assertSame(401, $e->getStatusCode());
            self::assertSame('unauthorized', $e->getErrorCode());
            self::assertSame('invalid API key', $e->getMessage());
        }
    }

    public function testVerifyWebhookAcceptsValidSignature(): void
    {
        $secret = 'whsec_test';
        $body = json_encode([
            'event' => 'verification.verified',
            'session_id' => 'sess1',
            'verified_msisdn' => '+8801712345678',
            'channel' => 'sms',
            'ts' => '2026-01-01T00:00:00Z',
        ], JSON_THROW_ON_ERROR);
        $sig = hash_hmac('sha256', $body, $secret);

        $ctv = $this->client(new FakeHttpClient(), $secret);
        $ev = $ctv->verifyWebhook($body, $sig);

        self::assertSame('verification.verified', $ev->event);
        self::assertSame('sess1', $ev->sessionId);
        self::assertSame('+8801712345678', $ev->verifiedMsisdn);
        self::assertSame('sms', $ev->channel);
    }

    public function testVerifyWebhookRejectsTamperedSignature(): void
    {
        $secret = 'whsec_test';
        $body = json_encode([
            'event' => 'verification.verified',
            'session_id' => 'sess1',
            'verified_msisdn' => '+8801712345678',
            'channel' => 'sms',
            'ts' => '2026-01-01T00:00:00Z',
        ], JSON_THROW_ON_ERROR);
        $sig = hash_hmac('sha256', $body, $secret);
        $tampered = '0' . substr($sig, 1);

        $ctv = $this->client(new FakeHttpClient(), $secret);

        $this->expectException(CallToVerifyException::class);
        $ctv->verifyWebhook($body, $tampered);
    }

    public function testVerifyWebhookRejectsShortSignature(): void
    {
        $ctv = $this->client(new FakeHttpClient(), 'whsec_test');

        $this->expectException(CallToVerifyException::class);
        $ctv->verifyWebhook('{}', 'deadbeef');
    }

    public function testVerifyWebhookRejectsTamperedBody(): void
    {
        $secret = 'whsec_test';
        $body = json_encode([
            'event' => 'verification.verified',
            'session_id' => 'sess1',
            'verified_msisdn' => '+8801712345678',
            'channel' => 'sms',
            'ts' => '2026-01-01T00:00:00Z',
        ], JSON_THROW_ON_ERROR);
        $sig = hash_hmac('sha256', $body, $secret);

        $ctv = $this->client(new FakeHttpClient(), $secret);

        $this->expectException(CallToVerifyException::class);
        $ctv->verifyWebhook($body . ' ', $sig);
    }

    public function testVerifyWebhookRequiresSecret(): void
    {
        $ctv = $this->client(new FakeHttpClient());

        $this->expectException(InvalidArgumentException::class);
        $ctv->verifyWebhook('{}', 'abc');
    }

    public function testConstructorValidatesBaseUrl(): void
    {
        $this->expectException(InvalidArgumentException::class);
        new CallToVerify('', 'k');
    }

    public function testConstructorValidatesApiKey(): void
    {
        $this->expectException(InvalidArgumentException::class);
        new CallToVerify('https://x', '');
    }

    public function testConstructorTrimsTrailingSlash(): void
    {
        $http = new FakeHttpClient();
        $http->on('GET', '/v1/verifications/s', 200, [
            'session_id' => 's', 'status' => 'pending', 'channel' => 'sms', 'expires_at' => 'e',
        ]);

        $ctv = new CallToVerify('https://verify.example.com/', 'k', null, ['http' => $http]);
        $ctv->checkStatus('s');

        self::assertSame('https://verify.example.com/v1/verifications/s', $http->lastRequest['url']);
    }
}
