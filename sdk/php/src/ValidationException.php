<?php

declare(strict_types=1);

namespace Dimmi;

/**
 * Thrown for a dimmi API error; HTTP 400 or 422: the request was refused; see $fields.
 */
class ValidationException extends DimmiException
{
}
