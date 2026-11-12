<?php

declare(strict_types=1);

namespace Dimmi\Tests;

use Dimmi\Identity;
use Dimmi\IdentityException;
use Firebase\JWT\JWT;
use Firebase\JWT\Key;
use PHPUnit\Framework\TestCase;

final class IdentityTest extends TestCase
{
    private const SECRET = 'dmi_idsec_0123456789abcdefghijklmnopqrstuvwxyzABCDEF';

    private static function parts(string $token): array
    {
        [$header, $payload] = explode('.', $token);

        return [
            json_decode(JWT::urlsafeB64Decode($header), true),
            json_decode(JWT::urlsafeB64Decode($payload), true),
        ];
    }

    public function testHs256RoundTripWithTheWholeSecretAsKey(): void
    {
        $token = Identity::sign([
            'id' => 42,
            'email' => 'ada@example.test',
            'emailVerified' => true,
            'name' => 'Ada',
            'phone' => '',
            'phoneVerified' => true,
            'avatarUrl' => null,
            'locale' => 'en',
            'timezone' => 'Europe/Berlin',
            'signedUpAt' => 1700000000000,
            'company' => ['id' => 7, 'name' => 'Acme', 'domain' => '', 'attributes' => ['seats' => 10]],
            'attributes' => ['plan' => 'pro', 'trial' => false, 'note' => null],
        ], secret: self::SECRET, kid: 'isk_1', audience: 'wgt_1', nonce: 'n-1');

        $claims = (array) JWT::decode($token, new Key(self::SECRET, 'HS256'));
        [$header] = self::parts($token);

        $this->assertSame(['typ' => 'JWT', 'alg' => 'HS256', 'kid' => 'isk_1'], $header);
        $this->assertSame('42', $claims['sub']);
        $this->assertSame('ada@example.test', $claims['email']);
        $this->assertTrue($claims['email_verified']);
        $this->assertSame('Ada', $claims['name']);
        $this->assertArrayNotHasKey('phone', $claims);
        $this->assertArrayNotHasKey('phone_verified', $claims);
        $this->assertArrayNotHasKey('avatar_url', $claims);
        $this->assertSame('en', $claims['locale']);
        $this->assertSame('Europe/Berlin', $claims['timezone']);
        $this->assertSame(['plan' => 'pro', 'trial' => false, 'note' => null, 'signed_up_at' => 1700000000], (array) $claims['custom_attributes']);
        $this->assertSame(['id' => '7', 'name' => 'Acme', 'custom_attributes' => ['seats' => 10]], json_decode(json_encode($claims['company']), true));
        $this->assertSame('wgt_1', $claims['aud']);
        $this->assertSame('n-1', $claims['nonce']);
        $this->assertSame(600, $claims['exp'] - $claims['iat']);
    }

    public function testEmailIsUnverifiedUnlessSaidAndKidIsOptional(): void
    {
        $token = Identity::sign(['id' => 'u1', 'email' => 'a@b.test', 'emailVerified' => 'yes'], secret: self::SECRET, expiresIn: 3600);
        [$header, $claims] = self::parts($token);
        $this->assertArrayNotHasKey('kid', $header);
        $this->assertArrayNotHasKey('email_verified', $claims);
        $this->assertSame(3600, $claims['exp'] - $claims['iat']);
        $this->assertSame(['sub' => 'u1', 'email' => 'a@b.test'], array_diff_key($claims, ['iat' => 1, 'exp' => 1]));
    }

    public function testSignedUpAtAcceptsDatesStringsAndSeconds(): void
    {
        $this->assertSame(1700000000, Identity::claims(['id' => 1, 'signedUpAt' => new \DateTimeImmutable('@1700000000')])['custom_attributes']['signed_up_at']);
        $this->assertSame(1700000000, Identity::claims(['id' => 1, 'signedUpAt' => '2023-11-14T22:13:20Z'])['custom_attributes']['signed_up_at']);
        $this->assertSame(1700000000, Identity::claims(['id' => 1, 'signedUpAt' => 1700000000])['custom_attributes']['signed_up_at']);
    }

    public function testSecretFromTheEnvironment(): void
    {
        $_ENV['DIMMI_IDENTITY_SECRET'] = ' ' . self::SECRET . "\n";
        try {
            $claims = (array) JWT::decode(Identity::sign(['id' => 'u1']), new Key(self::SECRET, 'HS256'));
            $this->assertSame('u1', $claims['sub']);
        } finally {
            unset($_ENV['DIMMI_IDENTITY_SECRET']);
        }
    }

    public function testEs256RoundTripFromAPkcs8Key(): void
    {
        $key = openssl_pkey_new(['private_key_type' => OPENSSL_KEYTYPE_EC, 'curve_name' => 'prime256v1']);
        openssl_pkey_export($key, $pem);
        $public = openssl_pkey_get_details($key)['key'];

        $token = Identity::sign(['id' => 'u1', 'email' => 'a@b.test'], kid: 'isk_es', privateKey: $pem);

        [$header] = self::parts($token);
        $this->assertSame(['typ' => 'JWT', 'alg' => 'ES256', 'kid' => 'isk_es'], $header);
        $this->assertSame(64, strlen(JWT::urlsafeB64Decode(explode('.', $token)[2])));
        $claims = (array) JWT::decode($token, new Key($public, 'ES256'));
        $this->assertSame('u1', $claims['sub']);
    }

    public function testRs256RoundTrip(): void
    {
        $key = openssl_pkey_new(['private_key_type' => OPENSSL_KEYTYPE_RSA, 'private_key_bits' => 2048]);
        openssl_pkey_export($key, $pem);
        $token = Identity::sign(['id' => 'u1'], kid: 'isk_rs', privateKey: $pem);
        $claims = (array) JWT::decode($token, new Key(openssl_pkey_get_details($key)['key'], 'RS256'));
        $this->assertSame('u1', $claims['sub']);
    }

    public function testEdDsaRoundTripFromAPkcs8Key(): void
    {
        if (!function_exists('sodium_crypto_sign_seed_keypair')) {
            $this->markTestSkipped('sodium missing');
        }
        $seed = random_bytes(32);
        $pem = "-----BEGIN PRIVATE KEY-----\n" . base64_encode(hex2bin('302e020100300506032b657004220420') . $seed) . "\n-----END PRIVATE KEY-----\n";
        $public = sodium_crypto_sign_publickey(sodium_crypto_sign_seed_keypair($seed));

        $token = Identity::sign(['id' => 'u1'], kid: 'isk_ed', privateKey: $pem);

        $claims = (array) JWT::decode($token, new Key(base64_encode($public), 'EdDSA'));
        $this->assertSame('u1', $claims['sub']);
        $this->assertSame('EdDSA', self::parts($token)[0]['alg']);
    }

    /**
     * @return iterable<string, array{array<string, mixed>, array<string, mixed>, string}>
     */
    public static function failures(): iterable
    {
        yield 'no id' => [['email' => 'a@b.test'], [], 'identify_user_invalid'];
        yield 'blank id' => [['id' => '  '], [], 'identify_user_invalid'];
        yield 'long id' => [['id' => str_repeat('x', 256)], [], 'identify_user_invalid'];
        yield 'unknown field' => [['id' => 1, 'plan' => 'pro'], [], 'user_field_unknown'];
        yield 'too many attributes' => [['id' => 1, 'attributes' => array_fill_keys(array_map(fn ($i) => "a{$i}", range(1, 101)), 1)], [], 'attribute_limit'];
        yield 'bad key' => [['id' => 1, 'attributes' => ['planName' => 'x']], [], 'attribute_key_invalid'];
        yield 'nested value' => [['id' => 1, 'attributes' => ['plan' => ['x']]], [], 'attribute_value_invalid'];
        yield 'company without id' => [['id' => 1, 'company' => ['name' => 'Acme']], [], 'identify_user_invalid'];
        yield 'expires too long' => [['id' => 1], ['expiresIn' => 3601], 'expires_in_invalid'];
        yield 'expires zero' => [['id' => 1], ['expiresIn' => 0], 'expires_in_invalid'];
        yield 'no secret' => [['id' => 1], ['secret' => ''], 'secret_missing'];
        yield 'key without kid' => [['id' => 1], ['privateKey' => "-----BEGIN PRIVATE KEY-----\nAA==\n-----END PRIVATE KEY-----"], 'kid_required'];
        yield 'public key' => [['id' => 1], ['kid' => 'k', 'privateKey' => "-----BEGIN PUBLIC KEY-----\nAA==\n-----END PUBLIC KEY-----"], 'private_key_invalid'];
        yield 'garbage key' => [['id' => 1], ['kid' => 'k', 'privateKey' => "-----BEGIN PRIVATE KEY-----\nnot a key\n-----END PRIVATE KEY-----"], 'private_key_invalid'];
    }

    /**
     * @dataProvider failures
     */
    #[\PHPUnit\Framework\Attributes\DataProvider('failures')]
    public function testRefusesBadInput(array $user, array $options, string $code): void
    {
        unset($_ENV['DIMMI_IDENTITY_SECRET'], $_ENV['DIMMI_IDENTITY_KID'], $_ENV['DIMMI_IDENTITY_PRIVATE_KEY']);
        putenv('DIMMI_IDENTITY_SECRET');
        try {
            Identity::sign($user, ...($options + ['secret' => self::SECRET]));
            $this->fail('expected IdentityException ' . $code);
        } catch (IdentityException $e) {
            $this->assertSame($code, $e->errorCode);
            $this->assertStringStartsWith("[dimmi] {$code}: ", $e->getMessage());
        }
    }
}
