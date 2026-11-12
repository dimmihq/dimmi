<?php

declare(strict_types=1);

namespace Dimmi\Laravel\Http\Controllers;

use Dimmi\Laravel\DimmiManager;
use Dimmi\Laravel\Events\DimmiWebhookReceived;
use Dimmi\WebhookVerificationException;
use Illuminate\Contracts\Events\Dispatcher;
use Illuminate\Http\JsonResponse;
use Illuminate\Http\Request;

/**
 * Verifies a dimmi webhook and fires DimmiWebhookReceived plus the named
 * event "dimmi.webhook.<type>". Answers 400 to anything unsigned.
 */
class WebhookController
{
    public function __invoke(Request $request, DimmiManager $dimmi, Dispatcher $events): JsonResponse
    {
        try {
            $event = $dimmi->verifyWebhook($request->getContent(), $request->headers->all());
        } catch (WebhookVerificationException|\InvalidArgumentException $e) {
            return new JsonResponse(['code' => 'webhook_invalid', 'message' => $e->getMessage()], 400);
        }

        $events->dispatch(new DimmiWebhookReceived($event));
        $events->dispatch('dimmi.webhook.' . $event->type, [$event]);

        return new JsonResponse(['received' => true]);
    }
}
