<?php

declare(strict_types=1);

namespace Mockly\Testcontainers\Model;

/** Result of {@see \Mockly\Driver\MocklyServer::listActiveScenarios()}. */
final class ActiveScenariosResponse
{
    /**
     * @param list<string> $active
     * @param list<Scenario> $scenarios
     */
    public function __construct(
        public readonly array $active = [],
        public readonly array $scenarios = [],
    ) {
    }
}
