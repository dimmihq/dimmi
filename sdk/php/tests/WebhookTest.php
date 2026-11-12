<?php

declare(strict_types=1);

namespace Dimmi\Tests;

use Dimmi\Webhook;
use Dimmi\WebhookEvent;
use Dimmi\WebhookVerificationException;
use PHPUnit\Framework\TestCase;
use StandardWebhooks\Webhook as StandardWebhook;

final class WebhookTest extends TestCase
{
    private const SECRET = 'whsec_MfKQ9r8GKYqrTwjUPD8ILPZIo2LaLaSw';

    private static function envelope(): string
    {
        return json_encode([
            'id' => 'evt_1',
            'object' => 'event',
            'type' => 'conversation.created',
            'api_version' => '2026-11-01',
            'created_at' => '2026-11-12T10:00:00Z',
            'workspace_id' => 'ws_1',
            'sequence' => 3,
            'test' => true,
            'test_mode' => false,
            'truncated' => false,
            'actor' => ['type' => 'member', 'id' => 'mem_1'],
            'data' => ['object' => ['id' => 'cnv_1', 'object' => 'conversation']],
        ]);
    }

    /**
     * @return array<string, string>
     */
    private static function headers(string $payload, ?int $timestamp = null, string $secret = self::SECRET): array
    {
        $timestamp ??= time();

        return [
            'Webhook-Id' => 'msg_1',
            'Webhook-Timestamp' => (string) $timestamp,
            'Webhook-Signature' => 'v1,bm9wZQ== ' . (new StandardWebhook($secret))->sign('msg_1', $timestamp, $payload),
        ];
    }

    public function testVerifiesASignedEvent(): void
    {
        $payload = self::envelope();
        $event = (new Webhook(self::SECRET))->verify($payload, self::headers($payload));

        $this->assertInstanceOf(WebhookEvent::class, $event);
        $this->assertSame('evt_1', $event->id);
        $this->assertSame('conversation.created', $event->type);
        $this->assertSame('2026-11-12T10:00:00+00:00', $event->createdAt?->format(DATE_ATOM));
        $this->assertSame('ws_1', $event->workspaceId);
        $this->assertSame(['type' => 'member', 'id' => 'mem_1'], $event->actor);
        $this->assertTrue($event->test);
        $this->assertSame('cnv_1', $event->object()['id']);
        $this->assertSame(3, $event->raw['sequence']);
    }

    public function testAcceptsServerStyleAndListHeaders(): void
    {
        $payload = self::envelope();
        $h = self::headers($payload);
        $server = ['HTTP_WEBHOOK_ID' => $h['Webhook-Id'], 'HTTP_WEBHOOK_TIMESTAMP' => $h['Webhook-Timestamp'], 'HTTP_WEBHOOK_SIGNATURE' => $h['Webhook-Signature']];
        $this->assertSame('evt_1', (new Webhook(self::SECRET))->verify($payload, $server)->id);
        $lists = array_map(fn ($v) => [$v], $h);
        $this->assertSame('evt_1', (new Webhook(self::SECRET))->verify($payload, $lists)->id);
    }

    public function testRefusesATamperedBody(): void
    {
        $payload = self::envelope();
        $headers = self::headers($payload);
        $this->expectException(WebhookVerificationException::class);
        (new Webhook(self::SECRET))->verify(str_replace('cnv_1', 'cnv_2', $payload), $headers);
    }

    public function testRefusesTheWrongSecret(): void
    {
        $payload = self::envelope();
        $this->expectException(WebhookVerificationException::class);
        (new Webhook('whsec_' . base64_encode('another secret entirely')))->verify($payload, self::headers($payload));
    }

    public function testRefusesAnOldTimestamp(): void
    {
        $payload = self::envelope();
        $this->expectException(WebhookVerificationException::class);
        $this->expectExceptionMessage('too old');
        (new Webhook(self::SECRET))->verify($payload, self::headers($payload, time() - 3600));
    }

    public function testRefusesMissingAndMalformedHeaders(): void
    {
        $payload = self::envelope();
        try {
            (new Webhook(self::SECRET))->verify($payload, ['webhook-id' => 'msg_1']);
            $this->fail('expected an exception');
        } catch (WebhookVerificationException $e) {
            $this->assertStringContainsString('webhook-timestamp', $e->getMessage());
        }
        $this->expectException(WebhookVerificationException::class);
        (new Webhook(self::SECRET))->verify($payload, ['webhook-id' => 'msg_1', 'webhook-timestamp' => (string) time(), 'webhook-signature' => 'garbage']);
    }
}
