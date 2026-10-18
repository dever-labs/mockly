<?php

declare(strict_types=1);

namespace Mockly\Testcontainers\Internal;

use Mockly\Testcontainers\Model\Scenario;

/**
 * Renders the Mockly YAML config copied into the container before starting it.
 * Kept separate from {@see \Mockly\Testcontainers\MocklyContainer} so it's independently
 * unit-testable.
 *
 * @internal
 */
final class ConfigRenderer
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

    /** Wraps a string in double-quotes with minimal escaping for YAML. */
    public static function yamlStr(string $value): string
    {
        $escaped = str_replace(['\\', '"', "\n"], ['\\\\', '\\"', '\\n'], $value);

        return "\"{$escaped}\"";
    }
}
