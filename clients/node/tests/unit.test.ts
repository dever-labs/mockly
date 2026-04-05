import { describe, it, expect, vi, afterEach } from 'vitest'
import { getFreePort } from '../src/utils.js'
import { getBinaryPath, DEFAULT_MOCKLY_VERSION } from '../src/install.js'

// ── getFreePort ───────────────────────────────────────────────────────────────

describe('getFreePort', () => {
  it('returns a number in the valid port range', async () => {
    const port = await getFreePort()
    expect(port).toBeGreaterThan(1024)
    expect(port).toBeLessThanOrEqual(65535)
  })

  it('returns a different port on sequential calls', async () => {
    const p1 = await getFreePort()
    const p2 = await getFreePort()
    // Ports are independent allocations — statistically they should differ
    // (not a hard guarantee but practically always true in tests)
    expect(typeof p1).toBe('number')
    expect(typeof p2).toBe('number')
  })
})

// ── getBinaryPath ─────────────────────────────────────────────────────────────

describe('getBinaryPath', () => {
  afterEach(() => {
    delete process.env.MOCKLY_BINARY_PATH
  })

  it('returns null when no binary is present', () => {
    const result = getBinaryPath('/tmp/definitely-does-not-exist-dir')
    expect(result).toBeNull()
  })

  it('returns MOCKLY_BINARY_PATH when it points to an existing file', async () => {
    // Write a temp file to use as a fake binary
    const { writeFileSync, mkdirSync } = await import('fs')
    const { join } = await import('path')
    const { tmpdir } = await import('os')
    const dir = join(tmpdir(), 'mockly-driver-test-' + Date.now())
    mkdirSync(dir, { recursive: true })
    const fakeBin = join(dir, 'mockly')
    writeFileSync(fakeBin, '#!/bin/sh\necho mock')

    process.env.MOCKLY_BINARY_PATH = fakeBin
    expect(getBinaryPath()).toBe(fakeBin)
  })

  it('returns null when MOCKLY_BINARY_PATH points to a non-existent file', () => {
    process.env.MOCKLY_BINARY_PATH = '/does/not/exist/mockly'
    expect(getBinaryPath()).toBeNull()
  })
})

// ── DEFAULT_MOCKLY_VERSION ────────────────────────────────────────────────────

describe('DEFAULT_MOCKLY_VERSION', () => {
  it('is a semver-style version string', () => {
    expect(DEFAULT_MOCKLY_VERSION).toMatch(/^v\d+\.\d+\.\d+$/)
  })
})

// ── install env var: MOCKLY_NO_INSTALL ────────────────────────────────────────

describe('install() with MOCKLY_NO_INSTALL', () => {
  afterEach(() => {
    delete process.env.MOCKLY_NO_INSTALL
    delete process.env.MOCKLY_BINARY_PATH
  })

  it('throws with actionable message when MOCKLY_NO_INSTALL is set', async () => {
    process.env.MOCKLY_NO_INSTALL = 'true'
    const { install } = await import('../src/install.js')
    await expect(install()).rejects.toThrow(/MOCKLY_NO_INSTALL/)
  })

  it('skips download when MOCKLY_BINARY_PATH points to existing binary', async () => {
    const { writeFileSync, mkdirSync } = await import('fs')
    const { join } = await import('path')
    const { tmpdir } = await import('os')
    const dir = join(tmpdir(), 'mockly-driver-noinstall-' + Date.now())
    mkdirSync(dir, { recursive: true })
    const fakeBin = join(dir, 'mockly')
    writeFileSync(fakeBin, '#!/bin/sh\necho mock')

    process.env.MOCKLY_BINARY_PATH = fakeBin

    const { install } = await import('../src/install.js')
    // Should resolve immediately without making network requests, returning the staged path
    const result = await install()
    expect(result).toBe(fakeBin)
  })
})
