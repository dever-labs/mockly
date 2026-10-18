<?php

declare(strict_types=1);

namespace Mockly\Testcontainers\Model;

/** A named set of {@see ScenarioPatch} overrides that can be activated/deactivated as a unit. */
final class Scenario
{
    /** @param list<ScenarioPatch> $patches */
    public function __construct(
        public readonly string $id,
        public readonly string $name,
        public readonly array $patches = [],
        public readonly ?string $description = null,
    ) {
    }
}
