import { useEffect, useRef, useState } from 'react'
import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query'
import { getLogs, clearLogs } from '../api/client'
import type { LogEntry } from '../types'
import { PageShell } from '../components/PageShell'
import { Button } from '../components/Button'

const PROTOCOL_COLOR: Record<string, string> = {
  http: 'text-blue-400',
  websocket: 'text-green-400',
  grpc: 'text-orange-400',
}

const STATUS_COLOR = (s?: number) => {
  if (!s) return 'text-zinc-500'
  if (s < 300) return 'text-green-400'
  if (s < 400) return 'text-yellow-400'
  return 'text-red-400'
}

export function LogsPage() {
  const qc = useQueryClient()
  const { data: initial = [] } = useQuery({ queryKey: ['logs'], queryFn: getLogs })
  const [entries, setEntries] = useState<LogEntry[]>([])
  const [live, setLive] = useState(true)
  const bottomRef = useRef<HTMLDivElement>(null)

  useEffect(() => { setEntries(initial) }, [initial])

  useEffect(() => {
    if (!live) return
    const es = new EventSource('/api/logs/stream')
    es.onmessage = (e) => {
      const entry: LogEntry = JSON.parse(e.data)
      setEntries((prev) => [...prev.slice(-499), entry])
    }
    return () => es.close()
  }, [live])

  useEffect(() => {
    if (live) bottomRef.current?.scrollIntoView({ behavior: 'smooth' })
  }, [entries, live])

  const clear = useMutation({
    mutationFn: clearLogs,
    onSuccess: () => { qc.invalidateQueries({ queryKey: ['logs'] }); setEntries([]) },
  })

  return (
    <PageShell
      title="Request Logs"
      actions={
        <>
          <Button size="sm" variant={live ? 'primary' : 'ghost'} onClick={() => setLive((v) => !v)}>
            {live ? '● Live' : '○ Paused'}
          </Button>
          <Button size="sm" variant="danger" onClick={() => clear.mutate()}>Clear</Button>
        </>
      }
    >
      <div className="font-mono text-xs space-y-0.5">
        {entries.length === 0 && (
          <p className="text-zinc-600 text-center py-8 not-mono text-sm">No requests logged yet.</p>
        )}
        {entries.map((e) => (
          <div key={e.id} className="flex items-baseline gap-3 px-3 py-1.5 rounded hover:bg-zinc-900 group">
            <span className="text-zinc-600 w-20 shrink-0">
              {new Date(e.timestamp).toLocaleTimeString()}
            </span>
            <span className={`w-12 shrink-0 font-bold ${PROTOCOL_COLOR[e.protocol] ?? 'text-zinc-400'}`}>
              {e.protocol.toUpperCase()}
            </span>
            {e.method && <span className="text-zinc-400 w-14 shrink-0">{e.method}</span>}
            <span className="text-zinc-200 flex-1 truncate">{e.path}</span>
            <span className={`w-10 text-right shrink-0 ${STATUS_COLOR(e.status)}`}>{e.status}</span>
            <span className="text-zinc-600 w-16 text-right shrink-0">{e.duration_ms}ms</span>
            {e.matched_id && (
              <span className="text-violet-500 shrink-0 hidden group-hover:inline">{e.matched_id}</span>
            )}
          </div>
        ))}
        <div ref={bottomRef} />
      </div>
    </PageShell>
  )
}
