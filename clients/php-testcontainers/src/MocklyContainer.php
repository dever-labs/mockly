<?php

declare(strict_types=1);

namespace Mockly\Testcontainers;

use Mockly\Testcontainers\Internal\ConfigRenderer;
use Mockly\Testcontainers\Model\ActiveScenariosResponse;
use Mockly\Testcontainers\Model\CallEntry;
use Mockly\Testcontainers\Model\CallSummary;
use Mockly\Testcontainers\Model\FaultConfig;
use Mockly\Testcontainers\Model\Mock;
use Mockly\Testcontainers\Model\MockRequest;
use Mockly\Testcontainers\Model\MockResponse;
use Mockly\Testcontainers\Model\MockResponsePatch;
use Mockly\Testcontainers\Model\Scenario;
use Mockly\Testcontainers\Model\ScenarioPatch;
use RuntimeException;
use Testcontainers\Container\GenericContainer;
use Testcontainers\Container\StartedGenericContainer;
use Testcontainers\Wait\WaitForHttp;

/**
 * Runs Mockly as a Docker container (`ghcr.io/dever-labs/mockly:latest`) and exposes
 * the same HTTP management API wrapper as {@see \Mockly\Driver\MocklyServer}.
 */
final class MocklyContainer
{
    public const DEFAULT_IMAGE = 'ghcr.io/dever-labs/mockly:latest';

    private const HTTP_PORT = 8090;
    private const API_PORT = 9091;
    private const CONTAINER_CONFIG_PATH = '/config/mockly.yaml';

    private readonly GenericContainer $container;
    private ?StartedGenericContainer $started = null;
    private ?string $httpBase = null;
    private ?string $apiBase = null;
    private string $configYaml;

    public function __construct(string $image = self::DEFAULT_IMAGE)
    {
        $this->container = new GenericContainer($image);
        $this->container->withExposedPorts(self::HTTP_PORT, self::API_PORT);
        $this->configYaml = ConfigRenderer::render(self::HTTP_PORT, self::API_PORT, null);
    }

    /** Overrides the rendered YAML config outright. */
    public function withInlineConfig(string $yaml): self
    {
        $this->configYaml = $yaml;

        return $this;
    }

    /** @param list<Scenario>|null $scenarios */
    public function withScenarios(?array $scenarios): self
    {
        $this->configYaml = ConfigRenderer::render(self::HTTP_PORT, self::API_PORT, $scenarios);

        return $this;
    }

    public function start(): self
    {
        $this->container
            ->withCopyContentToContainer([
                ['content' => $this->configYaml, 'target' => self::CONTAINER_CONFIG_PATH, 'mode' => 0o644],
            ])
            ->withCommand(['start', '-c', self::CONTAINER_CONFIG_PATH])
            ->withWait((new WaitForHttp(self::API_PORT))->withPath('/api/protocols'));

        $this->started = $this->container->start();
        $this->httpBase = "http://{$this->started->getHost()}:{$this->started->getMappedPort(self::HTTP_PORT)}";
        $this->apiBase = "http://{$this->started->getHost()}:{$this->started->getMappedPort(self::API_PORT)}";

        return $this;
    }

    public function stop(): void
    {
        $this->started?->stop();
        $this->started = null;
        $this->httpBase = null;
        $this->apiBase = null;
    }

    public function getHttpBase(): string
    {
        return $this->httpBase ?? throw new RuntimeException('MocklyContainer has not been started; call start() first.');
    }

    public function getApiBase(): string
    {
        return $this->apiBase ?? throw new RuntimeException('MocklyContainer has not been started; call start() first.');
    }

    public function addMock(Mock $mock): void
    {
        $this->request('POST', '/api/mocks/http', self::mockToArray($mock), [200, 201]);
    }

    /** @return list<Mock> */
    public function listMocks(): array
    {
        return array_map(self::parseMock(...), $this->requestJson('GET', '/api/mocks/http', expected: [200]));
    }

    public function updateMock(string $mockId, Mock $mock): Mock
    {
        $path = '/api/mocks/http/' . rawurlencode($mockId);

        return self::parseMock($this->requestJson('PUT', $path, self::mockToArray($mock), [200]));
    }

    public function patchMock(string $mockId, MockResponsePatch $patch): Mock
    {
        $path = '/api/mocks/http/' . rawurlencode($mockId);

        return self::parseMock($this->requestJson('PATCH', $path, self::mockResponsePatchToArray($patch), [200]));
    }

    public function deleteMock(string $mockId): void
    {
        $this->request('DELETE', '/api/mocks/http/' . rawurlencode($mockId), expected: [200]);
    }

    /** @return array<string, string> */
    public function getState(): array
    {
        return $this->requestJson('GET', '/api/state', expected: [200]);
    }

    /**
     * @param array<string, string> $kvMap
     * @return array<string, string>
     */
    public function setState(array $kvMap): array
    {
        return $this->requestJson('POST', '/api/state', $kvMap, [200]);
    }

    public function deleteState(string $key): void
    {
        $this->request('DELETE', '/api/state/' . rawurlencode($key), expected: [200]);
    }

    public function reset(): void
    {
        $this->request('POST', '/api/reset', expected: [200]);
    }

    public function activateScenario(string $scenarioId): void
    {
        $this->request('POST', '/api/scenarios/' . rawurlencode($scenarioId) . '/activate', expected: [200]);
    }

    public function deactivateScenario(string $scenarioId): void
    {
        $this->request('POST', '/api/scenarios/' . rawurlencode($scenarioId) . '/deactivate', expected: [200]);
    }

    /** @return list<Scenario> */
    public function listScenarios(): array
    {
        return array_map(self::parseScenario(...), $this->requestJson('GET', '/api/scenarios', expected: [200]));
    }

    public function createScenario(Scenario $scenario): Scenario
    {
        return self::parseScenario($this->requestJson('POST', '/api/scenarios', self::scenarioToArray($scenario), [201]));
    }

    public function getScenario(string $scenarioId): Scenario
    {
        return self::parseScenario($this->requestJson('GET', '/api/scenarios/' . rawurlencode($scenarioId), expected: [200]));
    }

    public function updateScenario(string $scenarioId, Scenario $scenario): Scenario
    {
        $path = '/api/scenarios/' . rawurlencode($scenarioId);

        return self::parseScenario($this->requestJson('PUT', $path, self::scenarioToArray($scenario), [200]));
    }

    public function deleteScenario(string $scenarioId): void
    {
        $this->request('DELETE', '/api/scenarios/' . rawurlencode($scenarioId), expected: [200]);
    }

    public function listActiveScenarios(): ActiveScenariosResponse
    {
        $data = $this->requestJson('GET', '/api/scenarios/active', expected: [200]);

        return new ActiveScenariosResponse(
            active: array_values($data['active'] ?? []),
            scenarios: array_map(self::parseScenario(...), $data['scenarios'] ?? []),
        );
    }

    public function setFault(FaultConfig $config): void
    {
        $this->request('POST', '/api/fault/http', self::faultConfigToArray($config), [200]);
    }

    public function clearFault(): void
    {
        $this->request('DELETE', '/api/fault', expected: [200, 204]);
    }

    /** @return list<CallEntry> */
    public function getLogs(?string $matchedId = null): array
    {
        $path = self::withOptionalMatchedId('/api/logs', $matchedId);

        return array_map(self::parseCallEntry(...), $this->requestJson('GET', $path, expected: [200]));
    }

    public function getLogsCount(?string $matchedId = null): int
    {
        $path = self::withOptionalMatchedId('/api/logs/count', $matchedId);

        return (int) ($this->requestJson('GET', $path, expected: [200])['count'] ?? 0);
    }

    public function clearLogs(): void
    {
        $this->request('DELETE', '/api/logs', expected: [200]);
    }

    public function getCalls(string $mockId): CallSummary
    {
        return self::parseCallSummary($this->requestJson('GET', '/api/calls/http/' . rawurlencode($mockId), expected: [200]));
    }

    public function clearCalls(string $mockId): void
    {
        $this->request('DELETE', '/api/calls/http/' . rawurlencode($mockId), expected: [200]);
    }

    public function clearAllCalls(): void
    {
        $this->request('DELETE', '/api/calls/http', expected: [200]);
    }

    public function waitForCalls(string $mockId, int $count = 1, int $timeoutSeconds = 10): CallSummary
    {
        $path = '/api/calls/http/' . rawurlencode($mockId) . '/wait';
        $body = ['count' => $count, 'timeout' => "{$timeoutSeconds}s"];

        try {
            $data = $this->requestJson('POST', $path, $body, [200]);
        } catch (RuntimeException $exc) {
            if (str_contains($exc->getMessage(), '408')) {
                throw new RuntimeException(
                    "waitForCalls: timeout waiting for {$count} call(s) on '{$mockId}'",
                    previous: $exc,
                );
            }
            throw $exc;
        }

        return self::parseCallSummary($data);
    }

    private static function withOptionalMatchedId(string $path, ?string $matchedId): string
    {
        if ($matchedId === null || $matchedId === '') {
            return $path;
        }

        return $path . '?matched_id=' . rawurlencode($matchedId);
    }

    /**
     * @param array<string, mixed>|list<mixed>|null $body
     * @param list<int> $expected
     * @return array<string, mixed>|list<mixed>
     */
    private function requestJson(string $method, string $path, array|null $body = null, array $expected = [200]): array
    {
        $raw = $this->request($method, $path, $body, $expected);

        return $raw === '' ? [] : json_decode($raw, true, flags: JSON_THROW_ON_ERROR);
    }

    /**
     * @param array<string, mixed>|list<mixed>|null $body
     * @param list<int> $expected
     */
    private function request(string $method, string $path, array|null $body = null, array $expected = [200]): string
    {
        $url = "{$this->getApiBase()}{$path}";
        $ch = curl_init($url);
        $options = [
            CURLOPT_CUSTOMREQUEST => $method,
            CURLOPT_RETURNTRANSFER => true,
        ];

        if ($body !== null) {
            $options[CURLOPT_POSTFIELDS] = json_encode($body, JSON_THROW_ON_ERROR);
            $options[CURLOPT_HTTPHEADER] = ['Content-Type: application/json'];
        }

        curl_setopt_array($ch, $options);
        $response = curl_exec($ch);
        $status = (int) curl_getinfo($ch, CURLINFO_HTTP_CODE);
        $curlError = curl_error($ch);
        curl_close($ch);

        if ($response === false) {
            throw new RuntimeException("Request failed for {$method} {$path}: {$curlError}");
        }

        if (!in_array($status, $expected, true)) {
            throw new RuntimeException("HTTP {$status} for {$method} {$path}: {$response}");
        }

        return $response;
    }

    /** @return array<string, mixed> */
    private static function mockToArray(Mock $mock): array
    {
        $request = ['method' => $mock->request->method, 'path' => $mock->request->path];
        if (!empty($mock->request->headers)) {
            $request['headers'] = $mock->request->headers;
        }

        return ['id' => $mock->id, 'request' => $request, 'response' => self::mockResponseToArray($mock->response)];
    }

    /** @return array<string, mixed> */
    private static function mockResponseToArray(MockResponse $response): array
    {
        $body = ['status' => $response->status];
        if ($response->body !== '') {
            $body['body'] = $response->body;
        }
        if (!empty($response->headers)) {
            $body['headers'] = $response->headers;
        }
        if ($response->delay !== null) {
            $body['delay'] = $response->delay;
        }

        return $body;
    }

    /** @return array<string, mixed> */
    private static function mockResponsePatchToArray(MockResponsePatch $patch): array
    {
        $body = [];
        if ($patch->status !== null) {
            $body['status'] = $patch->status;
        }
        if ($patch->body !== null) {
            $body['body'] = $patch->body;
        }
        if ($patch->headers !== null) {
            $body['headers'] = $patch->headers;
        }
        if ($patch->delay !== null) {
            $body['delay'] = $patch->delay;
        }

        return $body;
    }

    /** @return array<string, mixed> */
    private static function scenarioToArray(Scenario $scenario): array
    {
        $body = [
            'id' => $scenario->id,
            'name' => $scenario->name,
            'patches' => array_map(self::scenarioPatchToArray(...), $scenario->patches),
        ];
        if ($scenario->description !== null) {
            $body['description'] = $scenario->description;
        }

        return $body;
    }

    /** @return array<string, mixed> */
    private static function scenarioPatchToArray(ScenarioPatch $patch): array
    {
        $body = ['mock_id' => $patch->mockId];
        if ($patch->status !== null) {
            $body['status'] = $patch->status;
        }
        if ($patch->body !== null) {
            $body['body'] = $patch->body;
        }
        if ($patch->headers !== null) {
            $body['headers'] = $patch->headers;
        }
        if ($patch->delay !== null) {
            $body['delay'] = $patch->delay;
        }
        if ($patch->disabled !== null) {
            $body['disabled'] = $patch->disabled;
        }

        return $body;
    }

    /** @return array<string, mixed> */
    private static function faultConfigToArray(FaultConfig $config): array
    {
        $body = ['enabled' => $config->enabled];
        if ($config->delay !== null) {
            $body['delay'] = $config->delay;
        }
        if ($config->status !== null) {
            $body['status'] = $config->status;
        }
        if ($config->errorRate !== null) {
            $body['error_rate'] = $config->errorRate;
        }

        return $body;
    }

    /** @param array<string, mixed> $data */
    private static function parseMock(array $data): Mock
    {
        $request = $data['request'] ?? [];
        $response = $data['response'] ?? [];

        return new Mock(
            id: $data['id'] ?? '',
            request: new MockRequest(
                method: $request['method'] ?? '',
                path: $request['path'] ?? '',
                headers: $request['headers'] ?? [],
            ),
            response: new MockResponse(
                status: $response['status'] ?? 0,
                body: $response['body'] ?? '',
                headers: $response['headers'] ?? [],
                delay: $response['delay'] ?? null,
            ),
        );
    }

    /** @param array<string, mixed> $data */
    private static function parseScenario(array $data): Scenario
    {
        return new Scenario(
            id: $data['id'] ?? '',
            name: $data['name'] ?? '',
            patches: array_map(self::parseScenarioPatch(...), $data['patches'] ?? []),
            description: $data['description'] ?? null,
        );
    }

    /** @param array<string, mixed> $data */
    private static function parseScenarioPatch(array $data): ScenarioPatch
    {
        return new ScenarioPatch(
            mockId: $data['mock_id'] ?? '',
            status: $data['status'] ?? null,
            body: $data['body'] ?? null,
            headers: $data['headers'] ?? null,
            delay: $data['delay'] ?? null,
            disabled: $data['disabled'] ?? null,
        );
    }

    /** @param array<string, mixed> $data */
    private static function parseCallSummary(array $data): CallSummary
    {
        return new CallSummary(
            mockId: $data['mock_id'] ?? '',
            count: $data['count'] ?? 0,
            calls: array_map(self::parseCallEntry(...), $data['calls'] ?? []),
        );
    }

    /** @param array<string, mixed> $data */
    private static function parseCallEntry(array $data): CallEntry
    {
        return new CallEntry(
            id: $data['id'] ?? '',
            timestamp: $data['timestamp'] ?? '',
            protocol: $data['protocol'] ?? '',
            path: $data['path'] ?? '',
            durationMs: $data['duration_ms'] ?? 0,
            method: $data['method'] ?? null,
            status: $data['status'] ?? null,
            headers: $data['headers'] ?? [],
            body: $data['body'] ?? null,
            matchedId: $data['matched_id'] ?? null,
            pathParams: $data['path_params'] ?? [],
        );
    }
}
