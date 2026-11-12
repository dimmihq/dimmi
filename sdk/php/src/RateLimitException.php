<?php

declare(strict_types=1);

namespace Dimmi;

use Psr\Http\Message\ResponseInterface;

/**
 * HTTP 429 after the client ran out of retries. $retryAfter is the
 * number of seconds dimmi asked to wait, when it said.
 */
class RateLimitException extends DimmiException
{
    /**
     * @param array<string, list<string>> $fields
     * @param array<string, mixed>|null $body
     */
    public function __construct(
        string $message,
        int $status = 429,
        ?string $errorCode = null,
        array $fields = [],
        ?array $body = null,
        ?ResponseInterface $response = null,
        ?\Throwable $previous = null,
        public readonly ?int $retryAfter = null,
    ) {
        parent::__construct($message, $status, $errorCode, $fields, $body, $response, $previous);
    }
}
