export interface HTTPMock {
  id: string
  request: {
    method: string
    path: string
    headers?: Record<string, string>
    query?: Record<string, string>
    body?: string
  }
  response: {
    status: number
    headers?: Record<string, string>
    body?: string
    delay?: string
  }
  state?: { key: string; value: string }
}

export interface WebSocketMock {
  id: string
  path: string
  on_connect?: { send?: string; delay?: string }
  on_message?: Array<{ match: string; respond?: string; close?: boolean; delay?: string }>
  state?: { key: string; value: string }
}

export interface GRPCMock {
  id: string
  method: string
  response?: Record<string, unknown>
  error?: { code: number; message: string }
  delay?: string
}

export interface ProtocolInfo {
  protocol: string
  enabled: boolean
  port: number
  mocks: number
}

export interface MockPatch {
  mock_id: string
  status?: number
  headers?: Record<string, string>
  body?: string
  delay?: string
  disabled?: boolean
}

export interface Scenario {
  id: string
  name: string
  description?: string
  patches?: MockPatch[]
  faults?: Record<string, unknown>
}

export interface ActiveScenarios {
  active: string[]
  scenarios: Scenario[]
}

export const FAULT_PROTOCOLS = [
  'http', 'graphql', 'websocket', 'grpc', 'tcp', 'redis', 'mqtt', 'smtp',
  'snmp', 'dns', 'amqp', 'kafka', 'ldap', 'imap', 'ftp', 'memcached',
  'stomp', 'coap', 'sip',
] as const

export type FaultProtocol = (typeof FAULT_PROTOCOLS)[number]

export interface LogEntry {
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
}
