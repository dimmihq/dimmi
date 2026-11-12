<?php

declare(strict_types=1);

namespace Dimmi;

use Dimmi\Generated\Api;
use Dimmi\Generated\Operations;
use GuzzleHttp\Client as GuzzleClient;
use GuzzleHttp\Exception\ConnectException;
use GuzzleHttp\Exception\GuzzleException;
use GuzzleHttp\HandlerStack;
use GuzzleHttp\Middleware;
use Psr\Http\Message\RequestInterface;
use Psr\Http\Message\ResponseInterface;

/**
 * The dimmi REST API client.
 *
 *     $dimmi = new Dimmi\Client();  // DIMMI_TOKEN, DIMMI_WORKSPACE, DIMMI_BASE_URL
 *     $dimmi->contacts->store(['email' => 'ada@example.com']);
 *     $dimmi->conversations->messages->store($id, ['body' => 'Hi']);
 *     $dimmi->call('contacts.store', body: ['email' => 'ada@example.com']);
 *
 * Every operation in the OpenAPI document is reachable three ways: a
 * generated resource method, call() with its operation id, or request()
 * with a method and path. Answers are the decoded JSON as arrays.
 */
final class Client extends Api
{
    use HasResources;

    public const VERSION = '0.1.0';

    private const DEFAULTS = [
        'maxRetries' => 2,
        'timeout' => 30.0,
        'connectTimeout' => 10.0,
        'retryDelay' => 0.5,
        'maxRetryDelay' => 60.0,
        'headers' => [],
        'handler' => null,
    ];

    private readonly GuzzleClient $http;
    private readonly string $baseUrl;
    /** @var array<string, mixed> */
    private readonly array $options;

    /**
     * @param string|null $token API token; falls back to DIMMI_TOKEN.
     * @param string|null $workspace Workspace id (ws_...); falls back to DIMMI_WORKSPACE.
     * @param string|null $baseUrl Your dimmi host or API base, such as https://dimmi.acme.com;
     *                             "/api/v1" is added when missing. Falls back to DIMMI_BASE_URL, then DIMMI_HOST.
     * @param array{
     *     maxRetries?: int,
     *     timeout?: float,
     *     connectTimeout?: float,
     *     retryDelay?: float,
     *     maxRetryDelay?: float,
     *     headers?: array<string, string>,
     *     handler?: callable|HandlerStack|null,
     * } $options maxRetries retries 429, 503 and connection failures (default 2);
     *            retryDelay is the first backoff in seconds when no Retry-After is sent;
     *            handler swaps the Guzzle handler, for tests.
     */
    public function __construct(
        ?string $token = null,
        ?string $workspace = null,
        ?string $baseUrl = null,
        array $options = [],
    ) {
        $token ??= self::env('DIMMI_TOKEN');
        $workspace ??= self::env('DIMMI_WORKSPACE');
        $baseUrl ??= self::env('DIMMI_BASE_URL') ?? self::env('DIMMI_HOST');
        if ($token === null || $token === '') {
            throw new \InvalidArgumentException('dimmi: pass a token or set DIMMI_TOKEN.');
        }
        if ($baseUrl === null || $baseUrl === '') {
            throw new \InvalidArgumentException('dimmi: pass a baseUrl, such as https://dimmi.acme.com, or set DIMMI_BASE_URL.');
        }
        $this->baseUrl = self::apiBase($baseUrl);
        $this->options = $options + self::DEFAULTS;

        $handler = $this->options['handler'];
        $stack = $handler instanceof HandlerStack ? $handler : HandlerStack::create($handler);
        $stack->push(Middleware::retry($this->shouldRetry(...), $this->retryDelay(...)), 'dimmi_retry');

        $headers = [
            'Authorization' => 'Bearer ' . $token,
            'Accept' => 'application/json',
            'User-Agent' => sprintf('dimmi-php/%s PHP/%s', self::VERSION, PHP_VERSION),
        ];
        if ($workspace !== null && $workspace !== '') {
            $headers['Dimmi-Workspace'] = $workspace;
        }

        $this->http = new GuzzleClient([
            'handler' => $stack,
            'http_errors' => false,
            'timeout' => $this->options['timeout'],
            'connect_timeout' => $this->options['connectTimeout'],
            'headers' => $this->options['headers'] + $headers,
        ]);
    }

    /**
     * Runs an operation by its OpenAPI operation id, such as "contacts.store".
     *
     * @param array<string|int, string|int> $path Path parameters by name, or in order.
     * @param array<string, mixed> $query
     * @param array<string, mixed>|object|null $body
     * @param array{idempotencyKey?: string, headers?: array<string, string>, timeout?: float} $options
     * @return array<string, mixed>|string|null
     */
    public function call(
        string $operation,
        array $path = [],
        array $query = [],
        array|object|null $body = null,
        array $options = [],
    ): array|string|null {
        $op = self::operation($operation);

        return $this->request($op['method'], self::expand($op, $path), $query, $body, $options);
    }

    /**
     * Sends a request to a path under the API base, such as "/contacts".
     *
     * @param array<string, mixed> $query
     * @param array<string, mixed>|object|null $body
     * @param array{idempotencyKey?: string, headers?: array<string, string>, timeout?: float} $options
     * @return array<string, mixed>|string|null The decoded JSON, the raw text for other content, null when empty.
     * @throws DimmiException
     */
    public function request(
        string $method,
        string $path,
        array $query = [],
        array|object|null $body = null,
        array $options = [],
    ): array|string|null {
        $method = strtoupper($method);
        $headers = $options['headers'] ?? [];
        $request = [];
        if ($query !== []) {
            $request['query'] = self::query($query);
        }
        if ($body !== null) {
            $headers['Content-Type'] = 'application/json';
            $request['body'] = self::json($body);
        }
        if ($method === 'POST') {
            $key = $options['idempotencyKey'] ?? null;
            if ($key === null && $this->options['maxRetries'] > 0) {
                $key = self::uuid();
            }
            if ($key !== null) {
                $headers['Idempotency-Key'] = $key;
            }
        }
        if (isset($options['timeout'])) {
            $request['timeout'] = $options['timeout'];
        }
        $request['headers'] = $headers;

        try {
            $response = $this->http->request($method, $this->baseUrl . '/' . ltrim($path, '/'), $request);
        } catch (GuzzleException $e) {
            throw new DimmiException('dimmi: request failed: ' . $e->getMessage(), 0, 'connection_failed', previous: $e);
        }

        if ($response->getStatusCode() >= 400) {
            throw DimmiException::fromResponse($response);
        }

        return self::decode($response);
    }

    /**
     * Walks a cursor-paginated list, yielding each item and fetching the
     * next page from meta.next_cursor as you go.
     *
     *     foreach ($dimmi->paginate('/conversations', ['limit' => 50]) as $conversation) { ... }
     *
     * @param string $path A path such as "/conversations", or an operation id such as "conversations.index".
     * @param array<string, mixed> $query
     * @param array<string|int, string|int> $params Path parameters when $path is an operation id.
     * @return \Generator<int, array<string, mixed>>
     */
    public function paginate(string $path, array $query = [], array $params = []): \Generator
    {
        $path = $this->resolvePath($path, $params);
        $query['limit'] ??= 100;
        while (true) {
            $page = $this->request('GET', $path, $query);
            $data = is_array($page) ? ($page['data'] ?? []) : [];
            foreach ($data as $item) {
                yield $item;
            }
            $next = is_array($page) ? ($page['meta']['next_cursor'] ?? null) : null;
            if ($data === [] || !is_string($next) || $next === '') {
                return;
            }
            $query['cursor'] = $next;
        }
    }

    /**
     * Walks a page-numbered list (?page=1, 2, ...), yielding each page's
     * items as a list and stopping at an empty or short page.
     *
     * @param array<string, mixed> $query
     * @param array<string|int, string|int> $params
     * @return \Generator<int, list<array<string, mixed>>>
     */
    public function pages(string $path, array $query = [], array $params = []): \Generator
    {
        $path = $this->resolvePath($path, $params);
        $limit = (int) ($query['limit'] ??= 100);
        $page = (int) ($query['page'] ?? 1);
        while (true) {
            $query['page'] = $page;
            $answer = $this->request('GET', $path, $query);
            $data = is_array($answer) ? array_values($answer['data'] ?? []) : [];
            if ($data === []) {
                return;
            }
            yield $data;
            if (count($data) < $limit) {
                return;
            }
            $page++;
        }
    }

    public function baseUrl(): string
    {
        return $this->baseUrl;
    }

    protected function dimmiClient(): Client
    {
        return $this;
    }

    /**
     * @return array{method: string, path: string, params: list<string>, query: list<string>, body: ?string, bodyRequired: bool, response: ?string}
     */
    public static function operation(string $id): array
    {
        return Operations::ALL[$id] ?? throw new \InvalidArgumentException("dimmi: unknown operation {$id}.");
    }

    private function resolvePath(string $path, array $params): string
    {
        if (!str_contains($path, '/') && isset(Operations::ALL[$path])) {
            return self::expand(Operations::ALL[$path], $params);
        }

        return $path;
    }

    /**
     * @param array{path: string, params: list<string>} $op
     * @param array<string|int, string|int> $values
     */
    private static function expand(array $op, array $values): string
    {
        $path = $op['path'];
        foreach ($op['params'] as $i => $name) {
            $value = $values[$name] ?? $values[$i] ?? null;
            if ($value === null || $value === '') {
                throw new \InvalidArgumentException("dimmi: {$op['path']} needs the {$name} path parameter.");
            }
            $path = str_replace('{' . $name . '}', rawurlencode((string) $value), $path);
        }

        return $path;
    }

    private function shouldRetry(int $retries, RequestInterface $request, ?ResponseInterface $response = null, ?\Throwable $error = null): bool
    {
        if ($retries >= $this->options['maxRetries']) {
            return false;
        }
        if ($error instanceof ConnectException) {
            return true;
        }

        return $response !== null && in_array($response->getStatusCode(), [429, 503], true);
    }

    /**
     * Milliseconds to wait before retry $retries: Retry-After when dimmi
     * sent one, else exponential backoff with jitter.
     */
    private function retryDelay(int $retries, ?ResponseInterface $response = null): int
    {
        $max = (float) $this->options['maxRetryDelay'];
        $after = DimmiException::retryAfter($response);
        if ($after !== null) {
            return (int) (min($after, $max) * 1000);
        }
        $base = (float) $this->options['retryDelay'] * (2 ** max(0, $retries - 1));
        $jitter = $base * (mt_rand(0, 250) / 1000);

        return (int) (min($base + $jitter, $max) * 1000);
    }

    /**
     * @return array<string, mixed>|string|null
     */
    private static function decode(ResponseInterface $response): array|string|null
    {
        $raw = (string) $response->getBody();
        if ($response->getStatusCode() === 204 || $raw === '') {
            return null;
        }
        if (str_contains($response->getHeaderLine('Content-Type'), 'json') || $raw[0] === '{' || $raw[0] === '[') {
            $data = json_decode($raw, true);
            if (json_last_error() === JSON_ERROR_NONE) {
                return $data;
            }
        }

        return $raw;
    }

    private static function json(array|object $body): string
    {
        if ($body === []) {
            return '{}';
        }

        return json_encode($body, JSON_THROW_ON_ERROR | JSON_UNESCAPED_SLASHES | JSON_UNESCAPED_UNICODE);
    }

    /**
     * @param array<string, mixed> $query
     * @return array<string, mixed>
     */
    private static function query(array $query): array
    {
        $out = [];
        foreach ($query as $key => $value) {
            if ($value === null) {
                continue;
            }
            $out[$key] = match (true) {
                is_bool($value) => $value ? 'true' : 'false',
                is_array($value) => self::query($value),
                default => $value,
            };
        }

        return $out;
    }

    private static function apiBase(string $url): string
    {
        $url = rtrim($url, '/');

        return preg_match('#/api/v\d+$#', $url) ? $url : $url . '/api/v1';
    }

    private static function env(string $name): ?string
    {
        $value = $_ENV[$name] ?? $_SERVER[$name] ?? getenv($name);

        return is_string($value) && $value !== '' ? $value : null;
    }

    public static function uuid(): string
    {
        $bytes = random_bytes(16);
        $bytes[6] = chr((ord($bytes[6]) & 0x0f) | 0x40);
        $bytes[8] = chr((ord($bytes[8]) & 0x3f) | 0x80);

        return vsprintf('%s%s-%s-%s-%s-%s%s%s', str_split(bin2hex($bytes), 4));
    }
}
