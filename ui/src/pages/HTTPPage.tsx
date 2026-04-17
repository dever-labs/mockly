import { useState, useRef } from 'react'
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
          key={editing?.id ?? 'new'}
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
            <span className="font-mono text-sm text-zinc-300">
              {m.request.path}
              {m.request.query && Object.keys(m.request.query).length > 0 && (
                <span className="ml-1 text-zinc-500">
                  ?{new URLSearchParams(m.request.query).toString()}
                </span>
              )}
            </span>
            <span className="flex-1" />
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

function KVEditor({
  label,
  hint,
  value,
  onChange,
}: {
  label: string
  hint?: string
  value: Record<string, string>
  onChange: (v: Record<string, string>) => void
}) {
  type Row = { id: number; key: string; val: string }
  const [rows, setRows] = useState<Row[]>(() =>
    Object.entries(value).map(([k, v], i) => ({ id: i, key: k, val: v }))
  )
  const nextId = useRef(rows.length)

  const emit = (newRows: Row[]) => {
    onChange(Object.fromEntries(newRows.filter((r) => r.key !== '').map((r) => [r.key, r.val])))
  }

  const add = () =>
    setRows((prev) => [...prev, { id: nextId.current++, key: '', val: '' }])

  const remove = (id: number) => {
    const next = rows.filter((r) => r.id !== id)
    setRows(next)
    emit(next)
  }

  const update = (id: number, key: string, val: string) => {
    const next = rows.map((r) => (r.id === id ? { ...r, key, val } : r))
    setRows(next)
    emit(next)
  }

  return (
    <div className="col-span-2 flex flex-col gap-1">
      <div className="flex items-center gap-2">
        <span className="text-xs text-zinc-500">{label}</span>
        {hint && <span className="text-xs text-zinc-600 italic">{hint}</span>}
        <button
          type="button"
          className="ml-auto text-xs text-violet-400 hover:text-violet-300"
          onClick={add}
        >
          + Add
        </button>
      </div>
      {rows.length > 0 && (
        <div className="flex flex-col gap-1">
          {rows.map((r) => (
            <div key={r.id} className="flex gap-2 items-center">
              <input
                className="bg-zinc-800 border border-zinc-700 rounded px-2 py-1 text-xs text-zinc-200 font-mono flex-1"
                placeholder="key"
                value={r.key}
                onChange={(e) => update(r.id, e.target.value, r.val)}
              />
              <input
                className="bg-zinc-800 border border-zinc-700 rounded px-2 py-1 text-xs text-zinc-200 font-mono flex-1"
                placeholder="value or * (wildcard)"
                value={r.val}
                onChange={(e) => update(r.id, r.key, e.target.value)}
              />
              <button
                type="button"
                className="text-zinc-600 hover:text-red-400 text-xs px-1"
                onClick={() => remove(r.id)}
              >
                ✕
              </button>
            </div>
          ))}
        </div>
      )}
    </div>
  )
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
        <KVEditor
          label="Query Params"
          hint="use * as wildcard value"
          value={value.request.query ?? {}}
          onChange={(q) =>
            onChange({ ...value, request: { ...value.request, query: Object.keys(q).length ? q : undefined } })
          }
        />
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
