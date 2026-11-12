<?php

declare(strict_types=1);

namespace Dimmi;

use Firebase\JWT\JWT;

/**
 * Signs identity tokens for the dimmi messenger.
 *
 *     $token = Dimmi\Identity::sign([
 *         'id' => $user->id,
 *         'email' => $user->email,
 *         'emailVerified' => true,
 *         'name' => $user->name,
 *         'attributes' => ['plan' => 'pro'],
 *     ]);
 *
 * The shared secret comes from DIMMI_IDENTITY_SECRET (HS256, the whole
 * dmi_idsec_ string is the key), or a PKCS #8 PEM private key from
 * DIMMI_IDENTITY_PRIVATE_KEY with its key id in DIMMI_IDENTITY_KID
 * (ES256, RS256 or EdDSA, found from the key). Sign on your server only.
 */
final class Identity
{
    public const ALGORITHMS = ['HS256', 'ES256', 'RS256', 'EdDSA'];

    private const KNOWN = [
        'id', 'email', 'emailVerified', 'name', 'phone', 'phoneVerified', 'avatarUrl',
        'locale', 'timezone', 'signedUpAt', 'company', 'attributes',
    ];

    /**
     * Signs a token for the user.
     *
     * @param array{
     *     id: string|int,
     *     email?: ?string,
     *     emailVerified?: bool,
     *     name?: ?string,
     *     phone?: ?string,
     *     phoneVerified?: bool,
     *     avatarUrl?: ?string,
     *     locale?: ?string,
     *     timezone?: ?string,
     *     signedUpAt?: \DateTimeInterface|int|float|string|null,
     *     company?: ?array{id: string|int, name?: ?string, domain?: ?string, attributes?: array<string, scalar|null>},
     *     attributes?: array<string, scalar|null>,
     * } $user
     * @param int $expiresIn Seconds the token lives: 1 to 3600, 600 unless set.
     * @param string|null $audience "dimmi", a messenger app id or a workspace id.
     * @param string|null $nonce A one-time value, for tokens that must work only once.
     * @throws IdentityException
     */
    public static function sign(
        array $user,
        ?string $secret = null,
        ?string $kid = null,
        ?string $privateKey = null,
        ?string $algorithm = null,
        int $expiresIn = 600,
        ?string $audience = null,
        ?string $nonce = null,
    ): string {
        if ($expiresIn <= 0 || $expiresIn > 3600) {
            throw new IdentityException('expires_in_invalid', 'expiresIn is 1 to 3600 seconds.');
        }
        $now = time();
        $claims = self::claims($user) + ['iat' => $now, 'exp' => $now + $expiresIn];
        if ($audience !== null && $audience !== '') {
            $claims['aud'] = $audience;
        }
        if ($nonce !== null && $nonce !== '') {
            $claims['nonce'] = $nonce;
        }

        $kid ??= self::env('DIMMI_IDENTITY_KID');
        $pem = $privateKey ?? self::env('DIMMI_IDENTITY_PRIVATE_KEY');
        if ($pem !== null) {
            if ($kid === null) {
                throw new IdentityException('kid_required', 'set DIMMI_IDENTITY_KID to the key id dimmi shows for your public key.');
            }
            [$alg, $key] = self::privateKey($pem, $algorithm);

            return self::encode($claims, $key, $alg, $kid);
        }

        $secret ??= self::env('DIMMI_IDENTITY_SECRET');
        $secret = $secret === null ? '' : trim($secret);
        if ($secret === '') {
            throw new IdentityException('secret_missing', 'set DIMMI_IDENTITY_SECRET to the secret from Settings > Identity verification.');
        }

        return self::encode($claims, $secret, 'HS256', $kid);
    }

    /**
     * Turns a user into token claims, checking it the way dimmi will.
     *
     * @param array<string, mixed> $user
     * @return array<string, mixed>
     * @throws IdentityException
     */
    public static function claims(array $user): array
    {
        $id = $user['id'] ?? null;
        $sub = is_int($id) || is_float($id) ? (string) $id : $id;
        if (!is_string($sub) || trim($sub) === '' || strlen($sub) > 255) {
            throw new IdentityException('identify_user_invalid', 'the user needs an id: a string or number, 1 to 255 long.');
        }
        foreach (array_keys($user) as $field) {
            if (!in_array($field, self::KNOWN, true)) {
                throw new IdentityException('user_field_unknown', "put {$field} under attributes.");
            }
        }

        $claims = ['sub' => $sub];
        self::set($claims, 'email', $user['email'] ?? null);
        if (isset($claims['email']) && ($user['emailVerified'] ?? false) === true) {
            $claims['email_verified'] = true;
        }
        self::set($claims, 'name', $user['name'] ?? null);
        self::set($claims, 'phone', $user['phone'] ?? null);
        if (isset($claims['phone']) && ($user['phoneVerified'] ?? false) === true) {
            $claims['phone_verified'] = true;
        }
        self::set($claims, 'avatar_url', $user['avatarUrl'] ?? null);
        self::set($claims, 'locale', $user['locale'] ?? null);
        self::set($claims, 'timezone', $user['timezone'] ?? null);

        $attributes = self::attributes($user['attributes'] ?? []);
        $signedUp = self::epochSeconds($user['signedUpAt'] ?? null);
        if ($signedUp !== null) {
            $attributes['signed_up_at'] = $signedUp;
        }
        if ($attributes !== []) {
            $claims['custom_attributes'] = $attributes;
        }
        if (!empty($user['company'])) {
            $claims['company'] = self::company($user['company']);
        }

        return $claims;
    }

    /**
     * @param array<string, mixed> $claims
     */
    private static function set(array &$claims, string $name, mixed $value): void
    {
        if (is_string($value) && $value !== '') {
            $claims[$name] = $value;
        }
    }

    /**
     * @return array<string, mixed>
     */
    private static function company(mixed $company): array
    {
        if (!is_array($company)) {
            throw new IdentityException('identify_user_invalid', 'company needs an id.');
        }
        $id = $company['id'] ?? null;
        $id = is_int($id) || is_float($id) ? (string) $id : $id;
        if (!is_string($id) || $id === '') {
            throw new IdentityException('identify_user_invalid', 'company needs an id.');
        }
        $out = ['id' => $id];
        self::set($out, 'name', $company['name'] ?? null);
        self::set($out, 'domain', $company['domain'] ?? null);
        if (!empty($company['attributes'])) {
            $out['custom_attributes'] = self::attributes($company['attributes']);
        }

        return $out;
    }

    /**
     * @return array<string, scalar|null>
     */
    private static function attributes(mixed $attributes): array
    {
        if (!is_array($attributes)) {
            throw new IdentityException('attribute_value_invalid', 'attributes must be an array of key => value.');
        }
        if (count($attributes) > 100) {
            throw new IdentityException('attribute_limit', 'send at most 100 attributes; update others through the API.');
        }
        foreach ($attributes as $key => $value) {
            $key = (string) $key;
            if (!preg_match('/^[a-z0-9_]{1,64}$/', $key)) {
                throw new IdentityException('attribute_key_invalid', "use snake_case keys such as plan_name, not {$key}.");
            }
            if ($value !== null && !is_scalar($value)) {
                throw new IdentityException('attribute_value_invalid', "{$key} must be a string, number, boolean or null.");
            }
        }

        return $attributes;
    }

    private static function epochSeconds(mixed $value): ?int
    {
        if ($value === null || $value === '') {
            return null;
        }
        if ($value instanceof \DateTimeInterface) {
            return $value->getTimestamp();
        }
        if (is_int($value) || is_float($value)) {
            return (int) floor($value > 1e11 ? $value / 1000 : $value);
        }
        if (is_string($value)) {
            if (is_numeric($value)) {
                return self::epochSeconds((float) $value);
            }
            $time = strtotime($value);

            return $time === false ? null : $time;
        }

        return null;
    }

    /**
     * Finds the algorithm of a PKCS #8 PEM private key and the key in the
     * form firebase/php-jwt signs with.
     *
     * @return array{0: string, 1: string}
     */
    private static function privateKey(string $pem, ?string $algorithm): array
    {
        if (preg_match('/BEGIN (RSA |EC )?PUBLIC KEY/', $pem)) {
            throw new IdentityException('private_key_invalid', 'that is a public key; sign with the private key.');
        }
        if ($algorithm !== null && !in_array($algorithm, self::ALGORITHMS, true)) {
            throw new IdentityException('private_key_invalid', 'algorithm is one of ES256, RS256 or EdDSA.');
        }
        $der = base64_decode((string) preg_replace(['/-----[^-]+-----/', '/\s+/'], '', $pem), true);
        if ($der === false || $der === '') {
            throw new IdentityException('private_key_invalid', 'the private key must be PKCS #8 PEM: Ed25519, P-256 or RSA.');
        }

        // Ed25519 PKCS #8: OID 1.3.101.112, the 32-byte seed at the end.
        if (str_contains($der, "\x06\x03\x2b\x65\x70") && strlen($der) >= 48) {
            if ($algorithm !== null && $algorithm !== 'EdDSA') {
                throw new IdentityException('private_key_invalid', "the key is Ed25519, not {$algorithm}.");
            }
            if (!function_exists('sodium_crypto_sign_seed_keypair')) {
                throw new IdentityException('private_key_invalid', 'EdDSA needs the sodium extension.');
            }
            $seed = substr($der, -32);
            $secret = sodium_crypto_sign_secretkey(sodium_crypto_sign_seed_keypair($seed));

            return ['EdDSA', base64_encode($secret)];
        }

        $key = @openssl_pkey_get_private($pem);
        if ($key === false) {
            throw new IdentityException('private_key_invalid', 'the private key must be PKCS #8 PEM: Ed25519, P-256 or RSA.');
        }
        $details = openssl_pkey_get_details($key) ?: [];
        $found = match ($details['type'] ?? null) {
            OPENSSL_KEYTYPE_EC => ($details['ec']['curve_name'] ?? '') === 'prime256v1' ? 'ES256' : null,
            OPENSSL_KEYTYPE_RSA => 'RS256',
            default => null,
        };
        if ($found === null) {
            throw new IdentityException('private_key_invalid', 'the private key must be PKCS #8 PEM: Ed25519, P-256 or RSA.');
        }
        if ($algorithm !== null && $algorithm !== $found) {
            throw new IdentityException('private_key_invalid', "the key is for {$found}, not {$algorithm}.");
        }

        return [$found, $pem];
    }

    /**
     * @param array<string, mixed> $claims
     */
    private static function encode(array $claims, string $key, string $alg, ?string $kid): string
    {
        try {
            return JWT::encode($claims, $key, $alg, $kid === '' ? null : $kid);
        } catch (\DomainException|\InvalidArgumentException $e) {
            $code = $alg === 'HS256' ? 'secret_missing' : 'private_key_invalid';

            throw new IdentityException($code, 'the key could not sign: ' . $e->getMessage(), $e);
        }
    }

    private static function env(string $name): ?string
    {
        $value = $_ENV[$name] ?? $_SERVER[$name] ?? getenv($name);

        return is_string($value) && $value !== '' ? $value : null;
    }
}
