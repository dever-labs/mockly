import { GenericContainer, Wait } from 'testcontainers'
import type { StartedTestContainer } from 'testcontainers'
import type { HttpMock, FaultConfig } from './types.js'

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

  withImage(image: string): this {
    this.image = image
    return this
  }

  withInlineConfig(yaml: string): this {
    this.inlineConfig = yaml
    return this
  }

  async start(): Promise<StartedMocklyContainer> {
    const configYaml = this.inlineConfig ?? DEFAULT_CONFIG
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

  async deleteMock(id: string): Promise<void> {
    const res = await this.delete(`/api/mocks/http/${id}`)
    this.assertOk(res, `deleteMock(${id})`)
  }

  async reset(): Promise<void> {
    const res = await this.post('/api/reset', null)
    this.assertOk(res, 'reset')
  }

  async activateScenario(id: string): Promise<void> {
    const res = await this.post(`/api/scenarios/${id}/activate`, null)
    this.assertOk(res, `activateScenario(${id})`)
  }

  async deactivateScenario(id: string): Promise<void> {
    const res = await this.post(`/api/scenarios/${id}/deactivate`, null)
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

  async getLogs(): Promise<string> {
    const res = await this.get('/api/logs')
    this.assertOk(res, 'getLogs')
    return await res.text()
  }

  async clearLogs(): Promise<void> {
    const res = await this.delete('/api/logs')
    this.assertOk(res, 'clearLogs')
  }

  private assertOk(res: Response, operation: string): void {
    if (!res.ok) {
      throw new Error(`${operation} failed: HTTP ${res.status}`)
    }
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

  private async delete(path: string): Promise<Response> {
    return fetch(`${this.getApiBase()}${path}`, { method: 'DELETE' })
  }
}
