<?php

declare(strict_types=1);

namespace Dimmi\Laravel\Tests;

use Dimmi\Laravel\DimmiServiceProvider;
use Dimmi\Laravel\Facades\Dimmi;
use Orchestra\Testbench\TestCase as Orchestra;

abstract class TestCase extends Orchestra
{
    public const SECRET = 'dmi_idsec_0123456789abcdefghijklmnopqrstuvwxyzABCDEF';
    public const WEBHOOK_SECRET = 'whsec_MfKQ9r8GKYqrTwjUPD8ILPZIo2LaLaSw';

    protected function getPackageProviders($app): array
    {
        return [DimmiServiceProvider::class];
    }

    protected function getPackageAliases($app): array
    {
        return ['Dimmi' => Dimmi::class];
    }

    protected function defineEnvironment($app): void
    {
        $app['config']->set('app.key', 'base64:' . base64_encode(str_repeat('k', 32)));
        $app['config']->set('dimmi.token', 'tok_test');
        $app['config']->set('dimmi.workspace', 'ws_test');
        $app['config']->set('dimmi.base_url', 'https://dimmi.test/api/v1');
        $app['config']->set('dimmi.app_id', 'wgt_test');
        $app['config']->set('dimmi.identity.secret', self::SECRET);
        $app['config']->set('dimmi.webhook.secret', self::WEBHOOK_SECRET);
    }
}
