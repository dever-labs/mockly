<?php

declare(strict_types=1);

namespace Mockly\Driver\Model;

/** Summary of recorded calls for a specific HTTP mock. */
final class CallSummary
{
    /** @param list<CallEntry> $calls */
    public function __construct(
        public readonly string $mockId,
        public readonly int $count,
        public readonly array $calls = [],
    ) {
    }
}
