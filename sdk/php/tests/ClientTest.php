<?php

declare(strict_types=1);

namespace Dimmi\Tests;

use Dimmi\AuthenticationException;
use Dimmi\Client;
use Dimmi\DimmiException;
use Dimmi\Generated\Resources\ConversationsMessages;
use Dimmi\NotFoundException;
use Dimmi\PermissionDeniedException;
use Dimmi\RateLimitException;
use Dimmi\ServerException;
use Dimmi\ValidationException;
use GuzzleHttp\Exception\ConnectException;
use GuzzleHttp\Handler\MockHandler;
use GuzzleHttp\Psr7\Request;
use GuzzleHttp\Psr7\Response;
use PHPUnit\Framework\TestCase;
use Psr\Http\Message\RequestInterface;

final class ClientTest extends TestCase
{
    /** @var list<array{request: RequestInterface}> */
    private array $history = [];

    private function client(array $responses, array $options = []): Client
    {
        return new Client('tok_test', 'ws_test', 'https://dimmi.test', $options + ['handler' => $this->recorder($responses), 'retryDelay' => 0]);
    }

    /**
     * A mock handler that records every request it answers, retries included.
     */
    private function recorder(array $responses): callable
    {
        $this->history = [];
        $mock = new MockHandler($responses);

        return function (RequestInterface $request, array $options) use ($mock) {
            $this->history[] = ['request' => $request, 'options' => $options];
            unset($options['delay']);

            return $mock($request, $options);
        };
    }

    private static function json(int $status, array $body, array $headers = []): Response
    {
        return new Response($status, ['Content-Type' => 'application/json'] + $headers, json_encode($body));
    }

    private function request(int $i): RequestInterface
    {
        return $this->history[$i]['request'];
    }

    public function testSendsAuthAndWorkspaceHeadersToTheApiBase(): void
    {
        $dimmi = $this->client([self::json(200, ['data' => []])]);
        $dimmi->conversations->index(['limit' => 5, 'unread' => true, 'skip' => null]);

        $request = $this->request(0);
        $this->assertSame('GET', $request->getMethod());
        $this->assertSame('https://dimmi.test/api/v1/conversations?limit=5&unread=true', (string) $request->getUri());
        $this->assertSame('Bearer tok_test', $request->getHeaderLine('Authorization'));
        $this->assertSame('ws_test', $request->getHeaderLine('Dimmi-Workspace'));
        $this->assertStringStartsWith('dimmi-php/', $request->getHeaderLine('User-Agent'));
    }

    public function testKeepsAnExplicitApiBase(): void
    {
        $dimmi = new Client('t', null, 'http://localhost:8098/api/v1/', ['handler' => new MockHandler([])]);
        $this->assertSame('http://localhost:8098/api/v1', $dimmi->baseUrl());
    }

    public function testReadsCredentialsFromTheEnvironment(): void
    {
        $_ENV['DIMMI_TOKEN'] = 'env_token';
        $_ENV['DIMMI_WORKSPACE'] = 'ws_env';
        $_ENV['DIMMI_BASE_URL'] = 'https://env.dimmi.test';
        try {
            (new Client(options: ['handler' => $this->recorder([self::json(200, ['data' => []])])]))->request('GET', '/me');
            $this->assertSame('Bearer env_token', $this->request(0)->getHeaderLine('Authorization'));
            $this->assertSame('ws_env', $this->request(0)->getHeaderLine('Dimmi-Workspace'));
            $this->assertSame('https://env.dimmi.test/api/v1/me', (string) $this->request(0)->getUri());
        } finally {
            unset($_ENV['DIMMI_TOKEN'], $_ENV['DIMMI_WORKSPACE'], $_ENV['DIMMI_BASE_URL']);
        }
    }

    public function testNeedsAToken(): void
    {
        $this->expectException(\InvalidArgumentException::class);
        new Client('', null, 'https://dimmi.test');
    }

    public function testGeneratedResourcesExpandPathParametersAndSendJson(): void
    {
        $dimmi = $this->client([self::json(201, ['data' => ['id' => 'msg_1']])]);
        $this->assertInstanceOf(ConversationsMessages::class, $dimmi->conversations->messages);

        $answer = $dimmi->conversations->messages->store('cnv 1', ['body' => 'Hi']);

        $this->assertSame(['data' => ['id' => 'msg_1']], $answer);
        $request = $this->request(0);
        $this->assertSame('POST', $request->getMethod());
        $this->assertSame('/api/v1/conversations/cnv%201/messages', $request->getUri()->getPath());
        $this->assertSame('application/json', $request->getHeaderLine('Content-Type'));
        $this->assertSame(['body' => 'Hi'], json_decode((string) $request->getBody(), true));
    }

    public function testCallRunsAnOperationById(): void
    {
        $dimmi = $this->client([self::json(201, ['data' => ['id' => 'con_1']]), self::json(200, ['data' => ['id' => 'con_1']])]);
        $dimmi->call('contacts.store', path: [], query: ['upsert' => 'email'], body: ['email' => 'ada@example.test']);
        $dimmi->call('contacts.show', path: ['con_1']);

        $this->assertSame('/api/v1/contacts?upsert=email', $this->request(0)->getUri()->getPath() . '?' . $this->request(0)->getUri()->getQuery());
        $this->assertSame('/api/v1/contacts/con_1', $this->request(1)->getUri()->getPath());
    }

    public function testUnknownOperationsAndMissingPathParametersFail(): void
    {
        $dimmi = $this->client([]);
        try {
            $dimmi->call('nope.index');
            $this->fail('expected an error');
        } catch (\InvalidArgumentException $e) {
            $this->assertStringContainsString('unknown operation', $e->getMessage());
        }
        $this->expectException(\InvalidArgumentException::class);
        $dimmi->call('contacts.show');
    }

    public function testEmptyBodyIsAnObject(): void
    {
        $dimmi = $this->client([self::json(201, ['data' => []])]);
        $dimmi->call('webhook-endpoints.tests.store', ['wh_1'], body: []);
        $this->assertSame('{}', (string) $this->request(0)->getBody());
    }

    public function testRetriesRateLimitsHonouringRetryAfterWithOneIdempotencyKey(): void
    {
        $dimmi = $this->client([
            self::json(429, ['code' => 'rate_limited', 'message' => 'Slow down'], ['Retry-After' => '0']),
            self::json(503, ['code' => 'unavailable', 'message' => 'Busy'], ['Retry-After' => '0']),
            self::json(201, ['data' => ['id' => 'con_1']]),
        ], ['maxRetries' => 2]);

        $answer = $dimmi->contacts->store(['email' => 'ada@example.test']);

        $this->assertSame('con_1', $answer['data']['id']);
        $this->assertCount(3, $this->history);
        $keys = array_map(fn ($h) => $h['request']->getHeaderLine('Idempotency-Key'), $this->history);
        $this->assertMatchesRegularExpression('/^[0-9a-f]{8}-[0-9a-f]{4}-4[0-9a-f]{3}-[89ab][0-9a-f]{3}-[0-9a-f]{12}$/', $keys[0]);
        $this->assertSame([$keys[0], $keys[0], $keys[0]], $keys);
    }

    public function testRetryAfterBecomesTheDelay(): void
    {
        $seen = [];
        $mock = new MockHandler([
            self::json(429, ['code' => 'rate_limited', 'message' => 'Slow down'], ['Retry-After' => '2']),
            self::json(200, ['data' => []]),
        ]);
        $spy = function (RequestInterface $request, array $options) use ($mock, &$seen) {
            $seen[] = $options['delay'] ?? 0;
            unset($options['delay']);

            return $mock($request, $options);
        };
        $dimmi = new Client('t', 'ws', 'https://dimmi.test', ['handler' => $spy, 'maxRetryDelay' => 30]);
        $dimmi->conversations->index();

        $this->assertSame([0, 2000], $seen);
    }

    public function testGivesUpAfterMaxRetriesWithARateLimitException(): void
    {
        $dimmi = $this->client([
            self::json(429, ['code' => 'rate_limited', 'message' => 'Slow down'], ['Retry-After' => '0']),
            self::json(429, ['code' => 'rate_limited', 'message' => 'Slow down'], ['Retry-After' => '7']),
        ], ['maxRetries' => 1, 'maxRetryDelay' => 0]);

        try {
            $dimmi->conversations->index();
            $this->fail('expected RateLimitException');
        } catch (RateLimitException $e) {
            $this->assertSame(429, $e->status);
            $this->assertSame('rate_limited', $e->errorCode);
            $this->assertSame(7, $e->retryAfter);
            $this->assertCount(2, $this->history);
        }
    }

    public function testRetriesConnectionFailures(): void
    {
        $dimmi = $this->client([
            new ConnectException('refused', new Request('GET', 'https://dimmi.test')),
            self::json(200, ['data' => []]),
        ]);
        $this->assertSame(['data' => []], $dimmi->conversations->index());
    }

    public function testDoesNotRetryOtherErrors(): void
    {
        $dimmi = $this->client([self::json(500, ['code' => 'internal', 'message' => 'Boom'])]);
        $this->expectException(ServerException::class);
        try {
            $dimmi->conversations->index();
        } finally {
            $this->assertCount(1, $this->history);
        }
    }

    public function testUsesTheGivenIdempotencyKeyAndNoneWithoutRetries(): void
    {
        $dimmi = $this->client([self::json(201, []), self::json(200, [])]);
        $dimmi->contacts->store(['email' => 'a@b.test'], options: ['idempotencyKey' => 'key-1']);
        $dimmi->contacts->index();
        $this->assertSame('key-1', $this->request(0)->getHeaderLine('Idempotency-Key'));
        $this->assertFalse($this->request(1)->hasHeader('Idempotency-Key'));

        $dimmi = $this->client([self::json(201, [])], ['maxRetries' => 0]);
        $dimmi->contacts->store(['email' => 'a@b.test']);
        $this->assertFalse($this->request(0)->hasHeader('Idempotency-Key'));
    }

    /**
     * @return iterable<string, array{int, class-string<DimmiException>}>
     */
    public static function errors(): iterable
    {
        yield '400' => [400, ValidationException::class];
        yield '401' => [401, AuthenticationException::class];
        yield '403' => [403, PermissionDeniedException::class];
        yield '404' => [404, NotFoundException::class];
        yield '409' => [409, DimmiException::class];
        yield '422' => [422, ValidationException::class];
        yield '500' => [500, ServerException::class];
    }

    /**
     * @dataProvider errors
     */
    #[\PHPUnit\Framework\Attributes\DataProvider('errors')]
    public function testMapsErrorsToTypedExceptions(int $status, string $class): void
    {
        $dimmi = $this->client([self::json($status, [
            'code' => 'some_code',
            'message' => 'Something is off',
            'errors' => ['email' => ['is invalid', 'is taken']],
        ])]);
        try {
            $dimmi->contacts->store(['email' => 'nope']);
            $this->fail('expected ' . $class);
        } catch (DimmiException $e) {
            $this->assertSame($class, $e::class);
            $this->assertSame($status, $e->status);
            $this->assertSame('some_code', $e->errorCode);
            $this->assertSame('Something is off', $e->getMessage());
            $this->assertSame(['email' => ['is invalid', 'is taken']], $e->fields);
        }
    }

    public function testAcceptsAFieldsKeyAndNonJsonErrors(): void
    {
        $dimmi = $this->client([
            self::json(422, ['code' => 'invalid', 'message' => 'Bad', 'fields' => ['name' => ['required']]]),
            new Response(502, ['Content-Type' => 'text/html'], '<html>bad gateway</html>'),
        ]);
        try {
            $dimmi->contacts->store([]);
        } catch (ValidationException $e) {
            $this->assertSame(['name' => ['required']], $e->fields);
        }
        try {
            $dimmi->contacts->index();
        } catch (ServerException $e) {
            $this->assertSame('dimmi answered HTTP 502', $e->getMessage());
            $this->assertNull($e->errorCode);
        }
    }

    public function testPaginateFollowsCursors(): void
    {
        $dimmi = $this->client([
            self::json(200, ['data' => [['id' => 1], ['id' => 2]], 'meta' => ['next_cursor' => 'c2']]),
            self::json(200, ['data' => [['id' => 3]], 'meta' => ['next_cursor' => null]]),
        ]);

        $ids = array_map(fn ($c) => $c['id'], iterator_to_array($dimmi->paginate('/conversations', ['limit' => 2]), false));

        $this->assertSame([1, 2, 3], $ids);
        $this->assertSame('limit=2', $this->request(0)->getUri()->getQuery());
        $this->assertSame('limit=2&cursor=c2', $this->request(1)->getUri()->getQuery());
    }

    public function testPaginateTakesAnOperationIdAndStopsOnAnEmptyPage(): void
    {
        $dimmi = $this->client([
            self::json(200, ['data' => [], 'meta' => ['next_cursor' => 'never']]),
        ]);
        $this->assertSame([], iterator_to_array($dimmi->paginate('conversations.messages.index', params: ['cnv_1'])));
        $this->assertSame('/api/v1/conversations/cnv_1/messages', $this->request(0)->getUri()->getPath());
        $this->assertSame('limit=100', $this->request(0)->getUri()->getQuery());
    }

    public function testPagesWalkPageNumbersUntilAShortPage(): void
    {
        $dimmi = $this->client([
            self::json(200, ['data' => [['id' => 1], ['id' => 2]]]),
            self::json(200, ['data' => [['id' => 3]]]),
        ]);

        $pages = iterator_to_array($dimmi->pages('/tags', ['limit' => 2]), false);

        $this->assertSame([[['id' => 1], ['id' => 2]], [['id' => 3]]], $pages);
        $this->assertSame('limit=2&page=1', $this->request(0)->getUri()->getQuery());
        $this->assertSame('limit=2&page=2', $this->request(1)->getUri()->getQuery());
    }

    public function testPagesStopOnAnEmptyPage(): void
    {
        $dimmi = $this->client([
            self::json(200, ['data' => [['id' => 1]]]),
            self::json(200, ['data' => []]),
        ]);
        $this->assertCount(1, iterator_to_array($dimmi->pages('/tags', ['limit' => 1]), false));
        $this->assertCount(2, $this->history);
    }

    public function testEmptyAnswersAreNull(): void
    {
        $dimmi = $this->client([new Response(204)]);
        $this->assertNull($dimmi->call('contacts.destroy', ['con_1']));
        $this->assertSame('DELETE', $this->request(0)->getMethod());
    }
}
