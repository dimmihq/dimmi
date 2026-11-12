<?php

declare(strict_types=1);

namespace Dimmi;

/**
 * Thrown for a dimmi API error; HTTP 403: the token lacks the ability or workspace access.
 */
class PermissionDeniedException extends DimmiException
{
}
