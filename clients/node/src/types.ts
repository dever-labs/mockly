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

/** Partial response update for an existing HTTP mock. */
export interface MockResponsePatch {
  status?: number
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
  headers?: Record<string, string>
  delay?: string
  disabled?: boolean
}

/** A named scenario that patches one or more mock responses when activated. */
export interface Scenario {
  id: string
  name: string
  description?: string
  patches: ScenarioPatch[]
}

/** Active scenario state reported by the server. */
export interface ActiveScenariosResponse {
  active: string[]
  scenarios: Scenario[]
}

/** Global fault injection configuration. */
export interface FaultConfig {
  enabled: boolean
  /** Artificial delay added to every request — e.g. `'200ms'` */
  delay?: string
  /** Override the HTTP status of every matched response */
  status?: number
  /** Probability (0–1) that the override fires; 0 means always */
  error_rate?: number
}

/** A single recorded HTTP request captured by the Mockly server. */
export interface CallEntry {
  id: string
  timestamp: string
  protocol: string
  method?: string
  path: string
  status?: number
  duration_ms: number
  headers?: Record<string, string>
  body?: string
  matched_id?: string
  path_params?: Record<string, string>
}

/** Summary of recorded calls for a specific HTTP mock. */
export interface CallSummary {
  mock_id: string
  count: number
  calls: CallEntry[]
}

/** Options accepted by `MocklyServer.create()`. */
export interface MocklyServerOptions {
  /** Scenarios to include in the startup config. */
  scenarios?: Scenario[]
}
