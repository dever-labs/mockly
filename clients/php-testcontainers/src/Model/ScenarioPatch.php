<?php

declare(strict_types=1);

namespace Mockly\Testcontainers\Model;

/** A response override applied to a specific mock while a {@see Scenario} is active. */
final class ScenarioPatch
{
    /** @param array<string, string>|null $headers */
    public function __construct(
        public readonly string $mockId,
        public readonly ?int $status = null,
        public readonly ?string $body = null,
        public readonly ?array $headers = null,
        public readonly ?string $delay = null,
        public readonly ?bool $disabled = null,
    ) {
    }
}
