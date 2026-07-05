import { GenericContainer, Wait } from 'testcontainers'
import type { StartedTestContainer } from 'testcontainers'
import type {
  ActiveScenariosResponse,
  CallEntry,
  CallSummary,
  FaultConfig,
  HttpMock,
  MockResponsePatch,
  MocklyServerOptions,
  Scenario,
} from './types.js'

export const DEFAULT_IMAGE = 'ghcr.io/dever-labs/mockly:latest'
export const HTTP_PORT = 8090
export const API_PORT = 9091
export const CONTAINER_CONFIG_PATH = '/config/mockly.yaml'

const DEFAULT_CONFIG = `mockly:
  api:
    port: 9091
protocols:
  http:
    enabled: true
    port: 8090
`

export class MocklyContainerBuilder {
  private image = DEFAULT_IMAGE
  private inlineConfig: string | undefined
  private options: MocklyServerOptions = {}

  withImage(image: string): this {
    this.image = image
    return this
  }

  withInlineConfig(yaml: string): this {
    this.inlineConfig = yaml
    return this
  }

  withOptions(options: MocklyServerOptions): this {
    this.options = options
    return this
  }

  async start(): Promise<StartedMocklyContainer> {
    const configYaml = this.inlineConfig ?? buildConfigYaml(this.options)
    const cmd = ['start', '-c', CONTAINER_CONFIG_PATH]

    const container = await new GenericContainer(this.image)
      .withExposedPorts(HTTP_PORT, API_PORT)
      .withCommand(cmd)
      .withCopyContentToContainer([
        {
          content: Buffer.from(configYaml),
          target: CONTAINER_CONFIG_PATH,
          mode: 0o644,
        },
      ])
      .withWaitStrategy(
        Wait.forHttp('/api/protocols', API_PORT).withStartupTimeout(60_000)
      )
      .start()

    return new StartedMocklyContainer(container)
  }
}

export class StartedMocklyContainer {
  constructor(private readonly container: StartedTestContainer) {}

  getHttpBase(): string {
    return `http://${this.container.getHost()}:${this.container.getMappedPort(HTTP_PORT)}`
  }

  getApiBase(): string {
    return `http://${this.container.getHost()}:${this.container.getMappedPort(API_PORT)}`
  }

  async stop(): Promise<void> {
    await this.container.stop()
  }

  async addMock(mock: HttpMock): Promise<void> {
    const res = await this.post('/api/mocks/http', mock)
    this.assertOk(res, `addMock(${mock.id})`)
  }

  async listMocks(): Promise<HttpMock[]> {
    return this.getJson<HttpMock[]>('/api/mocks/http')
  }

  async updateMock(id: string, mock: HttpMock): Promise<HttpMock> {
    return this.putAndRead<HttpMock>(`/api/mocks/http/${encodeURIComponent(id)}`, mock)
  }

  async patchMock(id: string, patch: MockResponsePatch): Promise<HttpMock> {
    return this.patchAndRead<HttpMock>(`/api/mocks/http/${encodeURIComponent(id)}`, patch)
  }

  async deleteMock(id: string): Promise<void> {
    const res = await this.delete(`/api/mocks/http/${encodeURIComponent(id)}`)
    this.assertOk(res, `deleteMock(${id})`)
  }

  async getState(): Promise<Record<string, string>> {
    return this.getJson<Record<string, string>>('/api/state')
  }

  async setState(kvMap: Record<string, string>): Promise<Record<string, string>> {
    return this.postAndRead<Record<string, string>>('/api/state', kvMap)
  }

  async deleteState(key: string): Promise<void> {
    const res = await this.delete(`/api/state/${encodeURIComponent(key)}`)
    this.assertOk(res, `deleteState(${key})`)
  }

  async reset(): Promise<void> {
    const res = await this.post('/api/reset', null)
    this.assertOk(res, 'reset')
  }

  async listScenarios(): Promise<Scenario[]> {
    return this.getJson<Scenario[]>('/api/scenarios')
  }

  async createScenario(scenario: Scenario): Promise<Scenario> {
    return this.postAndRead<Scenario>('/api/scenarios', scenario)
  }

  async getScenario(id: string): Promise<Scenario> {
    return this.getJson<Scenario>(`/api/scenarios/${encodeURIComponent(id)}`)
  }

  async updateScenario(id: string, scenario: Scenario): Promise<Scenario> {
    return this.putAndRead<Scenario>(`/api/scenarios/${encodeURIComponent(id)}`, scenario)
  }

  async deleteScenario(id: string): Promise<void> {
    const res = await this.delete(`/api/scenarios/${encodeURIComponent(id)}`)
    this.assertOk(res, `deleteScenario(${id})`)
  }

  async listActiveScenarios(): Promise<ActiveScenariosResponse> {
    return this.getJson<ActiveScenariosResponse>('/api/scenarios/active')
  }

  async activateScenario(id: string): Promise<void> {
    const res = await this.post(`/api/scenarios/${encodeURIComponent(id)}/activate`, null)
    this.assertOk(res, `activateScenario(${id})`)
  }

  async deactivateScenario(id: string): Promise<void> {
    const res = await this.delete(`/api/scenarios/${encodeURIComponent(id)}/activate`)
    this.assertOk(res, `deactivateScenario(${id})`)
  }

  async setFault(config: FaultConfig): Promise<void> {
    const res = await this.post('/api/fault/http', config)
    this.assertOk(res, 'setFault')
  }

  async clearFault(): Promise<void> {
    const res = await this.delete('/api/fault')
    this.assertOk(res, 'clearFault')
  }

  async getLogs(matchedId?: string): Promise<CallEntry[]> {
    return this.getJson<CallEntry[]>(this.withMatchedId('/api/logs', matchedId))
  }

  async clearLogs(): Promise<void> {
    const res = await this.delete('/api/logs')
    this.assertOk(res, 'clearLogs')
  }

  async getLogsCount(matchedId?: string): Promise<number> {
    const data = await this.getJson<{ count: number }>(
      this.withMatchedId('/api/logs/count', matchedId)
    )
    return data.count
  }

  async getCalls(mockId: string): Promise<CallSummary> {
    return this.getJson<CallSummary>(`/api/calls/http/${encodeURIComponent(mockId)}`)
  }

  async clearCalls(mockId: string): Promise<void> {
    const res = await this.delete(`/api/calls/http/${encodeURIComponent(mockId)}`)
    this.assertOk(res, `clearCalls(${mockId})`)
  }

  async clearAllCalls(): Promise<void> {
    const res = await this.delete('/api/calls/http')
    this.assertOk(res, 'clearAllCalls()')
  }

  async waitForCalls(mockId: string, count = 1, timeout = '10s'): Promise<CallSummary> {
    return this.postAndRead<CallSummary>(
      `/api/calls/http/${encodeURIComponent(mockId)}/wait`,
      { count, timeout }
    )
  }

  private assertOk(res: Response, operation: string): void {
    if (!res.ok) {
      throw new Error(`${operation} failed: HTTP ${res.status}`)
    }
  }

  private withMatchedId(path: string, matchedId?: string): string {
    if (matchedId === undefined) {
      return path
    }

    const params = new URLSearchParams({ matched_id: matchedId })
    return `${path}?${params.toString()}`
  }

  private async getJson<T>(path: string): Promise<T> {
    const res = await this.get(path)
    this.assertOk(res, `GET ${path}`)
    return res.json() as Promise<T>
  }

  private async postAndRead<T>(path: string, body: unknown): Promise<T> {
    const res = await this.post(path, body)
    this.assertOk(res, `POST ${path}`)
    return res.json() as Promise<T>
  }

  private async putAndRead<T>(path: string, body: unknown): Promise<T> {
    const res = await this.put(path, body)
    this.assertOk(res, `PUT ${path}`)
    return res.json() as Promise<T>
  }

  private async patchAndRead<T>(path: string, body: unknown): Promise<T> {
    const res = await this.patch(path, body)
    this.assertOk(res, `PATCH ${path}`)
    return res.json() as Promise<T>
  }

  private async get(path: string): Promise<Response> {
    return fetch(`${this.getApiBase()}${path}`)
  }

  private async post(path: string, body: unknown): Promise<Response> {
    return fetch(`${this.getApiBase()}${path}`, {
      method: 'POST',
      headers: body !== null ? { 'Content-Type': 'application/json' } : {},
      body: body !== null ? JSON.stringify(body) : undefined,
    })
  }

  private async put(path: string, body: unknown): Promise<Response> {
    return fetch(`${this.getApiBase()}${path}`, {
      method: 'PUT',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify(body),
    })
  }

  private async patch(path: string, body: unknown): Promise<Response> {
    return fetch(`${this.getApiBase()}${path}`, {
      method: 'PATCH',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify(body),
    })
  }

  private async delete(path: string): Promise<Response> {
    return fetch(`${this.getApiBase()}${path}`, { method: 'DELETE' })
  }
}

function buildConfigYaml(options: MocklyServerOptions): string {
  const scenarios = options.scenarios ?? []

  if (scenarios.length === 0) {
    return DEFAULT_CONFIG
  }

  const lines = [
    'mockly:',
    '  api:',
    `    port: ${API_PORT}`,
    'protocols:',
    '  http:',
    '    enabled: true',
    `    port: ${HTTP_PORT}`,
    'scenarios:',
  ]

  for (const scenario of scenarios) {
    lines.push(`  - id: ${yamlString(scenario.id)}`)
    lines.push(`    name: ${yamlString(scenario.name)}`)
    if (scenario.description !== undefined) {
      lines.push(`    description: ${yamlString(scenario.description)}`)
    }
    lines.push('    patches:')

    for (const patch of scenario.patches) {
      lines.push(`      - mock_id: ${yamlString(patch.mock_id)}`)
      if (patch.status !== undefined) {
        lines.push(`        status: ${patch.status}`)
      }
      if (patch.body !== undefined) {
        lines.push(`        body: ${yamlString(patch.body)}`)
      }
      if (patch.headers !== undefined) {
        if (Object.keys(patch.headers).length === 0) {
          lines.push('        headers: {}')
        } else {
          lines.push('        headers:')
          for (const [key, value] of Object.entries(patch.headers)) {
            lines.push(`          ${yamlString(key)}: ${yamlString(value)}`)
          }
        }
      }
      if (patch.delay !== undefined) {
        lines.push(`        delay: ${yamlString(patch.delay)}`)
      }
      if (patch.disabled !== undefined) {
        lines.push(`        disabled: ${patch.disabled}`)
      }
    }
  }

  return `${lines.join('\n')}\n`
}

function yamlString(value: string): string {
  return JSON.stringify(value)
}
