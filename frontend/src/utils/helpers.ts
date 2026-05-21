import { FREE_REFERENCE_IMAGE_COUNT, MAX_REFERENCE_IMAGES } from '../constants/app'
import type { ReferenceImageItem } from '../types/app'
import type { CreditLogItem, UserActivityItem } from '@/types/api'

// ── Formatting ──────────────────────────────────────────────────────

export function formatDateTime(value: string): string {
  if (!value) {
    return '-'
  }

  try {
    return new Intl.DateTimeFormat('zh-CN', {
      year: '2-digit',
      month: '2-digit',
      day: '2-digit',
      hour: '2-digit',
      minute: '2-digit',
      hour12: false,
    }).format(new Date(value))
  } catch {
    return value
  }
}

export function formatDateOnly(value: string): string {
  if (!value) {
    return '-'
  }

  try {
    return new Intl.DateTimeFormat('zh-CN', {
      year: '2-digit',
      month: '2-digit',
      day: '2-digit',
    }).format(new Date(value))
  } catch {
    return value
  }
}

export function formatSignedNumber(value: number): string {
  const safeValue = Number.isFinite(value) ? value : 0
  return `${safeValue >= 0 ? '+' : ''}${safeValue}`
}

export function money(cents: number): string {
  const value = Number(cents || 0) / 100
  return `${value.toFixed(2)} 元`
}

// ── Clipboard ───────────────────────────────────────────────────────

export async function copyTextToClipboard(text: string): Promise<boolean> {
  const value = String(text ?? '')
  if (!value.trim()) {
    return false
  }

  if (window.isSecureContext && navigator.clipboard?.writeText) {
    try {
      await navigator.clipboard.writeText(value)
      return true
    } catch {
      // Fall through to the textarea copy path for HTTP or restricted browsers.
    }
  }

  const textarea = document.createElement('textarea')
  textarea.value = value
  textarea.setAttribute('readonly', 'readonly')
  textarea.setAttribute('aria-hidden', 'true')
  textarea.style.position = 'fixed'
  textarea.style.top = '-1000px'
  textarea.style.left = '-1000px'
  textarea.style.width = '2em'
  textarea.style.height = '2em'
  textarea.style.padding = '0'
  textarea.style.border = '0'
  textarea.style.opacity = '0.01'
  textarea.style.pointerEvents = 'none'
  textarea.style.fontSize = '16px'
  document.body.appendChild(textarea)

  const selection = document.getSelection()
  const selectedRange = selection && selection.rangeCount > 0 ? selection.getRangeAt(0) : null
  textarea.focus({ preventScroll: true })
  textarea.select()
  textarea.setSelectionRange(0, textarea.value.length)

  let copied = false
  try {
    copied = document.execCommand('copy')
  } catch {
    copied = false
  } finally {
    document.body.removeChild(textarea)
    if (selectedRange && selection) {
      selection.removeAllRanges()
      selection.addRange(selectedRange)
    }
  }

  return copied
}

// ── Prompt tags ─────────────────────────────────────────────────────

export function parsePromptTagString(value: string): string[] {
  const trimmed = value.trim()
  if (!trimmed) return []
  if (trimmed.startsWith('[')) {
    try {
      const parsed = JSON.parse(trimmed)
      if (Array.isArray(parsed)) return parsed.map((item) => String(item ?? ''))
    } catch {
      // Fall back to separator parsing for legacy or malformed values.
    }
  }
  return trimmed.split(/[,，#\s]+/).filter(Boolean)
}

// ── Task / Activity status labels ───────────────────────────────────

export function getTaskStatusLabel(status: string): string {
  switch (status) {
    case 'idle':
      return '待开始'
    case 'running':
    case 'generating':
      return '正在生成'
    case 'success':
      return '成功'
    case 'partial_success':
      return '部分成功'
    case 'failed':
      return '失败'
    default:
      return status || '未知'
  }
}

export function getActivityRecordLabel(recordType: UserActivityItem['recordType']): string {
  switch (recordType) {
    case 'image_task':
      return '生图'
    case 'points_log':
      return '积分'
    case 'credit_log':
      return '福利次数'
    default:
      return '记录'
  }
}

export function getActivityShortLabel(recordType: UserActivityItem['recordType']): string {
  switch (recordType) {
    case 'image_task':
      return '图'
    case 'points_log':
      return '分'
    case 'credit_log':
      return '次'
    default:
      return '记'
  }
}

export function getActivityCreditDelta(item: UserActivityItem): number {
  return Number(item.deltaCredits ?? (item.deltaFreeCredits ?? 0) + (item.deltaPaidCredits ?? 0))
}

export function getActivityRecordValue(item: UserActivityItem): string {
  if (item.recordType === 'points_log' && item.deltaPoints != null) {
    return `积分 ${formatSignedNumber(item.deltaPoints)}`
  }
  const creditDelta = getActivityCreditDelta(item)
  if (item.recordType === 'credit_log' || creditDelta !== 0) {
    return `福利次数 ${formatSignedNumber(creditDelta)}`
  }
  if (item.status) {
    return getTaskStatusLabel(item.status)
  }
  return formatDateTime(item.createdAt)
}

// ── User / Role labels ──────────────────────────────────────────────

export function getUserRoleLabel(role: string): string {
  switch (role) {
    case 'admin':
      return '管理员'
    case 'user':
      return '普通用户'
    default:
      return role || '未知'
  }
}

// ── Points / Credits labels ─────────────────────────────────────────

export function getPointsChangeTypeLabel(changeType: string): string {
  switch (changeType) {
    case 'checkin':
      return '每日签到'
    case 'purchase':
      return '兑换码入账'
    case 'purchase_bonus':
      return '兑换码赠送'
    case 'redeem':
      return '积分兑换'
    case 'settlement':
      return '日终转换'
    case 'generate':
      return '生图消耗'
    case 'refund':
      return '失败退回'
    case 'adjust':
      return '后台调整'
    case 'redeem_code':
      return '兑换码兑换'
    case 'invite':
      return '邀请奖励'
    default:
      return changeType || '积分变动'
  }
}

export function getCreditChangeTypeLabel(changeType: string): string {
  switch (changeType) {
    case 'checkin':
      return '每日签到福利次数'
    case 'register':
      return '注册赠送福利次数'
    case 'redeem':
      return '积分兑换福利次数'
    case 'purchase':
      return '兑换码入账福利次数'
    case 'settlement':
      return '日终转换福利次数'
    case 'generate':
      return '生图消耗福利次数'
    case 'refund':
      return '失败退回福利次数'
    case 'adjust':
      return '后台调整福利次数'
    case 'redeem_code':
      return '兑换码兑换福利次数'
    case 'invite':
      return '邀请奖励福利次数'
    default:
      return changeType || '福利次数变动'
  }
}

export function getCreditLogTotals(item: CreditLogItem): { beforeTotal: number; afterTotal: number; deltaTotal: number } {
  const beforeTotal = Number(item.beforeCredits ?? item.beforeFree + item.beforePaid)
  const afterTotal = Number(item.afterCredits ?? item.afterFree + item.afterPaid)
  const deltaTotal = Number(item.deltaCredits ?? item.deltaFree + item.deltaPaid)

  return { beforeTotal, afterTotal, deltaTotal }
}

// ── Invite ──────────────────────────────────────────────────────────

export function buildInviteLink(inviteCode: string): string {
  const code = inviteCode.trim()
  if (!code) {
    return ''
  }
  return `${window.location.origin}/register?invite=${encodeURIComponent(code)}`
}

export function formatInviteReward(credits: number, points: number): string {
  const parts: string[] = []
  if (credits > 0) {
    parts.push(`${credits} 福利次数`)
  }
  if (points > 0) {
    parts.push(`${points} 积分`)
  }
  return parts.length > 0 ? parts.join(' + ') : '0 奖励'
}

// ── Reference Images ────────────────────────────────────────────────

export function getReferenceImageNameFromURL(rawURL: string): string {
  try {
    const parsed = new URL(rawURL, window.location.origin)
    const fileName = decodeURIComponent(parsed.pathname.split('/').filter(Boolean).pop() || '')
    return fileName || ''
  } catch {
    return rawURL.split('/').filter(Boolean).pop() || ''
  }
}

export function syncPrimaryReferenceImage(images: ReferenceImageItem[]): ReferenceImageItem[] {
  return images
    .map((image) => ({
      ...image,
      url: image.url.trim(),
      name: image.name.trim(),
    }))
    .filter((image) => image.url)
    .slice(0, MAX_REFERENCE_IMAGES)
}

export function getReferenceImageExtraPointCost(count: number): number {
  return Math.max(0, count - FREE_REFERENCE_IMAGE_COUNT)
}

export function triggerImageDownload(blob: Blob, fileName: string): void {
  const downloadBlob = blob.type === 'application/octet-stream' ? blob : new Blob([blob], { type: 'application/octet-stream' })
  const objectUrl = window.URL.createObjectURL(downloadBlob)
  const anchor = document.createElement('a')
  anchor.href = objectUrl
  anchor.download = fileName
  anchor.rel = 'noopener noreferrer'
  anchor.target = '_self'
  anchor.style.display = 'none'
  document.body.appendChild(anchor)
  anchor.click()
  window.setTimeout(() => {
    window.URL.revokeObjectURL(objectUrl)
    anchor.remove()
  }, 10_000)
}

export function isWeChatBrowser(): boolean {
  if (typeof navigator === 'undefined') {
    return false
  }

  const userAgent = navigator.userAgent.toLowerCase()
  return userAgent.includes('micromessenger')
}
