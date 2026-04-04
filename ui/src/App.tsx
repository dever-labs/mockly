import { BrowserRouter, Routes, Route } from 'react-router-dom'
import { QueryClient, QueryClientProvider } from '@tanstack/react-query'
import { Sidebar } from './components/Sidebar'
import { Dashboard } from './pages/Dashboard'
import { HTTPPage } from './pages/HTTPPage'
import { WebSocketPage } from './pages/WebSocketPage'
import { GRPCPage } from './pages/GRPCPage'
import { LogsPage } from './pages/LogsPage'
import { StatePage } from './pages/StatePage'

const qc = new QueryClient()

export default function App() {
  return (
    <QueryClientProvider client={qc}>
      <BrowserRouter>
        <div className="flex min-h-screen bg-zinc-950">
          <Sidebar />
          <Routes>
            <Route path="/" element={<Dashboard />} />
            <Route path="/http" element={<HTTPPage />} />
            <Route path="/websocket" element={<WebSocketPage />} />
            <Route path="/grpc" element={<GRPCPage />} />
            <Route path="/logs" element={<LogsPage />} />
            <Route path="/state" element={<StatePage />} />
          </Routes>
        </div>
      </BrowserRouter>
    </QueryClientProvider>
  )
}
