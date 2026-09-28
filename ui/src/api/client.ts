import type { HTTPMock, WebSocketMock, GRPCMock, ProtocolInfo, LogEntry, Scenario, ActiveScenarios } from '../types'

const BASE = '/api'

async function req<T>(path: string, options?: RequestInit): Promise<T> {
  const res = await fetch(BASE + path, {
    headers: { 'Content-Type': 'application/json', ...options?.headers },
    ...options,
  })
  if (!res.ok) {
    const err = await res.json().catch(() => ({ error: res.statusText }))
    throw new Error(err.error ?? res.statusText)
  }
  if (res.status === 204) return undefined as T
  const text = await res.text()
  return (text ? JSON.parse(text) : undefined) as T
}

export const getProtocols = () => req<ProtocolInfo[]>('/protocols')

export const getHTTPMocks = () => req<HTTPMock[]>('/mocks/http')
export const createHTTPMock = (m: Omit<HTTPMock, 'id'> & { id?: string }) =>
  req<HTTPMock>('/mocks/http', { method: 'POST', body: JSON.stringify(m) })
export const updateHTTPMock = (id: string, m: HTTPMock) =>
  req<HTTPMock>(`/mocks/http/${id}`, { method: 'PUT', body: JSON.stringify(m) })
export const deleteHTTPMock = (id: string) =>
  req<{ deleted: string }>(`/mocks/http/${id}`, { method: 'DELETE' })

export const getWSMocks = () => req<WebSocketMock[]>('/mocks/websocket')
export const createWSMock = (m: Omit<WebSocketMock, 'id'> & { id?: string }) =>
  req<WebSocketMock>('/mocks/websocket', { method: 'POST', body: JSON.stringify(m) })
export const updateWSMock = (id: string, m: WebSocketMock) =>
  req<WebSocketMock>(`/mocks/websocket/${id}`, { method: 'PUT', body: JSON.stringify(m) })
export const deleteWSMock = (id: string) =>
  req<{ deleted: string }>(`/mocks/websocket/${id}`, { method: 'DELETE' })

export const getGRPCMocks = () => req<GRPCMock[]>('/mocks/grpc')
export const createGRPCMock = (m: Omit<GRPCMock, 'id'> & { id?: string }) =>
  req<GRPCMock>('/mocks/grpc', { method: 'POST', body: JSON.stringify(m) })
export const updateGRPCMock = (id: string, m: GRPCMock) =>
  req<GRPCMock>(`/mocks/grpc/${id}`, { method: 'PUT', body: JSON.stringify(m) })
export const deleteGRPCMock = (id: string) =>
  req<{ deleted: string }>(`/mocks/grpc/${id}`, { method: 'DELETE' })

export const getState = () => req<Record<string, string>>('/state')
export const setState = (data: Record<string, string>) =>
  req<Record<string, string>>('/state', { method: 'POST', body: JSON.stringify(data) })
export const deleteStateKey = (key: string) =>
  req<{ deleted: string }>(`/state/${key}`, { method: 'DELETE' })

export const getLogs = () => req<LogEntry[]>('/logs')
export const clearLogs = () => req<{ status: string }>('/logs', { method: 'DELETE' })
export const resetAll = () => req<{ status: string }>('/reset', { method: 'POST' })

// Scenarios
export const getScenarios = () => req<Scenario[]>('/scenarios')
export const getActiveScenarios = () => req<ActiveScenarios>('/scenarios/active')
export const createScenario = (sc: Omit<Scenario, 'id'> & { id?: string }) =>
  req<Scenario>('/scenarios', { method: 'POST', body: JSON.stringify(sc) })
export const updateScenario = (id: string, sc: Scenario) =>
  req<Scenario>(`/scenarios/${id}`, { method: 'PUT', body: JSON.stringify(sc) })
export const deleteScenario = (id: string) =>
  req<{ deleted: string }>(`/scenarios/${id}`, { method: 'DELETE' })
export const activateScenario = (id: string) =>
  req<{ activated: string; active: string[] }>(`/scenarios/${id}/activate`, { method: 'POST' })
export const deactivateScenario = (id: string) =>
  req<{ deactivated: string; active: string[] }>(`/scenarios/${id}/deactivate`, { method: 'POST' })

// Fault injection
export const getAllFaults = () => req<Record<string, unknown>>('/fault')
export const clearAllFaults = () => req<void>('/fault', { method: 'DELETE' })
export const getProtocolFault = (protocol: string) => req<unknown>(`/fault/${protocol}`)
export const getEffectiveProtocolFault = (protocol: string) => req<unknown>(`/fault/${protocol}/effective`)
export const setProtocolFault = (protocol: string, fault: unknown) =>
  req<unknown>(`/fault/${protocol}`, { method: 'POST', body: JSON.stringify(fault) })
export const clearProtocolFault = (protocol: string) =>
  req<void>(`/fault/${protocol}`, { method: 'DELETE' })
