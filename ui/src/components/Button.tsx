interface Props extends React.ButtonHTMLAttributes<HTMLButtonElement> {
  variant?: 'primary' | 'danger' | 'ghost'
  size?: 'sm' | 'md'
}

export function Button({ variant = 'primary', size = 'md', className = '', ...props }: Props) {
  const base = 'inline-flex items-center gap-1.5 font-medium rounded-md transition-colors focus:outline-none focus:ring-2 focus:ring-violet-500 disabled:opacity-40 cursor-pointer'
  const variants = {
    primary: 'bg-violet-600 hover:bg-violet-500 text-white',
    danger: 'bg-red-700 hover:bg-red-600 text-white',
    ghost: 'bg-transparent hover:bg-zinc-800 text-zinc-400 hover:text-zinc-100',
  }
  const sizes = {
    sm: 'px-2.5 py-1 text-xs',
    md: 'px-4 py-1.5 text-sm',
  }
  return (
    <button
      className={`${base} ${variants[variant]} ${sizes[size]} ${className}`}
      {...props}
    />
  )
}
