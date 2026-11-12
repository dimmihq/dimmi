<?php

declare(strict_types=1);

namespace Dimmi;

use StandardWebhooks\Exception\WebhookVerificationException as StandardException;
use StandardWebhooks\Webhook as StandardWebhook;

/**
 * Verifies dimmi webhooks (Standard Webhooks signatures).
 *
 *     $event = (new Dimmi\Webhook($_ENV['DIMMI_WEBHOOK_SECRET']))
 *         ->verify(file_get_contents('php://input'), getallheaders());
 *
 * Pass the raw body exactly as received, before any JSON decoding.
 */
final class Webhook
{
    private readonly StandardWebhook $webhook;

    /**
     * @param string $secret The endpoint's signing secret, whsec_...
     */
    public function __construct(#[\SensitiveParameter] string $secret)
    {
        $secret = trim($secret);
        if ($secret === '') {
            throw new \InvalidArgumentException('dimmi: the webhook secret (whsec_...) is empty.');
        }
        $this->webhook = new StandardWebhook($secret);
    }

    /**
     * @param array<string, string|list<string>> $headers Request headers, any case;
     *        $_SERVER-style HTTP_WEBHOOK_ID keys work too.
     * @throws WebhookVerificationException
     */
    public function verify(string $payload, array $headers): WebhookEvent
    {
        $normalized = self::headers($headers);
        foreach (['webhook-id', 'webhook-timestamp', 'webhook-signature'] as $name) {
            if (($normalized[$name] ?? '') === '') {
                throw new WebhookVerificationException("dimmi: the {$name} header is missing.");
            }
        }
        // Drop malformed signature entries the library cannot split.
        $normalized['webhook-signature'] = implode(' ', array_filter(
            explode(' ', $normalized['webhook-signature']),
            fn (string $part) => str_contains($part, ','),
        ));
        if ($normalized['webhook-signature'] === '') {
            throw new WebhookVerificationException('dimmi: the webhook-signature header has no signatures.');
        }

        try {
            $envelope = $this->webhook->verify($payload, $normalized);
        } catch (StandardException $e) {
            throw new WebhookVerificationException('dimmi: webhook verification failed: ' . $e->getMessage(), 0, $e);
        }
        if (!is_array($envelope)) {
            throw new WebhookVerificationException('dimmi: the webhook body is not a JSON object.');
        }

        return WebhookEvent::fromArray($envelope);
    }

    /**
     * @param array<string, mixed> $headers
     * @return array<string, string>
     */
    private static function headers(array $headers): array
    {
        $out = [];
        foreach ($headers as $name => $value) {
            $name = strtolower((string) $name);
            if (str_starts_with($name, 'http_')) {
                $name = substr($name, 5);
            }
            $name = str_replace('_', '-', $name);
            $out[$name] = is_array($value) ? (string) ($value[0] ?? '') : (string) $value;
        }

        return $out;
    }
}
