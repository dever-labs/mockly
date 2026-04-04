interface Props {
  label: string
  badge?: string | number
  badgeColor?: string
}

export function ProtocolCard({ label, badge, badgeColor = 'bg-violet-600' }: Props) {
  return (
    <div className="rounded-lg border border-zinc-800 bg-zinc-900 p-4 flex items-center justify-between">
      <span className="text-sm text-zinc-300 font-medium">{label}</span>
      {badge !== undefined && (
        <span className={`text-xs font-semibold px-2 py-0.5 rounded-full text-white ${badgeColor}`}>
          {badge}
        </span>
      )}
    </div>
  )
}
