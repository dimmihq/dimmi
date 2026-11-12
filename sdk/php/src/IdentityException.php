<?php

declare(strict_types=1);

namespace Dimmi;

/**
 * A user that cannot become an identity token, or a missing or bad key.
 * $errorCode is stable (identify_user_invalid, user_field_unknown,
 * attribute_limit, attribute_key_invalid, attribute_value_invalid,
 * expires_in_invalid, kid_required, secret_missing, private_key_invalid)
 * and the message carries the fix: "[dimmi] <code>: <fix>".
 */
class IdentityException extends \InvalidArgumentException
{
    public function __construct(public readonly string $errorCode, string $fix, ?\Throwable $previous = null)
    {
        parent::__construct("[dimmi] {$errorCode}: {$fix}", 0, $previous);
    }
}
