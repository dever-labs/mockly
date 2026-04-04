import { useState } from 'react'
import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query'
import { getWSMocks, createWSMock, updateWSMock, deleteWSMock } from '../api/client'
import type { WebSocketMock } from '../types'
import { PageShell } from '../components/PageShell'
import { Button } from '../components/Button'

const emptyMock = (): Omit<WebSocketMock, 'id'> => ({
  path: '/ws/example',
  on_connect: { send: 'connected' },
  on_message: [{ match: 'ping', respond: 'pong' }],
})

export function WebSocketPage() {
  const qc = useQueryClient()
  const { data: mocks = [], isLoading } = useQuery({ queryKey: ['mocks-ws'], queryFn: getWSMocks })
  const [editing, setEditing] = useState<WebSocketMock | null>(null)
  const [adding, setAdding] = useState(false)
  const [draft, setDraft] = useState<Omit<WebSocketMock, 'id'>>(emptyMock())

  const create = useMutation({
    mutationFn: createWSMock,
    onSuccess: () => { qc.invalidateQueries({ queryKey: ['mocks-ws'] }); setAdding(false); setDraft(emptyMock()) },
  })
  const update = useMutation({
    mutationFn: ({ id, m }: { id: string; m: WebSocketMock }) => updateWSMock(id, m),
    onSuccess: () => { qc.invalidateQueries({ queryKey: ['mocks-ws'] }); setEditing(null) },
  })
  const remove = useMutation({
    mutationFn: deleteWSMock,
    onSuccess: () => qc.invalidateQueries({ queryKey: ['mocks-ws'] }),
  })

  return (
    <PageShell
      title="WebSocket Mocks"
      actions={<Button size="sm" onClick={() => { setAdding(true); setDraft(emptyMock()) }}>+ Add Mock</Button>}
    >
      {isLoading && <p className="text-zinc-500 text-sm">Loading...</p>}

      {(adding || editing) && (
        <MockForm
          value={editing ?? draft}
          onCancel={() => { setAdding(false); setEditing(null) }}
          onSave={(m) => {
            if (editing) update.mutate({ id: editing.id, m: { ...m, id: editing.id } as WebSocketMock })
            else create.mutate(m)
          }}
          onChange={(m) => editing ? setEditing({ ...editing, ...m }) : setDraft(m as Omit<WebSocketMock, 'id'>)}
        />
      )}

      <div className="space-y-2 mt-4">
        {mocks.map((m) => (
          <div key={m.id} className="rounded-lg border border-zinc-800 bg-zinc-900 px-4 py-3">
            <div className="flex items-center gap-3 mb-2">
              <span className="text-xs font-bold text-green-400 font-mono">WS</span>
              <span className="font-mono text-sm text-zinc-300 flex-1">{m.path}</span>
              <span className="text-xs text-zinc-600 font-mono">{m.id}</span>
              <Button size="sm" variant="ghost" onClick={() => setEditing(m)}>Edit</Button>
              <Button size="sm" variant="danger" onClick={() => remove.mutate(m.id)}>✕</Button>
            </div>
            {m.on_connect?.send && (
              <div className="text-xs text-zinc-500 ml-10">
                on_connect → <span className="text-zinc-300 font-mono">"{m.on_connect.send}"</span>
              </div>
            )}
            {m.on_message?.map((r, i) => (
              <div key={i} className="text-xs text-zinc-500 ml-10">
                on_message <span className="text-violet-400 font-mono">"{r.match}"</span> →{' '}
                <span className="text-zinc-300 font-mono">"{r.respond}"</span>
              </div>
            ))}
          </div>
        ))}
        {mocks.length === 0 && !isLoading && (
          <p className="text-zinc-600 text-sm text-center py-8">No WebSocket mocks configured.</p>
        )}
      </div>
    </PageShell>
  )
}

interface FormProps {
  value: Omit<WebSocketMock, 'id'>
  onChange: (m: Omit<WebSocketMock, 'id'>) => void
  onSave: (m: Omit<WebSocketMock, 'id'>) => void
  onCancel: () => void
}

function MockForm({ value, onChange, onSave, onCancel }: FormProps) {
  const rulesJson = JSON.stringify(value.on_message ?? [], null, 2)

  return (
    <div className="rounded-xl border border-violet-700 bg-zinc-900 p-5 mb-4">
      <h3 className="text-sm font-semibold text-zinc-300 mb-4">Configure WebSocket Mock</h3>
      <div className="grid grid-cols-2 gap-4">
        <label className="col-span-2 flex flex-col gap-1">
          <span className="text-xs text-zinc-500">Path</span>
          <input
            className="bg-zinc-800 border border-zinc-700 rounded px-2 py-1.5 text-sm text-zinc-200 font-mono"
            value={value.path}
            onChange={(e) => onChange({ ...value, path: e.target.value })}
          />
        </label>
        <label className="col-span-2 flex flex-col gap-1">
          <span className="text-xs text-zinc-500">On connect — send message</span>
          <input
            className="bg-zinc-800 border border-zinc-700 rounded px-2 py-1.5 text-sm text-zinc-200 font-mono"
            value={value.on_connect?.send ?? ''}
            onChange={(e) => onChange({ ...value, on_connect: { ...value.on_connect, send: e.target.value } })}
          />
        </label>
        <label className="col-span-2 flex flex-col gap-1">
          <span className="text-xs text-zinc-500">on_message rules (JSON array)</span>
          <textarea
            rows={5}
            className="bg-zinc-800 border border-zinc-700 rounded px-2 py-1.5 text-sm text-zinc-200 font-mono resize-y"
            defaultValue={rulesJson}
            onBlur={(e) => {
              try {
                const parsed = JSON.parse(e.target.value)
                onChange({ ...value, on_message: parsed })
              } catch { /* keep current */ }
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
