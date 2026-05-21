import type { ReactNode } from 'react'

export function PageHeader({
  title,
  description,
  action,
}: {
  title: string
  description: string
  action?: ReactNode
}) {
  return (
    <div className="flex flex-col gap-3 rounded-lg border border-slate-800 bg-slate-900/70 p-4 shadow-panel sm:flex-row sm:items-center sm:justify-between">
      <div>
        <h1 className="text-xl font-semibold text-slate-50">{title}</h1>
        <p className="mt-1 text-sm text-slate-400">{description}</p>
      </div>
      {action}
    </div>
  )
}
