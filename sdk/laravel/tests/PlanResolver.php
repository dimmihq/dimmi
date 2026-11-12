<?php

declare(strict_types=1);

namespace Dimmi\Laravel\Tests;

class PlanResolver
{
    public function __invoke($user): array
    {
        return ['id' => 'custom-' . $user->id, 'attributes' => ['plan' => 'enterprise']];
    }
}
