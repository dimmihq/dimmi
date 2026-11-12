<?php

declare(strict_types=1);

namespace Dimmi;

/**
 * Thrown for a dimmi API error; HTTP 401: the token is missing, wrong or revoked.
 */
class AuthenticationException extends DimmiException
{
}
