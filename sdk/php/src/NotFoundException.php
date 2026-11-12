<?php

declare(strict_types=1);

namespace Dimmi;

/**
 * Thrown for a dimmi API error; HTTP 404: the record or route does not exist.
 */
class NotFoundException extends DimmiException
{
}
