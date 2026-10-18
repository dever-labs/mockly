<?php

declare(strict_types=1);

namespace Mockly\Testcontainers\Tests;

use Mockly\Testcontainers\Model\Mock;
use Mockly\Testcontainers\Model\MockRequest;
use Mockly\Testcontainers\Model\MockResponse;
use Mockly\Testcontainers\Model\Scenario;
use Mockly\Testcontainers\Model\ScenarioPatch;
use Mockly\Testcontainers\MocklyContainer;
use PHPUnit\Framework\Attributes\Group;
use PHPUnit\Framework\TestCase;

/**
 * Requires a running Docker daemon.
 *
 * Run with: vendor/bin/phpunit --group integration
 * Excluded from `composer test` (see composer.json's `test` script).
 */
#[Group('integration')]
final class MocklyContainerIntegrationTest extends TestCase
{
    private static MocklyContainer $container;

    public static function setUpBeforeClass(): void
    {
        self::$container = (new MocklyContainer())->start();
    }

    public static function tearDownAfterClass(): void
    {
        self::$container->stop();
    }

    protected function tearDown(): void
    {
        self::$container->reset();
    }

    public function testSmoke(): void
    {
        self::assertStringStartsWith('http://', self::$container->getHttpBase());
        self::assertStringStartsWith('http://', self::$container->getApiBase());
    }

    public function testAddMockAndFetch(): void
    {
        self::$container->addMock(new Mock(
            id: 'hello-mock',
            request: new MockRequest(method: 'GET', path: '/hello'),
            response: new MockResponse(status: 200, body: 'world'),
        ));

        $body = file_get_contents(self::$container->getHttpBase() . '/hello');

        self::assertSame('world', $body);
    }

    public function testScenarioActivation(): void
    {
        self::$container->addMock(new Mock(
            id: 'ping',
            request: new MockRequest(method: 'GET', path: '/ping'),
            response: new MockResponse(status: 200, body: 'ok'),
        ));
        self::$container->createScenario(new Scenario(
            id: 'maintenance',
            name: 'Maintenance',
            patches: [new ScenarioPatch(mockId: 'ping', status: 503)],
        ));

        self::$container->activateScenario('maintenance');
        $context = stream_context_create(['http' => ['ignore_errors' => true]]);
        file_get_contents(self::$container->getHttpBase() . '/ping', context: $context);
        self::assertStringContainsString('503', $http_response_header[0]);

        self::$container->deactivateScenario('maintenance');
    }

    public function testFaultInjection(): void
    {
        self::$container->addMock(new Mock(
            id: 'fault-mock',
            request: new MockRequest(method: 'GET', path: '/fault'),
            response: new MockResponse(status: 200, body: 'ok'),
        ));
        self::$container->setFault(new \Mockly\Testcontainers\Model\FaultConfig(enabled: true, status: 500));

        $context = stream_context_create(['http' => ['ignore_errors' => true]]);
        file_get_contents(self::$container->getHttpBase() . '/fault', context: $context);
        self::assertStringContainsString('500', $http_response_header[0]);

        self::$container->clearFault();
    }
}
