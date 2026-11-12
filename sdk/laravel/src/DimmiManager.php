<?php

declare(strict_types=1);

namespace Dimmi\Laravel;

use Dimmi\Client;
use Dimmi\Identity;
use Dimmi\Messenger;
use Dimmi\Resource;
use Dimmi\Webhook;
use Dimmi\WebhookEvent;
use Illuminate\Contracts\Auth\Authenticatable;
use Illuminate\Contracts\Container\Container;

/**
 * What the Dimmi facade stands for: the API client (every Client method
 * and resource), identity tokens for your users, the messenger snippet
 * and webhook verification, all configured from config/dimmi.php.
 *
 * @mixin Client
 */
class DimmiManager
{
    private ?Client $client = null;

    /**
     * @param array<string, mixed> $config
     */
    public function __construct(private readonly Container $app, private readonly array $config)
    {
    }

    /**
     * The REST API client.
     */
    public function client(): Client
    {
        return $this->client ??= new Client(
            $this->config['token'] ?? null,
            $this->config['workspace'] ?? null,
            $this->config['base_url'] ?? null,
            (array) ($this->config['client'] ?? []),
        );
    }

    /**
     * A signed identity token for the user, null for a guest.
     *
     * @param array<string, mixed> $options expiresIn, audience, nonce
     */
    public function identityToken(mixed $user, array $options = []): ?string
    {
        $identity = $this->identityFor($user);
        if ($identity === null) {
            return null;
        }
        $config = (array) ($this->config['identity'] ?? []);

        return Identity::sign(
            $identity,
            secret: $config['secret'] ?? null,
            kid: $config['kid'] ?? null,
            privateKey: self::pem($config['private_key'] ?? null),
            expiresIn: (int) ($options['expiresIn'] ?? $config['expires_in'] ?? 600),
            audience: $options['audience'] ?? $config['audience'] ?? null,
            nonce: $options['nonce'] ?? null,
        );
    }

    /**
     * The identity array for a user, through the configured resolver.
     *
     * @return array<string, mixed>|null
     */
    public function identityFor(mixed $user): ?array
    {
        if ($user === null) {
            return null;
        }
        if (is_array($user)) {
            return $user;
        }
        $resolver = $this->config['user']['resolver'] ?? null;
        if (is_string($resolver) && class_exists($resolver)) {
            $resolver = $this->app->make($resolver);
        }
        if (is_callable($resolver)) {
            return $resolver($user);
        }

        return (new UserIdentity((array) ($this->config['user']['attributes'] ?? [])))($user);
    }

    /**
     * The messenger snippet, identifying the signed-in user.
     *
     * @param array{nonce?: string, identity?: string} $options
     */
    public function messenger(?Authenticatable $user = null, array $options = []): string
    {
        $appId = $this->config['app_id'] ?? null;
        if (!$appId) {
            return "<!-- dimmi: set DIMMI_APP_ID to show the messenger -->\n";
        }
        $nonce = $options['nonce'] ?? (class_exists(\Illuminate\Foundation\Vite::class) ? $this->app->make(\Illuminate\Foundation\Vite::class)->cspNonce() : null);
        $mode = $options['identity'] ?? ($this->config['messenger']['identity'] ?? 'inline');

        if ($mode === 'url' && ($this->config['token_route']['path'] ?? null)) {
            return Messenger::snippet($appId, $this->host(), null, '/' . ltrim((string) $this->config['token_route']['path'], '/'), $nonce);
        }

        return Messenger::snippet($appId, $this->host(), $this->identityToken($user), null, $nonce);
    }

    /**
     * Verifies a webhook request body and its headers.
     *
     * @param array<string, mixed> $headers
     */
    public function verifyWebhook(string $payload, array $headers): WebhookEvent
    {
        return (new Webhook((string) ($this->config['webhook']['secret'] ?? '')))->verify($payload, $headers);
    }

    /**
     * Where browsers load the messenger from.
     */
    public function host(): string
    {
        $host = $this->config['host'] ?? null ?: ($this->config['base_url'] ?? '');

        return (string) preg_replace('#/api/v\d+/?$#', '', rtrim((string) $host, '/'));
    }

    /**
     * @return array<string, mixed>
     */
    public function config(): array
    {
        return $this->config;
    }

    public function __get(string $name): Resource
    {
        return $this->client()->{$name};
    }

    /**
     * @param array<int, mixed> $arguments
     */
    public function __call(string $method, array $arguments): mixed
    {
        $client = $this->client();
        if ($arguments === [] && !method_exists($client, $method) && isset($client->{$method})) {
            return $client->{$method};
        }

        return $client->{$method}(...$arguments);
    }

    private static function pem(?string $value): ?string
    {
        if ($value === null || $value === '') {
            return null;
        }
        if (!str_contains($value, '-----BEGIN') && is_file($value)) {
            return (string) file_get_contents($value);
        }

        return str_replace('\n', "\n", $value);
    }
}
