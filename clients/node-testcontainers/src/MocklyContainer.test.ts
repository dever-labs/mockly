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
import type {
  ActiveScenariosResponse,
  CallEntry,
  CallSummary,
  HttpMock,
  MocklyServerOptions,
  Scenario,
} from './types.js'

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

  it('withOptions stores options', () => {
    const builder = new MocklyContainerBuilder().withOptions({
      scenarios: [{ id: 's1', name: 'Scenario', patches: [{ mock_id: 'users', status: 503 }] }],
    }) as MocklyContainerBuilder & { options?: MocklyServerOptions }

    expect(builder.options).toEqual({
      scenarios: [{ id: 's1', name: 'Scenario', patches: [{ mock_id: 'users', status: 503 }] }],
    })
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

  it('withOptions generates scenario config yaml', async () => {
    await new MocklyContainerBuilder().withOptions({
      scenarios: [
        {
          id: 'payments-down',
          name: 'Payments down',
          description: 'Simulate outage',
          patches: [
            {
              mock_id: 'charge',
              status: 503,
              body: '{"error":"unavailable"}',
              headers: { 'Content-Type': 'application/json' },
              delay: '750ms',
              disabled: false,
            },
          ],
        },
      ],
    }).start()

    expect(genericContainerMocks.chain.withCopyContentToContainer).toHaveBeenCalledWith([
      {
        content: Buffer.from(`mockly:
  api:
    port: 9091
protocols:
  http:
    enabled: true
    port: 8090
scenarios:
  - id: "payments-down"
    name: "Payments down"
    description: "Simulate outage"
    patches:
      - mock_id: "charge"
        status: 503
        body: "{\\"error\\":\\"unavailable\\"}"
        headers:
          "Content-Type": "application/json"
        delay: "750ms"
        disabled: false
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

  it('supports full management API parity', async () => {
    const mock: HttpMock = {
      id: 'users',
      request: { method: 'GET', path: '/users' },
      response: { status: 200, body: '[]' },
    }
    const updatedMock: HttpMock = {
      ...mock,
      response: { ...mock.response, status: 201 },
    }
    const scenario: Scenario = {
      id: 'payments-down',
      name: 'Payments down',
      patches: [{ mock_id: 'users', status: 503 }],
    }
    const logs: CallEntry[] = [
      {
        id: 'log-1',
        timestamp: '2026-07-05T00:00:00Z',
        protocol: 'http',
        method: 'GET',
        path: '/users',
        status: 200,
        duration_ms: 1,
        matched_id: 'users',
      },
    ]
    const calls: CallSummary = { mock_id: 'users', count: 1, calls: logs }
    const activeScenarios: ActiveScenariosResponse = {
      active: ['payments-down'],
      scenarios: [scenario],
    }

    const fetchMock = vi.fn()
      .mockResolvedValueOnce(new Response(null, { status: 201 }))
      .mockResolvedValueOnce(jsonResponse([mock]))
      .mockResolvedValueOnce(jsonResponse(updatedMock))
      .mockResolvedValueOnce(jsonResponse(updatedMock))
      .mockResolvedValueOnce(new Response(null, { status: 204 }))
      .mockResolvedValueOnce(jsonResponse({ env: 'test' }))
      .mockResolvedValueOnce(jsonResponse({ env: 'prod' }))
      .mockResolvedValueOnce(new Response(null, { status: 204 }))
      .mockResolvedValueOnce(new Response(null, { status: 200 }))
      .mockResolvedValueOnce(jsonResponse([scenario]))
      .mockResolvedValueOnce(jsonResponse(scenario))
      .mockResolvedValueOnce(jsonResponse(scenario))
      .mockResolvedValueOnce(jsonResponse(scenario))
      .mockResolvedValueOnce(new Response(null, { status: 204 }))
      .mockResolvedValueOnce(jsonResponse(activeScenarios))
      .mockResolvedValueOnce(new Response(null, { status: 200 }))
      .mockResolvedValueOnce(new Response(null, { status: 204 }))
      .mockResolvedValueOnce(new Response(null, { status: 200 }))
      .mockResolvedValueOnce(new Response(null, { status: 204 }))
      .mockResolvedValueOnce(jsonResponse(logs))
      .mockResolvedValueOnce(jsonResponse(logs))
      .mockResolvedValueOnce(jsonResponse({ count: 1 }))
      .mockResolvedValueOnce(jsonResponse({ count: 1 }))
      .mockResolvedValueOnce(new Response(null, { status: 204 }))
      .mockResolvedValueOnce(jsonResponse(calls))
      .mockResolvedValueOnce(new Response(null, { status: 204 }))
      .mockResolvedValueOnce(new Response(null, { status: 204 }))
      .mockResolvedValueOnce(jsonResponse(calls))

    vi.stubGlobal('fetch', fetchMock)

    const container = new StartedMocklyContainer(fakeStartedContainer())

    await container.addMock(mock)
    expect(await container.listMocks()).toEqual([mock])
    expect(await container.updateMock('users', updatedMock)).toEqual(updatedMock)
    expect(await container.patchMock('users', { status: 201 })).toEqual(updatedMock)
    await container.deleteMock('users')
    expect(await container.getState()).toEqual({ env: 'test' })
    expect(await container.setState({ env: 'prod' })).toEqual({ env: 'prod' })
    await container.deleteState('env')
    await container.reset()
    expect(await container.listScenarios()).toEqual([scenario])
    expect(await container.createScenario(scenario)).toEqual(scenario)
    expect(await container.getScenario('payments-down')).toEqual(scenario)
    expect(await container.updateScenario('payments-down', scenario)).toEqual(scenario)
    await container.deleteScenario('payments-down')
    expect(await container.listActiveScenarios()).toEqual(activeScenarios)
    await container.activateScenario('payments-down')
    await container.deactivateScenario('payments-down')
    await container.setFault({ enabled: true, delay: '10ms', status: 503, error_rate: 0.5 })
    await container.clearFault()
    expect(await container.getLogs()).toEqual(logs)
    expect(await container.getLogs('users')).toEqual(logs)
    expect(await container.getLogsCount()).toBe(1)
    expect(await container.getLogsCount('users')).toBe(1)
    await container.clearLogs()
    expect(await container.getCalls('users')).toEqual(calls)
    await container.clearCalls('users')
    await container.clearAllCalls()
    expect(await container.waitForCalls('users', 2, '15s')).toEqual(calls)

    expect(fetchMock.mock.calls).toEqual([
      ['http://localhost:19091/api/mocks/http', { method: 'POST', headers: { 'Content-Type': 'application/json' }, body: JSON.stringify(mock) }],
      ['http://localhost:19091/api/mocks/http'],
      ['http://localhost:19091/api/mocks/http/users', { method: 'PUT', headers: { 'Content-Type': 'application/json' }, body: JSON.stringify(updatedMock) }],
      ['http://localhost:19091/api/mocks/http/users', { method: 'PATCH', headers: { 'Content-Type': 'application/json' }, body: JSON.stringify({ status: 201 }) }],
      ['http://localhost:19091/api/mocks/http/users', { method: 'DELETE' }],
      ['http://localhost:19091/api/state'],
      ['http://localhost:19091/api/state', { method: 'POST', headers: { 'Content-Type': 'application/json' }, body: JSON.stringify({ env: 'prod' }) }],
      ['http://localhost:19091/api/state/env', { method: 'DELETE' }],
      ['http://localhost:19091/api/reset', { method: 'POST', headers: {}, body: undefined }],
      ['http://localhost:19091/api/scenarios'],
      ['http://localhost:19091/api/scenarios', { method: 'POST', headers: { 'Content-Type': 'application/json' }, body: JSON.stringify(scenario) }],
      ['http://localhost:19091/api/scenarios/payments-down'],
      ['http://localhost:19091/api/scenarios/payments-down', { method: 'PUT', headers: { 'Content-Type': 'application/json' }, body: JSON.stringify(scenario) }],
      ['http://localhost:19091/api/scenarios/payments-down', { method: 'DELETE' }],
      ['http://localhost:19091/api/scenarios/active'],
      ['http://localhost:19091/api/scenarios/payments-down/activate', { method: 'POST', headers: {}, body: undefined }],
      ['http://localhost:19091/api/scenarios/payments-down/activate', { method: 'DELETE' }],
      ['http://localhost:19091/api/fault/http', { method: 'POST', headers: { 'Content-Type': 'application/json' }, body: JSON.stringify({ enabled: true, delay: '10ms', status: 503, error_rate: 0.5 }) }],
      ['http://localhost:19091/api/fault', { method: 'DELETE' }],
      ['http://localhost:19091/api/logs'],
      ['http://localhost:19091/api/logs?matched_id=users'],
      ['http://localhost:19091/api/logs/count'],
      ['http://localhost:19091/api/logs/count?matched_id=users'],
      ['http://localhost:19091/api/logs', { method: 'DELETE' }],
      ['http://localhost:19091/api/calls/http/users'],
      ['http://localhost:19091/api/calls/http/users', { method: 'DELETE' }],
      ['http://localhost:19091/api/calls/http', { method: 'DELETE' }],
      ['http://localhost:19091/api/calls/http/users/wait', { method: 'POST', headers: { 'Content-Type': 'application/json' }, body: JSON.stringify({ count: 2, timeout: '15s' }) }],
    ])
  })
})

function fakeStartedContainer() {
  return {
    getHost: () => 'localhost',
    getMappedPort: (port: number) => (port === API_PORT ? 19091 : 18090),
    stop: vi.fn(),
  }
}

function jsonResponse(body: unknown, status = 200): Response {
  return new Response(JSON.stringify(body), {
    status,
    headers: { 'Content-Type': 'application/json' },
  })
}
