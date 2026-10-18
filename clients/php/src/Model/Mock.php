<?php

declare(strict_types=1);

namespace Mockly\Driver\Model;

/** A single HTTP mock: an id, a request matcher, and a canned response. */
final class Mock
{
    public function __construct(
        public readonly string $id,
        public readonly MockRequest $request,
        public readonly MockResponse $response,
    ) {
    }
}
