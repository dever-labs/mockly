import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query'
import { getProtocols, resetAll } from '../api/client'
import { PageShell } from '../components/PageShell'
import { Button } from '../components/Button'
import { Link } from 'react-router-dom'

const PROTOCOL_META: Record<string, { color: string; path: string }> = {
  http: { color: 'bg-blue-600', path: '/http' },
  websocket: { color: 'bg-green-600', path: '/websocket' },
  grpc: { color: 'bg-orange-600', path: '/grpc' },
}

export function Dashboard() {
  const qc = useQueryClient()
  const { data: protocols = [], isLoading } = useQuery({
    queryKey: ['protocols'],
    queryFn: getProtocols,
    refetchInterval: 5000,
  })

  const reset = useMutation({
    mutationFn: resetAll,
    onSuccess: () => qc.invalidateQueries(),
  })

  return (
    <PageShell
      title="Dashboard"
      actions={
        <Button variant="danger" size="sm" onClick={() => reset.mutate()}>
          Reset All
        </Button>
      }
    >
      {isLoading && <p className="text-zinc-500 text-sm">Loading...</p>}

      <div className="grid grid-cols-1 sm:grid-cols-3 gap-4 mb-8">
        {protocols.map((p) => {
          const meta = PROTOCOL_META[p.protocol] ?? { color: 'bg-zinc-600', path: '/' }
          return (
            <Link key={p.protocol} to={meta.path}>
              <div className="rounded-xl border border-zinc-800 bg-zinc-900 p-5 hover:border-violet-700 transition-colors cursor-pointer">
                <div className="flex items-start justify-between mb-3">
                  <span className={`text-xs font-semibold px-2 py-0.5 rounded-full text-white ${meta.color}`}>
                    {p.protocol.toUpperCase()}
                  </span>
                  <span className={`text-xs px-1.5 py-0.5 rounded font-medium ${p.enabled ? 'text-green-400 bg-green-900/30' : 'text-zinc-500 bg-zinc-800'}`}>
                    {p.enabled ? 'running' : 'disabled'}
                  </span>
                </div>
                <div className="text-2xl font-bold text-zinc-100">{p.mocks}</div>
                <div className="text-xs text-zinc-500 mt-0.5">mocks · port {p.port}</div>
              </div>
            </Link>
          )
        })}
      </div>

      <div className="rounded-xl border border-zinc-800 bg-zinc-900 p-5">
        <h2 className="text-sm font-semibold text-zinc-400 mb-3">Quick start</h2>
        <pre className="text-xs text-zinc-300 bg-zinc-950 rounded p-3 overflow-x-auto">{`# Start Mockly
mockly start --config mockly.yaml

# Add an HTTP mock at runtime
mockly add http --method GET --path /api/users --status 200 --body '[{"id":1}]'

# View all mocks
mockly list

# Reset state
mockly reset`}</pre>
      </div>
    </PageShell>
  )
}
