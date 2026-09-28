import { useState } from 'react'
import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query'
import {
  getAllFaults, getProtocolFault, getEffectiveProtocolFault,
  setProtocolFault, clearProtocolFault, clearAllFaults,
} from '../api/client'
import { FAULT_PROTOCOLS } from '../types'
import { PageShell } from '../components/PageShell'
import { Button } from '../components/Button'

export function FaultPage() {
  const qc = useQueryClient()
  const [protocol, setProtocol] = useState<string>(FAULT_PROTOCOLS[0])
  const [draft, setDraft] = useState('{}')
  const [error, setError] = useState<string | null>(null)

  const { data: allFaults = {} } = useQuery({ queryKey: ['faults'], queryFn: getAllFaults, refetchInterval: 5000 })

  const { data: fault } = useQuery({
    queryKey: ['fault', protocol],
    queryFn: () => getProtocolFault(protocol),
    refetchInterval: 5000,
  })

  const { data: effective, refetch: refetchEffective } = useQuery({
    queryKey: ['fault-effective', protocol],
    queryFn: () => getEffectiveProtocolFault(protocol),
    enabled: false,
  })

  const set = useMutation({
    mutationFn: (body: unknown) => setProtocolFault(protocol, body),
    onSuccess: () => {
      setError(null)
      qc.invalidateQueries({ queryKey: ['fault', protocol] })
      qc.invalidateQueries({ queryKey: ['faults'] })
    },
    onError: (e: Error) => setError(e.message),
  })
  const clear = useMutation({
    mutationFn: () => clearProtocolFault(protocol),
    onSuccess: () => {
      qc.invalidateQueries({ queryKey: ['fault', protocol] })
      qc.invalidateQueries({ queryKey: ['faults'] })
    },
  })
  const clearAll = useMutation({
    mutationFn: clearAllFaults,
    onSuccess: () => qc.invalidateQueries({ queryKey: ['faults'] }),
  })

  return (
    <PageShell
      title="Fault Injection"
      actions={<Button size="sm" variant="danger" onClick={() => clearAll.mutate()}>Clear All</Button>}
    >
      <div className="rounded-xl border border-zinc-800 bg-zinc-900 p-5 mb-6">
        <h3 className="text-sm font-semibold text-zinc-400 mb-3">Set a direct protocol fault</h3>
        <div className="flex gap-2 mb-3">
          <select
            className="bg-zinc-800 border border-zinc-700 rounded px-2 py-1.5 text-sm text-zinc-200 font-mono"
            value={protocol}
            onChange={(e) => { setProtocol(e.target.value); setDraft('{}') }}
          >
            {FAULT_PROTOCOLS.map((p) => (
              <option key={p} value={p}>{p}</option>
            ))}
          </select>
          <Button size="sm" variant="ghost" onClick={() => refetchEffective()}>Get effective</Button>
        </div>
        <textarea
          rows={5}
          className="w-full bg-zinc-800 border border-zinc-700 rounded px-2 py-1.5 text-sm text-zinc-200 font-mono resize-y"
          placeholder='e.g. {"delay":"200ms","error_rate":0.5}'
          value={draft}
          onChange={(e) => setDraft(e.target.value)}
        />
        {error && <p className="text-xs text-red-400 mt-2">{error}</p>}
        <div className="flex gap-2 mt-3">
          <Button
            size="sm"
            onClick={() => {
              try {
                set.mutate(JSON.parse(draft))
              } catch {
                setError('Invalid JSON')
              }
            }}
          >
            Set fault
          </Button>
          <Button size="sm" variant="danger" onClick={() => clear.mutate()}>Clear {protocol}</Button>
        </div>

        <div className="grid grid-cols-2 gap-4 mt-4">
          <div>
            <div className="text-xs text-zinc-500 mb-1">Current direct fault</div>
            <pre className="bg-zinc-950 border border-zinc-800 rounded p-2 text-xs text-zinc-300 font-mono overflow-auto max-h-40">
              {JSON.stringify(fault ?? null, null, 2)}
            </pre>
          </div>
          <div>
            <div className="text-xs text-zinc-500 mb-1">Effective fault (direct + active scenario)</div>
            <pre className="bg-zinc-950 border border-zinc-800 rounded p-2 text-xs text-zinc-300 font-mono overflow-auto max-h-40">
              {JSON.stringify(effective ?? null, null, 2)}
            </pre>
          </div>
        </div>
      </div>

      <h3 className="text-sm font-semibold text-zinc-400 mb-3">All active direct faults</h3>
      <pre className="bg-zinc-950 border border-zinc-800 rounded p-4 text-xs text-zinc-300 font-mono overflow-auto">
        {Object.keys(allFaults).length === 0
          ? 'No direct faults active.'
          : JSON.stringify(allFaults, null, 2)}
      </pre>
    </PageShell>
  )
}
