<?php

declare(strict_types=1);

namespace Dimmi;

/**
 * Lazily builds the resource accessors listed in a class's RESOURCES
 * constant, so $dimmi->conversations->messages costs nothing until used.
 *
 * @internal
 */
trait HasResources
{
    /** @var array<string, Resource> */
    private array $resourceCache = [];

    public function __get(string $name): Resource
    {
        if (isset($this->resourceCache[$name])) {
            return $this->resourceCache[$name];
        }
        $class = static::RESOURCES[$name] ?? null;
        if ($class === null) {
            throw new \Error(sprintf('dimmi: %s has no resource named %s', static::class, $name));
        }

        return $this->resourceCache[$name] = new $class($this->dimmiClient());
    }

    public function __isset(string $name): bool
    {
        return isset(static::RESOURCES[$name]);
    }

    abstract protected function dimmiClient(): Client;
}
