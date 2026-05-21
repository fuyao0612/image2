import { useState, useEffect } from 'react';
import { api } from '@/api/client';
import type { PointsLogItem, CreditLogItem } from '@/types/api';
import { useAppContext } from '@/contexts/AppContext';
import { Wallet, Sparkles } from 'lucide-react';
import type { FlowLogType } from '@/types/app';
import { FlowLogTabButton, PointsLogRow, CreditLogRow, PanelPagination } from '@/components/shared';

const FLOW_LOG_PAGE_SIZE = 10;

export function FlowsPage() {
  const { token, currentUser, setBusy, handleApiError } = useAppContext();
  
  const [pointsLogItems, setPointsLogItems] = useState<PointsLogItem[]>([]);
  const [pointsLogPage, setPointsLogPage] = useState(1);
  const [pointsLogTotal, setPointsLogTotal] = useState(0);

  const [creditLogItems, setCreditLogItems] = useState<CreditLogItem[]>([]);
  const [creditLogPage, setCreditLogPage] = useState(1);
  const [creditLogTotal, setCreditLogTotal] = useState(0);

  async function loadPointsLogPage(page: number, options: { showBusy?: boolean } = {}) {
    if (!token) return;
    const { showBusy = false } = options;
    try {
      if (showBusy) setBusy('refresh');
      const pageResult = await api.listPointsLogs(page, FLOW_LOG_PAGE_SIZE);
      setPointsLogItems(pageResult.items || []);
      setPointsLogPage(pageResult.page);
      setPointsLogTotal(pageResult.total);
    } catch (error) {
      handleApiError(error, '积分流水加载失败');
    } finally {
      if (showBusy) setBusy(null);
    }
  }

  async function loadCreditLogPage(page: number, options: { showBusy?: boolean } = {}) {
    if (!token) return;
    const { showBusy = false } = options;
    try {
      if (showBusy) setBusy('refresh');
      const pageResult = await api.listCreditLogs(page, FLOW_LOG_PAGE_SIZE);
      setCreditLogItems(pageResult.items || []);
      setCreditLogPage(pageResult.page);
      setCreditLogTotal(pageResult.total);
    } catch (error) {
      handleApiError(error, '福利次数流水加载失败');
    } finally {
      if (showBusy) setBusy(null);
    }
  }

  useEffect(() => {
    if (token && currentUser) {
      void loadPointsLogPage(1);
      void loadCreditLogPage(1);
    }
  }, [token, currentUser]);

  return (
    <div className="space-y-4">
      <FlowLogsPanel
        pointsLogs={pointsLogItems}
        creditLogs={creditLogItems}
        pointsPage={pointsLogPage}
        creditPage={creditLogPage}
        pageSize={FLOW_LOG_PAGE_SIZE}
        pointsTotal={pointsLogTotal}
        creditTotal={creditLogTotal}
        onPointsPageChange={(page: number) => void loadPointsLogPage(page)}
        onCreditPageChange={(page: number) => void loadCreditLogPage(page)}
      />
    </div>
  );
}
export function FlowLogsPanel({
  pointsLogs,
  creditLogs,
  pointsPage,
  creditPage,
  pageSize,
  pointsTotal,
  creditTotal,
  onPointsPageChange,
  onCreditPageChange,
}: {
  pointsLogs: PointsLogItem[]
  creditLogs: CreditLogItem[]
  pointsPage: number
  creditPage: number
  pageSize: number
  pointsTotal: number
  creditTotal: number
  onPointsPageChange: (page: number) => void
  onCreditPageChange: (page: number) => void
}) {
  const [activeType, setActiveType] = useState<FlowLogType>('points')
  const showingPoints = activeType === 'points'
  const currentPage = showingPoints ? pointsPage : creditPage
  const currentTotal = showingPoints ? pointsTotal : creditTotal
  const onCurrentPageChange = showingPoints ? onPointsPageChange : onCreditPageChange

  return (
    <section className="rounded-lg border border-slate-800 bg-slate-900/70 p-4 shadow-panel">
      <div className="flex flex-wrap items-center justify-between gap-3">
        <div className="inline-flex rounded-md border border-slate-800 bg-slate-950 p-1">
          <FlowLogTabButton
            active={showingPoints}
            icon={<Wallet className="h-4 w-4" />}
            label="积分流水"
            onClick={() => setActiveType('points')}
          />
          <FlowLogTabButton
            active={!showingPoints}
            icon={<Sparkles className="h-4 w-4" />}
            label="福利次数流水"
            onClick={() => setActiveType('credits')}
          />
        </div>
        <div className="text-xs text-slate-500">
          共 {currentTotal} 条，当前第 {currentPage} 页
        </div>
      </div>

      <div className="mt-4 space-y-3">
        {showingPoints ? (
          pointsLogs.length ? (
            pointsLogs.map((item) => <PointsLogRow key={item.id} item={item} />)
          ) : (
            <div className="rounded-lg border border-dashed border-slate-800 bg-slate-950 p-4 text-sm text-slate-500">
              暂无积分流水。
            </div>
          )
        ) : creditLogs.length ? (
          creditLogs.map((item) => <CreditLogRow key={item.id} item={item} />)
        ) : (
          <div className="rounded-lg border border-dashed border-slate-800 bg-slate-950 p-4 text-sm text-slate-500">
            暂无福利次数流水。
          </div>
        )}
      </div>

      <PanelPagination page={currentPage} pageSize={pageSize} total={currentTotal} onPageChange={onCurrentPageChange} />
    </section>
  )
}
