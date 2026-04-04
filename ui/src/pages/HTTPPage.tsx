import { useState } from 'react'
import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query'
import { getHTTPMocks, createHTTPMock, updateHTTPMock, deleteHTTPMock } from '../api/client'
import type { HTTPMock } from '../types'
import { PageShell } from '../components/PageShell'
import { Button } from '../components/Button'

const METHODS = ['GET', 'POST', 'PUT', 'PATCH', 'DELETE', 'OPTIONS', 'HEAD']

const emptyMock = (): Omit<HTTPMock, 'id'> => ({
  request: { method: 'GET', path: '/api/example' },
  response: { status: 200, headers: { 'Content-Type': 'application/json' }, body: '{"ok":true}' },
})

export function HTTPPage() {
  const qc = useQueryClient()
  const { data: mocks = [], isLoading } = useQuery({ queryKey: ['mocks-http'], queryFn: getHTTPMocks })

  const [editing, setEditing] = useState<HTTPMock | null>(null)
  const [adding, setAdding] = useState(false)
  const [draft, setDraft] = useState<Omit<HTTPMock, 'id'>>(emptyMock())

  const create = useMutation({
    mutationFn: createHTTPMock,
    onSuccess: () => { qc.invalidateQueries({ queryKey: ['mocks-http'] }); setAdding(false); setDraft(emptyMock()) },
  })
  const update = useMutation({
    mutationFn: ({ id, m }: { id: string; m: HTTPMock }) => updateHTTPMock(id, m),
    onSuccess: () => { qc.invalidateQueries({ queryKey: ['mocks-http'] }); setEditing(null) },
  })
  const remove = useMutation({
    mutationFn: deleteHTTPMock,
    onSuccess: () => qc.invalidateQueries({ queryKey: ['mocks-http'] }),
  })

  const methodColor = (m: string) => ({
    GET: 'text-green-400', POST: 'text-blue-400', PUT: 'text-yellow-400',
    PATCH: 'text-orange-400', DELETE: 'text-red-400',
  }[m] ?? 'text-zinc-400')

  return (
    <PageShell
      title="HTTP Mocks"
      actions={<Button size="sm" onClick={() => { setAdding(true); setDraft(emptyMock()) }}>+ Add Mock</Button>}
    >
      {isLoading && <p className="text-zinc-500 text-sm">Loading...</p>}

      {(adding || editing) && (
        <MockForm
          value={editing ?? draft}
          onCancel={() => { setAdding(false); setEditing(null) }}
          onSave={(m) => {
            if (editing) update.mutate({ id: editing.id, m: { ...m, id: editing.id } as HTTPMock })
            else create.mutate(m)
          }}
          onChange={(m) => editing ? setEditing({ ...editing, ...m }) : setDraft(m)}
        />
      )}

      <div className="space-y-2 mt-4">
        {mocks.map((m) => (
          <div key={m.id} className="flex items-center gap-3 rounded-lg border border-zinc-800 bg-zinc-900 px-4 py-3">
            <span className={`font-mono text-xs font-bold w-16 ${methodColor(m.request.method)}`}>
              {m.request.method}
            </span>
            <span className="font-mono text-sm text-zinc-300 flex-1">{m.request.path}</span>
            <span className={`text-xs px-1.5 py-0.5 rounded font-mono ${m.response.status < 400 ? 'bg-green-900/30 text-green-400' : 'bg-red-900/30 text-red-400'}`}>
              {m.response.status}
            </span>
            {m.response.delay && (
              <span className="text-xs text-zinc-500">{m.response.delay}</span>
            )}
            <span className="text-xs text-zinc-600 font-mono">{m.id}</span>
            <Button size="sm" variant="ghost" onClick={() => setEditing(m)}>Edit</Button>
            <Button size="sm" variant="danger" onClick={() => remove.mutate(m.id)}>✕</Button>
          </div>
        ))}
        {mocks.length === 0 && !isLoading && (
          <p className="text-zinc-600 text-sm text-center py-8">No HTTP mocks configured. Add one above.</p>
        )}
      </div>
    </PageShell>
  )
}

interface FormProps {
  value: Omit<HTTPMock, 'id'>
  onChange: (m: Omit<HTTPMock, 'id'>) => void
  onSave: (m: Omit<HTTPMock, 'id'>) => void
  onCancel: () => void
}

function MockForm({ value, onChange, onSave, onCancel }: FormProps) {
  const setReq = (k: keyof HTTPMock['request'], v: string) =>
    onChange({ ...value, request: { ...value.request, [k]: v } })
  const setRes = (k: keyof HTTPMock['response'], v: string | number) =>
    onChange({ ...value, response: { ...value.response, [k]: v } })

  return (
    <div className="rounded-xl border border-violet-700 bg-zinc-900 p-5 mb-4">
      <h3 className="text-sm font-semibold text-zinc-300 mb-4">Configure Mock</h3>
      <div className="grid grid-cols-2 gap-4">
        <label className="flex flex-col gap-1">
          <span className="text-xs text-zinc-500">Method</span>
          <select
            className="bg-zinc-800 border border-zinc-700 rounded px-2 py-1.5 text-sm text-zinc-200"
            value={value.request.method}
            onChange={(e) => setReq('method', e.target.value)}
          >
            {['GET','POST','PUT','PATCH','DELETE','OPTIONS','HEAD'].map((m) => (
              <option key={m}>{m}</option>
            ))}
          </select>
        </label>
        <label className="flex flex-col gap-1">
          <span className="text-xs text-zinc-500">Path</span>
          <input
            className="bg-zinc-800 border border-zinc-700 rounded px-2 py-1.5 text-sm text-zinc-200 font-mono"
            value={value.request.path}
            onChange={(e) => setReq('path', e.target.value)}
          />
        </label>
        <label className="flex flex-col gap-1">
          <span className="text-xs text-zinc-500">Status Code</span>
          <input
            type="number"
            className="bg-zinc-800 border border-zinc-700 rounded px-2 py-1.5 text-sm text-zinc-200"
            value={value.response.status}
            onChange={(e) => setRes('status', parseInt(e.target.value, 10))}
          />
        </label>
        <label className="flex flex-col gap-1">
          <span className="text-xs text-zinc-500">Delay (e.g. 100ms)</span>
          <input
            className="bg-zinc-800 border border-zinc-700 rounded px-2 py-1.5 text-sm text-zinc-200 font-mono"
            value={value.response.delay ?? ''}
            onChange={(e) => setRes('delay', e.target.value)}
          />
        </label>
        <label className="col-span-2 flex flex-col gap-1">
          <span className="text-xs text-zinc-500">Response Body</span>
          <textarea
            rows={4}
            className="bg-zinc-800 border border-zinc-700 rounded px-2 py-1.5 text-sm text-zinc-200 font-mono resize-y"
            value={value.response.body ?? ''}
            onChange={(e) => setRes('body', e.target.value)}
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

export { METHODS }
