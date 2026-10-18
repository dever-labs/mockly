<?php

declare(strict_types=1);

namespace Mockly\Driver\Tests;

use Mockly\Driver\Installer;
use PHPUnit\Framework\TestCase;
use RuntimeException;

final class InstallerTest extends TestCase
{
    private array $envBackup = [];

    protected function setUp(): void
    {
        foreach (['MOCKLY_BINARY_PATH', 'MOCKLY_NO_INSTALL', 'MOCKLY_VERSION', 'MOCKLY_DOWNLOAD_BASE_URL'] as $key) {
            $this->envBackup[$key] = getenv($key);
            putenv($key);
        }
    }

    protected function tearDown(): void
    {
        foreach ($this->envBackup as $key => $value) {
            putenv($value === false ? $key : "{$key}={$value}");
        }
    }

    public function testGetBinaryPathReturnsNullWhenMissing(): void
    {
        $dir = sys_get_temp_dir() . '/mockly-installer-test-' . uniqid();
        self::assertNull(Installer::getBinaryPath($dir));
    }

    public function testGetBinaryPathFindsExecutableInBinDir(): void
    {
        $dir = sys_get_temp_dir() . '/mockly-installer-test-' . uniqid();
        mkdir($dir, 0o755, true);
        $binary = $dir . '/mockly';
        file_put_contents($binary, '#!/bin/sh');
        chmod($binary, 0o755);

        self::assertSame($binary, Installer::getBinaryPath($dir));

        unlink($binary);
        rmdir($dir);
    }

    public function testGetBinaryPathPrefersEnvVar(): void
    {
        $tmp = tempnam(sys_get_temp_dir(), 'mockly-env-');
        putenv("MOCKLY_BINARY_PATH={$tmp}");

        self::assertSame($tmp, Installer::getBinaryPath('/nonexistent'));

        unlink($tmp);
    }

    public function testGetBinaryPathEnvVarMissingFileReturnsNull(): void
    {
        putenv('MOCKLY_BINARY_PATH=/nonexistent/mockly');

        self::assertNull(Installer::getBinaryPath());
    }

    public function testInstallThrowsWhenNoInstallSet(): void
    {
        putenv('MOCKLY_NO_INSTALL=1');

        $this->expectException(RuntimeException::class);
        $this->expectExceptionMessageMatches('/MOCKLY_NO_INSTALL/');

        Installer::install();
    }

    public function testInstallReturnsExistingBinaryViaEnv(): void
    {
        $tmp = tempnam(sys_get_temp_dir(), 'mockly-env-');
        putenv("MOCKLY_BINARY_PATH={$tmp}");

        self::assertSame($tmp, Installer::install());

        unlink($tmp);
    }

    public function testInstallReturnsStagedBinaryWithoutDownloading(): void
    {
        $dir = sys_get_temp_dir() . '/mockly-installer-test-' . uniqid();
        mkdir($dir, 0o755, true);
        $binary = $dir . '/mockly';
        file_put_contents($binary, '#!/bin/sh');

        self::assertSame($binary, Installer::install(binDir: $dir));

        unlink($binary);
        rmdir($dir);
    }
}
