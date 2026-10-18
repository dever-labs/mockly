<?php

declare(strict_types=1);

namespace Mockly\Driver\Tests;

use Mockly\Driver\MocklyServer;
use Mockly\Driver\Model\FaultConfig;
use Mockly\Driver\Model\Mock;
use Mockly\Driver\Model\MockRequest;
use Mockly\Driver\Model\MockResponse;
use Mockly\Driver\Model\MockResponsePatch;
use Mockly\Driver\Model\Scenario;
use Mockly\Driver\Model\ScenarioPatch;
use Mockly\Driver\Tests\Fixtures\FakeHttpServer;
use PHPUnit\Framework\TestCase;
use ReflectionClass;
use RuntimeException;

/**
 * Exercises every {@see MocklyServer} HTTP method against a fake management API
 * (PHP's built-in web server), without needing a real mockly binary.
 */
final class MocklyServerTest extends TestCase
{
    private static FakeHttpServer $fake;

    public static function setUpBeforeClass(): void
    {
        self::$fake = FakeHttpServer::start();
    }

    public static function tearDownAfterClass(): void
    {
        self::$fake->stop();
    }

    protected function setUp(): void
    {
        self::$fake->setResponse(200, '');
    }

    /** Builds a MocklyServer pointed at the fake API, bypassing the real-binary constructor. */
    private function server(): MocklyServer
    {
        $reflection = new ReflectionClass(MocklyServer::class);
        $server = $reflection->newInstanceWithoutConstructor();
        foreach (['httpPort' => 9999, 'apiPort' => 9998, 'httpBase' => 'http://127.0.0.1:9999', 'apiBase' => self::$fake->baseUrl] as $name => $value) {
            $reflection->getProperty($name)->setValue($server, $value);
        }

        return $server;
    }

    private function sampleMock(string $id = 'm1'): Mock
    {
        return new Mock($id, new MockRequest('GET', '/ping'), new MockResponse(200, 'ok'));
    }

    public function testAddMockSendsPost(): void
    {
        self::$fake->setResponse(201, '');
        $this->server()->addMock($this->sampleMock());

        $req = self::$fake->lastRequest();
        self::assertSame('POST', $req['method']);
        self::assertSame('/api/mocks/http', $req['path']);
        self::assertStringContainsString('"id":"m1"', $req['body']);
    }

    public function testAddMockThrowsOnUnexpectedStatus(): void
    {
        self::$fake->setResponse(500, '{"error":"boom"}');
        $this->expectException(RuntimeException::class);
        $this->expectExceptionMessageMatches('/HTTP 500/');
        $this->server()->addMock($this->sampleMock());
    }

    public function testListMocksSendsGetAndParsesResponse(): void
    {
        self::$fake->setResponse(200, json_encode([[
            'id' => 'm1',
            'request' => ['method' => 'GET', 'path' => '/ping'],
            'response' => ['status' => 200, 'body' => 'ok'],
        ]]));

        $mocks = $this->server()->listMocks();

        self::assertSame('GET', self::$fake->lastRequest()['method']);
        self::assertSame('/api/mocks/http', self::$fake->lastRequest()['path']);
        self::assertCount(1, $mocks);
        self::assertSame('m1', $mocks[0]->id);
        self::assertSame('GET', $mocks[0]->request->method);
        self::assertSame(200, $mocks[0]->response->status);
    }

    public function testUpdateMockSendsPut(): void
    {
        self::$fake->setResponse(200, json_encode([
            'id' => 'm1',
            'request' => ['method' => 'GET', 'path' => '/ping'],
            'response' => ['status' => 201, 'body' => 'updated'],
        ]));

        $updated = $this->server()->updateMock('m1', $this->sampleMock());

        self::assertSame('PUT', self::$fake->lastRequest()['method']);
        self::assertSame('/api/mocks/http/m1', self::$fake->lastRequest()['path']);
        self::assertSame(201, $updated->response->status);
    }

    public function testPatchMockSendsPatch(): void
    {
        self::$fake->setResponse(200, json_encode([
            'id' => 'm1',
            'request' => ['method' => 'GET', 'path' => '/ping'],
            'response' => ['status' => 503, 'body' => ''],
        ]));

        $patched = $this->server()->patchMock('m1', new MockResponsePatch(status: 503));

        self::assertSame('PATCH', self::$fake->lastRequest()['method']);
        self::assertStringContainsString('"status":503', self::$fake->lastRequest()['body']);
        self::assertSame(503, $patched->response->status);
    }

    public function testDeleteMockSendsDelete(): void
    {
        $this->server()->deleteMock('m1');

        self::assertSame('DELETE', self::$fake->lastRequest()['method']);
        self::assertSame('/api/mocks/http/m1', self::$fake->lastRequest()['path']);
    }

    public function testGetStateSendsGet(): void
    {
        self::$fake->setResponse(200, json_encode(['k' => 'v']));
        $state = $this->server()->getState();

        self::assertSame('GET', self::$fake->lastRequest()['method']);
        self::assertSame('/api/state', self::$fake->lastRequest()['path']);
        self::assertSame(['k' => 'v'], $state);
    }

    public function testSetStateSendsPost(): void
    {
        self::$fake->setResponse(200, json_encode(['k' => 'v']));
        $result = $this->server()->setState(['k' => 'v']);

        self::assertSame('POST', self::$fake->lastRequest()['method']);
        self::assertStringContainsString('"k":"v"', self::$fake->lastRequest()['body']);
        self::assertSame(['k' => 'v'], $result);
    }

    public function testDeleteStateSendsDelete(): void
    {
        $this->server()->deleteState('k');

        self::assertSame('DELETE', self::$fake->lastRequest()['method']);
        self::assertSame('/api/state/k', self::$fake->lastRequest()['path']);
    }

    public function testGetLogsSendsGetWithMatchedId(): void
    {
        self::$fake->setResponse(200, json_encode([[
            'id' => 'c1', 'timestamp' => '2026-01-01T00:00:00Z', 'protocol' => 'http',
            'path' => '/ping', 'duration_ms' => 5, 'method' => 'GET', 'status' => 200, 'matched_id' => 'm1',
        ]]));

        $logs = $this->server()->getLogs('m1');

        self::assertSame('GET', self::$fake->lastRequest()['method']);
        self::assertSame('/api/logs?matched_id=m1', self::$fake->lastRequest()['full_path']);
        self::assertCount(1, $logs);
        self::assertSame('m1', $logs[0]->matchedId);
    }

    public function testClearLogsSendsDelete(): void
    {
        $this->server()->clearLogs();
        self::assertSame('DELETE', self::$fake->lastRequest()['method']);
        self::assertSame('/api/logs', self::$fake->lastRequest()['path']);
    }

    public function testGetLogsCountSendsGet(): void
    {
        self::$fake->setResponse(200, json_encode(['count' => 3]));
        $count = $this->server()->getLogsCount();

        self::assertSame('/api/logs/count', self::$fake->lastRequest()['path']);
        self::assertSame(3, $count);
    }

    public function testResetSendsPost(): void
    {
        $this->server()->reset();
        self::assertSame('POST', self::$fake->lastRequest()['method']);
        self::assertSame('/api/reset', self::$fake->lastRequest()['path']);
    }

    public function testCreateScenarioSendsPostAndParses(): void
    {
        self::$fake->setResponse(201, json_encode([
            'id' => 's1', 'name' => 'Test', 'patches' => [['mock_id' => 'm1']],
        ]));

        $scenario = $this->server()->createScenario(new Scenario('s1', 'Test', [new ScenarioPatch('m1')]));

        self::assertSame('POST', self::$fake->lastRequest()['method']);
        self::assertSame('/api/scenarios', self::$fake->lastRequest()['path']);
        self::assertSame('s1', $scenario->id);
        self::assertCount(1, $scenario->patches);
    }

    public function testListScenariosSendsGetAndParses(): void
    {
        self::$fake->setResponse(200, json_encode([
            ['id' => 's1', 'name' => 'Test', 'patches' => []],
        ]));

        $scenarios = $this->server()->listScenarios();

        self::assertSame('GET', self::$fake->lastRequest()['method']);
        self::assertSame('/api/scenarios', self::$fake->lastRequest()['path']);
        self::assertCount(1, $scenarios);
        self::assertSame('s1', $scenarios[0]->id);
    }

    public function testGetScenarioSendsGet(): void
    {
        self::$fake->setResponse(200, json_encode(['id' => 's1', 'name' => 'Test', 'patches' => []]));
        $scenario = $this->server()->getScenario('s1');

        self::assertSame('/api/scenarios/s1', self::$fake->lastRequest()['path']);
        self::assertSame('s1', $scenario->id);
    }

    public function testUpdateScenarioSendsPut(): void
    {
        self::$fake->setResponse(200, json_encode(['id' => 's1', 'name' => 'Test v2', 'patches' => []]));
        $scenario = $this->server()->updateScenario('s1', new Scenario('s1', 'Test v2', []));

        self::assertSame('PUT', self::$fake->lastRequest()['method']);
        self::assertSame('Test v2', $scenario->name);
    }

    public function testDeleteScenarioSendsDelete(): void
    {
        $this->server()->deleteScenario('s1');
        self::assertSame('DELETE', self::$fake->lastRequest()['method']);
        self::assertSame('/api/scenarios/s1', self::$fake->lastRequest()['path']);
    }

    public function testListActiveScenariosSendsGetAndParses(): void
    {
        self::$fake->setResponse(200, json_encode([
            'active' => ['s1'],
            'scenarios' => [['id' => 's1', 'name' => 'Test', 'patches' => []]],
        ]));

        $result = $this->server()->listActiveScenarios();

        self::assertSame('/api/scenarios/active', self::$fake->lastRequest()['path']);
        self::assertSame(['s1'], $result->active);
        self::assertCount(1, $result->scenarios);
    }

    public function testActivateScenarioSendsPost(): void
    {
        $this->server()->activateScenario('s1');
        self::assertSame('POST', self::$fake->lastRequest()['method']);
        self::assertSame('/api/scenarios/s1/activate', self::$fake->lastRequest()['path']);
    }

    public function testDeactivateScenarioSendsPost(): void
    {
        $this->server()->deactivateScenario('s1');
        self::assertSame('/api/scenarios/s1/deactivate', self::$fake->lastRequest()['path']);
    }

    public function testSetFaultSendsPost(): void
    {
        $this->server()->setFault(new FaultConfig(enabled: true, delay: '200ms', status: 500, errorRate: 0.5));

        self::assertSame('POST', self::$fake->lastRequest()['method']);
        self::assertSame('/api/fault/http', self::$fake->lastRequest()['path']);
        self::assertStringContainsString('"delay":"200ms"', self::$fake->lastRequest()['body']);
    }

    public function testClearFaultSendsDelete(): void
    {
        $this->server()->clearFault();
        self::assertSame('DELETE', self::$fake->lastRequest()['method']);
        self::assertSame('/api/fault', self::$fake->lastRequest()['path']);
    }

    public function testGetCallsSendsGetAndParses(): void
    {
        self::$fake->setResponse(200, json_encode(['mock_id' => 'm1', 'count' => 2, 'calls' => []]));
        $summary = $this->server()->getCalls('m1');

        self::assertSame('/api/calls/http/m1', self::$fake->lastRequest()['path']);
        self::assertSame(2, $summary->count);
    }

    public function testClearCallsSendsDelete(): void
    {
        $this->server()->clearCalls('m1');
        self::assertSame('/api/calls/http/m1', self::$fake->lastRequest()['path']);
    }

    public function testClearAllCallsSendsDelete(): void
    {
        $this->server()->clearAllCalls();
        self::assertSame('/api/calls/http', self::$fake->lastRequest()['path']);
    }

    public function testWaitForCallsSendsPostWithTimeout(): void
    {
        self::$fake->setResponse(200, json_encode(['mock_id' => 'm1', 'count' => 1, 'calls' => []]));
        $summary = $this->server()->waitForCalls('m1', count: 1, timeoutSeconds: 5);

        self::assertSame('/api/calls/http/m1/wait', self::$fake->lastRequest()['path']);
        self::assertStringContainsString('"timeout":"5s"', self::$fake->lastRequest()['body']);
        self::assertSame(1, $summary->count);
    }

    public function testWaitForCallsRaisesFriendlyMessageOn408(): void
    {
        self::$fake->setResponse(408, '{"error":"timeout"}');

        $this->expectException(RuntimeException::class);
        $this->expectExceptionMessageMatches('/timeout waiting for 1 call/');

        $this->server()->waitForCalls('m1');
    }
}
