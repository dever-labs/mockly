import { afterAll, beforeAll, describe, expect, test } from 'vitest'

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
