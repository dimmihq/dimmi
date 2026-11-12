<?php

declare(strict_types=1);

namespace Dimmi\Laravel;

use Illuminate\Support\Arr;
use Illuminate\Support\Str;

/**
 * The default user → identity mapping: the auth identifier, email, name,
 * email_verified_at (as emailVerified) and created_at (as signedUpAt),
 * plus the attributes named in config('dimmi.user.attributes'), either
 * ['plan'] or ['team_name' => 'team.name'] for a key and a path.
 */
class UserIdentity
{
    /**
     * @param array<int|string, string> $attributes
     */
    public function __construct(private readonly array $attributes = [])
    {
    }

    /**
     * @return array<string, mixed>
     */
    public function __invoke(mixed $user): array
    {
        $id = method_exists($user, 'getAuthIdentifier') ? $user->getAuthIdentifier() : data_get($user, 'id');
        $identity = [
            'id' => $id,
            'email' => data_get($user, 'email'),
            'emailVerified' => data_get($user, 'email_verified_at') !== null,
            'name' => data_get($user, 'name'),
        ];
        $created = data_get($user, 'created_at');
        if ($created !== null) {
            $identity['signedUpAt'] = $created instanceof \DateTimeInterface ? $created : (string) $created;
        }

        $attributes = [];
        foreach ($this->attributes as $key => $path) {
            $key = is_int($key) ? Str::snake(str_replace('.', '_', $path)) : $key;
            $value = data_get($user, $path);
            if ($value instanceof \DateTimeInterface) {
                $value = $value->getTimestamp();
            } elseif ($value instanceof \BackedEnum) {
                $value = $value->value;
            }
            $attributes[$key] = $value;
        }
        if ($attributes !== []) {
            $identity['attributes'] = $attributes;
        }

        return Arr::where($identity, fn ($value) => $value !== null);
    }
}
