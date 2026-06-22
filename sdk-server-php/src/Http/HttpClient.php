<?php

declare(strict_types=1);

namespace CallToVerify\Http;

/**
 * Tiny HTTP abstraction so the transport can be swapped (e.g. mocked in tests).
 *
 * Implementations must NOT throw on non-2xx responses; they return the status
 * code and body, and the SDK decides how to map them.
 */
interface HttpClient
{
    /**
     * Perform an HTTP request.
     *
     * @param 'GET'|'POST'           $method
     * @param string                 $url     Absolute URL.
     * @param array<string, string>  $headers Header name => value.
     * @param string|null            $body    Raw request body, or null for none.
     */
    public function request(string $method, string $url, array $headers = [], ?string $body = null): HttpResponse;
}
