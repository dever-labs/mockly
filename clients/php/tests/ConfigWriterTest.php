<?php

declare(strict_types=1);

namespace Mockly\Driver\Tests;

use Mockly\Driver\Internal\ConfigWriter;
use Mockly\Driver\Model\Scenario;
use Mockly\Driver\Model\ScenarioPatch;
use PHPUnit\Framework\TestCase;

final class ConfigWriterTest extends TestCase
{
    public function testYamlStrPlain(): void
    {
        self::assertSame('"hello"', ConfigWriter::yamlStr('hello'));
    }

    public function testYamlStrDoubleQuote(): void
    {
        self::assertSame('"say \\"hi\\""', ConfigWriter::yamlStr('say "hi"'));
    }

    public function testYamlStrBackslash(): void
    {
        self::assertSame('"a\\\\b"', ConfigWriter::yamlStr('a\\b'));
    }

    public function testYamlStrEmpty(): void
    {
        self::assertSame('""', ConfigWriter::yamlStr(''));
    }

    public function testYamlStrNewline(): void
    {
        self::assertSame('"line1\\nline2"', ConfigWriter::yamlStr("line1\nline2"));
    }

    public function testRenderContainsPortKeys(): void
    {
        $yaml = ConfigWriter::render(8080, 9090, null);
        self::assertStringContainsString('port: 9090', $yaml);
        self::assertStringContainsString('port: 8080', $yaml);
    }

    public function testRenderNoScenariosSectionWhenNull(): void
    {
        $yaml = ConfigWriter::render(8080, 9090, null);
        self::assertStringNotContainsString('scenarios:', $yaml);
    }

    public function testRenderEmptyListOmitsScenarios(): void
    {
        $yaml = ConfigWriter::render(8080, 9090, []);
        self::assertStringNotContainsString('scenarios:', $yaml);
    }

    public function testRenderIncludesScenarioAndPatchFields(): void
    {
        $yaml = ConfigWriter::render(8080, 9090, [
            new Scenario(
                id: 'maintenance',
                name: 'Maintenance mode',
                patches: [
                    new ScenarioPatch(mockId: 'get-user', status: 503, delay: '5ms', disabled: false),
                ],
                description: 'Returns 503',
            ),
        ]);

        self::assertStringContainsString('scenarios:', $yaml);
        self::assertStringContainsString('id: "maintenance"', $yaml);
        self::assertStringContainsString('name: "Maintenance mode"', $yaml);
        self::assertStringContainsString('description: "Returns 503"', $yaml);
        self::assertStringContainsString('mock_id: "get-user"', $yaml);
        self::assertStringContainsString('status: 503', $yaml);
        self::assertStringContainsString('delay: "5ms"', $yaml);
        self::assertStringContainsString('disabled: false', $yaml);
    }

    public function testWriteToTempFileReturnsExistingFile(): void
    {
        $path = ConfigWriter::writeToTempFile(8080, 9090, null);
        self::assertFileExists($path);
        self::assertStringContainsString('port: 8080', file_get_contents($path));
        unlink($path);
    }

    /** @dataProvider portConflictMessages */
    public function testIsPortConflict(string $message, bool $expected): void
    {
        self::assertSame($expected, ConfigWriter::isPortConflict($message));
    }

    /** @return iterable<string, array{string, bool}> */
    public static function portConflictMessages(): iterable
    {
        yield 'address already in use' => ['listen tcp 127.0.0.1:8080: address already in use', true];
        yield 'EADDRINUSE' => ['Error: EADDRINUSE', true];
        yield 'EADDRINUSE lowercase' => ['error: eaddrinuse', true];
        yield 'bind prefix' => ['bind: permission denied', true];
        yield 'connection refused' => ['dial tcp: connection refused', false];
        yield 'timeout' => ['context deadline exceeded', false];
        yield 'empty' => ['', false];
    }
}
