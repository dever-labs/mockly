<?php

declare(strict_types=1);

// Router for PHP's built-in web server (`php -S`), used by FakeHttpServer in tests
// to fake Mockly's management API without needing a real binary. Request/response
// state is exchanged via files (since `php -S` forks a fresh process per request)
// at paths given through MOCKLY_TEST_STATUS_FILE / MOCKLY_TEST_BODY_FILE /
// MOCKLY_TEST_LOG_FILE environment variables, set by FakeHttpServer::start().

$statusFile = getenv('MOCKLY_TEST_STATUS_FILE');
$bodyFile = getenv('MOCKLY_TEST_BODY_FILE');
$logFile = getenv('MOCKLY_TEST_LOG_FILE');

$status = (int) (is_file($statusFile) ? trim((string) file_get_contents($statusFile)) : 200) ?: 200;
$body = is_file($bodyFile) ? (string) file_get_contents($bodyFile) : '';

$uri = $_SERVER['REQUEST_URI'] ?? '/';
$path = parse_url($uri, PHP_URL_PATH) ?? $uri;
$requestBody = file_get_contents('php://input') ?: '';

$entry = [
    'method' => $_SERVER['REQUEST_METHOD'] ?? 'GET',
    'path' => $path,
    'full_path' => $uri,
    'body' => $requestBody,
];
file_put_contents($logFile, json_encode($entry) . "\n", FILE_APPEND | LOCK_EX);

http_response_code($status);
header('Content-Type: application/json');
echo $body;
