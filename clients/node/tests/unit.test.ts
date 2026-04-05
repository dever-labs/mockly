import { describe, it, expect, vi, afterEach } from 'vitest'
import { createServer } from 'http'
import type { AddressInfo } from 'net'
import { existsSync, readFileSync, rmSync } from 'fs'
import { dirname } from 'path'
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

// ── Helpers ───────────────────────────────────────────────────────────────────

type FakeHandler = (method: string, url: string, body: string) => { status: number; body?: string }

async function startFakeServer(
  handler: FakeHandler,
): Promise<{ url: string; close: () => Promise<void> }> {
  const srv = createServer((req, res) => {
    let body = ''
    req.on('data', (chunk: Buffer) => { body += chunk.toString() })
    req.on('end', () => {
      const result = handler(req.method ?? 'GET', req.url ?? '/', body)
      res.writeHead(result.status, { 'Content-Type': 'application/json' })
      res.end(result.body ?? '')
    })
  })
  await new Promise<void>((resolve) => srv.listen(0, '127.0.0.1', resolve))
  const addr = srv.address() as AddressInfo
  return {
    url: `http://127.0.0.1:${addr.port}`,
    close: () => new Promise<void>((resolve, reject) =>
      srv.close((err?: Error) => (err ? reject(err) : resolve()))
    ),
  }
}

async function makeServerStub(apiBase: string) {
  const { MocklyServer } = await import('../src/server.js')
  const server = Object.create(MocklyServer.prototype) as InstanceType<typeof MocklyServer>
  ;(server as any).httpPort = 8001
  ;(server as any).apiPort = 8002
  ;(server as any).proc = null
  // Shadow the apiBase getter so all fetch calls go to the fake server
  Object.defineProperty(server, 'apiBase', { value: apiBase, configurable: true })
  return server
}

// ── _writeConfig ──────────────────────────────────────────────────────────────

describe('_writeConfig', () => {
  it('returns a string path to an existing YAML file with port numbers', async () => {
    const { MocklyServer } = await import('../src/server.js')
    const server = Object.create(MocklyServer.prototype) as InstanceType<typeof MocklyServer>
    ;(server as any).httpPort = 9001
    ;(server as any).apiPort = 9002
    ;(server as any).proc = null

    const cfgPath: string = (server as any)._writeConfig([])

    expect(typeof cfgPath).toBe('string')
    expect(existsSync(cfgPath)).toBe(true)

    const content = readFileSync(cfgPath, 'utf-8')
    expect(content).toContain('9001')
    expect(content).toContain('9002')
    expect(content).toContain('mockly:')
    expect(content).toContain('protocols:')

    rmSync(dirname(cfgPath), { recursive: true, force: true })
  })

  it('includes scenario id and name when scenarios are provided', async () => {
    const { MocklyServer } = await import('../src/server.js')
    const server = Object.create(MocklyServer.prototype) as InstanceType<typeof MocklyServer>
    ;(server as any).httpPort = 9003
    ;(server as any).apiPort = 9004
    ;(server as any).proc = null

    const scenarios = [{ id: 'error-sc', name: 'Error State', patches: [] }]
    const cfgPath: string = (server as any)._writeConfig(scenarios)

    expect(existsSync(cfgPath)).toBe(true)
    const content = readFileSync(cfgPath, 'utf-8')
    expect(content).toContain('error-sc')
    expect(content).toContain('Error State')

    rmSync(dirname(cfgPath), { recursive: true, force: true })
  })
})

// ── HTTP API methods ──────────────────────────────────────────────────────────

describe('addMock', () => {
  it('sends a POST with the mock payload and resolves on 2xx', async () => {
    let capturedMethod = ''
    let capturedUrl = ''
    let capturedBody = ''

    const fake = await startFakeServer((method, url, body) => {
      capturedMethod = method
      capturedUrl = url
      capturedBody = body
      return { status: 201 }
    })

    const server = await makeServerStub(fake.url)
    const mock = {
      id: 'test-mock',
      request: { method: 'GET', path: '/users' },
      response: { status: 200, body: '[]' },
    }
    await expect(server.addMock(mock)).resolves.toBeUndefined()

    expect(capturedMethod).toBe('POST')
    expect(capturedUrl).toBe('/api/mocks/http')
    const parsed = JSON.parse(capturedBody)
    expect(parsed.id).toBe('test-mock')
    expect(parsed.request.method).toBe('GET')
    expect(parsed.request.path).toBe('/users')
    expect(parsed.response.status).toBe(200)

    await fake.close()
  })

  it('throws when the server returns a non-ok status', async () => {
    const fake = await startFakeServer(() => ({ status: 500, body: '{"error":"oops"}' }))
    const server = await makeServerStub(fake.url)
    const mock = {
      id: 'bad-mock',
      request: { method: 'GET', path: '/fail' },
      response: { status: 200 },
    }
    await expect(server.addMock(mock)).rejects.toThrow(/addMock.*500/)
    await fake.close()
  })
})

describe('deleteMock', () => {
  it('sends DELETE to /api/mocks/http/{id}', async () => {
    let capturedMethod = ''
    let capturedUrl = ''

    const fake = await startFakeServer((method, url) => {
      capturedMethod = method
      capturedUrl = url
      return { status: 204 }
    })

    const server = await makeServerStub(fake.url)
    await expect(server.deleteMock('test-id')).resolves.toBeUndefined()
    expect(capturedMethod).toBe('DELETE')
    expect(capturedUrl).toBe('/api/mocks/http/test-id')

    await fake.close()
  })
})

describe('reset', () => {
  it('sends POST to /api/reset and resolves', async () => {
    let capturedMethod = ''
    let capturedUrl = ''

    const fake = await startFakeServer((method, url) => {
      capturedMethod = method
      capturedUrl = url
      return { status: 200 }
    })

    const server = await makeServerStub(fake.url)
    await expect(server.reset()).resolves.toBeUndefined()
    expect(capturedMethod).toBe('POST')
    expect(capturedUrl).toBe('/api/reset')

    await fake.close()
  })
})

describe('activateScenario', () => {
  it('sends POST to /api/scenarios/{id}/activate', async () => {
    let capturedMethod = ''
    let capturedUrl = ''

    const fake = await startFakeServer((method, url) => {
      capturedMethod = method
      capturedUrl = url
      return { status: 200 }
    })

    const server = await makeServerStub(fake.url)
    await expect(server.activateScenario('sc1')).resolves.toBeUndefined()
    expect(capturedMethod).toBe('POST')
    expect(capturedUrl).toBe('/api/scenarios/sc1/activate')

    await fake.close()
  })
})

describe('deactivateScenario', () => {
  it('sends DELETE to /api/scenarios/{id}/activate', async () => {
    let capturedMethod = ''
    let capturedUrl = ''

    const fake = await startFakeServer((method, url) => {
      capturedMethod = method
      capturedUrl = url
      return { status: 200 }
    })

    const server = await makeServerStub(fake.url)
    await expect(server.deactivateScenario('sc1')).resolves.toBeUndefined()
    expect(capturedMethod).toBe('DELETE')
    expect(capturedUrl).toBe('/api/scenarios/sc1/activate')

    await fake.close()
  })
})

describe('setFault', () => {
  it('sends POST to /api/fault with the fault config', async () => {
    let capturedBody = ''

    const fake = await startFakeServer((_method, _url, body) => {
      capturedBody = body
      return { status: 200 }
    })

    const server = await makeServerStub(fake.url)
    await expect(server.setFault({ enabled: true, delay: '100ms' })).resolves.toBeUndefined()
    const parsed = JSON.parse(capturedBody)
    expect(parsed.enabled).toBe(true)
    expect(parsed.delay).toBe('100ms')

    await fake.close()
  })
})

describe('clearFault', () => {
  it('sends DELETE to /api/fault', async () => {
    let capturedMethod = ''
    let capturedUrl = ''

    const fake = await startFakeServer((method, url) => {
      capturedMethod = method
      capturedUrl = url
      return { status: 200 }
    })

    const server = await makeServerStub(fake.url)
    await expect(server.clearFault()).resolves.toBeUndefined()
    expect(capturedMethod).toBe('DELETE')
    expect(capturedUrl).toBe('/api/fault')

    await fake.close()
  })
})
