<?php

declare(strict_types=1);

namespace Dimmi\Laravel\Console;

use Illuminate\Console\Command;

/**
 * php artisan dimmi:install: publishes config/dimmi.php and adds the dimmi
 * keys to .env and .env.example, leaving keys already there alone.
 */
class InstallCommand extends Command
{
    protected $signature = 'dimmi:install {--force : Overwrite config/dimmi.php}';

    protected $description = 'Publish the dimmi config and add its keys to .env';

    public const KEYS = [
        'DIMMI_BASE_URL' => '',
        'DIMMI_TOKEN' => '',
        'DIMMI_WORKSPACE' => '',
        'DIMMI_APP_ID' => '',
        'DIMMI_IDENTITY_SECRET' => '',
        'DIMMI_IDENTITY_KID' => '',
        'DIMMI_WEBHOOK_SECRET' => '',
    ];

    public function handle(): int
    {
        $this->call('vendor:publish', ['--tag' => 'dimmi-config', '--force' => (bool) $this->option('force')]);

        foreach (['.env', '.env.example'] as $file) {
            $path = $this->laravel->basePath($file);
            if (!is_file($path)) {
                continue;
            }
            $added = $this->appendKeys($path);
            $this->line($added === [] ? "{$file}: dimmi keys already present" : "{$file}: added " . implode(', ', $added));
        }

        $this->newLine();
        $this->line('Next: fill the keys in .env, add @dimmiMessenger before </body>, then run php artisan dimmi:test.');

        return self::SUCCESS;
    }

    /**
     * @return list<string>
     */
    private function appendKeys(string $path): array
    {
        $contents = (string) file_get_contents($path);
        $missing = [];
        foreach (self::KEYS as $key => $value) {
            if (!preg_match('/^' . preg_quote($key, '/') . '=/m', $contents)) {
                $missing[] = $key;
            }
        }
        if ($missing === []) {
            return [];
        }
        $block = ($contents === '' || str_ends_with($contents, "\n") ? '' : "\n") . "\n";
        foreach ($missing as $key) {
            $block .= $key . '=' . self::KEYS[$key] . "\n";
        }
        file_put_contents($path, $contents . $block);

        return $missing;
    }
}
