import { useState } from 'react'
import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query'
import { getGRPCMocks, createGRPCMock, updateGRPCMock, deleteGRPCMock } from '../api/client'
import type { GRPCMock } from '../types'
import { PageShell } from '../components/PageShell'
import { Button } from '../components/Button'

const emptyMock = (): Omit<GRPCMock, 'id'> => ({
  method: 'GetUser',
  response: { id: '1', name: 'Alice' },
})

export function GRPCPage() {
  const qc = useQueryClient()
  const { data: mocks = [], isLoading } = useQuery({ queryKey: ['mocks-grpc'], queryFn: getGRPCMocks })
  const [editing, setEditing] = useState<GRPCMock | null>(null)
  const [adding, setAdding] = useState(false)
  const [draft, setDraft] = useState<Omit<GRPCMock, 'id'>>(emptyMock())

  const create = useMutation({
    mutationFn: createGRPCMock,
    onSuccess: () => { qc.invalidateQueries({ queryKey: ['mocks-grpc'] }); setAdding(false); setDraft(emptyMock()) },
  })
  const update = useMutation({
    mutationFn: ({ id, m }: { id: string; m: GRPCMock }) => updateGRPCMock(id, m),
    onSuccess: () => { qc.invalidateQueries({ queryKey: ['mocks-grpc'] }); setEditing(null) },
  })
  const remove = useMutation({
    mutationFn: deleteGRPCMock,
    onSuccess: () => qc.invalidateQueries({ queryKey: ['mocks-grpc'] }),
  })

  return (
    <PageShell
      title="gRPC Mocks"
      actions={<Button size="sm" onClick={() => { setAdding(true); setDraft(emptyMock()) }}>+ Add Mock</Button>}
    >
      {isLoading && <p className="text-zinc-500 text-sm">Loading...</p>}

      {(adding || editing) && (
        <MockForm
          value={editing ?? draft}
          onCancel={() => { setAdding(false); setEditing(null) }}
          onSave={(m) => {
            if (editing) update.mutate({ id: editing.id, m: { ...m, id: editing.id } as GRPCMock })
            else create.mutate(m)
          }}
          onChange={(m) => editing ? setEditing({ ...editing, ...m }) : setDraft(m)}
        />
      )}

      <div className="space-y-2 mt-4">
        {mocks.map((m) => (
          <div key={m.id} className="flex items-center gap-3 rounded-lg border border-zinc-800 bg-zinc-900 px-4 py-3">
            <span className="text-xs font-bold text-orange-400 font-mono">gRPC</span>
            <span className="font-mono text-sm text-zinc-300 flex-1">{m.method}</span>
            {m.error && (
              <span className="text-xs px-1.5 py-0.5 rounded bg-red-900/30 text-red-400 font-mono">
                error {m.error.code}
              </span>
            )}
            {m.delay && <span className="text-xs text-zinc-500">{m.delay}</span>}
            <span className="text-xs text-zinc-600 font-mono">{m.id}</span>
            <Button size="sm" variant="ghost" onClick={() => setEditing(m)}>Edit</Button>
            <Button size="sm" variant="danger" onClick={() => remove.mutate(m.id)}>✕</Button>
          </div>
        ))}
        {mocks.length === 0 && !isLoading && (
          <p className="text-zinc-600 text-sm text-center py-8">No gRPC mocks configured.</p>
        )}
      </div>
    </PageShell>
  )
}

interface FormProps {
  value: Omit<GRPCMock, 'id'>
  onChange: (m: Omit<GRPCMock, 'id'>) => void
  onSave: (m: Omit<GRPCMock, 'id'>) => void
  onCancel: () => void
}

function MockForm({ value, onChange, onSave, onCancel }: FormProps) {
  return (
    <div className="rounded-xl border border-violet-700 bg-zinc-900 p-5 mb-4">
      <h3 className="text-sm font-semibold text-zinc-300 mb-4">Configure gRPC Mock</h3>
      <div className="grid grid-cols-2 gap-4">
        <label className="col-span-2 flex flex-col gap-1">
          <span className="text-xs text-zinc-500">Method name (e.g. GetUser or *)</span>
          <input
            className="bg-zinc-800 border border-zinc-700 rounded px-2 py-1.5 text-sm text-zinc-200 font-mono"
            value={value.method}
            onChange={(e) => onChange({ ...value, method: e.target.value })}
          />
        </label>
        <label className="flex flex-col gap-1">
          <span className="text-xs text-zinc-500">Delay (e.g. 50ms)</span>
          <input
            className="bg-zinc-800 border border-zinc-700 rounded px-2 py-1.5 text-sm text-zinc-200 font-mono"
            value={value.delay ?? ''}
            onChange={(e) => onChange({ ...value, delay: e.target.value })}
          />
        </label>
        <label className="col-span-2 flex flex-col gap-1">
          <span className="text-xs text-zinc-500">Response body (JSON object)</span>
          <textarea
            rows={5}
            className="bg-zinc-800 border border-zinc-700 rounded px-2 py-1.5 text-sm text-zinc-200 font-mono resize-y"
            defaultValue={JSON.stringify(value.response ?? {}, null, 2)}
            onBlur={(e) => {
              try { onChange({ ...value, response: JSON.parse(e.target.value) }) } catch { /* keep */ }
            }}
          />
        </label>
      </div>
      <div className="flex gap-2 mt-4">
        <Button onClick={() => onSave(value)}>Save</Button>
        <Button variant="ghost" onClick={onCancel}>Cancel</Button>
      </div>
    </div>
  )
}
