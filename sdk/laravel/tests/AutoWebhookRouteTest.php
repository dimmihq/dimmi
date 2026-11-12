<?php

declare(strict_types=1);

namespace Dimmi\Laravel\Tests;

class AutoWebhookRouteTest extends TestCase
{
    protected function defineEnvironment($app): void
    {
        parent::defineEnvironment($app);
        $app['config']->set('dimmi.webhook.path', 'hooks/dimmi');
        $app['config']->set('dimmi.token_route.path', null);
    }

    public function testConfigPathRegistersTheReceiverAndTokenRouteCanBeOff(): void
    {
        $this->postJson('/hooks/dimmi', [])->assertStatus(400);
        $this->get('/dimmi/token')->assertNotFound();
    }
}
