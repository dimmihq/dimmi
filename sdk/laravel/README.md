# dimmi-laravel

dimmi for Laravel 10, 11 and 12: the messenger with signed identity, the REST API, webhooks and artisan checks.

```bash
composer require dimmi/dimmi-laravel
php artisan dimmi:install
```

`dimmi:install` publishes `config/dimmi.php` and adds the keys to `.env` and `.env.example`:

```dotenv
DIMMI_BASE_URL=https://dimmi.acme.com
DIMMI_TOKEN=
DIMMI_WORKSPACE=ws_...
DIMMI_APP_ID=wgt_...
DIMMI_IDENTITY_SECRET=dmi_idsec_...
DIMMI_IDENTITY_KID=
DIMMI_WEBHOOK_SECRET=whsec_...
```

Then check everything against your dimmi:

```bash
php artisan dimmi:test
```

It calls the API, signs an identity token and has dimmi validate it, and exits non-zero when anything fails, so it fits a deploy pipeline.

## The messenger

Put the directive before `</body>` in your layout:

```blade
@dimmiMessenger
```

A signed-in user (`auth()->user()`) is identified with a ten-minute token. Guests get the anonymous messenger. Vite's CSP nonce is added when set.

Long sessions can let the messenger fetch the token itself from the built-in `GET /dimmi/token` route (`{"token": ...}`, never cached):

```blade
@dimmiMessenger(['identity' => 'url'])
```

Or set `DIMMI_MESSENGER_IDENTITY=url` for every page.

### What a user sends

By default: the auth identifier, `email`, `name`, `email_verified_at` (as verified email) and `created_at` (as signed-up time). Add model attributes in `config/dimmi.php`:

```php
'user' => [
    'attributes' => ['plan', 'team_name' => 'team.name'],
],
```

Or take over completely with a class or callable:

```php
'user' => ['resolver' => App\Support\DimmiIdentity::class],

class DimmiIdentity
{
    public function __invoke(User $user): array
    {
        return [
            'id' => $user->id,
            'email' => $user->email,
            'emailVerified' => $user->hasVerifiedEmail(),
            'company' => ['id' => $user->team_id, 'name' => $user->team->name],
            'attributes' => ['plan' => $user->plan],
        ];
    }
}
```

Sign with a private key instead of the secret: set `DIMMI_IDENTITY_PRIVATE_KEY` to the PEM or a path to it, plus `DIMMI_IDENTITY_KID`.

## The API

The `Dimmi` facade is the [`dimmi/dimmi-php`](../php) client:

```php
use Dimmi\Laravel\Facades\Dimmi;

Dimmi::contacts()->store(['email' => $user->email, 'external_id' => (string) $user->id]);
Dimmi::conversations()->messages->store($id, ['body' => 'Thanks!']);
Dimmi::call('tags.index');

foreach (Dimmi::paginate('/conversations') as $conversation) { /* ... */ }

$token = Dimmi::identityToken($user);
```

`Dimmi\Client` can also be injected. Errors throw the typed exceptions of `dimmi/dimmi-php`.

## Webhooks

Register the receiver in `routes/web.php` (CSRF is skipped for it):

```php
Route::dimmiWebhooks('dimmi/webhook');
```

Or set `DIMMI_WEBHOOK_PATH=dimmi/webhook` to register it outside the web middleware group. Unsigned requests get 400.

Each verified webhook fires two events, both carrying a `Dimmi\WebhookEvent`:

- `Dimmi\Laravel\Events\DimmiWebhookReceived` for every type.
- A string event named `dimmi.webhook.<type>`, such as `dimmi.webhook.conversation.created`.

String events were picked over one class per type: dimmi adds event types over time, and every one works without a package update.

```php
use Dimmi\Laravel\Events\DimmiWebhookReceived;
use Dimmi\WebhookEvent;

Event::listen(DimmiWebhookReceived::class, function (DimmiWebhookReceived $received) {
    logger('dimmi', ['type' => $received->event->type]);
});

Event::listen('dimmi.webhook.conversation.created', function (WebhookEvent $event) {
    $conversation = $event->object();
});

Event::listen('dimmi.webhook.*', function (string $name, array $payload) {
    [$event] = $payload;
});
```

Listeners can be queued as usual. Answer fast: dimmi retries webhooks that time out.

## Configuration

| Key | Env | Purpose |
| --- | --- | --- |
| `token`, `workspace`, `base_url` | `DIMMI_TOKEN`, `DIMMI_WORKSPACE`, `DIMMI_BASE_URL` | API access |
| `client` | | `Dimmi\Client` options: `maxRetries`, `timeout`, ... |
| `host` | `DIMMI_HOST` | Where browsers load the messenger; the `base_url` host by default |
| `app_id` | `DIMMI_APP_ID` | The messenger app, `wgt_...` |
| `messenger.identity` | `DIMMI_MESSENGER_IDENTITY` | `inline` or `url` |
| `identity.*` | `DIMMI_IDENTITY_SECRET`, `DIMMI_IDENTITY_KID`, `DIMMI_IDENTITY_PRIVATE_KEY` | Signing key, `expires_in`, `audience` |
| `user.resolver`, `user.attributes` | | User to identity mapping |
| `token_route.path` | `DIMMI_TOKEN_PATH` | `dimmi/token`; `null` turns it off |
| `webhook.secret`, `webhook.path` | `DIMMI_WEBHOOK_SECRET`, `DIMMI_WEBHOOK_PATH` | Webhook receiver |

## Regenerating after API changes

The API methods (`Dimmi::contacts()`, `Dimmi::call(...)`) come from `dimmi/dimmi-php`, so this package has nothing to regenerate. After the API changes, regenerate the PHP client, then run both test suites:

```bash
cd sdk/php && composer install && bin/generate && vendor/bin/phpunit
cd ../laravel && composer install && vendor/bin/phpunit
```

`sdk/laravel` installs `sdk/php` through a path repository, so its tests use the code just generated. `make sdk-check` does all of this for every SDK. The release splits `sdk/laravel` into the read-only mirror `dimmihq/dimmi-laravel`, without the path repository.

## Development

```bash
composer install
vendor/bin/phpunit
```
