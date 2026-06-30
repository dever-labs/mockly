import { spawn, ChildProcess } from 'child_process'
import { writeFileSync, mkdirSync } from 'fs'
import { join } from 'path'
import { tmpdir } from 'os'
import yaml from 'js-yaml'
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
import type { InstallOptions } from './install.js'
import { install, getBinaryPath } from './install.js'
import { getFreePorts, sleep } from './utils.js'

/**
 * Controls a Mockly server process for use in integration tests.
 *
 * @example
 * ```ts
 * // Recommended: ensure binary is installed, then start
 * const server = await MocklyServer.ensure()
 *
 * await server.addMock({
 *   id: 'get-users',
 *   request: { method: 'GET', path: '/users' },
 *   response: { status: 200, body: '[{"id":1}]', headers: { 'Content-Type': 'application/json' } },
 * })
 *
 * // Point your HTTP client at server.httpBase
 * const res = await fetch(`${server.httpBase}/users`)
 *
 * await server.stop()
 * ```
 */
export class MocklyServer {
  private proc: ChildProcess | null = null

  private constructor(
    readonly httpPort: number,
    readonly apiPort: number,
  ) {}

  /** Base URL of the HTTP mock server — e.g. `http://127.0.0.1:45123` */
  get httpBase(): string { return `http://127.0.0.1:${this.httpPort}` }

  /** Base URL of the management API — e.g. `http://127.0.0.1:45124` */
  get apiBase(): string { return `http://127.0.0.1:${this.apiPort}` }

  /**
   * Installs the Mockly binary if it is not already present, then starts the
   * server. This is the recommended entry point for most test setups.
   *
   * Respects all `InstallOptions` and their corresponding environment
   * variables — see {@link install} for details.
   */
  static async ensure(opts: MocklyServerOptions & InstallOptions = {}): Promise<MocklyServer> {
    if (!getBinaryPath(opts.binDir)) {
      await install(opts)
    }
    return MocklyServer.create(opts)
  }

  /**
   * Starts the server using an already-installed binary.
   * Throws immediately if the binary cannot be found — call `ensure()` instead
   * if you want automatic installation.
   *
   * Ports are allocated atomically (both held open simultaneously) to avoid
   * TOCTOU races where another process could claim a port between allocations.
   * If startup fails due to a port conflict, create() retries up to 3 times
   * with freshly allocated ports before giving up.
   */
  static async create(opts: MocklyServerOptions = {}): Promise<MocklyServer> {
    const MAX_ATTEMPTS = 3
    let lastError: Error | undefined

    for (let attempt = 0; attempt < MAX_ATTEMPTS; attempt++) {
      const [httpPort, apiPort] = await getFreePorts(2)
      const server = new MocklyServer(httpPort, apiPort)
      try {
        await server._start(opts.scenarios ?? [])
        return server
      } catch (err) {
        await server.stop()
        const msg = (err as Error).message
        if (isPortConflict(msg) && attempt < MAX_ATTEMPTS - 1) {
          lastError = err as Error
          continue
        }
        throw err
      }
    }

    throw lastError ?? new Error('Failed to start Mockly after multiple port allocation attempts')
  }

  /** Kills the Mockly process and waits for it to exit. */
  async stop(): Promise<void> {
    if (this.proc) {
      this.proc.kill()
      await new Promise<void>((r) => this.proc!.once('exit', r))
      this.proc = null
    }
  }

  // ── Management API ──────────────────────────────────────────────────────────

  /**
   * Registers a new HTTP mock via the management API.
   * Mocks are matched in insertion order — the first match wins.
   *
   * **Header matching** uses exact string comparison.
   * Place more-specific mocks (with header requirements) before less-specific
   * fallbacks to ensure correct priority.
   */
  async addMock(mock: HttpMock): Promise<void> {
    const res = await this._post('/api/mocks/http', mock)
    if (!res.ok) throw new Error(`addMock(${mock.id}) failed: HTTP ${res.status}`)
  }

  /** Returns all configured HTTP mocks. */
  async listMocks(): Promise<HttpMock[]> {
    return this._get<HttpMock[]>('/api/mocks/http')
  }

  /** Replaces an existing mock definition and returns the updated mock. */
  async updateMock(id: string, mock: HttpMock): Promise<HttpMock> {
    return this._putAndRead<HttpMock>(`/api/mocks/http/${encodeURIComponent(id)}`, mock)
  }

  /** Applies a partial response patch to an existing mock and returns the updated mock. */
  async patchMock(id: string, patch: MockResponsePatch): Promise<HttpMock> {
    return this._patchAndRead<HttpMock>(`/api/mocks/http/${encodeURIComponent(id)}`, patch)
  }

  /** Removes a mock by id. */
  async deleteMock(id: string): Promise<void> {
    await fetch(`${this.apiBase}/api/mocks/http/${id}`, { method: 'DELETE' })
  }

  /** Returns the full server state map. */
  async getState(): Promise<Record<string, string>> {
    return this._get<Record<string, string>>('/api/state')
  }

  /** Replaces or merges server state and returns the updated state map. */
  async setState(kvMap: Record<string, string>): Promise<Record<string, string>> {
    const res = await this._post('/api/state', kvMap)
    if (!res.ok) throw new Error(`setState failed: HTTP ${res.status}`)
    return res.json() as Promise<Record<string, string>>
  }

  /** Deletes a single state key. */
  async deleteState(key: string): Promise<void> {
    const res = await fetch(`${this.apiBase}/api/state/${encodeURIComponent(key)}`, { method: 'DELETE' })
    if (!res.ok) throw new Error(`deleteState(${key}) failed: HTTP ${res.status}`)
  }

  /** Returns recorded logs, optionally filtered by matched mock ID. */
  async getLogs(matchedId?: string): Promise<CallEntry[]> {
    const path = matchedId
      ? `/api/logs?matched_id=${encodeURIComponent(matchedId)}`
      : '/api/logs'
    return this._get<CallEntry[]>(path)
  }

  /** Clears all recorded logs. */
  async clearLogs(): Promise<void> {
    const res = await fetch(`${this.apiBase}/api/logs`, { method: 'DELETE' })
    if (!res.ok) throw new Error(`clearLogs failed: HTTP ${res.status}`)
  }

  /** Returns the number of recorded logs, optionally filtered by matched mock ID. */
  async getLogsCount(matchedId?: string): Promise<number> {
    const path = matchedId
      ? `/api/logs/count?matched_id=${encodeURIComponent(matchedId)}`
      : '/api/logs/count'
    const res = await this._get<{ count: number }>(path)
    return res.count
  }

  /** Returns all configured scenarios. */
  async listScenarios(): Promise<Scenario[]> {
    return this._get<Scenario[]>('/api/scenarios')
  }

  /** Creates a scenario and returns the created value. */
  async createScenario(scenario: Scenario): Promise<Scenario> {
    const res = await this._post('/api/scenarios', scenario)
    if (!res.ok) throw new Error(`createScenario(${scenario.id}) failed: HTTP ${res.status}`)
    return res.json() as Promise<Scenario>
  }

  /** Returns a scenario by ID. */
  async getScenario(id: string): Promise<Scenario> {
    return this._get<Scenario>(`/api/scenarios/${encodeURIComponent(id)}`)
  }

  /** Replaces an existing scenario and returns the updated scenario. */
  async updateScenario(id: string, scenario: Scenario): Promise<Scenario> {
    return this._putAndRead<Scenario>(`/api/scenarios/${encodeURIComponent(id)}`, scenario)
  }

  /** Deletes a scenario by ID. */
  async deleteScenario(id: string): Promise<void> {
    const res = await fetch(`${this.apiBase}/api/scenarios/${encodeURIComponent(id)}`, { method: 'DELETE' })
    if (!res.ok) throw new Error(`deleteScenario(${id}) failed: HTTP ${res.status}`)
  }

  /** Returns all active scenario IDs together with their full definitions. */
  async listActiveScenarios(): Promise<ActiveScenariosResponse> {
    return this._get<ActiveScenariosResponse>('/api/scenarios/active')
  }

  /**
   * Activates a named scenario, patching the responses of referenced mocks.
   * The scenario must have been declared in `MocklyServerOptions.scenarios`.
   */
  async activateScenario(id: string): Promise<void> {
    const res = await this._post(`/api/scenarios/${id}/activate`, null)
    if (!res.ok) throw new Error(`activateScenario(${id}) failed: HTTP ${res.status}`)
  }

  /** Deactivates a previously activated scenario. */
  async deactivateScenario(id: string): Promise<void> {
    const res = await fetch(`${this.apiBase}/api/scenarios/${id}/activate`, { method: 'DELETE' })
    if (!res.ok) throw new Error(`deactivateScenario(${id}) failed: HTTP ${res.status}`)
  }

  /**
   * Enables global fault injection.
   * Faults apply to every request regardless of mock matching.
   */
  async setFault(config: FaultConfig): Promise<void> {
    const res = await this._post('/api/fault/http', config)
    if (!res.ok) throw new Error(`setFault failed: HTTP ${res.status}`)
  }

  /** Disables all active fault injection. */
  async clearFault(): Promise<void> {
    await fetch(`${this.apiBase}/api/fault`, { method: 'DELETE' })
  }

  /**
   * Returns the recorded calls for a specific HTTP mock.
   * @param mockId the mock ID to look up
   */
  async getCalls(mockId: string): Promise<CallSummary> {
    const res = await fetch(`${this.apiBase}/api/calls/http/${encodeURIComponent(mockId)}`)
    if (!res.ok) throw new Error(`getCalls(${mockId}) failed: HTTP ${res.status}`)
    return res.json() as Promise<CallSummary>
  }

  /**
   * Clears recorded calls for a specific HTTP mock.
   * @param mockId the mock ID to clear calls for
   */
  async clearCalls(mockId: string): Promise<void> {
    const res = await fetch(`${this.apiBase}/api/calls/http/${encodeURIComponent(mockId)}`, { method: 'DELETE' })
    if (!res.ok) throw new Error(`clearCalls(${mockId}) failed: HTTP ${res.status}`)
  }

  /** Clears all recorded HTTP calls across every mock. */
  async clearAllCalls(): Promise<void> {
    const res = await fetch(`${this.apiBase}/api/calls/http`, { method: 'DELETE' })
    if (!res.ok) throw new Error(`clearAllCalls() failed: HTTP ${res.status}`)
  }

  /**
   * Blocks until a mock has been called at least `count` times, or until
   * `timeout` expires. Throws if the timeout is reached first.
   *
   * @param mockId  the mock ID to wait on
   * @param count   minimum number of calls to wait for (default: 1)
   * @param timeout Go-style duration string, e.g. `'5s'`, `'500ms'` (default: `'10s'`)
   */
  async waitForCalls(mockId: string, count = 1, timeout = '10s'): Promise<CallSummary> {
    const res = await this._post(`/api/calls/http/${encodeURIComponent(mockId)}/wait`, { count, timeout })
    if (!res.ok) throw new Error(`waitForCalls(${mockId}, ${count}) timed out: HTTP ${res.status}`)
    return res.json() as Promise<CallSummary>
  }

  /**
   * Resets all state: removes dynamically added mocks, deactivates scenarios,
   * and clears fault injection. Mocks from the startup config are preserved.
   *
   * Call this in `beforeEach` to keep tests isolated.
   */
  async reset(): Promise<void> {
    await this._post('/api/reset', null)
  }

  // ── Private ─────────────────────────────────────────────────────────────────

  private async _start(scenarios: Scenario[]): Promise<void> {
    const bin = getBinaryPath()
    if (!bin) {
      throw new Error(
        'Mockly binary not found. Use `MocklyServer.ensure()` instead of ' +
        '`MocklyServer.create()` to install automatically, or call `install()` first.\n' +
        'For pre-staged binaries set MOCKLY_BINARY_PATH to the absolute path.'
      )
    }

    const cfgPath = this._writeConfig(scenarios)
    let stderrOutput = ''

    this.proc = spawn(bin, ['start', '--config', cfgPath, `--api-port=${this.apiPort}`], {
      stdio: ['ignore', 'pipe', 'pipe'],
    })
    this.proc.stderr?.on('data', (chunk: Buffer) => { stderrOutput += chunk.toString() })
    this.proc.on('error', (err) => { throw new Error(`mockly spawn error: ${err.message}`) })

    try {
      await this._waitReady()
    } catch (err) {
      const detail = stderrOutput.trim() ? `
Mockly output:
${stderrOutput.trim()}` : ''
      throw new Error(`${(err as Error).message}${detail}`)
    }
  }

  private _writeConfig(scenarios: Scenario[]): string {
    const dir = join(tmpdir(), `mockly-node-${Date.now()}`)
    mkdirSync(dir, { recursive: true })
    const cfgPath = join(dir, 'mockly.yaml')

    const config: Record<string, unknown> = {
      mockly: { api: { port: this.apiPort } },
      protocols: { http: { enabled: true, port: this.httpPort } },
    }

    if (scenarios.length > 0) {
      config.scenarios = scenarios.map((s) => ({
        id: s.id,
        name: s.name,
        ...(s.description !== undefined ? { description: s.description } : {}),
        patches: s.patches.map((p) => {
          const patch: Record<string, unknown> = { mock_id: p.mock_id }
          if (p.status !== undefined) patch.status = p.status
          if (p.body !== undefined) patch.body = p.body
          if (p.headers !== undefined) patch.headers = p.headers
          if (p.delay !== undefined) patch.delay = p.delay
          if (p.disabled !== undefined) patch.disabled = p.disabled
          return patch
        }),
      }))
    }

    writeFileSync(cfgPath, yaml.dump(config), 'utf-8')
    return cfgPath
  }

  private async _waitReady(maxMs = 10_000): Promise<void> {
    const deadline = Date.now() + maxMs
    while (Date.now() < deadline) {
      try {
        const res = await fetch(`${this.apiBase}/api/protocols`, {
          signal: AbortSignal.timeout(300),
        })
        if (res.ok) return
      } catch { /* not ready yet */ }
      await sleep(50)
    }
    throw new Error(`Mockly did not become ready on port ${this.apiPort} within ${maxMs}ms`)
  }

  private async _get<T>(path: string): Promise<T> {
    const res = await fetch(`${this.apiBase}${path}`)
    if (!res.ok) throw new Error(`GET ${path} failed: HTTP ${res.status}`)
    return res.json() as Promise<T>
  }

  private _post(path: string, body: unknown): Promise<Response> {
    return fetch(`${this.apiBase}${path}`, {
      method: 'POST',
      headers: body !== null ? { 'Content-Type': 'application/json' } : {},
      body: body !== null ? JSON.stringify(body) : undefined,
    })
  }

  private _put(path: string, body: unknown): Promise<Response> {
    return fetch(`${this.apiBase}${path}`, {
      method: 'PUT',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify(body),
    })
  }

  private _patch(path: string, body: unknown): Promise<Response> {
    return fetch(`${this.apiBase}${path}`, {
      method: 'PATCH',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify(body),
    })
  }

  private async _putAndRead<T>(path: string, body: unknown): Promise<T> {
    const res = await this._put(path, body)
    if (!res.ok) throw new Error(`PUT ${path} failed: HTTP ${res.status}`)
    return res.json() as Promise<T>
  }

  private async _patchAndRead<T>(path: string, body: unknown): Promise<T> {
    const res = await this._patch(path, body)
    if (!res.ok) throw new Error(`PATCH ${path} failed: HTTP ${res.status}`)
    return res.json() as Promise<T>
  }
}

function isPortConflict(errorMessage: string): boolean {
  return /address already in use|EADDRINUSE|bind/i.test(errorMessage)
}
