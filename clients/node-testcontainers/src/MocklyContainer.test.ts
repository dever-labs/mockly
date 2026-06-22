import { afterEach, describe, expect, it, vi } from 'vitest'

const genericContainerMocks = vi.hoisted(() => {
  const startedContainer = {
    getHost: () => '127.0.0.1',
    getMappedPort: (port: number) => (port === 8090 ? 38090 : 39091),
    stop: vi.fn(),
  }

  const chain = {
    withExposedPorts: vi.fn(),
    withCommand: vi.fn(),
    withCopyContentToContainer: vi.fn(),
    withWaitStrategy: vi.fn(),
    start: vi.fn(),
  }

  chain.withExposedPorts.mockReturnValue(chain)
  chain.withCommand.mockReturnValue(chain)
  chain.withCopyContentToContainer.mockReturnValue(chain)
  chain.withWaitStrategy.mockReturnValue(chain)
  chain.start.mockResolvedValue(startedContainer)

  const GenericContainer = vi.fn(function (this: unknown) {
    return chain
  })
  const waitStrategy = { withStartupTimeout: vi.fn().mockReturnValue('wait-strategy') }
  const Wait = {
    forHttp: vi.fn(() => waitStrategy),
  }

  return { GenericContainer, Wait, chain, waitStrategy, startedContainer }
})

vi.mock('testcontainers', () => ({
  GenericContainer: genericContainerMocks.GenericContainer,
  Wait: genericContainerMocks.Wait,
}))

import {
  API_PORT,
  CONTAINER_CONFIG_PATH,
  DEFAULT_IMAGE,
  HTTP_PORT,
  MocklyContainerBuilder,
  StartedMocklyContainer,
} from './index.js'
import type { HttpMock } from './types.js'

afterEach(() => {
  vi.unstubAllGlobals()
  vi.restoreAllMocks()
  genericContainerMocks.GenericContainer.mockClear()
  genericContainerMocks.Wait.forHttp.mockClear()
  genericContainerMocks.waitStrategy.withStartupTimeout.mockClear()
  genericContainerMocks.chain.withExposedPorts.mockClear()
  genericContainerMocks.chain.withCommand.mockClear()
  genericContainerMocks.chain.withCopyContentToContainer.mockClear()
  genericContainerMocks.chain.withWaitStrategy.mockClear()
  genericContainerMocks.chain.start.mockClear()
  genericContainerMocks.chain.start.mockResolvedValue(genericContainerMocks.startedContainer)
})

describe('MocklyContainerBuilder', () => {
  it('sets correct default image', () => {
    expect(DEFAULT_IMAGE).toBe('ghcr.io/dever-labs/mockly:latest')
  })

  it('sets correct ports', () => {
    expect(HTTP_PORT).toBe(8090)
    expect(API_PORT).toBe(9091)
  })

  it('withInlineConfig stores config', () => {
    const builder = new MocklyContainerBuilder().withInlineConfig('mockly: {}') as MocklyContainerBuilder & {
      inlineConfig?: string
    }

    expect(builder.inlineConfig).toBe('mockly: {}')
  })

  it('start copies inline config and uses config path', async () => {
    const builder = new MocklyContainerBuilder()
      .withImage('ghcr.io/dever-labs/mockly:test')
      .withInlineConfig('mockly:\n  api:\n    port: 1234\n')

    const container = await builder.start()

    expect(genericContainerMocks.GenericContainer).toHaveBeenCalledWith('ghcr.io/dever-labs/mockly:test')
    expect(genericContainerMocks.chain.withCommand).toHaveBeenCalledWith(['start', '-c', CONTAINER_CONFIG_PATH])
    expect(genericContainerMocks.chain.withCopyContentToContainer).toHaveBeenCalledWith([
      {
        content: Buffer.from('mockly:\n  api:\n    port: 1234\n'),
        target: CONTAINER_CONFIG_PATH,
        mode: 0o644,
      },
    ])
    expect(genericContainerMocks.Wait.forHttp).toHaveBeenCalledWith('/api/protocols', API_PORT)
    expect(genericContainerMocks.waitStrategy.withStartupTimeout).toHaveBeenCalledWith(60_000)
    expect(container.getHttpBase()).toBe('http://127.0.0.1:38090')
  })

  it('start copies default config when no inline config is provided', async () => {
    await new MocklyContainerBuilder().start()

    expect(genericContainerMocks.chain.withCommand).toHaveBeenCalledWith(['start', '-c', CONTAINER_CONFIG_PATH])
    expect(genericContainerMocks.chain.withCopyContentToContainer).toHaveBeenCalledWith([
      {
        content: Buffer.from(`mockly:
  api:
    port: 9091
protocols:
  http:
    enabled: true
    port: 8090
`),
        target: CONTAINER_CONFIG_PATH,
        mode: 0o644,
      },
    ])
  })
})

describe('StartedMocklyContainer', () => {
  it('getHttpBase returns correct URL', () => {
    const fakeContainer = {
      getHost: () => '127.0.0.1',
      getMappedPort: (port: number) => (port === HTTP_PORT ? 38090 : 39091),
      stop: vi.fn(),
    }

    const container = new StartedMocklyContainer(fakeContainer as any)
    expect(container.getHttpBase()).toBe('http://127.0.0.1:38090')
  })

  it('addMock posts to correct endpoint', async () => {
    const fetchMock = vi.fn().mockResolvedValue(new Response(null, { status: 201 }))
    vi.stubGlobal('fetch', fetchMock)

    const mock: HttpMock = {
      id: 'users',
      request: { method: 'GET', path: '/users' },
      response: { status: 200, body: '[]' },
    }

    const container = new StartedMocklyContainer(fakeStartedContainer())
    await container.addMock(mock)

    expect(fetchMock).toHaveBeenCalledWith('http://localhost:19091/api/mocks/http', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify(mock),
    })
  })

  it('reset posts to /api/reset', async () => {
    const fetchMock = vi.fn().mockResolvedValue(new Response(null, { status: 200 }))
    vi.stubGlobal('fetch', fetchMock)

    const container = new StartedMocklyContainer(fakeStartedContainer())
    await container.reset()

    expect(fetchMock).toHaveBeenCalledWith('http://localhost:19091/api/reset', {
      method: 'POST',
      headers: {},
      body: undefined,
    })
  })

  it('getLogs calls GET /api/logs', async () => {
    const fetchMock = vi.fn().mockResolvedValue(
      new Response(JSON.stringify([{ matched_id: 'users' }]), {
        status: 200,
        headers: { 'Content-Type': 'application/json' },
      })
    )
    vi.stubGlobal('fetch', fetchMock)

    const container = new StartedMocklyContainer(fakeStartedContainer())
    const logs = await container.getLogs()

    expect(fetchMock).toHaveBeenCalledWith('http://localhost:19091/api/logs')
    expect(logs).toBe('[{"matched_id":"users"}]')
  })
})

function fakeStartedContainer() {
  return {
    getHost: () => 'localhost',
    getMappedPort: (port: number) => (port === API_PORT ? 19091 : 18090),
    stop: vi.fn(),
  }
}
