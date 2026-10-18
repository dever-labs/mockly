<?php

declare(strict_types=1);

namespace Mockly\Driver;

use Mockly\Driver\Model\ActiveScenariosResponse;
use Mockly\Driver\Model\CallEntry;
use Mockly\Driver\Model\CallSummary;
use Mockly\Driver\Internal\ConfigWriter;
use Mockly\Driver\Model\FaultConfig;
use Mockly\Driver\Model\Mock;
use Mockly\Driver\Model\MockRequest;
use Mockly\Driver\Model\MockResponse;
use Mockly\Driver\Model\MockResponsePatch;
use Mockly\Driver\Model\Scenario;
use Mockly\Driver\Model\ScenarioPatch;
use RuntimeException;

/** A running Mockly process with helpers for managing HTTP mocks. */
final class MocklyServer
{
    public readonly int $httpPort;
    public readonly int $apiPort;
    public readonly string $httpBase;
    public readonly string $apiBase;

    /** @var resource */
    private $process;
    private readonly string $configPath;

    /** @param resource $process */
    private function __construct($process, int $httpPort, int $apiPort, string $configPath)
    {
        $this->process = $process;
        $this->configPath = $configPath;
        $this->httpPort = $httpPort;
        $this->apiPort = $apiPort;
        $this->httpBase = "http://127.0.0.1:{$httpPort}";
        $this->apiBase = "http://127.0.0.1:{$apiPort}";
    }

    /**
     * Starts Mockly using an already-installed binary.
     *
     * @param list<Scenario>|null $scenarios
     * @throws RuntimeException if the binary cannot be found
     */
    public static function create(?array $scenarios = null): self
    {
        $binary = Installer::getBinaryPath();
        if ($binary === null) {
            throw new RuntimeException(
                'mockly binary not found. Call Installer::install() or set MOCKLY_BINARY_PATH.'
            );
        }

        return self::start($binary, $scenarios);
    }

    /**
     * Installs the binary if necessary, then starts Mockly. Recommended for most cases.
     *
     * @param list<Scenario>|null $scenarios
     */
    public static function ensure(?array $scenarios = null, ?string $version = null, ?string $baseUrl = null, ?string $binDir = null): self
    {
        Installer::install(version: $version, baseUrl: $baseUrl, binDir: $binDir);

        return self::create($scenarios);
    }

    /** @param list<Scenario>|null $scenarios */
    private static function start(string $binary, ?array $scenarios, int $attempts = 3): self
    {
        $lastError = null;

        for ($i = 0; $i < $attempts; $i++) {
            $httpPort = self::getFreePort();
            $apiPort = self::getFreePort();
            $configPath = ConfigWriter::writeToTempFile($httpPort, $apiPort, $scenarios);

            $descriptors = [
                0 => ['pipe', 'r'],
                1 => ['pipe', 'w'],
                2 => ['pipe', 'w'],
            ];

            $process = proc_open(
                [$binary, 'start', '--config', $configPath, '--api-port', (string) $apiPort],
                $descriptors,
                $pipes,
            );

            if (!is_resource($process)) {
                @unlink($configPath);
                throw new RuntimeException("Failed to start mockly binary at {$binary}");
            }
            fclose($pipes[0]);
            stream_set_blocking($pipes[2], false);

            $server = new self($process, $httpPort, $apiPort, $configPath);

            try {
                $server->waitReady();
                fclose($pipes[1]);
                fclose($pipes[2]);

                return $server;
            } catch (RuntimeException $exc) {
                $status = proc_get_status($process);
                $stderr = stream_get_contents($pipes[2]) ?: '';
                fclose($pipes[1]);
                fclose($pipes[2]);

                if (!$status['running'] && ConfigWriter::isPortConflict($stderr)) {
                    $lastError = $exc;
                    proc_close($process);
                    @unlink($configPath);
                    continue;
                }

                proc_terminate($process);
                proc_close($process);
                @unlink($configPath);
                throw $exc;
            }
        }

        throw new RuntimeException("Failed to start mockly after {$attempts} attempts", previous: $lastError);
    }

    private static function getFreePort(): int
    {
        $socket = stream_socket_server('tcp://127.0.0.1:0', $errno, $errstr);
        if ($socket === false) {
            throw new RuntimeException("Failed to allocate a free port: {$errstr}");
        }
        $name = stream_socket_get_name($socket, false);
        fclose($socket);

        return (int) substr($name, strrpos($name, ':') + 1);
    }

    private function waitReady(int $maxMs = 10000): void
    {
        $deadline = microtime(true) + $maxMs / 1000;
        while (microtime(true) < $deadline) {
            $ch = curl_init("{$this->apiBase}/api/protocols");
            curl_setopt_array($ch, [
                CURLOPT_RETURNTRANSFER => true,
                CURLOPT_TIMEOUT_MS => 1000,
                CURLOPT_NOBODY => true,
            ]);
            curl_exec($ch);
            $ok = curl_errno($ch) === 0;
            curl_close($ch);
            if ($ok) {
                return;
            }
            usleep(50_000);
        }

        throw new RuntimeException("Mockly did not become ready within {$maxMs}ms (api={$this->apiBase})");
    }

    /** Kills the Mockly process and cleans up the config file. */
    public function stop(): void
    {
        if (is_resource($this->process)) {
            $status = proc_get_status($this->process);
            if ($status['running']) {
                proc_terminate($this->process);
                proc_close($this->process);
            } else {
                proc_close($this->process);
            }
        }
        @unlink($this->configPath);
    }

    public function addMock(Mock $mock): void
    {
        $this->request('POST', '/api/mocks/http', self::mockToArray($mock), [200, 201]);
    }

    /** @return list<Mock> */
    public function listMocks(): array
    {
        $data = $this->requestJson('GET', '/api/mocks/http', expected: [200]);

        return array_map(self::parseMock(...), $data);
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

    /** @return list<CallEntry> */
    public function getLogs(?string $matchedId = null): array
    {
        $path = self::withOptionalMatchedId('/api/logs', $matchedId);

        return array_map(self::parseCallEntry(...), $this->requestJson('GET', $path, expected: [200]));
    }

    public function clearLogs(): void
    {
        $this->request('DELETE', '/api/logs', expected: [200]);
    }

    public function getLogsCount(?string $matchedId = null): int
    {
        $path = self::withOptionalMatchedId('/api/logs/count', $matchedId);
        $data = $this->requestJson('GET', $path, expected: [200]);

        return (int) ($data['count'] ?? 0);
    }

    /** @return list<Scenario> */
    public function listScenarios(): array
    {
        return array_map(self::parseScenario(...), $this->requestJson('GET', '/api/scenarios', expected: [200]));
    }

    public function createScenario(Scenario $scenario): Scenario
    {
        $data = $this->requestJson('POST', '/api/scenarios', self::scenarioToArray($scenario), [201]);

        return self::parseScenario($data);
    }

    public function getScenario(string $scenarioId): Scenario
    {
        $path = '/api/scenarios/' . rawurlencode($scenarioId);

        return self::parseScenario($this->requestJson('GET', $path, expected: [200]));
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

    public function setFault(FaultConfig $config): void
    {
        $this->request('POST', '/api/fault/http', self::faultConfigToArray($config), [200]);
    }

    public function clearFault(): void
    {
        $this->request('DELETE', '/api/fault', expected: [200, 204]);
    }

    public function getCalls(string $mockId): CallSummary
    {
        $path = '/api/calls/http/' . rawurlencode($mockId);

        return self::parseCallSummary($this->requestJson('GET', $path, expected: [200]));
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
        $url = "{$this->apiBase}{$path}";
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
        $request = [
            'method' => $mock->request->method,
            'path' => $mock->request->path,
        ];
        if (!empty($mock->request->headers)) {
            $request['headers'] = $mock->request->headers;
        }

        return [
            'id' => $mock->id,
            'request' => $request,
            'response' => self::mockResponseToArray($mock->response),
        ];
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
