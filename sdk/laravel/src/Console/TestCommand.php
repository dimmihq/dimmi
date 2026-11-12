<?php

declare(strict_types=1);

namespace Dimmi\Laravel\Console;

use Dimmi\DimmiException;
use Dimmi\Laravel\DimmiManager;
use Illuminate\Console\Command;

/**
 * php artisan dimmi:test: checks the config, calls the API, signs an
 * identity token and has dimmi validate it. Exits 1 when anything fails.
 */
class TestCommand extends Command
{
    protected $signature = 'dimmi:test {--email=dimmi-test@example.com : Email of the test identity}';

    protected $description = 'Check the dimmi API token, identity verification and messenger config';

    private int $failures = 0;

    public function handle(DimmiManager $dimmi): int
    {
        $config = $dimmi->config();

        $this->step('config', function () use ($config) {
            $missing = array_keys(array_filter([
                'DIMMI_BASE_URL' => $config['base_url'] ?? null,
                'DIMMI_TOKEN' => $config['token'] ?? null,
                'DIMMI_APP_ID' => $config['app_id'] ?? null,
            ], fn ($v) => $v === null || $v === ''));
            if ($missing !== []) {
                throw new \RuntimeException('missing ' . implode(', ', $missing));
            }

            return 'base URL, token and app id set';
        });

        $this->step('API', function () use ($dimmi) {
            $answer = $dimmi->call('conversations.index', query: ['limit' => 1]);

            return 'GET /conversations answered' . (isset($answer['meta']['total']) ? " ({$answer['meta']['total']} conversations)" : '');
        });

        $this->step('identity token', function () use ($dimmi, $config) {
            $token = $dimmi->identityToken([
                'id' => 'dimmi-artisan-test',
                'email' => (string) $this->option('email'),
                'name' => 'dimmi:test',
            ]);
            $answer = $dimmi->call('identity-verification.validations.store', body: array_filter([
                'app_id' => $config['app_id'] ?? null,
                'token' => $token,
            ]));
            $result = $answer['data'] ?? [];
            if (!($result['valid'] ?? false)) {
                throw new \RuntimeException(trim(($result['code'] ?? 'invalid') . ': ' . ($result['message'] ?? '') . ' ' . ($result['hint'] ?? '')));
            }
            $alg = $result['header']['alg'] ?? '?';

            return "signed {$alg} token validated by dimmi";
        });

        $this->step('messenger', function () use ($dimmi) {
            return 'loader ' . $dimmi->host() . '/widget/v1/loader.js';
        });

        $secret = $config['webhook']['secret'] ?? null;
        if ($secret) {
            $this->step('webhook secret', function () use ($secret) {
                new \Dimmi\Webhook((string) $secret);

                return 'set';
            });
        } else {
            $this->line('  <comment>SKIP</comment> webhook secret: DIMMI_WEBHOOK_SECRET is not set');
        }

        $this->newLine();
        if ($this->failures > 0) {
            $this->error("dimmi: {$this->failures} check(s) failed");

            return self::FAILURE;
        }
        $this->info('dimmi: all checks passed');

        return self::SUCCESS;
    }

    private function step(string $name, callable $check): void
    {
        try {
            $this->line("  <info>PASS</info> {$name}: " . $check());
        } catch (DimmiException $e) {
            $this->failures++;
            $this->line("  <error>FAIL</error> {$name}: HTTP {$e->status} {$e->errorCode}: {$e->getMessage()}");
        } catch (\Throwable $e) {
            $this->failures++;
            $this->line("  <error>FAIL</error> {$name}: {$e->getMessage()}");
        }
    }
}
