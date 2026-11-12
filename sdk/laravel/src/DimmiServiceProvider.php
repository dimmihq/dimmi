<?php

declare(strict_types=1);

namespace Dimmi\Laravel;

use Dimmi\Client;
use Dimmi\Laravel\Console\InstallCommand;
use Dimmi\Laravel\Console\TestCommand;
use Dimmi\Laravel\Http\Controllers\TokenController;
use Dimmi\Laravel\Http\Controllers\WebhookController;
use Illuminate\Routing\Route as RouteDefinition;
use Illuminate\Support\Facades\Blade;
use Illuminate\Support\Facades\Route;
use Illuminate\Support\ServiceProvider;

class DimmiServiceProvider extends ServiceProvider
{
    public function register(): void
    {
        $this->mergeConfigFrom(__DIR__ . '/../config/dimmi.php', 'dimmi');

        $this->app->singleton(DimmiManager::class, fn ($app) => new DimmiManager($app, (array) $app['config']->get('dimmi', [])));
        $this->app->alias(DimmiManager::class, 'dimmi');
        $this->app->bind(Client::class, fn ($app) => $app->make(DimmiManager::class)->client());
    }

    public function boot(): void
    {
        if ($this->app->runningInConsole()) {
            $this->publishes([__DIR__ . '/../config/dimmi.php' => $this->app->configPath('dimmi.php')], ['dimmi-config', 'dimmi']);
            $this->commands([InstallCommand::class, TestCommand::class]);
        }

        Blade::directive('dimmiMessenger', fn (string $expression) => '<?php echo app(\\' . DimmiManager::class . '::class)->messenger(auth()->user(), ' . ($expression !== '' ? $expression : '[]') . '); ?>');

        Route::macro('dimmiWebhooks', function (string $uri = 'dimmi/webhook'): RouteDefinition {
            return Route::post($uri, WebhookController::class)
                ->name('dimmi.webhook')
                ->withoutMiddleware(DimmiServiceProvider::csrfMiddleware());
        });

        $this->registerRoutes();
    }

    /**
     * The CSRF middleware classes of Laravel 10 to 12, to leave out of
     * the webhook route.
     *
     * @return list<class-string>
     */
    public static function csrfMiddleware(): array
    {
        return array_values(array_filter([
            'Illuminate\Foundation\Http\Middleware\VerifyCsrfToken',
            'Illuminate\Foundation\Http\Middleware\ValidateCsrfToken',
            'App\Http\Middleware\VerifyCsrfToken',
        ], 'class_exists'));
    }

    private function registerRoutes(): void
    {
        if ($this->app->routesAreCached()) {
            return;
        }
        $config = $this->app['config'];

        $token = $config->get('dimmi.token_route.path');
        if (is_string($token) && $token !== '') {
            Route::middleware((array) $config->get('dimmi.token_route.middleware', ['web']))
                ->get($token, TokenController::class)
                ->name('dimmi.token');
        }

        $webhook = $config->get('dimmi.webhook.path');
        if (is_string($webhook) && $webhook !== '') {
            Route::dimmiWebhooks($webhook);
        }
    }
}
