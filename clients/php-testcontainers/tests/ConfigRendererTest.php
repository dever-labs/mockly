<?php

declare(strict_types=1);

namespace Mockly\Testcontainers\Tests;

use Mockly\Testcontainers\Internal\ConfigRenderer;
use Mockly\Testcontainers\Model\Scenario;
use Mockly\Testcontainers\Model\ScenarioPatch;
use PHPUnit\Framework\TestCase;

final class ConfigRendererTest extends TestCase
{
    public function testYamlStrPlain(): void
    {
        self::assertSame('"hello"', ConfigRenderer::yamlStr('hello'));
    }

    public function testYamlStrDoubleQuote(): void
    {
        self::assertSame('"say \\"hi\\""', ConfigRenderer::yamlStr('say "hi"'));
    }

    public function testRenderContainsPortKeys(): void
    {
        $yaml = ConfigRenderer::render(8090, 9091, null);
        self::assertStringContainsString('port: 9091', $yaml);
        self::assertStringContainsString('port: 8090', $yaml);
    }

    public function testRenderNoScenariosSectionWhenNull(): void
    {
        self::assertStringNotContainsString('scenarios:', ConfigRenderer::render(8090, 9091, null));
    }

    public function testRenderIncludesScenarioAndPatchFields(): void
    {
        $yaml = ConfigRenderer::render(8090, 9091, [
            new Scenario(
                id: 'maintenance',
                name: 'Maintenance mode',
                patches: [new ScenarioPatch(mockId: 'get-user', status: 503, delay: '5ms', disabled: false)],
                description: 'Returns 503',
            ),
        ]);

        self::assertStringContainsString('id: "maintenance"', $yaml);
        self::assertStringContainsString('name: "Maintenance mode"', $yaml);
        self::assertStringContainsString('description: "Returns 503"', $yaml);
        self::assertStringContainsString('mock_id: "get-user"', $yaml);
        self::assertStringContainsString('status: 503', $yaml);
        self::assertStringContainsString('delay: "5ms"', $yaml);
        self::assertStringContainsString('disabled: false', $yaml);
    }
}
