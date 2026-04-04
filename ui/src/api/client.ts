import type { HTTPMock, WebSocketMock, GRPCMock, ProtocolInfo, LogEntry } from '../types'

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
  return res.json()
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
