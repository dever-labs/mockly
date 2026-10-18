<?php

declare(strict_types=1);

namespace Mockly\Driver\Model;

/** Injects latency and/or error responses across all requests until cleared. */
final class FaultConfig
{
    public function __construct(
        public readonly bool $enabled,
        public readonly ?string $delay = null, // e.g. "200ms"
        public readonly ?int $status = null,
        public readonly ?float $errorRate = null, // 0.0-1.0
    ) {
    }
}
