<?php

declare(strict_types=1);

namespace Mockly\Driver\Internal;

use Mockly\Driver\Model\Scenario;

/**
 * Renders the Mockly YAML config written to a temp file before starting a process.
 * Kept separate from {@see \Mockly\Driver\MocklyServer} so it's independently unit-testable.
 *
 * @internal
 */
final class ConfigWriter
{
    private function __construct()
    {
    }

    /** @param list<Scenario>|null $scenarios */
    public static function render(int $httpPort, int $apiPort, ?array $scenarios): string
    {
        $lines = [
            'mockly:',
            '  api:',
            "    port: {$apiPort}",
            'protocols:',
            '  http:',
            '    enabled: true',
            "    port: {$httpPort}",
        ];

        if (!empty($scenarios)) {
            $lines[] = 'scenarios:';
            foreach ($scenarios as $scenario) {
                $lines[] = '  - id: ' . self::yamlStr($scenario->id);
                $lines[] = '    name: ' . self::yamlStr($scenario->name);
                if ($scenario->description !== null) {
                    $lines[] = '    description: ' . self::yamlStr($scenario->description);
                }
                if (!empty($scenario->patches)) {
                    $lines[] = '    patches:';
                    foreach ($scenario->patches as $patch) {
                        $lines[] = '      - mock_id: ' . self::yamlStr($patch->mockId);
                        if ($patch->status !== null) {
                            $lines[] = "        status: {$patch->status}";
                        }
                        if ($patch->body !== null) {
                            $lines[] = '        body: ' . self::yamlStr($patch->body);
                        }
                        if (!empty($patch->headers)) {
                            $lines[] = '        headers:';
                            foreach ($patch->headers as $key => $value) {
                                $lines[] = '          ' . self::yamlStr($key) . ': ' . self::yamlStr($value);
                            }
                        }
                        if ($patch->delay !== null) {
                            $lines[] = '        delay: ' . self::yamlStr($patch->delay);
                        }
                        if ($patch->disabled !== null) {
                            $lines[] = '        disabled: ' . ($patch->disabled ? 'true' : 'false');
                        }
                    }
                }
            }
        }

        return implode("\n", $lines) . "\n";
    }

    /** @param list<Scenario>|null $scenarios */
    public static function writeToTempFile(int $httpPort, int $apiPort, ?array $scenarios): string
    {
        $path = tempnam(sys_get_temp_dir(), 'mockly-') . '.yaml';
        file_put_contents($path, self::render($httpPort, $apiPort, $scenarios));

        return $path;
    }

    /** Wraps a string in double-quotes with minimal escaping for YAML. */
    public static function yamlStr(string $value): string
    {
        $escaped = str_replace(['\\', '"', "\n"], ['\\\\', '\\"', '\\n'], $value);

        return "\"{$escaped}\"";
    }

    public static function isPortConflict(string $errorMsg): bool
    {
        $msg = strtolower($errorMsg);
        foreach (['address already in use', 'eaddrinuse', 'bind:'] as $token) {
            if (str_contains($msg, $token)) {
                return true;
            }
        }

        return false;
    }
}
