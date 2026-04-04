import { useState } from 'react'
import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query'
import { getState, setState, deleteStateKey } from '../api/client'
import { PageShell } from '../components/PageShell'
import { Button } from '../components/Button'

export function StatePage() {
  const qc = useQueryClient()
  const { data: state = {}, isLoading } = useQuery({ queryKey: ['state'], queryFn: getState, refetchInterval: 3000 })

  const [newKey, setNewKey] = useState('')
  const [newVal, setNewVal] = useState('')

  const set = useMutation({
    mutationFn: (data: Record<string, string>) => setState(data),
    onSuccess: () => { qc.invalidateQueries({ queryKey: ['state'] }); setNewKey(''); setNewVal('') },
  })
  const del = useMutation({
    mutationFn: deleteStateKey,
    onSuccess: () => qc.invalidateQueries({ queryKey: ['state'] }),
  })

  return (
    <PageShell title="Runtime State">
      {isLoading && <p className="text-zinc-500 text-sm">Loading...</p>}

      <div className="mb-6 rounded-xl border border-zinc-800 bg-zinc-900 p-5">
        <h3 className="text-sm font-semibold text-zinc-400 mb-3">Set state variable</h3>
        <div className="flex gap-2">
          <input
            placeholder="key"
            className="bg-zinc-800 border border-zinc-700 rounded px-2 py-1.5 text-sm text-zinc-200 font-mono flex-1"
            value={newKey}
            onChange={(e) => setNewKey(e.target.value)}
          />
          <input
            placeholder="value"
            className="bg-zinc-800 border border-zinc-700 rounded px-2 py-1.5 text-sm text-zinc-200 font-mono flex-1"
            value={newVal}
            onChange={(e) => setNewVal(e.target.value)}
          />
          <Button onClick={() => set.mutate({ [newKey]: newVal })} disabled={!newKey}>Set</Button>
        </div>
        <p className="text-xs text-zinc-600 mt-2">
          State variables can be used in mock conditions (e.g. fire a mock only when <code className="text-zinc-400">authenticated=true</code>).
        </p>
      </div>

      <div className="space-y-2">
        {Object.entries(state).map(([k, v]) => (
          <div key={k} className="flex items-center gap-3 rounded-lg border border-zinc-800 bg-zinc-900 px-4 py-3">
            <span className="font-mono text-sm text-violet-400 flex-1">{k}</span>
            <span className="font-mono text-sm text-zinc-300 flex-1">{v}</span>
            <Button size="sm" variant="danger" onClick={() => del.mutate(k)}>✕</Button>
          </div>
        ))}
        {Object.keys(state).length === 0 && !isLoading && (
          <p className="text-zinc-600 text-sm text-center py-8">No state variables set.</p>
        )}
      </div>
    </PageShell>
  )
}
