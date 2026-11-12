<?php

declare(strict_types=1);

namespace Dimmi;

/**
 * A verified webhook event.
 *
 * $data is the event's data, usually with the record under data['object'];
 * $actor is who caused it ({type, id}); $raw is the whole decoded envelope
 * (api_version, sequence, truncated and so on).
 */
final class WebhookEvent
{
    /**
     * @param array<string, mixed> $data
     * @param array<string, mixed>|null $actor
     * @param array<string, mixed> $raw
     */
    public function __construct(
        public readonly string $id,
        public readonly string $type,
        public readonly ?\DateTimeImmutable $createdAt,
        public readonly ?string $workspaceId,
        public readonly array $data,
        public readonly ?array $actor,
        public readonly bool $test,
        public readonly array $raw,
    ) {
    }

    /**
     * @param array<string, mixed> $envelope
     */
    public static function fromArray(array $envelope): self
    {
        $created = null;
        if (is_string($envelope['created_at'] ?? null)) {
            try {
                $created = new \DateTimeImmutable($envelope['created_at']);
            } catch (\Exception) {
                $created = null;
            }
        }

        return new self(
            id: (string) ($envelope['id'] ?? ''),
            type: (string) ($envelope['type'] ?? ''),
            createdAt: $created,
            workspaceId: isset($envelope['workspace_id']) ? (string) $envelope['workspace_id'] : null,
            data: is_array($envelope['data'] ?? null) ? $envelope['data'] : [],
            actor: is_array($envelope['actor'] ?? null) ? $envelope['actor'] : null,
            test: (bool) ($envelope['test'] ?? $envelope['test_mode'] ?? false),
            raw: $envelope,
        );
    }

    /**
     * The record the event is about, data['object'], when there is one.
     *
     * @return array<string, mixed>|null
     */
    public function object(): ?array
    {
        return is_array($this->data['object'] ?? null) ? $this->data['object'] : null;
    }
}
