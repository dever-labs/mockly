/** Request matching criteria for an HTTP mock. */
export interface MockRequest {
  method: string
  path: string
  /** Exact header value matching — e.g. `{ Authorization: 'Bearer token123' }` */
  headers?: Record<string, string>
}

/** Response definition for an HTTP mock. */
export interface MockResponse {
  status: number
  body?: string
  headers?: Record<string, string>
  /** Artificial delay — e.g. `'100ms'`, `'1s'` */
  delay?: string
}

/** A full HTTP mock definition. */
export interface HttpMock {
  id: string
  request: MockRequest
  response: MockResponse
}

/** A single patch applied when a scenario is activated. */
export interface ScenarioPatch {
  mock_id: string
  status?: number
  body?: string
  delay?: string
}

/** A named scenario that patches one or more mock responses when activated. */
export interface Scenario {
  id: string
  name: string
  patches: ScenarioPatch[]
}

/** Global fault injection configuration. */
export interface FaultConfig {
  enabled: boolean
  /** Artificial delay added to every request — e.g. `'200ms'` */
  delay?: string
  /** Override the HTTP status of every matched response */
  status_override?: number
  /** Probability (0–1) that the override fires; 0 means always */
  error_rate?: number
}

/** Options accepted by `MocklyServer.create()`. */
export interface MocklyServerOptions {
  /**
   * Scenarios to include in the startup config.
   * Scenarios can only be activated/deactivated via the management API;
   * they cannot be created dynamically after the server starts.
   */
  scenarios?: Scenario[]
}
