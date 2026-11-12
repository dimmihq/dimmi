<?php

declare(strict_types=1);

namespace Dimmi\Tests;

use PHPUnit\Framework\TestCase;

/**
 * bin/generate keeps public methods stable across spec changes.
 */
final class GeneratorTest extends TestCase
{
    private string $dir;

    protected function setUp(): void
    {
        $this->dir = sys_get_temp_dir() . '/dimmi-generate-' . bin2hex(random_bytes(4));
        mkdir($this->dir . '/bin', 0777, true);
        copy(dirname(__DIR__) . '/bin/generate', $this->dir . '/bin/generate');
    }

    protected function tearDown(): void
    {
        exec('rm -rf ' . escapeshellarg($this->dir));
    }

    private function generate(array $paths, bool $update = true, int $exit = 0): string
    {
        $spec = ['openapi' => '3.1.0', 'info' => ['title' => 't', 'version' => 'test'], 'paths' => $paths];
        file_put_contents($this->dir . '/spec.json', json_encode($spec));
        $flag = $update ? ' --update-surface' : '';
        exec(PHP_BINARY . ' ' . escapeshellarg($this->dir . '/bin/generate') . $flag . ' ' . escapeshellarg($this->dir . '/spec.json') . ' 2>&1', $out, $code);
        $this->assertSame($exit, $code, implode("\n", $out));

        return implode("\n", $out);
    }

    private function resource(string $class): string
    {
        return (string) file_get_contents("{$this->dir}/src/Generated/Resources/{$class}.php");
    }

    private static function op(string $id, bool $body = false): array
    {
        $op = ['operationId' => $id, 'responses' => ['200' => ['description' => 'OK']]];
        if ($body) {
            $op['requestBody'] = ['required' => true, 'content' => ['application/json' => ['schema' => ['$ref' => '#/components/schemas/WidgetBody']]]];
        }

        return $op;
    }

    public function testKeepsDroppedOperationsAndSignaturesStable(): void
    {
        $this->generate([
            '/widgets' => ['get' => self::op('widgets.index'), 'post' => self::op('widgets.store', true)],
            '/widgets/{widget}' => ['delete' => self::op('widgets.destroy')],
            '/widgets/{widget}/pings' => ['post' => self::op('widgets.pings.store')],
        ]);
        $this->assertStringContainsString('public function destroy(string|int $widget, array $query = [], array $options = [])', $this->resource('Widgets'));

        $log = $this->generate([
            '/widgets' => ['get' => self::op('widgets.index'), 'post' => self::op('widgets.store', true)],
            '/widgets/{thing}/pings' => ['post' => self::op('widgets.pings.store', true)],
        ]);

        $widgets = $this->resource('Widgets');
        $this->assertStringContainsString('@deprecated', $widgets);
        $this->assertStringContainsString('public function destroy(string|int $widget, array $query = [], array $options = [])', $widgets);
        $this->assertStringContainsString(
            'public function store(string|int $widget, array $query = [], array $options = [], array|object|null $body = null)',
            $this->resource('WidgetsPings'),
        );
        $this->assertStringContainsString("'path' => '/widgets/{widget}/pings'", (string) file_get_contents($this->dir . '/src/Generated/Operations.php'));
        $this->assertStringContainsString('widgets.destroy: dropped from the spec', $log);

        // A third run keeps everything as the second left it.
        $this->generate([
            '/widgets' => ['get' => self::op('widgets.index'), 'post' => self::op('widgets.store', true)],
            '/widgets/{thing}/pings' => ['post' => self::op('widgets.pings.store', true)],
        ]);
        $this->assertSame($widgets, $this->resource('Widgets'));
    }

    public function testFailsWhenTheCommittedSurfaceWouldChange(): void
    {
        $paths = [
            '/widgets' => ['get' => self::op('widgets.index')],
            '/widgets/{widget}' => ['delete' => self::op('widgets.destroy')],
        ];
        $this->generate($paths);
        $surface = (string) file_get_contents($this->dir . '/api-surface.json');
        $this->assertStringContainsString('"widgets.destroy": {"method":"DELETE","path":"/widgets/{widget}","params":["widget"],"body":"none"}', $surface);

        // Unchanged and additive runs pass without touching the file.
        $this->generate($paths, false);
        $log = $this->generate($paths + ['/gadgets' => ['get' => self::op('gadgets.index')]], false);
        $this->assertStringContainsString('gadgets.index', $log);

        // A dropped operation fails the run, and the file stays as committed.
        unset($paths['/widgets/{widget}']);
        $log = $this->generate($paths, false, 1);
        $this->assertStringContainsString('no longer matches api-surface.json', $log);
        $this->assertStringContainsString('widgets.destroy', $log);
        $this->assertSame($surface, file_get_contents($this->dir . '/api-surface.json'));
        $this->assertStringContainsString('@deprecated', $this->resource('Widgets'));

        // --update-surface records it as deprecated.
        $this->generate($paths);
        $this->assertStringContainsString('"body":"none","deprecated":true}', (string) file_get_contents($this->dir . '/api-surface.json'));
    }
}
