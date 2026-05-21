import type { ReactNode } from 'react'

export function MiniStat({ label, value }: { label: string; value: ReactNode }) {
  return (
    <div className="rounded-lg border border-slate-800 bg-slate-950 p-3">
      <div className="text-xs text-slate-500">{label}</div>
      <div className="mt-2 text-lg font-semibold text-slate-100">{value}</div>
    </div>
  )
}

export function RewardLines({ points, credits }: { points: number; credits: number }) {
  return (
    <div className="space-y-1 leading-tight">
      <div>{Math.max(0, Number(points) || 0)}积分</div>
      <div>{Math.max(0, Number(credits) || 0)}福利次数</div>
    </div>
  )
}
