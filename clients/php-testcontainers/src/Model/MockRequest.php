<?php

declare(strict_types=1);

namespace Mockly\Testcontainers\Model;

/** Request matcher for an HTTP mock. */
final class MockRequest
{
    /** @param array<string, string> $headers */
    public function __construct(
        public readonly string $method,
        public readonly string $path,
        public readonly array $headers = [],
    ) {
    }
}
