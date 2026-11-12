<?php

declare(strict_types=1);

namespace Dimmi;

/**
 * Base of the generated resource accessors. Each generated method is a
 * one-line call into Client::call with the operation id from the spec.
 */
abstract class Resource
{
    use HasResources;

    /** @var array<string, class-string<Resource>> */
    protected const RESOURCES = [];

    public function __construct(protected readonly Client $client)
    {
    }

    protected function dimmiClient(): Client
    {
        return $this->client;
    }
}
