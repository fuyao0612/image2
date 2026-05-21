import React from 'react';
import type { ReactNode } from 'react';
import type { ImageTaskDetailResponse, ImageTaskItem } from '../types/api';
import type { GenerationProgress, GenerationStatus, TaskRequestSnapshot } from '../types/app';
import { getTaskStatusLabel } from '../utils/helpers';
import { sizeOptions, styleOptions, qualityOptions, outputFormatOptions } from '../constants/app';

export function formatPreviewOptionValue(value: string | undefined, options: { value: string; label: string }[]) {
  if (!value) return '-'
  return options.find((o) => o.value === value)?.label || value
}

export function normalizeReferenceImagesFromSnapshot(snapshot: TaskRequestSnapshot | null) {
  if (!snapshot?.referenceImages) return []
  return snapshot.referenceImages.filter((img) => typeof img === 'string' || (img && typeof img.url === 'string'))
}

export function normalizeImageUrlForDedupe(url: string) {
  try {
    const u = new URL(url)
    u.search = ''
    return u.toString()
  } catch {
    return url.split('?')[0]
  }
}

export function isLikelyThumbnailUrl(url: string) {
  return url.includes('thumbnail') || url.includes('w=') || url.includes('width=')
}

export function buildImageElapsedSeconds(total: number, elapsed: number, existing: number[]) {
  return [elapsed]
}

export function getImageTaskStatusLabel(status: string): string {
  return getTaskStatusLabel(status)
}

export function limitImageURLs(imageURLs: string[], requestedCount?: number): string[] {
  const limit = Math.max(1, Math.floor(Number(requestedCount || 0)))
  if (!Number.isFinite(limit) || limit <= 0 || imageURLs.length <= limit) {
    return imageURLs
  }
  return imageURLs.slice(0, limit)
}

export function normalizeImageURLs(imageUrls?: string[], imageUrl?: string): string[] {
  const seen = new Map<string, string>()
  const result: string[] = []
  for (const value of [...(imageUrls ?? []), imageUrl ?? '']) {
    const trimmed = value.trim()
    const dedupeKey = normalizeImageUrlForDedupe(trimmed)
    if (!trimmed) {
      continue
    }
    const existing = seen.get(dedupeKey)
    if (existing) {
      if (isLikelyThumbnailUrl(existing) && !isLikelyThumbnailUrl(trimmed)) {
        const index = result.indexOf(existing)
        if (index >= 0) {
          result[index] = trimmed
        }
        seen.set(dedupeKey, trimmed)
      }
      continue
    }
    seen.set(dedupeKey, trimmed)
    result.push(trimmed)
  }
  return result
}

export function getPreviewRequestedCount(preview: Pick<ImageTaskDetailResponse, 'requestedCount' | 'requestParams' | 'costCredits' | 'costPoints'>, fallback = 0): number {
  const parsed = parseTaskRequestParams(preview.requestParams)
  const value = Number(parsed?.count ?? preview.requestedCount ?? fallback)
  if (Number.isFinite(value) && value > 0) {
    return clampCount(value)
  }

  const legacyValue = Number(preview.costCredits || preview.costPoints || fallback)
  return Number.isFinite(legacyValue) && legacyValue > 0 ? clampCount(legacyValue) : Math.max(0, fallback)
}

export function isPartialImageTaskStatus(status: string, imageCount: number, requestedCount?: number): boolean {
  const requested = Math.max(1, Math.floor(Number(requestedCount || 0)))
  if (!Number.isFinite(requested) || requested <= 1) {
    return false
  }
  return status === 'success' && imageCount > 0 && imageCount < requested
}

export function getTaskStatusBadgeClass(status: string): string {
  switch (status) {
    case 'success':
      return 'bg-emerald-500/15 text-emerald-200'
    case 'partial_success':
      return 'bg-amber-500/15 text-amber-100'
    case 'failed':
      return 'bg-rose-500/15 text-rose-200'
    case 'running':
    case 'generating':
      return 'bg-amber-500/15 text-amber-200'
    default:
      return 'bg-slate-800 text-slate-300'
  }
}

export function formatTaskCost(
  task: Pick<ImageTaskDetailResponse, 'costPoints' | 'costCredits' | 'emailNotifyCostPoints'>,
  fallbackCredits = 0,
): string {
  const points =
    Math.max(0, Math.floor(Number(task.costPoints || 0))) +
    Math.max(0, Math.floor(Number(task.emailNotifyCostPoints || 0)))
  const credits = Math.max(0, Math.floor(Number(task.costCredits || 0)))

  if (points > 0 && credits > 0) {
    return `${points}积分+${credits}福利次数`
  }
  if (points > 0) {
    return `${points}积分`
  }
  if (credits > 0) {
    return `${credits}福利次数`
  }

  const fallback = Math.max(0, Math.floor(Number(fallbackCredits || 0)))
  return fallback > 0 ? `${fallback}福利次数` : '-'
}

export function getTaskCreditCost(task: { costCredits?: number; freeCreditsCost?: number; paidCreditsCost?: number }): number {
  return Number(task.costCredits ?? (task.freeCreditsCost ?? 0) + (task.paidCreditsCost ?? 0))
}

export function getPreviewElapsedSeconds(preview: Pick<ImageTaskDetailResponse, 'createdAt' | 'updatedAt'>): number {
  const created = Date.parse(preview.createdAt || '')
  const updated = Date.parse(preview.updatedAt || '')
  if (!Number.isFinite(created) || !Number.isFinite(updated) || updated < created) {
    return 0
  }
  return Math.max(0, Math.floor((updated - created) / 1000))
}

export function formatElapsedTriplet(total: number, elapsedSeconds: number, existing: number[] | undefined): string {
  const count = Math.max(1, Math.floor(Number(total || 0)))
  const values = buildImageElapsedSeconds(count, elapsedSeconds, existing ?? [])
  return values.join('/')
}

export function buildPreviewImageCandidates(thumbnailUrl?: string, imageUrl?: string): string[] {
  const candidates: string[] = []
  for (const value of [thumbnailUrl ?? '', imageUrl ?? '']) {
    const trimmed = value.trim()
    if (!trimmed) {
      continue
    }
    if (candidates.includes(trimmed)) {
      continue
    }
    candidates.push(trimmed)
  }
  return candidates
}

export function createCenteredThumbnailSlots(
  imageURLs: string[],
  activeIndex: number,
  maxSlots = 5,
): Array<{ imageUrl: string; index: number } | null> {
  const center = Math.floor(maxSlots / 2)
  return Array.from({ length: maxSlots }, (_, slotIndex) => {
    const imageIndex = activeIndex + slotIndex - center
    if (imageIndex < 0 || imageIndex >= imageURLs.length) {
      return null
    }
    return {
      imageUrl: imageURLs[imageIndex],
      index: imageIndex,
    }
  })
}

export function getPreviewEmptyMessage(status: string): string {
  switch (status) {
    case 'running':
    case 'generating':
      return '请稍等，任务耗时会随着图片质量增加，感谢理解。'
    case 'failed':
      return '本次生成未成功，可查看失败原因后调整参数再试。'
    case 'partial_success':
      return '部分图片生成成功，其余图片暂未返回，可稍后刷新或查看历史任务。'
    case 'success':
      return '任务已完成，但暂未读取到可展示图片，请稍后刷新或查看历史任务。'
    case 'idle':
      return '填写提示词并提交后，生成结果会显示在这里。'
    default:
      return '当前暂无可展示图片，请稍后刷新或重新选择任务。'
  }
}

export function highlightSearchText(text: string, keyword: string): ReactNode {
  const trimmed = keyword.trim()
  if (!trimmed) return text
  const index = text.toLowerCase().indexOf(trimmed.toLowerCase())
  if (index < 0) return text
  return (
    <React.Fragment>
      {text.slice(0, index)}
      <span className="text-orange-500">{text.slice(index, index + trimmed.length)}</span>
      {text.slice(index + trimmed.length)}
    </React.Fragment>
  )
}

export function getGenerationStatusLabel(status: GenerationStatus): string {
  return getTaskStatusLabel(status)
}

export function buildPreviewRequestOptions(preview: ImageTaskDetailResponse): Array<{ label: string; value: string; wide?: boolean }> {
  const parsed = parseTaskRequestParams(preview.requestParams)
  const count = Number(parsed?.count ?? preview.requestedCount ?? preview.costCredits ?? preview.costPoints ?? 0)
  const referenceImages = normalizeReferenceImagesFromSnapshot(parsed)

  return [
    {
      label: '提示词',
      value: parsed?.prompt || preview.prompt || '-',
      wide: true,
    },
    {
      label: '图片尺寸',
      value: formatPreviewOptionValue(parsed?.size, sizeOptions),
    },
    {
      label: '生成张数',
      value: Number.isFinite(count) && count > 0 ? `${count} 张` : '-',
    },
    {
      label: '风格',
      value: formatPreviewOptionValue(parsed?.style, styleOptions),
    },
    {
      label: '质量',
      value: formatPreviewOptionValue(parsed?.quality, qualityOptions),
    },
    {
      label: '输出格式',
      value: formatPreviewOptionValue(parsed?.outputFormat, outputFormatOptions),
    },
    {
      label: '参考图片',
      value: referenceImages.length ? `${referenceImages.length} 张` : '未使用',
      wide: true,
    },
  ]
}

export function parseTaskRequestParams(raw?: string): TaskRequestSnapshot | null {
  if (!raw?.trim()) {
    return null
  }

  try {
    const parsed = JSON.parse(raw) as TaskRequestSnapshot
    return parsed && typeof parsed === 'object' && !Array.isArray(parsed) ? parsed : null
  } catch {
    return null
  }
}

export function clampCount(value: number): number {
  if (!Number.isFinite(value) || value <= 0) {
    return 1
  }
  return Math.min(6, Math.max(1, Math.floor(value)))
}