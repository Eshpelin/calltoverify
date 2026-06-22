<?php

declare(strict_types=1);

namespace CallToVerify\Http;

use CallToVerify\CallToVerifyException;

/**
 * Default, dependency-free HTTP client built on ext-curl.
 *
 * The SDK ships without Guzzle so it installs anywhere a normal PHP runtime
 * with the curl extension is available. To use a different transport (Guzzle,
 * PSR-18, etc.), implement {@see HttpClient} and pass it via the SDK options.
 */
final class CurlHttpClient implements HttpClient
{
    /** @param float $timeout Total request timeout in seconds. */
    public function __construct(private readonly float $timeout = 10.0)
    {
    }

    public function request(string $method, string $url, array $headers = [], ?string $body = null): HttpResponse
    {
        $ch = curl_init();
        if ($ch === false) {
            throw new CallToVerifyException(0, 'transport_error', 'failed to initialize curl');
        }

        $headerLines = [];
        foreach ($headers as $name => $value) {
            $headerLines[] = $name . ': ' . $value;
        }

        curl_setopt_array($ch, [
            CURLOPT_URL => $url,
            CURLOPT_CUSTOMREQUEST => $method,
            CURLOPT_RETURNTRANSFER => true,
            CURLOPT_HTTPHEADER => $headerLines,
            CURLOPT_TIMEOUT_MS => (int) round($this->timeout * 1000),
            CURLOPT_CONNECTTIMEOUT_MS => (int) round($this->timeout * 1000),
        ]);

        if ($body !== null) {
            curl_setopt($ch, CURLOPT_POSTFIELDS, $body);
        }

        $responseBody = curl_exec($ch);
        if ($responseBody === false) {
            $error = curl_error($ch);
            curl_close($ch);
            throw new CallToVerifyException(0, 'transport_error', $error !== '' ? $error : 'request failed');
        }

        $status = (int) curl_getinfo($ch, CURLINFO_RESPONSE_CODE);
        curl_close($ch);

        return new HttpResponse($status, (string) $responseBody);
    }
}
