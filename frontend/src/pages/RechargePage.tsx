import { useState, useEffect, ReactNode } from 'react';
import { KeyRound } from 'lucide-react';
import { api } from '@/api/client';
import type { PackageItem } from '@/types/api';
import { useAppContext } from '@/contexts/AppContext';
import { LoaderCircle } from 'lucide-react';
import type { BusyAction } from '@/types/app';

export function RechargePage() {
  const { token, currentUser, setBusy, handleApiError, setPoints, busy, setMessage, points } = useAppContext();
  
  const [packages, setPackages] = useState<PackageItem[]>([]);
  const [redeemCodeInput, setRedeemCodeInput] = useState('');
  const [activeRechargePackageCode, setActiveRechargePackageCode] = useState<string | null>(null);
  const [rechargeConfirmDialog, setRechargeConfirmDialog] = useState<{
    open: boolean;
    title: string;
    lines: string[];
    confirmLabel: string;
    href?: string;
  }>({ open: false, title: '', lines: [], confirmLabel: '' });

  async function loadPackages() {
    try {
      const res = await api.listPackages();
      setPackages(res.items || []);
    } catch (e) {
      console.error('Failed to load packages', e);
    }
  }

  useEffect(() => {
    if (token) {
      void loadPackages();
    }
  }, [token]);

  async function handlePurchase(packageCode: string) {
    if (!token) {
      setMessage('请先登录后再购买兑换码商品。');
      return;
    }
    const selectedPackage = packages.find((item) => item.code === packageCode);
    if (!selectedPackage) {
      setMessage('兑换码商品不存在或已停用。');
      return;
    }
    if (!selectedPackage.productUrl) {
      setMessage('该兑换码商品暂未配置购买链接，请联系管理员处理。');
      return;
    }
    setActiveRechargePackageCode(packageCode);

    setRechargeConfirmDialog({
      open: true,
      title: `确认购买 ${selectedPackage.name}`,
      lines: [
        `到账积分：${selectedPackage.bonusPoints} 积分`,
        `赠送福利次数：${selectedPackage.creditsAmount} 福利次数`,
        `商品价格：${(selectedPackage.amountCents / 100).toFixed(2)} 元`,
        '确认后将跳转至兑换码购买平台。购买完成后，请按平台提示获取兑换码，并返回本站完成兑换。',
      ],
      confirmLabel: '确定',
      href: selectedPackage.productUrl,
    });
  }

  async function confirmPackageRecharge(packageCode: string) {
    const selectedPackage = packages.find((item) => item.code === packageCode);
    if (!selectedPackage?.productUrl) {
      setRechargeConfirmDialog({ open: false, title: '', lines: [], confirmLabel: '' });
      setMessage('该兑换码商品暂未配置购买链接，请联系管理员处理。');
      return;
    }

    window.open(selectedPackage.productUrl, '_blank', 'noopener,noreferrer');
    setRechargeConfirmDialog({ open: false, title: '', lines: [], confirmLabel: '' });
    setMessage('已打开兑换码购买平台。购买完成后，请获取兑换码并返回本站兑换。');
  }

  function handleCancelRechargeConfirm() {
    setRechargeConfirmDialog({ open: false, title: '', lines: [], confirmLabel: '' });
  }

  async function handleConfirmRechargeDialog() {
    if (rechargeConfirmDialog.href) {
      window.open(rechargeConfirmDialog.href, '_blank', 'noopener,noreferrer');
      setRechargeConfirmDialog({ open: false, title: '', lines: [], confirmLabel: '' });
      setMessage(rechargeConfirmDialog.title === '确认前往购买' ? '已打开商品购买页面。' : '已打开兑换码购买平台。购买完成后，请获取兑换码并返回本站兑换。');
      return;
    }
    if (!activeRechargePackageCode || !token) return;
    void confirmPackageRecharge(activeRechargePackageCode);
  }

  async function handleRedeemCode() {
    if (!redeemCodeInput.trim() || !token) return;
    try {
      setBusy('redeem-code');
      const res = await api.redeemCode({ code: redeemCodeInput.trim() });
      setPoints(points ? {
        ...points,
        balance: res.pointsBalance,
        freeCredits: res.freeCreditsBalance,
        paidCredits: res.paidCreditsBalance,
        creditsBalance: res.creditsBalance,
        totalCredits: res.totalCreditsBalance,
      } : null);
      setRedeemCodeInput('');
      setMessage('兑换成功！');
    } catch (error) {
      handleApiError(error, '兑换失败');
    } finally {
      setBusy(null);
    }
  }

  return (
    <div className="space-y-4">
      <div className="grid gap-4">
        <section className="space-y-4 rounded-lg border border-slate-800 bg-slate-900/70 p-4 shadow-panel">
          <PackageSection
            title="兑换码商品"
            description="购买兑换码商品后，返回本页输入兑换码即可兑换积分或福利次数。"
            icon={<KeyRound className="h-4 w-4 text-violet-300" />}
            items={packages}
            busy={busy}
            emptyText="暂无可用兑换码商品。"
            actionLabel={(item) => (item.productUrl ? '前往购买' : '待配置')}
            busyPrefix="purchase:"
            onAction={handlePurchase}
            renderMeta={(item) => (
              <>
                <span>{item.creditsAmount} 福利次数</span>
                <span>{item.bonusPoints} 积分</span>
                <span>{(item.amountCents / 100).toFixed(2)} 元</span>
                {item.creditsAmount > 0 ? <span>赠送 {item.creditsAmount} 福利次数</span> : null}
                {item.productUrl ? <span>兑换码商品</span> : null}
              </>
            )}
          />
          <RedeemCodePanel
            code={redeemCodeInput}
            busy={busy === 'redeem-code'}
            onCodeChange={setRedeemCodeInput}
            onSubmit={handleRedeemCode}
          />
        </section>
      </div>
      {rechargeConfirmDialog.open ? (
        <RechargeConfirmDialog
          title={rechargeConfirmDialog.title}
          lines={rechargeConfirmDialog.lines}
          confirmLabel={rechargeConfirmDialog.confirmLabel}
          busy={busy === 'purchase:custom' || busy === `purchase:${activeRechargePackageCode}`}
          onCancel={handleCancelRechargeConfirm}
          onConfirm={handleConfirmRechargeDialog}
        />
      ) : null}
    </div>
  );
}
export function PackageSection({
  title,
  description,
  icon,
  items,
  busy,
  emptyText,
  actionLabel,
  busyPrefix,
  onAction,
  renderMeta,
}: {
  title: string
  description: string
  icon: ReactNode
  items: PackageItem[]
  busy: BusyAction
  emptyText: string
  actionLabel: string | ((item: PackageItem) => string)
  busyPrefix: 'purchase:'
  onAction: (packageCode: string) => void
  renderMeta: (item: PackageItem) => ReactNode
}) {
  return (
    <div className="rounded-lg border border-slate-800 bg-slate-950 p-4">
      <div className="flex items-center justify-between gap-2">
        <div>
          <h3 className="text-sm font-medium text-slate-100">{title}</h3>
          <p className="mt-1 text-xs text-slate-500">{description}</p>
        </div>
        {icon}
      </div>

      <div className="mt-3 space-y-2">
        {items.length ? (
          items.map((item) => {
            const label = typeof actionLabel === 'function' ? actionLabel(item) : actionLabel
            return (
              <button
                key={item.code}
                type="button"
                onClick={() => onAction(item.code)}
                disabled={busy === `${busyPrefix}${item.code}` || item.enabled === false}
                className="flex w-full items-start justify-between gap-3 rounded-lg border border-slate-800 bg-slate-900 p-3 text-left transition hover:border-violet-400/50 hover:bg-slate-800 disabled:cursor-not-allowed disabled:opacity-60"
              >
                <div className="min-w-0 flex-1">
                  <div className="flex items-center gap-2">
                    <span className="text-sm font-medium text-slate-100">{item.name}</span>
                    {item.enabled === false ? (
                      <span className="rounded-md bg-slate-800 px-2 py-0.5 text-xs text-slate-500">停用</span>
                    ) : null}
                  </div>
                  <div className="mt-2 flex flex-wrap gap-2 text-xs text-slate-500">{renderMeta(item)}</div>
                </div>
                <span className="shrink-0 rounded-md bg-violet-500 px-3 py-1.5 text-sm font-medium text-slate-950">
                  {busy === `${busyPrefix}${item.code}` ? '处理中' : label}
                </span>
              </button>
            )
          })
        ) : (
          <div className="rounded-md border border-dashed border-slate-800 bg-slate-900 p-4 text-sm text-slate-500">
            {emptyText}
          </div>
        )}
      </div>
    </div>
  )
}
export function RedeemCodePanel({
  code,
  busy,
  onCodeChange,
  onSubmit,
}: {
  code: string
  busy: boolean
  onCodeChange: (value: string) => void
  onSubmit: () => void
}) {
  return (
    <div className="rounded-lg border border-slate-800 bg-slate-950 p-4">
      <div className="flex items-center justify-between gap-2">
        <div>
          <h3 className="text-sm font-medium text-slate-100">兑换码</h3>
          <p className="mt-1 text-xs text-slate-500">输入已获得的兑换码，兑换积分或福利次数。</p>
        </div>
        <KeyRound className="h-4 w-4 text-emerald-300" />
      </div>

      <div className="mt-3 grid gap-3 sm:grid-cols-[minmax(0,1fr)_auto] sm:items-end">
        <label className="block">
          <span className="mb-2 block text-xs text-slate-500">兑换码</span>
          <input
            value={code}
            onChange={(event) => onCodeChange(event.target.value.toUpperCase())}
            onKeyDown={(event) => {
              if (event.key === 'Enter') {
                onSubmit()
              }
            }}
            className="h-10 w-full rounded-md border border-slate-700 bg-slate-900 px-3 text-sm uppercase tracking-wider text-slate-100 outline-none transition focus:border-emerald-400"
            placeholder="输入兑换码"
            autoComplete="off"
          />
        </label>
        <button
          type="button"
          onClick={onSubmit}
          disabled={busy}
          className="inline-flex h-10 items-center justify-center gap-2 rounded-md bg-emerald-400 px-4 text-sm font-medium text-slate-950 transition hover:bg-emerald-300 disabled:cursor-not-allowed disabled:opacity-60"
        >
          {busy ? <LoaderCircle className="h-4 w-4 animate-spin" /> : <KeyRound className="h-4 w-4" />}
          兑换
        </button>
      </div>
    </div>
  )
}
export function RechargeConfirmDialog({
  title,
  lines,
  confirmLabel,
  busy,
  onCancel,
  onConfirm,
}: {
  title: string
  lines: string[]
  confirmLabel: string
  busy: boolean
  onCancel: () => void
  onConfirm: () => void
}) {
  return (
    <div className="fixed inset-0 z-50 flex items-center justify-center bg-slate-950/80 px-4 py-6 backdrop-blur-sm" role="dialog" aria-modal="true">
      <button type="button" className="absolute inset-0 cursor-default" aria-label="关闭兑换码购买确认" onClick={onCancel} />
      <div className="relative w-full max-w-sm rounded-lg border border-slate-700 bg-slate-950 shadow-2xl">
        <div className="px-4 py-4 text-center">
          <h2 className="text-base font-medium text-slate-50">{title}</h2>
          <div className="mt-3 space-y-1 text-sm leading-6 text-slate-300">
            {lines.map((line) => (
              <div key={line}>{line}</div>
            ))}
          </div>
        </div>
        <div className="grid grid-cols-2 border-t border-slate-800">
          <button
            type="button"
            onClick={onCancel}
            className="h-12 border-r border-slate-800 text-sm font-medium text-slate-300 transition hover:bg-slate-900 hover:text-white"
          >
            取消
          </button>
          <button
            type="button"
            onClick={onConfirm}
            disabled={busy}
            className="inline-flex h-12 items-center justify-center gap-2 text-sm font-medium text-violet-300 transition hover:bg-slate-900 hover:text-violet-200 disabled:cursor-not-allowed disabled:opacity-60"
          >
            {busy ? <LoaderCircle className="h-4 w-4 animate-spin" /> : null}
            {confirmLabel}
          </button>
        </div>
      </div>
    </div>
  )
}
