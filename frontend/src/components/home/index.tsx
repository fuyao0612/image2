import React, { useRef, useState, useEffect } from 'react';
import { ArrowUpDown, BadgeCheck, CheckCircle2, ChevronDown, ChevronLeft, ChevronRight, Copy, Download, Eye, EyeOff, Gift, Heart, History, Home, ImageIcon, ImagePlus, KeyRound, Link2, LoaderCircle, LogIn, LogOut, Mail, Megaphone, Menu, Moon, PanelLeftClose, PanelLeftOpen, RefreshCcw, ReceiptText, Send, Search, Sparkles, ShoppingBag, Sun, Timer, Upload, UserRound, UserRoundPlus, ShieldCheck, Wallet, XCircle, MoreVertical } from 'lucide-react';
import { formatDateTime, money } from '../../utils/helpers';
import type { ImageTaskDetailResponse, ImageTaskItem } from '../../types/api';
import type { PromptGalleryItem, ReferenceImageItem, GenerationProgress } from '../../types/app';
import { MiniStat, PanelPagination } from '../shared';
import { getImageTaskStatusLabel, limitImageURLs, normalizeImageURLs, getPreviewRequestedCount, isPartialImageTaskStatus, getTaskStatusBadgeClass, formatTaskCost, getTaskCreditCost, getPreviewElapsedSeconds, formatElapsedTriplet, buildPreviewImageCandidates, createCenteredThumbnailSlots, getPreviewEmptyMessage, highlightSearchText, getGenerationStatusLabel, buildPreviewRequestOptions, parseTaskRequestParams } from '../../utils/imageTasks';
import { HISTORY_PAGE_SIZE, MAX_REFERENCE_IMAGES, FREE_REFERENCE_IMAGE_COUNT, sizeOptions, styleOptions, qualityOptions, outputFormatOptions, CUSTOM_SIZE_MAX_SIDE, GENERATION_WAITING_NOTICE } from '../../constants/app';

export function WechatSaveGuide({
  imageUrl,
  title,
  onClose,
}: {
  imageUrl: string
  title: string
  onClose: () => void
}) {
  return (
    <div className="fixed inset-0 z-50 flex items-center justify-center bg-slate-950/80 px-4 py-6 backdrop-blur-sm" role="dialog" aria-modal="true">
      <button type="button" className="absolute inset-0 cursor-default" aria-label="关闭保存提示" onClick={onClose} />
      <div className="relative flex max-h-full w-full max-w-3xl flex-col overflow-hidden rounded-lg border border-slate-700 bg-slate-950 shadow-2xl">
        <div className="flex items-center justify-between border-b border-slate-800 px-4 py-3">
          <div>
            <h2 className="text-base font-medium text-slate-50">保存图片</h2>
            <p className="mt-1 text-xs text-slate-400">微信内请长按图片保存到相册，也可以点击右上角菜单在浏览器中打开后下载。</p>
          </div>
          <button
            type="button"
            onClick={onClose}
            className="inline-flex h-9 w-9 items-center justify-center rounded-md border border-slate-800 bg-slate-900 text-slate-300 transition hover:border-violet-400 hover:text-white"
            aria-label="关闭"
          >
            <XCircle className="h-5 w-5" />
          </button>
        </div>

        <div className="flex min-h-0 flex-1 flex-col gap-4 overflow-auto p-4">
          <div className="flex min-h-[300px] items-center justify-center rounded-md border border-slate-800 bg-slate-900/70 p-3">
            <img src={imageUrl} alt={title || '当前图片'} className="max-h-[70vh] w-full select-auto rounded-md object-contain" />
          </div>
          <div className="flex flex-col gap-2 sm:flex-row sm:items-center sm:justify-between">
            <p className="text-sm text-slate-400">如果长按没有出现保存选项，请点击下面的“打开原图”后再长按保存。</p>
            <a
              href={imageUrl}
              target="_blank"
              rel="noopener noreferrer"
              className="inline-flex h-10 items-center justify-center gap-2 rounded-md border border-slate-700 bg-slate-900 px-4 text-sm text-slate-100 transition hover:border-violet-400 hover:text-white"
            >
              <Link2 className="h-4 w-4" />
              打开原图
            </a>
          </div>
        </div>
      </div>
    </div>
  )
}

export function ReferenceImagesDialog({
  images,
  onDelete,
  onClose,
}: {
  images: ReferenceImageItem[]
  onDelete: (index: number) => void
  onClose: () => void
}) {
  return (
    <div className="fixed inset-0 z-50 flex items-center justify-center bg-slate-950/80 px-4 py-6 backdrop-blur-sm" role="dialog" aria-modal="true">
      <button type="button" className="absolute inset-0 cursor-default" aria-label="关闭参考图列表" onClick={onClose} />
      <div className="relative flex max-h-[82vh] w-full max-w-md flex-col overflow-hidden rounded-2xl border border-slate-200 bg-slate-50 shadow-2xl">
        <div className="flex items-center justify-between gap-3 border-b border-slate-200 bg-white px-4 py-3">
          <div>
            <h2 className="text-base font-semibold text-slate-900">已添加的参考图片</h2>
            <p className="mt-1 text-xs text-slate-500">共 {images.length} 张，超过 {FREE_REFERENCE_IMAGE_COUNT} 张后每张额外 1 积分。</p>
          </div>
          <button
            type="button"
            onClick={onClose}
            className="inline-flex h-8 w-8 items-center justify-center rounded-full border border-slate-200 text-slate-500 transition hover:border-violet-300 hover:text-violet-600"
            aria-label="关闭"
          >
            <XCircle className="h-4 w-4" />
          </button>
        </div>
        <div className="space-y-3 overflow-y-auto p-3">
          {images.map((image, index) => (
            <div key={`${image.url}-${index}`} className="flex items-center gap-3 rounded-2xl bg-white p-3 shadow-sm">
              <div className="h-16 w-16 shrink-0 overflow-hidden rounded-2xl bg-slate-100">
                <img src={image.url} alt={image.name || `参考图 ${index + 1}`} className="h-full w-full object-cover" />
              </div>
              <div className="min-w-0 flex-1">
                <div className="truncate text-sm font-semibold text-slate-900" title={image.name}>
                  {image.name || `参考图 ${index + 1}`}
                </div>
                <div className="mt-1 line-clamp-2 break-all text-xs leading-5 text-slate-500" title={image.url}>
                  {image.url}
                </div>
              </div>
              <button
                type="button"
                onClick={() => onDelete(index)}
                className="shrink-0 rounded-xl px-2 py-1 text-sm text-slate-700 transition hover:bg-rose-50 hover:text-rose-600"
              >
                删除
              </button>
            </div>
          ))}
        </div>
      </div>
    </div>
  )
}

export function PreviewDetailDialog({
  preview,
  onApply,
  onClose,
}: {
  preview: ImageTaskDetailResponse
  onApply: () => void
  onClose: () => void
}) {
  const requestOptions = buildPreviewRequestOptions(preview)
  const parsed = parseTaskRequestParams(preview.requestParams)
  const promptText = parsed?.prompt?.trim() || preview.prompt || '未填写'

  return (
    <div className="fixed inset-0 z-50 flex items-center justify-center bg-slate-950/80 px-4 py-6 backdrop-blur-sm" role="dialog" aria-modal="true">
      <button type="button" className="absolute inset-0 cursor-default" aria-label="关闭图片详情" onClick={onClose} />
      <div className="relative flex max-h-full w-full max-w-3xl flex-col overflow-hidden rounded-lg border border-slate-700 bg-slate-950 shadow-2xl">
        <div className="flex items-center justify-between gap-3 border-b border-slate-800 px-4 py-3">
          <div className="min-w-0">
            <h2 className="text-base font-medium text-slate-50">提示词详情</h2>
            <p className="mt-1 text-xs text-slate-400">只展示提示词和生成选项，方便直接复用。</p>
          </div>
          <button
            type="button"
            onClick={onClose}
            className="inline-flex h-9 w-9 shrink-0 items-center justify-center rounded-md border border-slate-800 bg-slate-900 text-slate-300 transition hover:border-violet-400 hover:text-white"
            aria-label="关闭"
          >
            <XCircle className="h-5 w-5" />
          </button>
        </div>

        <div className="min-h-0 flex-1 overflow-auto p-4">
          <div className="space-y-4">
            <div className="rounded-md border border-slate-800 bg-slate-900/70 px-3 py-3">
              <div className="text-xs text-slate-500">提示词</div>
              <div className="mt-2 whitespace-pre-wrap break-words text-sm leading-6 text-slate-100">
                {promptText}
              </div>
            </div>

            <div className="grid gap-2 sm:grid-cols-2">
              {requestOptions
                .filter((option) => option.label !== '提示词')
                .map((option) => (
                  <div
                    key={option.label}
                    className={`min-w-0 rounded-md border border-slate-800 bg-slate-900/70 px-3 py-2 ${
                      option.wide ? 'sm:col-span-2' : ''
                    }`}
                  >
                    <div className="text-xs text-slate-500">{option.label}</div>
                    <div className="mt-1 whitespace-pre-wrap break-words text-sm leading-6 text-slate-100">
                      {option.value}
                    </div>
                  </div>
                ))}
            </div>

            <div className="flex items-center justify-end gap-2 pt-1">
              <button
                type="button"
                onClick={onClose}
                className="inline-flex h-10 items-center justify-center rounded-md border border-slate-700 bg-slate-900 px-4 text-sm text-slate-100 transition hover:border-slate-500 hover:text-white"
              >
                关闭
              </button>
              <button
                type="button"
                onClick={onApply}
                className="inline-flex h-10 items-center justify-center rounded-md bg-violet-500 px-4 text-sm font-medium text-slate-950 transition hover:bg-violet-400"
              >
                一键应用
              </button>
            </div>
          </div>
        </div>
      </div>
    </div>
  )
}

export function TaskHistoryPanel({
  items,
  page,
  pageSize,
  total,
  selectedTaskId,
  onSelect,
  onPageChange,
}: {
  items: ImageTaskItem[]
  page: number
  pageSize: number
  total: number
  selectedTaskId: string | null
  onSelect: (item: ImageTaskItem) => void
  onPageChange: (page: number) => void
}) {
  return (
    <section className="rounded-lg border border-slate-800 bg-slate-900/70 p-4 shadow-panel">
      <div className="flex items-center justify-between gap-3">
        <div>
          <h2 className="text-lg font-medium text-slate-50">历史记录</h2>
          <p className="mt-1 text-sm text-slate-400">最近提交的生图任务，每页 {pageSize} 条</p>
        </div>
        <History className="h-5 w-5 text-violet-300" />
      </div>

      <div className="mt-4 space-y-3">
        {items.length ? (
          items.map((item) => {
            const imageURLs = limitImageURLs(normalizeImageURLs(item.imageUrls, item.imageUrl), item.requestedCount)
            const cover = item.thumbnailUrl || item.thumbnailUrls?.[0] || imageURLs[0] || ''
            const requestedCount = getPreviewRequestedCount(item, imageURLs.length || 1)
            const isPartialSuccess = isPartialImageTaskStatus(item.status, imageURLs.length, requestedCount)
            const displayStatus = isPartialSuccess ? 'partial_success' : item.status
            return (
              <button
                key={item.id}
                type="button"
                aria-pressed={selectedTaskId === item.id}
                onClick={() => onSelect(item)}
                className={`relative flex w-full items-start gap-3 overflow-hidden rounded-lg border p-3 text-left transition focus:outline-none focus-visible:ring-2 focus-visible:ring-violet-400/30 ${
                  selectedTaskId === item.id
                    ? 'border-violet-300 bg-violet-50 shadow-md ring-2 ring-violet-300/40'
                    : 'border-slate-200 bg-white hover:border-violet-300 hover:bg-violet-50/50'
                }`}
              >
                {selectedTaskId === item.id ? (
                  <span className="absolute inset-y-0 left-0 w-1.5 bg-violet-500" aria-hidden="true" />
                ) : null}
                <div className="flex h-14 w-14 shrink-0 items-center justify-center overflow-hidden rounded-md border border-slate-800 bg-slate-900 text-xs font-medium text-slate-300">
                  {cover ? (
                    <img src={cover} alt={item.prompt} className="h-full w-full object-cover" />
                  ) : (
                    <span>{getImageTaskStatusLabel(displayStatus)}</span>
                  )}
                </div>
                <div className="min-w-0 flex-1">
                  <div className="flex items-center justify-between gap-2">
                    <span className="truncate text-sm font-medium text-slate-200">{item.prompt}</span>
                    <span className={`shrink-0 rounded-md px-2 py-0.5 text-xs ${getTaskStatusBadgeClass(displayStatus)}`}>
                      {getImageTaskStatusLabel(displayStatus)}
                    </span>
                  </div>
                  <div className="mt-2 flex flex-wrap items-center gap-2 text-xs text-slate-500">
                    <span>{formatDateTime(item.createdAt)}</span>
                    <span>消耗 {formatTaskCost(item, getTaskCreditCost(item))}</span>
                    {isPartialSuccess ? <span>{imageURLs.length}/{requestedCount} 张图</span> : imageURLs.length > 1 ? <span>{imageURLs.length} 张图</span> : null}
                  </div>
                </div>
              </button>
            )
          })
        ) : (
          <div className="rounded-lg border border-dashed border-slate-800 bg-slate-950 p-4 text-sm text-slate-500">
            暂无生图历史。
          </div>
        )}
      </div>

      <PanelPagination page={page} pageSize={pageSize} total={total} onPageChange={onPageChange} />
    </section>
  )
}

export function TaskDetailPanel({
  preview,
  progress,
}: {
  preview: ImageTaskDetailResponse
  progress: GenerationProgress | null
}) {
  const imageURLs = limitImageURLs(normalizeImageURLs(preview.imageUrls, preview.imageUrl), preview.requestedCount)
  const imageCount = imageURLs.length
  const rawStatus = progress?.status ?? preview.status
  const totalCount = Math.max(1, progress?.total ?? getPreviewRequestedCount(preview, imageCount || 1))
  const isPartialSuccess = isPartialImageTaskStatus(rawStatus, imageCount, totalCount)
  const displayStatus = isPartialSuccess ? 'partial_success' : rawStatus
  const completedFallback = rawStatus === 'success' ? (isPartialSuccess ? imageCount : totalCount) : imageCount
  const completedCount = Math.min(totalCount, Math.max(0, progress?.completed ?? completedFallback))
  const elapsedSeconds = progress?.elapsedSeconds ?? getPreviewElapsedSeconds(preview)
  const elapsedText = formatElapsedTriplet(totalCount, elapsedSeconds, progress?.imageElapsedSeconds ?? preview.imageElapsedSeconds)
  const visibleErrorMessage = rawStatus === 'failed' || isPartialSuccess ? preview.errorMessage : ''

  return (
    <div className="mt-4 rounded-lg border border-slate-800 bg-slate-950 p-4">
      <div className="flex items-center justify-between gap-3">
        <div className="flex items-center gap-2 text-sm font-medium text-slate-200">
          <Copy className="h-4 w-4 text-slate-400" />
          任务状态
        </div>
        <span className={`rounded-md px-2 py-1 text-xs ${getTaskStatusBadgeClass(displayStatus)}`}>
          {getImageTaskStatusLabel(displayStatus)}
        </span>
      </div>

      <div className="mt-4 grid gap-3 sm:grid-cols-3">
        <MiniStat label="完成" value={`${completedCount}/${totalCount}`} />
        <MiniStat label="耗时·s" value={elapsedText} />
        <MiniStat label="消耗" value={formatTaskCost(preview, totalCount)} />
      </div>

      {visibleErrorMessage ? (
        <div className={`mt-3 rounded-md border px-3 py-2 text-sm ${isPartialSuccess ? 'border-amber-500/20 bg-amber-500/10 text-amber-100' : 'border-rose-500/20 bg-rose-500/10 text-rose-200'}`}>
          {visibleErrorMessage}
        </div>
      ) : null}
    </div>
  )
}

export function PreviewImageStage({
  preview,
  imageURLs,
  thumbnailURLs,
  activeIndex,
  onPrevious,
  onNext,
  onSaveCurrent,
  onSelectThumbnail,
  onImageLoadError,
}: {
  preview: ImageTaskDetailResponse
  imageURLs: string[]
  thumbnailURLs: string[]
  activeIndex: number
  onPrevious: () => void
  onNext: () => void
  onSaveCurrent: () => void
  onSelectThumbnail: (index: number) => void
  onImageLoadError: (taskId: string) => Promise<ImageTaskDetailResponse | null>
}) {
  const hasImages = imageURLs.length > 0
  const currentIndex = hasImages ? Math.min(Math.max(activeIndex, 0), imageURLs.length - 1) : -1
  const currentImageUrl = currentIndex >= 0 ? imageURLs[currentIndex] : ''
  const currentThumbnailUrl = currentIndex >= 0 ? thumbnailURLs[currentIndex] ?? thumbnailURLs[0] ?? '' : ''
  const displayCandidates = buildPreviewImageCandidates(currentImageUrl, currentThumbnailUrl)
  const [failedDisplayURLs, setFailedDisplayURLs] = useState<Set<string>>(() => new Set())
  const imageReloadedRef = useRef<string>('')
  const canNavigate = imageURLs.length > 1
  const thumbnailSlots = createCenteredThumbnailSlots(thumbnailURLs.length > 0 ? thumbnailURLs : imageURLs, currentIndex, 5)
  const currentDisplayUrl = displayCandidates.find((candidate) => !failedDisplayURLs.has(candidate)) ?? ''
  const emptyMessage = getPreviewEmptyMessage(preview.status)

  useEffect(() => {
    setFailedDisplayURLs(new Set())
    imageReloadedRef.current = ''
  }, [currentIndex, currentImageUrl, currentThumbnailUrl, preview.taskId])

  function handleImageError() {
    if (currentDisplayUrl) {
      setFailedDisplayURLs((current) => {
        const next = new Set(current)
        next.add(currentDisplayUrl)
        return next
      })
    }
    if (displayCandidates.some((candidate) => candidate !== currentDisplayUrl && !failedDisplayURLs.has(candidate))) {
      return
    }
    if (!preview.taskId || preview.taskId === '-') {
      return
    }
    const reloadKey = `${preview.taskId}:${currentIndex}:${currentImageUrl}:${currentThumbnailUrl}`
    if (imageReloadedRef.current === reloadKey) {
      return
    }
    imageReloadedRef.current = reloadKey
    void onImageLoadError(preview.taskId)
  }

  return (
    <div className="mt-4 rounded-lg border border-slate-800 bg-slate-950 p-4">
      <div className="flex min-h-[460px] items-center justify-center rounded-md border border-dashed border-slate-700 bg-slate-900/40 p-4">
        {currentDisplayUrl ? (
          <img
            key={`${preview.taskId}:${currentIndex}:${currentDisplayUrl}`}
            src={currentDisplayUrl}
            alt={preview.prompt ? `${preview.prompt} 第${currentIndex + 1}张` : `第 ${currentIndex + 1} 张图片`}
            className="max-h-[420px] w-full rounded-md object-contain"
            loading="eager"
            decoding="async"
            onError={handleImageError}
          />
        ) : (
          <div className="max-w-2xl text-center">
            <div className="mx-auto flex h-16 w-16 items-center justify-center rounded-md border border-slate-700 bg-slate-900 text-violet-300">
              {preview.status === 'generating' || preview.status === 'running' ? (
                <LoaderCircle className="h-8 w-8 animate-spin" />
              ) : (
                <ImageIcon className="h-8 w-8" />
              )}
            </div>
            <h3 className="mt-4 text-xl font-medium text-slate-50">{getImageTaskStatusLabel(preview.status)}</h3>
            <p className="mt-2 text-sm leading-6 text-slate-400">{emptyMessage}</p>
          </div>
        )}
      </div>

      <div className="mt-4 flex flex-wrap items-center justify-between gap-3">
        <div className="flex items-center gap-2">
          <button
            type="button"
            onClick={onPrevious}
            disabled={!canNavigate}
            className="inline-flex h-10 w-10 items-center justify-center rounded-md border border-slate-700 bg-slate-900 text-slate-200 transition hover:border-violet-400 disabled:cursor-not-allowed disabled:opacity-40"
            aria-label="上一张"
          >
            <ChevronLeft className="h-4 w-4" />
          </button>
          <button
            type="button"
            onClick={onNext}
            disabled={!canNavigate}
            className="inline-flex h-10 w-10 items-center justify-center rounded-md border border-slate-700 bg-slate-900 text-slate-200 transition hover:border-violet-400 disabled:cursor-not-allowed disabled:opacity-40"
            aria-label="下一张"
          >
            <ChevronRight className="h-4 w-4" />
          </button>
          <span className="text-sm text-slate-500">
            {hasImages ? `${currentIndex + 1} / ${imageURLs.length}` : '0 / 0'}
          </span>
        </div>

        <button
          type="button"
          onClick={onSaveCurrent}
          disabled={!currentImageUrl}
          className="inline-flex h-10 items-center gap-2 rounded-md border border-slate-700 bg-slate-900 px-3 text-sm text-slate-200 transition hover:border-violet-400 hover:text-white disabled:cursor-not-allowed disabled:opacity-40"
        >
          <Download className="h-4 w-4" />
          保存当前图片
        </button>
      </div>

      {hasImages ? (
        <div className="mt-4 grid grid-cols-5 gap-2">
          {thumbnailSlots.map((slot, slotIndex) =>
            slot ? (
              <button
                key={slot.index}
                type="button"
                onClick={() => onSelectThumbnail(slot.index)}
                className={`aspect-square overflow-hidden rounded-md border transition ${
                  slot.index === currentIndex ? 'border-violet-400' : 'border-slate-800 hover:border-slate-600'
                }`}
              >
                <img
                  key={`${preview.taskId}:thumb:${slot.index}:${slot.imageUrl}`}
                  src={slot.imageUrl}
                  alt={`缩略图 ${slot.index + 1}`}
                  className="h-full w-full object-cover"
                />
              </button>
            ) : (
              <div key={`empty-${slotIndex}`} className="aspect-square rounded-md border border-dashed border-slate-800 bg-slate-900/40" />
            ),
          )}
        </div>
      ) : null}
    </div>
  )
}

export function PromptSearchOverlay({
  value,
  suggestions,
  onChange,
  onClose,
  onClear,
  onSearch,
}: {
  value: string
  suggestions: Array<PromptGalleryItem & { serialNo?: string }>
  onChange: (value: string) => void
  onClose: () => void
  onClear: () => void
  onSearch: (value?: string) => void
}) {
  const keyword = value.trim()
  return (
    <div className="prompt-search-overlay fixed inset-0 z-50 bg-slate-950 text-slate-100">
      <div className="flex h-full flex-col">
        <div className="flex items-center gap-2 px-3 pb-3 pt-10">
          <button
            type="button"
            onClick={onClose}
            className="inline-flex h-10 w-10 shrink-0 items-center justify-center rounded-md text-slate-200 transition hover:bg-slate-800"
            aria-label="返回"
          >
            <ChevronLeft className="h-7 w-7" />
          </button>
          <div className="flex h-11 min-w-0 flex-1 items-center overflow-hidden rounded-xl bg-slate-800">
            <input
              value={value}
              onChange={(event) => onChange(event.target.value)}
              onKeyDown={(event) => {
                if (event.key === 'Enter') onSearch()
              }}
              autoFocus
              placeholder="搜索编号、标签或上传人"
              className="h-full min-w-0 flex-1 bg-transparent px-4 text-base text-slate-50 outline-none placeholder:text-slate-500"
            />
            {value ? (
              <button
                type="button"
                onClick={onClear}
                className="mr-2 inline-flex h-7 w-7 items-center justify-center rounded-full bg-slate-300 text-slate-700"
                aria-label="清空搜索"
              >
                <XCircle className="h-4 w-4" />
              </button>
            ) : null}
          </div>
          <button
            type="button"
            onClick={() => onSearch()}
            className="h-11 shrink-0 rounded-xl bg-red-600 px-5 text-base font-semibold text-white transition hover:bg-red-500"
          >
            搜索
          </button>
        </div>

        <div className="min-h-0 flex-1 overflow-y-auto">
          {suggestions.length ? (
            suggestions.map((item) => (
              <button
                key={item.id}
                type="button"
                onClick={() => onSearch(item.serialNo ?? item.title)}
                className="flex h-14 w-full items-center gap-3 border-b border-slate-700 px-4 text-left text-lg text-slate-100"
              >
                <Search className="h-6 w-6 shrink-0 text-slate-400" />
                <span className="min-w-0 truncate">
                  {highlightSearchText(item.title, keyword)}
                  {item.serialNo ? <span className="ml-2 text-sm text-slate-500">#{item.serialNo}</span> : null}
                </span>
              </button>
            ))
          ) : (
            <div className="px-4 py-8 text-center text-sm text-slate-500">没有找到相关提示词</div>
          )}
        </div>
      </div>
    </div>
  )
}

export function GenerationProgressPanel({ progresses }: { progresses: GenerationProgress[] }) {
  const runningCount = progresses.filter((item) => item.status === 'running').length
  const successCount = progresses.filter((item) => item.status === 'success').length
  const failedCount = progresses.filter((item) => item.status === 'failed').length

  return (
    <div className="mt-4 rounded-lg border border-slate-800 bg-slate-900 p-4">
      <div className="flex flex-col gap-3 sm:flex-row sm:items-center sm:justify-between">
        <div className="flex items-center gap-2 text-sm font-medium text-slate-100">
          {runningCount > 0 ? (
            <LoaderCircle className="h-4 w-4 animate-spin text-amber-300" />
          ) : failedCount > 0 && successCount === 0 ? (
            <XCircle className="h-4 w-4 text-rose-300" />
          ) : (
            <CheckCircle2 className="h-4 w-4 text-emerald-300" />
          )}
          生成进度
          <span className="rounded-md border border-slate-700 bg-slate-950 px-2 py-0.5 text-xs text-slate-300">
            {progresses.length} 个任务
          </span>
        </div>
        <span className="rounded-md border border-slate-700 bg-slate-950 px-2 py-1 text-xs text-slate-300">
          运行中 {runningCount} · 成功 {successCount} · 失败 {failedCount}
        </span>
      </div>

      <div className="mt-4 space-y-3">
        {progresses.map((progress) => (
          <GenerationProgressCard key={progress.taskId ?? `${progress.backendUrl}-${progress.startedAt}`} progress={progress} />
        ))}
      </div>
    </div>
  )
}

export function GenerationProgressCard({ progress }: { progress: GenerationProgress }) {
  const isRunning = progress.status === 'running'
  const isSuccess = progress.status === 'success'
  const statusClass = isSuccess
    ? 'border-emerald-500/30 bg-emerald-500/10 text-emerald-200'
    : progress.status === 'failed'
      ? 'border-rose-500/30 bg-rose-500/10 text-rose-200'
      : 'border-amber-500/30 bg-amber-500/10 text-amber-200'

  return (
    <div className="rounded-lg border border-slate-800 bg-slate-950 p-4">
      <div className="flex flex-col gap-3 sm:flex-row sm:items-center sm:justify-between">
        <div className="flex items-center gap-2 text-sm font-medium text-slate-100">
          {isRunning ? (
            <LoaderCircle className="h-4 w-4 animate-spin text-amber-300" />
          ) : isSuccess ? (
            <CheckCircle2 className="h-4 w-4 text-emerald-300" />
          ) : (
            <XCircle className="h-4 w-4 text-rose-300" />
          )}
          {progress.taskId || '未知任务'}
        </div>
        <span className={`rounded-md border px-2 py-1 text-xs ${statusClass}`}>
          {getGenerationStatusLabel(progress.status)}
        </span>
      </div>

      <div className="mt-3 grid gap-3 sm:grid-cols-2 xl:grid-cols-3">
        <ProgressStat label="服务地址" value={progress.backendUrl} />
        <ProgressStat label="温馨提示" value={isRunning ? GENERATION_WAITING_NOTICE : progress.prompt || '未填写'} />
        <ProgressStat label="完成数量" value={`${progress.completed}/${progress.total}`} />
        <ProgressStat
          label="耗时·s"
          value={formatElapsedTriplet(progress.total, Math.max(0, Math.floor(progress.elapsedSeconds)), progress.imageElapsedSeconds)}
        />
        <ProgressStat label="运行中" value={String(progress.running)} />
        <ProgressStat label="轮询次数" value={isRunning ? `${Math.max(1, progress.attempt)} 次` : '-'} />
      </div>

      <div className="mt-3 flex items-start gap-2 rounded-md border border-slate-800 bg-slate-900 px-3 py-2 text-sm text-slate-300">
        <Timer className="mt-0.5 h-4 w-4 shrink-0 text-slate-400" />
        <span>{progress.message}</span>
      </div>
    </div>
  )
}

export function ProgressStat({ label, value }: { label: string; value: string }) {
  return (
    <div className="rounded-md border border-slate-800 bg-slate-950 px-3 py-2">
      <div className="text-xs text-slate-500">{label}</div>
      <div className="mt-1 break-all text-sm text-slate-100">{value}</div>
    </div>
  )
}
