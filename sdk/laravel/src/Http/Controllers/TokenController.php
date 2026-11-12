<?php

declare(strict_types=1);

namespace Dimmi\Laravel\Http\Controllers;

use Dimmi\Laravel\DimmiManager;
use Illuminate\Http\JsonResponse;
use Illuminate\Http\Request;

/**
 * GET /dimmi/token: {"token": "..."} for the signed-in user, {"token": null}
 * for a guest. Never cached, never a redirect to a login page.
 */
class TokenController
{
    public function __invoke(Request $request, DimmiManager $dimmi): JsonResponse
    {
        return new JsonResponse(['token' => $dimmi->identityToken($request->user())], 200, [
            'Cache-Control' => 'no-store, private',
        ]);
    }
}
