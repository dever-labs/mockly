<?php

declare(strict_types=1);

namespace Mockly\Driver\Model;

/** Partial response override used by {@see \Mockly\Driver\MocklyServer::patchMock()}. */
final class MockResponsePatch
{
    /** @param array<string, string>|null $headers */
    public function __construct(
        public readonly ?int $status = null,
        public readonly ?string $body = null,
        public readonly ?array $headers = null,
        public readonly ?string $delay = null,
    ) {
    }
}
