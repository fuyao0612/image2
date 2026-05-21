import React from 'react';
import { ChevronLeft, ChevronRight, MoreVertical } from 'lucide-react';

export function PanelPagination({
  page,
  pageSize,
  total,
  onPageChange,
}: {
  page: number
  pageSize: number
  total: number
  onPageChange: (page: number) => void
}) {
  const totalPages = Math.max(1, Math.ceil(total / pageSize))
  const hasPrev = page > 1
  const hasNext = page < totalPages

  return (
    <div className="mt-4 flex items-center justify-between gap-3 border-t border-slate-800 pt-3 text-xs text-slate-500">
      <span>
        第 {page} / {totalPages} 页 · 共 {total} 条
      </span>
      <div className="flex items-center gap-2">
        <button
          type="button"
          onClick={() => onPageChange(Math.max(1, page - 1))}
          disabled={!hasPrev}
          className="rounded-md border border-slate-700 bg-slate-950 px-3 py-1.5 text-slate-300 transition hover:border-violet-400 hover:text-white disabled:cursor-not-allowed disabled:opacity-40"
        >
          上一页
        </button>
        <button
          type="button"
          onClick={() => onPageChange(Math.min(totalPages, page + 1))}
          disabled={!hasNext}
          className="rounded-md border border-slate-700 bg-slate-950 px-3 py-1.5 text-slate-300 transition hover:border-violet-400 hover:text-white disabled:cursor-not-allowed disabled:opacity-40"
        >
          下一页
        </button>
      </div>
    </div>
  )
}