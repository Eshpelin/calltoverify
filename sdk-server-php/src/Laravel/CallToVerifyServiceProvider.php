<?php

declare(strict_types=1);

namespace CallToVerify\Laravel;

use CallToVerify\CallToVerify;
use Illuminate\Contracts\Foundation\Application;
use Illuminate\Support\ServiceProvider;

/**
 * Registers the CallToVerify client as a singleton in the Laravel container.
 *
 * Auto-discovered via composer's extra.laravel.providers. Publish the config
 * with: php artisan vendor:publish --tag=calltoverify-config
 */
final class CallToVerifyServiceProvider extends ServiceProvider
{
    public function register(): void
    {
        $this->mergeConfigFrom(__DIR__ . '/../../config/calltoverify.php', 'calltoverify');

        $this->app->singleton(CallToVerify::class, static function (Application $app): CallToVerify {
            /** @var array<string, mixed> $config */
            $config = $app['config']->get('calltoverify', []);

            $opts = [];
            if (isset($config['timeout'])) {
                $opts['timeout'] = (float) $config['timeout'];
            }

            return new CallToVerify(
                (string) ($config['base_url'] ?? ''),
                (string) ($config['api_key'] ?? ''),
                isset($config['webhook_secret']) ? (string) $config['webhook_secret'] : null,
                $opts,
            );
        });

        $this->app->alias(CallToVerify::class, 'calltoverify');
    }

    public function boot(): void
    {
        if ($this->app->runningInConsole()) {
            $this->publishes([
                __DIR__ . '/../../config/calltoverify.php' => $this->app->configPath('calltoverify.php'),
            ], 'calltoverify-config');
        }
    }

    /**
     * @return array<int, string>
     */
    public function provides(): array
    {
        return [CallToVerify::class, 'calltoverify'];
    }
}
