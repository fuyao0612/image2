import type { ReactNode } from 'react'
import type { PointsLogItem, CreditLogItem } from '@/types/api'
import {
  formatDateTime,
  getPointsChangeTypeLabel,
  getCreditChangeTypeLabel,
  getCreditLogTotals,
  formatSignedNumber,
} from '@/utils/helpers'

export function FlowLogTabButton({
  active,
  icon,
  label,
  onClick,
}: {
  active: boolean
  icon: ReactNode
  label: string
  onClick: () => void
}) {
  return (
    <button
      type="button"
      onClick={onClick}
      className={`inline-flex h-9 items-center gap-2 rounded-md px-3 text-sm transition ${
        active ? 'bg-violet-500 text-slate-950' : 'text-slate-400 hover:bg-slate-900 hover:text-slate-100'
      }`}
    >
      {icon}
      {label}
    </button>
  )
}

export function PointsLogRow({ item }: { item: PointsLogItem }) {
  const isPositive = item.delta >= 0
  const title = item.remark || getPointsChangeTypeLabel(item.changeType)
  return (
    <div className="rounded-lg border border-slate-800 bg-slate-950 p-3">
      <div className="flex items-center justify-between gap-2">
        <span className="truncate text-sm text-slate-200">{title}</span>
        <span className={isPositive ? 'text-emerald-300' : 'text-rose-300'}>
          {isPositive ? '+' : ''}
          {item.delta}
        </span>
      </div>
      <div className="mt-2 flex items-center justify-between text-xs text-slate-500">
        <span>{formatDateTime(item.createdAt)}</span>
        <span>
          {item.beforeBalance} → {item.afterBalance}
        </span>
      </div>
    </div>
  )
}

export function CreditLogRow({ item }: { item: CreditLogItem }) {
  const { beforeTotal, afterTotal, deltaTotal } = getCreditLogTotals(item)
  const title = item.remark || getCreditChangeTypeLabel(item.changeType)
  return (
    <div className="rounded-lg border border-slate-800 bg-slate-950 p-3">
      <div className="flex items-center justify-between gap-2">
        <span className="truncate text-sm text-slate-200">{title}</span>
        <span className="text-xs text-slate-500">{getCreditChangeTypeLabel(item.changeType)}</span>
      </div>
      <div className="mt-2 flex flex-wrap items-center justify-between gap-2 text-xs text-slate-500">
        <span>福利次数 {beforeTotal} → {afterTotal}</span>
        <span>变动 {formatSignedNumber(deltaTotal)}</span>
      </div>
      <div className="mt-2 flex items-center justify-between text-xs text-slate-500">
        <span>{formatDateTime(item.createdAt)}</span>
        <span>总福利次数 {afterTotal}</span>
      </div>
    </div>
  )
}
