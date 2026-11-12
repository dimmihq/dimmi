<?php

declare(strict_types=1);

namespace Dimmi;

/**
 * A webhook request whose signature, timestamp or body does not check
 * out. Answer it with 400 and do not act on it.
 */
class WebhookVerificationException extends \RuntimeException
{
}
