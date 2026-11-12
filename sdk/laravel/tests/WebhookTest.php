<?php

declare(strict_types=1);

namespace Dimmi\Laravel\Tests;

use Dimmi\Laravel\Events\DimmiWebhookReceived;
use Dimmi\WebhookEvent;
use Illuminate\Support\Facades\Event;
use Illuminate\Support\Facades\Route;
use StandardWebhooks\Webhook as StandardWebhook;

class WebhookTest extends TestCase
{
    protected function defineRoutes($router): void
    {
        // Inside the web group, as in routes/web.php: CSRF must be skipped.
        Route::middleware('web')->group(fn () => Route::dimmiWebhooks('dimmi/webhook'));
    }

    private function send(string $payload, ?string $secret = null): \Illuminate\Testing\TestResponse
    {
        $t = time();
        $signature = (new StandardWebhook($secret ?? self::WEBHOOK_SECRET))->sign('msg_1', $t, $payload);

        return $this->call('POST', '/dimmi/webhook', [], [], [], [
            'CONTENT_TYPE' => 'application/json',
            'HTTP_WEBHOOK_ID' => 'msg_1',
            'HTTP_WEBHOOK_TIMESTAMP' => (string) $t,
            'HTTP_WEBHOOK_SIGNATURE' => $signature,
        ], $payload);
    }

    private static function payload(): string
    {
        return json_encode(['id' => 'evt_1', 'object' => 'event', 'type' => 'conversation.created', 'created_at' => '2026-11-12T10:00:00Z', 'workspace_id' => 'ws_1', 'test' => true, 'actor' => ['type' => 'member', 'id' => 'm'], 'data' => ['object' => ['id' => 'cnv_1']]]);
    }

    public function testVerifiesAndDispatchesBothEvents(): void
    {
        Event::fake([DimmiWebhookReceived::class, 'dimmi.webhook.conversation.created']);

        $this->send(self::payload())->assertOk()->assertJson(['received' => true]);

        Event::assertDispatched(DimmiWebhookReceived::class, fn (DimmiWebhookReceived $e) => $e->event->id === 'evt_1' && $e->event->object()['id'] === 'cnv_1');
        Event::assertDispatched('dimmi.webhook.conversation.created', fn (string $name, array $payload) => $payload[0] instanceof WebhookEvent && $payload[0]->type === 'conversation.created');
    }

    public function testNamedListenersReceiveTheEvent(): void
    {
        $got = null;
        Event::listen('dimmi.webhook.conversation.created', function (WebhookEvent $event) use (&$got) {
            $got = $event->id;
        });
        $this->send(self::payload())->assertOk();
        $this->assertSame('evt_1', $got);
    }

    public function testRefusesBadSignatures(): void
    {
        Event::fake();
        $this->send(self::payload(), 'whsec_' . base64_encode('wrong secret value'))->assertStatus(400)->assertJson(['code' => 'webhook_invalid']);
        $this->postJson('/dimmi/webhook', ['type' => 'x'])->assertStatus(400);
        Event::assertNotDispatched(DimmiWebhookReceived::class);
    }

    public function testRouteIsNamed(): void
    {
        $this->assertSame(url('/dimmi/webhook'), route('dimmi.webhook'));
    }
}
