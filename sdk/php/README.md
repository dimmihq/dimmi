# dimmi-php

The PHP client for dimmi: the REST API, identity tokens for the messenger and webhook verification.

```bash
composer require dimmi/dimmi-php
```

PHP 8.2 or newer. Using Laravel? Install [`dimmi/dimmi-laravel`](../laravel) instead; it wraps this package.

## The API

```php
$dimmi = new Dimmi\Client(
    token: getenv('DIMMI_TOKEN'),          // falls back to DIMMI_TOKEN
    workspace: getenv('DIMMI_WORKSPACE'),  // falls back to DIMMI_WORKSPACE
    baseUrl: 'https://dimmi.acme.com',     // falls back to DIMMI_BASE_URL; "/api/v1" is added
);

$contact = $dimmi->contacts->store(['email' => 'ada@example.com', 'name' => 'Ada']);

$conversation = $dimmi->conversations->store([
    'inbox_id' => 'inbox_...',
    'contact' => ['id' => $contact['data']['id']],
    'subject' => 'Your order',
    'body' => 'Hi Ada, your order shipped.',
]);

$dimmi->conversations->messages->store($conversation['data']['id'], ['body' => 'Tracking: 1Z...']);
```

Every operation in the OpenAPI document is reachable three ways:

| Style | Example |
| --- | --- |
| Resource method | `$dimmi->webhookEndpoints->tests->store($id, [])` |
| Operation id | `$dimmi->call('webhook-endpoints.tests.store', path: [$id], body: [])` |
| Method and path | `$dimmi->request('POST', "/webhook-endpoints/{$id}/tests", body: [])` |

Resource names are the operation id in camelCase: `ai.action-calls.index` is `$dimmi->ai->actionCalls->index()`. Methods take path parameters first, then `$body` (writes only), `$query` and `$options`. Answers are the decoded JSON as arrays, `null` for an empty answer.

### Lists

```php
foreach ($dimmi->paginate('/conversations', ['limit' => 50]) as $conversation) {
    // every conversation, following meta.next_cursor
}

foreach ($dimmi->pages('/tags', ['limit' => 100]) as $page) {
    // lists that use ?page=1, 2, ... ; each $page is a list of items
}
```

Both take an operation id too: `$dimmi->paginate('conversations.messages.index', params: [$id])`.

### Retries and idempotency

429 and 503 answers and connection failures are retried twice by default, waiting as long as `Retry-After` says (or backing off from 0.5 s). Every `POST` carries an `Idempotency-Key`, the same one on each retry, so a retried write never runs twice. Pass your own to make a write safe across your own retries:

```php
$dimmi->contacts->store($body, options: ['idempotencyKey' => "signup-{$user->id}"]);

new Dimmi\Client(options: ['maxRetries' => 5, 'timeout' => 10, 'maxRetryDelay' => 30]);
```

### Errors

An error answer throws a `Dimmi\DimmiException` subclass with `status`, `errorCode` and `fields` (per-field messages):

| Status | Exception |
| --- | --- |
| 400, 422 | `ValidationException` |
| 401 | `AuthenticationException` |
| 403 | `PermissionDeniedException` |
| 404 | `NotFoundException` |
| 429 | `RateLimitException` (`retryAfter` in seconds) |
| 5xx | `ServerException` |

```php
try {
    $dimmi->contacts->store(['email' => 'not-an-email']);
} catch (Dimmi\ValidationException $e) {
    $e->errorCode;  // "validation_failed"
    $e->fields;     // ['email' => ['...']]
}
```

## Identity tokens

The messenger trusts who a visitor is only from a token your server signs.

```php
$token = Dimmi\Identity::sign([
    'id' => $user->id,
    'email' => $user->email,
    'emailVerified' => true,
    'name' => $user->name,
    'signedUpAt' => $user->created_at,
    'company' => ['id' => $team->id, 'name' => $team->name],
    'attributes' => ['plan' => 'pro'],
]);

echo Dimmi\Messenger::snippet('wgt_...', 'https://dimmi.acme.com', $token);
```

- The key is `DIMMI_IDENTITY_SECRET` (HS256), or a PKCS #8 PEM private key in `DIMMI_IDENTITY_PRIVATE_KEY` with `DIMMI_IDENTITY_KID` (ES256, RS256 or EdDSA, read from the key). Or pass `secret:`, `kid:`, `privateKey:`, `algorithm:`.
- `expiresIn` is 1 to 3600 seconds, 600 by default. `audience:` and `nonce:` are optional.
- `email_verified` is set only when `emailVerified` is `true`. Empty fields are left out.
- Attributes are at most 100 snake_case keys with flat values.
- A bad user throws `Dimmi\IdentityException` with a stable `errorCode` and the fix in the message, such as `[dimmi] user_field_unknown: put plan under attributes.`

For long sessions, serve `{"token": "..."}` (with `Cache-Control: no-store, private`) from a route and let the messenger fetch it:

```php
echo Dimmi\Messenger::snippet('wgt_...', 'https://dimmi.acme.com', identityUrl: '/dimmi/token', nonce: $cspNonce);
```

## Webhooks

```php
try {
    $event = (new Dimmi\Webhook(getenv('DIMMI_WEBHOOK_SECRET')))
        ->verify(file_get_contents('php://input'), getallheaders());
} catch (Dimmi\WebhookVerificationException $e) {
    http_response_code(400);
    exit;
}

$event->type;        // "conversation.created"
$event->object();    // the conversation
$event->id; $event->createdAt; $event->workspaceId; $event->actor; $event->test; $event->raw;
```

Signatures follow [Standard Webhooks](https://www.standardwebhooks.com) and are checked with its official library, including the 5-minute timestamp window. Pass the raw body, before any JSON decoding.

## Why a generated operation table, not a generated SDK

The dimmi OpenAPI document has about 500 operations and over 900 schemas, in OpenAPI 3.1. Full generators were considered and rejected:

- openapi-generator and Jane emit a model class per schema: tens of thousands of lines to ship and review.
- Both still trip on 3.1 nullable types (`"type": ["string", "null"]`), so the models would need hand fixes after every regeneration.

So this client is thin. `bin/generate` reads the spec and writes:

- `src/Generated/Operations.php`: operation id to method, path, path and query parameters, request and response schema names.
- `src/Generated/Resources/*`: one small class per resource with PHPDoc, so IDEs autocomplete `$dimmi->conversations->messages->store()`.

Requests and answers stay plain arrays, and a new endpoint is one regeneration away.

## Regenerating after API changes

`src/Generated` is never committed in this repository (.gitignore); `make sdk-generate` writes it from the committed contract. The release generates it and splits `sdk/php`, generated code included, into the read-only mirror `dimmihq/dimmi-php`, which Packagist serves. After cloning and after the API changes:

```bash
cd sdk/php
composer install
bin/generate                                   # tests/contracts/api_v1.openapi.json
bin/generate path/to/openapi.json              # or another snapshot
bin/generate http://localhost:8080/api/v1/openapi.json
vendor/bin/phpunit
```

With no argument it reads `tests/contracts/api_v1.openapi.json` at the repository root, or `http://localhost:8080/api/v1/openapi.json` when that file is missing. `composer generate` runs the same script.

Public methods never break between runs. The current `src/Generated/Operations.php` is the baseline:

- An operation the spec dropped is kept and marked `@deprecated`, for removal in the next major version.
- A renamed path parameter keeps its old argument name.
- A body added to an operation that had none becomes the last argument, `$body = null`.
- A body the spec dropped stays an optional argument; a body that became required stays optional in the signature.
- Changing the number of path parameters cannot be kept compatible; the script warns, and that change needs a major version.

The script prints each of these as a warning. Review them before committing the contract change; `make sdk-compat` compares the public surface with the last release on the mirror.

## Development

```bash
composer install
vendor/bin/phpunit
```
