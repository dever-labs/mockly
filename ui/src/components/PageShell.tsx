interface Props {
  children: React.ReactNode
  title?: string
  actions?: React.ReactNode
}

export function PageShell({ children, title, actions }: Props) {
  return (
    <div className="flex-1 flex flex-col overflow-hidden">
      {(title || actions) && (
        <header className="flex items-center justify-between px-6 py-4 border-b border-zinc-800 bg-zinc-950">
          {title && <h1 className="text-lg font-semibold text-zinc-100">{title}</h1>}
          {actions && <div className="flex gap-2">{actions}</div>}
        </header>
      )}
      <main className="flex-1 overflow-auto p-6 bg-zinc-950 text-zinc-300">{children}</main>
    </div>
  )
}
