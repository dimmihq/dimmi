<?php

declare(strict_types=1);

namespace Dimmi\Laravel\Tests;

use Dimmi\Laravel\DimmiManager;
use Dimmi\Laravel\Facades\Dimmi;
use GuzzleHttp\Handler\MockHandler;
use GuzzleHttp\Psr7\Response;

class CommandsTest extends TestCase
{
    private array $requests = [];

    private function mockApi(array $responses): void
    {
        $mock = new MockHandler($responses);
        config(['dimmi.client' => ['maxRetries' => 0, 'handler' => function ($request, $options) use ($mock) {
            $this->requests[] = $request;

            return $mock($request, $options);
        }]]);
        $this->app->forgetInstance(DimmiManager::class);
        Dimmi::clearResolvedInstances();
    }

    public function testDimmiTestPassesWhenTheApiAndTokenCheckOut(): void
    {
        $this->mockApi([
            new Response(200, ['Content-Type' => 'application/json'], '{"data":[],"meta":{"next_cursor":null}}'),
            new Response(200, ['Content-Type' => 'application/json'], '{"data":{"valid":true,"header":{"alg":"HS256"}}}'),
        ]);

        $this->artisan('dimmi:test')
            ->expectsOutputToContain('PASS API')
            ->expectsOutputToContain('signed HS256 token validated by dimmi')
            ->assertExitCode(0);

        $this->assertSame('/api/v1/identity-verification/validations', $this->requests[1]->getUri()->getPath());
        $body = json_decode((string) $this->requests[1]->getBody(), true);
        $this->assertSame('wgt_test', $body['app_id']);
        $this->assertCount(3, explode('.', $body['token']));
    }

    public function testDimmiTestFailsOnABadTokenOrInvalidIdentity(): void
    {
        $this->mockApi([
            new Response(401, ['Content-Type' => 'application/json'], '{"code":"unauthenticated","message":"Bad token"}'),
            new Response(200, ['Content-Type' => 'application/json'], '{"data":{"valid":false,"code":"signature_invalid","message":"The signature does not match.","hint":"Check the secret."}}'),
        ]);

        $this->artisan('dimmi:test')
            ->expectsOutputToContain('FAIL API: HTTP 401 unauthenticated')
            ->expectsOutputToContain('FAIL identity token: signature_invalid')
            ->assertExitCode(1);
    }

    public function testDimmiTestFailsWithoutConfig(): void
    {
        config(['dimmi.token' => null]);
        $this->app->forgetInstance(DimmiManager::class);
        Dimmi::clearResolvedInstances();
        $this->artisan('dimmi:test')->expectsOutputToContain('missing DIMMI_TOKEN')->assertExitCode(1);
    }

    public function testInstallPublishesConfigAndAppendsEnvKeys(): void
    {
        $env = $this->app->basePath('.env');
        $had = is_file($env) ? file_get_contents($env) : null;
        file_put_contents($env, "APP_NAME=Test\nDIMMI_TOKEN=keep\n");
        @unlink($this->app->configPath('dimmi.php'));
        try {
            $this->artisan('dimmi:install')->assertExitCode(0);
            $contents = file_get_contents($env);
            $this->assertStringContainsString("DIMMI_TOKEN=keep\n", $contents);
            $this->assertSame(1, substr_count($contents, 'DIMMI_TOKEN='));
            $this->assertStringContainsString("DIMMI_APP_ID=\n", $contents);
            $this->assertStringContainsString("DIMMI_WEBHOOK_SECRET=\n", $contents);
            $this->assertFileExists($this->app->configPath('dimmi.php'));

            $this->artisan('dimmi:install')->expectsOutputToContain('already present')->assertExitCode(0);
        } finally {
            $had === null ? @unlink($env) : file_put_contents($env, $had);
            @unlink($this->app->configPath('dimmi.php'));
        }
    }

    public function testFacadeProxiesTheClient(): void
    {
        $this->mockApi([new Response(201, ['Content-Type' => 'application/json'], '{"data":{"id":"con_1"}}'), new Response(200, [], '{"data":[]}')]);
        $this->assertSame('con_1', Dimmi::contacts()->store(['email' => 'a@b.test'])['data']['id']);
        $this->assertSame(['data' => []], Dimmi::call('tags.index'));
        $this->assertSame('Bearer tok_test', $this->requests[0]->getHeaderLine('Authorization'));
        $this->assertSame('ws_test', $this->requests[0]->getHeaderLine('Dimmi-Workspace'));
        $this->assertInstanceOf(\Dimmi\Client::class, app(\Dimmi\Client::class));
    }
}
