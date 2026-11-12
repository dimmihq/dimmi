<?php

declare(strict_types=1);

namespace Dimmi\Laravel\Events;

use Dimmi\WebhookEvent;
use Illuminate\Foundation\Events\Dispatchable;

/**
 * Fired for every verified dimmi webhook. The same event is also fired
 * by name as "dimmi.webhook.<type>", such as "dimmi.webhook.conversation.created".
 */
class DimmiWebhookReceived
{
    use Dispatchable;

    public function __construct(public readonly WebhookEvent $event)
    {
    }
}
