<?php

declare(strict_types=1);

namespace CallToVerify;

use CallToVerify\Http\CurlHttpClient;
use CallToVerify\Http\HttpClient;
use InvalidArgumentException;

/**
 * CallToVerify server SDK.
 *
 * A thin, dependency-free client for the Coordinator's developer REST API. Use
 * it from your PHP/Laravel backend to start phone-number verifications, poll
 * their status, and verify webhooks.
 */
final class CallToVerify
{
    private string $baseUrl;
    private string $apiKey;
    private ?string $webhookSecret;
    private HttpClient $http;

    /**
     * @param string                $baseUrl       Base URL of the Coordinator, e.g. https://verify.example.com
     * @param string                $apiKey        Developer API key (sent as a Bearer token).
     * @param string|null           $webhookSecret Webhook signing secret; required only to call verifyWebhook().
     * @param array<string, mixed>  $opts          Options: 'http' => HttpClient, 'timeout' => float (seconds).
     */
    public function __construct(string $baseUrl, string $apiKey, ?string $webhookSecret = null, array $opts = [])
    {
        if ($baseUrl === '') {
            throw new InvalidArgumentException('CallToVerify: baseUrl is required');
        }
        if ($apiKey === '') {
            throw new InvalidArgumentException('CallToVerify: apiKey is required');
        }

        $this->baseUrl = rtrim($baseUrl, '/');
        $this->apiKey = $apiKey;
        $this->webhookSecret = $webhookSecret;

        $http = $opts['http'] ?? null;
        if ($http !== null && !$http instanceof HttpClient) {
            throw new InvalidArgumentException('CallToVerify: opts["http"] must implement ' . HttpClient::class);
        }
        $timeout = isset($opts['timeout']) ? (float) $opts['timeout'] : 10.0;
        $this->http = $http ?? new CurlHttpClient($timeout);
    }

    /**
     * Start a verification and return the user-facing instructions.
     *
     * @param array<string, mixed> $params Keys: 'channel', 'binding_mode' (or 'bindingMode'),
     *                                      'claimed_msisdn' (or 'claimedMsisdn'). Null fields are omitted.
     *
     * @throws CallToVerifyException on a non-2xx response.
     */
    public function startVerification(array $params = []): Verification
    {
        $body = [
            'channel' => $params['channel'] ?? null,
            'binding_mode' => $params['binding_mode'] ?? $params['bindingMode'] ?? null,
            'claimed_msisdn' => $params['claimed_msisdn'] ?? $params['claimedMsisdn'] ?? null,
        ];

        $data = $this->request('POST', '/v1/verifications', $body);

        return Verification::fromArray($data);
    }

    /**
     * Poll a verification's current status.
     *
     * @throws CallToVerifyException on a non-2xx response.
     */
    public function checkStatus(string $sessionId): VerificationStatus
    {
        $data = $this->request('GET', '/v1/verifications/' . rawurlencode($sessionId));

        return VerificationStatus::fromArray($data);
    }

    /**
     * Verify and parse a webhook.
     *
     * Pass the RAW request body bytes and the X-CTV-Signature header. The HMAC is
     * recomputed over the raw body and compared in constant time.
     *
     * @throws CallToVerifyException on signature mismatch.
     */
    public function verifyWebhook(string $rawBody, string $signature, ?int $maxAgeSeconds = null): WebhookEvent
    {
        if ($this->webhookSecret === null || $this->webhookSecret === '') {
            throw new InvalidArgumentException('CallToVerify: webhookSecret is required to verify webhooks');
        }

        $expected = hash_hmac('sha256', $rawBody, $this->webhookSecret);
        if (!hash_equals($expected, $signature)) {
            throw new CallToVerifyException(401, 'invalid_signature', 'webhook signature mismatch');
        }

        /** @var array<string, mixed> $payload */
        $payload = json_decode($rawBody, true, 512, JSON_THROW_ON_ERROR);

        // Optional replay defense: reject events whose ts is outside the window.
        // De-dupe on session_id for idempotency regardless.
        if ($maxAgeSeconds !== null) {
            $ts = isset($payload['ts']) ? strtotime((string) $payload['ts']) : false;
            if ($ts === false || abs(time() - $ts) > $maxAgeSeconds) {
                throw new CallToVerifyException(401, 'webhook_expired', 'webhook timestamp outside the allowed window');
            }
        }

        return WebhookEvent::fromArray($payload);
    }

    /**
     * @param 'GET'|'POST'           $method
     * @param array<string, mixed>|null $body Associative body; null fields are stripped before sending.
     *
     * @return array<string, mixed>
     *
     * @throws CallToVerifyException
     */
    private function request(string $method, string $path, ?array $body = null): array
    {
        $headers = ['Authorization' => 'Bearer ' . $this->apiKey];
        $payload = null;

        if ($body !== null) {
            $filtered = array_filter($body, static fn ($v): bool => $v !== null);
            $payload = json_encode($filtered, JSON_THROW_ON_ERROR);
            $headers['Content-Type'] = 'application/json';
        }

        $response = $this->http->request($method, $this->baseUrl . $path, $headers, $payload);

        $json = [];
        if ($response->body !== '') {
            $decoded = json_decode($response->body, true);
            if (is_array($decoded)) {
                $json = $decoded;
            }
        }

        if ($response->statusCode < 200 || $response->statusCode >= 300) {
            $code = is_string($json['error'] ?? null) ? $json['error'] : 'error';
            $detail = is_string($json['detail'] ?? null) ? $json['detail'] : ('HTTP ' . $response->statusCode);
            throw new CallToVerifyException($response->statusCode, $code, $detail);
        }

        return $json;
    }
}
