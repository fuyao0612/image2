import { useState, useEffect } from 'react';
import { api } from '@/api/client';
import type { InviteInfoResponse, PointsResponse } from '@/types/api';
import type { PasswordChangeForm } from '@/types/app';
import { useAppContext } from '@/contexts/AppContext';
import { UserRound, LoaderCircle, BadgeCheck, Gift, Copy, ShieldCheck, KeyRound, EyeOff, Eye } from 'lucide-react';
import { getUserRoleLabel, buildInviteLink, formatInviteReward } from '@/utils/helpers';
import { MiniStat, RewardLines, Field } from '@/components/shared';
import type { UserInfo, BillingSettingsResponse } from '@/types/api';
import type { BusyAction } from '@/types/app';

function isSameShanghaiDay(dateString: string | null) {
  if (!dateString) return false;
  const target = new Date(dateString);
  const now = new Date();
  const targetSh = new Date(target.getTime() + 8 * 60 * 60 * 1000);
  const nowSh = new Date(now.getTime() + 8 * 60 * 60 * 1000);
  return (
    targetSh.getUTCFullYear() === nowSh.getUTCFullYear() &&
    targetSh.getUTCMonth() === nowSh.getUTCMonth() &&
    targetSh.getUTCDate() === nowSh.getUTCDate()
  );
}

export function ProfilePage() {
  const { token, currentUser, points, setPoints, billingSettings, busy, setBusy, message, setMessage, handleApiError, logout } = useAppContext();
  
  const [inviteInfo, setInviteInfo] = useState<InviteInfoResponse | null>(null);
  const [showChangePassword, setShowChangePassword] = useState(false);
  const [changePasswordForm, setChangePasswordForm] = useState<PasswordChangeForm>({
    currentPassword: '',
    newPassword: '',
    confirmNewPassword: '',
  });

  const checkedInToday = isSameShanghaiDay(points?.lastCheckinAt ?? null);

  useEffect(() => {
    if (token && currentUser) {
      void api.getInviteInfo().then(setInviteInfo).catch(() => null);
    }
  }, [token, currentUser]);

  async function handleCheckin() {
    if (!token || !currentUser || busy === 'checkin' || checkedInToday) return;
    try {
      setBusy('checkin');
      const res = await api.checkin();
      setPoints(points ? {
        ...points,
        balance: res.pointsBalance,
        streak: res.streak,
        freeCredits: res.freeCredits,
        paidCredits: res.paidCredits,
        creditsBalance: res.creditsBalance,
        totalCredits: res.totalCredits,
        lastCheckinAt: res.lastCheckinAt
      } : null);
      setMessage('签到成功！');
    } catch (error) {
      handleApiError(error, '签到失败');
    } finally {
      setBusy(null);
    }
  }

  async function handleCopyInviteLink(link: string) {
    if (busy === 'copy-invite-link') return;
    try {
      setBusy('copy-invite-link');
      if (navigator.clipboard && window.isSecureContext) {
        await navigator.clipboard.writeText(link);
      } else {
        const textArea = document.createElement('textarea');
        textArea.value = link;
        textArea.style.position = 'fixed';
        textArea.style.left = '-999999px';
        textArea.style.top = '-999999px';
        document.body.appendChild(textArea);
        textArea.focus();
        textArea.select();
        try {
          document.execCommand('copy');
        } finally {
          textArea.remove();
        }
      }
      setMessage('邀请链接已复制到剪贴板');
    } catch (e) {
      console.error(e);
      setMessage('复制失败，请手动选择复制');
    } finally {
      setBusy(null);
    }
  }

  async function handleChangePassword() {
    if (
      !token ||
      busy === 'change-password' ||
      !changePasswordForm.currentPassword.trim() ||
      !changePasswordForm.newPassword.trim() ||
      !changePasswordForm.confirmNewPassword.trim()
    ) {
      return;
    }
    if (changePasswordForm.newPassword !== changePasswordForm.confirmNewPassword) {
      setMessage('两次输入的新密码不一致');
      return;
    }
    try {
      setBusy('change-password');
      await api.changePassword({
        currentPassword: changePasswordForm.currentPassword,
        newPassword: changePasswordForm.newPassword,
        confirmNewPassword: changePasswordForm.confirmNewPassword,
      });
      setMessage('密码修改成功，请重新登录');
      logout();
    } catch (error) {
      handleApiError(error, '密码修改失败');
    } finally {
      setBusy(null);
    }
  }

  return (
    <div className="space-y-4">
      <div className="grid gap-4 xl:grid-cols-2">
        <UserSummaryPanel
          currentUser={currentUser}
          points={points}
          billingSettings={billingSettings}
          busy={busy}
          checkedInToday={checkedInToday}
          onCheckin={handleCheckin}
        />
        <InvitePanel
          info={inviteInfo}
          busy={busy}
          onCopyLink={handleCopyInviteLink}
        />
        <ChangePasswordPanel
          form={changePasswordForm}
          showPassword={showChangePassword}
          busy={busy}
          onToggleShowPassword={() => setShowChangePassword((current) => !current)}
          onChangeForm={setChangePasswordForm}
          onSubmit={handleChangePassword}
        />
      </div>
    </div>
  );
}
export function UserSummaryPanel({
  currentUser,
  points,
  billingSettings,
  busy,
  checkedInToday,
  onCheckin,
}: {
  currentUser: UserInfo | null
  points: PointsResponse | null
  billingSettings: BillingSettingsResponse | null
  busy: BusyAction
  checkedInToday: boolean
  onCheckin: () => void
}) {
  return (
    <section className="rounded-lg border border-slate-800 bg-slate-900/70 p-4 shadow-panel">
      <div className="flex items-center justify-end">
        <UserRound className="h-5 w-5 text-violet-300" />
      </div>

      <div className="mt-4 space-y-4">
        <div className="rounded-lg border border-slate-800 bg-slate-950 p-3">
          <div className="flex items-start justify-between gap-3">
            <div className="min-w-0">
              <div className="truncate text-sm font-medium text-slate-100">{currentUser?.username ?? '已登录用户'}</div>
              <div className="mt-1 truncate text-xs text-slate-500">{currentUser?.email ?? '-'}</div>
            </div>
            <div className="flex shrink-0 flex-wrap justify-end gap-1.5">
              {currentUser?.id ? (
                <span className="rounded-md border border-violet-500/30 bg-violet-500/10 px-2 py-1 text-xs text-violet-200">
                  #{currentUser.id}
                </span>
              ) : null}
              <span className="rounded-md border border-violet-500/30 bg-violet-500/10 px-2 py-1 text-xs text-violet-200">
                {getUserRoleLabel(currentUser?.role ?? 'user')}
              </span>
            </div>
          </div>
        </div>

        <div className="grid grid-cols-2 gap-3">
          <MiniStat label="当前积分" value={points ? String(points.balance) : '0'} />
          <MiniStat label="可用福利次数" value={points ? String(points.totalCredits) : '0'} />
        </div>

        <div className="grid grid-cols-2 gap-3">
          <MiniStat
            label="今日签到奖励"
            value={<RewardLines points={billingSettings?.checkinPoints ?? 0} credits={billingSettings?.checkinCredits ?? 0} />}
          />
          <MiniStat
            label="累计签到奖励"
            value={<RewardLines points={points?.checkinPointsTotal ?? 0} credits={points?.checkinCreditsTotal ?? 0} />}
          />
        </div>

        <button
          type="button"
          onClick={onCheckin}
          disabled={busy === 'checkin' || checkedInToday}
          className="inline-flex w-full items-center justify-center gap-2 rounded-md border border-slate-700 bg-slate-950 px-4 py-3 font-medium text-slate-100 transition hover:border-violet-400 hover:text-white disabled:cursor-not-allowed disabled:opacity-50"
        >
          {busy === 'checkin' ? <LoaderCircle className="h-4 w-4 animate-spin" /> : <BadgeCheck className="h-4 w-4" />}
          {busy === 'checkin' ? '签到中' : checkedInToday ? '今日已签到' : '每日签到'}
        </button>
      </div>
    </section>
  )
}
export function InvitePanel({
  info,
  busy,
  onCopyLink,
}: {
  info: InviteInfoResponse | null
  busy: BusyAction
  onCopyLink: (link: string) => void
}) {
  const inviteCode = info?.inviteCode ?? ''
  const inviteLink = buildInviteLink(inviteCode)
  const rewardText = formatInviteReward(info?.inviteRewardCredits ?? 0, info?.inviteRewardPoints ?? 0)

  return (
    <section className="rounded-lg border border-slate-800 bg-slate-900/70 p-4 shadow-panel">
      <div className="flex flex-wrap items-start justify-between gap-3">
        <div>
          <h2 className="text-lg font-medium text-slate-50">好友邀请</h2>
          <p className="mt-1 text-sm text-slate-400">
            邀请好友注册后，对方首次使用 ID+用户ID 开头的兑换码可为你带来 {rewardText}，新用户注册赠送 {info?.registrationGiftCredits ?? 0} 福利次数。
          </p>
        </div>
        <div className="flex h-10 w-10 items-center justify-center rounded-md border border-violet-500/20 bg-violet-500/10 text-violet-300">
          <Gift className="h-5 w-5" />
        </div>
      </div>

      <div className="mt-4 grid gap-3 md:grid-cols-3">
        <MiniStat label="已邀请人数" value={String(info?.invitedCount ?? 0)} />
        <MiniStat label="累计奖励福利次数" value={String(info?.rewardCreditsTotal ?? 0)} />
        <MiniStat label="累计奖励积分" value={String(info?.rewardPointsTotal ?? 0)} />
      </div>

      <div className="mt-4 grid gap-3 lg:grid-cols-[180px_minmax(0,1fr)_auto]">
        <div className="rounded-md border border-slate-800 bg-slate-950 px-3 py-2">
          <div className="text-xs text-slate-500">我的邀请码</div>
          <div className="mt-1 font-mono text-lg font-semibold tracking-[0.2em] text-violet-200">{inviteCode || '-'}</div>
        </div>
        <div className="min-w-0 rounded-md border border-slate-800 bg-slate-950 px-3 py-2">
          <div className="text-xs text-slate-500">邀请链接</div>
          <div className="mt-1 truncate text-sm text-slate-200">{inviteLink || '-'}</div>
        </div>
        <button
          type="button"
          onClick={() => onCopyLink(inviteLink)}
          disabled={!inviteLink || busy === 'copy-invite-link'}
          className="inline-flex items-center justify-center gap-2 rounded-md border border-slate-700 bg-slate-950 px-4 py-2 text-sm text-slate-100 transition hover:border-violet-400 hover:text-white disabled:cursor-not-allowed disabled:opacity-50"
        >
          {busy === 'copy-invite-link' ? <LoaderCircle className="h-4 w-4 animate-spin" /> : <Copy className="h-4 w-4" />}
          复制链接
        </button>
      </div>
    </section>
  )
}
export function ChangePasswordPanel({
  form,
  showPassword,
  busy,
  onToggleShowPassword,
  onChangeForm,
  onSubmit,
}: {
  form: PasswordChangeForm
  showPassword: boolean
  busy: BusyAction
  onToggleShowPassword: () => void
  onChangeForm: (next: PasswordChangeForm) => void
  onSubmit: () => void | Promise<void>
}) {
  const inputType = showPassword ? 'text' : 'password'
  const isBusy = busy === 'change-password'

  return (
    <section className="rounded-lg border border-slate-800 bg-slate-900/70 p-4 shadow-panel">
      <div className="flex items-start justify-between gap-3">
        <div>
          <h2 className="text-lg font-medium text-slate-50">修改密码</h2>
          <p className="mt-1 text-sm text-slate-400">先验证当前密码，再设置新的登录密码。</p>
        </div>
        <div className="flex h-10 w-10 items-center justify-center rounded-md border border-violet-500/20 bg-violet-500/10 text-violet-300">
          <ShieldCheck className="h-5 w-5" />
        </div>
      </div>

      <form
        className="mt-4 space-y-3"
        onSubmit={(event) => {
          event.preventDefault()
          onSubmit()
        }}
      >
        <Field label="当前密码" icon={<KeyRound className="h-4 w-4" />}>
          <div className="relative">
            <input
              type={inputType}
              value={form.currentPassword}
              onChange={(event) => onChangeForm({ ...form, currentPassword: event.target.value })}
              className="w-full rounded-md border border-slate-700 bg-slate-950 py-2 pl-3 pr-10 text-slate-100 outline-none transition placeholder:text-slate-500 focus:border-violet-400"
              placeholder="请输入当前密码"
              autoComplete="current-password"
            />
          </div>
        </Field>

        <Field label="新密码" icon={<KeyRound className="h-4 w-4" />}>
          <div className="relative">
            <input
              type={inputType}
              value={form.newPassword}
              onChange={(event) => onChangeForm({ ...form, newPassword: event.target.value })}
              className="w-full rounded-md border border-slate-700 bg-slate-950 py-2 pl-3 pr-10 text-slate-100 outline-none transition placeholder:text-slate-500 focus:border-violet-400"
              placeholder="请输入新密码"
              autoComplete="new-password"
            />
            <button
              type="button"
              onClick={onToggleShowPassword}
              aria-label={showPassword ? '隐藏密码' : '显示密码'}
              className="absolute inset-y-0 right-0 flex w-10 items-center justify-center text-slate-400 transition hover:text-slate-200"
            >
              {showPassword ? <EyeOff className="h-4 w-4" /> : <Eye className="h-4 w-4" />}
            </button>
          </div>
        </Field>

        <Field label="确认新密码" icon={<KeyRound className="h-4 w-4" />}>
          <div className="relative">
            <input
              type={inputType}
              value={form.confirmNewPassword}
              onChange={(event) => onChangeForm({ ...form, confirmNewPassword: event.target.value })}
              className="w-full rounded-md border border-slate-700 bg-slate-950 py-2 pl-3 pr-10 text-slate-100 outline-none transition placeholder:text-slate-500 focus:border-violet-400"
              placeholder="请再次输入新密码"
              autoComplete="new-password"
            />
            <button
              type="button"
              onClick={onToggleShowPassword}
              aria-label={showPassword ? '隐藏密码' : '显示密码'}
              className="absolute inset-y-0 right-0 flex w-10 items-center justify-center text-slate-400 transition hover:text-slate-200"
            >
              {showPassword ? <EyeOff className="h-4 w-4" /> : <Eye className="h-4 w-4" />}
            </button>
          </div>
        </Field>

        <button
          type="submit"
          disabled={isBusy}
          className="inline-flex w-full items-center justify-center gap-2 rounded-md bg-violet-500 px-4 py-3 font-medium text-slate-950 transition hover:bg-violet-400 disabled:cursor-not-allowed disabled:opacity-70"
        >
          {isBusy ? <LoaderCircle className="h-4 w-4 animate-spin" /> : <ShieldCheck className="h-4 w-4" />}
          确认修改
        </button>
      </form>
    </section>
  )
}
