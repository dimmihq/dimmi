<?php

declare(strict_types=1);

namespace Dimmi\Laravel\Tests;

use Illuminate\Foundation\Auth\User as Authenticatable;

class User extends Authenticatable
{
    protected $guarded = [];

    protected $casts = ['email_verified_at' => 'datetime', 'created_at' => 'datetime'];
}
