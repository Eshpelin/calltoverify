# calltoverify/sdk (PHP)

Backend SDK for PHP, with first-class Laravel support. A thin, dependency-free client for the
CallToVerify Coordinator's developer API. PHP/Laravel is widely used in the Bangladesh market
this project targets first.

> **Status: alpha (Phase 1 implemented).** Start verifications, poll status, verify webhooks.

## Install

```bash
composer require calltoverify/sdk
```

Requires PHP 8.1+ with the `curl` and `json` extensions. The SDK is **dependency-free**: it ships
a small curl-based HTTP client so it installs anywhere a normal PHP runtime is available. If you
prefer Guzzle/PSR-18, implement `CallToVerify\Http\HttpClient` and pass it via the `http` option.

## Usage (plain PHP)

```php
use CallToVerify\CallToVerify;
use CallToVerify\CallToVerifyException;

$ctv = new CallToVerify(
    baseUrl: 'https://verify.example.com',          // your Coordinator
    apiKey: getenv('CTV_API_KEY'),
    webhookSecret: getenv('CTV_WEBHOOK_SECRET') ?: null, // only needed for verifyWebhook
);

// Start a verification. Returns instructions to show the user.
$v = $ctv->startVerification(['channel' => 'sms']);
echo $v->instructions->action;        // "Send 123456 to +8801700000001"
// $v->instructions -> number, code, channel, action, deepLink, expiresAt

// Poll, or rely on the webhook.
$status = $ctv->checkStatus($v->sessionId);
if ($status->status === 'verified') {
    echo "verified number: {$status->verifiedMsisdn}";
}
```

### Claim mode

```php
$v = $ctv->startVerification([
    'channel'        => 'sms',
    'binding_mode'   => 'claim',          // or 'bindingMode'
    'claimed_msisdn' => '+8801712345678', // or 'claimedMsisdn'
]);
```

### Verifying webhooks

```php
// Pass the RAW request body bytes and the X-CTV-Signature header.
try {
    $event = $ctv->verifyWebhook($rawBody, $_SERVER['HTTP_X_CTV_SIGNATURE']);
    // $event -> event, sessionId, verifiedMsisdn, channel, ts
} catch (CallToVerifyException $e) {
    // signature mismatch -> reject the request
    http_response_code(400);
}
```

## Usage (Laravel)

The package auto-registers a service provider and a `CallToVerify` facade (Laravel auto-discovery).

1. Publish the config:

   ```bash
   php artisan vendor:publish --tag=calltoverify-config
   ```

2. Set environment variables in `.env`:

   ```dotenv
   CTV_BASE_URL=https://verify.example.com
   CTV_API_KEY=your-developer-api-key
   CTV_WEBHOOK_SECRET=your-webhook-secret
   ```

3. Use the facade, or resolve the singleton from the container:

   ```php
   use CallToVerify\Laravel\CallToVerifyFacade as CallToVerify;

   $v = CallToVerify::startVerification(['channel' => 'sms']);
   $status = CallToVerify::checkStatus($v->sessionId);
   ```

   ```php
   // Or via dependency injection / the container:
   public function store(\CallToVerify\CallToVerify $ctv) {
       $v = $ctv->startVerification(['channel' => 'sms']);
       // ...
   }
   ```

   Webhook controller example (use the raw body, not the parsed JSON):

   ```php
   public function handle(\Illuminate\Http\Request $request, \CallToVerify\CallToVerify $ctv) {
       try {
           $event = $ctv->verifyWebhook(
               $request->getContent(),                 // RAW body
               $request->header('X-CTV-Signature', '')
           );
       } catch (\CallToVerify\CallToVerifyException $e) {
           abort(400);
       }
       // $event->sessionId, $event->verifiedMsisdn, ...
   }
   ```

## API

- `new CallToVerify(string $baseUrl, string $apiKey, ?string $webhookSecret = null, array $opts = [])`
  - `$opts['http']` — a custom `CallToVerify\Http\HttpClient` (defaults to the bundled curl client).
  - `$opts['timeout']` — request timeout in seconds (default `10.0`).
- `startVerification(array $params = []): Verification`
  - keys: `channel`, `binding_mode` (or `bindingMode`), `claimed_msisdn` (or `claimedMsisdn`). Null fields are omitted.
- `checkStatus(string $sessionId): VerificationStatus`
- `verifyWebhook(string $rawBody, string $signature, ?int $maxAgeSeconds = null): WebhookEvent` — throws `CallToVerifyException` on signature mismatch; pass `$maxAgeSeconds` to also reject events whose `ts` is older than the window.

### Result objects (readonly)

- `Verification`: `sessionId`, `status`, `instructions`
- `Instructions`: `number`, `code` (nullable), `channel`, `action`, `deepLink`, `expiresAt`
- `VerificationStatus`: `sessionId`, `status`, `channel`, `verifiedMsisdn` (nullable), `expiresAt`
- `WebhookEvent`: `event`, `sessionId`, `verifiedMsisdn`, `channel`, `ts`

Non-2xx responses throw `CallToVerifyException` with `getStatusCode()`, `getErrorCode()`, and `getMessage()`.

## Develop

```bash
composer install
composer test     # runs PHPUnit (vendor/bin/phpunit)
```

Tests use an injectable HTTP client (`CallToVerify\Http\HttpClient`), so they run without a network.

## License

Apache-2.0. See the repository [LICENSE](../LICENSE).
