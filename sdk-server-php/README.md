# sdk-server-php

Backend SDK for PHP, with first-class Laravel support. PHP/Laravel is widely used in the
Bangladesh market this project targets first.

> **Status: planned (Phase 4).** Placeholder directory.

## Planned API

```php
$ctv = new CallToVerify($baseUrl, $apiKey, $apiSecret);

$v = $ctv->startVerification(['channel' => 'sms', 'claimed_msisdn' => $msisdn]);
// $v->instructions: number, code, deep_link, expires_at

$status = $ctv->checkStatus($v->session_id);

$ctv->verifyWebhook($rawBody, $signatureHeader); // throws on mismatch
```

## Planned stack

PHP 8.x, Composer package, optional Laravel service provider + facade.
