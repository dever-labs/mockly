import { useState } from 'react'
import { useQuery, useMutation, useQueryClient } from '@tanstack/react-query'
import {
  getScenarios, getActiveScenarios, createScenario, updateScenario,
  deleteScenario, activateScenario, deactivateScenario,
} from '../api/client'
import type { Scenario } from '../types'
import { PageShell } from '../components/PageShell'
import { Button } from '../components/Button'

const emptyScenario = (): Omit<Scenario, 'id'> => ({
  name: 'new-scenario',
  description: '',
  patches: [],
})

export function ScenariosPage() {
  const qc = useQueryClient()
  const { data: scenarios = [], isLoading } = useQuery({ queryKey: ['scenarios'], queryFn: getScenarios })
  const { data: activeData } = useQuery({ queryKey: ['scenarios-active'], queryFn: getActiveScenarios, refetchInterval: 5000 })
  const active = new Set(activeData?.active ?? [])

  const [editing, setEditing] = useState<Scenario | null>(null)
  const [adding, setAdding] = useState(false)
  const [draft, setDraft] = useState<Omit<Scenario, 'id'>>(emptyScenario())

  const invalidate = () => {
    qc.invalidateQueries({ queryKey: ['scenarios'] })
    qc.invalidateQueries({ queryKey: ['scenarios-active'] })
  }

  const create = useMutation({
    mutationFn: createScenario,
    onSuccess: () => { invalidate(); setAdding(false); setDraft(emptyScenario()) },
  })
  const update = useMutation({
    mutationFn: ({ id, sc }: { id: string; sc: Scenario }) => updateScenario(id, sc),
    onSuccess: () => { invalidate(); setEditing(null) },
  })
  const remove = useMutation({ mutationFn: deleteScenario, onSuccess: invalidate })
  const activate = useMutation({ mutationFn: activateScenario, onSuccess: invalidate })
  const deactivate = useMutation({ mutationFn: deactivateScenario, onSuccess: invalidate })

  return (
    <PageShell
      title="Scenarios"
      actions={<Button size="sm" onClick={() => { setAdding(true); setDraft(emptyScenario()) }}>+ Add Scenario</Button>}
    >
      {isLoading && <p className="text-zinc-500 text-sm">Loading...</p>}

      {(adding || editing) && (
        <ScenarioForm
          key={editing?.id ?? 'new'}
          value={editing ?? draft}
          onCancel={() => { setAdding(false); setEditing(null) }}
          onSave={(sc) => {
            if (editing) update.mutate({ id: editing.id, sc: { ...sc, id: editing.id } as Scenario })
            else create.mutate(sc)
          }}
          onChange={(sc) => (editing ? setEditing({ ...editing, ...sc }) : setDraft(sc))}
        />
      )}

      <div className="space-y-2 mt-4">
        {scenarios.map((sc) => {
          const isActive = active.has(sc.id)
          return (
            <div key={sc.id} className="rounded-lg border border-zinc-800 bg-zinc-900 px-4 py-3">
              <div className="flex items-center gap-3">
                <span className={`text-xs px-1.5 py-0.5 rounded font-medium ${isActive ? 'text-green-400 bg-green-900/30' : 'text-zinc-500 bg-zinc-800'}`}>
                  {isActive ? 'active' : 'inactive'}
                </span>
                <span className="font-mono text-sm text-zinc-200">{sc.name}</span>
                <span className="text-xs text-zinc-600 font-mono">{sc.id}</span>
                <span className="flex-1" />
                <span className="text-xs text-zinc-500">{sc.patches?.length ?? 0} patches</span>
                {isActive ? (
                  <Button size="sm" variant="danger" onClick={() => deactivate.mutate(sc.id)}>Deactivate</Button>
                ) : (
                  <Button size="sm" onClick={() => activate.mutate(sc.id)}>Activate</Button>
                )}
                <Button size="sm" variant="ghost" onClick={() => setEditing(sc)}>Edit</Button>
                <Button size="sm" variant="danger" onClick={() => remove.mutate(sc.id)}>✕</Button>
              </div>
              {sc.description && <p className="text-xs text-zinc-500 mt-2">{sc.description}</p>}
            </div>
          )
        })}
        {scenarios.length === 0 && !isLoading && (
          <p className="text-zinc-600 text-sm text-center py-8">No scenarios configured. Add one above.</p>
        )}
      </div>
    </PageShell>
  )
}

interface FormProps {
  value: Omit<Scenario, 'id'>
  onChange: (sc: Omit<Scenario, 'id'>) => void
  onSave: (sc: Omit<Scenario, 'id'>) => void
  onCancel: () => void
}

function ScenarioForm({ value, onChange, onSave, onCancel }: FormProps) {
  return (
    <div className="rounded-xl border border-violet-700 bg-zinc-900 p-5 mb-4">
      <h3 className="text-sm font-semibold text-zinc-300 mb-4">Configure Scenario</h3>
      <div className="grid grid-cols-2 gap-4">
        <label className="flex flex-col gap-1">
          <span className="text-xs text-zinc-500">Name</span>
          <input
            className="bg-zinc-800 border border-zinc-700 rounded px-2 py-1.5 text-sm text-zinc-200 font-mono"
            value={value.name}
            onChange={(e) => onChange({ ...value, name: e.target.value })}
          />
        </label>
        <label className="flex flex-col gap-1">
          <span className="text-xs text-zinc-500">Description</span>
          <input
            className="bg-zinc-800 border border-zinc-700 rounded px-2 py-1.5 text-sm text-zinc-200 font-mono"
            value={value.description ?? ''}
            onChange={(e) => onChange({ ...value, description: e.target.value })}
          />
        </label>
        <label className="col-span-2 flex flex-col gap-1">
          <span className="text-xs text-zinc-500">Mock patches (JSON array — mock_id, status, headers, body, delay, disabled)</span>
          <textarea
            rows={8}
            className="bg-zinc-800 border border-zinc-700 rounded px-2 py-1.5 text-sm text-zinc-200 font-mono resize-y"
            defaultValue={JSON.stringify(value.patches ?? [], null, 2)}
            onBlur={(e) => {
              try { onChange({ ...value, patches: JSON.parse(e.target.value) }) } catch { /* keep last valid */ }
            }}
          />
        </label>
        <label className="col-span-2 flex flex-col gap-1">
          <span className="text-xs text-zinc-500">Faults (JSON object, keyed by protocol — optional)</span>
          <textarea
            rows={4}
            className="bg-zinc-800 border border-zinc-700 rounded px-2 py-1.5 text-sm text-zinc-200 font-mono resize-y"
            defaultValue={JSON.stringify(value.faults ?? {}, null, 2)}
            onBlur={(e) => {
              try { onChange({ ...value, faults: JSON.parse(e.target.value) }) } catch { /* keep last valid */ }
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
