<?php

declare(strict_types=1);

namespace Mockly\Testcontainers\Tests;

use Mockly\Testcontainers\Model\FaultConfig;
use Mockly\Testcontainers\Model\Mock;
use Mockly\Testcontainers\Model\MockRequest;
use Mockly\Testcontainers\Model\MockResponse;
use Mockly\Testcontainers\Model\MockResponsePatch;
use Mockly\Testcontainers\Model\Scenario;
use Mockly\Testcontainers\Model\ScenarioPatch;
use Mockly\Testcontainers\MocklyContainer;
use Mockly\Testcontainers\Tests\Fixtures\FakeHttpServer;
use PHPUnit\Framework\TestCase;
use ReflectionClass;
use RuntimeException;

/**
 * Exercises every {@see MocklyContainer} HTTP method against a fake management API
 * (PHP's built-in web server), without needing Docker. Complements
 * {@see MocklyContainerIntegrationTest}, which validates the real container wiring.
 */
final class MocklyContainerTest extends TestCase
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

    /** Builds a MocklyContainer pointed at the fake API, bypassing the real Docker-backed constructor. */
    private function container(): MocklyContainer
    {
        $reflection = new ReflectionClass(MocklyContainer::class);
        $container = $reflection->newInstanceWithoutConstructor();
        $reflection->getProperty('apiBase')->setValue($container, self::$fake->baseUrl);
        $reflection->getProperty('httpBase')->setValue($container, 'http://127.0.0.1:9999');

        return $container;
    }

    private function sampleMock(string $id = 'm1'): Mock
    {
        return new Mock($id, new MockRequest('GET', '/ping'), new MockResponse(200, 'ok'));
    }

    public function testGetApiBaseAndHttpBaseReturnCachedValues(): void
    {
        $container = $this->container();

        self::assertSame(self::$fake->baseUrl, $container->getApiBase());
        self::assertSame('http://127.0.0.1:9999', $container->getHttpBase());
    }

    public function testGetApiBaseThrowsBeforeStart(): void
    {
        $container = (new ReflectionClass(MocklyContainer::class))->newInstanceWithoutConstructor();

        $this->expectException(RuntimeException::class);
        $this->expectExceptionMessageMatches('/has not been started/');
        $container->getApiBase();
    }

    public function testAddMockSendsPost(): void
    {
        self::$fake->setResponse(201, '');
        $this->container()->addMock($this->sampleMock());

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
        $this->container()->addMock($this->sampleMock());
    }

    public function testListMocksSendsGetAndParsesResponse(): void
    {
        self::$fake->setResponse(200, json_encode([[
            'id' => 'm1',
            'request' => ['method' => 'GET', 'path' => '/ping'],
            'response' => ['status' => 200, 'body' => 'ok'],
        ]]));

        $mocks = $this->container()->listMocks();

        self::assertSame('GET', self::$fake->lastRequest()['method']);
        self::assertSame('/api/mocks/http', self::$fake->lastRequest()['path']);
        self::assertCount(1, $mocks);
        self::assertSame('m1', $mocks[0]->id);
    }

    public function testUpdateMockSendsPut(): void
    {
        self::$fake->setResponse(200, json_encode([
            'id' => 'm1',
            'request' => ['method' => 'GET', 'path' => '/ping'],
            'response' => ['status' => 201, 'body' => 'updated'],
        ]));

        $updated = $this->container()->updateMock('m1', $this->sampleMock());

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

        $patched = $this->container()->patchMock('m1', new MockResponsePatch(status: 503));

        self::assertSame('PATCH', self::$fake->lastRequest()['method']);
        self::assertStringContainsString('"status":503', self::$fake->lastRequest()['body']);
        self::assertSame(503, $patched->response->status);
    }

    public function testDeleteMockSendsDelete(): void
    {
        $this->container()->deleteMock('m1');

        self::assertSame('DELETE', self::$fake->lastRequest()['method']);
        self::assertSame('/api/mocks/http/m1', self::$fake->lastRequest()['path']);
    }

    public function testGetStateSendsGet(): void
    {
        self::$fake->setResponse(200, json_encode(['k' => 'v']));
        $state = $this->container()->getState();

        self::assertSame('GET', self::$fake->lastRequest()['method']);
        self::assertSame('/api/state', self::$fake->lastRequest()['path']);
        self::assertSame(['k' => 'v'], $state);
    }

    public function testSetStateSendsPost(): void
    {
        self::$fake->setResponse(200, json_encode(['k' => 'v']));
        $result = $this->container()->setState(['k' => 'v']);

        self::assertSame('POST', self::$fake->lastRequest()['method']);
        self::assertStringContainsString('"k":"v"', self::$fake->lastRequest()['body']);
        self::assertSame(['k' => 'v'], $result);
    }

    public function testDeleteStateSendsDelete(): void
    {
        $this->container()->deleteState('k');

        self::assertSame('DELETE', self::$fake->lastRequest()['method']);
        self::assertSame('/api/state/k', self::$fake->lastRequest()['path']);
    }

    public function testResetSendsPost(): void
    {
        $this->container()->reset();
        self::assertSame('POST', self::$fake->lastRequest()['method']);
        self::assertSame('/api/reset', self::$fake->lastRequest()['path']);
    }

    public function testActivateScenarioSendsPost(): void
    {
        $this->container()->activateScenario('s1');
        self::assertSame('POST', self::$fake->lastRequest()['method']);
        self::assertSame('/api/scenarios/s1/activate', self::$fake->lastRequest()['path']);
    }

    public function testDeactivateScenarioSendsPost(): void
    {
        $this->container()->deactivateScenario('s1');
        self::assertSame('/api/scenarios/s1/deactivate', self::$fake->lastRequest()['path']);
    }

    public function testListScenariosSendsGetAndParses(): void
    {
        self::$fake->setResponse(200, json_encode([
            ['id' => 's1', 'name' => 'Test', 'patches' => []],
        ]));

        $scenarios = $this->container()->listScenarios();

        self::assertSame('GET', self::$fake->lastRequest()['method']);
        self::assertSame('/api/scenarios', self::$fake->lastRequest()['path']);
        self::assertCount(1, $scenarios);
        self::assertSame('s1', $scenarios[0]->id);
    }

    public function testCreateScenarioSendsPostAndParses(): void
    {
        self::$fake->setResponse(201, json_encode([
            'id' => 's1', 'name' => 'Test', 'patches' => [['mock_id' => 'm1']],
        ]));

        $scenario = $this->container()->createScenario(new Scenario('s1', 'Test', [new ScenarioPatch('m1')]));

        self::assertSame('POST', self::$fake->lastRequest()['method']);
        self::assertSame('/api/scenarios', self::$fake->lastRequest()['path']);
        self::assertSame('s1', $scenario->id);
        self::assertCount(1, $scenario->patches);
    }

    public function testGetScenarioSendsGet(): void
    {
        self::$fake->setResponse(200, json_encode(['id' => 's1', 'name' => 'Test', 'patches' => []]));
        $scenario = $this->container()->getScenario('s1');

        self::assertSame('/api/scenarios/s1', self::$fake->lastRequest()['path']);
        self::assertSame('s1', $scenario->id);
    }

    public function testUpdateScenarioSendsPut(): void
    {
        self::$fake->setResponse(200, json_encode(['id' => 's1', 'name' => 'Test v2', 'patches' => []]));
        $scenario = $this->container()->updateScenario('s1', new Scenario('s1', 'Test v2', []));

        self::assertSame('PUT', self::$fake->lastRequest()['method']);
        self::assertSame('Test v2', $scenario->name);
    }

    public function testDeleteScenarioSendsDelete(): void
    {
        $this->container()->deleteScenario('s1');
        self::assertSame('DELETE', self::$fake->lastRequest()['method']);
        self::assertSame('/api/scenarios/s1', self::$fake->lastRequest()['path']);
    }

    public function testListActiveScenariosSendsGetAndParses(): void
    {
        self::$fake->setResponse(200, json_encode([
            'active' => ['s1'],
            'scenarios' => [['id' => 's1', 'name' => 'Test', 'patches' => []]],
        ]));

        $result = $this->container()->listActiveScenarios();

        self::assertSame('/api/scenarios/active', self::$fake->lastRequest()['path']);
        self::assertSame(['s1'], $result->active);
        self::assertCount(1, $result->scenarios);
    }

    public function testSetFaultSendsPost(): void
    {
        $this->container()->setFault(new FaultConfig(enabled: true, delay: '200ms', status: 500, errorRate: 0.5));

        self::assertSame('POST', self::$fake->lastRequest()['method']);
        self::assertSame('/api/fault/http', self::$fake->lastRequest()['path']);
        self::assertStringContainsString('"delay":"200ms"', self::$fake->lastRequest()['body']);
    }

    public function testClearFaultSendsDelete(): void
    {
        $this->container()->clearFault();
        self::assertSame('DELETE', self::$fake->lastRequest()['method']);
        self::assertSame('/api/fault', self::$fake->lastRequest()['path']);
    }

    public function testGetLogsSendsGetWithMatchedId(): void
    {
        self::$fake->setResponse(200, json_encode([[
            'id' => 'c1', 'timestamp' => '2026-01-01T00:00:00Z', 'protocol' => 'http',
            'path' => '/ping', 'duration_ms' => 5, 'method' => 'GET', 'status' => 200, 'matched_id' => 'm1',
        ]]));

        $logs = $this->container()->getLogs('m1');

        self::assertSame('GET', self::$fake->lastRequest()['method']);
        self::assertSame('/api/logs?matched_id=m1', self::$fake->lastRequest()['full_path']);
        self::assertCount(1, $logs);
        self::assertSame('m1', $logs[0]->matchedId);
    }

    public function testGetLogsCountSendsGet(): void
    {
        self::$fake->setResponse(200, json_encode(['count' => 3]));
        $count = $this->container()->getLogsCount();

        self::assertSame('/api/logs/count', self::$fake->lastRequest()['path']);
        self::assertSame(3, $count);
    }

    public function testClearLogsSendsDelete(): void
    {
        $this->container()->clearLogs();
        self::assertSame('DELETE', self::$fake->lastRequest()['method']);
        self::assertSame('/api/logs', self::$fake->lastRequest()['path']);
    }

    public function testGetCallsSendsGetAndParses(): void
    {
        self::$fake->setResponse(200, json_encode(['mock_id' => 'm1', 'count' => 2, 'calls' => []]));
        $summary = $this->container()->getCalls('m1');

        self::assertSame('/api/calls/http/m1', self::$fake->lastRequest()['path']);
        self::assertSame(2, $summary->count);
    }

    public function testClearCallsSendsDelete(): void
    {
        $this->container()->clearCalls('m1');
        self::assertSame('/api/calls/http/m1', self::$fake->lastRequest()['path']);
    }

    public function testClearAllCallsSendsDelete(): void
    {
        $this->container()->clearAllCalls();
        self::assertSame('/api/calls/http', self::$fake->lastRequest()['path']);
    }

    public function testWaitForCallsSendsPostWithTimeout(): void
    {
        self::$fake->setResponse(200, json_encode(['mock_id' => 'm1', 'count' => 1, 'calls' => []]));
        $summary = $this->container()->waitForCalls('m1', count: 1, timeoutSeconds: 5);

        self::assertSame('/api/calls/http/m1/wait', self::$fake->lastRequest()['path']);
        self::assertStringContainsString('"timeout":"5s"', self::$fake->lastRequest()['body']);
        self::assertSame(1, $summary->count);
    }

    public function testWaitForCallsRaisesFriendlyMessageOn408(): void
    {
        self::$fake->setResponse(408, '{"error":"timeout"}');

        $this->expectException(RuntimeException::class);
        $this->expectExceptionMessageMatches('/timeout waiting for 1 call/');

        $this->container()->waitForCalls('m1');
    }

    public function testWithInlineConfigOverridesRenderedYaml(): void
    {
        $container = new MocklyContainer();
        $container->withInlineConfig("mockly:\n  api:\n    port: 1\n");

        $reflection = new ReflectionClass(MocklyContainer::class);
        self::assertSame("mockly:\n  api:\n    port: 1\n", $reflection->getProperty('configYaml')->getValue($container));
    }

    public function testWithScenariosRendersYamlWithScenarios(): void
    {
        $container = new MocklyContainer();
        $container->withScenarios([new Scenario('s1', 'Test', [])]);

        $reflection = new ReflectionClass(MocklyContainer::class);
        $yaml = $reflection->getProperty('configYaml')->getValue($container);
        self::assertStringContainsString('scenarios:', $yaml);
        self::assertStringContainsString('s1', $yaml);
    }
}
