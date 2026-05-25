







import React, { useRef, useState, useEffect, useMemo, DragEvent, ClipboardEvent, ChangeEvent } from 'react';
import {
  ArrowUpDown, BadgeCheck, CheckCircle2, ChevronDown, ChevronLeft, ChevronRight,
  Copy, Download, Eye, EyeOff, Gift, Heart, History, Home, Image as ImageIcon,
  ImagePlus, KeyRound, Link2, LoaderCircle, LogIn, LogOut, Mail, Megaphone,
  Menu, Moon, PanelLeftClose, PanelLeftOpen, RefreshCcw, ReceiptText, Send,
  Search, Sparkles, ShoppingBag, Sun, Timer, Upload, UserRound, UserRoundPlus,
  ShieldCheck, Wallet, XCircle
} from 'lucide-react';
import { sizeOptions, CUSTOM_SIZE_MAX_SIDE, styleOptions, qualityOptions, outputFormatOptions, HISTORY_PAGE_SIZE, FREE_REFERENCE_IMAGE_COUNT, MAX_REFERENCE_IMAGES, GENERATION_WAITING_NOTICE } from '../constants/app';
import { getImageTaskStatusLabel, limitImageURLs, normalizeImageURLs, getPreviewRequestedCount, isPartialImageTaskStatus, getTaskStatusBadgeClass, formatTaskCost, getTaskCreditCost, getPreviewElapsedSeconds, formatElapsedTriplet, buildPreviewImageCandidates, createCenteredThumbnailSlots, getPreviewEmptyMessage, highlightSearchText, getGenerationStatusLabel, buildPreviewRequestOptions, parseTaskRequestParams, limitGenerationProgresses, progressFromImageTaskItem, normalizeElapsedSeconds, buildImageElapsedSeconds, detailToPreview, emptyPreview, toPreview, getDownloadCacheKey, trimDownloadFileCache, parsePreviewSizeState, normalizeReferenceImagesFromSnapshot } from '../utils/imageTasks';
import * as api from "../api/client";
import type { UploadImageResponse, ImageTaskItem, ImageTaskDetailResponse } from "../types/api";
import type { ReferenceImageSource, ReferenceImageItem, TaskRequestSnapshot, GenerationProgress } from "../types/app";
import { syncPrimaryReferenceImage, getReferenceImageNameFromURL, getReferenceImageExtraPointCost, isWeChatBrowser, triggerImageDownload } from '../utils/helpers';
import { RechargeConfirmDialog } from './RechargePage';
import { WechatSaveGuide, ReferenceImagesDialog, PreviewDetailDialog, TaskHistoryPanel, TaskDetailPanel, PreviewImageStage, PromptSearchOverlay, GenerationProgressPanel, GenerationProgressCard, ProgressStat } from '../components/home';
import { useBodyScrollLock } from '../hooks/useBodyScrollLock';

const CUSTOM_SIZE_MIN_PIXELS = 655360
const CUSTOM_SIZE_MAX_PIXELS = 8294400
const CUSTOM_SIZE_MAX_ASPECT_RATIO = 3
const FOUR_K_PIXELS = 3840 * 2160
const CUSTOM_SIZE_LIMIT_MESSAGE = '自定义尺寸不符合要求：宽度和高度最高 3840，总像素必须在 655,360-8,294,400 之间。'

function parseCustomDimension(value: string | number): number {
  if (typeof value === 'number') {
    return value
  }
  const trimmed = value.trim()
  if (!trimmed) {
    return Number.NaN
  }
  return Number(trimmed)
}
function resolveSelectedSize(size: string, width: string | number, height: string | number): string {
  if (size !== 'custom') {
    return size
  }

  const safeWidth = Math.min(CUSTOM_SIZE_MAX_SIDE, Math.max(1, Math.floor(parseCustomDimension(width) || 1024)))
  const safeHeight = Math.min(CUSTOM_SIZE_MAX_SIDE, Math.max(1, Math.floor(parseCustomDimension(height) || 1024)))
  return `${safeWidth}x${safeHeight}`
}

function parseSelectedSize(size: string, width: string | number, height: string | number): { width: number; height: number } | null {
  const resolved = resolveSelectedSize(size, width, height)
  const match = resolved.trim().toLowerCase().match(/^(\d+)x(\d+)$/)
  if (!match) {
    return null
  }
  return {
    width: Number(match[1]),
    height: Number(match[2]),
  }
}

function isFourKOrLargerSize(size: string, width: string | number, height: string | number): boolean {
  const parsed = parseSelectedSize(size, width, height)
  if (!parsed) {
    return false
  }
  return parsed.width * parsed.height >= FOUR_K_PIXELS
}

function validateSelectedSize(size: string, width: string | number, height: string | number): string | null {
  if (size !== 'custom') {
    return null
  }

  const normalizedWidth = Math.floor(parseCustomDimension(width))
  const normalizedHeight = Math.floor(parseCustomDimension(height))
  if (
    !Number.isFinite(normalizedWidth) ||
    !Number.isFinite(normalizedHeight) ||
    normalizedWidth <= 0 ||
    normalizedHeight <= 0 ||
    normalizedWidth > CUSTOM_SIZE_MAX_SIDE ||
    normalizedHeight > CUSTOM_SIZE_MAX_SIDE
  ) {
    return CUSTOM_SIZE_LIMIT_MESSAGE
  }

  const totalPixels = normalizedWidth * normalizedHeight
  if (totalPixels < CUSTOM_SIZE_MIN_PIXELS || totalPixels > CUSTOM_SIZE_MAX_PIXELS) {
    return CUSTOM_SIZE_LIMIT_MESSAGE
  }
  if (normalizedWidth % 16 !== 0 || normalizedHeight % 16 !== 0) {
    return '自定义尺寸的宽度和高度都必须是 16 的倍数。'
  }

  const longerSide = Math.max(normalizedWidth, normalizedHeight)
  const shorterSide = Math.min(normalizedWidth, normalizedHeight)
  if (longerSide > shorterSide * CUSTOM_SIZE_MAX_ASPECT_RATIO) {
    return '自定义尺寸宽高比不能超过 3:1，请把宽度和高度调得更接近一些。'
  }

  return null
}

export interface HomePageProps {
  historyRefreshTrigger?: {timestamp: number, resetPage?: boolean};
  onTaskCreated: (result: import('../types/api').CreateImageTaskResponse, requestSnapshot: TaskRequestSnapshot, startedAt: number) => void;
  activePage: string;
  points: import('../types/api').PointsResponse | null;
  billingSettings: import('../types/api').BillingSettingsResponse | null;
  emailNotifyAvailable: boolean;
  emailNotifyOnComplete: boolean;
  setEmailNotifyOnComplete: (val: boolean) => void;
  emailNotifyPointCost: number;
  busy: import('../types/app').BusyAction;
  clampCount: (val: number) => number;
  isAuthed: boolean;
  imageGenerationEnabled: boolean;
  setMessage: (msg: string) => void;
  setBusy: (busy: import('../types/app').BusyAction) => void;
  handleApiError: (err: unknown, fallback: string) => void;
  dashboardRunningTasks?: import('../types/api').ImageTaskItem[];
  onDashboardRefresh: () => void;
}

export function HomePage(props: HomePageProps) {
  const [generationProgresses, setGenerationProgresses] = useState<GenerationProgress[]>([]);
  const [prompt, setPrompt] = useState('')
  const pollingTaskIdsRef = useRef<Set<string>>(new Set())
  const manualPreviewSuppressedTaskIdRef = useRef<string | null>(null)
  const hasRunningGeneration = generationProgresses.some((item) => item.status === 'running' && item.taskId)
  const [size, setSize] = useState('1024x1024')
  const [style, setStyle] = useState('')
  const [count, setCount] = useState(1)
  const [quality, setQuality] = useState('auto')
  const [outputFormat, setOutputFormat] = useState('default')
  const [customWidth, setCustomWidth] = useState('1024')
  const [customHeight, setCustomHeight] = useState('1024')
  const qualityAutoOnly = isFourKOrLargerSize(size, customWidth, customHeight)
  const [referenceImageURL, setReferenceImageURL] = useState('')
  const [referenceImagesDialogOpen, setReferenceImagesDialogOpen] = useState(false)
  const [showNetworkImageInput, setShowNetworkImageInput] = useState(false)
  const [promptDragActive, setPromptDragActive] = useState(false)

  const [referenceImageName, setReferenceImageName] = useState('')
  const [referenceImageSource, setReferenceImageSource] = useState<ReferenceImageSource>('none')
  const [referenceImages, setReferenceImages] = useState<ReferenceImageItem[]>([])
  const [referenceImageUploading, setReferenceImageUploading] = useState(false)
  const referenceImageInputRef = useRef<HTMLInputElement | null>(null)

  useEffect(() => {
    if (qualityAutoOnly && quality !== 'auto') {
      setQuality('auto')
    }
  }, [quality, qualityAutoOnly])

  async function uploadLocalReferenceImage(file?: File) {
    if (!file) {
      return
    }
    if (!['image/jpeg', 'image/png', 'image/webp', 'image/gif'].includes(file.type)) {
      props.setMessage('仅支持上传 JPEG、PNG、WebP 或 GIF 图片。')
      return
    }

    try {
      setReferenceImageUploading(true)
      props.setBusy('upload-reference')
      if (referenceImages.length >= MAX_REFERENCE_IMAGES) {
        props.setMessage(`最多只能添加 ${MAX_REFERENCE_IMAGES} 张参考图片。`)
        return
      }
      const result: UploadImageResponse = await api.uploadReferenceImage(file)
      const nextImage: ReferenceImageItem = {
        url: result.imageUrl,
        name: result.fileName || file.name,
        source: 'local',
      }
      setReferenceImages((current) => syncPrimaryReferenceImage([...current, nextImage]))
      setReferenceImageURL(nextImage.url)
      setReferenceImageName(nextImage.name)
      setReferenceImageSource('local')
      setShowNetworkImageInput(false)
    } catch (error) {
      props.handleApiError(error, '上传图片失败')
    } finally {
      setReferenceImageUploading(false)
      props.setBusy(null)
    }
    }

  async function handleLocalReferenceImageChange(event: ChangeEvent<HTMLInputElement>) {
    const files = Array.from(event.target.files ?? [])
    event.target.value = ''
    for (const file of files.slice(0, MAX_REFERENCE_IMAGES - referenceImages.length)) {
      await uploadLocalReferenceImage(file)
    }
    if (files.length > MAX_REFERENCE_IMAGES - referenceImages.length) {
      props.setMessage(`最多只能添加 ${MAX_REFERENCE_IMAGES} 张参考图片。`)
    }
    }

  function handleNetworkReferenceImageInput(value: string) {
    setReferenceImageURL(value)
    setReferenceImageSource(value.trim() ? 'network' : 'none')
    }

  function handleAddNetworkReferenceImage() {
    const trimmed = (referenceImageURL || '').trim()
    if (!trimmed) {
      props.setMessage('请输入网络图片链接。')
      return
    }
    if (referenceImages.length >= MAX_REFERENCE_IMAGES) {
      props.setMessage(`最多只能添加 ${MAX_REFERENCE_IMAGES} 张参考图片。`)
      return
    }
    const nextImage: ReferenceImageItem = {
      url: trimmed,
      name: getReferenceImageNameFromURL(trimmed) || '网络图片',
      source: 'network',
    }
    setReferenceImages((current) => syncPrimaryReferenceImage([...current, nextImage]))
    setReferenceImageName(nextImage.name)
    setReferenceImageSource('network')
    setReferenceImageURL('')
    setShowNetworkImageInput(false)
    }

  function handleDeleteReferenceImage(index: number) {
    setReferenceImages((current) => {
      const next = syncPrimaryReferenceImage(current.filter((_, itemIndex: number) => itemIndex !== index))
      const first = next[0]
      setReferenceImageURL(first?.url || '')
      setReferenceImageName(first?.name || '')
      setReferenceImageSource(first?.source || 'none')
      if (!next.length) {
        setReferenceImagesDialogOpen(false)
      }
      return next
    })
    }

  function handlePromptDrag(event: DragEvent<HTMLTextAreaElement>) {
    if (!Array.from(event.dataTransfer.types).includes('Files')) {
      return
    }
    event.preventDefault()
    event.dataTransfer.dropEffect = 'copy'
    setPromptDragActive(true)
    }

  function handlePromptDragLeave(event: DragEvent<HTMLTextAreaElement>) {
    if (event.currentTarget.contains(event.relatedTarget as Node | null)) {
      return
    }
    setPromptDragActive(false)
    }

  async function handlePromptDrop(event: DragEvent<HTMLTextAreaElement>) {
    event.preventDefault()
    setPromptDragActive(false)
    const files = Array.from(event.dataTransfer.files).filter((item) => item.type.startsWith('image/'))
    if (!files.length) {
      props.setMessage('请拖入 JPEG、PNG、WebP 或 GIF 图片。')
      return
    }
    for (const file of files.slice(0, MAX_REFERENCE_IMAGES - referenceImages.length)) {
      await uploadLocalReferenceImage(file)
    }
    }

  async function handlePromptPaste(event: ClipboardEvent<HTMLTextAreaElement>) {
    const files = Array.from(event.clipboardData.files).filter((item) => item.type.startsWith('image/'))
    if (!files.length) {
      return
    }
    event.preventDefault()
    for (const file of files.slice(0, MAX_REFERENCE_IMAGES - referenceImages.length)) {
      await uploadLocalReferenceImage(file)
    }
    }


      const [generateConfirmOpen, setGenerateConfirmOpen] = useState(false)
  const imageGeneratePointCost = Math.max(0, Math.floor(Number(props.billingSettings?.imageGeneratePointCost ?? 20)))

  
  const [preview, setPreview] = useState<import('../types/api').ImageTaskDetailResponse>(emptyPreview)
  const [previewImageIndex, setPreviewImageIndex] = useState(0)
  const [previewDetailOpen, setPreviewDetailOpen] = useState(false)
  const [wechatSaveGuide, setWechatSaveGuide] = useState({
    open: false,
    imageUrl: '',
    title: '',
  })
  useBodyScrollLock(generateConfirmOpen || referenceImagesDialogOpen || previewDetailOpen || wechatSaveGuide.open)
  const downloadFileCacheRef = useRef<Map<string, import('../api/client').DownloadedFile>>(new Map())
  const downloadPendingRef = useRef<Map<string, Promise<import('../api/client').DownloadedFile>>>(new Map())

  const previewImageURLs = limitImageURLs(normalizeImageURLs(preview.imageUrls, preview.imageUrl), preview.requestedCount)
  const previewThumbnailURLs = limitImageURLs(
    normalizeImageURLs(preview.thumbnailUrls, preview.thumbnailUrl),
    preview.requestedCount,
  )

  useEffect(() => {
    if (!props.isAuthed) {
      setPreview(emptyPreview)
      setPreviewImageIndex(0)
      setPreviewDetailOpen(false)
      downloadFileCacheRef.current.clear()
    }
  }, [props.isAuthed])
useEffect(() => {
    const imageURLs = normalizeImageURLs(preview.imageUrls, preview.imageUrl)
    if (!props.isAuthed || isWeChatBrowser() || preview.status !== 'success' || !preview.taskId || preview.taskId === '-') {
      return
    }

    imageURLs.forEach((_, index: number) => {
      void prefetchImageDownload(preview.taskId, index)
    })
    // eslint-disable-next-line react-hooks/exhaustive-deps
    }, [props.isAuthed, preview.taskId, preview.status, preview.imageUrl, preview.imageUrls])

useEffect(() => {
    if (previewImageURLs.length === 0) {
      if (previewImageIndex !== 0) {
        setPreviewImageIndex(0)
      }
      return
    }

    if (previewImageIndex >= previewImageURLs.length) {
      setPreviewImageIndex(previewImageURLs.length - 1)
    }
    }, [previewImageIndex, previewImageURLs.length])

function handleSelectTask(item: ImageTaskItem) {
    if (preview.taskId === item.id) {
      manualPreviewSuppressedTaskIdRef.current = item.id
      setPreview(emptyPreview)
      setPreviewImageIndex(0)
      setPreviewDetailOpen(false)
      return
    }
    manualPreviewSuppressedTaskIdRef.current = null
    const nextPreview = toPreview(item)
    setPreview(nextPreview)
    setPreviewImageIndex(0)
    if (item.id && item.id !== '-') {
      void loadPreviewTaskDetail(item.id)
    }
    }

function handlePreviousPreviewImage() {
    setPreviewImageIndex((current) => {
      const total = limitImageURLs(normalizeImageURLs(preview.imageUrls, preview.imageUrl), preview.requestedCount).length
      if (total <= 1) {
        return current
      }
      return (current - 1 + total) % total
    })
    }

function handleNextPreviewImage() {
    setPreviewImageIndex((current) => {
      const total = limitImageURLs(normalizeImageURLs(preview.imageUrls, preview.imageUrl), preview.requestedCount).length
      if (total <= 1) {
        return current
      }
      return (current + 1) % total
    })
    }

async function handleSaveCurrentImage() {
    const imageURLs = limitImageURLs(normalizeImageURLs(preview.imageUrls, preview.imageUrl), preview.requestedCount)
    const imageUrl = imageURLs[previewImageIndex] ?? imageURLs[0]
    if (!imageUrl) {
      props.setMessage('当前没有可保存的图片。')
      return
    }
    if (isWeChatBrowser()) {
      setWechatSaveGuide({
        open: true,
        imageUrl,
        title: preview.prompt ? `第 ${previewImageIndex + 1} 张图片` : '当前图片',
      })
      props.setMessage('微信内请长按图片保存。')
      return
    }
    if (!preview.taskId || preview.taskId === '-') {
      props.setMessage('当前图片缺少任务编号，暂时无法直接下载。')
      return
    }

    try {
      const cacheKey = getDownloadCacheKey(preview.taskId, previewImageIndex)
      const cached = downloadFileCacheRef.current.get(cacheKey)
      if (cached) {
        triggerImageDownload(cached.blob, cached.fileName)
        return
      }

      props.setMessage('正在准备下载，请稍等。')
      const downloaded = await loadImageDownload(preview.taskId, previewImageIndex)
      triggerImageDownload(downloaded.blob, downloaded.fileName)
    } catch (error) {
      props.handleApiError(error, '保存当前图片失败')
    }
    }

async function prefetchImageDownload(taskId: string, imageIndex: number) {
    const cacheKey = getDownloadCacheKey(taskId, imageIndex)
    if (downloadFileCacheRef.current.has(cacheKey) || downloadPendingRef.current.has(cacheKey)) {
      return
    }

    try {
      await loadImageDownload(taskId, imageIndex)
    } catch (error) {
      console.warn('prefetch image download failed', error)
    }
    }

async function loadImageDownload(taskId: string, imageIndex: number): Promise<api.DownloadedFile> {
    const cacheKey = getDownloadCacheKey(taskId, imageIndex)
    const cached = downloadFileCacheRef.current.get(cacheKey)
    if (cached) {
      return cached
    }

    const pending = downloadPendingRef.current.get(cacheKey)
    if (pending) {
      return pending
    }

    const pendingDownload = api.downloadImageTaskImage(taskId, imageIndex)
      .then((downloaded) => {
        downloadFileCacheRef.current.set(cacheKey, downloaded)
        trimDownloadFileCache(downloadFileCacheRef.current)
        return downloaded
      })
      .finally(() => {
        downloadPendingRef.current.delete(cacheKey)
      })

    downloadPendingRef.current.set(cacheKey, pendingDownload)
    return pendingDownload
    }

async function handleOpenPreviewDetail() {
    if (!preview.taskId || preview.taskId === '-') {
      props.setMessage('当前展示区还没有提示词详情。')
      return
    }

    const detail = await loadPreviewTaskDetail(preview.taskId)
    if (!detail) {
      return
    }
    setPreviewDetailOpen(true)
    }

  async function loadPreviewTaskDetail(taskId: string): Promise<ImageTaskDetailResponse | null> {
    if (!taskId || taskId === '-') {
      return null
    }
    if (manualPreviewSuppressedTaskIdRef.current === taskId) {
      return null
    }

    try {
      const detail = await api.getImageTask(taskId)
      const nextPreview = detailToPreview(detail)
      if (manualPreviewSuppressedTaskIdRef.current === taskId) {
        return null
      }
      setPreview((current) => (current.taskId === taskId ? nextPreview : current))
      return nextPreview
    } catch (error) {
      props.handleApiError(error, '任务详情加载失败')
      return null
    }
    }

function handleApplyPreviewDetail() {
    manualPreviewSuppressedTaskIdRef.current = null
    const parsed = parseTaskRequestParams(preview.requestParams)
    const promptText = parsed?.prompt?.trim() || preview.prompt.trim()

    setPrompt(promptText)
    if (typeof parsed?.style === 'string') {
      setStyle(parsed.style)
    }
    if (typeof parsed?.count === 'number' || typeof parsed?.count === 'string') {
      setCount(props.clampCount(Number(parsed.count)))
    }
    if (typeof parsed?.quality === 'string') {
      setQuality(parsed.quality)
    }
    if (typeof parsed?.outputFormat === 'string') {
      setOutputFormat(parsed.outputFormat)
    }

    const sizeState = parsePreviewSizeState(parsed?.size)
    if (sizeState) {
      setSize(sizeState.size)
      if (typeof sizeState.customWidth === 'number' && typeof sizeState.customHeight === 'number') {
          setCustomWidth(String(sizeState.customWidth))
          setCustomHeight(String(sizeState.customHeight))
      }
    }

    const parsedReferenceImages = normalizeReferenceImagesFromSnapshot(parsed)
    const firstReferenceImage = parsedReferenceImages[0]
    setReferenceImages(parsedReferenceImages)
    setReferenceImageURL(firstReferenceImage?.url || '')
    setReferenceImageName(firstReferenceImage?.name || '')
    setReferenceImageSource(firstReferenceImage?.source || 'none')
    setShowNetworkImageInput(false)
            setPreviewDetailOpen(false)
    }


  
  function handleClearPromptOptions() {
    const confirmed = window.confirm('是否清除当前提示词以及选项信息？')
    if (!confirmed) {
      return
    }

    setPrompt('')
    setSize('1024x1024')
    setStyle('')
    setCount(1)
    setQuality('auto')
    setOutputFormat('default')
    setCustomWidth('1024')
    setCustomHeight('1024')
    setReferenceImageURL('')
    setReferenceImageName('')
    setReferenceImageSource('none')
    setShowNetworkImageInput(false)
    if (referenceImageInputRef?.current) {
      referenceImageInputRef.current.value = ''
    }
  }

  
  const [history, setHistory] = useState<ImageTaskItem[]>([])
  const [historyPage, setHistoryPage] = useState(1)
  const [historyTotal, setHistoryTotal] = useState(0)

  async function loadHistoryPage(page: number, options: { showBusy?: boolean } = {}) {
    const { showBusy = false } = options
    try {
      if (showBusy) {
        props.setBusy('refresh')
      }
      const pageResult = await api.listImageTasks(page, HISTORY_PAGE_SIZE)
      setHistory(pageResult.items)
      setHistoryPage(pageResult.page)
      setHistoryTotal(pageResult.total)
    } catch (error) {
      props.handleApiError(error, '历史记录加载失败')
    } finally {
      if (showBusy) {
        props.setBusy(null)
      }
    }
  }

  useEffect(() => {
    if (props.isAuthed) {
      if (props.historyRefreshTrigger?.resetPage) {
        setHistoryPage(1)
        void loadHistoryPage(1, { showBusy: false })
      } else {
        void loadHistoryPage(historyPage, { showBusy: false })
      }
    } else {
      setHistory([])
      setHistoryPage(1)
      setHistoryTotal(0)
    }
  }, [props.isAuthed, props.historyRefreshTrigger])

  const estimatedGenerationPointCost = useMemo(() => {
    const taskCount = props.clampCount(Number(count) || 1)
    const availableCredits = Math.max(0, Math.floor(Number(props.points?.totalCredits ?? 0)))
    const pointPaidCount = Math.max(0, taskCount - Math.min(taskCount, availableCredits))
    const referenceExtraPointCost = getReferenceImageExtraPointCost(referenceImages.length)
    const emailPointCost = props.emailNotifyAvailable && props.emailNotifyOnComplete ? props.emailNotifyPointCost : 0
    return pointPaidCount * imageGeneratePointCost + referenceExtraPointCost + emailPointCost
  }, [
    count,
    referenceImages.length,
    props.emailNotifyAvailable,
    props.emailNotifyOnComplete,
    props.emailNotifyPointCost,
    imageGeneratePointCost,
    props.points?.totalCredits,
  ])

  function handleOpenCreateTaskConfirm() {
    if (props.busy === 'create-task') {
      return
    }
    if (!props.isAuthed) {
      props.setMessage('请先登录，再提交生图任务。')
      return
    }
    if (!props.imageGenerationEnabled) {
      return
    }
    if (!(prompt || '').trim()) {
      props.setMessage('提示词不能为空。')
      return
    }
    if (referenceImageUploading || props.busy === 'upload-reference') {
      props.setMessage('参考图还在上传，请等上传完成后再生成。大图会稍慢一点。')
      return
    }
    setGenerateConfirmOpen(true)
  }

  function handleCancelCreateTaskConfirm() {
    setGenerateConfirmOpen(false)
  }

  function handleConfirmCreateTask() {
    setGenerateConfirmOpen(false)
    void handleCreateTask()
  }

  async function handleCreateTask() {
    if (props.busy === 'create-task') {
      return
    }
    if (!props.isAuthed) {
      props.setMessage('请先登录，再提交生图任务。')
      return
    }

    if (!props.imageGenerationEnabled) {
      return
    }

    if (!(prompt || '').trim()) {
      props.setMessage('提示词不能为空。')
      return
    }
    if (referenceImageUploading || props.busy === 'upload-reference') {
      props.setMessage('参考图还在上传，请等上传完成后再生成。大图会稍慢一点。')
      return
    }

    const promptText = (prompt || '').trim()
    const sizeValidationMessage = validateSelectedSize(size, customWidth, customHeight)
    if (sizeValidationMessage) {
      props.setMessage(sizeValidationMessage)
      return
    }

    const requestSize = resolveSelectedSize(size, customWidth, customHeight)
    const requestQuality = isFourKOrLargerSize(size, customWidth, customHeight) ? 'auto' : quality
    const taskCount = props.clampCount(count)
    const selectedReferenceImages = referenceImages || []
    const requestSnapshot = {
      prompt: promptText,
      size: requestSize,
      style: style,
      count: taskCount,
      quality: requestQuality,
      outputFormat: outputFormat,
      referenceImageUrl: selectedReferenceImages[0]?.url || (referenceImageURL || '').trim() || undefined,
      referenceImageUrls: selectedReferenceImages.map((item) => item.url).filter(Boolean),
      referenceImages: selectedReferenceImages,
      emailNotifyOnComplete: props.emailNotifyAvailable && props.emailNotifyOnComplete,
    } satisfies TaskRequestSnapshot
    const startedAt = Date.now()

    try {
      props.setBusy('create-task')
      const result = await api.createImageTask(requestSnapshot)
      props.onTaskCreated(result, requestSnapshot, startedAt)

      let taskCount = requestSnapshot.count || 1;
      let requestedCount = Number(result.requestedCount ?? taskCount);

            upsertGenerationProgress(progressFromImageTaskItem({
              id: result.taskId,
              prompt: promptText,
              requestedCount,
              status: 'generating',
              createdAt: new Date().toISOString(),
              imageUrls: [],
              thumbnailUrls: [],
              imageUrl: '',
              thumbnailUrl: '',
              imageElapsedSeconds: []
            } as unknown as import('../types/api').ImageTaskItem))
      const resultImageURLs = limitImageURLs(normalizeImageURLs(result.imageUrls, result.imageUrl), requestedCount)
      const now = new Date().toISOString()
      setPreview(detailToPreview({
        taskId: result.taskId,
        requestedCount,
        prompt: promptText,
        imageUrl: result.imageUrl,
        imageUrls: resultImageURLs,
        costPoints: Number(result.costPoints ?? 0),
        costCredits: Number(result.costCredits ?? taskCount),
        freeCreditsCost: Number(result.freeCreditsCost ?? 0),
        paidCreditsCost: Number(result.paidCreditsCost ?? 0),
        creditSource: result.creditSource || 'free',
        emailNotifyEnabled: Boolean(result.emailNotifyEnabled),
        emailNotifyCostPoints: Number(result.emailNotifyCostPoints ?? 0),
        status: result.status || 'generating',
        requestParams: '',
        errorMessage: result.errorMessage,
        createdAt: now,
        updatedAt: now,
      }))
      setPreviewImageIndex(0)
    } catch (error) {
      props.handleApiError(error, '提交失败')
    } finally {
      props.setBusy(null)
    }
  }
      useEffect(() => {
        if (!props.isAuthed) {
          pollingTaskIdsRef.current.clear()
          setGenerationProgresses([])
        }
      }, [props.isAuthed])

      useEffect(() => {
        if (props.dashboardRunningTasks && props.dashboardRunningTasks.length > 0) {
          setGenerationProgresses((current) => {
            const currentIds = new Set(current.map((item) => item.taskId))
            const next = [...current]
            for (const task of [...props.dashboardRunningTasks!].reverse()) {
              if (currentIds.has(task.id)) continue
              next.push(progressFromImageTaskItem(task))
            }
            return next
          })
        }
      }, [props.dashboardRunningTasks])

      useEffect(() => {
        if (!hasRunningGeneration) return
        const timer = window.setInterval(() => {
          setGenerationProgresses((current) =>
            current.map((item) => {
              if (item.status !== 'running' || !item.startedAt) return item
              const elapsedSeconds = Math.max(0, Math.floor((Date.now() - item.startedAt) / 1000))
              return {
                ...item,
                elapsedSeconds,
                imageElapsedSeconds: buildImageElapsedSeconds(item.total, elapsedSeconds, item.imageElapsedSeconds, item.completed, item.status),
                running: Math.max(0, item.total - item.completed),
              }
            })
          )
        }, 1000)
        return () => window.clearInterval(timer)
      }, [hasRunningGeneration])

      useEffect(() => {
        for (const progress of generationProgresses) {
          if (!progress.taskId || progress.status !== 'running' || pollingTaskIdsRef.current.has(progress.taskId)) continue
          pollingTaskIdsRef.current.add(progress.taskId)
          void pollGenerationTask(progress)
        }
      }, [generationProgresses])

      useEffect(() => {
        return () => {
          pollingTaskIdsRef.current.clear()
        }
      }, [])

      function upsertGenerationProgress(progress: GenerationProgress) {
        if (!progress.taskId) return
        setGenerationProgresses((current) => limitGenerationProgresses([progress, ...current.filter((item) => item.taskId !== progress.taskId)]))
      }

      function updateGenerationProgress(taskId: string, updater: (current: GenerationProgress) => GenerationProgress) {
        setGenerationProgresses((current) => current.map((item) => (item.taskId === taskId ? updater(item) : item)))
      }

      async function pollGenerationTask(progress: GenerationProgress) {
        const taskId = progress.taskId
        if (!taskId) return

        const startedAt = progress.startedAt ?? Date.now()
        let attempt = Math.max(0, progress.attempt - 1)
        const sleep = (delayMs: number) => new Promise<void>((resolve) => { window.setTimeout(resolve, delayMs) })

        try {
          while (pollingTaskIdsRef.current.has(taskId)) {
            attempt += 1
            try {
              const detail = await api.getImageTask(taskId)
              if (!pollingTaskIdsRef.current.has(taskId)) return
              const previewSuppressed = manualPreviewSuppressedTaskIdRef.current === taskId

              const detailCount = getPreviewRequestedCount(detail, 1)
              const imageURLs = limitImageURLs(normalizeImageURLs(detail.imageUrls, detail.imageUrl), detail.requestedCount || detailCount)
              const persistedElapsedSeconds = normalizeElapsedSeconds(detail.imageElapsedSeconds)
              const elapsedSeconds = Math.max(0, Math.floor((Date.now() - startedAt) / 1000))
              const isFinal = detail.status === 'success' || detail.status === 'failed'
              const finalCount = Math.max(1, imageURLs.length || detailCount)
              const nextMessage = detail.status === 'failed' ? detail.errorMessage ?? '生成失败' : imageURLs.length > 0 ? `已拿到 ${imageURLs.length} 张图片` : '任务已完成'

              // LOCAL Update Preview
              if (!previewSuppressed) {
                setPreview((current) => {
                  if (current.taskId !== taskId && current.taskId !== '-' && current.taskId) return current
                  return detailToPreview(detail)
                })
              }
              
              // LOCAL Update History
              setHistory(current => current.map(item => {
                if (item.id === taskId) {
                  return {
                    ...item,
                    status: detail.status as import('../types/app').GenerationStatus,
                    imageUrl: detail.imageUrl,
                    imageUrls: imageURLs,
                    imageElapsedSeconds: persistedElapsedSeconds,
                    thumbnailUrl: detail.thumbnailUrl,
                    thumbnailUrls: detail.thumbnailUrls,
                    errorMessage: detail.errorMessage,
                    updatedAt: detail.updatedAt,
                  }
                }
                return item
              }));

              if (isFinal) {
                updateGenerationProgress(taskId, (current) => ({
                  ...current,
                  prompt: detail.prompt,
                  status: detail.status === 'failed' ? 'failed' : 'success',
                  elapsedSeconds,
                  imageElapsedSeconds: buildImageElapsedSeconds(finalCount, elapsedSeconds, persistedElapsedSeconds.length > 0 ? persistedElapsedSeconds : current.imageElapsedSeconds, finalCount, detail.status === 'failed' ? 'failed' : 'success'),
                  total: finalCount,
                  completed: finalCount,
                  success: detail.status === 'failed' ? 0 : finalCount,
                  running: 0,
                  attempt,
                  message: nextMessage,
                }))
                if (detail.status === 'failed') {
                  props.setMessage(`生图失败：${detail.errorMessage ?? '未知错误'}`)
                }
                props.onDashboardRefresh()
                return
              }

              updateGenerationProgress(taskId, (current) => ({
                ...current,
                prompt: detail.prompt,
                elapsedSeconds,
                imageElapsedSeconds: buildImageElapsedSeconds(Math.max(detailCount, current.total, imageURLs.length || 1), elapsedSeconds, persistedElapsedSeconds.length > 0 ? persistedElapsedSeconds : current.imageElapsedSeconds, imageURLs.length, 'running'),
                total: Math.max(detailCount, current.total, imageURLs.length || 1),
                completed: imageURLs.length,
                success: 0,
                running: Math.max(0, Math.max(detailCount, current.total, imageURLs.length || 1) - imageURLs.length),
                attempt,
                message: imageURLs.length > 0 ? nextMessage : '任务正在生成中',
              }))
            } catch (error: unknown) {
              if (!pollingTaskIdsRef.current.has(taskId)) return
              
              if (error && typeof error === 'object' && 'status' in error && (error as { status?: number }).status === 401) {
                pollingTaskIdsRef.current.clear()
                return
              }
              
              const errorMessage = error instanceof Error ? error.message : '轮询任务状态失败'
              updateGenerationProgress(taskId, (current) => ({
                ...current,
                elapsedSeconds: Math.max(0, Math.floor((Date.now() - startedAt) / 1000)),
                imageElapsedSeconds: buildImageElapsedSeconds(current.total, Math.max(0, Math.floor((Date.now() - startedAt) / 1000)), current.imageElapsedSeconds, current.completed, current.status),
                attempt,
                message: `轮询状态失败：${errorMessage}`,
              }))
            }

            await sleep(2000)
          }
        } finally {
          pollingTaskIdsRef.current.delete(taskId)
        }
      }


  return (
    <div className={props.activePage === 'home' ? 'block' : 'hidden'}>
              <div className="space-y-4">
                <div className="grid min-w-0 grid-cols-1 gap-4 xl:grid-cols-[360px_minmax(0,1fr)] 2xl:grid-cols-[360px_minmax(0,1fr)_340px]">
                  <section className="min-w-0 overflow-visible rounded-lg border border-slate-800 bg-slate-900/60 p-3 shadow-panel sm:p-4">
                    <div className="flex items-center justify-between">
                      <div>
                        <h2 className="text-lg font-medium text-slate-50">提示词</h2>
                        <p className="mt-1 text-sm text-slate-400">在这里输入提示词，再选择生成参数。</p>
                      </div>
                      <button
                        type="button"
                        onClick={handleClearPromptOptions}
                        className="inline-flex items-center gap-1.5 rounded-md border border-slate-700 bg-slate-950 px-2.5 py-1.5 text-xs text-slate-300 transition hover:border-rose-400 hover:text-rose-200"
                        title="清除当前提示词和生成选项"
                      >
                        <XCircle className="h-3.5 w-3.5" />
                        清除
                      </button>
                    </div>

                    <div className="mt-4 grid grid-cols-2 gap-2 text-sm">
                      <div className="rounded-md border border-slate-800 bg-slate-950 px-3 py-2 text-slate-300">
                        <div className="text-xs text-slate-500">当前积分</div>
                        <div className="mt-1 text-base font-semibold text-slate-100">{props.points ? props.points.balance : 0}</div>
                      </div>
                      <div className="rounded-md border border-slate-800 bg-slate-950 px-3 py-2 text-slate-300">
                        <div className="text-xs text-slate-500">可用福利次数</div>
                        <div className="mt-1 text-base font-semibold text-slate-100">
                          {props.points ? props.points.totalCredits : 0}
                        </div>
                      </div>
                      <div className="col-span-2 rounded-md border border-slate-800 bg-slate-950 px-3 py-2 text-slate-300">
                        <div className="text-sm font-medium text-slate-100">
                          今日生图消耗 {props.billingSettings?.imageGeneratePointCost ?? 20} 积分
                        </div>
                      </div>
                    </div>

                    {props.emailNotifyAvailable ? (
                      <label className="mt-3 flex items-start gap-3 rounded-md border border-slate-800 bg-slate-950 px-3 py-2 text-sm text-slate-300">
                        <input
                          type="checkbox"
                          checked={props.emailNotifyOnComplete}
                          onChange={(event) => props.setEmailNotifyOnComplete(event.target.checked as boolean)}
                          className="mt-1 h-4 w-4 rounded border-slate-600 bg-slate-900 text-violet-500 focus:ring-violet-400"
                        />
                        <span className="min-w-0 leading-6">
                          额外消耗 {props.emailNotifyPointCost} 积分，任务完成后发送邮件至账号邮箱，失败将退还积分。
                        </span>
                      </label>
                    ) : null}

                    <textarea
                      value={prompt}
                      onChange={(event) => setPrompt(event.target.value)}
                      maxLength={8192}
                      onDragEnter={handlePromptDrag}
                      onDragOver={handlePromptDrag}
                      onDragLeave={handlePromptDragLeave}
                      onDrop={handlePromptDrop}
                      onPaste={handlePromptPaste}
                      className={`mt-4 min-h-32 w-full resize-y rounded-md border px-3 py-3 text-sm leading-6 text-slate-100 outline-none transition placeholder:text-slate-500 focus:border-violet-400 ${
                        promptDragActive
                          ? 'border-violet-400 bg-violet-950/40 ring-2 ring-violet-400/40'
                          : 'border-slate-700 bg-slate-900'
                      }`}
                      placeholder={`请输入提示词，或将本地图片拖入 / 粘贴到此框中\n可免费使用 ${FREE_REFERENCE_IMAGE_COUNT} 张参考图，后续每多 1 张额外消耗 1 积分，最多可添加 ${MAX_REFERENCE_IMAGES} 张。`}
                    />

                    <div className="mt-3 grid grid-cols-2 gap-2">
                      <button
                        type="button"
                        onClick={() => referenceImageInputRef.current?.click()}
                        disabled={referenceImageUploading || props.busy === 'upload-reference'}
                        className="inline-flex items-center justify-center gap-2 rounded-md border border-slate-700 bg-slate-900 px-3 py-2 text-sm text-slate-200 transition hover:border-violet-400 hover:text-white disabled:cursor-not-allowed disabled:opacity-60"
                      >
                        {referenceImageUploading || props.busy === 'upload-reference' ? (
                          <LoaderCircle className="h-4 w-4 animate-spin" />
                        ) : (
                          <ImagePlus className="h-4 w-4" />
                        )}
                        添加本地图片
                      </button>
                      <button
                        type="button"
                        onClick={() => setShowNetworkImageInput((current) => !current)}
                        className="inline-flex items-center justify-center gap-2 rounded-md border border-slate-700 bg-slate-900 px-3 py-2 text-sm text-slate-200 transition hover:border-violet-400 hover:text-white"
                      >
                        <Link2 className="h-4 w-4" />
                        添加网络图片
                      </button>
                    </div>

                    <input
                      ref={referenceImageInputRef}
                      type="file"
                      accept="image/jpeg,image/png,image/webp,image/gif"
                      multiple
                      className="hidden"
                      onChange={handleLocalReferenceImageChange}
                    />

                    {showNetworkImageInput ? (
                      <label className="mt-3 block">
                        <span className="mb-2 block text-sm text-slate-400">网络图片链接</span>
                        <div className="flex overflow-hidden rounded-md border border-slate-700 bg-slate-900 focus-within:border-violet-400">
                          <input
                            value={referenceImageURL}
                            onChange={(event) => handleNetworkReferenceImageInput(event.target.value)}
                            onKeyDown={(event) => {
                              if (event.key === 'Enter') {
                                event.preventDefault()
                                handleAddNetworkReferenceImage()
                              }
                            }}
                            className="h-11 min-w-0 flex-1 bg-transparent px-3 text-sm text-slate-100 outline-none placeholder:text-slate-500"
                            placeholder="请输入公网图片链接"
                          />
                          <button
                            type="button"
                            onClick={handleAddNetworkReferenceImage}
                            className="h-11 shrink-0 border-l border-slate-700 px-4 text-sm font-medium text-violet-200 transition hover:bg-violet-500/10 hover:text-violet-100"
                          >
                            确定
                          </button>
                        </div>
                      </label>
                    ) : null}

                    <div className="mt-3 space-y-2">
                      {referenceImages.length ? (
                        <>
                          <button
                            type="button"
                            onClick={() => setReferenceImagesDialogOpen(true)}
                            className="inline-flex w-full items-center justify-center gap-2 rounded-lg border border-slate-700 bg-slate-900 px-3 py-2.5 text-sm font-medium text-slate-100 transition hover:border-violet-400 hover:bg-slate-800"
                          >
                            查看已添加的 {referenceImages.length} 张图片（最多 {MAX_REFERENCE_IMAGES} 张）
                          </button>
                          <div className="rounded-lg border border-slate-800 bg-slate-950 px-3 py-2 text-xs text-slate-500">
                            参考图额外消耗 {getReferenceImageExtraPointCost(referenceImages.length)} 积分
                          </div>
                        </>
                      ) : (
                        <div className="rounded-lg border border-slate-800 bg-slate-950 p-3 text-sm text-slate-500">未添加参考图片。</div>
                      )}
                    </div>

                    <div className="mt-4 grid grid-cols-2 gap-2">
                      <label className="block">
                        <span className="mb-2 block text-sm text-slate-400">尺寸</span>
                        <select
                          value={size}
                          onChange={(event) => setSize(event.target.value)}
                          className="h-11 w-full rounded-md border border-slate-700 bg-slate-900 px-3 text-sm text-slate-100 outline-none transition focus:border-violet-400"
                        >
                          {sizeOptions.map((option) => (
                            <option key={option.value} value={option.value}>
                              {option.label}
                            </option>
                          ))}
                        </select>
                      </label>

                      <label className="block">
                        <span className="mb-2 block text-sm text-slate-400">张数</span>
                        <select
                          value={count}
                          onChange={(event) => setCount(Number(event.target.value))}
                          className="h-11 w-full rounded-md border border-slate-700 bg-slate-900 px-3 text-sm text-slate-100 outline-none transition focus:border-violet-400"
                        >
                          {[1, 2, 3, 4, 5, 6].map((value) => (
                            <option key={value} value={value}>
                              {value} 张
                            </option>
                          ))}
                        </select>
                      </label>
                    </div>

                    {size === 'custom' ? (
                      <div className="mt-3 grid grid-cols-2 gap-2">
                        <label className="block">
                          <span className="mb-2 block text-sm text-slate-400">自定义宽度</span>
                          <input
                            type="number"
                            min={1}
                            max={CUSTOM_SIZE_MAX_SIDE}
                            step={64}
                            value={customWidth}
                            onChange={(event) => setCustomWidth(event.target.value)}
                            placeholder="1024"
                            className="h-11 w-full rounded-md border border-slate-700 bg-slate-900 px-3 text-sm text-slate-100 outline-none transition focus:border-violet-400"
                          />
                        </label>
                        <label className="block">
                          <span className="mb-2 block text-sm text-slate-400">自定义高度</span>
                          <input
                            type="number"
                            min={1}
                            max={CUSTOM_SIZE_MAX_SIDE}
                            step={64}
                            value={customHeight}
                            onChange={(event) => setCustomHeight(event.target.value)}
                            placeholder="1024"
                            className="h-11 w-full rounded-md border border-slate-700 bg-slate-900 px-3 text-sm text-slate-100 outline-none transition focus:border-violet-400"
                          />
                        </label>
                      </div>
                    ) : null}

                    <div className="mt-3 grid grid-cols-3 gap-2">
                      <label className="block min-w-0">
                        <span className="mb-2 block text-sm text-slate-400">风格</span>
                        <select
                          value={style}
                          onChange={(event) => setStyle(event.target.value)}
                          className="h-11 w-full min-w-0 rounded-md border border-slate-700 bg-slate-900 px-3 text-sm text-slate-100 outline-none transition focus:border-violet-400"
                        >
                          {styleOptions.map((option) => (
                            <option key={option.value || 'none'} value={option.value}>
                              {option.label}
                            </option>
                          ))}
                        </select>
                      </label>

                      <label className="block min-w-0">
                        <span className="mb-2 block text-sm text-slate-400">质量</span>
                        <select
                          value={quality}
                          onChange={(event) => setQuality(event.target.value)}
                          className="h-11 w-full min-w-0 rounded-md border border-slate-700 bg-slate-900 px-3 text-sm text-slate-100 outline-none transition focus:border-violet-400"
                        >
                          {qualityOptions.map((option) => (
                            <option key={option.value} value={option.value} disabled={qualityAutoOnly && option.value !== 'auto'}>
                              {option.value === 'auto' ? '自动，4k以上图片仅支持该选项' : option.label}
                            </option>
                          ))}
                        </select>
                      </label>

                      <label className="block min-w-0">
                        <span className="mb-2 block text-sm text-slate-400">输出格式</span>
                        <select
                          value={outputFormat}
                          onChange={(event) => setOutputFormat(event.target.value)}
                          className="h-11 w-full min-w-0 rounded-md border border-slate-700 bg-slate-900 px-3 text-sm text-slate-100 outline-none transition focus:border-violet-400"
                        >
                          {outputFormatOptions.map((option) => (
                            <option key={option.value} value={option.value}>
                              {option.label}
                            </option>
                          ))}
                        </select>
                      </label>
                    </div>

                    <div className="mt-3">
                      <button
                        type="button"
                        onClick={handleOpenCreateTaskConfirm}
                        disabled={!props.isAuthed || props.busy === 'create-task' || props.busy === 'upload-reference' || referenceImageUploading || !prompt.trim() || !props.imageGenerationEnabled}
                        className="inline-flex h-11 w-full items-center justify-center gap-2 rounded-md bg-violet-500 px-4 text-sm font-medium text-slate-950 transition hover:bg-violet-400 disabled:cursor-not-allowed disabled:opacity-60"
                      >
                        {props.busy === 'create-task' ? (
                          <LoaderCircle className="h-4 w-4 animate-spin" />
                        ) : referenceImageUploading || props.busy === 'upload-reference' ? (
                          <LoaderCircle className="h-4 w-4 animate-spin" />
                        ) : (
                          <Sparkles className="h-4 w-4" />
                        )}
                        {props.imageGenerationEnabled
                          ? referenceImageUploading || props.busy === 'upload-reference'
                            ? '参考图上传中...'
                            : `生成图片，本次预计消耗 ${estimatedGenerationPointCost} 积分`
                          : '暂时关闭，详情见公告'}
                      </button>
                    </div>
                  </section>

                  <section className="min-w-0 overflow-visible rounded-lg border border-slate-800 bg-slate-900/60 p-3 shadow-panel sm:p-4">
                    <div className="flex items-center justify-between">
                      <div>
                        <h2 className="text-lg font-medium text-slate-50">展示区</h2>
                        <p className="mt-1 text-sm text-slate-400">长按图片可选择放大预览</p>
                      </div>
                      <button
                        type="button"
                        onClick={() => void handleOpenPreviewDetail()}
                        className="inline-flex items-center gap-2 rounded-md border border-slate-700 bg-slate-950 px-3 py-2 text-sm text-slate-200 transition hover:border-violet-400 hover:text-white"
                      >
                        <Eye className="h-4 w-4" />
                        提示词详情
                      </button>
                    </div>

                    <div className="mt-4 grid min-w-0 gap-4 2xl:grid-cols-[minmax(0,1fr)_340px]">
                      <div className="min-w-0 space-y-4">
                        <PreviewImageStage
                          preview={preview}
                          imageURLs={previewImageURLs}
                          thumbnailURLs={previewThumbnailURLs}
                          activeIndex={previewImageIndex}
                          onPrevious={handlePreviousPreviewImage}
                          onNext={handleNextPreviewImage}
                          onSaveCurrent={handleSaveCurrentImage}
                          onSelectThumbnail={setPreviewImageIndex}
                          onImageLoadError={loadPreviewTaskDetail}
                        />
                      </div>

                      <div className="min-w-0 space-y-4">
                        <TaskDetailPanel
                          preview={preview}
                          progress={generationProgresses.find((item) => item.taskId === preview.taskId) ?? null}
                        />
                      </div>
                    </div>
              </section>

                  <aside className="min-w-0 space-y-4 overflow-visible xl:col-start-2 xl:row-start-2 2xl:col-start-3 2xl:row-start-1">
                    <TaskHistoryPanel
                      items={history}
                      page={historyPage}
                      pageSize={HISTORY_PAGE_SIZE}
                      total={historyTotal}
                      selectedTaskId={preview.taskId && preview.taskId !== '-' ? preview.taskId : null}
                      onSelect={handleSelectTask}
                      onPageChange={(page: number) => void loadHistoryPage(page)}
                    />
                  </aside>
                </div>
              </div>

      
      {wechatSaveGuide.open ? (
        <WechatSaveGuide
          imageUrl={wechatSaveGuide.imageUrl}
          title={wechatSaveGuide.title}
          onClose={() => setWechatSaveGuide({ open: false, imageUrl: '', title: '' })}
        />
      ) : null}
      {previewDetailOpen ? (
        <PreviewDetailDialog
          preview={preview}
          onClose={() => setPreviewDetailOpen(false)}
          onApply={handleApplyPreviewDetail}
        />
      ) : null}
      {referenceImagesDialogOpen ? (
        <ReferenceImagesDialog
          images={referenceImages}
          onDelete={handleDeleteReferenceImage}
          onClose={() => setReferenceImagesDialogOpen(false)}
        />
      ) : null}

    
      {generateConfirmOpen ? (
        <RechargeConfirmDialog
          title="确认提交生图任务"
          lines={[
            '确认是否提交生图任务？',
            `上传参考图：${referenceImages.length} 张`,
            `生成张数：${props.clampCount(Number(count) || 1)} 张`,
            `本次预计消耗：${estimatedGenerationPointCost} 积分`,
          ]}
          confirmLabel="确定"
          busy={props.busy === 'create-task'}
          onCancel={handleCancelCreateTaskConfirm}
          onConfirm={handleConfirmCreateTask}
        />
      ) : null}
    </div>
  );
}
