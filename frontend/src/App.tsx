import { useEffect, useMemo, useRef, useState } from 'react'
import type { ChangeEvent, ClipboardEvent, DragEvent, ReactNode } from 'react'
import {
  ArrowUpDown,
  BadgeCheck,
  CheckCircle2,
  ChevronDown,
  ChevronLeft,
  ChevronRight,
  Copy,
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
  Megaphone,
  Menu,
  Moon,
  PanelLeftClose,
  PanelLeftOpen,
  RefreshCcw,
  ReceiptText,
  Send,
  Search,
  Sparkles,
  ShoppingBag,
  Sun,
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
import type { DownloadedFile } from '@/api/client'
import { AdminConsole } from './admin/AdminConsole'
import { TopNotice } from './components/TopNotice'
import type {
  AdminActionLogItem,
  AdminContentAuditItem,
  AdminImageTaskItem,
  AdminPackageItem,
  AdminPackagePayload,
  AdminStatsResponse,
  AdminUserItem,
  AnnouncementItem,
  BillingSettingsResponse,
  InviteInfoResponse,
  ListCreditLogsResponse,
  ListPointsLogsResponse,
  ListUserActivityResponse,
  CreditLogItem,
  AuthResponse,
  CaptchaResponse,
  ImageTaskDetailResponse,
  ImageTaskItem,
  PackageItem,
  ProductCategoryItem,
  ProductItem,
  PointsLogItem,
  PointsResponse,
  UploadImageResponse,
  UserActivityItem,
  UserInfo,
} from '@/types/api'
import { normalizePromptTags } from "./pages/PromptsPage";
import { OrdersPage } from "./pages/OrdersPage";
import { FlowsPage } from "./pages/FlowsPage";
import { ProfilePage } from "./pages/ProfilePage";
import { RechargePage } from "./pages/RechargePage";
import { ProductsPage } from "./pages/ProductsPage";
import { PromptsPage } from "./pages/PromptsPage";
import { HomePage } from './pages/HomePage';
import { RechargeConfirmDialog } from "./pages/RechargePage";
import { AppProvider } from './contexts/AppContext';
import type { AppContextType } from './contexts/AppContext';
import { sizeOptions, CUSTOM_SIZE_MAX_SIDE, styleOptions, qualityOptions, outputFormatOptions, HISTORY_PAGE_SIZE, FREE_REFERENCE_IMAGE_COUNT, MAX_REFERENCE_IMAGES, GENERATION_WAITING_NOTICE } from './constants/app';
import { getImageTaskStatusLabel, limitImageURLs, normalizeImageURLs, getPreviewRequestedCount, isPartialImageTaskStatus, getTaskStatusBadgeClass, formatTaskCost, getTaskCreditCost, getPreviewElapsedSeconds, formatElapsedTriplet, buildPreviewImageCandidates, createCenteredThumbnailSlots, getPreviewEmptyMessage, highlightSearchText, getGenerationStatusLabel, buildPreviewRequestOptions, parseTaskRequestParams, clampCount, normalizeElapsedSeconds, buildImageElapsedSeconds, normalizeGenerationStatus, detailToPreview, toPreview, getDownloadCacheKey, trimDownloadFileCache } from './utils/imageTasks';
import { MiniStat, PanelPagination } from './components/shared';
import { getReferenceImageNameFromURL, syncPrimaryReferenceImage, getReferenceImageExtraPointCost } from './utils/helpers';
import { TaskDetailPanel } from './components/home';

type AuthMode = 'login' | 'register' | 'reset'
type AppPage = 'home' | 'orders' | 'profile' | 'flows' | 'recharge' | 'prompts' | 'products'
type UserTheme = 'light' | 'dark'
export type FlowLogType = 'points' | 'credits'
type FlowLogDetail =
  | { type: 'points'; item: PointsLogItem }
  | { type: 'credits'; item: CreditLogItem }
  | null
export type BusyAction =
  | 'send-code'
  | 'check-register-availability'
  | 'check-reset-email'
  | 'auth'
  | 'checkin'
  | 'upload-reference'
  | 'create-task'
  | 'refresh'
  | `retry:${string}`
  | `purchase:${string}`
  | 'copy-invite-link'
  | 'redeem-code'
  | 'change-password'
  | null

type ReferenceImageSource = 'none' | 'local' | 'network'

interface ReferenceImageItem {
  url: string
  name: string
  source: Exclude<ReferenceImageSource, 'none'>
}

type GenerationStatus = 'idle' | 'running' | 'success' | 'failed'

const REGISTRATION_ENABLED = String(import.meta.env.VITE_REGISTRATION_ENABLED ?? 'true').toLowerCase() !== 'false'
const USER_THEME_STORAGE_KEY = 'image2-user-theme'
const ACTIVE_PAGE_STORAGE_KEY = 'image2-active-page'
const BRAND_ICON_URL = '/brand-avatar.jpg'
const ICP_RECORD_NUMBER = '粤ICP备2026057742号'
const ICP_RECORD_URL = 'https://beian.miit.gov.cn/'
const POLICE_RECORD_NUMBER = '粤公网安备44050702001607号'
const POLICE_RECORD_URL = 'https://beian.mps.gov.cn/#/query/webSearch?code=44050702001607'
const APP_PAGES: AppPage[] = ['home', 'orders', 'profile', 'flows', 'recharge', 'prompts', 'products']
const REGISTRATION_DAILY_LIMIT_MESSAGE = '当日注册额度已达到上限，请0点后再尝试'



interface TaskRequestSnapshot {
  prompt?: string
  size?: string
  style?: string
  count?: number
  quality?: string
  outputFormat?: string
  referenceImageUrl?: string
  referenceImageUrls?: string[]
  referenceImages?: ReferenceImageItem[]
  emailNotifyOnComplete?: boolean
}

interface PromptGalleryItem {
  id: string
  title: string
  prompt: string
  previewUrl: string
  previewThumbUrl?: string
  tags: string[]
  author: string
  status: 'approved' | 'pending' | 'rejected'
  favorites: number
  createdAt: string
  approvedAt?: string
  serialNo?: string
  isFavorited?: boolean
}

export type PromptSortMode = 'recommended' | 'favorites' | 'latest-approved' | 'serial'

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

type RechargeConfirmDialogState =
  | {
      open: false
    }
  | {
      open: true
      kind: 'package'
      title: string
      lines: string[]
      confirmLabel: string
      href?: string
    }

function escapeXml(value: string): string {
  return value
    .replace(/&/g, '&amp;')
    .replace(/</g, '&lt;')
    .replace(/>/g, '&gt;')
    .replace(/"/g, '&quot;')
    .replace(/'/g, '&apos;')
}

function readStoredUserTheme(): UserTheme {
  try {
    return window.localStorage.getItem(USER_THEME_STORAGE_KEY) === 'dark' ? 'dark' : 'light'
  } catch {
    return 'light'
  }
}

function readStoredActivePage(): AppPage {
  try {
    const value = window.localStorage.getItem(ACTIVE_PAGE_STORAGE_KEY)
    return APP_PAGES.includes(value as AppPage) ? (value as AppPage) : 'home'
  } catch {
    return 'home'
  }
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
  { page: 'recharge', label: '兑换码', icon: <KeyRound className="h-4 w-4" /> },
  { page: 'products', label: '商品', icon: <ShoppingBag className="h-4 w-4" /> },
]

const featuredPromptItems: PromptGalleryItem[] = []



const emptyPreview: ImageTaskDetailResponse = {
  taskId: '-',
  requestedCount: 0,
  prompt: '还没有生成任务，先提交一条提示词。',
  imageUrl: '',
  costPoints: 0,
  costCredits: 0,
  freeCreditsCost: 0,
  paidCreditsCost: 0,
  creditSource: 'free',
  status: 'idle',
  requestParams: '',
  createdAt: '',
  updatedAt: '',
}

const ACTIVITY_PAGE_SIZE = 10
const FLOW_LOG_PAGE_SIZE = 10
function App() {
    const [dashboardRunningTasks, setDashboardRunningTasks] = useState<any[]>([]);
    const [mode, setMode] = useState<AuthMode>(() =>
    window.location.pathname === '/reset-password'
      ? 'reset'
      : REGISTRATION_ENABLED && window.location.pathname === '/register'
        ? 'register'
        : 'login',
    )
    const [activePage, setActivePage] = useState<AppPage>(() => readStoredActivePage())
    const [userTheme, setUserTheme] = useState<UserTheme>(() => readStoredUserTheme())
    const [sidebarCollapsed, setSidebarCollapsed] = useState(false)
    const [sidebarOpen, setSidebarOpen] = useState(false)
    const [busy, setBusy] = useState<BusyAction>(null)
    const [message, setMessage] = useState('')
    const [token, setToken] = useState(() => getStoredToken())
    const [authForm, setAuthForm] = useState({
    email: '',
    code: '',
    captchaCode: '',
    username: '',
    inviteCode: readInviteCodeFromLocation(),
    account: '',
    password: '',
    confirmPassword: '',
    rememberMe: false,
    })
    const [authFormSeed, setAuthFormSeed] = useState(0)
    const [captcha, setCaptcha] = useState<CaptchaResponse | null>(null)
    const [emailCodeCountdown, setEmailCodeCountdown] = useState(0)
    const [emailCaptchaPassUntil, setEmailCaptchaPassUntil] = useState(0)
    const [registrationDailyLimitReached, setRegistrationDailyLimitReached] = useState(false)
    const [registerAvailabilityCheck, setRegisterAvailabilityCheck] = useState<{
    email: string
    username: string
    emailAvailable: boolean
    usernameAvailable: boolean
    available: boolean
    message: string
    } | null>(null)
    const [registerUsernameCheck, setRegisterUsernameCheck] = useState<{
    username: string
    available: boolean
    message: string
    } | null>(null)
    const [registerEmailCheck, setRegisterEmailCheck] = useState<{
    email: string
    available: boolean
    message: string
    } | null>(null)
    const [resetEmailCheck, setResetEmailCheck] = useState<{ email: string; available: boolean; message: string } | null>(null)
    const [changePasswordForm, setChangePasswordForm] = useState<PasswordChangeForm>({
    currentPassword: '',
    newPassword: '',
    confirmNewPassword: '',
    })

    const [emailNotifyOnComplete, setEmailNotifyOnComplete] = useState(false)
    
    const [showRegisterPassword, setShowRegisterPassword] = useState(false)
    const [currentUser, setCurrentUser] = useState<UserInfo | null>(null)
    const [adminOpen, setAdminOpen] = useState(() => window.location.pathname.startsWith('/admin'))
    const [adminStats, setAdminStats] = useState<AdminStatsResponse | null>(null)
    const [adminUsers, setAdminUsers] = useState<AdminUserItem[]>([])
    const [adminTasks, setAdminTasks] = useState<AdminImageTaskItem[]>([])
    const [adminAudits, setAdminAudits] = useState<AdminContentAuditItem[]>([])
    const [adminActionLogs, setAdminActionLogs] = useState<AdminActionLogItem[]>([])
    const [adminPackages, setAdminPackages] = useState<AdminPackageItem[]>([])
    const [points, setPoints] = useState<PointsResponse | null>(null)
    const [historyRefreshTrigger, setHistoryRefreshTrigger] = useState<{timestamp: number, resetPage?: boolean}>({ timestamp: 0 })
    const [packages, setPackages] = useState<PackageItem[]>([])
    const [billingSettings, setBillingSettings] = useState<BillingSettingsResponse | null>(null)
    const [announcements, setAnnouncements] = useState<AnnouncementItem[]>([])
    const [announcementDialogOpen, setAnnouncementDialogOpen] = useState(false)
    const [activeAnnouncementId, setActiveAnnouncementId] = useState<string | null>(null)
    const [wechatSaveGuide, setWechatSaveGuide] = useState<WechatSaveGuideState>({
    open: false,
    imageUrl: '',
    title: '',
    })
    const [rechargeConfirmDialog, setRechargeConfirmDialog] = useState<RechargeConfirmDialogState>({
    open: false,
    })
    const [serverPromptItems, setServerPromptItems] = useState<PromptGalleryItem[]>([])
    const [promptSubmission, setPromptSubmission] = useState({
    title: '',
    prompt: '',
    previewUrl: '',
    tags: [''],
    })
    const checkinInFlightRef = useRef(false)

    const isAuthed = token.length > 0
    const isAdmin = currentUser?.role === 'admin'
    const canRegister = REGISTRATION_ENABLED
    const checkedInToday = isSameShanghaiDay(points?.lastCheckinAt ?? null)
    const imageGenerationEnabled = billingSettings?.imageGenerationEnabled ?? true
    const emailNotifyPointCost = Math.max(0, Math.floor(Number(billingSettings?.emailNotifyPointCost ?? 0)))
    const imageGeneratePointCost = Math.max(0, Math.floor(Number(billingSettings?.imageGeneratePointCost ?? 20)))
    const emailNotifyAvailable = Boolean(
    isAuthed &&
      currentUser?.email &&
      (billingSettings?.emailNotifyEnabled ?? false) &&
      emailNotifyPointCost > 0,
    )
    
    const promptGalleryItems = useMemo(
    () =>
      [...serverPromptItems, ...featuredPromptItems].map((item: any) => ({
        ...item,
        tags: normalizePromptTags(item.tags ?? []),
        approvedAt: item.approvedAt ?? (item.status === 'approved' ? item.createdAt : undefined),
      })),
    [serverPromptItems],
    )
    const favoritePromptIds = useMemo(() => promptGalleryItems.filter((item) => item.isFavorited).map((item) => item.id), [promptGalleryItems])
    const unreadAnnouncementCount = announcements.filter((item) => !item.isRead).length
    const forcedAnnouncement = announcements.find((item) => item.kind === 'pinned' && !item.isRead) ?? null
    const activeAnnouncement = announcements.find((item) => item.id === activeAnnouncementId) ?? forcedAnnouncement ?? announcements[0] ?? null

    useEffect(() => {
    try {
      window.localStorage.setItem(USER_THEME_STORAGE_KEY, userTheme)
    } catch {
      // Ignore storage errors; the visual state still works for the current session.
    }
    }, [userTheme])

    useEffect(() => {
    try {
      window.localStorage.setItem(ACTIVE_PAGE_STORAGE_KEY, activePage)
    } catch {
      // Ignore storage errors; page state still works for the current session.
    }
    }, [activePage])

    useEffect(() => {
    if (!emailNotifyAvailable && emailNotifyOnComplete) {
      setEmailNotifyOnComplete(false)
    }
    }, [emailNotifyAvailable, emailNotifyOnComplete])

    useEffect(() => {
    if (!isAuthed || !forcedAnnouncement) {
      return
    }
    setActiveAnnouncementId(forcedAnnouncement.id)
    setAnnouncementDialogOpen(true)
    }, [forcedAnnouncement, isAuthed])

    useEffect(() => {
    if (!announcementDialogOpen || !activeAnnouncementId) {
      return
    }
    void markNormalAnnouncementRead(activeAnnouncementId)
    // eslint-disable-next-line react-hooks/exhaustive-deps
    }, [activeAnnouncementId, announcementDialogOpen])

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
    if (isAuthed) {
      void loadDashboard()
    } else {
      setCurrentUser(null)
      setAdminOpen(false)
      setAdminActionLogs([])
      setPoints(null)
      setAnnouncements([])
      setAnnouncementDialogOpen(false)
      setActiveAnnouncementId(null)
      setWechatSaveGuide({ open: false, imageUrl: '', title: '' })
    }
    // eslint-disable-next-line react-hooks/exhaustive-deps
    }, [isAuthed])

    useEffect(() => {
    const nextPath = isAuthed
      ? (adminOpen ? '/admin' : '/app')
      : mode === 'register' && canRegister
        ? '/register'
        : mode === 'reset'
          ? '/reset-password'
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
        setAuthForm((current: any) => {
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
      } else if (path === '/reset-password') {
        setMode('reset')
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
    if ((mode === 'register' && REGISTRATION_ENABLED) || mode === 'reset') {
      void refreshCaptcha(false)
      return
    }
    setCaptcha(null)
    setRegistrationDailyLimitReached(false)
    setRegisterAvailabilityCheck(null)
    }, [mode])

    useEffect(() => {
    if (emailCodeCountdown <= 0) {
      return
    }
    const timer = window.setInterval(() => {
      setEmailCodeCountdown((current) => Math.max(0, current - 1))
    }, 1000)
    return () => window.clearInterval(timer)
    }, [emailCodeCountdown])

    async function loadPackages() {
    try {
      const [rechargeResult, settingsResult] = await Promise.all([
        api.listPackages('recharge'),
        api.getBillingSettings(),
      ])
      setPackages((rechargeResult.items ?? []).filter((item) => item.packageType === 'recharge' && item.enabled))
      setBillingSettings(settingsResult)
    } catch (error) {
      console.error(error)
    }
    }

    async function loadDashboard(options: { showBusy?: boolean; updatePreview?: boolean; updateMessage?: boolean } = {}) {

              const { showBusy = true, updateMessage = true } = options;
              try {
                if (showBusy) setBusy('refresh');
                const meResult = await api.getMe();
                setCurrentUser(meResult);
                const [
                  pointsResult,
                  latestResult,
                  announcementsResult,
                  settingsResult
                ] = await Promise.all([
                  api.getPoints(),
                  api.listImageTasks(1, HISTORY_PAGE_SIZE),
                  api.listAnnouncements().catch(() => null),
                  api.getBillingSettings().catch(() => null)
                ]);
                
                setHistoryRefreshTrigger({ timestamp: Date.now(), resetPage: false })
                setPoints(pointsResult);
                
                if (settingsResult) {
                  setBillingSettings(settingsResult);
                }
                if (announcementsResult) {
                  setAnnouncements(announcementsResult.items);
                  const unreadPinned = announcementsResult.items.find((item: any) => item.kind === 'pinned' && !item.isRead);
                  if (unreadPinned) {
                    setActiveAnnouncementId(unreadPinned.id);
                    setAnnouncementDialogOpen(true);
                  }
                }
                
                const runningTasks = latestResult.items.filter((item: any) => item.status === 'generating' || item.status === 'running');
                setDashboardRunningTasks(runningTasks);
                
                if (updateMessage) {
                  setMessage('');
                }
              } catch (error) {
                handleApiError(error, '加载失败');
              } finally {
                if (showBusy) setBusy(null);
              }
            
    }


    async function refreshCaptcha(showMessage = false) {
    if (mode !== 'register' && mode !== 'reset') {
      return
    }
    if (mode === 'register' && !REGISTRATION_ENABLED) {
      return
    }

    try {
      const result = await api.getCaptcha()
      setCaptcha(result)
      setRegistrationDailyLimitReached(false)
      setEmailCaptchaPassUntil(0)
      setAuthForm((current: any) => ({ ...current, captchaCode: '' }))
      if (showMessage) {
        setMessage('图形验证码已刷新。')
      }
    } catch (error) {
      setCaptcha(null)
      if (isRegistrationDailyLimitError(error)) {
        setRegistrationDailyLimitReached(true)
        setMessage(REGISTRATION_DAILY_LIMIT_MESSAGE)
        return
      }
      setMessage(error instanceof Error ? error.message : '图形验证码加载失败')
    }
    }

    async function handleSendCode() {
    if (mode === 'register' && registrationDailyLimitReached) {
      setMessage(REGISTRATION_DAILY_LIMIT_MESSAGE)
      return
    }
    if (!authForm.email.trim()) {
      setMessage('请先填写邮箱。')
      return
    }
    const hasCaptchaPass = Date.now() < emailCaptchaPassUntil
    if (!hasCaptchaPass && (!captcha?.captchaId || !authForm.captchaCode.trim())) {
      setMessage('请先输入图形验证码。')
      return
    }
    if (mode === 'register' && !REGISTRATION_ENABLED) {
      setMessage('注册功能暂时关闭。')
      return
    }
    if (mode === 'register') {
      if (!authForm.username.trim()) {
        setMessage('请先填写用户名。')
        return
      }
      const checked = await handleCheckRegisterAvailability()
      if (!checked) {
        return
      }
    }
    if (mode === 'reset') {
      const checked = await handleCheckResetEmail()
      if (!checked) {
        return
      }
    }

    try {
      setBusy('send-code')
      const result = await api.sendEmailCode({
        email: authForm.email,
        scene: mode === 'reset' ? 'reset_password' : 'register',
        captchaId: hasCaptchaPass ? undefined : captcha?.captchaId,
        captchaCode: hasCaptchaPass ? undefined : authForm.captchaCode,
      })
      setAuthForm((current: any) => ({ ...current, code: '' }))
      setEmailCaptchaPassUntil(Date.now() + result.expiresIn * 1000)
      setEmailCodeCountdown(60)
      setMessage(`验证码已发送，请查收最新一封邮件。有效时间 ${Math.floor(result.expiresIn / 60)} 分钟。`)
    } catch (error) {
      if (isRegistrationDailyLimitError(error)) {
        setRegistrationDailyLimitReached(true)
        setCaptcha(null)
        setMessage(REGISTRATION_DAILY_LIMIT_MESSAGE)
        return
      }
      setMessage(error instanceof Error ? error.message : '发送验证码失败')
    } finally {
      setBusy(null)
    }
    }

    async function handleCheckResetEmail(): Promise<boolean> {
    const email = authForm.email.trim()
    if (mode !== 'reset' || !email) {
      return false
    }
    try {
      setBusy('check-reset-email')
      const result = await api.checkResetPasswordEmail({ email })
      const message = result.message || (result.available ? '该邮箱可以重置密码。' : '该邮箱暂不可重置密码。')
      setResetEmailCheck({ email, available: result.available, message })
      if (!result.available) {
        setMessage(message)
      }
      return result.available
    } catch (error) {
      const message = error instanceof Error ? error.message : '邮箱检查失败'
      setResetEmailCheck({ email, available: false, message })
      setMessage(message)
      return false
    } finally {
      setBusy(null)
    }
    }

    async function handleCheckRegisterAvailability(): Promise<boolean> {
    const email = authForm.email.trim()
    const username = authForm.username.trim()
    if (mode !== 'register' || !email || !username) {
      return false
    }
    try {
      setBusy('check-register-availability')
      const result = await api.checkRegisterAvailability({ email, username })
      const next = {
        email,
        username,
        emailAvailable: result.emailAvailable,
        usernameAvailable: result.usernameAvailable,
        available: result.available,
        message: result.message || (result.available ? '邮箱和用户名都可以注册。' : '当前信息暂不可注册。'),
      }
      setRegisterAvailabilityCheck(next)
      if (!result.available) {
        setMessage(next.message)
      }
      return result.available
    } catch (error) {
      const next = {
        email,
        username,
        emailAvailable: false,
        usernameAvailable: false,
        available: false,
        message: error instanceof Error ? error.message : '注册信息检查失败',
      }
      setRegisterAvailabilityCheck(next)
      setMessage(next.message)
      return false
    } finally {
      setBusy(null)
    }
    }

    async function handleCheckRegisterUsername(): Promise<boolean> {
    const username = authForm.username.trim()
    const email = authForm.email.trim() || '__placeholder__@example.com'
    if (mode !== 'register' || !username) {
      setRegisterUsernameCheck(null)
      return false
    }
    try {
      setBusy('check-register-availability')
      const result = await api.checkRegisterAvailability({ email, username })
      const available = result.usernameAvailable
      setRegisterUsernameCheck({
        username,
        available,
        message: available ? '该用户名可以注册。' : '该用户名已被使用，请更换其他用户名。',
      })
      return available
    } catch (error) {
      setRegisterUsernameCheck({
        username,
        available: false,
        message: error instanceof Error ? error.message : '用户名检查失败',
      })
      return false
    } finally {
      setBusy(null)
    }
    }

    async function handleCheckRegisterEmail(): Promise<boolean> {
    const email = authForm.email.trim()
    const username = authForm.username.trim() || 'placeholder123'
    if (mode !== 'register' || !email) {
      setRegisterEmailCheck(null)
      return false
    }
    try {
      setBusy('check-register-availability')
      const result = await api.checkRegisterAvailability({ email, username })
      const available = result.emailAvailable
      setRegisterEmailCheck({
        email,
        available,
        message: available ? '该邮箱可以注册。' : '该邮箱已被注册，请直接登录。',
      })
      return available
    } catch (error) {
      setRegisterEmailCheck({
        email,
        available: false,
        message: error instanceof Error ? error.message : '邮箱检查失败',
      })
      return false
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
        const checked = await handleCheckRegisterAvailability()
        if (!checked) {
          return
        }
        result = await api.register({
          email: authForm.email,
          code: authForm.code,
          username: authForm.username,
          password: authForm.password,
          inviteCode: authForm.inviteCode.trim() || undefined,
        })
      } else if (mode === 'reset') {
        if (!authForm.email || !authForm.code || !authForm.password || !authForm.confirmPassword) {
          setMessage('请把重置密码所需字段补齐。')
          return
        }
        if (authForm.password !== authForm.confirmPassword) {
          setMessage('两次输入的密码不一致。')
          return
        }
        await api.resetPassword({
          email: authForm.email,
          code: authForm.code,
          password: authForm.password,
          confirmPassword: authForm.confirmPassword,
        })
        setMode('login')
        setAuthForm((current) => ({
          ...current,
          account: current.email,
          code: '',
          captchaCode: '',
          password: '',
          confirmPassword: '',
        }))
        setEmailCodeCountdown(0)
        setEmailCaptchaPassUntil(0)
        setResetEmailCheck(null)
        setMessage('密码已重置，请使用新密码登录。')
        return
      } else {
        if (!authForm.account || !authForm.password) {
          setMessage('请把登录所需字段补齐。')
          return
        }
        result = await api.login({
          account: authForm.account,
          password: authForm.password,
          rememberMe: authForm.rememberMe,
        })
      }

      saveStoredToken(result.accessToken)
      setToken(result.accessToken)
      setCurrentUser(result.user)
      setAdminOpen(result.user.role === 'admin')
      setPoints({
        balance: result.user.pointsBalance,
        creditsBalance: getTotalCredits(result.user),
        freeCredits: result.user.freeCredits,
        paidCredits: result.user.paidCredits,
        totalCredits: getTotalCredits(result.user),
        streak: result.user.checkinStreak,
        lastCheckinAt: result.user.lastCheckinAt ?? null,
        checkinPointsTotal: 0,
        checkinCreditsTotal: 0,
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

    async function confirmPackageRecharge(packageCode: string) {
    const selectedPackage = packages.find((item) => item.code === packageCode)
    if (!selectedPackage?.productUrl) {
      setRechargeConfirmDialog({ open: false })
      setMessage('该兑换码商品暂未配置购买链接，请联系管理员处理。')
      return
    }

    window.open(selectedPackage.productUrl, '_blank', 'noopener,noreferrer')
    setRechargeConfirmDialog({ open: false })
    setMessage('已打开兑换码购买平台。购买完成后，请获取兑换码并返回本站兑换。')
    }

      function onTaskCreated(result: any, requestSnapshot: TaskRequestSnapshot, startedAt: number) {
    const taskCount = requestSnapshot.count
    const now = new Date().toISOString()
    const requestedCount = Number(result.requestedCount ?? taskCount)
    const resultImageURLs = limitImageURLs(normalizeImageURLs(result.imageUrls, result.imageUrl), requestedCount)
    setHistoryRefreshTrigger({ timestamp: Date.now(), resetPage: true })
    setPoints((current: any) => {
      if (!current) {
        return current
      }
      return {
        ...current,
        balance: result.pointsBalance ?? current.balance,
        creditsBalance: getResponseCreditsBalance(result, current.totalCredits),
        freeCredits: result.freeCreditsBalance ?? current.freeCredits,
        paidCredits: result.paidCreditsBalance ?? current.paidCredits,
        totalCredits: getResponseCreditsBalance(result, current.totalCredits),
      }
    })
    const generationStatus = normalizeGenerationStatus(result.status)
    if (generationStatus === 'failed') {
      setMessage(`任务已提交，任务编号：${result.taskId}，${result.errorMessage || '但生成失败。'}`)
    }
  }

    

    

    

    

    

    

    

    

    

    

    async function loadAdminData() {
    if (!isAdmin) {
      return
    }
    try {
      setBusy('refresh')
      const [stats, users, tasks, audits, packages, actionLogs] = await Promise.all([
        api.adminStats(),
        api.adminListUsers(1, 8),
        api.adminListImageTasks(1, 8),
        api.adminListContentAudits(1, 8),
        api.adminListPackages(1, 50),
        api.adminListActionLogs(1, 20, 'billing_package'),
      ])
      setAdminStats(stats)
      setAdminUsers(users.items)
      setAdminTasks(tasks.items)
      setAdminAudits(audits.items)
      setAdminActionLogs(actionLogs.items)
      setAdminPackages((packages.items ?? []).filter((item) => item.packageType === 'recharge'))
    } catch (error) {
      handleApiError(error, '后台数据加载失败')
    } finally {
      setBusy(null)
    }
    }

    function handleToggleAdmin() {
    setAdminOpen((current: any) => !current)
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
      handleApiError(error, '保存商品套餐失败')
    } finally {
      setBusy(null)
    }
    }

    function handleLogout() {

              // api.logout()
              setToken('');
              setCurrentUser(null);
              setPoints(null);
              setBillingSettings(null);
              setAnnouncements([]);
              setActiveAnnouncementId(null);
              setAnnouncementDialogOpen(false);
    }

    function handleLogoutRequest() {
    if (window.confirm('确定要退出登录吗？')) {
      handleLogout()
    }
    }

    function handleApiError(error: unknown, fallback: string) {
    if (shouldForceLogout(error)) {
      handleLogout()
      setMessage(error instanceof APIError && error.status === 403 ? '账号已被禁用，请联系管理员。' : '登录已过期，请重新登录。')
      return
    }
    setMessage(error instanceof Error ? error.message : fallback)
    }

    function openAnnouncements() {
    const firstUnread = announcements.find((item) => !item.isRead)
    setActiveAnnouncementId(firstUnread?.id ?? announcements[0]?.id ?? null)
    setAnnouncementDialogOpen(true)
    if (announcements.length === 0) {
      void refreshAnnouncements()
    }
    }

    async function refreshAnnouncements() {
    try {
      const resp = await api.listAnnouncements()
      setAnnouncements(resp.items)
      if (!activeAnnouncementId && resp.items.length > 0) {
        setActiveAnnouncementId(resp.items[0].id)
      }
    } catch (error) {
      handleApiError(error, '公告加载失败')
    }
    }

    async function markActiveAnnouncementRead() {
    const targetAnnouncement = activeAnnouncement
    if (!targetAnnouncement) {
      return
    }
    try {
      const resp = await api.markAnnouncementRead(targetAnnouncement.id)
      setAnnouncements((current) =>
        current.map((item) =>
          item.id === targetAnnouncement.id
            ? { ...item, isRead: true, readAt: resp.readAt }
            : item,
        ),
      )
      const nextPinned = announcements.find((item) => item.id !== targetAnnouncement.id && item.kind === 'pinned' && !item.isRead)
      if (nextPinned) {
        setActiveAnnouncementId(nextPinned.id)
        return
      }
      const nextUnread = announcements.find((item) => item.id !== targetAnnouncement.id && !item.isRead)
      if (nextUnread) setActiveAnnouncementId(nextUnread.id)
    } catch (error) {
      handleApiError(error, '标记公告已读失败')
    }
    }

    async function markNormalAnnouncementRead(id: string) {
    const targetAnnouncement = announcements.find((item) => item.id === id)
    if (!targetAnnouncement || targetAnnouncement.kind === 'pinned' || targetAnnouncement.isRead) {
      return
    }
    try {
      const resp = await api.markAnnouncementRead(targetAnnouncement.id)
      setAnnouncements((current) =>
        current.map((item) =>
          item.id === targetAnnouncement.id
            ? { ...item, isRead: true, readAt: resp.readAt }
            : item,
        ),
      )
    } catch (error) {
      handleApiError(error, '标记公告已读失败')
    }
    }

    function navigateToPage(page: AppPage) {
    setActivePage(page)
    setSidebarOpen(false)
    }





    // ── Unified context value for child pages ──────────────────────────
    // App.tsx owns all shared state; pages read/write through this context.
    const appContextValue = useMemo<AppContextType>(() => ({
      userTheme, setUserTheme,
      token, setToken: (t: string) => setToken(t),
      currentUser, setCurrentUser,
      points, setPoints,
      billingSettings, setBillingSettings,
      busy, setBusy,
      message, setMessage,
      logout: handleLogout,
      handleApiError,
    }), [userTheme, token, currentUser, points, billingSettings, busy, message])

    if (!isAuthed) {
    return (
      <AppProvider value={appContextValue}>
      <div className="auth-light-shell min-h-screen bg-slate-950 text-slate-100">
        <TopNotice message={message} onDismiss={() => setMessage('')} />
        <main className="flex min-h-screen flex-col items-center justify-center px-4 py-8">
          <section className="w-full max-w-md rounded-lg border border-slate-800 bg-slate-900/85 p-6 shadow-panel">
            <div className="text-center">
              <div className="mx-auto inline-flex items-center gap-3">
                <div className="flex h-12 w-12 overflow-hidden rounded-md border border-slate-800 bg-slate-950">
                  <img className="h-full w-full object-cover" src={BRAND_ICON_URL} alt="" />
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
                {mode !== 'login' ? (
                  <>
                    {mode === 'register' && registrationDailyLimitReached ? (
                      <div className="rounded-lg border border-amber-400/40 bg-amber-500/10 px-3 py-2 text-sm text-amber-100">
                        {REGISTRATION_DAILY_LIMIT_MESSAGE}
                      </div>
                    ) : null}
                    {mode === 'register' ? (
                      <Field label="用户名" icon={<UserRoundPlus className="h-4 w-4" />}>
                        <input
                          name={`register_username_${authFormSeed}`}
                          autoComplete="off"
                          value={authForm.username}
                          onBlur={() => {
                            if (authForm.username.trim()) {
                              void handleCheckRegisterUsername()
                            }
                          }}
                          onChange={(event) => {
                            setAuthForm((current: any) => ({ ...current, username: event.target.value }))
                            setRegisterAvailabilityCheck(null)
                            setRegisterUsernameCheck(null)
                          }}
                          className="w-full rounded-md border border-slate-700 bg-slate-950 px-3 py-2 text-slate-100 outline-none transition placeholder:text-slate-500 focus:border-violet-400"
                          placeholder="请输入用户名（英文或者数字组成）"
                        />
                        {registerUsernameCheck?.username === authForm.username.trim() ? (
                          <div
                            className={`mt-2 text-xs ${
                              registerUsernameCheck.available ? 'text-emerald-300' : 'text-amber-300'
                            }`}
                          >
                            {registerUsernameCheck.message}
                          </div>
                        ) : null}
                      </Field>
                    ) : null}
                    <Field label="邮箱" icon={<Mail className="h-4 w-4" />}>
                      <input
                        name={`${mode}_email_${authFormSeed}`}
                        autoComplete="off"
                        value={authForm.email}
                        onBlur={() => {
                          if (mode === 'register' && authForm.email.trim()) {
                            void handleCheckRegisterEmail()
                          }
                          if (mode === 'reset' && authForm.email.trim()) {
                            void handleCheckResetEmail()
                          }
                        }}
                        onChange={(event) => {
                          setAuthForm((current: any) => ({ ...current, email: event.target.value }))
                          setEmailCodeCountdown(0)
                          setEmailCaptchaPassUntil(0)
                          setRegisterAvailabilityCheck(null)
                          setRegisterEmailCheck(null)
                          setResetEmailCheck(null)
                        }}
                        className="w-full rounded-md border border-slate-700 bg-slate-950 px-3 py-2 text-slate-100 outline-none transition placeholder:text-slate-500 focus:border-violet-400"
                        placeholder="请输入邮箱"
                      />
                      {mode === 'register' && registerEmailCheck?.email === authForm.email.trim() ? (
                        <div className={`mt-2 text-xs ${registerEmailCheck.available ? 'text-emerald-300' : 'text-amber-300'}`}>
                          {registerEmailCheck.message}
                        </div>
                      ) : null}
                      {mode === 'reset' && resetEmailCheck?.email === authForm.email.trim() ? (
                        <div className={`mt-2 text-xs ${resetEmailCheck.available ? 'text-emerald-300' : 'text-amber-300'}`}>
                          {resetEmailCheck.message}
                        </div>
                      ) : null}
                    </Field>
                    <Field label="图形验证码" icon={<ShieldCheck className="h-4 w-4" />}>
                      <div className="flex min-w-0 items-stretch gap-2">
                        <input
                          name={`register_captcha_${authFormSeed}`}
                          autoComplete="off"
                          value={authForm.captchaCode}
                          onChange={(event) =>
                            setAuthForm((current) => ({
                              ...current,
                              captchaCode: event.target.value.toUpperCase().replace(/[^A-Z0-9]/g, '').slice(0, 4),
                            }))
                          }
                          className="min-w-0 flex-1 rounded-md border border-slate-700 bg-slate-950 px-3 py-2 text-slate-100 outline-none transition placeholder:text-slate-500 focus:border-violet-400"
                          placeholder="输入右侧字符"
                        />
                        <button
                          type="button"
                          onClick={() => void refreshCaptcha(true)}
                          className="flex h-10 w-32 shrink-0 items-center justify-center overflow-hidden rounded-md border border-slate-700 bg-slate-950 text-slate-200 transition hover:border-violet-400 disabled:cursor-not-allowed disabled:opacity-60 min-[380px]:w-36"
                          title="点击刷新图形验证码"
                        >
                          {captcha?.image ? (
                            <img src={captcha.image} alt="图形验证码" className="h-full w-full object-contain" />
                          ) : (
                            <span className="inline-flex items-center gap-2 px-3 text-sm">
                              <RefreshCcw className="h-4 w-4" />
                              刷新
                            </span>
                          )}
                        </button>
                      </div>
                    </Field>
                    <Field label="验证码" icon={<KeyRound className="h-4 w-4" />}>
                      <div className="flex min-w-0 items-stretch gap-2">
                        <input
                          name={`register_code_${authFormSeed}`}
                          autoComplete="one-time-code"
                          inputMode="numeric"
                          maxLength={4}
                          pattern="[0-9]{4}"
                          value={authForm.code}
                          onChange={(event) =>
                            setAuthForm((current) => ({
                              ...current,
                              code: event.target.value.replace(/\D/g, '').slice(0, 4),
                            }))
                          }
                          className="min-w-0 flex-1 rounded-md border border-slate-700 bg-slate-950 px-3 py-2 text-slate-100 outline-none transition placeholder:text-slate-500 focus:border-violet-400"
                          placeholder="请输入验证码"
                        />
                        <button
                          type="button"
                          onClick={handleSendCode}
                          disabled={
                            (mode === 'register' && registrationDailyLimitReached) ||
                            busy === 'send-code' ||
                            busy === 'check-reset-email' ||
                            busy === 'check-register-availability' ||
                            emailCodeCountdown > 0
                          }
                          className="inline-flex h-10 w-32 shrink-0 items-center justify-center gap-1.5 rounded-md border border-slate-700 bg-slate-950 px-2 text-sm text-slate-200 transition hover:border-violet-400 hover:text-white disabled:cursor-not-allowed disabled:opacity-60 min-[380px]:w-36"
                        >
                          {busy === 'send-code' ? (
                            <LoaderCircle className="h-4 w-4 animate-spin" />
                          ) : (
                            <Send className="h-4 w-4" />
                          )}
                          {emailCodeCountdown > 0 ? `${emailCodeCountdown}s 后重发` : '发送验证码'}
                        </button>
                      </div>
                    </Field>
                    {mode === 'register' ? (
                      <>
                        <Field label="邀请码（选填）" icon={<Gift className="h-4 w-4" />}>
                          <input
                            name={`register_invite_${authFormSeed}`}
                            autoComplete="off"
                            value={authForm.inviteCode}
                            onChange={(event) =>
                              setAuthForm((current) => ({
                                ...current,
                                inviteCode: event.target.value.toUpperCase().replace(/[^A-Z0-9]/g, '').slice(0, 16),
                              }))
                            }
                            className="w-full rounded-md border border-slate-700 bg-slate-950 px-3 py-2 text-slate-100 outline-none transition placeholder:text-slate-500 focus:border-violet-400"
                            placeholder={`填写邀请码可额外获得 ${billingSettings?.registrationInviteBonusPoints ?? 0} 积分`}
                          />
                        </Field>
                      </>
                    ) : null}
                    <Field label="密码" icon={<KeyRound className="h-4 w-4" />}>
                      <div className="relative">
                        <input
                          name={`${mode}_password_${authFormSeed}`}
                          autoComplete="new-password"
                          type={showRegisterPassword ? 'text' : 'password'}
                          value={authForm.password}
                          onChange={(event) => setAuthForm((current: any) => ({ ...current, password: event.target.value }))}
                          className="w-full rounded-md border border-slate-700 bg-slate-950 py-2 pl-3 pr-10 text-slate-100 outline-none transition placeholder:text-slate-500 focus:border-violet-400"
                          placeholder="请设置至少 8 位密码"
                        />
                        <button
                          type="button"
                          onClick={() => setShowRegisterPassword((current: any) => !current)}
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
                          name={`${mode}_confirm_password_${authFormSeed}`}
                          autoComplete="new-password"
                          type={showRegisterPassword ? 'text' : 'password'}
                          value={authForm.confirmPassword}
                          onChange={(event) =>
                            setAuthForm((current: any) => ({ ...current, confirmPassword: event.target.value }))
                          }
                          className="w-full rounded-md border border-slate-700 bg-slate-950 py-2 pl-3 pr-10 text-slate-100 outline-none transition placeholder:text-slate-500 focus:border-violet-400"
                          placeholder="请再次输入密码"
                        />
                        <button
                          type="button"
                          onClick={() => setShowRegisterPassword((current: any) => !current)}
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
                        onChange={(event) => setAuthForm((current: any) => ({ ...current, account: event.target.value }))}
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
                        onChange={(event) => setAuthForm((current: any) => ({ ...current, password: event.target.value }))}
                        className="w-full rounded-md border border-slate-700 bg-slate-950 px-3 py-2 text-slate-100 outline-none transition placeholder:text-slate-500 focus:border-violet-400"
                        placeholder="请输入密码"
                      />
                    </Field>
                    <label className="flex items-center justify-between gap-3 rounded-md border border-slate-200 bg-white px-3 py-2.5 text-sm text-slate-600 shadow-sm transition hover:border-violet-200 hover:bg-violet-50/40">
                      <span className="min-w-0">
                        <span className="font-medium text-slate-700">记住我</span>
                        <span className="ml-2 text-xs text-slate-400">勾选后登录状态保留 7 天</span>
                      </span>
                      <input
                        type="checkbox"
                        checked={authForm.rememberMe}
                        onChange={(event) => setAuthForm((current: any) => ({ ...current, rememberMe: event.target.checked }))}
                        className="h-4 w-4 shrink-0 rounded border-slate-300 bg-white text-violet-500 accent-violet-500"
                      />
                    </label>
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
                  ) : mode === 'reset' ? (
                    <KeyRound className="h-4 w-4" />
                  ) : (
                    <LogIn className="h-4 w-4" />
                  )}
                  {mode === 'register' ? '注册' : mode === 'reset' ? '重置密码' : '登录'}
                </button>
                {mode === 'login' ? (
                  <div className="flex items-center justify-center gap-2 rounded-md border border-emerald-100 bg-emerald-50/80 px-3 py-2 text-xs text-emerald-700">
                    <ShieldCheck className="h-3.5 w-3.5 shrink-0" />
                    <span>本站为正规网站，已完成工信部ICP备案和公安备案</span>
                  </div>
                ) : null}
              </form>

              <div className="mt-5 border-t border-slate-800 pt-4 text-center text-sm text-slate-400">
                {mode === 'register' || mode === 'reset' ? (
                  canRegister || mode === 'reset' ? (
                    <>
                      {mode === 'register' ? '已有账号？' : '想起密码了？'}
                      <button
                        type="button"
                        onClick={() => {
                          setMode('login')
                          setMessage('')
                          setShowRegisterPassword(false)
                          setResetEmailCheck(null)
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
                      <span className="mx-2 text-slate-600">/</span>
                      <button
                        type="button"
                        onClick={() => {
                          setMode('reset')
                          setMessage('')
                          setShowRegisterPassword(false)
                          setResetEmailCheck(null)
                        }}
                        className="font-medium text-violet-300 transition hover:text-violet-200"
                      >
                        忘记密码？
                      </button>
                    </>
                  ) : (
                    <>
                      <span>当前仅支持登录。</span>
                      <button
                        type="button"
                        onClick={() => {
                          setMode('reset')
                          setMessage('')
                          setShowRegisterPassword(false)
                          setResetEmailCheck(null)
                        }}
                        className="ml-2 font-medium text-violet-300 transition hover:text-violet-200"
                      >
                        忘记密码？
                      </button>
                    </>
                  )
                )}
              </div>
            </div>
          </section>
          <IcpFooter className="mt-6" />
        </main>
      </div>
      </AppProvider>
    )
    }

    if (isAuthed && adminOpen) {
    return (
      <AppProvider value={appContextValue}>
      <AdminConsole
        currentUser={currentUser}
        onBackToApp={handleExitAdmin}
        onLogout={handleLogoutRequest}
      />
      </AppProvider>
    )
    }

    return (
    <AppProvider value={appContextValue}>
    <div
      className="user-glass-shell min-h-screen w-full max-w-[100dvw] overflow-x-hidden bg-slate-950 text-slate-100"
      data-user-theme={userTheme}
    >
      <TopNotice message={message} onDismiss={() => setMessage('')} />
      <div
        className="app-shell"
        data-sidebar-open={sidebarOpen ? 'true' : 'false'}
        data-sidebar-collapsed={sidebarCollapsed ? 'true' : 'false'}
      >
        {sidebarOpen ? (
          <button
            type="button"
            aria-label="关闭侧边栏"
            onClick={() => setSidebarOpen(false)}
            className="app-shell__scrim"
          />
        ) : null}
        <AppSidebar
          activePage={activePage}
          collapsed={sidebarCollapsed}
          open={sidebarOpen}
          onNavigate={navigateToPage}
          onToggleCollapsed={() => {
            setSidebarCollapsed((current: any) => !current)
          }}
          onCloseMobile={() => {
            setSidebarOpen(false)
          }}
        />

        <AppTopBar
          sidebarOpen={sidebarOpen}
          activePage={activePage}
          userTheme={userTheme}
          currentUser={currentUser}
          onOpenSidebar={() => {
            setSidebarOpen(true)
          }}
          onToggleUserTheme={() => {
            setUserTheme((current: any) => (current === 'light' ? 'dark' : 'light'))
          }}
          onNavigate={navigateToPage}
          onToggleAdmin={handleToggleAdmin}
          adminOpen={adminOpen}
          onLogout={handleLogoutRequest}
          unreadAnnouncementCount={unreadAnnouncementCount}
          onOpenAnnouncements={openAnnouncements}
        />

        <main className="app-shell__main min-w-0 overflow-x-hidden px-3 py-4 sm:px-4 lg:px-6">
            <div className={activePage === 'home' ? 'block' : 'hidden'}>
              <HomePage
                onTaskCreated={onTaskCreated}
                setMessage={setMessage}
                setBusy={setBusy}
                handleApiError={handleApiError}
                activePage={activePage}
                points={points}
                billingSettings={billingSettings}
                emailNotifyAvailable={emailNotifyAvailable}
                emailNotifyOnComplete={emailNotifyOnComplete}
                setEmailNotifyOnComplete={setEmailNotifyOnComplete}
                emailNotifyPointCost={emailNotifyPointCost}
                busy={busy}
                clampCount={clampCount}
                emptyPreview={emptyPreview}
                toPreview={toPreview}
                detailToPreview={detailToPreview}
                getDownloadCacheKey={getDownloadCacheKey}
                trimDownloadFileCache={trimDownloadFileCache}
                triggerImageDownload={triggerImageDownload}
                isWeChatBrowser={isWeChatBrowser}
                parseTaskRequestParams={parseTaskRequestParams}
                parsePreviewSizeState={parsePreviewSizeState}
                normalizeReferenceImagesFromSnapshot={normalizeReferenceImagesFromSnapshot}
                limitImageURLs={limitImageURLs}
                normalizeImageURLs={normalizeImageURLs}
                historyRefreshTrigger={historyRefreshTrigger}
                isAuthed={isAuthed}
                imageGenerationEnabled={imageGenerationEnabled}
                dashboardRunningTasks={dashboardRunningTasks}
                onDashboardRefresh={() => loadDashboard({ showBusy: false, updatePreview: false, updateMessage: false })}
              />
            </div>

            {activePage === 'orders' ? <OrdersPage /> : null}

            {activePage === 'profile' ? <ProfilePage /> : null}

            {activePage === 'flows' ? <FlowsPage /> : null}

            {activePage === 'recharge' ? <RechargePage /> : null}

            {activePage === 'prompts' ? <PromptsPage /> : null}
            {activePage === 'products' ? <ProductsPage /> : null}
            <IcpFooter className="mt-8" />
        </main>
      </div>

      
      

      {announcementDialogOpen ? (
        <AnnouncementDialog
          items={announcements}
          activeItem={activeAnnouncement}
          activeId={activeAnnouncementId}
          forced={Boolean(forcedAnnouncement)}
          onSelect={(id) => {
            setActiveAnnouncementId(id)
          }}
          onClose={() => setAnnouncementDialogOpen(false)}
          onMarkRead={() => void markActiveAnnouncementRead()}
        />
      ) : null}

    </div>
    </AppProvider>
    )
}

function IcpFooter({ className = '' }: { className?: string }) {
  return (
    <footer className={`flex flex-wrap items-center justify-center gap-x-3 gap-y-1 text-center text-xs text-slate-500 ${className}`}>
      <a
        href={ICP_RECORD_URL}
        target="_blank"
        rel="noreferrer"
        className="transition hover:text-slate-700 dark:hover:text-slate-300"
      >
        {ICP_RECORD_NUMBER}
      </a>
      <a
        href={POLICE_RECORD_URL}
        target="_blank"
        rel="noreferrer"
        className="inline-flex items-center gap-1 transition hover:text-slate-700 dark:hover:text-slate-300"
      >
        <img src="/police-badge.png" alt="" aria-hidden="true" className="h-3.5 w-3.5 shrink-0 object-contain" />
        {POLICE_RECORD_NUMBER}
      </a>
    </footer>
  )
}

function AnnouncementDialog({
  items,
  activeItem,
  activeId,
  forced,
  onSelect,
  onClose,
  onMarkRead,
}: {
  items: AnnouncementItem[]
  activeItem: AnnouncementItem | null
  activeId: string | null
  forced: boolean
  onSelect: (id: string) => void
  onClose: () => void
  onMarkRead: () => void
}) {
  const pinnedItems = items.filter((item) => item.kind === 'pinned')
  const normalItems = items.filter((item) => item.kind !== 'pinned')

  return (
    <div className="fixed inset-0 z-50 flex items-center justify-center bg-slate-950/80 px-4 py-6 backdrop-blur-sm" role="dialog" aria-modal="true">
      {!forced ? (
        <button type="button" className="absolute inset-0 cursor-default" aria-label="关闭公告" onClick={onClose} />
      ) : null}
      <div className="relative flex max-h-full w-full max-w-4xl flex-col overflow-hidden rounded-lg border border-slate-700 bg-slate-950 shadow-2xl">
        <div className="flex items-center justify-between gap-3 border-b border-slate-800 px-4 py-3">
          <div className="min-w-0">
            <div className="flex items-center gap-2 text-base font-medium text-slate-50">
              <Megaphone className="h-4 w-4 text-amber-300" />
              公告
            </div>
            <p className="mt-1 text-xs text-slate-500">
              {forced ? '请先阅读置顶公告并标记已读。' : '查看平台置顶公告和普通公告。'}
            </p>
          </div>
          {!forced ? (
            <button
              type="button"
              onClick={onClose}
              className="inline-flex h-8 w-8 shrink-0 items-center justify-center rounded-md border border-slate-700 text-slate-300 transition hover:border-slate-500 hover:text-white"
              aria-label="关闭公告"
            >
              <XCircle className="h-4 w-4" />
            </button>
          ) : null}
        </div>

        <div className="grid min-h-0 flex-1 overflow-hidden md:grid-cols-[260px_minmax(0,1fr)]">
          <div className="order-2 min-h-0 max-h-[34vh] overflow-y-auto border-t border-slate-800 p-3 md:order-1 md:max-h-none md:border-r md:border-t-0">
            {items.length ? (
              <div className="space-y-3">
                {pinnedItems.length ? (
                  <AnnouncementListGroup title="置顶公告" items={pinnedItems} activeId={activeId} onSelect={onSelect} />
                ) : null}
                {normalItems.length ? (
                  <AnnouncementListGroup title="普通公告" items={normalItems} activeId={activeId} onSelect={onSelect} />
                ) : null}
              </div>
            ) : (
              <div className="rounded-md border border-dashed border-slate-700 bg-slate-900/60 p-4 text-sm text-slate-500">
                暂无公告
              </div>
            )}
          </div>

          <div className="order-1 flex min-h-0 flex-col md:order-2">
            <div className="min-h-0 flex-1 overflow-y-auto p-4">
              {activeItem ? (
                <article>
                  <div className="flex flex-wrap items-center gap-2">
                    <span className={`rounded-md px-2 py-1 text-xs ${activeItem.kind === 'pinned' ? 'bg-amber-500/15 text-amber-200' : 'bg-slate-800 text-slate-300'}`}>
                      {activeItem.kind === 'pinned' ? '置顶' : '普通'}
                    </span>
                    <span className={`rounded-md px-2 py-1 text-xs ${activeItem.isRead ? 'bg-emerald-500/15 text-emerald-200' : 'bg-rose-500/15 text-rose-200'}`}>
                      {activeItem.isRead ? '已读' : '未读'}
                    </span>
                    <span className="text-xs text-slate-500">更新于 {formatDateTime(activeItem.updatedAt)}</span>
                  </div>
                  <h2 className="mt-4 text-xl font-semibold text-slate-50">{activeItem.title}</h2>
                  <div className="mt-4 whitespace-pre-wrap break-words text-sm leading-7 text-slate-300">
                    {activeItem.content}
                  </div>
                  {activeItem.imageUrl ? (
                    <div className="mt-6 border-t border-slate-800 pt-5">
                      <img
                        src={activeItem.imageUrl}
                        alt={`${activeItem.title} 底部图片`}
                        className="max-h-[420px] max-w-full rounded-lg border border-slate-700 bg-slate-900 object-contain"
                      />
                    </div>
                  ) : null}
                </article>
              ) : (
                <div className="flex min-h-[240px] items-center justify-center rounded-md border border-dashed border-slate-700 bg-slate-900/50 text-sm text-slate-500">
                  请选择一条公告
                </div>
              )}
            </div>

            <div className="flex items-center justify-end gap-2 border-t border-slate-800 px-4 py-3">
              {!forced ? (
                <button
                  type="button"
                  onClick={onClose}
                  className="inline-flex h-10 items-center justify-center rounded-md border border-slate-700 px-4 text-sm text-slate-300 transition hover:border-slate-500 hover:text-white"
                >
                  关闭
                </button>
              ) : null}
              <button
                type="button"
                onClick={onMarkRead}
                disabled={!activeItem || activeItem.isRead}
                className="inline-flex h-10 items-center justify-center gap-2 rounded-md border border-emerald-500/40 bg-emerald-500/15 px-4 text-sm text-emerald-100 transition hover:border-emerald-300 hover:text-white disabled:cursor-not-allowed disabled:opacity-50"
              >
                <CheckCircle2 className="h-4 w-4" />
                标记已读
              </button>
            </div>
          </div>
        </div>
      </div>
    </div>
  )
}

function AnnouncementListGroup({
  title,
  items,
  activeId,
  onSelect,
}: {
  title: string
  items: AnnouncementItem[]
  activeId: string | null
  onSelect: (id: string) => void
}) {
  return (
    <div>
      <div className="mb-2 text-xs text-slate-500">{title}</div>
      <div className="space-y-2">
        {items.map((item) => {
          const active = activeId === item.id
          return (
            <button
              key={item.id}
              type="button"
              onClick={() => onSelect(item.id)}
              className={`relative w-full rounded-md border px-3 py-2 text-left transition ${
                active
                  ? 'border-amber-300 bg-amber-400/15 text-slate-50 shadow-[0_0_0_1px_rgba(251,191,36,0.38),0_12px_28px_rgba(251,191,36,0.12)] ring-1 ring-amber-300/40'
                  : 'border-slate-800 bg-slate-900/70 text-slate-300 hover:border-slate-600 hover:text-white'
              }`}
            >
              {active ? <span className="absolute left-0 top-2 bottom-2 w-1 rounded-r-full bg-amber-300" /> : null}
              <div className="flex items-start justify-between gap-2">
                <span className="line-clamp-2 text-sm font-medium">{item.title}</span>
                {!item.isRead ? <span className="mt-1 h-2 w-2 shrink-0 rounded-full bg-rose-400" /> : null}
              </div>
              <div className="mt-1 text-xs text-slate-500">{formatDateOnly(item.updatedAt)}</div>
            </button>
          )
        })}
      </div>
    </div>
  )
}

export function Field({
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
  const labelClass = ''
  return (
    <aside
      className={`app-shell__sidebar flex h-screen min-w-0 flex-col overflow-hidden border-r border-slate-800 bg-slate-950 ${
        open ? '' : 'pointer-events-none'
      }`}
    >
      <div className="flex h-20 items-center gap-2 border-b border-slate-800 px-3">
        <button
          type="button"
          onClick={onCloseMobile}
          className="inline-flex h-12 w-12 shrink-0 items-center justify-center rounded-md border border-slate-800 bg-slate-900 text-slate-200 transition hover:border-violet-400 hover:text-white"
          title="关闭侧边栏"
        >
          <XCircle className="h-5 w-5" />
        </button>
        <button
          type="button"
          onClick={onToggleCollapsed}
          className="hidden h-12 w-12 shrink-0 items-center justify-center rounded-md border border-slate-800 bg-slate-900 text-slate-200 transition hover:border-violet-400 hover:text-white"
          title={collapsed ? '展开侧边栏' : '收起侧边栏'}
        >
          {collapsed ? <PanelLeftOpen className="h-5 w-5" /> : <PanelLeftClose className="h-5 w-5" />}
        </button>
        <div className={`min-w-0 ${labelClass || 'block'}`}>
          <div className="flex items-center gap-2 text-sm text-slate-400">
            <Image className="h-4 w-4 text-violet-300" />
            <span>AI 生图</span>
          </div>
          <div className="truncate text-sm font-medium text-slate-100">
            用户工作台
          </div>
        </div>
      </div>

      <nav className="flex-1 space-y-2 overflow-y-auto px-3 py-5">
        {appNavItems.map((item) => {
          const active = activePage === item.page
          return (
            <button
              key={item.page}
              type="button"
              onClick={() => onNavigate(item.page)}
              className={`flex h-12 w-full min-w-0 items-center gap-3 rounded-md px-3 text-left text-sm transition ${
                active
                  ? 'bg-violet-500 text-slate-950 shadow-lg shadow-violet-950/25'
                  : 'text-slate-300 hover:bg-slate-900 hover:text-white'
              }`}
              title={item.label}
            >
              <span className="flex h-5 w-5 shrink-0 items-center justify-center">{item.icon}</span>
              <span className={`min-w-0 truncate ${labelClass}`}>{item.label}</span>
            </button>
          )
        })}
      </nav>
    </aside>
  )
}

function AppTopBar({
  sidebarOpen,
  activePage,
  userTheme,
  currentUser,
  onOpenSidebar,
  onToggleUserTheme,
  onNavigate,
  onToggleAdmin,
  adminOpen,
  onLogout,
  unreadAnnouncementCount,
  onOpenAnnouncements,
}: {
  sidebarOpen: boolean
  activePage: AppPage
  userTheme: UserTheme
  currentUser: UserInfo | null
  onOpenSidebar: () => void
  onToggleUserTheme: () => void
  onNavigate: (page: AppPage) => void
  onToggleAdmin: () => void
  adminOpen: boolean
  onLogout: () => void
  unreadAnnouncementCount: number
  onOpenAnnouncements: () => void
}) {
  const [userMenuOpen, setUserMenuOpen] = useState(false)
  const desktopUserMenuRef = useRef<HTMLDivElement | null>(null)
  const mobileUserMenuRef = useRef<HTMLDivElement | null>(null)

  useEffect(() => {
    function handlePointerDown(event: MouseEvent) {
      const target = event.target as Node
      if (!desktopUserMenuRef.current?.contains(target) && !mobileUserMenuRef.current?.contains(target)) {
        setUserMenuOpen(false)
      }
    }

    window.addEventListener('mousedown', handlePointerDown)
    return () => window.removeEventListener('mousedown', handlePointerDown)
  }, [])

  function renderUserMenu(ref: React.RefObject<HTMLDivElement>) {
    return (
          <div
            ref={ref}
            className="relative"
          >
            <button
              type="button"
              onClick={() => setUserMenuOpen((current: any) => !current)}
              className="inline-flex h-9 max-w-[42vw] items-center gap-1.5 rounded-md border border-slate-800 bg-slate-900 px-2 text-slate-100 transition hover:border-slate-600 hover:bg-slate-800 min-[380px]:max-w-[48vw] sm:max-w-none"
              title={currentUser ? currentUser.username : '已登录'}
              aria-expanded={userMenuOpen}
            >
              <span className="inline-flex h-6 w-6 shrink-0 items-center justify-center rounded-full bg-amber-500 text-xs font-semibold text-white">
                {currentUser?.username?.slice(0, 1).toUpperCase() || 'U'}
              </span>
              <span className="app-user-menu-name truncate text-sm font-medium text-slate-200">
                {currentUser ? currentUser.username : '已登录'}
              </span>
              <ChevronDown
                className={`h-3.5 w-3.5 shrink-0 text-slate-400 transition ${userMenuOpen ? 'rotate-180' : ''}`}
              />
            </button>

            {userMenuOpen ? (
              <div className="absolute right-0 top-[calc(100%+4px)] z-50 min-w-[152px] rounded-md border border-slate-800 bg-slate-900 p-1 shadow-2xl shadow-black/40">
                <button
                  type="button"
                  onClick={onToggleUserTheme}
                  className="flex w-full items-center gap-2 rounded-lg px-3 py-2 text-left text-sm text-slate-200 transition hover:bg-slate-800 hover:text-white"
                  title={userTheme === 'light' ? '浅色模式' : '深色模式'}
                >
                  {userTheme === 'light' ? <Sun className="h-4 w-4" /> : <Moon className="h-4 w-4" />}
                  {userTheme === 'light' ? '浅色模式' : '深色模式'}
                </button>
                <div className="user-theme-menu-divider my-1 h-px bg-slate-800" />
                <button
                  type="button"
                  onClick={() => {
                    setUserMenuOpen(false)
                    onNavigate('profile')
                  }}
                  className="flex w-full items-center gap-2 rounded-lg px-3 py-2 text-left text-sm text-slate-200 transition hover:bg-slate-800 hover:text-white"
                >
                  <UserRound className="h-4 w-4" />
                  个人
                </button>
                <button
                  type="button"
                  onClick={() => {
                    setUserMenuOpen(false)
                    onNavigate('recharge')
                  }}
                  className="flex w-full items-center gap-2 rounded-lg px-3 py-2 text-left text-sm text-slate-200 transition hover:bg-slate-800 hover:text-white"
                >
                  <KeyRound className="h-4 w-4" />
                  兑换码
                </button>
                <button
                  type="button"
                  onClick={() => {
                    setUserMenuOpen(false)
                    onLogout()
                  }}
                  className="flex w-full items-center gap-2 rounded-lg px-3 py-2 text-left text-sm text-slate-200 transition hover:bg-slate-800 hover:text-white"
                >
                  <LogOut className="h-4 w-4" />
                  退出登录
                </button>
                {currentUser?.role === 'admin' && !adminOpen ? (
                  <button
                    type="button"
                    onClick={() => {
                      setUserMenuOpen(false)
                      onToggleAdmin()
                    }}
                    className="flex w-full items-center gap-2 rounded-lg px-3 py-2 text-left text-sm text-emerald-200 transition hover:bg-slate-800 hover:text-emerald-100"
                  >
                    <ShieldCheck className="h-4 w-4" />
                    后台
                  </button>
                ) : null}
              </div>
            ) : null}
          </div>
    )
  }

  return (
    <header className="app-shell__topbar sticky top-0 z-30 min-w-0 overflow-visible border-b border-slate-800 bg-slate-950/95 px-2 py-2 backdrop-blur sm:px-4 lg:flex lg:h-16 lg:items-center lg:px-6 lg:py-0">
      <div className="app-desktop-topbar hidden w-full min-w-0 grid-cols-[minmax(0,1fr)_auto] items-center gap-2 lg:grid">
        <div className="flex min-w-0 items-center gap-2 pr-1">
          <button
            type="button"
            onClick={onOpenSidebar}
            className={`app-desktop-nav-btn inline-flex shrink-0 items-center justify-center ${
              sidebarOpen ? 'hidden' : ''
            }`}
            title="打开侧边栏"
          >
            <Menu className="h-4 w-4" />
            <span>导航</span>
          </button>
          <nav className="flex flex-nowrap items-center gap-2 whitespace-nowrap text-sm font-medium">
            <button
              type="button"
              onClick={() => onNavigate('home')}
              className={`app-desktop-nav-btn ${activePage === 'home' ? 'is-active' : ''}`}
            >
              <Home className="h-4 w-4" />
              <span>首页</span>
            </button>
            <button
              type="button"
              onClick={() => onNavigate('prompts')}
              className={`app-desktop-nav-btn ${activePage === 'prompts' ? 'is-active' : ''}`}
            >
              <Sparkles className="h-4 w-4" />
              <span>广场</span>
            </button>
            <button
              type="button"
              onClick={() => onNavigate('products')}
              className={`app-desktop-nav-btn ${activePage === 'products' ? 'is-active' : ''}`}
            >
              <ShoppingBag className="h-4 w-4" />
              <span>商品</span>
            </button>
            <button
              type="button"
              onClick={onOpenAnnouncements}
              className="app-desktop-nav-btn relative"
              aria-label="查看公告"
              title="查看公告"
            >
              <Megaphone className="h-4 w-4" />
              <span>公告</span>
              {unreadAnnouncementCount > 0 ? (
                <span className="absolute -right-1 -top-1 inline-flex h-4 min-w-4 items-center justify-center rounded-full bg-rose-500 px-1 text-[10px] font-semibold text-white">
                  {unreadAnnouncementCount > 9 ? '9+' : unreadAnnouncementCount}
                </span>
              ) : null}
            </button>
          </nav>
        </div>

        <div className="flex min-w-0 shrink-0 items-center gap-2 text-sm">
          {renderUserMenu(desktopUserMenuRef)}
        </div>
      </div>

      <div className="app-mobile-topbar grid grid-cols-[minmax(0,1fr)_auto] items-center gap-1.5 lg:hidden">
        <nav className="grid min-w-0 grid-cols-5 gap-1">
          <button type="button" onClick={onOpenSidebar} className="app-mobile-nav-btn" aria-label="展开导航">
            <Menu className="h-4 w-4" />
            <span>导航</span>
          </button>
          <button type="button" onClick={() => onNavigate('home')} className={`app-mobile-nav-btn ${activePage === 'home' ? 'is-active' : ''}`} aria-label="首页">
            <Home className="h-4 w-4" />
            <span>首页</span>
          </button>
          <button type="button" onClick={() => onNavigate('prompts')} className={`app-mobile-nav-btn ${activePage === 'prompts' ? 'is-active' : ''}`} aria-label="提示词广场">
            <Sparkles className="h-4 w-4" />
            <span>广场</span>
          </button>
          <button type="button" onClick={() => onNavigate('products')} className={`app-mobile-nav-btn ${activePage === 'products' ? 'is-active' : ''}`} aria-label="商品">
            <ShoppingBag className="h-4 w-4" />
            <span>商品</span>
          </button>
          <button type="button" onClick={onOpenAnnouncements} className="app-mobile-nav-btn relative" aria-label="公告">
            <Megaphone className="h-4 w-4" />
            <span>公告</span>
            {unreadAnnouncementCount > 0 ? (
              <span className="absolute right-2 top-1 inline-flex h-3.5 min-w-3.5 items-center justify-center rounded-full bg-rose-500 px-1 text-[9px] font-semibold text-white">
                {unreadAnnouncementCount > 9 ? '9+' : unreadAnnouncementCount}
              </span>
            ) : null}
          </button>
        </nav>
        <div className="app-mobile-user-slot flex items-center justify-end">
          {renderUserMenu(mobileUserMenuRef)}
        </div>
      </div>
    </header>
  )
}
export function PageHeader({
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

export function FlowLogTabButton({
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
      className={`inline-flex h-9 items-center gap-2 rounded-md px-3 text-sm transition ${
        active ? 'bg-violet-500 text-slate-950' : 'text-slate-400 hover:bg-slate-900 hover:text-slate-100'
      }`}
    >
      {icon}
      {label}
    </button>
  )
}

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

function normalizePromptGalleryItem(item: PromptGalleryItem): PromptGalleryItem {
  return {
    ...item,
    tags: normalizePromptTags((item as PromptGalleryItem & { tags?: unknown }).tags),
  }
}

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

export function PromptCard({
  item,
  priority,
  onFavorite,
  onOpenDetail,
}: {
  item: PromptGalleryItem & { serialNo?: string; favorite?: boolean; displayFavorites?: number }
  priority?: boolean
  onFavorite: () => void
  onOpenDetail: () => void
}) {
  const displayImageUrl = item.previewThumbUrl || item.previewUrl
  return (
    <article
      onClick={onOpenDetail}
      className="prompt-card group cursor-pointer overflow-hidden rounded-lg border shadow-panel transition"
    >
      <div className="prompt-card-media relative aspect-[5/4]">
        {displayImageUrl ? (
          <img
            src={displayImageUrl}
            alt={item.title}
            loading={priority ? 'eager' : 'lazy'}
            decoding="async"
            className="h-full w-full bg-slate-800 object-cover"
          />
        ) : (
          <div className="flex h-full items-center justify-center text-slate-500">
            <Image className="h-8 w-8" />
          </div>
        )}
      </div>
      <div className="prompt-card-body flex min-h-[92px] flex-col p-2">
        <div className="min-w-0 flex-1">
          <h3 className="line-clamp-2 min-h-9 text-[13px] font-semibold leading-[18px]">{item.title}</h3>
          {item.tags?.length ? (
            <div className="mt-1 flex flex-wrap gap-1">
              {item.tags.slice(0, 4).map((tag) => (
                <span key={tag} className="prompt-card-tag rounded-full bg-slate-800/80 px-1.5 py-0.5 text-[10px] leading-4 text-slate-300">
                  #{tag}
                </span>
              ))}
            </div>
          ) : null}
        </div>
        <div className="prompt-card-footer mt-1.5 grid grid-cols-[auto_auto] items-center justify-between gap-2">
          <span className="prompt-card-serial text-xs font-medium">{item.serialNo ? `#${item.serialNo}` : '-'}</span>
          <button
            type="button"
            onClick={(event) => {
              event.stopPropagation()
              onFavorite()
            }}
            className={`prompt-card-like inline-flex items-center gap-1 rounded-md border px-2 py-0.5 text-xs transition ${
              item.favorite ? 'is-active' : ''
            }`}
          >
            <Heart className="h-3.5 w-3.5" />
            {item.displayFavorites ?? item.favorites}
          </button>
        </div>
      </div>
    </article>
  )
}

export function PromptDetailDialog({
  item,
  onClose,
  onFavorite,
  onCopy,
}: {
  item: (PromptGalleryItem & { serialNo?: string; favorite?: boolean; displayFavorites?: number }) | null
  onClose: () => void
  onFavorite: () => void
  onCopy: () => void
}) {
  if (!item) return null
  const displayImageUrl = item.previewThumbUrl || item.previewUrl
  return (
    <div className="fixed inset-0 z-50 flex items-center justify-center bg-slate-950/80 p-4">
      <button type="button" className="absolute inset-0" aria-label="关闭详情" onClick={onClose} />
      <div className="relative w-full max-w-lg overflow-hidden rounded-xl border border-slate-700 bg-slate-900 shadow-2xl">
        <div className="flex items-center justify-between border-b border-slate-800 px-4 py-3">
          <div className="min-w-0">
            <p className="text-xs text-slate-400">上传人：{item.author || '-'}</p>
            <h3 className="truncate text-base font-semibold text-slate-100">{item.title}</h3>
          </div>
          <button
            type="button"
            onClick={onClose}
            className="inline-flex h-8 w-8 items-center justify-center rounded-md border border-slate-700 text-slate-300 transition hover:border-slate-500 hover:text-white"
          >
            <XCircle className="h-4 w-4" />
          </button>
        </div>
        <div className="max-h-[72vh] overflow-y-auto p-4">
          <p className="mb-2 text-xs text-slate-500">为保证性能体验，展示图质量会进行压缩</p>
          <div className="overflow-hidden rounded-lg border border-slate-800 bg-slate-950">
            {displayImageUrl ? (
              <img src={displayImageUrl} alt={item.title} loading="lazy" decoding="async" className="h-auto w-full object-cover" />
            ) : (
              <div className="flex h-40 items-center justify-center text-slate-500">
                <Image className="h-8 w-8" />
              </div>
            )}
          </div>
          <div className="mt-4 whitespace-pre-wrap rounded-lg border border-slate-800 bg-slate-950/70 p-3 text-sm leading-6 text-slate-200">
            {item.prompt}
          </div>
          {item.tags?.length ? (
            <div className="mt-3 flex flex-wrap gap-2">
              {item.tags.slice(0, 4).map((tag) => (
                <span key={tag} className="rounded-full border border-slate-700 px-2 py-1 text-xs text-slate-300">
                  #{tag}
                </span>
              ))}
            </div>
          ) : null}
          <div className="mt-4 flex items-center justify-between gap-2">
            <button
              type="button"
              onClick={onFavorite}
              className={`inline-flex h-10 items-center gap-2 rounded-md border px-3 text-sm transition ${
                item.favorite
                  ? 'border-rose-400/50 bg-rose-500/10 text-rose-100'
                  : 'border-slate-700 text-slate-200 hover:border-slate-500'
              }`}
            >
              <Heart className="h-4 w-4" />
              {item.displayFavorites ?? item.favorites}
            </button>
            <button
              type="button"
              onClick={onCopy}
              className="inline-flex h-10 items-center gap-2 rounded-md bg-violet-500 px-4 text-sm font-medium text-slate-950 transition hover:bg-violet-400"
            >
              <Copy className="h-4 w-4" />
              复制提示词
            </button>
          </div>
        </div>
      </div>
    </div>
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

export function RewardLines({ points, credits }: { points: number; credits: number }) {
  return (
    <div className="space-y-1 leading-tight">
      <div>{Math.max(0, Number(points) || 0)}积分</div>
      <div>{Math.max(0, Number(credits) || 0)}福利次数</div>
    </div>
  )
}

export function PointsLogRow({ item }: { item: PointsLogItem }) {
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

export function CreditLogRow({ item }: { item: CreditLogItem }) {
  const { beforeTotal, afterTotal, deltaTotal } = getCreditLogTotals(item)
  const title = item.remark || getCreditChangeTypeLabel(item.changeType)
  return (
    <div className="rounded-lg border border-slate-800 bg-slate-950 p-3">
      <div className="flex items-center justify-between gap-2">
        <span className="truncate text-sm text-slate-200">{title}</span>
        <span className="text-xs text-slate-500">{getCreditChangeTypeLabel(item.changeType)}</span>
      </div>
      <div className="mt-2 flex flex-wrap items-center justify-between gap-2 text-xs text-slate-500">
        <span>福利次数 {beforeTotal} → {afterTotal}</span>
        <span>变动 {formatSignedNumber(deltaTotal)}</span>
      </div>
      <div className="mt-2 flex items-center justify-between text-xs text-slate-500">
        <span>{formatDateTime(item.createdAt)}</span>
        <span>总福利次数 {afterTotal}</span>
      </div>
    </div>
  )
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

function getCreditSourceLabel(source?: string): string {
  switch (source) {
    case 'free':
    case 'paid':
    case 'mixed':
      return '福利次数'
    case 'points':
      return '积分'
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

function normalizeReferenceImagesFromSnapshot(snapshot: TaskRequestSnapshot | null): ReferenceImageItem[] {
  if (!snapshot) {
    return []
  }
  if (Array.isArray(snapshot.referenceImages) && snapshot.referenceImages.length) {
    return syncPrimaryReferenceImage(
      snapshot.referenceImages.map((image) => ({
        url: image.url,
        name: image.name || getReferenceImageNameFromURL(image.url) || '参考图片',
        source: image.source === 'local' ? 'local' : 'network',
      })),
    )
  }
  const urls = Array.isArray(snapshot.referenceImageUrls) && snapshot.referenceImageUrls.length
    ? snapshot.referenceImageUrls
    : snapshot.referenceImageUrl
      ? [snapshot.referenceImageUrl]
      : []
  return syncPrimaryReferenceImage(
    urls.map((url) => ({
      url,
      name: getReferenceImageNameFromURL(url) || '参考图片',
      source: 'network',
    })),
  )
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

function getCreditChangeTypeLabel(changeType: string): string {
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

function readInviteCodeFromLocation(): string {
  try {
    const params = new URLSearchParams(window.location.search)
    return (params.get('invite') || params.get('inviteCode') || '').toUpperCase().replace(/[^A-Z0-9]/g, '').slice(0, 16)
  } catch {
    return ''
  }
}

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
      return '兑换码商品'
    case 'redeem':
      return '兑换'
    default:
      return packageType || '未知'
  }
}

function clampDimension(value: number): number {
  if (!Number.isFinite(value) || value <= 0) {
    return 1024
  }
  return Math.min(CUSTOM_SIZE_MAX_SIDE, Math.max(1, Math.floor(value)))
}
















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

export function money(cents: number): string {
  const value = Number(cents || 0) / 100
  return `${value.toFixed(2)} 元`
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

function getShanghaiDateKey(value: Date): string {
  return new Intl.DateTimeFormat('en-CA', {
    timeZone: 'Asia/Shanghai',
    year: 'numeric',
    month: '2-digit',
    day: '2-digit',
  }).format(value)
}

function isSameShanghaiDay(value?: string | null): boolean {
  if (!value) {
    return false
  }

  const date = new Date(value)
  if (Number.isNaN(date.getTime())) {
    return false
  }

  return getShanghaiDateKey(date) === getShanghaiDateKey(new Date())
}

type CreditsBalanceLike = {
  creditsBalance?: number
  totalCredits?: number
  freeCredits?: number
  paidCredits?: number
}

type CreditsBalanceResponseLike = {
  creditsBalance?: number
  totalCreditsBalance?: number
  freeCreditsBalance?: number
  paidCreditsBalance?: number
}

function getTotalCredits(value: CreditsBalanceLike | null | undefined, fallback = 0): number {
  if (!value) {
    return fallback
  }

  if (value.creditsBalance != null) {
    return Number(value.creditsBalance)
  }
  if (value.totalCredits != null) {
    return Number(value.totalCredits)
  }
  if (value.freeCredits != null || value.paidCredits != null) {
    return Number((value.freeCredits ?? 0) + (value.paidCredits ?? 0))
  }

  return fallback
}

function getResponseCreditsBalance(value: CreditsBalanceResponseLike | null | undefined, fallback = 0): number {
  if (!value) {
    return fallback
  }

  if (value.creditsBalance != null) {
    return Number(value.creditsBalance)
  }
  if (value.totalCreditsBalance != null) {
    return Number(value.totalCreditsBalance)
  }
  if (value.freeCreditsBalance != null || value.paidCreditsBalance != null) {
    return Number((value.freeCreditsBalance ?? 0) + (value.paidCreditsBalance ?? 0))
  }

  return fallback
}

function getAdminStatsCreditsTotal(stats: AdminStatsResponse | null): number {
  if (!stats) {
    return 0
  }

  const compatibleStats = stats as AdminStatsResponse & {
    creditsBalanceTotal?: number
    creditsTotal?: number
    totalCredits?: number
  }

  return Number(
    compatibleStats.creditsBalanceTotal ??
      compatibleStats.creditsTotal ??
      compatibleStats.totalCredits ??
      (stats.freeCreditsTotal ?? 0) + (stats.paidCreditsTotal ?? 0),
  )
}

function getCreditLogTotals(item: CreditLogItem): { beforeTotal: number; afterTotal: number; deltaTotal: number } {
  const beforeTotal = Number(item.beforeCredits ?? item.beforeFree + item.beforePaid)
  const afterTotal = Number(item.afterCredits ?? item.afterFree + item.afterPaid)
  const deltaTotal = Number(item.deltaCredits ?? item.deltaFree + item.deltaPaid)

  return { beforeTotal, afterTotal, deltaTotal }
}

function getActivityCreditDelta(item: UserActivityItem): number {
  return Number(item.deltaCredits ?? (item.deltaFreeCredits ?? 0) + (item.deltaPaidCredits ?? 0))
}

function formatSignedNumber(value: number): string {
  const safeValue = Number.isFinite(value) ? value : 0
  return `${safeValue >= 0 ? '+' : ''}${safeValue}`
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

function shouldForceLogout(error: unknown): boolean {
  return error instanceof APIError && (error.status === 401 || (error.status === 403 && error.message.includes('禁用')))
}

function isRegistrationDailyLimitError(error: unknown): boolean {
  return error instanceof APIError && error.message.includes(REGISTRATION_DAILY_LIMIT_MESSAGE)
}

export default App
