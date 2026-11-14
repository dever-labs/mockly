<?php

declare(strict_types=1);

namespace Mockly\Driver;

use RuntimeException;

/** Binary download and location helpers for mockly-driver. */
final class Installer
{
    public const DEFAULT_VERSION = 'v0.17.0'; // x-release-please-version

    private const DEFAULT_BASE_URL = 'https://github.com/dever-labs/mockly/releases/download';

    private function __construct()
    {
    }

    private static function exeSuffix(): string
    {
        return PHP_OS_FAMILY === 'Windows' ? '.exe' : '';
    }

    private static function assetName(): string
    {
        $machine = strtolower(php_uname('m'));
        $arch = match (true) {
            in_array($machine, ['x86_64', 'amd64'], true) => 'amd64',
            in_array($machine, ['arm64', 'aarch64'], true) => 'arm64',
            default => throw new RuntimeException("Unsupported architecture: {$machine}"),
        };

        $osName = match (PHP_OS_FAMILY) {
            'Linux' => 'linux',
            'Darwin' => 'darwin',
            'Windows' => 'windows',
            default => throw new RuntimeException('Unsupported OS: ' . PHP_OS_FAMILY),
        };

        $suffix = $osName === 'windows' ? '.exe' : '';

        return "mockly-{$osName}-{$arch}{$suffix}";
    }

    /**
     * Returns the path to an existing mockly binary, or null if not found.
     *
     * Search order:
     * 1. MOCKLY_BINARY_PATH environment variable (if set and file exists)
     * 2. <binDir>/mockly[.exe] (if binDir provided and file exists)
     * 3. <cwd>/bin/mockly[.exe]
     */
    public static function getBinaryPath(?string $binDir = null): ?string
    {
        $envPath = getenv('MOCKLY_BINARY_PATH');
        if ($envPath !== false && $envPath !== '') {
            return is_file($envPath) ? $envPath : null;
        }

        $name = 'mockly' . self::exeSuffix();

        if ($binDir !== null) {
            $candidate = rtrim($binDir, '/') . '/' . $name;
            if (is_file($candidate)) {
                return $candidate;
            }
        }

        $cwdCandidate = getcwd() . '/bin/' . $name;
        if (is_file($cwdCandidate)) {
            return $cwdCandidate;
        }

        return null;
    }

    /**
     * Downloads the mockly binary and returns its path.
     *
     * Respects:
     * - MOCKLY_NO_INSTALL         — throw instead of downloading
     * - MOCKLY_BINARY_PATH        — return immediately if the file exists
     * - MOCKLY_VERSION            — version override
     * - MOCKLY_DOWNLOAD_BASE_URL  — base URL override
     * - HTTPS_PROXY / HTTP_PROXY  — proxy for downloads
     */
    public static function install(
        ?string $version = null,
        ?string $baseUrl = null,
        ?string $binDir = null,
        bool $force = false,
    ): string {
        if (getenv('MOCKLY_NO_INSTALL') !== false) {
            throw new RuntimeException(
                'MOCKLY_NO_INSTALL is set; refusing to download the mockly binary. ' .
                'Provide a pre-staged binary via MOCKLY_BINARY_PATH or disable MOCKLY_NO_INSTALL.'
            );
        }

        $envPath = getenv('MOCKLY_BINARY_PATH');
        if ($envPath !== false && $envPath !== '' && is_file($envPath)) {
            return $envPath;
        }

        $version ??= getenv('MOCKLY_VERSION') ?: self::DEFAULT_VERSION;
        $baseUrl ??= getenv('MOCKLY_DOWNLOAD_BASE_URL') ?: self::DEFAULT_BASE_URL;

        $name = 'mockly' . self::exeSuffix();
        $destDir = $binDir ?? (getcwd() . '/bin');
        $destPath = rtrim($destDir, '/') . '/' . $name;

        if (!$force && is_file($destPath)) {
            return $destPath;
        }

        $asset = self::assetName();
        $url = "{$baseUrl}/{$version}/{$asset}";

        if (!is_dir($destDir) && !mkdir($destDir, 0o755, true) && !is_dir($destDir)) {
            throw new RuntimeException("Failed to create directory: {$destDir}");
        }

        fwrite(STDERR, "Downloading mockly {$version} from {$url} …\n");

        $ch = curl_init($url);
        $out = fopen($destPath, 'wb');
        if ($out === false) {
            throw new RuntimeException("Failed to open {$destPath} for writing");
        }

        curl_setopt_array($ch, [
            CURLOPT_FILE => $out,
            CURLOPT_FOLLOWLOCATION => true,
            CURLOPT_FAILONERROR => true,
            CURLOPT_TIMEOUT => 120,
        ]);

        $proxy = getenv('HTTPS_PROXY') ?: getenv('HTTP_PROXY');
        if ($proxy !== false && $proxy !== '') {
            curl_setopt($ch, CURLOPT_PROXY, $proxy);
        }

        $ok = curl_exec($ch);
        $error = curl_error($ch);
        curl_close($ch);
        fclose($out);

        if ($ok === false) {
            @unlink($destPath);
            throw new RuntimeException("Failed to download mockly binary from {$url}: {$error}");
        }

        if (PHP_OS_FAMILY !== 'Windows') {
            chmod($destPath, 0o755);
        }

        return $destPath;
    }
}
