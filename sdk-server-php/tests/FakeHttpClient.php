<?php

declare(strict_types=1);

namespace CallToVerify\Tests;

use CallToVerify\Http\HttpClient;
use CallToVerify\Http\HttpResponse;

/**
 * In-memory HTTP client for tests. Records the last request and returns a
 * canned response keyed by "METHOD /path", so tests never touch the network.
 */
final class FakeHttpClient implements HttpClient
{
    /** @var array<string, HttpResponse> */
    private array $responses = [];

    /** @var array<string, mixed>|null */
    public ?array $lastRequest = null;

    public function on(string $method, string $path, int $status, mixed $json): void
    {
        $this->responses[$method . ' ' . $path] = new HttpResponse(
            $status,
            json_encode($json, JSON_THROW_ON_ERROR),
        );
    }

    public function request(string $method, string $url, array $headers = [], ?string $body = null): HttpResponse
    {
        $path = (string) parse_url($url, PHP_URL_PATH);
        $this->lastRequest = [
            'method' => $method,
            'url' => $url,
            'path' => $path,
            'headers' => $headers,
            'body' => $body,
        ];

        return $this->responses[$method . ' ' . $path]
            ?? new HttpResponse(404, json_encode(['error' => 'not_found'], JSON_THROW_ON_ERROR));
    }
}
