<?php

declare(strict_types=1);

namespace Dimmi\Laravel\Tests;

use Dimmi\Laravel\Facades\Dimmi;
use Firebase\JWT\JWT;
use Firebase\JWT\Key;
use Illuminate\Support\Facades\Blade;

class IdentityTest extends TestCase
{
    private function user(array $attributes = []): User
    {
        $user = new User($attributes + [
            'name' => 'Ada Lovelace',
            'email' => 'ada@example.test',
            'email_verified_at' => '2024-01-01 00:00:00',
            'created_at' => '2023-11-14 22:13:20',
            'plan' => 'pro',
        ]);
        $user->id = 7;

        return $user;
    }

    private static function claims(string $token): array
    {
        return json_decode(json_encode(JWT::decode($token, new Key(self::SECRET, 'HS256'))), true);
    }

    public function testSignsTheDefaultMappingForAUser(): void
    {
        config(['dimmi.user.attributes' => ['plan', 'display' => 'name']]);
        $claims = self::claims(Dimmi::identityToken($this->user()));

        $this->assertSame('7', $claims['sub']);
        $this->assertSame('ada@example.test', $claims['email']);
        $this->assertTrue($claims['email_verified']);
        $this->assertSame('Ada Lovelace', $claims['name']);
        $this->assertSame(['plan' => 'pro', 'display' => 'Ada Lovelace', 'signed_up_at' => 1700000000], $claims['custom_attributes']);
    }

    public function testUnverifiedEmailAndGuests(): void
    {
        $claims = self::claims(Dimmi::identityToken($this->user(['email_verified_at' => null])));
        $this->assertArrayNotHasKey('email_verified', $claims);
        $this->assertNull(Dimmi::identityToken(null));
    }

    public function testUsesAResolverClassOrCallable(): void
    {
        config(['dimmi.user.resolver' => PlanResolver::class]);
        $this->refreshManager();
        $claims = self::claims(Dimmi::identityToken($this->user()));
        $this->assertSame('custom-7', $claims['sub']);
        $this->assertSame(['plan' => 'enterprise'], $claims['custom_attributes']);

        config(['dimmi.user.resolver' => fn ($user) => ['id' => 'fn-' . $user->id]]);
        $this->refreshManager();
        $this->assertSame('fn-7', self::claims(Dimmi::identityToken($this->user()))['sub']);
    }

    public function testSignsEs256FromAPrivateKeyFile(): void
    {
        $key = openssl_pkey_new(['private_key_type' => OPENSSL_KEYTYPE_EC, 'curve_name' => 'prime256v1']);
        openssl_pkey_export($key, $pem);
        $file = tempnam(sys_get_temp_dir(), 'dimmi') . '.pem';
        file_put_contents($file, $pem);
        config(['dimmi.identity.private_key' => $file, 'dimmi.identity.kid' => 'isk_es']);
        $this->refreshManager();

        $token = Dimmi::identityToken($this->user());
        $claims = (array) JWT::decode($token, new Key(openssl_pkey_get_details($key)['key'], 'ES256'));
        $this->assertSame('7', $claims['sub']);
        unlink($file);
    }

    public function testBladeDirectiveIdentifiesTheSignedInUser(): void
    {
        $this->actingAs($this->user());
        $html = Blade::render('<body>@dimmiMessenger</body>');

        $this->assertStringContainsString('<script async src="https://dimmi.test/widget/v1/loader.js" data-app-id="wgt_test"></script>', $html);
        preg_match('/return "([^"]+)"/', $html, $m);
        $this->assertSame('ada@example.test', self::claims($m[1])['email']);
    }

    public function testBladeDirectiveIdentifiesNobodyForGuests(): void
    {
        $html = Blade::render('@dimmiMessenger');
        $this->assertStringNotContainsString('identify', $html);
        $this->assertStringContainsString('data-app-id="wgt_test"', $html);
    }

    public function testBladeDirectiveCanUseTheTokenUrlAndANonce(): void
    {
        $html = Blade::render("@dimmiMessenger(['identity' => 'url', 'nonce' => 'abc'])");
        $this->assertStringContainsString('data-identity-url="/dimmi/token"', $html);
        $this->assertStringContainsString('nonce="abc"', $html);
        $this->assertStringNotContainsString('identify', $html);
    }

    public function testNoAppIdRendersAComment(): void
    {
        config(['dimmi.app_id' => null]);
        $this->refreshManager();
        $this->assertStringContainsString('<!-- dimmi: set DIMMI_APP_ID', Blade::render('@dimmiMessenger'));
    }

    public function testTokenRouteAnswersNoStoreJson(): void
    {
        $guest = $this->get('/dimmi/token');
        $guest->assertOk()->assertExactJson(['token' => null]);
        $this->assertStringContainsString('no-store', $guest->headers->get('Cache-Control'));
        $this->assertStringContainsString('private', $guest->headers->get('Cache-Control'));

        $signed = $this->actingAs($this->user())->get('/dimmi/token');
        $signed->assertOk();
        $this->assertSame('application/json', $signed->headers->get('Content-Type'));
        $this->assertSame('7', self::claims($signed->json('token'))['sub']);
    }

    private function refreshManager(): void
    {
        $this->app->forgetInstance(\Dimmi\Laravel\DimmiManager::class);
        Dimmi::clearResolvedInstances();
    }
}
