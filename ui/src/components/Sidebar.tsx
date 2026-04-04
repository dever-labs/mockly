import { NavLink } from 'react-router-dom'

const links = [
  { to: '/', label: 'Dashboard', icon: '⊞' },
  { to: '/http', label: 'HTTP', icon: '⬡' },
  { to: '/websocket', label: 'WebSocket', icon: '⟳' },
  { to: '/grpc', label: 'gRPC', icon: '◈' },
  { to: '/logs', label: 'Logs', icon: '≡' },
  { to: '/state', label: 'State', icon: '◎' },
]

export function Sidebar() {
  return (
    <aside className="w-56 shrink-0 bg-zinc-900 text-zinc-100 flex flex-col min-h-screen border-r border-zinc-800">
      <div className="px-5 py-4 border-b border-zinc-800">
        <span className="font-bold text-lg tracking-tight text-violet-400">mockly</span>
        <span className="ml-2 text-xs text-zinc-500">v0.1</span>
      </div>
      <nav className="flex-1 py-3">
        {links.map(({ to, label, icon }) => (
          <NavLink
            key={to}
            to={to}
            end={to === '/'}
            className={({ isActive }) =>
              `flex items-center gap-3 px-5 py-2 text-sm transition-colors ${
                isActive
                  ? 'bg-violet-700/20 text-violet-300 border-r-2 border-violet-500'
                  : 'text-zinc-400 hover:text-zinc-100 hover:bg-zinc-800'
              }`
            }
          >
            <span className="text-base">{icon}</span>
            {label}
          </NavLink>
        ))}
      </nav>
      <div className="px-5 py-3 border-t border-zinc-800 text-xs text-zinc-600">
        API on :9091
      </div>
    </aside>
  )
}
