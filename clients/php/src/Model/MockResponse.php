<?php

declare(strict_types=1);

namespace Mockly\Driver\Model;

/** Canned response for an HTTP mock. */
final class MockResponse
{
    /** @param array<string, string> $headers */
    public function __construct(
        public readonly int $status,
        public readonly string $body = '',
        public readonly array $headers = [],
        public readonly ?string $delay = null, // e.g. "50ms"
    ) {
    }
}
