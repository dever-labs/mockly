<?php

declare(strict_types=1);

namespace Mockly\Driver\Model;

/** A single recorded request against a mocked HTTP endpoint. */
final class CallEntry
{
    /** @param array<string, string> $headers
     *  @param array<string, string> $pathParams */
    public function __construct(
        public readonly string $id,
        public readonly string $timestamp,
        public readonly string $protocol,
        public readonly string $path,
        public readonly int $durationMs,
        public readonly ?string $method = null,
        public readonly ?int $status = null,
        public readonly array $headers = [],
        public readonly ?string $body = null,
        public readonly ?string $matchedId = null,
        public readonly array $pathParams = [],
    ) {
    }
}
