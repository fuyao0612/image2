import { useEffect, useMemo, useRef, useState } from 'react'
import type { ChangeEvent, ReactNode } from 'react'
import {
  BadgeCheck,
  CheckCircle2,
  ChevronLeft,
  ChevronRight,
  Copy,
  CreditCard,
  Download,
  Eye,
  EyeOff,
  Gift,
  Heart,
  History,
  Home,
  Image,
  ImagePlus,
  KeyRound,
  Link2,
  LoaderCircle,
  LogIn,
  LogOut,
  Mail,
  Menu,
  PanelLeftClose,
  PanelLeftOpen,
  RefreshCcw,
  ReceiptText,
  Send,
  Sparkles,
  Timer,
  Upload,
  UserRound,
  UserRoundPlus,
  ShieldCheck,
  Wallet,
  XCircle,
} from 'lucide-react'
import {
  APIError,
  api,
  clearStoredToken,
  getStoredToken,
  saveStoredToken,
} from '@/api/client'
import { AdminConsole } from './admin/AdminConsole'
import { TopNotice } from './components/TopNotice'
import type {
  AdminActionLogItem,
  AdminContentAuditItem,
  AdminImageTaskItem,
  AdminPackageItem,
  AdminPackagePayload,
  AdminRechargeOrderItem,
  AdminStatsResponse,
  AdminUserItem,
  BillingSettingsResponse,
  ListCreditLogsResponse,
  ListPointsLogsResponse,
  ListUserActivityResponse,
  CreditLogItem,
  AuthResponse,
  ImageTaskDetailResponse,
  ImageTaskItem,
  PackageItem,
  PointsLogItem,
  PointsResponse,
  RechargeOrderItem,
  UploadImageResponse,
  UserActivityItem,
  UserInfo,
} from '@/types/api'

type AuthMode = 'login' | 'register'
type AppPage = 'home' | 'orders' | 'profile' | 'flows' | 'recharge' | 'prompts'
type FlowLogType = 'points' | 'credits'
type FlowLogDetail =
  | { type: 'points'; item: PointsLogItem }
  | { type: 'credits'; item: CreditLogItem }
  | null
type BusyAction =
  | 'send-code'
  | 'auth'
  | 'checkin'
  | 'upload-reference'
  | 'create-task'
  | 'refresh'
  | `retry:${string}`
  | `purchase:${string}`
  | `confirm-recharge:${string}`
  | 'change-password'
  | null

type ReferenceImageSource = 'none' | 'local' | 'network'

type GenerationStatus = 'idle' | 'running' | 'success' | 'failed'

const REGISTRATION_ENABLED = String(import.meta.env.VITE_REGISTRATION_ENABLED ?? 'true').toLowerCase() !== 'false'

interface GenerationProgress {
  taskId: string | null
  prompt: string
  status: GenerationStatus
  startedAt: number | null
  elapsedSeconds: number
  total: number
  completed: number
  success: number
  running: number
  attempt: number
  backendUrl: string
  message: string
}

interface TaskRequestSnapshot {
  prompt?: string
  size?: string
  style?: string
  count?: number
  quality?: string
  outputFormat?: string
  referenceImageUrl?: string
}

interface PromptGalleryItem {
  id: string
  title: string
  prompt: string
  previewUrl: string
  author: string
  status: 'approved' | 'pending' | 'rejected'
  favorites: number
  createdAt: string
}

interface PasswordChangeForm {
  currentPassword: string
  newPassword: string
  confirmNewPassword: string
}

interface WechatSaveGuideState {
  open: boolean
  imageUrl: string
  title: string
}

function escapeXml(value: string): string {
  return value
    .replace(/&/g, '&amp;')
    .replace(/</g, '&lt;')
    .replace(/>/g, '&gt;')
    .replace(/"/g, '&quot;')
    .replace(/'/g, '&apos;')
}

function createPromptPreviewArt(options: {
  title: string
  subtitle: string
  accent: string
  accentSoft: string
  motif: 'fox' | 'product' | 'room'
}): string {
  const { title, subtitle, accent, accentSoft, motif } = options
  const label = escapeXml(title)
  const sublabel = escapeXml(subtitle)

  const motifMarkup =
    motif === 'fox'
      ? `
        <path d="M220 352c42-92 118-146 196-146s154 54 196 146" fill="none" stroke="${accentSoft}" stroke-width="8" stroke-linecap="round" opacity="0.9"/>
        <path d="M304 302l64-76 64 76 52-54 54 54v98H252v-98z" fill="${accent}" opacity="0.65"/>
        <rect x="236" y="390" width="428" height="16" rx="8" fill="${accentSoft}" opacity="0.45"/>
        <rect x="180" y="432" width="548" height="12" rx="6" fill="${accentSoft}" opacity="0.25"/>
        <rect x="160" y="454" width="580" height="10" rx="5" fill="${accentSoft}" opacity="0.16"/>
      `
      : motif === 'product'
        ? `
        <path d="M226 156h448l-40 88H266z" fill="${accentSoft}" opacity="0.34"/>
        <rect x="278" y="238" width="344" height="156" rx="22" fill="${accent}" opacity="0.70"/>
        <rect x="320" y="280" width="260" height="72" rx="18" fill="#020617" opacity="0.42"/>
        <rect x="228" y="414" width="444" height="24" rx="12" fill="${accentSoft}" opacity="0.55"/>
        <rect x="258" y="442" width="384" height="14" rx="7" fill="${accentSoft}" opacity="0.28"/>
      `
        : `
        <rect x="186" y="142" width="528" height="304" rx="28" fill="${accentSoft}" opacity="0.14"/>
        <rect x="236" y="190" width="180" height="148" rx="16" fill="${accent}" opacity="0.42"/>
        <rect x="430" y="190" width="224" height="148" rx="16" fill="${accent}" opacity="0.20"/>
        <rect x="272" y="356" width="300" height="24" rx="12" fill="${accentSoft}" opacity="0.55"/>
        <rect x="320" y="386" width="204" height="18" rx="9" fill="${accentSoft}" opacity="0.28"/>
      `

  const svg = `
    <svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 900 600" role="img" aria-label="${label}">
      <rect width="900" height="600" fill="#020617"/>
      <rect width="900" height="600" fill="${accentSoft}" opacity="0.12"/>
      <rect x="32" y="32" width="836" height="536" rx="28" fill="#0f172a" stroke="${accentSoft}" stroke-width="2"/>
      <circle cx="770" cy="112" r="72" fill="${accent}" opacity="0.18"/>
      <circle cx="770" cy="112" r="44" fill="${accentSoft}" opacity="0.26"/>
      <g opacity="0.92">${motifMarkup}</g>
      <rect x="68" y="72" width="132" height="32" rx="16" fill="${accentSoft}" opacity="0.22"/>
      <text x="88" y="94" fill="#e2e8f0" font-family="Inter, ui-sans-serif, system-ui, sans-serif" font-size="18" font-weight="600">系统推荐</text>
      <text x="74" y="500" fill="#f8fafc" font-family="Inter, ui-sans-serif, system-ui, sans-serif" font-size="32" font-weight="700">${label}</text>
      <text x="74" y="538" fill="#94a3b8" font-family="Inter, ui-sans-serif, system-ui, sans-serif" font-size="20">${sublabel}</text>
    </svg>
  `
  return `data:image/svg+xml;charset=UTF-8,${encodeURIComponent(svg)}`
}

const appNavItems: Array<{
  page: Exclude<AppPage, 'prompts'>
  label: string
  icon: ReactNode
}> = [
  { page: 'home', label: '首页', icon: <Home className="h-4 w-4" /> },
  { page: 'orders', label: '日志', icon: <History className="h-4 w-4" /> },
  { page: 'flows', label: '流水', icon: <Wallet className="h-4 w-4" /> },
  { page: 'profile', label: '个人', icon: <UserRound className="h-4 w-4" /> },
  { page: 'recharge', label: '充值', icon: <CreditCard className="h-4 w-4" /> },
]

const featuredPromptItems: PromptGalleryItem[] = [
  {
    id: 'gallery-city-fox',
    title: '机械狐狸城市屋顶',
    prompt: '一只站在城市屋顶上的机械狐狸，电影感光影，雨夜霓虹，金属细节，高级质感',
    previewUrl: createPromptPreviewArt({
      title: '机械狐狸城市屋顶',
      subtitle: '雨夜霓虹 · 电影感光影',
      accent: '#7c3aed',
      accentSoft: '#38bdf8',
      motif: 'fox',
    }),
    author: '系统推荐',
    status: 'approved',
    favorites: 128,
    createdAt: '2026-05-01T10:00:00+08:00',
  },
  {
    id: 'gallery-product-light',
    title: '高级产品棚拍光影',
    prompt: '一款未来感智能设备，黑色磨砂材质，柔和棚拍光，极简背景，商业广告摄影',
    previewUrl: createPromptPreviewArt({
      title: '高级产品棚拍光影',
      subtitle: '极简背景 · 商业广告摄影',
      accent: '#db2777',
      accentSoft: '#f59e0b',
      motif: 'product',
    }),
    author: '系统推荐',
    status: 'approved',
    favorites: 86,
    createdAt: '2026-05-01T11:30:00+08:00',
  },
  {
    id: 'gallery-anime-room',
    title: '治愈系动画房间',
    prompt: '温暖的动画风格房间，午后阳光穿过窗帘，桌上有书和咖啡，柔和色彩，细节丰富',
    previewUrl: createPromptPreviewArt({
      title: '治愈系动画房间',
      subtitle: '午后阳光 · 柔和色彩',
      accent: '#10b981',
      accentSoft: '#a855f7',
      motif: 'room',
    }),
    author: '系统推荐',
    status: 'approved',
    favorites: 64,
    createdAt: '2026-05-02T09:20:00+08:00',
  },
]

const sizeOptions = [
  { value: '1024x1024', label: '1024x1024 方图' },
  { value: '1024x1536', label: '1024x1536 竖图' },
  { value: '1536x1024', label: '1536x1024 横图' },
  { value: '2048x2048', label: '2048x2048 高清方图' },
  { value: '2048x1152', label: '2048x1152 宽屏' },
  { value: '3840x2160', label: '3840x2160 4K 横屏' },
  { value: '2160x3840', label: '2160x3840 4K 竖屏' },
  { value: 'auto', label: '自动' },
  { value: 'custom', label: '自定义尺寸' },
]

const styleOptions = [
  { value: '', label: '无' },
  { value: 'realistic', label: '写实' },
  { value: 'anime', label: '动漫' },
  { value: 'oil painting', label: '油画' },
  { value: 'watercolor', label: '水彩' },
  { value: 'pixel art', label: '像素' },
  { value: '3d', label: '3D' },
  { value: 'cinematic', label: '电影感' },
]

const qualityOptions = [
  { value: 'auto', label: '自动' },
  { value: 'low', label: '低' },
  { value: 'medium', label: '中' },
  { value: 'high', label: '高' },
]

const outputFormatOptions = [
  { value: 'default', label: '默认' },
  { value: 'jpeg', label: 'JPEG' },
  { value: 'webp', label: 'WebP' },
  { value: 'png', label: 'PNG' },
]

const emptyGenerationProgress: GenerationProgress = {
  taskId: null,
  prompt: '',
  status: 'idle',
  startedAt: null,
  elapsedSeconds: 0,
  total: 1,
  completed: 0,
  success: 0,
  running: 0,
  attempt: 0,
  backendUrl: api.API_BASE_URL,
  message: '等待提交',
}

const emptyPreview: ImageTaskDetailResponse = {
  taskId: '-',
  prompt: '还没有生成任务，先提交一条提示词。',
  imageUrl: '',
  costPoints: 1,
  costCredits: 1,
  freeCreditsCost: 0,
  paidCreditsCost: 0,
  creditSource: 'free',
  status: 'idle',
  requestParams: '',
  createdAt: '',
}

const HISTORY_PAGE_SIZE = 5
const ACTIVITY_PAGE_SIZE = 10
const FLOW_LOG_PAGE_SIZE = 10
const RECHARGE_ORDER_PAGE_SIZE = 5

function App() {
  const [mode, setMode] = useState<AuthMode>(() =>
    REGISTRATION_ENABLED && window.location.pathname === '/register' ? 'register' : 'login',
  )
  const [activePage, setActivePage] = useState<AppPage>('home')
  const [sidebarCollapsed, setSidebarCollapsed] = useState(false)
  const [sidebarOpen, setSidebarOpen] = useState(false)
  const [busy, setBusy] = useState<BusyAction>(null)
  const [message, setMessage] = useState('')
  const [token, setToken] = useState(() => getStoredToken())
  const [authForm, setAuthForm] = useState({
    email: '',
    code: '',
    username: '',
    account: '',
    password: '',
    confirmPassword: '',
  })
  const [authFormSeed, setAuthFormSeed] = useState(0)
  const [changePasswordForm, setChangePasswordForm] = useState<PasswordChangeForm>({
    currentPassword: '',
    newPassword: '',
    confirmNewPassword: '',
  })
  const [prompt, setPrompt] = useState('')
  const [size, setSize] = useState('1024x1024')
  const [style, setStyle] = useState('')
  const [count, setCount] = useState(1)
  const [quality, setQuality] = useState('auto')
  const [outputFormat, setOutputFormat] = useState('default')
  const [customWidth, setCustomWidth] = useState(1024)
  const [customHeight, setCustomHeight] = useState(1024)
  const [referenceImageURL, setReferenceImageURL] = useState('')
  const [referenceImageName, setReferenceImageName] = useState('')
  const [referenceImageSource, setReferenceImageSource] = useState<ReferenceImageSource>('none')
  const [showNetworkImageInput, setShowNetworkImageInput] = useState(false)
  const [showRegisterPassword, setShowRegisterPassword] = useState(false)
  const [showChangePassword, setShowChangePassword] = useState(false)
  const [referenceImageUploading, setReferenceImageUploading] = useState(false)
  const referenceImageInputRef = useRef<HTMLInputElement | null>(null)
  const [currentUser, setCurrentUser] = useState<UserInfo | null>(null)
  const [adminOpen, setAdminOpen] = useState(() => window.location.pathname.startsWith('/admin'))
  const [adminStats, setAdminStats] = useState<AdminStatsResponse | null>(null)
  const [adminUsers, setAdminUsers] = useState<AdminUserItem[]>([])
  const [adminTasks, setAdminTasks] = useState<AdminImageTaskItem[]>([])
  const [adminOrders, setAdminOrders] = useState<AdminRechargeOrderItem[]>([])
  const [adminAudits, setAdminAudits] = useState<AdminContentAuditItem[]>([])
  const [adminActionLogs, setAdminActionLogs] = useState<AdminActionLogItem[]>([])
  const [adminPackages, setAdminPackages] = useState<AdminPackageItem[]>([])
  const [generationProgresses, setGenerationProgresses] = useState<GenerationProgress[]>([])
  const [points, setPoints] = useState<PointsResponse | null>(null)
  const [history, setHistory] = useState<ImageTaskItem[]>([])
  const [historyPage, setHistoryPage] = useState(1)
  const [historyTotal, setHistoryTotal] = useState(0)
  const [activityLogs, setActivityLogs] = useState<UserActivityItem[]>([])
  const [activityPage, setActivityPage] = useState(1)
  const [activityTotal, setActivityTotal] = useState(0)
  const [pointsLogItems, setPointsLogItems] = useState<ListPointsLogsResponse['items']>([])
  const [pointsLogPage, setPointsLogPage] = useState(1)
  const [pointsLogTotal, setPointsLogTotal] = useState(0)
  const [creditLogItems, setCreditLogItems] = useState<ListCreditLogsResponse['items']>([])
  const [creditLogPage, setCreditLogPage] = useState(1)
  const [creditLogTotal, setCreditLogTotal] = useState(0)
  const [packages, setPackages] = useState<PackageItem[]>([])
  const [billingSettings, setBillingSettings] = useState<BillingSettingsResponse | null>(null)
  const [customRechargeCount, setCustomRechargeCount] = useState(10)
  const [rechargeOrders, setRechargeOrders] = useState<RechargeOrderItem[]>([])
  const [rechargeOrderPage, setRechargeOrderPage] = useState(1)
  const [rechargeOrderTotal, setRechargeOrderTotal] = useState(0)
  const [preview, setPreview] = useState<ImageTaskDetailResponse>(emptyPreview)
  const [previewImageIndex, setPreviewImageIndex] = useState(0)
  const [previewDetailOpen, setPreviewDetailOpen] = useState(false)
  const [wechatSaveGuide, setWechatSaveGuide] = useState<WechatSaveGuideState>({
    open: false,
    imageUrl: '',
    title: '',
  })
  const [favoritePromptIds, setFavoritePromptIds] = useState<string[]>(['gallery-city-fox'])
  const [submittedPrompts, setSubmittedPrompts] = useState<PromptGalleryItem[]>([])
  const [promptSubmission, setPromptSubmission] = useState({
    title: '',
    prompt: '',
    previewUrl: '',
  })
  const previewTaskIdRef = useRef(emptyPreview.taskId)
  const pollingTaskIdsRef = useRef<Set<string>>(new Set())

  const isAuthed = token.length > 0
  const isAdmin = currentUser?.role === 'admin'
  const canRegister = REGISTRATION_ENABLED
  const hasRunningGeneration = generationProgresses.some((item) => item.status === 'running' && item.taskId)
  const promptGalleryItems = useMemo(
    () => [...submittedPrompts, ...featuredPromptItems],
    [submittedPrompts],
  )

  useEffect(() => {
    if (isAuthed && adminOpen && currentUser && currentUser.role !== 'admin') {
      setAdminOpen(false)
      setMessage('当前账号没有管理员权限，已返回用户端。')
    }
  }, [isAuthed, adminOpen, currentUser])

  useEffect(() => {
    void loadPackages()
  }, [])

  useEffect(() => {
    previewTaskIdRef.current = preview.taskId
  }, [preview.taskId])

  useEffect(() => {
    if (isAuthed) {
      void loadDashboard()
    } else {
      setCurrentUser(null)
      setAdminOpen(false)
      setAdminActionLogs([])
      setPoints(null)
      setChangePasswordForm({
        currentPassword: '',
        newPassword: '',
        confirmNewPassword: '',
      })
      setShowChangePassword(false)
      setHistory([])
      setHistoryPage(1)
      setHistoryTotal(0)
      setActivityLogs([])
      setActivityPage(1)
      setActivityTotal(0)
      setPointsLogItems([])
      setPointsLogPage(1)
      setPointsLogTotal(0)
      setCreditLogItems([])
      setCreditLogPage(1)
      setCreditLogTotal(0)
      setRechargeOrders([])
      setRechargeOrderPage(1)
      setRechargeOrderTotal(0)
      setPreview(emptyPreview)
      setPreviewImageIndex(0)
      setPreviewDetailOpen(false)
      setWechatSaveGuide({ open: false, imageUrl: '', title: '' })
      pollingTaskIdsRef.current.clear()
      setGenerationProgresses([])
    }
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [isAuthed])

  useEffect(() => {
    const nextPath = isAuthed
      ? (adminOpen ? '/admin' : '/app')
      : mode === 'register' && canRegister
        ? '/register'
        : '/login'
    if (window.location.pathname !== nextPath) {
      window.history.replaceState(null, '', nextPath)
    }
  }, [canRegister, isAuthed, mode, adminOpen])

  useEffect(() => {
    if (isAuthed || mode !== 'login') {
      return
    }

    const timers = [0, 150, 500].map((delay) =>
      window.setTimeout(() => {
        setAuthForm((current) => {
          if (!current.account && !current.password) {
            return current
          }
          return {
            ...current,
            account: '',
            password: '',
          }
        })
      }, delay),
    )

    return () => {
      timers.forEach((timer) => window.clearTimeout(timer))
    }
  }, [authFormSeed, isAuthed, mode])

  useEffect(() => {
    const syncFromPath = () => {
      const path = window.location.pathname
      if (path === '/register' && canRegister) {
        setMode('register')
      } else if (path === '/login') {
        setMode('login')
      } else if (path === '/register') {
        setMode('login')
      }

      if (path.startsWith('/admin')) {
        setAdminOpen(true)
      } else {
        setAdminOpen(false)
      }
    }

    window.addEventListener('popstate', syncFromPath)
    return () => window.removeEventListener('popstate', syncFromPath)
  }, [canRegister])

  useEffect(() => {
    if (!hasRunningGeneration) {
      return
    }

    const timer = window.setInterval(() => {
      setGenerationProgresses((current) =>
        current.map((item) => {
          if (item.status !== 'running' || !item.startedAt) {
            return item
          }

          const elapsedSeconds = Math.max(0, Math.floor((Date.now() - item.startedAt) / 1000))
          return {
            ...item,
            elapsedSeconds,
            running: Math.max(0, item.total - item.completed),
          }
        }),
      )
    }, 1000)

    return () => window.clearInterval(timer)
  }, [hasRunningGeneration])

  useEffect(() => {
    for (const progress of generationProgresses) {
      if (!progress.taskId || progress.status !== 'running' || pollingTaskIdsRef.current.has(progress.taskId)) {
        continue
      }
      pollingTaskIdsRef.current.add(progress.taskId)
      void pollGenerationTask(progress)
    }
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [generationProgresses])

  function upsertGenerationProgress(progress: GenerationProgress) {
    if (!progress.taskId) {
      return
    }

    setGenerationProgresses((current) =>
      limitGenerationProgresses([
        progress,
        ...current.filter((item) => item.taskId !== progress.taskId),
      ]),
    )
  }

  function updateGenerationProgress(taskId: string, updater: (current: GenerationProgress) => GenerationProgress) {
    setGenerationProgresses((current) =>
      current.map((item) => (item.taskId === taskId ? updater(item) : item)),
    )
  }

  async function pollGenerationTask(progress: GenerationProgress) {
    const taskId = progress.taskId
    if (!taskId) {
      return
    }

    const startedAt = progress.startedAt ?? Date.now()
    let attempt = Math.max(0, progress.attempt - 1)
    const sleep = (delayMs: number) =>
      new Promise<void>((resolve) => {
        window.setTimeout(resolve, delayMs)
      })

    try {
      while (pollingTaskIdsRef.current.has(taskId)) {
        attempt += 1

        try {
          const detail = await api.getImageTask(taskId)
          if (!pollingTaskIdsRef.current.has(taskId)) {
            return
          }

          const imageURLs = normalizeImageURLs(detail.imageUrls, detail.imageUrl)
          const elapsedSeconds = Math.max(0, Math.floor((Date.now() - startedAt) / 1000))
          const isFinal = detail.status === 'success' || detail.status === 'failed'
          const finalCount = Math.max(1, imageURLs.length || 1)
          const nextMessage =
            detail.status === 'failed'
              ? detail.errorMessage ?? '生成失败'
              : imageURLs.length > 0
                ? `已拿到 ${imageURLs.length} 张图片`
                : '任务已完成'

          setPreview((current) =>
            current.taskId === taskId
              ? {
                  ...current,
                  prompt: detail.prompt,
                  imageUrl: detail.imageUrl,
                  imageUrls: imageURLs,
                  costPoints: detail.costPoints,
                  costCredits: detail.costCredits,
                  freeCreditsCost: detail.freeCreditsCost,
                  paidCreditsCost: detail.paidCreditsCost,
                  creditSource: detail.creditSource,
                  status: detail.status,
                  requestParams: detail.requestParams,
                  errorMessage: detail.errorMessage,
                  createdAt: detail.createdAt,
                }
              : current,
          )
          setHistory((current) =>
            current.map((item) =>
              item.id === taskId
                ? {
                    ...item,
                    prompt: detail.prompt,
                    imageUrl: detail.imageUrl,
                    imageUrls: imageURLs,
                    costPoints: detail.costPoints,
                    costCredits: detail.costCredits,
                    freeCreditsCost: detail.freeCreditsCost,
                    paidCreditsCost: detail.paidCreditsCost,
                    creditSource: detail.creditSource,
                    status: detail.status,
                    requestParams: detail.requestParams,
                    errorMessage: detail.errorMessage,
                    createdAt: detail.createdAt,
                  }
                : item,
            ),
          )

          if (isFinal) {
          const finalPreview = detailToPreview(detail)
            setPreview((current) => (current.taskId === taskId ? finalPreview : current))
            if (previewTaskIdRef.current === taskId) {
              setPreviewImageIndex(0)
            }
            updateGenerationProgress(taskId, (current) => ({
              ...current,
              prompt: detail.prompt,
              status: detail.status === 'failed' ? 'failed' : 'success',
              elapsedSeconds,
              total: finalCount,
              completed: finalCount,
              success: detail.status === 'failed' ? 0 : finalCount,
              running: 0,
              attempt,
              message: nextMessage,
            }))
            if (detail.status === 'failed') {
              setMessage(`生图失败：${detail.errorMessage ?? '未知错误'}`)
            }
            void loadDashboard({ showBusy: false, updatePreview: false, updateMessage: false })
            return
          }

          updateGenerationProgress(taskId, (current) => ({
            ...current,
            prompt: detail.prompt,
            elapsedSeconds,
            total: Math.max(1, current.total),
            completed: 0,
            success: 0,
            running: Math.max(1, current.total),
            attempt,
            message: '任务正在生成中',
          }))
        } catch (error) {
          if (!pollingTaskIdsRef.current.has(taskId)) {
            return
          }
          if (error instanceof APIError && error.status === 401) {
            handleLogout()
            return
          }
          const errorMessage = error instanceof Error ? error.message : '轮询任务状态失败'
          updateGenerationProgress(taskId, (current) => ({
            ...current,
            elapsedSeconds: Math.max(0, Math.floor((Date.now() - startedAt) / 1000)),
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

  async function loadPackages() {
    try {
      const [rechargeResult, settingsResult] = await Promise.all([
        api.listPackages('recharge'),
        api.getBillingSettings(),
      ])
      setPackages(rechargeResult.items.filter((item) => item.packageType === 'recharge' && item.enabled))
      setBillingSettings(settingsResult)
    } catch (error) {
      console.error(error)
    }
  }

  async function loadDashboard(options: { showBusy?: boolean; updatePreview?: boolean; updateMessage?: boolean } = {}) {
    const { showBusy = true, updateMessage = true } = options
    try {
      if (showBusy) {
        setBusy('refresh')
      }
      const [meResult, pointsResult, historyResult, activityResult, pointsLogResult, creditLogResult, rechargeResult, latestResult] = await Promise.all([
        api.getMe(),
        api.getPoints(),
        api.listImageTasks(historyPage, HISTORY_PAGE_SIZE),
        api.listUserActivity(activityPage, ACTIVITY_PAGE_SIZE),
        api.listPointsLogs(pointsLogPage, FLOW_LOG_PAGE_SIZE),
        api.listCreditLogs(creditLogPage, FLOW_LOG_PAGE_SIZE),
        api.listRechargeOrders(rechargeOrderPage, RECHARGE_ORDER_PAGE_SIZE),
        api.listImageTasks(1, HISTORY_PAGE_SIZE),
      ])
      setCurrentUser(meResult)
      setPoints(pointsResult)
      setHistory(historyResult.items)
      setHistoryPage(historyResult.page)
      setHistoryTotal(historyResult.total)
      setActivityLogs(activityResult.items)
      setActivityPage(activityResult.page)
      setActivityTotal(activityResult.total)
      setPointsLogItems(pointsLogResult.items)
      setPointsLogPage(pointsLogResult.page)
      setPointsLogTotal(pointsLogResult.total)
      setCreditLogItems(creditLogResult.items)
      setCreditLogPage(creditLogResult.page)
      setCreditLogTotal(creditLogResult.total)
      setRechargeOrders(rechargeResult.items)
      setRechargeOrderPage(rechargeResult.page)
      setRechargeOrderTotal(rechargeResult.total)
      const runningTasks = latestResult.items.filter((item) => item.status === 'generating' || item.status === 'running')
      if (runningTasks.length > 0) {
        setGenerationProgresses((current) => {
          const currentIds = new Set(current.map((item) => item.taskId))
          const next = [...current]
          for (const task of [...runningTasks].reverse()) {
            if (currentIds.has(task.id)) {
              continue
            }
            next.unshift(progressFromImageTaskItem(task))
          }
          return limitGenerationProgresses(next)
        })
      }
    } catch (error) {
      if (error instanceof APIError && error.status === 401) {
        handleLogout()
        if (updateMessage) {
          setMessage('登录已过期，请重新登录。')
        }
      } else {
        if (updateMessage) {
          setMessage(error instanceof Error ? error.message : '同步失败')
        }
      }
    } finally {
      if (showBusy) {
        setBusy(null)
      }
    }
  }

  async function loadActivityPage(page: number, options: { showBusy?: boolean } = {}) {
    const { showBusy = false } = options
    try {
      if (showBusy) {
        setBusy('refresh')
      }
      const pageResult = await api.listUserActivity(page, ACTIVITY_PAGE_SIZE)
      setActivityLogs(pageResult.items)
      setActivityPage(pageResult.page)
      setActivityTotal(pageResult.total)
    } catch (error) {
      handleApiError(error, '日志加载失败')
    } finally {
      if (showBusy) {
        setBusy(null)
      }
    }
  }

  async function loadHistoryPage(page: number, options: { showBusy?: boolean } = {}) {
    const { showBusy = false } = options
    try {
      if (showBusy) {
        setBusy('refresh')
      }
      const pageResult = await api.listImageTasks(page, HISTORY_PAGE_SIZE)
      setHistory(pageResult.items)
      setHistoryPage(pageResult.page)
      setHistoryTotal(pageResult.total)
    } catch (error) {
      handleApiError(error, '历史记录加载失败')
    } finally {
      if (showBusy) {
        setBusy(null)
      }
    }
  }

  async function loadPointsLogPage(page: number, options: { showBusy?: boolean } = {}) {
    const { showBusy = false } = options
    try {
      if (showBusy) {
        setBusy('refresh')
      }
      const pageResult = await api.listPointsLogs(page, FLOW_LOG_PAGE_SIZE)
      setPointsLogItems(pageResult.items)
      setPointsLogPage(pageResult.page)
      setPointsLogTotal(pageResult.total)
    } catch (error) {
      handleApiError(error, '积分流水加载失败')
    } finally {
      if (showBusy) {
        setBusy(null)
      }
    }
  }

  async function loadCreditLogPage(page: number, options: { showBusy?: boolean } = {}) {
    const { showBusy = false } = options
    try {
      if (showBusy) {
        setBusy('refresh')
      }
      const pageResult = await api.listCreditLogs(page, FLOW_LOG_PAGE_SIZE)
      setCreditLogItems(pageResult.items)
      setCreditLogPage(pageResult.page)
      setCreditLogTotal(pageResult.total)
    } catch (error) {
      handleApiError(error, '次数流水加载失败')
    } finally {
      if (showBusy) {
        setBusy(null)
      }
    }
  }

  async function loadRechargeOrderPage(page: number, options: { showBusy?: boolean } = {}) {
    const { showBusy = false } = options
    try {
      if (showBusy) {
        setBusy('refresh')
      }
      const pageResult = await api.listRechargeOrders(page, RECHARGE_ORDER_PAGE_SIZE)
      setRechargeOrders(pageResult.items)
      setRechargeOrderPage(pageResult.page)
      setRechargeOrderTotal(pageResult.total)
    } catch (error) {
      handleApiError(error, '充值订单加载失败')
    } finally {
      if (showBusy) {
        setBusy(null)
      }
    }
  }

  async function handleSendCode() {
    if (!authForm.email.trim()) {
      setMessage('请先填写邮箱。')
      return
    }
    if (!REGISTRATION_ENABLED) {
      setMessage('注册功能暂时关闭。')
      return
    }

    try {
      setBusy('send-code')
      const result = await api.sendEmailCode({
        email: authForm.email,
        scene: 'register',
      })
      if (result.debugCode) {
        setAuthForm((current) => ({ ...current, code: result.debugCode ?? current.code }))
      }
      setMessage(
        result.debugCode
          ? `验证码已生成，开发态验证码是 ${result.debugCode}。`
          : `验证码已发送，过期时间 ${result.expiresIn} 秒。`,
      )
    } catch (error) {
      setMessage(error instanceof Error ? error.message : '发送验证码失败')
    } finally {
      setBusy(null)
    }
  }

  async function handleAuthSubmit() {
    try {
      setBusy('auth')

      let result: AuthResponse
      if (mode === 'register') {
        if (!REGISTRATION_ENABLED) {
          setMessage('注册功能暂时关闭。')
          return
        }
        if (!authForm.email || !authForm.code || !authForm.username || !authForm.password || !authForm.confirmPassword) {
          setMessage('请把注册所需字段补齐。')
          return
        }
        if (authForm.password !== authForm.confirmPassword) {
          setMessage('两次输入的密码不一致。')
          return
        }
        result = await api.register({
          email: authForm.email,
          code: authForm.code,
          username: authForm.username,
          password: authForm.password,
        })
      } else {
        if (!authForm.account || !authForm.password) {
          setMessage('请把登录所需字段补齐。')
          return
        }
        result = await api.login({
          account: authForm.account,
          password: authForm.password,
        })
      }

      saveStoredToken(result.accessToken)
      setToken(result.accessToken)
      setCurrentUser(result.user)
      setAdminOpen(result.user.role === 'admin')
      setPoints({
        balance: result.user.pointsBalance,
        freeCredits: result.user.freeCredits,
        paidCredits: result.user.paidCredits,
        totalCredits: result.user.totalCredits,
        streak: result.user.checkinStreak,
        lastCheckinAt: result.user.lastCheckinAt ?? null,
        recentLogs: [],
        recentCreditLogs: [],
      })
      await loadPackages()
      await loadDashboard()
    } catch (error) {
      setMessage(error instanceof Error ? error.message : '认证失败')
    } finally {
      setBusy(null)
    }
  }

  async function handleCheckin() {
    if (!isAuthed) {
      setMessage('请先登录，再执行签到。')
      return
    }

    try {
      setBusy('checkin')
      const result = await api.checkin()
      setMessage(
        result.checkedIn
          ? `签到成功，获得 ${result.gainedPoints} 积分和 ${result.gainedCredits} 次生图次数。`
          : '今天已经签到过了。',
      )
      await loadDashboard()
    } catch (error) {
      handleApiError(error, '签到失败')
    } finally {
      setBusy(null)
    }
  }

  async function handleChangePassword() {
    if (!isAuthed) {
      setMessage('请先登录后再修改密码。')
      return
    }

    if (
      !changePasswordForm.currentPassword.trim() ||
      !changePasswordForm.newPassword.trim() ||
      !changePasswordForm.confirmNewPassword.trim()
    ) {
      setMessage('请把修改密码所需字段补齐。')
      return
    }
    if (changePasswordForm.newPassword !== changePasswordForm.confirmNewPassword) {
      setMessage('两次输入的新密码不一致。')
      return
    }

    try {
      setBusy('change-password')
      await api.changePassword({
        currentPassword: changePasswordForm.currentPassword,
        newPassword: changePasswordForm.newPassword,
        confirmNewPassword: changePasswordForm.confirmNewPassword,
      })
      setChangePasswordForm({
        currentPassword: '',
        newPassword: '',
        confirmNewPassword: '',
      })
      setShowChangePassword(false)
      setMessage('密码修改成功。')
    } catch (error) {
      handleApiError(error, '修改密码失败')
    } finally {
      setBusy(null)
    }
  }

  async function handlePurchase(packageCode: string) {
    if (!isAuthed) {
      setMessage('请先登录，再创建充值订单。')
      return
    }

    const selectedPackage = packages.find((item) => item.code === packageCode)
    if (!selectedPackage) {
      setMessage('充值套餐不存在或已停用。')
      return
    }

    const confirmed = window.confirm(
      [
        `确认购买：${selectedPackage.name}`,
        `到账次数：${selectedPackage.creditsAmount} 次`,
        `支付金额：${(selectedPackage.amountCents / 100).toFixed(2)} 元`,
        `赠送积分：${selectedPackage.bonusPoints} 积分`,
      ].join('\n'),
    )
    if (!confirmed) {
      return
    }

    try {
      setBusy(`purchase:${packageCode}`)
      const result = await api.createRechargeOrder({ rechargeType: 'package', packageCode })
      setMessage(
        `订单已创建，订单号 ${result.orderNo}。待支付成功后将入账 ${result.creditsAmount} 次充值次数${
          result.bonusPoints > 0 ? `，并额外赠送 ${result.bonusPoints} 积分` : ''
        }。`,
      )
      await loadDashboard({ showBusy: false, updatePreview: false, updateMessage: false })
    } catch (error) {
      handleApiError(error, '创建充值订单失败')
    } finally {
      setBusy(null)
    }
  }

  async function handleCustomRecharge() {
    if (!isAuthed) {
      setMessage('请先登录，再创建充值订单。')
      return
    }

    const countValue = Math.max(1, Math.floor(customRechargeCount || 0))
    const unitPriceCents = billingSettings?.customRechargeUnitPriceCents ?? 20
    const amountCents = countValue * unitPriceCents
    const confirmed = window.confirm(
      [
        '确认自定义充值',
        `充值次数：${countValue} 次`,
        `单次价格：${(unitPriceCents / 100).toFixed(2)} 元`,
        `支付金额：${(amountCents / 100).toFixed(2)} 元`,
        '提示：自定义充值不赠送积分。',
      ].join('\n'),
    )
    if (!confirmed) {
      return
    }

    try {
      setBusy('purchase:custom')
      const result = await api.createRechargeOrder({ rechargeType: 'custom', count: countValue })
      setMessage(`订单已创建，订单号 ${result.orderNo}。自定义充值 ${result.creditsAmount} 次，不赠送积分。`)
      await loadDashboard({ showBusy: false, updatePreview: false, updateMessage: false })
    } catch (error) {
      handleApiError(error, '创建自定义充值订单失败')
    } finally {
      setBusy(null)
    }
  }

  async function handleConfirmRechargeOrder(order: RechargeOrderItem) {
    if (!isAuthed) {
      setMessage('请先登录，再确认支付。')
      return
    }

    try {
      setBusy(`confirm-recharge:${order.orderNo}`)
      const result = await api.confirmRechargeOrder(order.orderNo, {
        providerTradeNo: `mock-${order.orderNo}`,
        paymentChannel: 'mock',
        amountCents: order.amountCents,
        paidAt: new Date().toISOString(),
        remark: '前端模拟支付成功',
      })
      setMessage(
        `支付确认成功，已入账 ${result.creditsAmount} 次充值次数${
          result.bonusPoints > 0 ? `，并赠送 ${result.bonusPoints} 积分` : ''
        }。`,
      )
      await loadDashboard({ showBusy: false, updatePreview: false, updateMessage: false })
    } catch (error) {
      handleApiError(error, '确认支付失败')
    } finally {
      setBusy(null)
    }
  }

  async function handleCreateTask() {
    if (!isAuthed) {
      setMessage('请先登录，再提交生图任务。')
      return
    }

    if (!prompt.trim()) {
      setMessage('提示词不能为空。')
      return
    }

    const promptText = prompt.trim()
    const requestSize = resolveSelectedSize(size, customWidth, customHeight)
    const taskCount = clampCount(count)
    const requestSnapshot = {
      prompt: promptText,
      size: requestSize,
      style,
      count: taskCount,
      quality,
      outputFormat,
      referenceImageUrl: referenceImageURL.trim() || undefined,
    } satisfies TaskRequestSnapshot
    const startedAt = Date.now()

    try {
      setBusy('create-task')
      const result = await api.createImageTask(requestSnapshot)

      const now = new Date().toISOString()
      const resultImageURLs = normalizeImageURLs(result.imageUrls, result.imageUrl)
      setPreview(
        detailToPreview({
          taskId: result.taskId,
          prompt: promptText,
          imageUrl: result.imageUrl,
          imageUrls: resultImageURLs,
          costPoints: taskCount,
          costCredits: taskCount,
          freeCreditsCost: 0,
          paidCreditsCost: 0,
          creditSource: 'free',
          status: result.status || 'generating',
          requestParams: stringifyTaskRequestSnapshot(requestSnapshot),
          errorMessage: result.errorMessage,
          createdAt: now,
        }),
      )
      setPreviewImageIndex(0)
      setHistoryPage(1)
      void loadHistoryPage(1, { showBusy: false })
      setPoints((current) => {
        if (!current) {
          return current
        }
        return {
          ...current,
          balance: result.pointsBalance ?? current.balance,
          freeCredits: result.freeCreditsBalance ?? current.freeCredits,
          paidCredits: result.paidCreditsBalance ?? current.paidCredits,
          totalCredits: result.totalCreditsBalance ?? current.totalCredits,
        }
      })
      const generationStatus = normalizeGenerationStatus(result.status)
      upsertGenerationProgress({
        taskId: result.taskId,
        prompt: promptText,
        status: generationStatus,
        startedAt,
        elapsedSeconds: 0,
        total: taskCount,
        completed: generationStatus === 'running' ? 0 : taskCount,
        success: generationStatus === 'success' ? taskCount : 0,
        running: generationStatus === 'running' ? taskCount : 0,
        attempt: 1,
        backendUrl: api.API_BASE_URL,
        message:
          generationStatus === 'running'
            ? `任务已提交，任务编号：${result.taskId}，消耗 ${taskCount} 次`
            : generationStatus === 'success'
              ? `任务已提交，任务编号：${result.taskId}，已直接完成。`
              : `任务已提交，任务编号：${result.taskId}，生成失败。`,
      })
      if (generationStatus === 'failed') {
        setMessage(`任务已提交，任务编号：${result.taskId}，但生成失败。`)
      }
    } catch (error) {
      handleApiError(error, '提交失败')
    } finally {
      setBusy(null)
    }
  }

  async function handleRetryTask(taskId: string) {
    if (!isAuthed) {
      setMessage('请先登录，再重试任务。')
      return
    }
    if (!taskId || taskId === '-') {
      setMessage('请先选择一个历史任务。')
      return
    }
    const startedAt = Date.now()
    const optimisticTotal = Math.max(1, clampCount(Number(preview.costCredits || preview.costPoints)))

    try {
      setBusy(`retry:${taskId}`)
      const result = await api.retryImageTask(taskId)
      const now = new Date().toISOString()
      const resultImageURLs = normalizeImageURLs(result.imageUrls, result.imageUrl)
      setPreview(
        detailToPreview({
          taskId: result.taskId,
          prompt: preview.prompt,
          imageUrl: result.imageUrl,
          imageUrls: resultImageURLs,
          costPoints: optimisticTotal,
          costCredits: optimisticTotal,
          freeCreditsCost: 0,
          paidCreditsCost: 0,
          creditSource: 'free',
          status: result.status || 'generating',
          requestParams: preview.requestParams,
          errorMessage: result.errorMessage,
          createdAt: now,
        }),
      )
      setPreviewImageIndex(0)
      setHistoryPage(1)
      void loadHistoryPage(1, { showBusy: false })
      setPoints((current) => {
        if (!current) {
          return current
        }
        return {
          ...current,
          balance: result.pointsBalance ?? current.balance,
          freeCredits: result.freeCreditsBalance ?? current.freeCredits,
          paidCredits: result.paidCreditsBalance ?? current.paidCredits,
          totalCredits: result.totalCreditsBalance ?? current.totalCredits,
        }
      })
      const generationStatus = normalizeGenerationStatus(result.status)
      upsertGenerationProgress({
        taskId: result.taskId,
        prompt: preview.prompt,
        status: generationStatus,
        startedAt,
        elapsedSeconds: 0,
        total: optimisticTotal,
        completed: generationStatus === 'running' ? 0 : optimisticTotal,
        success: generationStatus === 'success' ? optimisticTotal : 0,
        running: generationStatus === 'running' ? optimisticTotal : 0,
        attempt: 1,
        backendUrl: api.API_BASE_URL,
        message:
          generationStatus === 'running'
            ? `任务已重试，任务编号：${result.taskId}`
            : generationStatus === 'success'
              ? `任务已重试，任务编号：${result.taskId}，已直接完成。`
              : `任务已重试，任务编号：${result.taskId}，生成失败。`,
      })
      if (generationStatus === 'failed') {
        setMessage(`任务已重试，任务编号：${result.taskId}，但生成失败。`)
      }
    } catch (error) {
      handleApiError(error, '重试任务失败')
    } finally {
      setBusy(null)
    }
  }

  function handleSelectTask(item: ImageTaskItem) {
    setPreview(toPreview(item))
    setPreviewImageIndex(0)
  }

  function handlePreviousPreviewImage() {
    setPreviewImageIndex((current) => {
      const total = normalizeImageURLs(preview.imageUrls, preview.imageUrl).length
      if (total <= 1) {
        return current
      }
      return (current - 1 + total) % total
    })
  }

  function handleNextPreviewImage() {
    setPreviewImageIndex((current) => {
      const total = normalizeImageURLs(preview.imageUrls, preview.imageUrl).length
      if (total <= 1) {
        return current
      }
      return (current + 1) % total
    })
  }

  async function handleSaveCurrentImage() {
    const imageURLs = normalizeImageURLs(preview.imageUrls, preview.imageUrl)
    const imageUrl = imageURLs[previewImageIndex] ?? imageURLs[0]
    if (!imageUrl) {
      setMessage('当前没有可保存的图片。')
      return
    }
    if (isWeChatBrowser()) {
      setWechatSaveGuide({
        open: true,
        imageUrl,
        title: preview.prompt ? `第 ${previewImageIndex + 1} 张图片` : '当前图片',
      })
      setMessage('微信内请长按图片保存。')
      return
    }
    if (!preview.taskId || preview.taskId === '-') {
      setMessage('当前图片缺少任务编号，暂时无法直接下载。')
      return
    }

    try {
      const { blob, fileName } = await api.downloadImageTaskImage(preview.taskId, previewImageIndex)
      triggerImageDownload(blob, fileName)
    } catch (error) {
      handleApiError(error, '保存当前图片失败')
    }
  }

  function handleOpenPreviewDetail() {
    const imageURLs = normalizeImageURLs(preview.imageUrls, preview.imageUrl)
    if (!preview.taskId || preview.taskId === '-' || imageURLs.length === 0) {
      setMessage('当前展示区还没有图片详情。')
      return
    }
    setPreviewDetailOpen(true)
  }

  function handleApplyPreviewDetail() {
    const parsed = parseTaskRequestParams(preview.requestParams)
    const promptText = parsed?.prompt?.trim() || preview.prompt.trim()

    setPrompt(promptText)
    if (typeof parsed?.style === 'string') {
      setStyle(parsed.style)
    }
    if (typeof parsed?.count === 'number' || typeof parsed?.count === 'string') {
      setCount(clampCount(Number(parsed.count)))
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
        setCustomWidth(sizeState.customWidth)
        setCustomHeight(sizeState.customHeight)
      }
    }

    const referenceImageUrl = parsed?.referenceImageUrl?.trim() || ''
    setReferenceImageURL(referenceImageUrl)
    setReferenceImageName('')
    setReferenceImageSource(referenceImageUrl ? 'network' : 'none')
    setShowNetworkImageInput(Boolean(referenceImageUrl))
    setActivePage('home')
    setSidebarOpen(false)
    setPreviewDetailOpen(false)
  }

  async function loadAdminData() {
    if (!isAdmin) {
      return
    }
    try {
      setBusy('refresh')
      const [stats, users, tasks, orders, audits, packages, actionLogs] = await Promise.all([
        api.adminStats(),
        api.adminListUsers(1, 8),
        api.adminListImageTasks(1, 8),
        api.adminListRechargeOrders(1, 8),
        api.adminListContentAudits(1, 8),
        api.adminListPackages(1, 50),
        api.adminListActionLogs(1, 20, 'billing_package'),
      ])
      setAdminStats(stats)
      setAdminUsers(users.items)
      setAdminTasks(tasks.items)
      setAdminOrders(orders.items)
      setAdminAudits(audits.items)
      setAdminActionLogs(actionLogs.items)
      setAdminPackages(packages.items.filter((item) => item.packageType === 'recharge'))
    } catch (error) {
      handleApiError(error, '后台数据加载失败')
    } finally {
      setBusy(null)
    }
  }

  function handleToggleAdmin() {
    setAdminOpen((current) => !current)
  }

  function handleExitAdmin() {
    setAdminOpen(false)
    setSidebarOpen(false)
    setSidebarCollapsed(false)
  }

  async function handleAdminToggleUserStatus(user: AdminUserItem) {
    try {
      setBusy('refresh')
      const nextStatus = user.status === 1 ? 2 : 1
      await api.adminUpdateUserStatus(user.id, nextStatus)
      await loadAdminData()
      setMessage(`${user.username} 已${nextStatus === 1 ? '启用' : '禁用'}。`)
    } catch (error) {
      handleApiError(error, '更新用户状态失败')
    } finally {
      setBusy(null)
    }
  }

  async function handleAdminAdjustUserPoints(user: AdminUserItem, delta: number) {
    try {
      setBusy('refresh')
      await api.adminAdjustPoints(user.id, delta, '后台快速调整')
      await loadAdminData()
      setMessage(`${user.username} 积分已调整 ${delta > 0 ? '+' : ''}${delta}。`)
    } catch (error) {
      handleApiError(error, '调整积分失败')
    } finally {
      setBusy(null)
    }
  }

  async function handleAdminSavePackage(packageId: string | null, payload: AdminPackagePayload) {
    try {
      setBusy('refresh')
      if (packageId) {
        await api.adminUpdatePackage(packageId, payload)
      } else {
        await api.adminCreatePackage(payload)
      }
      await loadAdminData()
      await loadPackages()
      setMessage(packageId ? '套餐已更新。' : '套餐已创建。')
    } catch (error) {
      handleApiError(error, '保存套餐失败')
    } finally {
      setBusy(null)
    }
  }

  function handleLogout() {
    clearStoredToken()
    setToken('')
    setCurrentUser(null)
    setMode('login')
    setAuthForm({
      email: '',
      code: '',
      username: '',
      account: '',
      password: '',
      confirmPassword: '',
    })
    setAuthFormSeed((current) => current + 1)
    setShowRegisterPassword(false)
    setActivePage('home')
    setSidebarCollapsed(false)
    setSidebarOpen(false)
    setAdminOpen(false)
    setAdminStats(null)
    setAdminUsers([])
    setAdminTasks([])
    setAdminOrders([])
    setAdminAudits([])
    setAdminActionLogs([])
    setAdminPackages([])
    setPoints(null)
    setHistory([])
    setHistoryPage(1)
    setHistoryTotal(0)
    setActivityLogs([])
    setActivityPage(1)
    setActivityTotal(0)
    setPointsLogItems([])
    setPointsLogPage(1)
    setPointsLogTotal(0)
    setCreditLogItems([])
    setCreditLogPage(1)
    setCreditLogTotal(0)
    setRechargeOrders([])
    setRechargeOrderPage(1)
    setRechargeOrderTotal(0)
    setPreview(emptyPreview)
    setPreviewImageIndex(0)
    setWechatSaveGuide({ open: false, imageUrl: '', title: '' })
    pollingTaskIdsRef.current.clear()
    setGenerationProgresses([])
    setPrompt('')
    setSize('1024x1024')
    setStyle('')
    setCount(1)
    setQuality('auto')
    setOutputFormat('default')
    setCustomWidth(1024)
    setCustomHeight(1024)
    setReferenceImageURL('')
    setReferenceImageName('')
    setReferenceImageSource('none')
    setShowNetworkImageInput(false)
    setPreviewDetailOpen(false)
    setChangePasswordForm({
      currentPassword: '',
      newPassword: '',
      confirmNewPassword: '',
    })
    setShowChangePassword(false)
    if (referenceImageInputRef.current) {
      referenceImageInputRef.current.value = ''
    }
  }

  function handleLogoutRequest() {
    if (window.confirm('确定要退出登录吗？')) {
      handleLogout()
    }
  }

  function handleApiError(error: unknown, fallback: string) {
    if (error instanceof APIError && error.status === 401) {
      handleLogout()
      setMessage('登录已过期，请重新登录。')
      return
    }
    setMessage(error instanceof Error ? error.message : fallback)
  }

  function navigateToPage(page: AppPage) {
    setActivePage(page)
    setSidebarOpen(false)
  }

  function handleTogglePromptFavorite(promptId: string) {
    setFavoritePromptIds((current) =>
      current.includes(promptId) ? current.filter((item) => item !== promptId) : [...current, promptId],
    )
  }

  function handleUsePromptItem(item: PromptGalleryItem) {
    setPrompt(item.prompt)
    setActivePage('home')
    setSidebarOpen(false)
  }

  async function handleLocalReferenceImageChange(event: ChangeEvent<HTMLInputElement>) {
    const file = event.target.files?.[0]
    event.target.value = ''
    if (!file) {
      return
    }

    try {
      setReferenceImageUploading(true)
      setBusy('upload-reference')
      const result: UploadImageResponse = await api.uploadReferenceImage(file)
      setReferenceImageURL(result.imageUrl)
      setReferenceImageName(result.fileName || file.name)
      setReferenceImageSource('local')
      setShowNetworkImageInput(false)
    } catch (error) {
      handleApiError(error, '上传图片失败')
    } finally {
      setReferenceImageUploading(false)
      setBusy(null)
    }
  }

  function handleNetworkReferenceImageInput(value: string) {
    setReferenceImageURL(value)
    setReferenceImageSource(value.trim() ? 'network' : 'none')
  }

  function handleClearReferenceImage() {
    setReferenceImageURL('')
    setReferenceImageName('')
    setReferenceImageSource('none')
    setShowNetworkImageInput(false)
    if (referenceImageInputRef.current) {
      referenceImageInputRef.current.value = ''
    }
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
    setCustomWidth(1024)
    setCustomHeight(1024)
    setReferenceImageURL('')
    setReferenceImageName('')
    setReferenceImageSource('none')
    setShowNetworkImageInput(false)
    if (referenceImageInputRef.current) {
      referenceImageInputRef.current.value = ''
    }
  }

  function handleSubmitPromptDraft() {
    const title = promptSubmission.title.trim()
    const promptText = promptSubmission.prompt.trim()
    if (!title || !promptText) {
      setMessage('请先填写提示词标题和正文。')
      return
    }

    const now = new Date().toISOString()
    setSubmittedPrompts((current) => [
      {
        id: `local-${Date.now()}`,
        title,
        prompt: promptText,
        previewUrl: promptSubmission.previewUrl.trim(),
        author: currentUser?.username ?? '我',
        status: 'pending',
        favorites: 0,
        createdAt: now,
      },
      ...current,
    ])
    setPromptSubmission({ title: '', prompt: '', previewUrl: '' })
    setMessage('提示词已提交，等待审核。')
  }

  const previewImageURLs = normalizeImageURLs(preview.imageUrls, preview.imageUrl)

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

  if (!isAuthed) {
    return (
      <div className="min-h-screen bg-slate-950 text-slate-100">
        <TopNotice message={message} onDismiss={() => setMessage('')} />
        <main className="flex min-h-screen items-center justify-center px-4 py-8">
          <section className="w-full max-w-md rounded-lg border border-slate-800 bg-slate-900/85 p-6 shadow-panel">
            <div className="text-center">
              <div className="mx-auto inline-flex items-center gap-3">
                <div className="flex h-12 w-12 items-center justify-center rounded-md border border-slate-800 bg-slate-950 text-violet-300">
                  <Image className="h-6 w-6" />
                </div>
                <span className="text-2xl font-semibold tracking-normal text-slate-50">image2</span>
              </div>
            </div>

            <div className="mt-8">
              <form
                key={`${mode}-${authFormSeed}`}
                autoComplete="off"
                className="space-y-4"
                onSubmit={(event) => {
                  event.preventDefault()
                  void handleAuthSubmit()
                }}
              >
                {mode === 'register' ? (
                  <>
                    <Field label="邮箱" icon={<Mail className="h-4 w-4" />}>
                      <input
                        name={`register_email_${authFormSeed}`}
                        autoComplete="off"
                        value={authForm.email}
                        onChange={(event) => setAuthForm((current) => ({ ...current, email: event.target.value }))}
                        className="w-full rounded-md border border-slate-700 bg-slate-950 px-3 py-2 text-slate-100 outline-none transition placeholder:text-slate-500 focus:border-violet-400"
                        placeholder="请输入邮箱"
                      />
                    </Field>
                    <div className="grid gap-3 sm:grid-cols-[1fr_auto]">
                      <Field label="验证码" icon={<KeyRound className="h-4 w-4" />}>
                        <input
                          name={`register_code_${authFormSeed}`}
                          autoComplete="one-time-code"
                          value={authForm.code}
                          onChange={(event) => setAuthForm((current) => ({ ...current, code: event.target.value }))}
                          className="w-full rounded-md border border-slate-700 bg-slate-950 px-3 py-2 text-slate-100 outline-none transition placeholder:text-slate-500 focus:border-violet-400"
                          placeholder="请输入验证码"
                        />
                      </Field>
                      <div className="flex items-end">
                        <button
                          type="button"
                          onClick={handleSendCode}
                          disabled={busy === 'send-code'}
                          className="inline-flex h-10 w-full items-center justify-center gap-2 rounded-md border border-slate-700 bg-slate-950 px-3 text-sm text-slate-200 transition hover:border-violet-400 hover:text-white disabled:cursor-not-allowed disabled:opacity-60"
                        >
                          {busy === 'send-code' ? (
                            <LoaderCircle className="h-4 w-4 animate-spin" />
                          ) : (
                            <Send className="h-4 w-4" />
                          )}
                          发送验证码
                        </button>
                      </div>
                    </div>
                    <Field label="用户名" icon={<UserRoundPlus className="h-4 w-4" />}>
                      <input
                        name={`register_username_${authFormSeed}`}
                        autoComplete="off"
                        value={authForm.username}
                        onChange={(event) => setAuthForm((current) => ({ ...current, username: event.target.value }))}
                        className="w-full rounded-md border border-slate-700 bg-slate-950 px-3 py-2 text-slate-100 outline-none transition placeholder:text-slate-500 focus:border-violet-400"
                        placeholder="请输入用户名"
                      />
                    </Field>
                    <Field label="密码" icon={<KeyRound className="h-4 w-4" />}>
                      <div className="relative">
                        <input
                          name={`register_password_${authFormSeed}`}
                          autoComplete="new-password"
                          type={showRegisterPassword ? 'text' : 'password'}
                          value={authForm.password}
                          onChange={(event) => setAuthForm((current) => ({ ...current, password: event.target.value }))}
                          className="w-full rounded-md border border-slate-700 bg-slate-950 py-2 pl-3 pr-10 text-slate-100 outline-none transition placeholder:text-slate-500 focus:border-violet-400"
                          placeholder="请设置至少 8 位密码"
                        />
                        <button
                          type="button"
                          onClick={() => setShowRegisterPassword((current) => !current)}
                          aria-label={showRegisterPassword ? '隐藏密码' : '显示密码'}
                          className="absolute inset-y-0 right-0 flex w-10 items-center justify-center text-slate-400 transition hover:text-slate-200"
                        >
                          {showRegisterPassword ? <EyeOff className="h-4 w-4" /> : <Eye className="h-4 w-4" />}
                        </button>
                      </div>
                    </Field>
                    <Field label="再次输入密码" icon={<KeyRound className="h-4 w-4" />}>
                      <div className="relative">
                        <input
                          name={`register_confirm_password_${authFormSeed}`}
                          autoComplete="new-password"
                          type={showRegisterPassword ? 'text' : 'password'}
                          value={authForm.confirmPassword}
                          onChange={(event) =>
                            setAuthForm((current) => ({ ...current, confirmPassword: event.target.value }))
                          }
                          className="w-full rounded-md border border-slate-700 bg-slate-950 py-2 pl-3 pr-10 text-slate-100 outline-none transition placeholder:text-slate-500 focus:border-violet-400"
                          placeholder="请再次输入密码"
                        />
                        <button
                          type="button"
                          onClick={() => setShowRegisterPassword((current) => !current)}
                          aria-label={showRegisterPassword ? '隐藏密码' : '显示密码'}
                          className="absolute inset-y-0 right-0 flex w-10 items-center justify-center text-slate-400 transition hover:text-slate-200"
                        >
                          {showRegisterPassword ? <EyeOff className="h-4 w-4" /> : <Eye className="h-4 w-4" />}
                        </button>
                      </div>
                    </Field>
                  </>
                ) : (
                  <>
                    <Field label="账号（用户名或邮箱）" icon={<UserRound className="h-4 w-4" />}>
                      <input
                        name={`login_account_${authFormSeed}`}
                        autoComplete="off"
                        value={authForm.account}
                        onChange={(event) => setAuthForm((current) => ({ ...current, account: event.target.value }))}
                        className="w-full rounded-md border border-slate-700 bg-slate-950 px-3 py-2 text-slate-100 outline-none transition placeholder:text-slate-500 focus:border-violet-400"
                        placeholder="请输入用户名或邮箱"
                      />
                    </Field>
                    <Field label="密码" icon={<KeyRound className="h-4 w-4" />}>
                      <input
                        name={`login_password_${authFormSeed}`}
                        autoComplete="new-password"
                        type="password"
                        value={authForm.password}
                        onChange={(event) => setAuthForm((current) => ({ ...current, password: event.target.value }))}
                        className="w-full rounded-md border border-slate-700 bg-slate-950 px-3 py-2 text-slate-100 outline-none transition placeholder:text-slate-500 focus:border-violet-400"
                        placeholder="请输入密码"
                      />
                    </Field>
                  </>
                )}

                <button
                  type="submit"
                  disabled={busy === 'auth'}
                  className="inline-flex w-full items-center justify-center gap-2 rounded-md bg-violet-500 px-4 py-3 font-medium text-slate-950 transition hover:bg-violet-400 disabled:cursor-not-allowed disabled:opacity-70"
                >
                  {busy === 'auth' ? (
                    <LoaderCircle className="h-4 w-4 animate-spin" />
                  ) : mode === 'register' ? (
                    <UserRoundPlus className="h-4 w-4" />
                  ) : (
                    <LogIn className="h-4 w-4" />
                  )}
                  {mode === 'register' ? '注册' : '登录'}
                </button>
              </form>

              <div className="mt-5 border-t border-slate-800 pt-4 text-center text-sm text-slate-400">
                {mode === 'register' ? (
                  canRegister ? (
                    <>
                      已有账号？
                      <button
                        type="button"
                        onClick={() => {
                          setMode('login')
                          setMessage('')
                          setShowRegisterPassword(false)
                        }}
                        className="ml-1 font-medium text-violet-300 transition hover:text-violet-200"
                      >
                        返回登录
                      </button>
                    </>
                  ) : (
                    <span>注册功能暂时关闭。</span>
                  )
                ) : (
                  canRegister ? (
                    <>
                      还没有账号？
                      <button
                        type="button"
                        onClick={() => {
                          setMode('register')
                          setMessage('')
                          setShowRegisterPassword(false)
                        }}
                        className="ml-1 font-medium text-violet-300 transition hover:text-violet-200"
                      >
                        立即注册
                      </button>
                    </>
                  ) : (
                    <span>当前仅支持登录。</span>
                  )
                )}
              </div>
            </div>
          </section>
        </main>
      </div>
    )
  }

  if (isAuthed && adminOpen) {
    return (
      <AdminConsole
        currentUser={currentUser}
        onBackToApp={handleExitAdmin}
        onLogout={handleLogoutRequest}
      />
    )
  }

  return (
    <div className="min-h-screen w-full max-w-[100dvw] overflow-x-hidden bg-slate-950 text-slate-100">
      <TopNotice message={message} onDismiss={() => setMessage('')} />
      <div className="flex min-h-screen w-full max-w-[100dvw] overflow-x-hidden">
        {sidebarOpen ? (
          <button
            type="button"
            aria-label="关闭侧边栏"
            onClick={() => setSidebarOpen(false)}
            className="fixed inset-0 z-30 bg-slate-950/70 backdrop-blur-sm lg:hidden"
          />
        ) : null}

        <AppSidebar
          activePage={activePage}
          collapsed={sidebarCollapsed}
          open={sidebarOpen}
          onNavigate={navigateToPage}
          onToggleCollapsed={() => {
            setSidebarCollapsed((current) => !current)
          }}
          onCloseMobile={() => {
            setSidebarOpen(false)
          }}
        />

        <div className="flex min-w-0 flex-1 flex-col overflow-x-hidden">
          <AppTopBar
            activePage={activePage}
            sidebarOpen={sidebarOpen}
            currentUser={currentUser}
            onOpenSidebar={() => {
              setSidebarOpen(true)
            }}
            onNavigate={navigateToPage}
            onToggleAdmin={handleToggleAdmin}
            adminOpen={adminOpen}
            onLogout={handleLogoutRequest}
          />

          <main className="min-w-0 flex-1 overflow-x-hidden px-3 py-4 sm:px-4 lg:px-6">
            {isAdmin && adminOpen ? (
              <div className="mb-4">
                <AdminPanel
                  stats={adminStats}
                  users={adminUsers}
                  tasks={adminTasks}
                  orders={adminOrders}
                  audits={adminAudits}
                  actionLogs={adminActionLogs}
                  packages={adminPackages}
                  busy={busy === 'refresh'}
                  onRefresh={() => void loadAdminData()}
                  onToggleUserStatus={(user) => void handleAdminToggleUserStatus(user)}
                  onAdjustUserPoints={(user, delta) => void handleAdminAdjustUserPoints(user, delta)}
                  onSavePackage={(packageId, payload) => void handleAdminSavePackage(packageId, payload)}
                />
              </div>
            ) : null}

            {activePage === 'home' ? (
              <div className="space-y-4">
                <div className="grid min-w-0 grid-cols-1 gap-4 xl:grid-cols-[380px_minmax(0,1fr)_360px]">
                  <section className="min-w-0 overflow-hidden rounded-lg border border-slate-800 bg-slate-900/60 p-3 shadow-panel sm:p-4">
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
                        <div className="mt-1 text-base font-semibold text-slate-100">{points ? points.balance : 0}</div>
                      </div>
                      <div className="rounded-md border border-slate-800 bg-slate-950 px-3 py-2 text-slate-300">
                        <div className="text-xs text-slate-500">可用次数</div>
                        <div className="mt-1 text-base font-semibold text-slate-100">
                          {points ? points.totalCredits : 0}
                        </div>
                      </div>
                    </div>

                    <textarea
                      value={prompt}
                      onChange={(event) => setPrompt(event.target.value)}
                      className="mt-4 min-h-32 w-full resize-y rounded-md border border-slate-700 bg-slate-900 px-3 py-3 text-sm leading-6 text-slate-100 outline-none transition placeholder:text-slate-500 focus:border-violet-400"
                      placeholder="请输入提示词"
                    />

                    <div className="mt-3 grid grid-cols-2 gap-2">
                      <button
                        type="button"
                        onClick={() => referenceImageInputRef.current?.click()}
                        disabled={referenceImageUploading || busy === 'upload-reference'}
                        className="inline-flex items-center justify-center gap-2 rounded-md border border-slate-700 bg-slate-900 px-3 py-2 text-sm text-slate-200 transition hover:border-violet-400 hover:text-white disabled:cursor-not-allowed disabled:opacity-60"
                      >
                        {referenceImageUploading || busy === 'upload-reference' ? (
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
                      accept="image/*"
                      className="hidden"
                      onChange={handleLocalReferenceImageChange}
                    />

                    {showNetworkImageInput ? (
                      <label className="mt-3 block">
                        <span className="mb-2 block text-sm text-slate-400">网络图片链接</span>
                        <input
                          value={referenceImageURL}
                          onChange={(event) => handleNetworkReferenceImageInput(event.target.value)}
                          className="h-11 w-full rounded-md border border-slate-700 bg-slate-900 px-3 text-sm text-slate-100 outline-none transition placeholder:text-slate-500 focus:border-violet-400"
                          placeholder="请输入图片链接"
                        />
                      </label>
                    ) : null}

                    <div className="mt-3 rounded-lg border border-slate-800 bg-slate-950 p-3">
                      {referenceImageURL.trim() ? (
                        <div className="flex items-start gap-3">
                          <div className="h-16 w-16 shrink-0 overflow-hidden rounded-md border border-slate-800 bg-slate-900">
                            <img
                              src={referenceImageURL}
                              alt={referenceImageName || '参考图片'}
                              className="h-full w-full object-cover"
                            />
                          </div>
                          <div className="min-w-0 flex-1">
                            <div className="flex items-start justify-between gap-2">
                              <div className="min-w-0">
                                <div className="truncate text-sm text-slate-100">
                                  {referenceImageSource === 'local'
                                    ? referenceImageName || '本地图片'
                                    : '网络图片'}
                                </div>
                                <div className="mt-1 break-all text-xs text-slate-500">
                                  {referenceImageURL.trim()}
                                </div>
                              </div>
                              <button
                                type="button"
                                onClick={handleClearReferenceImage}
                                className="inline-flex h-8 items-center justify-center rounded-md border border-slate-700 bg-slate-900 px-2 text-xs text-slate-300 transition hover:border-rose-400 hover:text-rose-200"
                              >
                                清除
                              </button>
                            </div>
                          </div>
                        </div>
                      ) : (
                        <div className="text-sm text-slate-500">未添加参考图片。</div>
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
                            min={256}
                            max={4096}
                            step={64}
                            value={customWidth}
                            onChange={(event) => setCustomWidth(Number(event.target.value))}
                            className="h-11 w-full rounded-md border border-slate-700 bg-slate-900 px-3 text-sm text-slate-100 outline-none transition focus:border-violet-400"
                          />
                        </label>
                        <label className="block">
                          <span className="mb-2 block text-sm text-slate-400">自定义高度</span>
                          <input
                            type="number"
                            min={256}
                            max={4096}
                            step={64}
                            value={customHeight}
                            onChange={(event) => setCustomHeight(Number(event.target.value))}
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
                            <option key={option.value} value={option.value}>
                              {option.label}
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
                        onClick={handleCreateTask}
                        disabled={!isAuthed || busy === 'create-task' || !prompt.trim()}
                        className="inline-flex h-11 w-full items-center justify-center gap-2 rounded-md bg-violet-500 px-4 text-sm font-medium text-slate-950 transition hover:bg-violet-400 disabled:cursor-not-allowed disabled:opacity-60"
                      >
                        {busy === 'create-task' ? (
                          <LoaderCircle className="h-4 w-4 animate-spin" />
                        ) : (
                          <Sparkles className="h-4 w-4" />
                        )}
                        生成图片
                      </button>
                    </div>
                  </section>

                  <section className="min-w-0 overflow-hidden rounded-lg border border-slate-800 bg-slate-900/60 p-3 shadow-panel sm:p-4">
                    <div className="flex items-center justify-between">
                      <div>
                        <h2 className="text-lg font-medium text-slate-50">展示区</h2>
                        <p className="mt-1 text-sm text-slate-400">长按图片可选择放大预览</p>
                      </div>
                      <button
                        type="button"
                        onClick={handleOpenPreviewDetail}
                        className="inline-flex items-center gap-2 rounded-md border border-slate-700 bg-slate-950 px-3 py-2 text-sm text-slate-200 transition hover:border-violet-400 hover:text-white"
                      >
                        <Eye className="h-4 w-4" />
                        提示词详情
                      </button>
                    </div>

                    <PreviewImageStage
                      preview={preview}
                  imageURLs={previewImageURLs}
                  activeIndex={previewImageIndex}
                  onPrevious={handlePreviousPreviewImage}
                  onNext={handleNextPreviewImage}
                  onSaveCurrent={handleSaveCurrentImage}
                  onSelectThumbnail={setPreviewImageIndex}
                />

                <TaskDetailPanel
                  preview={preview}
                  progress={generationProgresses.find((item) => item.taskId === preview.taskId) ?? null}
                />
              </section>

                  <aside className="min-w-0 space-y-4 overflow-hidden">
                    <TaskHistoryPanel
                      items={history}
                      page={historyPage}
                      pageSize={HISTORY_PAGE_SIZE}
                      total={historyTotal}
                      onSelect={handleSelectTask}
                      onPageChange={(page) => void loadHistoryPage(page)}
                    />
                  </aside>
                </div>
              </div>
            ) : null}

            {activePage === 'orders' ? (
              <div className="space-y-4">
                <ActivityLogPanel
                  items={activityLogs}
                  page={activityPage}
                  pageSize={ACTIVITY_PAGE_SIZE}
                  total={activityTotal}
                  onPageChange={(page) => void loadActivityPage(page)}
                />
              </div>
            ) : null}

            {activePage === 'profile' ? (
              <div className="space-y-4">
                <div className="grid gap-4 xl:grid-cols-3">
                  <UserSummaryPanel
                    currentUser={currentUser}
                    points={points}
                    busy={busy}
                    onCheckin={handleCheckin}
                  />
                  <AccountOverviewPanel points={points} />
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
            ) : null}

            {activePage === 'flows' ? (
              <div className="space-y-4">
                <FlowLogsPanel
                  pointsLogs={pointsLogItems}
                  creditLogs={creditLogItems}
                  pointsPage={pointsLogPage}
                  creditPage={creditLogPage}
                  pageSize={FLOW_LOG_PAGE_SIZE}
                  pointsTotal={pointsLogTotal}
                  creditTotal={creditLogTotal}
                  onPointsPageChange={(page) => void loadPointsLogPage(page)}
                  onCreditPageChange={(page) => void loadCreditLogPage(page)}
                />
              </div>
            ) : null}

            {activePage === 'recharge' ? (
              <div className="space-y-4">
                <div className="grid gap-4 xl:grid-cols-[minmax(0,1fr)_420px]">
                  <section className="space-y-4 rounded-lg border border-slate-800 bg-slate-900/70 p-4 shadow-panel">
                    <CustomRechargePanel
                      count={customRechargeCount}
                      unitPriceCents={billingSettings?.customRechargeUnitPriceCents ?? 20}
                      busy={busy}
                      onCountChange={setCustomRechargeCount}
                      onSubmit={handleCustomRecharge}
                    />
                    <PackageSection
                      title="充值购买次数"
                      description="也可以选择后台配置的固定档位，部分档位会额外赠送积分。"
                      icon={<CreditCard className="h-4 w-4 text-violet-300" />}
                      items={packages}
                      busy={busy}
                      emptyText="暂无充值档位。"
                      actionLabel="下单"
                      busyPrefix="purchase:"
                      onAction={handlePurchase}
                      renderMeta={(item) => (
                        <>
                          <span>{item.creditsAmount} 次</span>
                          <span>{(item.amountCents / 100).toFixed(2)} 元</span>
                          {item.bonusPoints > 0 ? <span>赠送 {item.bonusPoints} 积分</span> : null}
                        </>
                      )}
                    />
                  </section>
                  <RechargeOrderPanel
                    orders={rechargeOrders}
                    page={rechargeOrderPage}
                    pageSize={RECHARGE_ORDER_PAGE_SIZE}
                    total={rechargeOrderTotal}
                    busy={busy}
                    onConfirm={handleConfirmRechargeOrder}
                    onPageChange={(page) => void loadRechargeOrderPage(page)}
                  />
                </div>
              </div>
            ) : null}

            {activePage === 'prompts' ? (
              <PromptPlaza
                items={promptGalleryItems}
                favoriteIds={favoritePromptIds}
                submission={promptSubmission}
                isAdmin={isAdmin}
                onSubmissionChange={setPromptSubmission}
                onSubmit={handleSubmitPromptDraft}
                onFavorite={handleTogglePromptFavorite}
                onUse={handleUsePromptItem}
                onBackHome={() => navigateToPage('home')}
              />
            ) : null}
          </main>
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
    </div>
  )
}

function WechatSaveGuide({
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

function PreviewDetailDialog({
  preview,
  onApply,
  onClose,
}: {
  preview: ImageTaskDetailResponse
  onApply: () => void
  onClose: () => void
}) {
  const requestOptions = buildPreviewRequestOptions(preview)

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
                {preview.prompt || '未填写'}
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

function Field({
  label,
  icon,
  children,
}: {
  label: string
  icon?: ReactNode
  children: ReactNode
}) {
  return (
    <label className="block">
      <span className="mb-2 flex items-center gap-2 text-sm text-slate-400">
        {icon}
        {label}
      </span>
      {children}
    </label>
  )
}

function AppSidebar({
  activePage,
  collapsed,
  open,
  onNavigate,
  onToggleCollapsed,
  onCloseMobile,
}: {
  activePage: AppPage
  collapsed: boolean
  open: boolean
  onNavigate: (page: AppPage) => void
  onToggleCollapsed: () => void
  onCloseMobile: () => void
}) {
  const railWidthClass = collapsed ? 'lg:w-16' : 'lg:w-72'
  const labelClass = collapsed ? 'lg:hidden' : ''
  return (
    <aside
      className={`fixed inset-y-0 left-0 z-40 flex w-72 flex-col border-r border-slate-800 bg-slate-950 transition-all duration-200 lg:static lg:z-10 lg:h-screen lg:translate-x-0 ${railWidthClass} ${
        open ? 'translate-x-0' : '-translate-x-full lg:translate-x-0'
      }`}
    >
      <div className="flex h-20 items-center gap-2 border-b border-slate-800 px-3">
        <button
          type="button"
          onClick={onCloseMobile}
          className="inline-flex h-12 w-12 shrink-0 items-center justify-center rounded-md border border-slate-800 bg-slate-900 text-slate-200 transition hover:border-violet-400 hover:text-white lg:hidden"
          title="关闭侧边栏"
        >
          <XCircle className="h-5 w-5" />
        </button>
        <button
          type="button"
          onClick={onToggleCollapsed}
          className="hidden h-12 w-12 shrink-0 items-center justify-center rounded-md border border-slate-800 bg-slate-900 text-slate-200 transition hover:border-violet-400 hover:text-white lg:inline-flex"
          title={collapsed ? '展开侧边栏' : '收起侧边栏'}
        >
          {collapsed ? <PanelLeftOpen className="h-5 w-5" /> : <PanelLeftClose className="h-5 w-5" />}
        </button>
        <div className={`min-w-0 ${labelClass || 'block'}`}>
          <div className={`flex items-center gap-2 text-sm text-slate-400 ${collapsed ? 'lg:hidden' : ''}`}>
            <Image className="h-4 w-4 text-violet-300" />
            <span>AI 生图</span>
          </div>
          <div className={`truncate text-sm font-medium text-slate-100 ${collapsed ? 'lg:hidden' : ''}`}>
            用户工作台
          </div>
        </div>
      </div>

      <nav className={`flex-1 space-y-3 ${collapsed ? 'px-2 py-4' : 'px-3 py-6'}`}>
        {appNavItems.map((item) => {
          const active = activePage === item.page
          return (
            <button
              key={item.page}
              type="button"
              onClick={() => onNavigate(item.page)}
              className={`flex h-14 w-full items-center gap-3 rounded-md px-3 text-left text-sm transition ${
                active
                  ? 'bg-violet-500 text-slate-950'
                  : 'text-slate-300 hover:bg-slate-900 hover:text-white'
              } ${collapsed ? 'lg:justify-center lg:px-0' : ''}`}
              title={item.label}
            >
              <span className="shrink-0">{item.icon}</span>
              <span className={labelClass}>{item.label}</span>
            </button>
          )
        })}
      </nav>
    </aside>
  )
}

function AppTopBar({
  activePage,
  sidebarOpen,
  currentUser,
  onOpenSidebar,
  onNavigate,
  onToggleAdmin,
  adminOpen,
  onLogout,
}: {
  activePage: AppPage
  sidebarOpen: boolean
  currentUser: UserInfo | null
  onOpenSidebar: () => void
  onNavigate: (page: AppPage) => void
  onToggleAdmin: () => void
  adminOpen: boolean
  onLogout: () => void
}) {
  return (
    <header className="sticky top-0 z-20 max-w-[100dvw] overflow-hidden border-b border-slate-800 bg-slate-950/95 px-2 py-2 backdrop-blur sm:px-4 lg:px-6">
      <div className="grid min-w-0 grid-cols-[minmax(0,1fr)_auto] items-center gap-2">
        <div className="flex min-w-0 items-center gap-2 overflow-x-auto pr-1 [scrollbar-width:none] [&::-webkit-scrollbar]:hidden">
          <button
            type="button"
            onClick={onOpenSidebar}
            className={`inline-flex h-10 w-10 shrink-0 items-center justify-center rounded-md border border-slate-800 bg-slate-900 text-slate-200 transition hover:border-violet-400 hover:text-white lg:hidden ${
              sidebarOpen ? 'hidden' : ''
            }`}
            title="打开侧边栏"
          >
            <Menu className="h-4 w-4" />
          </button>
          <nav className="flex flex-nowrap items-center gap-1 whitespace-nowrap text-sm font-medium sm:gap-2">
            <button
              type="button"
              onClick={() => onNavigate('home')}
              className={`rounded-md px-2.5 py-2 transition sm:px-3 ${
                activePage === 'home'
                  ? 'text-slate-50'
                  : 'text-slate-400 hover:text-slate-100'
              }`}
            >
              首页
            </button>
            <button
              type="button"
              onClick={() => onNavigate('prompts')}
              className={`rounded-md px-2.5 py-2 transition sm:px-3 ${
                activePage === 'prompts'
                  ? 'text-slate-50'
                  : 'text-slate-400 hover:text-slate-100'
              }`}
            >
              提示词广场
            </button>
          </nav>
        </div>

        <div className="flex min-w-0 shrink-0 items-center gap-1 text-sm sm:gap-2">
          {currentUser?.role === 'admin' && !adminOpen ? (
            <button
              type="button"
              onClick={onToggleAdmin}
              className="inline-flex shrink-0 items-center gap-1 rounded-md border border-emerald-500/40 bg-emerald-500/10 px-2 py-2 text-emerald-200 transition hover:border-emerald-400 hover:text-white sm:gap-2 sm:px-3"
            >
              <ShieldCheck className="h-4 w-4" />
              后台
            </button>
          ) : null}
          <span
            className="max-w-[22vw] truncate rounded-md border border-slate-800 bg-slate-900 px-2 py-2 text-slate-300 min-[380px]:max-w-[28vw] sm:max-w-none sm:px-3"
            title={currentUser ? currentUser.username : '已登录'}
          >
            {currentUser ? currentUser.username : '已登录'}
          </span>
          <button
            type="button"
            onClick={onLogout}
            className="inline-flex shrink-0 items-center gap-1 rounded-md border border-slate-700 bg-slate-900 px-2 py-2 text-slate-200 transition hover:border-slate-500 hover:text-white sm:gap-2 sm:px-3"
          >
            <LogOut className="h-4 w-4" />
            退出
          </button>
        </div>
      </div>
    </header>
  )
}
function PageHeader({
  title,
  description,
  action,
}: {
  title: string
  description: string
  action?: ReactNode
}) {
  return (
    <div className="flex flex-col gap-3 rounded-lg border border-slate-800 bg-slate-900/70 p-4 shadow-panel sm:flex-row sm:items-center sm:justify-between">
      <div>
        <h1 className="text-xl font-semibold text-slate-50">{title}</h1>
        <p className="mt-1 text-sm text-slate-400">{description}</p>
      </div>
      {action}
    </div>
  )
}

function UserSummaryPanel({
  currentUser,
  points,
  busy,
  onCheckin,
}: {
  currentUser: UserInfo | null
  points: PointsResponse | null
  busy: BusyAction
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
            <span className="shrink-0 rounded-md border border-violet-500/30 bg-violet-500/10 px-2 py-1 text-xs text-violet-200">
              {getUserRoleLabel(currentUser?.role ?? 'user')}
            </span>
          </div>
        </div>

        <div className="grid grid-cols-2 gap-3">
          <MiniStat label="当前积分" value={points ? String(points.balance) : '0'} />
          <MiniStat label="可用次数" value={points ? String(points.totalCredits) : '0'} />
        </div>

        <button
          type="button"
          onClick={onCheckin}
          disabled={busy === 'checkin'}
          className="inline-flex w-full items-center justify-center gap-2 rounded-md border border-slate-700 bg-slate-950 px-4 py-3 font-medium text-slate-100 transition hover:border-violet-400 hover:text-white disabled:cursor-not-allowed disabled:opacity-50"
        >
          {busy === 'checkin' ? <LoaderCircle className="h-4 w-4 animate-spin" /> : <BadgeCheck className="h-4 w-4" />}
          每日签到
        </button>
      </div>
    </section>
  )
}

function AccountOverviewPanel({ points }: { points: PointsResponse | null }) {
  return (
    <section className="rounded-lg border border-slate-800 bg-slate-900/70 p-4 shadow-panel">
      <div className="flex items-center justify-end">
        <Wallet className="h-5 w-5 text-violet-300" />
      </div>

      <div className="mt-4 grid grid-cols-2 gap-3 text-sm">
        <MiniStat label="当前积分" value={points ? String(points.balance) : '0'} />
        <MiniStat label="可用次数" value={points ? String(points.totalCredits) : '0'} />
      </div>

      <div className="mt-3 grid gap-3 text-sm sm:grid-cols-3">
        <MiniStat label="免费次数" value={String(points?.freeCredits ?? 0)} />
        <MiniStat label="充值次数" value={String(points?.paidCredits ?? 0)} />
        <MiniStat label="连续签到" value={points ? String(points.streak) : '0'} />
      </div>

      <div className="mt-3 rounded-lg border border-slate-800 bg-slate-950 p-3 text-sm text-slate-400">
        <div className="flex items-center justify-between gap-3">
          <span>最近签到时间</span>
          <span className="text-right text-slate-200">
            {points?.lastCheckinAt ? formatDateTime(points.lastCheckinAt) : '-'}
          </span>
        </div>
      </div>
      </section>
    )
  }

function ChangePasswordPanel({
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
  onSubmit: () => void
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

function PanelPagination({
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

function CustomRechargePanel({
  count,
  unitPriceCents,
  busy,
  onCountChange,
  onSubmit,
}: {
  count: number
  unitPriceCents: number
  busy: BusyAction
  onCountChange: (value: number) => void
  onSubmit: () => void
}) {
  const normalizedCount = Math.max(1, Math.floor(count || 0))
  const amountCents = normalizedCount * unitPriceCents

  return (
    <div className="rounded-lg border border-slate-800 bg-slate-950 p-4">
      <div className="flex items-center justify-between gap-2">
        <div>
          <h3 className="text-sm font-medium text-slate-100">自定义充值</h3>
          <p className="mt-1 text-xs text-slate-500">按次数充值，自定义充值不赠送积分。</p>
        </div>
        <CreditCard className="h-4 w-4 text-violet-300" />
      </div>

      <div className="mt-3 grid gap-3 sm:grid-cols-[minmax(0,1fr)_auto] sm:items-end">
        <label className="block">
          <span className="mb-2 block text-xs text-slate-500">充值次数</span>
          <input
            type="number"
            min={1}
            step={1}
            value={count}
            onChange={(event) => onCountChange(Math.max(1, Number(event.target.value) || 1))}
            className="h-10 w-full rounded-md border border-slate-700 bg-slate-900 px-3 text-sm text-slate-100 outline-none transition focus:border-violet-400"
          />
        </label>
        <button
          type="button"
          onClick={onSubmit}
          disabled={busy === 'purchase:custom'}
          className="inline-flex h-10 items-center justify-center gap-2 rounded-md bg-violet-500 px-4 text-sm font-medium text-white transition hover:bg-violet-400 disabled:cursor-not-allowed disabled:opacity-60"
        >
          {busy === 'purchase:custom' ? <LoaderCircle className="h-4 w-4 animate-spin" /> : <CreditCard className="h-4 w-4" />}
          充值
        </button>
      </div>

      <div className="mt-3 flex flex-wrap gap-2 text-xs text-slate-500">
        <span>单次 {(unitPriceCents / 100).toFixed(2)} 元</span>
        <span>合计 {(amountCents / 100).toFixed(2)} 元</span>
        <span>赠送 0 积分</span>
      </div>
    </div>
  )
}

function RechargeOrderPanel({
  orders,
  page,
  pageSize,
  total,
  busy,
  onConfirm,
  onPageChange,
}: {
  orders: RechargeOrderItem[]
  page: number
  pageSize: number
  total: number
  busy: BusyAction
  onConfirm: (order: RechargeOrderItem) => void
  onPageChange: (page: number) => void
}) {
  return (
    <section className="rounded-lg border border-slate-800 bg-slate-900/70 p-4 shadow-panel">
      <div className="flex items-start justify-between gap-3">
        <div>
          <h3 className="text-sm font-medium text-slate-100">订单</h3>
          <p className="mt-1 text-xs text-slate-500">查看当前充值订单与支付状态。</p>
        </div>
        <CreditCard className="h-5 w-5 text-violet-300" />
      </div>

      <div className="mt-4 space-y-2">
        {orders.length ? (
          orders.map((order) => (
            <div key={order.orderNo} className="rounded-lg border border-slate-800 bg-slate-950 p-3">
              <div className="flex items-center justify-between gap-2">
                <div className="min-w-0">
                  <div className="truncate text-sm text-slate-100">{order.packageName}</div>
                  <div className="mt-1 text-xs text-slate-500">
                    {order.orderNo} · {order.rechargeType === 'custom' ? '自定义充值' : '套餐充值'} · {(order.amountCents / 100).toFixed(2)} 元
                  </div>
                </div>
                <span
                  className={`rounded-md px-2 py-1 text-xs ${
                    order.status === 'paid'
                      ? 'bg-emerald-500/15 text-emerald-300'
                      : order.status === 'pending'
                        ? 'bg-amber-500/15 text-amber-300'
                        : 'bg-slate-800 text-slate-300'
                  }`}
                >
                  {getRechargeOrderStatusLabel(order.status)}
                </span>
              </div>
              <div className="mt-2 flex flex-wrap gap-2 text-xs text-slate-500">
                <span>次数 {order.creditsAmount}</span>
                {order.bonusPoints > 0 ? <span>赠送 {order.bonusPoints} 积分</span> : null}
                {order.paymentChannel ? <span>渠道 {getPaymentChannelLabel(order.paymentChannel)}</span> : null}
              </div>
              {order.status === 'pending' ? (
                <button
                  type="button"
                  onClick={() => onConfirm(order)}
                  disabled={busy === `confirm-recharge:${order.orderNo}`}
                  className="mt-3 inline-flex items-center gap-2 rounded-md border border-violet-500/40 bg-violet-500/10 px-3 py-2 text-xs text-violet-200 transition hover:border-violet-400 hover:text-white disabled:cursor-not-allowed disabled:opacity-60"
                >
                  {busy === `confirm-recharge:${order.orderNo}` ? (
                    <LoaderCircle className="h-3.5 w-3.5 animate-spin" />
                  ) : (
                    <CheckCircle2 className="h-3.5 w-3.5" />
                  )}
                  模拟支付成功
                </button>
              ) : null}
            </div>
          ))
        ) : (
          <div className="rounded-lg border border-dashed border-slate-700 bg-slate-950 p-4 text-sm text-slate-400">
            暂无充值订单
          </div>
        )}
      </div>

      <PanelPagination page={page} pageSize={pageSize} total={total} onPageChange={onPageChange} />
    </section>
  )
}

function TaskHistoryPanel({
  items,
  page,
  pageSize,
  total,
  onSelect,
  onPageChange,
}: {
  items: ImageTaskItem[]
  page: number
  pageSize: number
  total: number
  onSelect: (item: ImageTaskItem) => void
  onPageChange: (page: number) => void
}) {
  return (
    <section className="rounded-lg border border-slate-800 bg-slate-900/70 p-4 shadow-panel">
      <div className="flex items-center justify-between">
        <div>
          <h2 className="text-lg font-medium text-slate-50">历史记录</h2>
          <p className="mt-1 text-sm text-slate-400">最近提交的生图任务，每页 {pageSize} 条</p>
        </div>
        <History className="h-5 w-5 text-violet-300" />
      </div>

      <div className="mt-4 space-y-3">
        {items.length ? (
          items.map((item) => (
            <button
              key={item.id}
              type="button"
              onClick={() => onSelect(item)}
              className="flex w-full items-start gap-3 rounded-lg border border-slate-800 bg-slate-950 p-3 text-left transition hover:border-violet-400/50 hover:bg-slate-900"
            >
              <div className="flex h-14 w-14 shrink-0 items-center justify-center overflow-hidden rounded-md border border-slate-800 bg-slate-900 text-xs font-medium text-slate-300">
                {item.imageUrl ? (
                  <img src={item.imageUrl} alt={item.prompt} className="h-full w-full object-cover" />
                ) : (
                  <span>{getImageTaskStatusLabel(item.status)}</span>
                )}
              </div>
              <div className="min-w-0 flex-1">
                <div className="flex items-center justify-between gap-2">
                  <span className="truncate text-sm font-medium text-slate-200">{item.prompt}</span>
                  <span
                    className={`rounded-md px-2 py-0.5 text-xs ${
                      item.status === 'success'
                        ? 'bg-emerald-500/15 text-emerald-300'
                        : item.status === 'failed'
                          ? 'bg-rose-500/15 text-rose-300'
                          : 'bg-amber-500/15 text-amber-300'
                    }`}
                  >
                    {getImageTaskStatusLabel(item.status)}
                  </span>
                </div>
                <div className="mt-2 flex items-center justify-between text-xs text-slate-500">
                  <span>{formatDateTime(item.createdAt)}</span>
                  <span>
                    {item.costCredits || item.costPoints} 次 / {getCreditSourceLabel(item.creditSource)}
                  </span>
                </div>
                {item.errorMessage ? <p className="mt-2 text-xs text-rose-300">{item.errorMessage}</p> : null}
              </div>
            </button>
          ))
        ) : (
          <div className="rounded-lg border border-dashed border-slate-700 bg-slate-950 p-4 text-sm text-slate-400">
            暂无任务记录。
          </div>
        )}
      </div>

      <PanelPagination page={page} pageSize={pageSize} total={total} onPageChange={onPageChange} />
    </section>
  )
}

function ActivityLogPanel({
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
      <div className="flex items-center justify-end">
        <ReceiptText className="h-5 w-5 text-violet-300" />
      </div>

      <div className="mt-4 space-y-3">
        {items.length ? (
          items.map((item) => (
            <div key={item.id} className="flex items-start gap-3 rounded-lg border border-slate-800 bg-slate-950 p-3">
              <div className="flex h-14 w-14 shrink-0 items-center justify-center overflow-hidden rounded-md border border-slate-800 bg-slate-900 text-xs font-medium text-slate-300">
                {item.imageUrl ? (
                  <img src={item.imageUrl} alt={item.title} className="h-full w-full object-cover" />
                ) : (
                  <span className="text-[11px]">{activityRecordShortLabel(item.recordType)}</span>
                )}
              </div>
              <div className="min-w-0 flex-1">
                <div className="flex items-start justify-between gap-2">
                  <div className="min-w-0">
                    <div className="truncate text-sm font-medium text-slate-200">{item.title}</div>
                    <p className="mt-2 line-clamp-2 text-xs leading-5 text-slate-500">{item.description}</p>
                  </div>
                  <span className="shrink-0 rounded-md border border-slate-700 bg-slate-900 px-2 py-0.5 text-xs text-slate-300">
                    {activityRecordLabel(item.recordType)}
                  </span>
                </div>
                <div className="mt-2 flex items-center justify-between text-xs text-slate-500">
                  <span>{activityRecordTimeLabel(item)}</span>
                  <span>{activityRecordValue(item)}</span>
                </div>
              </div>
            </div>
          ))
        ) : (
          <div className="rounded-lg border border-dashed border-slate-700 bg-slate-950 p-4 text-sm text-slate-400">
            暂无日志记录。
          </div>
        )}
      </div>

      <PanelPagination page={page} pageSize={pageSize} total={total} onPageChange={onPageChange} />
    </section>
  )
}

function FlowLogsPanel({
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
  const [detail, setDetail] = useState<FlowLogDetail>(null)
  const showingPoints = activeType === 'points'
  const currentPage = showingPoints ? pointsPage : creditPage
  const currentTotal = showingPoints ? pointsTotal : creditTotal
  const onCurrentPageChange = showingPoints ? onPointsPageChange : onCreditPageChange

  return (
    <section className="rounded-lg border border-slate-800 bg-slate-900/70 p-4 shadow-panel">
      <div className="flex flex-wrap items-center justify-between gap-3">
        <div className="inline-flex rounded-md border border-slate-800 bg-slate-950 p-1">
          <FlowLogTabButton
            active={activeType === 'points'}
            icon={<Wallet className="h-4 w-4" />}
            label="积分流水"
            onClick={() => setActiveType('points')}
          />
          <FlowLogTabButton
            active={activeType === 'credits'}
            icon={<Sparkles className="h-4 w-4" />}
            label="次数流水"
            onClick={() => setActiveType('credits')}
          />
        </div>
        <div className="text-xs text-slate-500">
          共 {currentTotal} 条
        </div>
      </div>

      <div className="mt-4 space-y-2">
        {showingPoints ? (
          pointsLogs.length ? (
            pointsLogs.map((item) => (
              <FlowLogListRow
                key={`points-${item.id}`}
                date={formatDateTime(item.createdAt)}
                valueLabel={`积分 ${formatSignedNumber(item.delta)}`}
                positive={item.delta >= 0}
                onDetail={() => setDetail({ type: 'points', item })}
              />
            ))
          ) : (
            <div className="rounded-lg border border-dashed border-slate-700 bg-slate-950 p-4 text-sm text-slate-400">
              暂无积分流水。
            </div>
          )
        ) : creditLogs.length ? (
          creditLogs.map((item) => {
            const delta = item.deltaFree + item.deltaPaid
            return (
              <FlowLogListRow
                key={`credits-${item.id}`}
                date={formatDateTime(item.createdAt)}
                valueLabel={`次数 ${formatSignedNumber(delta)}`}
                positive={delta >= 0}
                onDetail={() => setDetail({ type: 'credits', item })}
              />
            )
          })
        ) : (
          <div className="rounded-lg border border-dashed border-slate-700 bg-slate-950 p-4 text-sm text-slate-400">
            暂无次数流水。
          </div>
        )}
      </div>

      <PanelPagination
        page={currentPage}
        pageSize={pageSize}
        total={currentTotal}
        onPageChange={onCurrentPageChange}
      />

      {detail ? <FlowLogDetailDialog detail={detail} onClose={() => setDetail(null)} /> : null}
    </section>
  )
}

function FlowLogTabButton({
  active,
  icon,
  label,
  onClick,
}: {
  active: boolean
  icon: ReactNode
  label: string
  onClick: () => void
}) {
  return (
    <button
      type="button"
      onClick={onClick}
      className={`inline-flex items-center gap-2 rounded px-3 py-2 text-sm transition ${
        active
          ? 'bg-violet-500 text-slate-950 shadow-sm'
          : 'text-slate-400 hover:bg-slate-900 hover:text-slate-100'
      }`}
    >
      {icon}
      {label}
    </button>
  )
}

function FlowLogListRow({
  date,
  valueLabel,
  positive,
  onDetail,
}: {
  date: string
  valueLabel: string
  positive: boolean
  onDetail: () => void
}) {
  return (
    <div className="flex items-center justify-between gap-3 rounded-lg border border-slate-800 bg-slate-950 px-3 py-3">
      <div className="min-w-0 flex-1 text-sm text-slate-300">{date}</div>
      <div className={`shrink-0 text-sm font-medium ${positive ? 'text-emerald-300' : 'text-rose-300'}`}>
        {valueLabel}
      </div>
      <button
        type="button"
        onClick={onDetail}
        className="inline-flex shrink-0 items-center gap-1.5 rounded-md border border-slate-700 bg-slate-900 px-2.5 py-1.5 text-xs text-slate-300 transition hover:border-violet-400 hover:text-white"
      >
        <Eye className="h-3.5 w-3.5" />
        详情
      </button>
    </div>
  )
}

function FlowLogDetailDialog({ detail, onClose }: { detail: NonNullable<FlowLogDetail>; onClose: () => void }) {
  const rows =
    detail.type === 'points'
      ? getPointsLogDetailRows(detail.item)
      : getCreditLogDetailRows(detail.item)

  return (
    <div className="fixed inset-0 z-50 flex items-center justify-center bg-slate-950/80 px-4 py-6">
      <div className="max-h-[88vh] w-full max-w-lg overflow-hidden rounded-lg border border-slate-800 bg-slate-950 shadow-2xl">
        <div className="flex items-center justify-between gap-3 border-b border-slate-800 px-4 py-3">
          <div>
            <h3 className="text-base font-medium text-slate-50">
              {detail.type === 'points' ? '积分流水详情' : '次数流水详情'}
            </h3>
            <p className="mt-1 text-xs text-slate-500">查看该条流水的完整变动信息。</p>
          </div>
          <button
            type="button"
            onClick={onClose}
            className="inline-flex h-9 w-9 items-center justify-center rounded-md border border-slate-700 text-slate-300 transition hover:border-rose-400 hover:text-rose-200"
          >
            <XCircle className="h-4 w-4" />
          </button>
        </div>

        <div className="max-h-[68vh] overflow-y-auto p-4">
          <div className="grid gap-2">
            {rows.map((row) => (
              <div key={row.label} className="rounded-md border border-slate-800 bg-slate-900/70 px-3 py-2">
                <div className="text-xs text-slate-500">{row.label}</div>
                <div className="mt-1 break-words text-sm text-slate-100">{row.value}</div>
              </div>
            ))}
          </div>
        </div>

        <div className="flex justify-end border-t border-slate-800 px-4 py-3">
          <button
            type="button"
            onClick={onClose}
            className="rounded-md bg-violet-500 px-4 py-2 text-sm font-medium text-slate-950 transition hover:bg-violet-400"
          >
            知道了
          </button>
        </div>
      </div>
    </div>
  )
}

function getPointsLogDetailRows(item: PointsLogItem): Array<{ label: string; value: string }> {
  return [
    { label: '流水编号', value: item.id },
    { label: '流水类型', value: getPointsChangeTypeLabel(item.changeType) },
    { label: '日期', value: formatDateTime(item.createdAt) },
    { label: '积分变化', value: formatSignedNumber(item.delta) },
    { label: '变动前积分', value: String(item.beforeBalance) },
    { label: '变动后积分', value: String(item.afterBalance) },
    { label: '备注', value: item.remark || '-' },
  ]
}

function getCreditLogDetailRows(item: CreditLogItem): Array<{ label: string; value: string }> {
  const beforeTotal = item.beforeFree + item.beforePaid
  const afterTotal = item.afterFree + item.afterPaid
  const deltaTotal = item.deltaFree + item.deltaPaid
  return [
    { label: '流水编号', value: item.id },
    { label: '流水类型', value: getCreditChangeTypeLabel(item.changeType) },
    { label: '日期', value: formatDateTime(item.createdAt) },
    { label: '次数变化', value: formatSignedNumber(deltaTotal) },
    { label: '免费次数变化', value: `${item.beforeFree} → ${item.afterFree}（${formatSignedNumber(item.deltaFree)}）` },
    { label: '充值次数变化', value: `${item.beforePaid} → ${item.afterPaid}（${formatSignedNumber(item.deltaPaid)}）` },
    { label: '变动前总次数', value: String(beforeTotal) },
    { label: '变动后总次数', value: String(afterTotal) },
    { label: '备注', value: item.remark || '-' },
  ]
}

function PointsLogPanel({
  logs,
  page,
  pageSize,
  total,
  onPageChange,
}: {
  logs: PointsLogItem[]
  page: number
  pageSize: number
  total: number
  onPageChange: (page: number) => void
}) {
  return (
    <section className="rounded-lg border border-slate-800 bg-slate-900/70 p-4 shadow-panel">
      <div className="flex items-center justify-end">
        <Wallet className="h-5 w-5 text-violet-300" />
      </div>

      <div className="mt-4 space-y-3">
        {logs.length ? (
          logs.map((item) => <PointsLogRow key={item.id} item={item} />)
        ) : (
          <div className="rounded-lg border border-dashed border-slate-700 bg-slate-950 p-4 text-sm text-slate-400">
            暂无积分流水。
          </div>
        )}
      </div>

      <PanelPagination page={page} pageSize={pageSize} total={total} onPageChange={onPageChange} />
    </section>
  )
}

function CreditLogPanel({
  logs,
  page,
  pageSize,
  total,
  onPageChange,
}: {
  logs: CreditLogItem[]
  page: number
  pageSize: number
  total: number
  onPageChange: (page: number) => void
}) {
  return (
    <section className="rounded-lg border border-slate-800 bg-slate-900/70 p-4 shadow-panel">
      <div className="flex items-center justify-end">
        <Sparkles className="h-5 w-5 text-violet-300" />
      </div>

      <div className="mt-4 space-y-3">
        {logs.length ? (
          logs.map((item) => <CreditLogRow key={item.id} item={item} />)
        ) : (
          <div className="rounded-lg border border-dashed border-slate-700 bg-slate-950 p-4 text-sm text-slate-400">
            暂无次数流水。
          </div>
        )}
      </div>

      <PanelPagination page={page} pageSize={pageSize} total={total} onPageChange={onPageChange} />
    </section>
  )
}

function activityRecordLabel(recordType: UserActivityItem['recordType']) {
  switch (recordType) {
    case 'image_task':
      return '生图任务'
    case 'recharge_order':
      return '充值订单'
    case 'points_log':
      return '积分流水'
    case 'credit_log':
      return '次数流水'
    default:
      return '记录'
  }
}

function activityRecordShortLabel(recordType: UserActivityItem['recordType']) {
  switch (recordType) {
    case 'image_task':
      return '任务'
    case 'recharge_order':
      return '订单'
    case 'points_log':
      return '积分'
    case 'credit_log':
      return '次数'
    default:
      return '记录'
  }
}

function activityRecordValue(item: UserActivityItem) {
  switch (item.recordType) {
    case 'image_task':
      return `${item.creditsAmount ?? 0} 次`
    case 'recharge_order':
      return `${((item.amountCents ?? 0) / 100).toFixed(2)} 元`
    case 'points_log':
      return `${(item.deltaPoints ?? 0) >= 0 ? '+' : ''}${item.deltaPoints ?? 0} 积分`
    case 'credit_log':
      return `免费 ${(item.deltaFreeCredits ?? 0) >= 0 ? '+' : ''}${item.deltaFreeCredits ?? 0} / 充值 ${
        (item.deltaPaidCredits ?? 0) >= 0 ? '+' : ''
      }${item.deltaPaidCredits ?? 0}`
    default:
      return ''
  }
}

function activityRecordTimeLabel(item: UserActivityItem): string {
  const timeText = formatDateTime(item.createdAt)
  if (item.recordType !== 'image_task') {
    return timeText
  }

  const taskId = item.id.replace(/^task-/, '').trim()
  return taskId ? `任务 #${taskId} · ${timeText}` : timeText
}

function TaskDetailPanel({
  preview,
  progress,
}: {
  preview: ImageTaskDetailResponse
  progress: GenerationProgress | null
}) {
  const imageCount = normalizeImageURLs(preview.imageUrls, preview.imageUrl).length
  const rawStatus = progress?.status ?? preview.status
  const isRunning = rawStatus === 'running' || rawStatus === 'generating'
  const isSuccess = rawStatus === 'success'
  const isFailed = rawStatus === 'failed'
  const costCount = Math.max(1, Number(preview.costCredits || preview.costPoints || progress?.total || 1))
  const totalCount = Math.max(
    1,
    progress?.total ?? (isSuccess ? imageCount || costCount : costCount),
  )
  const completedCount = Math.min(
    totalCount,
    Math.max(0, progress?.completed ?? (isSuccess ? totalCount : 0)),
  )
  const successCount = Math.min(
    totalCount,
    Math.max(0, progress?.success ?? (isSuccess ? Math.max(1, imageCount || totalCount) : 0)),
  )
  const runningCount = Math.min(
    totalCount,
    Math.max(0, progress?.running ?? (isRunning ? totalCount - completedCount : 0)),
  )
  const elapsedSeconds = Math.max(0, Math.floor(progress?.elapsedSeconds ?? 0))
  const elapsedText = progress || isRunning ? `${elapsedSeconds} 秒` : '-'
  const statusBadgeClass = isSuccess
    ? 'border-emerald-500/30 bg-emerald-500/10 text-emerald-200'
    : isFailed
      ? 'border-rose-500/30 bg-rose-500/10 text-rose-200'
    : isRunning
      ? 'border-amber-500/30 bg-amber-500/10 text-amber-200'
      : 'border-slate-700 bg-slate-900 text-slate-300'

  return (
    <div className="mt-4 rounded-lg border border-slate-800 bg-slate-950 p-4">
      <div className="flex items-center gap-2 text-sm font-medium text-slate-200">
        <Copy className="h-4 w-4 text-slate-400" />
        任务状态
      </div>

      <div className="mt-3 space-y-2">
        <div className="grid grid-cols-2 gap-2">
          <TaskMetaCell label="任务编号" value={preview.taskId} />
          <TaskMetaCell
            label="状态"
            value={
              <span className={`inline-flex rounded-md border px-2 py-0.5 text-xs ${statusBadgeClass}`}>
                {getTaskStatusLabel(rawStatus)}
              </span>
            }
          />
        </div>

        <div className="grid grid-cols-2 gap-2">
          <TaskMetaCell label="完成数量" value={`${completedCount}/${totalCount}`} />
          <TaskMetaCell label="成功数量" value={`${successCount} 张`} />
        </div>

        <div className="grid grid-cols-2 gap-2">
          <TaskMetaCell label="耗时" value={elapsedText} />
          <TaskMetaCell label="运行中" value={`${runningCount} 张`} />
        </div>

        <div className="grid grid-cols-2 gap-2">
          <TaskMetaCell label="消耗次数" value={`${costCount} 次`} />
          <TaskMetaCell label="消耗来源" value={getCreditSourceLabel(preview.creditSource)} />
        </div>

        <TaskMetaCell label="创建时间" value={preview.createdAt ? formatDateTime(preview.createdAt) : '-'} />

        {preview.errorMessage ? (
          <div className="rounded-md border border-rose-500/30 bg-rose-500/10 px-3 py-2 text-sm text-rose-200">
            错误：{preview.errorMessage}
          </div>
        ) : null}
      </div>
    </div>
  )
}

function TaskMetaCell({
  label,
  value,
}: {
  label: string
  value: ReactNode
}) {
  return (
    <div className="min-w-0 rounded-md border border-slate-800 bg-slate-900/70 px-3 py-2">
      <div className="text-xs text-slate-500">{label}</div>
      <div className="mt-1 min-w-0 truncate text-sm font-medium text-slate-100">{value}</div>
    </div>
  )
}

function PreviewImageStage({
  preview,
  imageURLs,
  activeIndex,
  onPrevious,
  onNext,
  onSaveCurrent,
  onSelectThumbnail,
}: {
  preview: ImageTaskDetailResponse
  imageURLs: string[]
  activeIndex: number
  onPrevious: () => void
  onNext: () => void
  onSaveCurrent: () => void
  onSelectThumbnail: (index: number) => void
}) {
  const hasImages = imageURLs.length > 0
  const currentIndex = hasImages ? Math.min(Math.max(activeIndex, 0), imageURLs.length - 1) : -1
  const currentImageUrl = currentIndex >= 0 ? imageURLs[currentIndex] : ''
  const canNavigate = imageURLs.length > 1
  const thumbnailSlots = createCenteredThumbnailSlots(imageURLs, currentIndex, 5)

  return (
    <div className="mt-4 rounded-lg border border-slate-800 bg-slate-950 p-4">
      <div className="flex min-h-[460px] items-center justify-center rounded-md border border-dashed border-slate-700 bg-slate-900/40 p-4">
        {currentImageUrl ? (
          <img
            src={currentImageUrl}
            alt={preview.prompt ? `${preview.prompt} 第${currentIndex + 1}张` : `第 ${currentIndex + 1} 张图片`}
            className="max-h-[420px] w-full rounded-md object-contain"
          />
        ) : (
          <div className="max-w-2xl text-center">
            <div className="mx-auto flex h-16 w-16 items-center justify-center rounded-md border border-slate-700 bg-slate-900 text-violet-300">
              <Image className="h-8 w-8" />
            </div>
            <h3 className="mt-4 text-xl font-medium text-slate-50">
              {preview.status === 'idle'
                ? '等待生成'
                : preview.status === 'failed'
                  ? '生成失败'
                  : preview.status === 'generating'
                    ? '生成中'
                    : '预览区'}
            </h3>
            <p className="mt-3 text-sm leading-6 text-slate-400">
              {preview.prompt || '生成完成后，当前图片会显示在这里。'}
            </p>
          </div>
        )}
      </div>

      <div className="mt-4 grid gap-2 sm:grid-cols-[minmax(0,1fr)_minmax(0,1.4fr)_minmax(0,1fr)]">
        <button
          type="button"
          onClick={onPrevious}
          disabled={!canNavigate}
          className="inline-flex h-11 items-center justify-center gap-2 rounded-md border border-slate-700 bg-slate-900 px-3 text-sm text-slate-200 transition hover:border-violet-400 hover:text-white disabled:cursor-not-allowed disabled:opacity-50"
        >
          <ChevronLeft className="h-4 w-4" />
          上一张
        </button>

        <button
          type="button"
          onClick={onSaveCurrent}
          disabled={!currentImageUrl}
          className="inline-flex min-h-11 flex-col items-center justify-center rounded-md bg-violet-500 px-3 py-2 text-sm font-medium text-slate-950 transition hover:bg-violet-400 disabled:cursor-not-allowed disabled:opacity-50"
        >
          <span className="inline-flex items-center gap-2">
            <Download className="h-4 w-4" />
            保存当前图片
          </span>
          <span className="mt-1 text-[11px] font-normal leading-tight text-slate-900/70">
            点击一次即可，然后稍作等待下载
          </span>
        </button>

        <button
          type="button"
          onClick={onNext}
          disabled={!canNavigate}
          className="inline-flex h-11 items-center justify-center gap-2 rounded-md border border-slate-700 bg-slate-900 px-3 text-sm text-slate-200 transition hover:border-violet-400 hover:text-white disabled:cursor-not-allowed disabled:opacity-50"
        >
          下一张
          <ChevronRight className="h-4 w-4" />
        </button>
      </div>

      <div className="mt-4">
        {hasImages ? (
          <div className="grid grid-cols-5 gap-2">
            {thumbnailSlots.map((slot, slotIndex) =>
              slot ? (
                <button
                  key={`${slot.index}-${slot.imageUrl}`}
                  type="button"
                  onClick={() => onSelectThumbnail(slot.index)}
                  className={`relative aspect-square overflow-hidden rounded-md border transition ${
                    slot.index === currentIndex
                      ? 'border-violet-400 ring-2 ring-violet-400/30'
                      : 'border-slate-800 hover:border-slate-600'
                  }`}
                  aria-label={`切换到第 ${slot.index + 1} 张图片`}
                >
                  <img src={slot.imageUrl} alt={`缩略图 ${slot.index + 1}`} className="h-full w-full object-cover" />
                  <span
                    className={`absolute inset-x-0 bottom-0 px-1 py-0.5 text-[10px] leading-none ${
                      slot.index === currentIndex ? 'bg-violet-500/85 text-slate-950' : 'bg-slate-950/65 text-slate-200'
                    }`}
                  >
                    {slot.index === currentIndex ? '当前' : `${slot.index + 1}`}
                  </span>
                </button>
              ) : (
                <div
                  key={`preview-empty-${slotIndex}`}
                  className="aspect-square rounded-md border border-dashed border-slate-800 bg-slate-950/40"
                />
              ),
            )}
          </div>
        ) : (
          <div className="rounded-md border border-dashed border-slate-800 bg-slate-950/40 px-4 py-5 text-center text-sm text-slate-500">
            暂无缩略图，生成完成后会自动显示最多 5 张。
          </div>
        )}
      </div>
    </div>
  )
}

function PromptPlaza({
  items,
  favoriteIds,
  submission,
  isAdmin,
  onSubmissionChange,
  onSubmit,
  onFavorite,
  onUse,
  onBackHome,
}: {
  items: PromptGalleryItem[]
  favoriteIds: string[]
  submission: { title: string; prompt: string; previewUrl: string }
  isAdmin: boolean
  onSubmissionChange: (value: { title: string; prompt: string; previewUrl: string }) => void
  onSubmit: () => void
  onFavorite: (id: string) => void
  onUse: (item: PromptGalleryItem) => void
  onBackHome: () => void
}) {
  const pendingItems = items.filter((item) => item.status === 'pending')
  return (
    <div className="space-y-4">
      <PageHeader
        title="提示词广场"
        description="展示优质提示词，收藏喜欢的灵感，也可以提交自己的提示词等待审核。"
        action={
          <button
            type="button"
            onClick={onBackHome}
            className="inline-flex items-center gap-2 rounded-md border border-slate-700 bg-slate-950 px-3 py-2 text-sm text-slate-200 transition hover:border-violet-400 hover:text-white"
          >
            <Home className="h-4 w-4" />
            返回首页
          </button>
        }
      />

      <div className="grid gap-4 xl:grid-cols-[minmax(0,1fr)_380px]">
        <section className="grid gap-4 md:grid-cols-2 2xl:grid-cols-3">
          {items.map((item) => (
            <PromptCard
              key={item.id}
              item={item}
              favorite={favoriteIds.includes(item.id)}
              onFavorite={() => onFavorite(item.id)}
              onUse={() => onUse(item)}
            />
          ))}
        </section>

        <aside className="space-y-4">
          <section className="rounded-lg border border-slate-800 bg-slate-900/70 p-4 shadow-panel">
            <div className="flex items-center justify-between">
              <div>
                <h2 className="text-lg font-medium text-slate-50">上传提示词</h2>
                <p className="mt-1 text-sm text-slate-400">当前先保存到前端待审核列表。</p>
              </div>
              <Upload className="h-5 w-5 text-violet-300" />
            </div>

            <div className="mt-4 space-y-3">
              <Field label="标题">
                <input
                  value={submission.title}
                  onChange={(event) => onSubmissionChange({ ...submission, title: event.target.value })}
                  className="h-11 w-full rounded-md border border-slate-700 bg-slate-950 px-3 text-sm text-slate-100 outline-none transition placeholder:text-slate-500 focus:border-violet-400"
                  placeholder="例如：赛博机械角色"
                />
              </Field>
              <Field label="预览图链接">
                <input
                  value={submission.previewUrl}
                  onChange={(event) => onSubmissionChange({ ...submission, previewUrl: event.target.value })}
                  className="h-11 w-full rounded-md border border-slate-700 bg-slate-950 px-3 text-sm text-slate-100 outline-none transition placeholder:text-slate-500 focus:border-violet-400"
                  placeholder="请输入预览图链接"
                />
              </Field>
              <Field label="提示词">
                <textarea
                  value={submission.prompt}
                  onChange={(event) => onSubmissionChange({ ...submission, prompt: event.target.value })}
                  className="min-h-28 w-full resize-y rounded-md border border-slate-700 bg-slate-950 px-3 py-3 text-sm leading-6 text-slate-100 outline-none transition placeholder:text-slate-500 focus:border-violet-400"
                  placeholder="写下可复用的生图提示词"
                />
              </Field>
              <button
                type="button"
                onClick={onSubmit}
                className="inline-flex w-full items-center justify-center gap-2 rounded-md bg-violet-500 px-4 py-3 font-medium text-slate-950 transition hover:bg-violet-400"
              >
                <Upload className="h-4 w-4" />
                提交审核
              </button>
            </div>
          </section>

          <section className="rounded-lg border border-slate-800 bg-slate-900/70 p-4 shadow-panel">
            <div className="flex items-center justify-between">
              <div>
                <h2 className="text-lg font-medium text-slate-50">审核队列</h2>
                <p className="mt-1 text-sm text-slate-400">后续由管理员审核发布。</p>
              </div>
              <ShieldCheck className="h-5 w-5 text-emerald-300" />
            </div>
            <div className="mt-4 space-y-2">
              {pendingItems.length ? (
                pendingItems.map((item) => (
                  <div key={item.id} className="rounded-lg border border-slate-800 bg-slate-950 p-3">
                    <div className="text-sm font-medium text-slate-100">{item.title}</div>
                    <p className="mt-2 line-clamp-2 text-xs leading-5 text-slate-500">{item.prompt}</p>
                    <div className="mt-2 text-xs text-amber-300">{isAdmin ? '待接入审核按钮' : '等待管理员审核'}</div>
                  </div>
                ))
              ) : (
                <div className="rounded-lg border border-dashed border-slate-700 bg-slate-950 p-4 text-sm text-slate-400">
                  暂无待审核提示词。
                </div>
              )}
            </div>
          </section>
        </aside>
      </div>
    </div>
  )
}

function PromptCard({
  item,
  favorite,
  onFavorite,
  onUse,
}: {
  item: PromptGalleryItem
  favorite: boolean
  onFavorite: () => void
  onUse: () => void
}) {
  const [imageReady, setImageReady] = useState(false)

  useEffect(() => {
    setImageReady(false)
  }, [item.previewUrl])

  return (
    <article className="overflow-hidden rounded-lg border border-slate-800 bg-slate-900/70 shadow-panel">
      <div className="relative aspect-[4/3] overflow-hidden bg-slate-950">
        <div className="absolute inset-0 flex flex-col justify-between bg-slate-950 px-4 py-4">
          <div className="flex items-center justify-between gap-3">
            <span className="rounded-md border border-slate-700 bg-slate-900/80 px-2 py-1 text-[11px] text-slate-300">
              预览
            </span>
            <Image className="h-5 w-5 text-violet-300/80" />
          </div>
          <div className="space-y-2">
            <div className="text-base font-medium text-slate-50">{item.title}</div>
            <p className="line-clamp-3 text-xs leading-5 text-slate-400">{item.prompt}</p>
          </div>
        </div>
        {item.previewUrl ? (
          <img
            src={item.previewUrl}
            alt={item.title}
            className={`absolute inset-0 h-full w-full object-cover transition-opacity duration-300 ${
              imageReady ? 'opacity-100' : 'opacity-0'
            }`}
            onLoad={() => setImageReady(true)}
            onError={() => setImageReady(false)}
          />
        ) : null}
        <div className="absolute inset-x-0 bottom-0 h-24 bg-gradient-to-t from-slate-950 via-slate-950/80 to-transparent" />
        <span
          className={`absolute left-3 top-3 rounded-md px-2 py-1 text-xs ${
            item.status === 'approved'
              ? 'bg-emerald-500/80 text-slate-950'
              : item.status === 'pending'
                ? 'bg-amber-500/80 text-slate-950'
                : 'bg-rose-500/80 text-white'
          }`}
        >
          {item.status === 'approved' ? '已发布' : item.status === 'pending' ? '待审核' : '未通过'}
        </span>
      </div>
      <div className="p-4">
        <div className="flex items-start justify-between gap-3">
          <div className="min-w-0">
            <h2 className="truncate text-base font-medium text-slate-50">{item.title}</h2>
            <p className="mt-1 text-xs text-slate-500">
              {item.author} · {formatDateTime(item.createdAt)}
            </p>
          </div>
          <button
            type="button"
            onClick={onFavorite}
            className={`inline-flex h-9 w-9 shrink-0 items-center justify-center rounded-md border transition ${
              favorite
                ? 'border-rose-400 bg-rose-500/15 text-rose-300'
                : 'border-slate-700 bg-slate-950 text-slate-300 hover:border-rose-400 hover:text-rose-300'
            }`}
            title="收藏"
          >
            <Heart className="h-4 w-4" />
          </button>
        </div>
        <p className="mt-3 line-clamp-3 text-sm leading-6 text-slate-400">{item.prompt}</p>
        <div className="mt-4 flex items-center justify-between gap-3">
          <span className="text-xs text-slate-500">{item.favorites + (favorite ? 1 : 0)} 收藏</span>
          <button
            type="button"
            onClick={onUse}
            className="inline-flex items-center gap-2 rounded-md border border-violet-500/40 bg-violet-500/10 px-3 py-2 text-sm text-violet-200 transition hover:border-violet-400 hover:text-white"
          >
            <Sparkles className="h-4 w-4" />
            使用
          </button>
        </div>
      </div>
    </article>
  )
}

function PackageSection({
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
  actionLabel: string
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
          items.map((item) => (
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
                  {!item.enabled ? (
                    <span className="rounded-md bg-slate-800 px-2 py-0.5 text-[11px] text-slate-400">停用</span>
                  ) : null}
                </div>
                <div className="mt-2 flex flex-wrap gap-2 text-xs text-slate-500">{renderMeta(item)}</div>
              </div>
              <span className="rounded-md bg-violet-500/15 px-2 py-1 text-xs text-violet-300">
                {busy === `${busyPrefix}${item.code}` ? '处理中' : actionLabel}
              </span>
            </button>
          ))
        ) : (
          <div className="rounded-lg border border-dashed border-slate-700 bg-slate-950 p-4 text-sm text-slate-400">
            {emptyText}
          </div>
        )}
      </div>
    </div>
  )
}

function AdminPanel({
  stats,
  users,
  tasks,
  orders,
  audits,
  actionLogs,
  packages,
  busy,
  onRefresh,
  onToggleUserStatus,
  onAdjustUserPoints,
  onSavePackage,
}: {
  stats: AdminStatsResponse | null
  users: AdminUserItem[]
  tasks: AdminImageTaskItem[]
  orders: AdminRechargeOrderItem[]
  audits: AdminContentAuditItem[]
  actionLogs: AdminActionLogItem[]
  packages: AdminPackageItem[]
  busy: boolean
  onRefresh: () => void
  onToggleUserStatus: (user: AdminUserItem) => void
  onAdjustUserPoints: (user: AdminUserItem, delta: number) => void
  onSavePackage: (packageId: string | null, payload: AdminPackagePayload) => void
}) {
  const [editingPackageId, setEditingPackageId] = useState<string | null>(null)
  const [packageForm, setPackageForm] = useState<AdminPackagePayload>({
    code: '',
    name: '',
    packageType: 'recharge',
    creditsAmount: 100,
    pointsCost: 0,
    amountCents: 1999,
    bonusPoints: 0,
    enabled: true,
    sortOrder: 0,
    note: '',
  })

  function startCreatePackage() {
    setEditingPackageId(null)
    setPackageForm({
      code: '',
      name: '',
      packageType: 'recharge',
      creditsAmount: 100,
      pointsCost: 0,
      amountCents: 1999,
      bonusPoints: 0,
      enabled: true,
      sortOrder: 0,
      note: '',
    })
  }

  function startEditPackage(item: AdminPackageItem) {
    setEditingPackageId(item.id)
    setPackageForm({
      code: item.code,
      name: item.name,
      packageType: item.packageType,
      creditsAmount: item.creditsAmount,
      pointsCost: item.pointsCost,
      amountCents: item.amountCents,
      bonusPoints: item.bonusPoints,
      enabled: item.enabled,
      sortOrder: item.sortOrder,
      note: item.note ?? '',
    })
  }

  function handlePackageTypeChange(nextType: 'recharge' | 'redeem') {
    setPackageForm((current) => {
      if (current.packageType === nextType) {
        return current
      }

      if (nextType === 'redeem') {
        return {
          ...current,
          packageType: nextType,
          pointsCost: current.pointsCost > 0 ? current.pointsCost : Math.max(3, current.creditsAmount * 3),
          amountCents: 0,
          bonusPoints: 0,
        }
      }

      return {
        ...current,
        packageType: nextType,
        pointsCost: 0,
        amountCents: current.amountCents > 0 ? current.amountCents : 1999,
        bonusPoints: current.bonusPoints >= 0 ? current.bonusPoints : 0,
      }
    })
  }

  const isRedeemPackage = packageForm.packageType === 'redeem'

  function submitPackageForm() {
    const payload: AdminPackagePayload = {
      code: editingPackageId ? undefined : packageForm.code,
      name: packageForm.name,
      packageType: editingPackageId ? undefined : packageForm.packageType,
      creditsAmount: packageForm.creditsAmount,
      pointsCost: packageForm.pointsCost,
      amountCents: packageForm.amountCents,
      bonusPoints: packageForm.bonusPoints,
      enabled: packageForm.enabled,
      sortOrder: packageForm.sortOrder,
      note: packageForm.note,
    }
    onSavePackage(editingPackageId, payload)
  }

  return (
    <section className="rounded-lg border border-emerald-600/30 bg-slate-900/70 p-4 shadow-panel">
      <div className="flex flex-col gap-3 border-b border-slate-800 pb-4 lg:flex-row lg:items-center lg:justify-between">
        <div>
          <div className="flex items-center gap-2 text-sm text-emerald-300">
            <ShieldCheck className="h-4 w-4" />
            后台管理
          </div>
          <h2 className="mt-1 text-xl font-semibold text-slate-50">运营、安全和风控概览</h2>
        </div>
        <button
          type="button"
          onClick={onRefresh}
          disabled={busy}
          className="inline-flex h-10 items-center justify-center gap-2 rounded-md border border-slate-700 bg-slate-950 px-4 text-sm text-slate-200 transition hover:border-emerald-400 hover:text-white disabled:cursor-not-allowed disabled:opacity-60"
        >
          {busy ? <LoaderCircle className="h-4 w-4 animate-spin" /> : <RefreshCcw className="h-4 w-4" />}
          刷新后台
        </button>
      </div>

      <div className="mt-4 grid gap-3 sm:grid-cols-2 xl:grid-cols-4">
        <MiniStat label="用户总数" value={String(stats?.usersTotal ?? 0)} />
        <MiniStat label="成功任务" value={String(stats?.tasksSuccess ?? 0)} />
        <MiniStat label="生成中" value={String(stats?.tasksGenerating ?? 0)} />
        <MiniStat label="拦截内容" value={String(stats?.auditRejected ?? 0)} />
        <MiniStat label="积分总量" value={String(stats?.pointsBalanceTotal ?? 0)} />
        <MiniStat label="免费次数" value={String(stats?.freeCreditsTotal ?? 0)} />
        <MiniStat label="充值次数" value={String(stats?.paidCreditsTotal ?? 0)} />
      </div>

      <div className="mt-4 grid gap-4 xl:grid-cols-2">
        <AdminBlock title="用户管理">
          <div className="space-y-2">
            {users.length ? (
              users.map((user) => (
                <div key={user.id} className="rounded-md border border-slate-800 bg-slate-950 p-3">
                  <div className="flex flex-wrap items-center justify-between gap-2">
                    <div className="min-w-0">
                      <div className="truncate text-sm font-medium text-slate-100">{user.username}</div>
                      <div className="mt-1 truncate text-xs text-slate-500">{user.email}</div>
                    </div>
                    <span className={`rounded-md px-2 py-1 text-xs ${user.status === 1 ? 'bg-emerald-500/15 text-emerald-300' : 'bg-rose-500/15 text-rose-300'}`}>
                      {user.status === 1 ? '正常' : '禁用'} / {getUserRoleLabel(user.role)}
                    </span>
                  </div>
                  <div className="mt-3 flex flex-wrap items-center justify-between gap-2 text-xs text-slate-500">
                    <span>积分 {user.pointsBalance}</span>
                    <span>免费 {user.freeCredits} / 充值 {user.paidCredits}</span>
                    <span>总次数 {user.totalCredits}</span>
                    <span>{formatDateTime(user.createdAt)}</span>
                  </div>
                  <div className="mt-3 flex flex-wrap gap-2">
                    <button
                      type="button"
                      onClick={() => onAdjustUserPoints(user, 10)}
                      className="rounded-md border border-slate-700 px-2 py-1 text-xs text-slate-200 transition hover:border-emerald-400"
                    >
                      +10
                    </button>
                    <button
                      type="button"
                      onClick={() => onAdjustUserPoints(user, -10)}
                      className="rounded-md border border-slate-700 px-2 py-1 text-xs text-slate-200 transition hover:border-amber-400"
                    >
                      -10
                    </button>
                    <button
                      type="button"
                      onClick={() => onToggleUserStatus(user)}
                      className="rounded-md border border-slate-700 px-2 py-1 text-xs text-slate-200 transition hover:border-rose-400"
                    >
                      {user.status === 1 ? '禁用' : '启用'}
                    </button>
                  </div>
                </div>
              ))
            ) : (
              <EmptyAdminText text="暂无用户数据" />
            )}
          </div>
        </AdminBlock>

        <AdminBlock title="套餐配置">
          <div className="space-y-3">
            <div className="flex flex-wrap items-center gap-2">
              <button
                type="button"
                onClick={startCreatePackage}
                className="rounded-md border border-slate-700 px-3 py-1.5 text-xs text-slate-200 transition hover:border-violet-400"
              >
                新建套餐
              </button>
              {editingPackageId ? (
                <span className="text-xs text-slate-500">正在编辑 #{editingPackageId}</span>
              ) : (
                <span className="text-xs text-slate-500">当前为新建模式</span>
              )}
            </div>

            <div className="grid gap-3 md:grid-cols-2">
              {!editingPackageId ? (
                <label className="block md:col-span-2">
                  <span className="mb-2 block text-xs text-slate-500">套餐编码</span>
                  <input
                    value={packageForm.code ?? ''}
                    onChange={(event) => setPackageForm((current) => ({ ...current, code: event.target.value }))}
                    className="h-10 w-full rounded-md border border-slate-700 bg-slate-950 px-3 text-sm text-slate-100 outline-none transition focus:border-violet-400"
                    placeholder="recharge-100"
                  />
                </label>
              ) : null}

              <label className="block">
                <span className="mb-2 block text-xs text-slate-500">套餐名称</span>
                <input
                  value={packageForm.name}
                  onChange={(event) => setPackageForm((current) => ({ ...current, name: event.target.value }))}
                  className="h-10 w-full rounded-md border border-slate-700 bg-slate-950 px-3 text-sm text-slate-100 outline-none transition focus:border-violet-400"
                />
              </label>

              {!editingPackageId ? (
                <label className="block">
                  <span className="mb-2 block text-xs text-slate-500">套餐类型</span>
                  <select
                    value={packageForm.packageType}
                    onChange={(event) => handlePackageTypeChange(event.target.value as 'recharge' | 'redeem')}
                    className="h-10 w-full rounded-md border border-slate-700 bg-slate-950 px-3 text-sm text-slate-100 outline-none transition focus:border-violet-400"
                  >
                    <option value="recharge">充值次数</option>
                    <option value="redeem">积分兑换</option>
                  </select>
                </label>
              ) : null}

              <label className="block">
                <span className="mb-2 block text-xs text-slate-500">次数</span>
                <input
                  type="number"
                  min={1}
                  value={packageForm.creditsAmount}
                  onChange={(event) =>
                    setPackageForm((current) => ({ ...current, creditsAmount: Number(event.target.value) }))
                  }
                  className="h-10 w-full rounded-md border border-slate-700 bg-slate-950 px-3 text-sm text-slate-100 outline-none transition focus:border-violet-400"
                />
              </label>

              {isRedeemPackage ? (
                <label className="block">
                  <span className="mb-2 block text-xs text-slate-500">积分消耗</span>
                  <input
                    type="number"
                    min={1}
                    value={packageForm.pointsCost}
                    onChange={(event) =>
                      setPackageForm((current) => ({ ...current, pointsCost: Number(event.target.value) }))
                    }
                    className="h-10 w-full rounded-md border border-slate-700 bg-slate-950 px-3 text-sm text-slate-100 outline-none transition focus:border-violet-400"
                  />
                </label>
              ) : (
                <>
                  <label className="block">
                    <span className="mb-2 block text-xs text-slate-500">价格（分）</span>
                    <input
                      type="number"
                      min={0}
                      value={packageForm.amountCents}
                      onChange={(event) =>
                        setPackageForm((current) => ({ ...current, amountCents: Number(event.target.value) }))
                      }
                      className="h-10 w-full rounded-md border border-slate-700 bg-slate-950 px-3 text-sm text-slate-100 outline-none transition focus:border-violet-400"
                    />
                  </label>

                  <label className="block">
                    <span className="mb-2 block text-xs text-slate-500">赠送积分</span>
                    <input
                      type="number"
                      min={0}
                      value={packageForm.bonusPoints}
                      onChange={(event) =>
                        setPackageForm((current) => ({ ...current, bonusPoints: Number(event.target.value) }))
                      }
                      className="h-10 w-full rounded-md border border-slate-700 bg-slate-950 px-3 text-sm text-slate-100 outline-none transition focus:border-violet-400"
                    />
                  </label>
                </>
              )}

              <label className="block">
                <span className="mb-2 block text-xs text-slate-500">排序</span>
                <input
                  type="number"
                  value={packageForm.sortOrder}
                  onChange={(event) =>
                    setPackageForm((current) => ({ ...current, sortOrder: Number(event.target.value) }))
                  }
                  className="h-10 w-full rounded-md border border-slate-700 bg-slate-950 px-3 text-sm text-slate-100 outline-none transition focus:border-violet-400"
                />
              </label>

              <label className="block md:col-span-2">
                <span className="mb-2 block text-xs text-slate-500">备注</span>
                <input
                  value={packageForm.note ?? ''}
                  onChange={(event) => setPackageForm((current) => ({ ...current, note: event.target.value }))}
                  className="h-10 w-full rounded-md border border-slate-700 bg-slate-950 px-3 text-sm text-slate-100 outline-none transition focus:border-violet-400"
                />
              </label>

              <label className="flex items-center gap-2 text-xs text-slate-400">
                <input
                  type="checkbox"
                  checked={packageForm.enabled ?? true}
                  onChange={(event) => setPackageForm((current) => ({ ...current, enabled: event.target.checked }))}
                />
                启用
              </label>

              <div className="md:col-span-2 text-xs leading-6 text-slate-500">
                {isRedeemPackage
                  ? '兑换档位只需要填写次数和积分消耗，价格与赠送积分会自动归零。'
                  : '充值档位只需要填写次数、价格和赠送积分，积分消耗会自动归零。'}
              </div>

              <div className="md:col-span-2">
                <button
                  type="button"
                  onClick={submitPackageForm}
                  className="rounded-md bg-violet-500 px-3 py-2 text-sm font-medium text-slate-950 transition hover:bg-violet-400"
                >
                  保存套餐
                </button>
              </div>
            </div>

            <div className="space-y-2">
              {packages.length ? (
                packages.map((item) => (
                  <button
                    key={item.id}
                    type="button"
                    onClick={() => startEditPackage(item)}
                    className="flex w-full items-start justify-between gap-2 rounded-md border border-slate-800 bg-slate-950 p-3 text-left transition hover:border-violet-400/50"
                  >
                    <div className="min-w-0">
                      <div className="truncate text-sm text-slate-100">
                        {item.name} <span className="text-xs text-slate-500">({getPackageTypeLabel(item.packageType)})</span>
                      </div>
                      <div className="mt-1 flex flex-wrap gap-2 text-xs text-slate-500">
                        <span>次数 {item.creditsAmount}</span>
                        {item.packageType === 'redeem' ? (
                          <span>积分 {item.pointsCost}</span>
                        ) : (
                          <>
                            <span>金额 {(item.amountCents / 100).toFixed(2)}</span>
                            <span>赠送 {item.bonusPoints}</span>
                          </>
                        )}
                        <span>{item.enabled ? '启用' : '停用'}</span>
                      </div>
                    </div>
                    <span className="text-xs text-slate-500">编辑</span>
                  </button>
                ))
              ) : (
                <EmptyAdminText text="暂无套餐配置" />
              )}
            </div>
          </div>
        </AdminBlock>

        <AdminBlock title="套餐审计">
          <div className="space-y-2">
            {actionLogs.length ? (
              actionLogs.slice(0, 8).map((item) => (
                <div key={item.id} className="rounded-md border border-slate-800 bg-slate-950 p-3">
                  <div className="flex items-center justify-between gap-2">
                    <span className="rounded-md bg-violet-500/15 px-2 py-1 text-xs text-violet-300">
                      {item.action}
                    </span>
                    <span className="text-xs text-slate-500">{formatDateTime(item.createdAt)}</span>
                  </div>
                  <div className="mt-2 flex flex-wrap gap-2 text-xs text-slate-500">
                    <span>目标 #{item.targetId}</span>
                    <span>管理员 {item.adminId}</span>
                    {item.remark ? <span>{item.remark}</span> : null}
                  </div>
                </div>
              ))
            ) : (
              <EmptyAdminText text="暂无套餐审计日志" />
            )}
          </div>
        </AdminBlock>

        <AdminBlock title="内容安全">
          <div className="space-y-2">
            {audits.length ? (
              audits.map((item) => (
                <div key={item.id} className="rounded-md border border-slate-800 bg-slate-950 p-3">
                  <div className="flex items-center justify-between gap-2">
                    <span className={`rounded-md px-2 py-1 text-xs ${item.status === 'rejected' ? 'bg-rose-500/15 text-rose-300' : 'bg-emerald-500/15 text-emerald-300'}`}>
                      {getAuditStatusLabel(item.status)}
                    </span>
                    <span className="text-xs text-slate-500">{formatDateTime(item.createdAt)}</span>
                  </div>
                  <p className="mt-2 line-clamp-2 text-sm leading-6 text-slate-300">{item.content}</p>
                  <p className="mt-2 text-xs text-slate-500">{item.reason || '-'}</p>
                </div>
              ))
            ) : (
              <EmptyAdminText text="暂无审核日志" />
            )}
          </div>
        </AdminBlock>

        <AdminBlock title="全站任务">
                <div className="space-y-2">
                  {tasks.length ? (
                    tasks.map((task) => (
                      <div key={task.id} className="rounded-md border border-slate-800 bg-slate-950 p-3">
                        <div className="flex items-center justify-between gap-2">
                          <span className="text-sm text-slate-200">#{task.id} / 用户 {task.userId}</span>
                          <span className="rounded-md bg-slate-800 px-2 py-1 text-xs text-slate-300">{getImageTaskStatusLabel(task.status)}</span>
                        </div>
                        <p className="mt-2 line-clamp-2 text-sm leading-6 text-slate-400">{task.prompt}</p>
                        <div className="mt-2 flex flex-wrap gap-2 text-xs text-slate-500">
                          <span>{task.costCredits || task.costPoints} 次</span>
                          <span>免费 {task.freeCreditsCost} / 充值 {task.paidCreditsCost}</span>
                          <span>{getCreditSourceLabel(task.creditSource)}</span>
                        </div>
                      </div>
                    ))
                  ) : (
              <EmptyAdminText text="暂无任务数据" />
            )}
          </div>
        </AdminBlock>

        <AdminBlock title="订单记录">
          <div className="space-y-2">
            {orders.length ? (
              orders.map((order) => (
                <div key={order.id} className="rounded-md border border-slate-800 bg-slate-950 p-3">
                  <div className="flex items-center justify-between gap-2">
                    <span className="truncate text-sm text-slate-200">{order.orderNo}</span>
                    <span className="rounded-md bg-slate-800 px-2 py-1 text-xs text-slate-300">{getRechargeOrderStatusLabel(order.status)}</span>
                  </div>
                  <div className="mt-2 flex items-center justify-between text-xs text-slate-500">
                    <span>{order.packageName}</span>
                    <span>{order.creditsAmount} 次 + {order.bonusPoints} 积分</span>
                  </div>
                  <div className="mt-2 flex flex-wrap gap-2 text-xs text-slate-500">
                    {order.paymentChannel ? <span>渠道 {getPaymentChannelLabel(order.paymentChannel)}</span> : null}
                    {order.providerTradeNo ? <span>交易号 {order.providerTradeNo}</span> : null}
                    {order.paidAt ? <span>{formatDateTime(order.paidAt)}</span> : null}
                  </div>
                </div>
              ))
            ) : (
              <EmptyAdminText text="暂无订单数据" />
            )}
          </div>
        </AdminBlock>
      </div>
    </section>
  )
}

function AdminBlock({ title, children }: { title: string; children: ReactNode }) {
  return (
    <div className="rounded-lg border border-slate-800 bg-slate-900 p-4">
      <h3 className="text-sm font-medium text-slate-100">{title}</h3>
      <div className="mt-3">{children}</div>
    </div>
  )
}

function EmptyAdminText({ text }: { text: string }) {
  return (
    <div className="rounded-md border border-dashed border-slate-700 bg-slate-950 p-4 text-sm text-slate-500">
      {text}
    </div>
  )
}

function StatCard({ label, value }: { label: string; value: string }) {
  return (
    <div className="rounded-lg border border-slate-800 bg-slate-950 p-3">
      <div className="text-xs text-slate-500">{label}</div>
      <div className="mt-2 break-all text-sm font-medium text-slate-100">{value}</div>
    </div>
  )
}

function GenerationProgressPanel({ progresses }: { progresses: GenerationProgress[] }) {
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

function GenerationProgressCard({ progress }: { progress: GenerationProgress }) {
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
        <ProgressStat label="提示词" value={progress.prompt || '未填写'} />
        <ProgressStat label="完成数量" value={`${progress.completed}/${progress.total}`} />
        <ProgressStat label="成功数量" value={String(progress.success)} />
        <ProgressStat label="运行中" value={String(progress.running)} />
        <ProgressStat label="耗时" value={`${progress.elapsedSeconds}s`} />
        <ProgressStat label="轮询次数" value={isRunning ? `${Math.max(1, progress.attempt)} 次` : '-'} />
      </div>

      <div className="mt-3 flex items-start gap-2 rounded-md border border-slate-800 bg-slate-900 px-3 py-2 text-sm text-slate-300">
        <Timer className="mt-0.5 h-4 w-4 shrink-0 text-slate-400" />
        <span>{progress.message}</span>
      </div>
    </div>
  )
}

function ProgressStat({ label, value }: { label: string; value: string }) {
  return (
    <div className="rounded-md border border-slate-800 bg-slate-950 px-3 py-2">
      <div className="text-xs text-slate-500">{label}</div>
      <div className="mt-1 break-all text-sm text-slate-100">{value}</div>
    </div>
  )
}

function MiniStat({ label, value }: { label: string; value: string }) {
  return (
    <div className="rounded-lg border border-slate-800 bg-slate-950 p-3">
      <div className="text-xs text-slate-500">{label}</div>
      <div className="mt-2 text-lg font-semibold text-slate-100">{value}</div>
    </div>
  )
}

function PointsLogRow({ item }: { item: PointsLogItem }) {
  const isPositive = item.delta >= 0
  const title = item.remark || getPointsChangeTypeLabel(item.changeType)
  return (
    <div className="rounded-lg border border-slate-800 bg-slate-950 p-3">
      <div className="flex items-center justify-between gap-2">
        <span className="truncate text-sm text-slate-200">{title}</span>
        <span className={isPositive ? 'text-emerald-300' : 'text-rose-300'}>
          {isPositive ? '+' : ''}
          {item.delta}
        </span>
      </div>
      <div className="mt-2 flex items-center justify-between text-xs text-slate-500">
        <span>{formatDateTime(item.createdAt)}</span>
        <span>
          {item.beforeBalance} → {item.afterBalance}
        </span>
      </div>
    </div>
  )
}

function CreditLogRow({ item }: { item: CreditLogItem }) {
  const totalAfter = item.afterFree + item.afterPaid
  const title = item.remark || getCreditChangeTypeLabel(item.changeType)
  return (
    <div className="rounded-lg border border-slate-800 bg-slate-950 p-3">
      <div className="flex items-center justify-between gap-2">
        <span className="truncate text-sm text-slate-200">{title}</span>
        <span className="text-xs text-slate-500">{getCreditChangeTypeLabel(item.changeType)}</span>
      </div>
      <div className="mt-2 flex flex-wrap items-center justify-between gap-2 text-xs text-slate-500">
        <span>
          免费 {item.beforeFree} → {item.afterFree} ({item.deltaFree >= 0 ? '+' : ''}
          {item.deltaFree})
        </span>
        <span>
          充值 {item.beforePaid} → {item.afterPaid} ({item.deltaPaid >= 0 ? '+' : ''}
          {item.deltaPaid})
        </span>
      </div>
      <div className="mt-2 flex items-center justify-between text-xs text-slate-500">
        <span>{formatDateTime(item.createdAt)}</span>
        <span>总次数 {totalAfter}</span>
      </div>
    </div>
  )
}

function getGenerationStatusLabel(status: GenerationStatus): string {
  return getTaskStatusLabel(status)
}

function getImageTaskStatusLabel(status: string): string {
  return getTaskStatusLabel(status)
}

function getTaskStatusLabel(status: string): string {
  switch (status) {
    case 'idle':
      return '待开始'
    case 'running':
    case 'generating':
      return '正在生成'
    case 'success':
      return '成功'
    case 'failed':
      return '失败'
    default:
      return status || '未知'
  }
}

function getUserRoleLabel(role: string): string {
  switch (role) {
    case 'admin':
      return '管理员'
    case 'user':
      return '普通用户'
    default:
      return role || '未知'
  }
}

function getCreditSourceLabel(source: string): string {
  switch (source) {
    case 'free':
      return '免费次数'
    case 'paid':
      return '充值次数'
    case 'mixed':
      return '混合次数'
    case 'legacy_points':
      return '历史积分模式'
    case 'legacy':
      return '旧版模式'
    default:
      return source || '未知'
  }
}

function stringifyTaskRequestSnapshot(snapshot: TaskRequestSnapshot): string {
  return JSON.stringify(snapshot)
}

function parseTaskRequestParams(raw?: string): TaskRequestSnapshot | null {
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

function parsePreviewSizeState(size?: string): { size: string; customWidth?: number; customHeight?: number } | null {
  const trimmed = size?.trim()
  if (!trimmed) {
    return null
  }

  if (sizeOptions.some((option) => option.value === trimmed)) {
    if (trimmed === 'custom') {
      return { size: 'custom' }
    }
    return { size: trimmed }
  }

  const match = trimmed.match(/^(\d{2,4})x(\d{2,4})$/i)
  if (!match) {
    return null
  }

  return {
    size: 'custom',
    customWidth: clampDimension(Number(match[1])),
    customHeight: clampDimension(Number(match[2])),
  }
}

function buildPreviewRequestOptions(preview: ImageTaskDetailResponse): Array<{ label: string; value: string; wide?: boolean }> {
  const parsed = parseTaskRequestParams(preview.requestParams)
  const count = Number(parsed?.count ?? preview.costCredits ?? preview.costPoints ?? 0)

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
      value: parsed?.referenceImageUrl || '未使用',
      wide: true,
    },
  ]
}

function formatPreviewOptionValue(
  value: string | undefined,
  options: Array<{ value: string; label: string }>,
): string {
  if (value === undefined) {
    return '-'
  }

  const trimmed = value.trim()
  const matched = options.find((option) => option.value === trimmed)
  return matched?.label ?? (trimmed || '-')
}

function getPointsChangeTypeLabel(changeType: string): string {
  switch (changeType) {
    case 'checkin':
      return '每日签到'
    case 'purchase':
      return '充值入账'
    case 'purchase_bonus':
      return '充值赠送'
    case 'redeem':
      return '积分兑换'
    case 'generate':
      return '生图消耗'
    case 'refund':
      return '失败退回'
    case 'adjust':
      return '后台调整'
    default:
      return changeType || '积分变动'
  }
}

function getCreditChangeTypeLabel(changeType: string): string {
  switch (changeType) {
    case 'redeem':
      return '积分兑换次数'
    case 'purchase':
      return '充值入账次数'
    case 'generate':
      return '生图消耗次数'
    case 'refund':
      return '失败退回次数'
    case 'adjust':
      return '后台调整次数'
    default:
      return changeType || '次数变动'
  }
}

function getRechargeOrderStatusLabel(status: string): string {
  switch (status) {
    case 'pending':
      return '待支付'
    case 'paid':
      return '已支付'
    case 'canceled':
      return '已取消'
    case 'refunded':
      return '已退款'
    default:
      return status || '未知'
  }
}

function getPaymentChannelLabel(channel: string): string {
  switch (channel) {
    case 'mock':
      return '模拟支付'
    default:
      return channel || '未知渠道'
  }
}

function getAuditStatusLabel(status: string): string {
  switch (status) {
    case 'approved':
      return '已通过'
    case 'pending':
      return '待审核'
    case 'rejected':
      return '未通过'
    default:
      return status || '未知'
  }
}

function getPackageTypeLabel(packageType: string): string {
  switch (packageType) {
    case 'recharge':
      return '充值'
    case 'redeem':
      return '兑换'
    default:
      return packageType || '未知'
  }
}

function clampCount(value: number): number {
  if (!Number.isFinite(value) || value <= 0) {
    return 1
  }
  return Math.min(6, Math.max(1, Math.floor(value)))
}

function clampDimension(value: number): number {
  if (!Number.isFinite(value) || value <= 0) {
    return 1024
  }
  return Math.min(4096, Math.max(256, Math.floor(value)))
}

function limitGenerationProgresses(progresses: GenerationProgress[], maxItems = 6): GenerationProgress[] {
  return progresses
    .filter((item) => item.taskId)
    .slice(0, maxItems)
}

function normalizeGenerationStatus(status: string): GenerationStatus {
  switch (status) {
    case 'success':
      return 'success'
    case 'failed':
      return 'failed'
    case 'running':
    case 'generating':
    default:
      return 'running'
  }
}

function progressFromImageTaskItem(item: ImageTaskItem): GenerationProgress {
  const taskCount = Math.max(1, clampCount(Number(item.costCredits || item.costPoints)))
  const status = normalizeGenerationStatus(item.status)
  return {
    taskId: item.id,
    prompt: item.prompt,
    status,
    startedAt: new Date(item.createdAt).getTime() || Date.now(),
    elapsedSeconds: 0,
    total: taskCount,
    completed: status === 'running' ? 0 : taskCount,
    success: status === 'success' ? taskCount : 0,
    running: status === 'running' ? taskCount : 0,
    attempt: 1,
    backendUrl: api.API_BASE_URL,
    message: `任务 ${item.id} ${getImageTaskStatusLabel(item.status)}`,
  }
}

function resolveSelectedSize(size: string, width: number, height: number): string {
  if (size !== 'custom') {
    return size
  }

  const safeWidth = Math.min(4096, Math.max(256, Math.floor(width || 1024)))
  const safeHeight = Math.min(4096, Math.max(256, Math.floor(height || 1024)))
  return `${safeWidth}x${safeHeight}`
}

function normalizeImageURLs(imageUrls?: string[], imageUrl?: string): string[] {
  const seen = new Set<string>()
  const result: string[] = []
  for (const value of [...(imageUrls ?? []), imageUrl ?? '']) {
    const trimmed = value.trim()
    if (!trimmed || seen.has(trimmed)) {
      continue
    }
    seen.add(trimmed)
    result.push(trimmed)
  }
  return result
}

function formatDateTime(value: string): string {
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

function formatDateOnly(value: string): string {
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

function formatSignedNumber(value: number): string {
  return `${value >= 0 ? '+' : ''}${value}`
}

function toPreview(item: ImageTaskItem): ImageTaskDetailResponse {
  return {
    taskId: item.id,
    prompt: item.prompt,
    imageUrl: item.imageUrl,
    imageUrls: normalizeImageURLs(item.imageUrls, item.imageUrl),
    costPoints: item.costPoints,
    costCredits: item.costCredits,
    freeCreditsCost: item.freeCreditsCost,
    paidCreditsCost: item.paidCreditsCost,
    creditSource: item.creditSource,
    status: item.status,
    requestParams: item.requestParams,
    errorMessage: item.errorMessage,
    createdAt: item.createdAt,
  }
}

function detailToPreview(detail: ImageTaskDetailResponse): ImageTaskDetailResponse {
  return {
    taskId: detail.taskId,
    prompt: detail.prompt,
    imageUrl: detail.imageUrl,
    imageUrls: normalizeImageURLs(detail.imageUrls, detail.imageUrl),
    costPoints: detail.costPoints,
    costCredits: detail.costCredits,
    freeCreditsCost: detail.freeCreditsCost,
    paidCreditsCost: detail.paidCreditsCost,
    creditSource: detail.creditSource,
    status: detail.status,
    requestParams: detail.requestParams,
    errorMessage: detail.errorMessage,
    createdAt: detail.createdAt,
  }
}

function createCenteredThumbnailSlots(
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

function triggerImageDownload(blob: Blob, fileName: string): void {
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

function isWeChatBrowser(): boolean {
  if (typeof navigator === 'undefined') {
    return false
  }

  const userAgent = navigator.userAgent.toLowerCase()
  return userAgent.includes('micromessenger')
}

export default App
