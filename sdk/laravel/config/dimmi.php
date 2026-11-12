<?php

return [

    /*
    | REST API access. The token is a workspace API token; base_url is your
    | dimmi host (https://dimmi.acme.com) or its API base (.../api/v1).
    */
    'token' => env('DIMMI_TOKEN'),
    'workspace' => env('DIMMI_WORKSPACE'),
    'base_url' => env('DIMMI_BASE_URL'),

    // Client options passed to Dimmi\Client: maxRetries, timeout, retryDelay...
    'client' => [
        'maxRetries' => 2,
        'timeout' => 30,
    ],

    /*
    | The messenger. host is where browsers load /widget/v1/loader.js from,
    | the base_url host unless set. app_id is the messenger app (wgt_...).
    | identity: "inline" signs a token into the page; "url" lets the
    | messenger fetch it from the token route, better for long sessions.
    */
    'host' => env('DIMMI_HOST'),
    'app_id' => env('DIMMI_APP_ID'),
    'messenger' => [
        'identity' => env('DIMMI_MESSENGER_IDENTITY', 'inline'),
    ],

    /*
    | Identity verification. Either the shared secret (HS256) or a PKCS #8
    | PEM private key (contents or a file path) with its key id.
    */
    'identity' => [
        'secret' => env('DIMMI_IDENTITY_SECRET'),
        'kid' => env('DIMMI_IDENTITY_KID'),
        'private_key' => env('DIMMI_IDENTITY_PRIVATE_KEY'),
        'expires_in' => 600,
        'audience' => null,
    ],

    /*
    | How a user of your app becomes a dimmi identity. resolver may be a
    | class with __invoke($user): ?array or any callable returning the
    | array Dimmi\Identity::sign takes. Without one, id, email, name,
    | email_verified_at and created_at are used, plus the model attributes
    | listed here: ['plan', 'team_name' => 'team.name'].
    */
    'user' => [
        'resolver' => null,
        'attributes' => [],
    ],

    /*
    | GET route answering {"token": ...} for the signed-in user, never
    | cached. Set path to null to turn it off.
    */
    'token_route' => [
        'path' => env('DIMMI_TOKEN_PATH', 'dimmi/token'),
        'middleware' => ['web'],
    ],

    /*
    | Webhooks. secret is the endpoint's whsec_... signing secret. Set path
    | to register the receiver automatically, or call
    | Route::dimmiWebhooks('dimmi/webhook') in a routes file.
    */
    'webhook' => [
        'secret' => env('DIMMI_WEBHOOK_SECRET'),
        'path' => env('DIMMI_WEBHOOK_PATH'),
    ],

];
