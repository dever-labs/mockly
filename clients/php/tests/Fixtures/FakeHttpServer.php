<?php

declare(strict_types=1);

namespace Mockly\Driver\Tests\Fixtures;

/**
 * A fake Mockly management API backed by PHP's built-in web server (`php -S`), used
 * to unit-test {@see \Mockly\Driver\MocklyServer}'s HTTP calls without a real binary.
 */
final class FakeHttpServer
{
    public readonly string $baseUrl;

    /** @var resource */
    private $process;
    private readonly string $statusFile;
    private readonly string $bodyFile;
    private readonly string $logFile;

    private function __construct($process, int $port, string $statusFile, string $bodyFile, string $logFile)
    {
        $this->process = $process;
        $this->baseUrl = "http://127.0.0.1:{$port}";
        $this->statusFile = $statusFile;
        $this->bodyFile = $bodyFile;
        $this->logFile = $logFile;
    }

    public static function start(): self
    {
        $socket = stream_socket_server('tcp://127.0.0.1:0', $errno, $errstr);
        if ($socket === false) {
            throw new \RuntimeException("Failed to allocate a free port: {$errstr}");
        }
        $name = stream_socket_get_name($socket, false);
        $port = (int) substr($name, strrpos($name, ':') + 1);
        fclose($socket);

        $dir = sys_get_temp_dir() . '/mockly-fake-http-' . uniqid();
        mkdir($dir, 0o755, true);
        $statusFile = $dir . '/status.txt';
        $bodyFile = $dir . '/body.txt';
        $logFile = $dir . '/requests.ndjson';
        file_put_contents($statusFile, '200');
        file_put_contents($bodyFile, '');
        file_put_contents($logFile, '');

        $router = __DIR__ . '/router.php';
        $env = array_merge(getenv() ?: [], [
            'MOCKLY_TEST_STATUS_FILE' => $statusFile,
            'MOCKLY_TEST_BODY_FILE' => $bodyFile,
            'MOCKLY_TEST_LOG_FILE' => $logFile,
        ]);

        $process = proc_open(
            [PHP_BINARY, '-S', "127.0.0.1:{$port}", $router],
            [1 => ['pipe', 'w'], 2 => ['pipe', 'w']],
            $pipes,
            null,
            $env,
        );
        if (!is_resource($process)) {
            throw new \RuntimeException('Failed to start PHP built-in server');
        }
        stream_set_blocking($pipes[1], false);
        stream_set_blocking($pipes[2], false);

        $server = new self($process, $port, $statusFile, $bodyFile, $logFile);
        $server->waitReady();

        return $server;
    }

    private function waitReady(): void
    {
        $deadline = microtime(true) + 5;
        while (microtime(true) < $deadline) {
            $conn = @stream_socket_client(str_replace('http://', 'tcp://', $this->baseUrl), $errno, $errstr, 0.2);
            if ($conn !== false) {
                fclose($conn);

                return;
            }
            usleep(20_000);
        }
        throw new \RuntimeException('Fake HTTP server did not become ready in time');
    }

    public function setResponse(int $status = 200, string $body = ''): void
    {
        file_put_contents($this->statusFile, (string) $status);
        file_put_contents($this->bodyFile, $body);
    }

    /** @return list<array{method: string, path: string, full_path: string, body: string}> */
    public function requests(): array
    {
        $lines = array_filter(explode("\n", (string) file_get_contents($this->logFile)));

        return array_map(static fn (string $line) => json_decode($line, true), $lines);
    }

    /** @return array{method: string, path: string, full_path: string, body: string} */
    public function lastRequest(): array
    {
        $requests = $this->requests();
        if ($requests === []) {
            throw new \RuntimeException('No requests recorded');
        }

        return $requests[array_key_last($requests)];
    }

    public function stop(): void
    {
        if (is_resource($this->process)) {
            proc_terminate($this->process);
            proc_close($this->process);
        }
        @unlink($this->statusFile);
        @unlink($this->bodyFile);
        @unlink($this->logFile);
        @rmdir(dirname($this->statusFile));
    }
}
