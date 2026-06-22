export interface MockRequest { method: string; path: string; headers?: Record<string, string> }
export interface MockResponse { status: number; body?: string; headers?: Record<string, string>; delay?: string }
export interface HttpMock { id: string; request: MockRequest; response: MockResponse }
export interface FaultConfig { enabled: boolean; delay?: string; status_override?: number; error_rate?: number }
