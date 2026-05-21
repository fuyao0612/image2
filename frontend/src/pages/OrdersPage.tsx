import { useState, useEffect } from 'react';
import { api } from '@/api/client';
import type { UserActivityItem } from '@/types/api';
import { useAppContext } from '@/contexts/AppContext';
import { getActivityShortLabel, getActivityRecordLabel, formatDateTime, getActivityRecordValue, PanelPagination } from "../App";
import { ReceiptText } from "lucide-react";
import { PageHeader } from "../App";

const ACTIVITY_PAGE_SIZE = 10;

export function OrdersPage() {
  const { token, currentUser, setBusy, handleApiError } = useAppContext();
  const [activityLogs, setActivityLogs] = useState<UserActivityItem[]>([]);
  const [activityPage, setActivityPage] = useState(1);
  const [activityTotal, setActivityTotal] = useState(0);

  async function loadActivityPage(page: number, options: { showBusy?: boolean } = {}) {
    if (!token) return;
    const { showBusy = false } = options;
    try {
      if (showBusy) {
        setBusy('refresh');
      }
      const pageResult = await api.listUserActivity(page, ACTIVITY_PAGE_SIZE);
      setActivityLogs(pageResult.items || []);
      setActivityPage(pageResult.page);
      setActivityTotal(pageResult.total);
    } catch (error) {
      handleApiError(error, '日志加载失败');
    } finally {
      if (showBusy) {
        setBusy(null);
      }
    }
  }

  useEffect(() => {
    if (token && currentUser) {
      void loadActivityPage(1);
    }
  }, [token, currentUser]);

  return (
    <div className="space-y-4">
      <ActivityLogPanel
        items={activityLogs}
        page={activityPage}
        pageSize={ACTIVITY_PAGE_SIZE}
        total={activityTotal}
        onPageChange={(page: number) => void loadActivityPage(page)}
      />
    </div>
  );
}
export function ActivityLogPanel({
  items,
  page,
  pageSize,
  total,
  onPageChange,
}: {
  items: UserActivityItem[]
  page: number
  pageSize: number
  total: number
  onPageChange: (page: number) => void
}) {
  return (
    <section className="rounded-lg border border-slate-800 bg-slate-900/70 p-4 shadow-panel">
      <PageHeader
        title="活动记录"
        description="查看生图、兑换码、积分和福利次数流水合并记录。"
        action={<ReceiptText className="h-5 w-5 text-violet-300" />}
      />

      <div className="mt-4 space-y-3">
        {items.length ? (
          items.map((item) => (
            <div key={item.id} className="flex items-start gap-3 rounded-lg border border-slate-800 bg-slate-950 p-3">
              <div className="flex h-14 w-14 shrink-0 items-center justify-center overflow-hidden rounded-md border border-slate-800 bg-slate-900 text-xs font-medium text-slate-300">
                {item.imageUrl ? (
                  <img src={item.imageUrl} alt={item.title} className="h-full w-full object-cover" />
                ) : (
                  <span className="text-[11px]">{getActivityShortLabel(item.recordType)}</span>
                )}
              </div>
              <div className="min-w-0 flex-1">
                <div className="flex items-start justify-between gap-2">
                  <div className="min-w-0">
                    <div className="truncate text-sm font-medium text-slate-200">{item.title}</div>
                    <p className="mt-2 line-clamp-2 text-xs leading-5 text-slate-500">{item.description}</p>
                  </div>
                  <span className="shrink-0 rounded-md border border-slate-700 bg-slate-900 px-2 py-0.5 text-xs text-slate-300">
                    {getActivityRecordLabel(item.recordType)}
                  </span>
                </div>
                <div className="mt-2 flex flex-wrap items-center justify-between gap-2 text-xs text-slate-500">
                  <span>{formatDateTime(item.createdAt)}</span>
                  <span>{getActivityRecordValue(item)}</span>
                </div>
              </div>
            </div>
          ))
        ) : (
          <div className="rounded-lg border border-dashed border-slate-800 bg-slate-950 p-4 text-sm text-slate-500">
            暂无活动记录。
          </div>
        )}
      </div>

      <PanelPagination page={page} pageSize={pageSize} total={total} onPageChange={onPageChange} />
    </section>
  )
}
