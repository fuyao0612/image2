import type { ReactNode } from 'react'

export function Field({
  label,
  icon,
  children,
}: {
  label: string
  icon?: ReactNode
  children: ReactNode
}) {
  return (
    <label className="block">
      <span className="mb-2 flex items-center gap-2 text-sm text-slate-400">
        {icon}
        {label}
      </span>
      {children}
    </label>
  )
}
