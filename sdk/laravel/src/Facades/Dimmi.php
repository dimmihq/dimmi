<?php

declare(strict_types=1);

namespace Dimmi\Laravel\Facades;

use Dimmi\Laravel\DimmiManager;
use Illuminate\Support\Facades\Facade;

/**
 * @method static \Dimmi\Client client()
 * @method static string|null identityToken(mixed $user, array $options = [])
 * @method static array|null identityFor(mixed $user)
 * @method static string messenger(?\Illuminate\Contracts\Auth\Authenticatable $user = null, array $options = [])
 * @method static \Dimmi\WebhookEvent verifyWebhook(string $payload, array $headers)
 * @method static array|string|null call(string $operation, array $path = [], array $query = [], array|object|null $body = null, array $options = [])
 * @method static array|string|null request(string $method, string $path, array $query = [], array|object|null $body = null, array $options = [])
 * @method static \Generator paginate(string $path, array $query = [], array $params = [])
 * @method static \Generator pages(string $path, array $query = [], array $params = [])
 * @method static \Dimmi\Generated\Resources\Contacts contacts()
 * @method static \Dimmi\Generated\Resources\Conversations conversations()
 *
 * Any other top-level resource works the same way: Dimmi::tags()->index().
 *
 * @see DimmiManager
 * @see \Dimmi\Client
 */
class Dimmi extends Facade
{
    protected static function getFacadeAccessor(): string
    {
        return DimmiManager::class;
    }
}
