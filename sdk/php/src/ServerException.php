<?php

declare(strict_types=1);

namespace Dimmi;

/**
 * Thrown for a dimmi API error; HTTP 5xx: dimmi failed; safe to retry later.
 */
class ServerException extends DimmiException
{
}
