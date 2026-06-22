<?php

declare(strict_types=1);

return [
    /*
    |--------------------------------------------------------------------------
    | Coordinator base URL
    |--------------------------------------------------------------------------
    |
    | The base URL of your CallToVerify Coordinator, e.g.
    | https://verify.example.com — no trailing slash required.
    |
    */
    'base_url' => env('CTV_BASE_URL', ''),

    /*
    |--------------------------------------------------------------------------
    | Developer API key
    |--------------------------------------------------------------------------
    |
    | Sent to the Coordinator as a Bearer token.
    |
    */
    'api_key' => env('CTV_API_KEY', ''),

    /*
    |--------------------------------------------------------------------------
    | Webhook signing secret
    |--------------------------------------------------------------------------
    |
    | Required only to call verifyWebhook(). Used to recompute the
    | HMAC-SHA256 over the raw webhook body.
    |
    */
    'webhook_secret' => env('CTV_WEBHOOK_SECRET'),

    /*
    |--------------------------------------------------------------------------
    | Request timeout (seconds)
    |--------------------------------------------------------------------------
    */
    'timeout' => env('CTV_TIMEOUT', 10.0),
];
