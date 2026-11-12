<?php

declare(strict_types=1);

namespace Dimmi;

use Psr\Http\Message\ResponseInterface;

/**
 * An error answer from the dimmi API: {"code", "message", "errors"}.
 *
 * $status is the HTTP status, $errorCode the stable code such as
 * "validation_failed", and $fields the per-field messages, keyed by field.
 */
class DimmiException extends \RuntimeException
{
    /**
     * @param array<string, list<string>> $fields
     * @param array<string, mixed>|null $body
     */
    public function __construct(
        string $message,
        public readonly int $status = 0,
        public readonly ?string $errorCode = null,
        public readonly array $fields = [],
        public readonly ?array $body = null,
        public readonly ?ResponseInterface $response = null,
        ?\Throwable $previous = null,
    ) {
        parent::__construct($message, $status, $previous);
    }

    /**
     * Builds the right exception class for an error response.
     */
    public static function fromResponse(ResponseInterface $response): self
    {
        $status = $response->getStatusCode();
        $raw = (string) $response->getBody();
        $body = json_decode($raw, true);
        $body = is_array($body) ? $body : null;

        $code = is_string($body['code'] ?? null) ? $body['code'] : null;
        $message = is_string($body['message'] ?? null) && $body['message'] !== ''
            ? $body['message']
            : sprintf('dimmi answered HTTP %d', $status);
        $fields = self::fields($body['errors'] ?? $body['fields'] ?? []);

        $class = match (true) {
            $status === 401 => AuthenticationException::class,
            $status === 403 => PermissionDeniedException::class,
            $status === 404 => NotFoundException::class,
            $status === 400, $status === 422 => ValidationException::class,
            $status === 429 => RateLimitException::class,
            $status >= 500 => ServerException::class,
            default => self::class,
        };

        if ($class === RateLimitException::class) {
            return new RateLimitException($message, $status, $code, $fields, $body, $response, retryAfter: self::retryAfter($response));
        }

        return new $class($message, $status, $code, $fields, $body, $response);
    }

    /**
     * @return array<string, list<string>>
     */
    private static function fields(mixed $errors): array
    {
        if (!is_array($errors)) {
            return [];
        }
        $out = [];
        foreach ($errors as $field => $messages) {
            $out[(string) $field] = array_values(array_map('strval', array_filter((array) $messages, 'is_scalar')));
        }

        return $out;
    }

    public static function retryAfter(?ResponseInterface $response): ?int
    {
        if ($response === null || !$response->hasHeader('Retry-After')) {
            return null;
        }
        $value = trim($response->getHeaderLine('Retry-After'));
        if (is_numeric($value)) {
            return max(0, (int) ceil((float) $value));
        }
        $time = strtotime($value);

        return $time === false ? null : max(0, $time - time());
    }
}
