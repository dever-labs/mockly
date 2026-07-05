import { afterAll, beforeAll, beforeEach, describe, expect, test } from 'vitest'

import { MocklyContainerBuilder, type StartedMocklyContainer } from './index.js'

const describeIntegration = process.env.INTEGRATION ? describe : describe.skip

const DEFAULT_INLINE_CONFIG = `mockly:
  api:
    port: 9091
protocols:
  http:
    enabled: true
    port: 8090
`

describeIntegration('MocklyContainer integration', () => {
  let container: StartedMocklyContainer

  beforeAll(async () => {
    container = await new MocklyContainerBuilder().start()
  })

  beforeEach(async () => {
    await container.reset()
    await container.clearLogs()
    await container.clearAllCalls()
  })

  afterAll(async () => {
    await container.stop()
  })

  test('container lifecycle and mock API', async () => {
    expect(container.getHttpBase()).toMatch(/^http:\/\//)
    expect(container.getApiBase()).toMatch(/^http:\/\//)

    const protocolsResponse = await fetch(`${container.getApiBase()}/api/protocols`)
    expect(protocolsResponse.status).toBe(200)

    await container.addMock({
      id: 'hello-mock',
      request: { method: 'GET', path: '/hello' },
      response: { status: 200, body: 'world' },
    })

    const mockResponse = await fetch(`${container.getHttpBase()}/hello`)
    expect(mockResponse.status).toBe(200)
    expect(await mockResponse.text()).toBe('world')

    await container.reset()

    const resetResponse = await fetch(`${container.getHttpBase()}/hello`)
    expect(resetResponse.status).not.toBe(200)
  })

  test('getLogs returns parsed call entries after request', async () => {
    await fetch(`${container.getHttpBase()}/any-path`)

    const logs = await container.getLogs()

    expect(logs.length).toBeGreaterThan(0)
    expect(logs[0]).toHaveProperty('path')
  })

  test('listMocks returns added mocks', async () => {
    await container.addMock({
      id: 'list-test',
      request: { method: 'GET', path: '/list-me' },
      response: { status: 200, body: 'listed' },
    })

    const mocks = await container.listMocks()

    expect(mocks.length).toBeGreaterThan(0)
    expect(mocks.some((mock) => mock.id === 'list-test')).toBe(true)
  })

  test('updateMock changes response', async () => {
    await container.addMock({
      id: 'update-test',
      request: { method: 'GET', path: '/update-me' },
      response: { status: 200, body: 'original' },
    })

    await container.updateMock('update-test', {
      id: 'update-test',
      request: { method: 'GET', path: '/update-me' },
      response: { status: 200, body: 'updated' },
    })

    const response = await fetch(`${container.getHttpBase()}/update-me`)

    expect(await response.text()).toBe('updated')
  })

  test('patchMock changes status', async () => {
    await container.addMock({
      id: 'patch-test',
      request: { method: 'GET', path: '/patch-me' },
      response: { status: 200, body: 'ok' },
    })

    await container.patchMock('patch-test', { status: 418 })

    const response = await fetch(`${container.getHttpBase()}/patch-me`)

    expect(response.status).toBe(418)
  })

  test('state CRUD works', async () => {
    await container.setState({ foo: 'bar' })

    const stateAfterSet = await container.getState()
    expect(stateAfterSet.foo).toBe('bar')

    await container.deleteState('foo')

    const stateAfterDelete = await container.getState()
    expect(stateAfterDelete.foo).toBeUndefined()
  })

  test('getLogsCount returns positive count after request', async () => {
    await fetch(container.getHttpBase())

    const count = await container.getLogsCount()

    expect(count).toBeGreaterThan(0)
  })

  test('scenario CRUD works', async () => {
    await expect(container.listScenarios()).resolves.toBeDefined()

    await container.createScenario({
      id: 'tc-s1',
      name: 'TC S1',
      patches: [],
    })

    const scenario = await container.getScenario('tc-s1')
    expect(scenario.id).toBe('tc-s1')

    const scenarios = await container.listScenarios()
    expect(scenarios.length).toBeGreaterThan(0)

    await container.deleteScenario('tc-s1')
  })

  test('getCalls and clearCalls work', async () => {
    await container.addMock({
      id: 'calls-test',
      request: { method: 'GET', path: '/calls-test' },
      response: { status: 200, body: 'ok' },
    })

    await fetch(`${container.getHttpBase()}/calls-test`)

    const callsAfterHit = await container.getCalls('calls-test')
    expect(callsAfterHit.count).toBeGreaterThan(0)

    await container.clearCalls('calls-test')

    const callsAfterClear = await container.getCalls('calls-test')
    expect(callsAfterClear.count).toBe(0)

    await expect(container.clearAllCalls()).resolves.toBeUndefined()
  })

  test('waitForCalls resolves when mock is hit', async () => {
    await container.addMock({
      id: 'wait-test',
      request: { method: 'GET', path: '/wait-test' },
      response: { status: 200, body: 'ok' },
    })

    const requestPromise = fetch(`${container.getHttpBase()}/wait-test`)
    const calls = await container.waitForCalls('wait-test', 1, '10s')

    await requestPromise

    expect(calls.count).toBeGreaterThanOrEqual(1)
  })
})

describeIntegration('MocklyContainer inline config integration', () => {
  let container: StartedMocklyContainer

  beforeAll(async () => {
    container = await new MocklyContainerBuilder()
      .withInlineConfig(DEFAULT_INLINE_CONFIG)
      .start()
  })

  afterAll(async () => {
    await container.stop()
  })

  test('withInlineConfig works', async () => {
    const response = await fetch(`${container.getApiBase()}/api/protocols`)
    expect(response.status).toBe(200)
  })
})

describeIntegration('MocklyContainer options integration', () => {
  let container: StartedMocklyContainer

  beforeAll(async () => {
    container = await new MocklyContainerBuilder()
      .withOptions({
        scenarios: [
          {
            id: 'payments-down',
            name: 'Payments down',
            patches: [{ mock_id: 'charge', status: 503 }],
          },
        ],
      })
      .start()
  })

  afterAll(async () => {
    await container.stop()
  })

  test('withOptions preloads scenarios', async () => {
    const scenarios = await container.listScenarios()
    expect(scenarios.map((scenario) => scenario.id)).toContain('payments-down')
  })
})
