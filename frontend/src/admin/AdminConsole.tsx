import { useEffect, useMemo, useState, type Dispatch, type ReactNode, type SetStateAction } from 'react'
import {
  ArrowLeft,
  AlertTriangle,
  BadgeCheck,
  CheckCircle2,
  ChevronLeft,
  ChevronRight,
  Edit3,
  Eye,
  Ban,
  Image as ImageIcon,
  FileText,
  Filter,
  LayoutDashboard,
  LoaderCircle,
  LogOut,
  Menu,
  Minus,
  Package,
  PanelLeftClose,
  PanelLeftOpen,
  RefreshCcw,
  ReceiptText,
  Search,
  Save,
  Settings,
  ShieldCheck,
  Plus,
  Trash2,
  Users,
  XCircle,
} from 'lucide-react'
import { api } from '@/api/client'
import { TopNotice } from '@/components/TopNotice'
import type {
  AdminActionLogItem,
  AdminContentAuditItem,
  AdminImageTaskItem,
  AdminListActionLogsResponse,
  AdminListContentAuditsResponse,
  AdminListImageTasksResponse,
  AdminListPackagesResponse,
  AdminListRechargeOrdersResponse,
  AdminListUsersResponse,
  AdminPackageItem,
  AdminReviewContentAuditRequest,
  AdminStatsResponse,
  AdminUserItem,
  BillingSettingsResponse,
  ListUserActivityResponse,
  UpdateBillingSettingsRequest,
  UserActivityItem,
  UserInfo,
} from '@/types/api'

type AdminTab = 'overview' | 'users' | 'packages' | 'billing' | 'tasks' | 'orders' | 'audits' | 'logs'
type BusyState = string | null
type UserPointsAction = 'increase' | 'decrease'

type AdminPackageForm = {
  code: string
  name: string
  packageType: 'recharge'
  creditsAmount: number
  pointsCost: number
  amountCents: number
  bonusPoints: number
  enabled: boolean
  sortOrder: number
  note: string
}

type BillingSettingsForm = UpdateBillingSettingsRequest

interface UserPointsDialogState {
  open: boolean
  user: AdminUserItem | null
  action: UserPointsAction
  amount: string
  remark: string
}

interface UserActivityDialogState {
  open: boolean
  user: AdminUserItem | null
  response: ListUserActivityResponse | null
  page: number
  loading: boolean
  error: string
}

interface TaskDetailDialogState {
  open: boolean
  task: AdminImageTaskItem | null
}

const PAGE_SIZE = 10
const USER_ACTIVITY_PAGE_SIZE = 10

const adminTabs: Array<{ id: AdminTab; label: string; icon: JSX.Element }> = [
  { id: 'overview', label: '概览', icon: <LayoutDashboard className="h-4 w-4" /> },
  { id: 'users', label: '用户', icon: <Users className="h-4 w-4" /> },
  { id: 'packages', label: '套餐', icon: <Package className="h-4 w-4" /> },
  { id: 'billing', label: '账本规则', icon: <Settings className="h-4 w-4" /> },
  { id: 'tasks', label: '任务', icon: <FileText className="h-4 w-4" /> },
  { id: 'orders', label: '订单', icon: <ReceiptText className="h-4 w-4" /> },
  { id: 'audits', label: '审核', icon: <ShieldCheck className="h-4 w-4" /> },
  { id: 'logs', label: '操作日志', icon: <FileText className="h-4 w-4" /> },
]

const emptyPackageForm: AdminPackageForm = {
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
}

export function AdminConsole({
  currentUser,
  onBackToApp,
  onLogout,
}: {
  currentUser: UserInfo | null
  onBackToApp: () => void
  onLogout: () => void
}) {
  const [activeTab, setActiveTab] = useState<AdminTab>('overview')
  const [sidebarCollapsed, setSidebarCollapsed] = useState(false)
  const [sidebarOpen, setSidebarOpen] = useState(false)
  const [busy, setBusy] = useState<BusyState>(null)
  const [message, setMessage] = useState('')

  const [stats, setStats] = useState<AdminStatsResponse | null>(null)

  const [usersResponse, setUsersResponse] = useState<AdminListUsersResponse | null>(null)
  const [userKeywordInput, setUserKeywordInput] = useState('')
  const [userKeyword, setUserKeyword] = useState('')
  const [userPage, setUserPage] = useState(1)

  const [packagesResponse, setPackagesResponse] = useState<AdminListPackagesResponse | null>(null)
  const [packageTypeFilter, setPackageTypeFilter] = useState('')
  const [packagePage, setPackagePage] = useState(1)
  const [editingPackageId, setEditingPackageId] = useState<string | null>(null)
  const [packageForm, setPackageForm] = useState<AdminPackageForm>(emptyPackageForm)
  const [billingSettings, setBillingSettings] = useState<BillingSettingsResponse | null>(null)
  const [billingForm, setBillingForm] = useState<BillingSettingsForm>({
    customRechargeUnitPriceCents: 20,
    imageGeneratePointCost: 20,
    checkinPoints: 3,
    checkinCredits: 1,
  })

  const [tasksResponse, setTasksResponse] = useState<AdminListImageTasksResponse | null>(null)
  const [taskStatusFilter, setTaskStatusFilter] = useState('')
  const [taskPage, setTaskPage] = useState(1)

  const [ordersResponse, setOrdersResponse] = useState<AdminListRechargeOrdersResponse | null>(null)
  const [orderStatusFilter, setOrderStatusFilter] = useState('')
  const [orderPage, setOrderPage] = useState(1)

  const [auditsResponse, setAuditsResponse] = useState<AdminListContentAuditsResponse | null>(null)
  const [auditStatusFilter, setAuditStatusFilter] = useState('')
  const [auditPage, setAuditPage] = useState(1)

  const [logsResponse, setLogsResponse] = useState<AdminListActionLogsResponse | null>(null)
  const [logTargetType, setLogTargetType] = useState('')
  const [logTargetId, setLogTargetId] = useState('')
  const [logPage, setLogPage] = useState(1)
  const [pointDialog, setPointDialog] = useState<UserPointsDialogState>({
    open: false,
    user: null,
    action: 'increase',
    amount: '10',
    remark: '后台人工调整',
  })
  const [userActivityDialog, setUserActivityDialog] = useState<UserActivityDialogState>({
    open: false,
    user: null,
    response: null,
    page: 1,
    loading: false,
    error: '',
  })
  const [taskDetailDialog, setTaskDetailDialog] = useState<TaskDetailDialogState>({
    open: false,
    task: null,
  })

  const isAdmin = currentUser?.role === 'admin'

  useEffect(() => {
    if (currentUser && !isAdmin) {
      onBackToApp()
    }
  }, [currentUser, isAdmin, onBackToApp])

  useEffect(() => {
    void refreshActiveTab()
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [activeTab])

  const currentPageLabel = useMemo(() => {
    switch (activeTab) {
      case 'overview':
        return '概览'
      case 'users':
        return '用户'
      case 'packages':
        return '套餐'
      case 'billing':
        return '账本规则'
      case 'tasks':
        return '任务'
      case 'orders':
        return '订单'
      case 'audits':
        return '审核'
      case 'logs':
        return '操作日志'
      default:
        return ''
    }
  }, [activeTab])

  async function refreshActiveTab() {
    try {
      switch (activeTab) {
        case 'overview':
          await loadOverview()
          break
        case 'users':
          await loadUsers(userPage, userKeyword)
          break
        case 'packages':
          await loadPackages(packagePage, packageTypeFilter)
          break
        case 'billing':
          await loadBillingSettings()
          break
        case 'tasks':
          await loadTasks(taskPage, taskStatusFilter)
          break
        case 'orders':
          await loadOrders(orderPage, orderStatusFilter)
          break
        case 'audits':
          await loadAudits(auditPage, auditStatusFilter)
          break
        case 'logs':
          await loadLogs(logPage, logTargetType, logTargetId)
          break
      }
    } catch (error) {
      setMessage(error instanceof Error ? error.message : '后台数据加载失败')
    }
  }

  async function loadOverview() {
    setBusy('overview')
    try {
      const resp = await api.adminStats()
      setStats(resp)
    } catch (error) {
      setMessage(error instanceof Error ? error.message : '概览数据加载失败')
    } finally {
      setBusy(null)
    }
  }

  async function loadUsers(page = userPage, keyword = userKeyword) {
    setBusy('users')
    try {
      const resp = await api.adminListUsers(page, PAGE_SIZE, keyword)
      setUsersResponse(resp)
      setUserPage(resp.page)
      setUserKeyword(keyword)
    } catch (error) {
      setMessage(error instanceof Error ? error.message : '用户列表加载失败')
    } finally {
      setBusy(null)
    }
  }

  async function loadPackages(page = packagePage, packageType = packageTypeFilter) {
    setBusy('packages')
    try {
      const resp = await api.adminListPackages(page, PAGE_SIZE, packageType)
      setPackagesResponse(resp)
      setPackagePage(resp.page)
      setPackageTypeFilter(packageType)
    } catch (error) {
      setMessage(error instanceof Error ? error.message : '套餐列表加载失败')
    } finally {
      setBusy(null)
    }
  }

  async function loadBillingSettings() {
    setBusy('billing')
    try {
      const resp = await api.adminGetBillingSettings()
      setBillingSettings(resp)
      setBillingForm(resp)
    } catch (error) {
      setMessage(error instanceof Error ? error.message : '账本规则加载失败')
    } finally {
      setBusy(null)
    }
  }

  async function saveBillingSettings() {
    const payload: BillingSettingsForm = {
      customRechargeUnitPriceCents: Math.max(1, Math.floor(billingForm.customRechargeUnitPriceCents || 0)),
      imageGeneratePointCost: Math.max(1, Math.floor(billingForm.imageGeneratePointCost || 0)),
      checkinPoints: Math.max(0, Math.floor(billingForm.checkinPoints || 0)),
      checkinCredits: Math.max(0, Math.floor(billingForm.checkinCredits || 0)),
    }

    setBusy('save-billing')
    try {
      const resp = await api.adminUpdateBillingSettings(payload)
      setBillingSettings(resp)
      setBillingForm(resp)
      setMessage('账本规则已更新。')
    } catch (error) {
      setMessage(error instanceof Error ? error.message : '账本规则保存失败')
    } finally {
      setBusy(null)
    }
  }

  async function loadTasks(page = taskPage, status = taskStatusFilter) {
    setBusy('tasks')
    try {
      const resp = await api.adminListImageTasks(page, PAGE_SIZE, status)
      setTasksResponse(resp)
      setTaskPage(resp.page)
      setTaskStatusFilter(status)
    } catch (error) {
      setMessage(error instanceof Error ? error.message : '任务列表加载失败')
    } finally {
      setBusy(null)
    }
  }

  async function loadOrders(page = orderPage, status = orderStatusFilter) {
    setBusy('orders')
    try {
      const resp = await api.adminListRechargeOrders(page, PAGE_SIZE, status)
      setOrdersResponse(resp)
      setOrderPage(resp.page)
      setOrderStatusFilter(status)
    } catch (error) {
      setMessage(error instanceof Error ? error.message : '订单列表加载失败')
    } finally {
      setBusy(null)
    }
  }

  async function loadAudits(page = auditPage, status = auditStatusFilter) {
    setBusy('audits')
    try {
      const resp = await api.adminListContentAudits(page, PAGE_SIZE, status)
      setAuditsResponse(resp)
      setAuditPage(resp.page)
      setAuditStatusFilter(status)
    } catch (error) {
      setMessage(error instanceof Error ? error.message : '审核列表加载失败')
    } finally {
      setBusy(null)
    }
  }

  async function loadLogs(page = logPage, targetType = logTargetType, targetId = logTargetId) {
    setBusy('logs')
    try {
      const resp = await api.adminListActionLogs(page, PAGE_SIZE, targetType, targetId)
      setLogsResponse(resp)
      setLogPage(resp.page)
      setLogTargetType(targetType)
      setLogTargetId(targetId)
    } catch (error) {
      setMessage(error instanceof Error ? error.message : '操作日志加载失败')
    } finally {
      setBusy(null)
    }
  }

  function startCreatePackage() {
    setEditingPackageId(null)
    setPackageForm(emptyPackageForm)
    setActiveTab('packages')
  }

  function startEditPackage(item: AdminPackageItem) {
    setEditingPackageId(item.id)
    setPackageForm({
      code: item.code,
      name: item.name,
      packageType: 'recharge',
      creditsAmount: item.creditsAmount,
      pointsCost: item.pointsCost,
      amountCents: item.amountCents,
      bonusPoints: item.bonusPoints,
      enabled: item.enabled,
      sortOrder: item.sortOrder,
      note: item.note ?? '',
    })
    setActiveTab('packages')
  }

  function resetPackageForm() {
    setEditingPackageId(null)
    setPackageForm(emptyPackageForm)
  }

  async function savePackage() {
    const packageCode = packageForm.code.trim()
    const packageName = packageForm.name.trim()

    try {
      if (!editingPackageId && !packageCode) {
        setMessage('请填写套餐编码。')
        return
      }
      if (!packageName) {
        setMessage('请填写套餐名称。')
        return
      }
      if (packageForm.amountCents <= 0) {
        setMessage('充值套餐需要填写价格。')
        return
      }

      setBusy('save-package')
      if (editingPackageId) {
        await api.adminUpdatePackage(editingPackageId, {
          name: packageName,
          creditsAmount: packageForm.creditsAmount,
          pointsCost: 0,
          amountCents: packageForm.amountCents,
          bonusPoints: packageForm.bonusPoints,
          enabled: packageForm.enabled,
          sortOrder: packageForm.sortOrder,
          note: packageForm.note,
        })
        setMessage('套餐已更新。')
      } else {
        await api.adminCreatePackage({
          code: packageCode,
          name: packageName,
          packageType: 'recharge',
          creditsAmount: packageForm.creditsAmount,
          pointsCost: 0,
          amountCents: packageForm.amountCents,
          bonusPoints: packageForm.bonusPoints,
          enabled: packageForm.enabled,
          sortOrder: packageForm.sortOrder,
          note: packageForm.note,
        })
        setMessage('套餐已创建。')
      }
      await loadPackages(1, packageTypeFilter)
      resetPackageForm()
    } catch (error) {
      setMessage(error instanceof Error ? error.message : '保存套餐失败')
    } finally {
      setBusy(null)
    }
  }

  function openAdjustPointsDialog(user: AdminUserItem, action: UserPointsAction) {
    setPointDialog({
      open: true,
      user,
      action,
      amount: '10',
      remark: '后台人工调整',
    })
  }

  function closeAdjustPointsDialog() {
    setPointDialog((current) => ({
      ...current,
      open: false,
      user: null,
    }))
  }

  async function submitPointAdjust() {
    const user = pointDialog.user
    if (!user) {
      return
    }

    const amount = Number(pointDialog.amount)
    if (!Number.isFinite(amount) || amount <= 0) {
      setMessage('请输入有效的积分数量。')
      return
    }

    const delta = pointDialog.action === 'increase' ? Math.floor(amount) : -Math.floor(amount)
    const remark = pointDialog.remark.trim() || '后台人工调整'

    try {
      setBusy(`user-points:${user.id}`)
      await api.adminAdjustPoints(user.id, delta, remark)
      setMessage(`已${delta > 0 ? '增加' : '减少'}用户「${user.username}」积分 ${Math.abs(delta)}。`)
      closeAdjustPointsDialog()
      await loadUsers(userPage, userKeyword)
      if (userActivityDialog.open && userActivityDialog.user?.id === user.id) {
        await loadUserActivityPage(user, userActivityDialog.page)
      }
    } catch (error) {
      setMessage(error instanceof Error ? error.message : '调整积分失败')
    } finally {
      setBusy(null)
    }
  }

  async function loadUserActivityPage(user: AdminUserItem, page = 1) {
    try {
      setUserActivityDialog((current) => ({
        ...current,
        open: true,
        user,
        page,
        loading: true,
        error: '',
      }))
      const resp = await api.adminListUserActivity(user.id, page, USER_ACTIVITY_PAGE_SIZE)
      setUserActivityDialog({
        open: true,
        user,
        response: resp,
        page: resp.page,
        loading: false,
        error: '',
      })
    } catch (error) {
      setUserActivityDialog((current) => ({
        ...current,
        loading: false,
        error: error instanceof Error ? error.message : '用户日志加载失败',
      }))
    }
  }

  function openUserActivityDialog(user: AdminUserItem) {
    void loadUserActivityPage(user, 1)
  }

  function closeUserActivityDialog() {
    setUserActivityDialog({
      open: false,
      user: null,
      response: null,
      page: 1,
      loading: false,
      error: '',
    })
  }

  function openTaskDetailDialog(task: AdminImageTaskItem) {
    setTaskDetailDialog({
      open: true,
      task,
    })
  }

  function closeTaskDetailDialog() {
    setTaskDetailDialog({
      open: false,
      task: null,
    })
  }

  async function disableUser(user: AdminUserItem) {
    if (user.status === 2) {
      setMessage(`用户「${user.username}」已经处于禁用状态。`)
      return
    }

    const confirmed = window.confirm(`确定要禁用用户「${user.username}」吗？`)
    if (!confirmed) {
      return
    }

    try {
      setBusy(`user-status:${user.id}`)
      await api.adminUpdateUserStatus(user.id, 2)
      setMessage(`已禁用用户「${user.username}」。`)
      await loadUsers(userPage, userKeyword)
      if (userActivityDialog.open && userActivityDialog.user?.id === user.id) {
        await loadUserActivityPage(user, userActivityDialog.page)
      }
    } catch (error) {
      setMessage(error instanceof Error ? error.message : '禁用用户失败')
    } finally {
      setBusy(null)
    }
  }

  async function deletePackage(item: AdminPackageItem) {
    const confirmed = window.confirm(`确定要删除套餐「${item.name}」吗？删除后将从后台列表中移除。`)
    if (!confirmed) {
      return
    }

    try {
      setBusy(`delete-package:${item.id}`)
      await api.adminDeletePackage(item.id)
      if (editingPackageId === item.id) {
        resetPackageForm()
      }
      await loadPackages(packagePage, packageTypeFilter)
      setMessage(`套餐「${item.name}」已删除。`)
    } catch (error) {
      setMessage(error instanceof Error ? error.message : '删除套餐失败')
    } finally {
      setBusy(null)
    }
  }

  async function reviewAudit(item: AdminContentAuditItem, status: 'pass' | 'rejected') {
    const defaultReason = status === 'pass' ? '管理员人工通过' : '管理员人工驳回'
    const reason = window.prompt('请输入审核备注', item.reason || defaultReason)
    if (reason === null) {
      return
    }

    const payload: AdminReviewContentAuditRequest = {
      status,
      action: status === 'pass' ? 'allow' : 'reject',
      reason: reason.trim() || defaultReason,
    }

    try {
      setBusy(`audit-review:${item.id}`)
      await api.adminReviewContentAudit(item.id, payload)
      setMessage(`审核记录 #${item.id} 已更新。`)
      await loadAudits(auditPage, auditStatusFilter)
      await loadOverview()
    } catch (error) {
      setMessage(error instanceof Error ? error.message : '审核更新失败')
    } finally {
      setBusy(null)
    }
  }

  async function handleFilterSearch(kind: AdminTab) {
    switch (kind) {
      case 'users':
        setUserPage(1)
        await loadUsers(1, userKeywordInput)
        break
      case 'packages':
        setPackagePage(1)
        await loadPackages(1, packageTypeFilter)
        break
      case 'tasks':
        setTaskPage(1)
        await loadTasks(1, taskStatusFilter)
        break
      case 'orders':
        setOrderPage(1)
        await loadOrders(1, orderStatusFilter)
        break
      case 'audits':
        setAuditPage(1)
        await loadAudits(1, auditStatusFilter)
        break
      case 'logs':
        setLogPage(1)
        await loadLogs(1, logTargetType, logTargetId)
        break
      default:
        break
    }
  }

  function openUserSidePanel() {
    setActiveTab('users')
  }

  if (!currentUser) {
    return (
      <div className="flex min-h-screen items-center justify-center bg-slate-950 text-slate-100">
        <TopNotice message={message} onDismiss={() => setMessage('')} />
        <div className="flex items-center gap-3 rounded-lg border border-slate-800 bg-slate-900 px-4 py-3 text-sm text-slate-300">
          <LoaderCircle className="h-4 w-4 animate-spin" />
          正在进入管理员后台...
        </div>
      </div>
    )
  }

  if (!isAdmin) {
    return (
      <div className="flex min-h-screen items-center justify-center bg-slate-950 text-slate-100">
        <TopNotice message={message} onDismiss={() => setMessage('')} />
        <div className="max-w-md rounded-lg border border-slate-800 bg-slate-900 p-6 text-center">
          <ShieldCheck className="mx-auto h-10 w-10 text-emerald-300" />
          <h1 className="mt-4 text-lg font-semibold text-slate-50">没有管理员权限</h1>
          <p className="mt-2 text-sm leading-6 text-slate-400">当前账号不是管理员，无法访问后台。</p>
          <button
            type="button"
            onClick={onBackToApp}
            className="mt-4 inline-flex items-center gap-2 rounded-md bg-violet-500 px-4 py-2 text-sm font-medium text-slate-950 transition hover:bg-violet-400"
          >
            <ArrowLeft className="h-4 w-4" />
            返回用户端
          </button>
        </div>
      </div>
    )
  }

  return (
    <div className="min-h-screen bg-slate-950 text-slate-100">
      <TopNotice message={message} onDismiss={() => setMessage('')} />
      <div className="flex min-h-screen">
        {sidebarOpen ? (
          <button
            type="button"
            aria-label="关闭后台侧边栏"
            onClick={() => setSidebarOpen(false)}
            className="fixed inset-0 z-30 bg-slate-950/70 backdrop-blur-sm lg:hidden"
          />
        ) : null}

        <AdminSidebar
          collapsed={sidebarCollapsed}
          open={sidebarOpen}
          activeTab={activeTab}
          onNavigate={(tab) => {
            setActiveTab(tab)
            setSidebarOpen(false)
          }}
          onToggleCollapsed={() => setSidebarCollapsed((current) => !current)}
          onCloseMobile={() => setSidebarOpen(false)}
          onBackToApp={onBackToApp}
        />

        <div className="flex min-w-0 flex-1 flex-col">
          <header className="sticky top-0 z-20 border-b border-slate-800 bg-slate-950/95 px-4 py-3 backdrop-blur">
            <div className="grid grid-cols-[minmax(0,1fr)_auto] items-center gap-3">
              <div className="flex min-w-0 items-center gap-2 overflow-x-auto [scrollbar-width:none] [&::-webkit-scrollbar]:hidden">
                <button
                  type="button"
                  onClick={() => setSidebarOpen(true)}
                  className={`inline-flex h-10 w-10 items-center justify-center rounded-md border border-slate-800 bg-slate-900 text-slate-200 transition hover:border-violet-400 hover:text-white lg:hidden ${
                    sidebarOpen ? 'hidden' : ''
                  }`}
                  title="打开后台侧边栏"
                >
                  <Menu className="h-4 w-4" />
                </button>

                <div className="flex min-w-0 items-center gap-2">
                  <ShieldCheck className="h-5 w-5 text-emerald-300" />
                  <div className="min-w-0">
                    <div className="truncate text-sm font-medium text-slate-50">管理员后台</div>
                    <div className="truncate text-xs text-slate-500">{currentPageLabel}</div>
                  </div>
                </div>

                <button
                  type="button"
                  onClick={refreshActiveTab}
                  disabled={busy !== null}
                  className="inline-flex items-center gap-2 rounded-md border border-slate-700 bg-slate-900 px-3 py-2 text-sm text-slate-200 transition hover:border-emerald-400 hover:text-white disabled:cursor-not-allowed disabled:opacity-60"
                >
                  {busy ? <LoaderCircle className="h-4 w-4 animate-spin" /> : <RefreshCcw className="h-4 w-4" />}
                  刷新
                </button>
              </div>

              <div className="flex items-center gap-2 text-sm">
                <span className="max-w-[34vw] truncate rounded-md border border-slate-800 bg-slate-900 px-2.5 py-2 text-slate-300 sm:max-w-none sm:px-3">
                  {currentUser.username}
                </span>
                <button
                  type="button"
                  onClick={onBackToApp}
                  className="inline-flex shrink-0 items-center gap-2 rounded-md border border-slate-700 bg-slate-900 px-2.5 py-2 text-slate-200 transition hover:border-slate-500 hover:text-white sm:px-3"
                >
                  <ArrowLeft className="h-4 w-4" />
                  返回
                </button>
                <button
                  type="button"
                  onClick={onLogout}
                  className="inline-flex shrink-0 items-center gap-2 rounded-md border border-slate-700 bg-slate-900 px-2.5 py-2 text-slate-200 transition hover:border-slate-500 hover:text-white sm:px-3"
                >
                  <LogOut className="h-4 w-4" />
                  退出
                </button>
              </div>
            </div>
          </header>

          <main className="flex-1 px-4 py-4 lg:px-6">

            {activeTab === 'overview' ? (
              <OverviewTab
                stats={stats}
                onOpenUsers={openUserSidePanel}
                onCreatePackage={startCreatePackage}
                busy={busy}
              />
            ) : null}

            {activeTab === 'users' ? (
              <UsersTab
                response={usersResponse}
                keywordInput={userKeywordInput}
                busy={busy}
                onKeywordInputChange={setUserKeywordInput}
                onSearch={() => void handleFilterSearch('users')}
                onPageChange={(page) => void loadUsers(page, userKeyword)}
                onIncreasePoints={(user) => openAdjustPointsDialog(user, 'increase')}
                onDecreasePoints={(user) => openAdjustPointsDialog(user, 'decrease')}
                onDisableUser={(user) => void disableUser(user)}
                onOpenActivity={(user) => openUserActivityDialog(user)}
              />
            ) : null}

            {activeTab === 'packages' ? (
              <PackagesTab
                response={packagesResponse}
                busy={busy}
                filterValue={packageTypeFilter}
                onFilterChange={(value) => setPackageTypeFilter(value)}
                onSearch={() => void handleFilterSearch('packages')}
                onPageChange={(page) => void loadPackages(page, packageTypeFilter)}
                editingPackageId={editingPackageId}
                form={packageForm}
                onFormChange={setPackageForm}
                onStartCreate={startCreatePackage}
                onStartEdit={startEditPackage}
                onReset={resetPackageForm}
                onSave={() => void savePackage()}
                onDelete={(item) => void deletePackage(item)}
              />
            ) : null}

            {activeTab === 'billing' ? (
              <BillingSettingsTab
                settings={billingSettings}
                form={billingForm}
                busy={busy}
                onFormChange={setBillingForm}
                onSave={() => void saveBillingSettings()}
                onReload={() => void loadBillingSettings()}
              />
            ) : null}

            {activeTab === 'tasks' ? (
              <TasksTab
                response={tasksResponse}
                busy={busy}
                statusFilter={taskStatusFilter}
                onStatusChange={setTaskStatusFilter}
                onSearch={() => void handleFilterSearch('tasks')}
                onPageChange={(page) => void loadTasks(page, taskStatusFilter)}
                onOpenDetail={openTaskDetailDialog}
              />
            ) : null}

            {activeTab === 'orders' ? (
              <OrdersTab
                response={ordersResponse}
                busy={busy}
                statusFilter={orderStatusFilter}
                onStatusChange={setOrderStatusFilter}
                onSearch={() => void handleFilterSearch('orders')}
                onPageChange={(page) => void loadOrders(page, orderStatusFilter)}
              />
            ) : null}

            {activeTab === 'audits' ? (
              <AuditsTab
                response={auditsResponse}
                busy={busy}
                statusFilter={auditStatusFilter}
                onStatusChange={setAuditStatusFilter}
                onSearch={() => void handleFilterSearch('audits')}
                onPageChange={(page) => void loadAudits(page, auditStatusFilter)}
                onReview={(item, status) => void reviewAudit(item, status)}
              />
            ) : null}

            {activeTab === 'logs' ? (
              <LogsTab
                response={logsResponse}
                busy={busy}
                targetType={logTargetType}
                targetId={logTargetId}
                onTargetTypeChange={setLogTargetType}
                onTargetIdChange={setLogTargetId}
                onSearch={() => void handleFilterSearch('logs')}
                onPageChange={(page) => void loadLogs(page, logTargetType, logTargetId)}
              />
            ) : null}
          </main>
        </div>
      </div>

      {pointDialog.open ? (
        <PointAdjustDialog
          dialog={pointDialog}
          busy={busy}
          onChange={setPointDialog}
          onClose={closeAdjustPointsDialog}
          onSubmit={() => void submitPointAdjust()}
        />
      ) : null}

      {userActivityDialog.open ? (
        <UserActivityDialog
          dialog={userActivityDialog}
          onClose={closeUserActivityDialog}
          onPageChange={(page) => {
            if (userActivityDialog.user) {
              void loadUserActivityPage(userActivityDialog.user, page)
            }
          }}
        />
      ) : null}

      {taskDetailDialog.open ? (
        <TaskDetailDialog dialog={taskDetailDialog} onClose={closeTaskDetailDialog} />
      ) : null}
    </div>
  )
}

function AdminSidebar({
  collapsed,
  open,
  activeTab,
  onNavigate,
  onToggleCollapsed,
  onCloseMobile,
  onBackToApp,
}: {
  collapsed: boolean
  open: boolean
  activeTab: AdminTab
  onNavigate: (tab: AdminTab) => void
  onToggleCollapsed: () => void
  onCloseMobile: () => void
  onBackToApp: () => void
}) {
  const railWidthClass = collapsed ? 'lg:w-16' : 'lg:w-68'
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
          title="关闭后台侧边栏"
        >
          <PanelLeftClose className="h-5 w-5" />
        </button>

        <button
          type="button"
          onClick={onToggleCollapsed}
          className="inline-flex h-12 w-12 shrink-0 items-center justify-center rounded-md border border-slate-800 bg-slate-900 text-slate-200 transition hover:border-violet-400 hover:text-white"
          title={collapsed ? '展开侧边栏' : '收起侧边栏'}
        >
          {collapsed ? <PanelLeftOpen className="h-5 w-5" /> : <PanelLeftClose className="h-5 w-5" />}
        </button>

        <div className={`${labelClass} min-w-0`}>
          <div className="truncate text-sm font-semibold text-slate-50">管理控制台</div>
          <div className="truncate text-xs text-slate-500">AI 生图平台</div>
        </div>
      </div>

      <nav className={`flex-1 space-y-3 ${collapsed ? 'px-2 py-4' : 'px-3 py-5'}`}>
        {adminTabs.map((item) => {
          const active = activeTab === item.id
          return (
            <button
              key={item.id}
              type="button"
              onClick={() => onNavigate(item.id)}
              className={`flex h-12 w-full items-center gap-3 rounded-md px-3 text-left text-sm transition ${
                active
                  ? 'bg-emerald-400 text-slate-950'
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

      <div className="border-t border-slate-800 p-3">
        <button
          type="button"
          onClick={onBackToApp}
          className={`flex h-12 w-full items-center gap-3 rounded-md border border-slate-700 bg-slate-900 px-3 text-left text-sm text-slate-200 transition hover:border-slate-500 hover:text-white ${
            collapsed ? 'lg:justify-center lg:px-0' : ''
          }`}
        >
          <ArrowLeft className="h-4 w-4 shrink-0" />
          <span className={labelClass}>返回用户端</span>
        </button>
      </div>
    </aside>
  )
}

function OverviewTab({
  stats,
  busy,
  onOpenUsers,
  onCreatePackage,
}: {
  stats: AdminStatsResponse | null
  busy: BusyState
  onOpenUsers: () => void
  onCreatePackage: () => void
}) {
  const items = [
    { label: '用户总数', value: stats?.usersTotal ?? 0 },
    { label: '禁用用户', value: stats?.usersDisabled ?? 0 },
    { label: '任务总数', value: stats?.tasksTotal ?? 0 },
    { label: '生成中', value: stats?.tasksGenerating ?? 0 },
    { label: '成功任务', value: stats?.tasksSuccess ?? 0 },
    { label: '失败任务', value: stats?.tasksFailed ?? 0 },
    { label: '已支付订单', value: stats?.ordersPaid ?? 0 },
    { label: '拦截审核', value: stats?.auditRejected ?? 0 },
    { label: '积分总量', value: stats?.pointsBalanceTotal ?? 0 },
    { label: '免费次数', value: stats?.freeCreditsTotal ?? 0 },
    { label: '充值次数', value: stats?.paidCreditsTotal ?? 0 },
  ]

  return (
    <section className="space-y-4">
      <div className="flex flex-col gap-3 border-b border-slate-800 pb-4 lg:flex-row lg:items-end lg:justify-between">
        <div>
          <h1 className="text-xl font-semibold text-slate-50">后台概览</h1>
          <p className="mt-1 text-sm text-slate-400">用户、套餐、任务、订单和审核的统一入口。</p>
        </div>
        <div className="flex flex-wrap gap-2">
          <button
            type="button"
            onClick={onOpenUsers}
            className="inline-flex items-center gap-2 rounded-md border border-slate-700 bg-slate-900 px-3 py-2 text-sm text-slate-200 transition hover:border-emerald-400 hover:text-white"
          >
            <Users className="h-4 w-4" />
            用户管理
          </button>
          <button
            type="button"
            onClick={onCreatePackage}
            className="inline-flex items-center gap-2 rounded-md bg-emerald-400 px-3 py-2 text-sm font-medium text-slate-950 transition hover:bg-emerald-300"
          >
            <Package className="h-4 w-4" />
            新建套餐
          </button>
        </div>
      </div>

      <div className="grid gap-3 sm:grid-cols-2 xl:grid-cols-4">
        {items.map((item) => (
          <StatTile key={item.label} label={item.label} value={String(item.value)} />
        ))}
      </div>

      <div className="rounded-lg border border-slate-800 bg-slate-900/70 p-4 text-sm leading-6 text-slate-300">
        <div className="flex items-center gap-2 text-slate-100">
          <BadgeCheck className="h-4 w-4 text-emerald-300" />
          管理说明
        </div>
        <p className="mt-2 text-slate-400">
          后台支持用户状态控制、积分调整、套餐配置、任务查看、订单查看、内容审核复审和操作留痕。
          页面中的所有列表都可以按页切换，筛选条件会直接作用在后端查询上。
        </p>
        {busy ? <p className="mt-2 text-xs text-slate-500">当前正在加载 {busy}。</p> : null}
      </div>
    </section>
  )
}

function UsersTab({
  response,
  keywordInput,
  busy,
  onKeywordInputChange,
  onSearch,
  onPageChange,
  onIncreasePoints,
  onDecreasePoints,
  onDisableUser,
  onOpenActivity,
}: {
  response: AdminListUsersResponse | null
  keywordInput: string
  busy: BusyState
  onKeywordInputChange: (value: string) => void
  onSearch: () => void
  onPageChange: (page: number) => void
  onIncreasePoints: (user: AdminUserItem) => void
  onDecreasePoints: (user: AdminUserItem) => void
  onDisableUser: (user: AdminUserItem) => void
  onOpenActivity: (user: AdminUserItem) => void
}) {
  const items = response?.items ?? []

  return (
    <SectionShell
      title="用户管理"
      description="查询用户、调整状态和积分。"
      toolbar={
        <div className="flex flex-wrap items-center gap-2">
          <div className="flex h-10 items-center gap-2 rounded-md border border-slate-700 bg-slate-900 px-3">
            <Search className="h-4 w-4 text-slate-500" />
            <input
              value={keywordInput}
              onChange={(event) => onKeywordInputChange(event.target.value)}
              placeholder="按用户名或邮箱搜索"
              className="w-56 bg-transparent text-sm text-slate-100 outline-none placeholder:text-slate-500"
            />
          </div>
          <button
            type="button"
            onClick={onSearch}
            className="inline-flex items-center gap-2 rounded-md bg-emerald-400 px-3 py-2 text-sm font-medium text-slate-950 transition hover:bg-emerald-300"
          >
            <Filter className="h-4 w-4" />
            搜索
          </button>
        </div>
      }
    >
      <TableShell columns="minmax(90px, 120px) minmax(220px, 1.2fr) minmax(160px, 180px) minmax(150px, 180px) minmax(130px, 160px) minmax(150px, 180px) minmax(200px, 220px)">
        <thead className="bg-slate-950/70 text-xs uppercase tracking-wide text-slate-500">
          <tr className="[&>th]:px-3 [&>th]:py-3 [&>th]:text-left">
            <th>ID</th>
            <th>用户</th>
            <th>角色/状态</th>
            <th>积分/次数</th>
            <th>签到</th>
            <th>注册时间</th>
            <th>操作</th>
          </tr>
        </thead>
        <tbody className="divide-y divide-slate-800 text-sm">
          {items.length ? (
            items.map((user) => (
              <tr
                key={user.id}
                className="cursor-pointer align-top transition hover:bg-slate-900/60 [&>td]:px-3 [&>td]:py-3"
                onClick={() => onOpenActivity(user)}
                title="查看用户日志"
              >
                <td className="whitespace-nowrap text-slate-500">#{user.id}</td>
                <td>
                  <div className="min-w-0">
                    <div className="truncate font-medium text-slate-100">{user.username}</div>
                    <div className="mt-1 truncate text-xs text-slate-500">{user.email}</div>
                  </div>
                </td>
                <td>
                  <div className="flex flex-wrap gap-2">
                    <span className={`rounded-md px-2 py-1 text-xs ${user.status === 1 ? 'bg-emerald-400/15 text-emerald-300' : 'bg-rose-500/15 text-rose-300'}`}>
                      {user.status === 1 ? '正常' : '禁用'}
                    </span>
                    <span className="rounded-md bg-violet-500/15 px-2 py-1 text-xs text-violet-200">
                      {getUserRoleLabel(user.role)}
                    </span>
                  </div>
                </td>
                <td className="text-slate-300">
                  <div>积分 {user.pointsBalance}</div>
                  <div className="mt-1 text-xs text-slate-500">
                    免费 {user.freeCredits} / 充值 {user.paidCredits}
                  </div>
                </td>
                <td className="text-slate-300">
                  <div>连续 {user.checkinStreak} 天</div>
                  <div className="mt-1 text-xs text-slate-500">{formatDateTime(user.lastCheckinAt)}</div>
                </td>
                <td className="whitespace-nowrap text-slate-400">{formatDateTime(user.createdAt)}</td>
                <td>
                  <div className="flex flex-wrap gap-2">
                    <button
                      type="button"
                      onClick={(event) => {
                        event.stopPropagation()
                        onIncreasePoints(user)
                      }}
                      disabled={busy === `user-points:${user.id}`}
                      className="inline-flex items-center gap-1 rounded-md border border-slate-700 px-2 py-1 text-xs text-slate-200 transition hover:border-emerald-400 disabled:cursor-not-allowed disabled:opacity-60"
                    >
                      <Plus className="h-3.5 w-3.5" />
                      增加
                    </button>
                    <button
                      type="button"
                      onClick={(event) => {
                        event.stopPropagation()
                        onDecreasePoints(user)
                      }}
                      disabled={busy === `user-points:${user.id}`}
                      className="inline-flex items-center gap-1 rounded-md border border-slate-700 px-2 py-1 text-xs text-slate-200 transition hover:border-amber-400 disabled:cursor-not-allowed disabled:opacity-60"
                    >
                      <Minus className="h-3.5 w-3.5" />
                      减少
                    </button>
                    <button
                      type="button"
                      onClick={(event) => {
                        event.stopPropagation()
                        onDisableUser(user)
                      }}
                      disabled={busy === `user-status:${user.id}` || user.status === 2}
                      className="inline-flex items-center gap-1 rounded-md border border-slate-700 px-2 py-1 text-xs text-slate-200 transition hover:border-rose-400 disabled:cursor-not-allowed disabled:opacity-60"
                    >
                      <Ban className="h-3.5 w-3.5" />
                      {user.status === 2 ? '已禁用' : '禁用'}
                    </button>
                    <button
                      type="button"
                      onClick={(event) => {
                        event.stopPropagation()
                        onOpenActivity(user)
                      }}
                      className="inline-flex items-center gap-1 rounded-md border border-slate-700 px-2 py-1 text-xs text-slate-200 transition hover:border-violet-400 disabled:cursor-not-allowed disabled:opacity-60"
                    >
                      <Eye className="h-3.5 w-3.5" />
                      查看日志
                    </button>
                  </div>
                </td>
              </tr>
            ))
          ) : (
            <tr>
              <td colSpan={7}>
                <EmptyState text="暂无用户数据" />
              </td>
            </tr>
          )}
        </tbody>
      </TableShell>

      <PaginationBar
        page={response?.page ?? 1}
        pageSize={response?.pageSize ?? PAGE_SIZE}
        total={response?.total ?? 0}
        onPageChange={onPageChange}
      />
    </SectionShell>
  )
}

function BillingSettingsTab({
  settings,
  form,
  busy,
  onFormChange,
  onSave,
  onReload,
}: {
  settings: BillingSettingsResponse | null
  form: BillingSettingsForm
  busy: BusyState
  onFormChange: Dispatch<SetStateAction<BillingSettingsForm>>
  onSave: () => void
  onReload: () => void
}) {
  return (
    <SectionShell
      title="账本规则"
      description="配置自定义充值价格、生图积分兜底扣费和每日签到奖励。"
      toolbar={
        <button
          type="button"
          onClick={onReload}
          disabled={busy === 'billing'}
          className="inline-flex items-center gap-2 rounded-md border border-slate-700 bg-slate-900 px-3 py-2 text-sm text-slate-200 transition hover:border-emerald-400 hover:text-white disabled:cursor-not-allowed disabled:opacity-60"
        >
          {busy === 'billing' ? <LoaderCircle className="h-4 w-4 animate-spin" /> : <RefreshCcw className="h-4 w-4" />}
          重新加载
        </button>
      }
    >
      <div className="grid gap-4 xl:grid-cols-[420px_minmax(0,1fr)]">
        <div className="rounded-lg border border-slate-800 bg-slate-950/60 p-4">
          <div className="grid gap-3">
            <FieldLabel label="自定义充值单次价格（分）">
              <input
                type="number"
                min={1}
                value={form.customRechargeUnitPriceCents}
                onChange={(event) =>
                  onFormChange((current) => ({
                    ...current,
                    customRechargeUnitPriceCents: Number(event.target.value),
                  }))
                }
                className="h-10 w-full rounded-md border border-slate-700 bg-slate-900 px-3 text-sm text-slate-100 outline-none"
              />
            </FieldLabel>
            <FieldLabel label="生图积分兜底扣费（积分/张）">
              <input
                type="number"
                min={1}
                value={form.imageGeneratePointCost}
                onChange={(event) =>
                  onFormChange((current) => ({ ...current, imageGeneratePointCost: Number(event.target.value) }))
                }
                className="h-10 w-full rounded-md border border-slate-700 bg-slate-900 px-3 text-sm text-slate-100 outline-none"
              />
            </FieldLabel>
            <div className="grid gap-3 sm:grid-cols-2">
              <FieldLabel label="签到赠送积分">
                <input
                  type="number"
                  min={0}
                  value={form.checkinPoints}
                  onChange={(event) =>
                    onFormChange((current) => ({ ...current, checkinPoints: Number(event.target.value) }))
                  }
                  className="h-10 w-full rounded-md border border-slate-700 bg-slate-900 px-3 text-sm text-slate-100 outline-none"
                />
              </FieldLabel>
              <FieldLabel label="签到赠送次数">
                <input
                  type="number"
                  min={0}
                  value={form.checkinCredits}
                  onChange={(event) =>
                    onFormChange((current) => ({ ...current, checkinCredits: Number(event.target.value) }))
                  }
                  className="h-10 w-full rounded-md border border-slate-700 bg-slate-900 px-3 text-sm text-slate-100 outline-none"
                />
              </FieldLabel>
            </div>

            <button
              type="button"
              onClick={onSave}
              disabled={busy === 'save-billing'}
              className="inline-flex items-center justify-center gap-2 rounded-md bg-emerald-400 px-3 py-2 text-sm font-medium text-slate-950 transition hover:bg-emerald-300 disabled:cursor-not-allowed disabled:opacity-60"
            >
              {busy === 'save-billing' ? <LoaderCircle className="h-4 w-4 animate-spin" /> : <Save className="h-4 w-4" />}
              保存规则
            </button>
          </div>
        </div>

        <div className="grid gap-3 sm:grid-cols-2">
          <RuleCard label="当前单次价格" value={`${((settings?.customRechargeUnitPriceCents ?? form.customRechargeUnitPriceCents) / 100).toFixed(2)} 元 / 次`} />
          <RuleCard label="过夜转积分" value="剩余次数 × 当日单次价格（分）" />
          <RuleCard label="扣费顺序" value="优先扣次数，不足再扣积分" />
          <RuleCard label="积分价值" value="1 积分 = 0.01 元" />
        </div>
      </div>
    </SectionShell>
  )
}

function RuleCard({ label, value }: { label: string; value: string }) {
  return (
    <div className="rounded-lg border border-slate-800 bg-slate-950/60 p-4">
      <div className="text-xs text-slate-500">{label}</div>
      <div className="mt-2 text-sm font-medium text-slate-100">{value}</div>
    </div>
  )
}

function PackagesTab({
  response,
  busy,
  filterValue,
  onFilterChange,
  onSearch,
  onPageChange,
  editingPackageId,
  form,
  onFormChange,
  onStartCreate,
  onStartEdit,
  onReset,
  onSave,
  onDelete,
}: {
  response: AdminListPackagesResponse | null
  busy: BusyState
  filterValue: string
  onFilterChange: (value: string) => void
  onSearch: () => void
  onPageChange: (page: number) => void
  editingPackageId: string | null
  form: AdminPackageForm
  onFormChange: Dispatch<SetStateAction<AdminPackageForm>>
  onStartCreate: () => void
  onStartEdit: (item: AdminPackageItem) => void
  onReset: () => void
  onSave: () => void
  onDelete: (item: AdminPackageItem) => void
}) {
  const items = response?.items ?? []
  const isRedeem = false

  return (
    <SectionShell
      title="套餐配置"
      description="维护充值档位和积分兑换档位。"
      toolbar={
        <div className="flex flex-wrap items-center gap-2">
          <select
            value={filterValue}
            onChange={(event) => onFilterChange(event.target.value)}
            className="h-10 rounded-md border border-slate-700 bg-slate-900 px-3 text-sm text-slate-100 outline-none"
          >
            <option value="">全部充值套餐</option>
            <option value="recharge">充值套餐</option>
          </select>
          <button
            type="button"
            onClick={onSearch}
            className="inline-flex items-center gap-2 rounded-md border border-slate-700 bg-slate-900 px-3 py-2 text-sm text-slate-200 transition hover:border-emerald-400 hover:text-white"
          >
            <Filter className="h-4 w-4" />
            筛选
          </button>
          <button
            type="button"
            onClick={onStartCreate}
            className="inline-flex items-center gap-2 rounded-md bg-emerald-400 px-3 py-2 text-sm font-medium text-slate-950 transition hover:bg-emerald-300"
          >
            <Package className="h-4 w-4" />
            新建
          </button>
        </div>
      }
    >
      <div className="grid gap-4 xl:grid-cols-[420px_minmax(0,1fr)]">
        <div className="rounded-lg border border-slate-800 bg-slate-950/60 p-4">
          <div className="flex flex-wrap items-center justify-between gap-2">
            <div>
              <div className="text-sm font-medium text-slate-50">{editingPackageId ? '编辑套餐' : '新建套餐'}</div>
              <div className="mt-1 text-xs text-slate-500">套餐切换时，字段会自动适配档位类型。</div>
            </div>
            <button
              type="button"
              onClick={onReset}
              className="inline-flex items-center gap-2 rounded-md border border-slate-700 px-3 py-2 text-xs text-slate-200 transition hover:border-slate-500"
            >
              <RefreshCcw className="h-3.5 w-3.5" />
              重置
            </button>
          </div>

          <div className="mt-4 grid gap-3">
            {!editingPackageId ? (
              <FieldLabel label="套餐编码">
                <input
                  value={form.code}
                  onChange={(event) => onFormChange((current) => ({ ...current, code: event.target.value }))}
                  className="h-10 w-full rounded-md border border-slate-700 bg-slate-900 px-3 text-sm text-slate-100 outline-none"
                  placeholder="recharge-100"
                />
              </FieldLabel>
            ) : null}

            <FieldLabel label="套餐名称">
              <input
                value={form.name}
                onChange={(event) => onFormChange((current) => ({ ...current, name: event.target.value }))}
                className="h-10 w-full rounded-md border border-slate-700 bg-slate-900 px-3 text-sm text-slate-100 outline-none"
                placeholder="例如：100 次充值包"
              />
            </FieldLabel>

            {!editingPackageId ? (
              <FieldLabel label="套餐类型">
                <select
                  value={form.packageType}
                  onChange={(event) =>
                    onFormChange((current) => ({
                      ...current,
                      packageType: 'recharge',
                      amountCents: current.amountCents || 1999,
                      pointsCost: 0,
                      bonusPoints: current.bonusPoints || 0,
                    }))
                  }
                  className="h-10 w-full rounded-md border border-slate-700 bg-slate-900 px-3 text-sm text-slate-100 outline-none"
                >
                  <option value="recharge">充值档位</option>
                </select>
              </FieldLabel>
            ) : null}

            <FieldLabel label="次数">
              <input
                type="number"
                min={1}
                value={form.creditsAmount}
                onChange={(event) =>
                  onFormChange((current) => ({ ...current, creditsAmount: Number(event.target.value) }))
                }
                className="h-10 w-full rounded-md border border-slate-700 bg-slate-900 px-3 text-sm text-slate-100 outline-none"
              />
            </FieldLabel>

            {isRedeem ? (
              <FieldLabel label="积分消耗">
                <input
                  type="number"
                  min={1}
                  value={form.pointsCost}
                  onChange={(event) =>
                    onFormChange((current) => ({ ...current, pointsCost: Number(event.target.value) }))
                  }
                  className="h-10 w-full rounded-md border border-slate-700 bg-slate-900 px-3 text-sm text-slate-100 outline-none"
                />
              </FieldLabel>
            ) : (
              <div className="grid gap-3 sm:grid-cols-2">
                <FieldLabel label="价格（分）">
                  <input
                    type="number"
                    min={0}
                    value={form.amountCents}
                    onChange={(event) =>
                      onFormChange((current) => ({ ...current, amountCents: Number(event.target.value) }))
                    }
                    className="h-10 w-full rounded-md border border-slate-700 bg-slate-900 px-3 text-sm text-slate-100 outline-none"
                  />
                </FieldLabel>
                <FieldLabel label="赠送积分">
                  <input
                    type="number"
                    min={0}
                    value={form.bonusPoints}
                    onChange={(event) =>
                      onFormChange((current) => ({ ...current, bonusPoints: Number(event.target.value) }))
                    }
                    className="h-10 w-full rounded-md border border-slate-700 bg-slate-900 px-3 text-sm text-slate-100 outline-none"
                  />
                </FieldLabel>
              </div>
            )}

            <FieldLabel label="排序">
              <input
                type="number"
                value={form.sortOrder}
                onChange={(event) => onFormChange((current) => ({ ...current, sortOrder: Number(event.target.value) }))}
                className="h-10 w-full rounded-md border border-slate-700 bg-slate-900 px-3 text-sm text-slate-100 outline-none"
              />
            </FieldLabel>

            <FieldLabel label="备注">
              <input
                value={form.note ?? ''}
                onChange={(event) => onFormChange((current) => ({ ...current, note: event.target.value }))}
                className="h-10 w-full rounded-md border border-slate-700 bg-slate-900 px-3 text-sm text-slate-100 outline-none"
                placeholder="用途说明"
              />
            </FieldLabel>

            <label className="flex items-center gap-2 rounded-md border border-slate-800 bg-slate-900 px-3 py-2 text-sm text-slate-300">
              <input
                type="checkbox"
                checked={form.enabled ?? true}
                onChange={(event) => onFormChange((current) => ({ ...current, enabled: event.target.checked }))}
              />
              启用
            </label>

            <div className="text-xs leading-6 text-slate-500">
              {isRedeem
                ? '积分兑换档位只需要填写次数和积分消耗。'
                : '充值档位只需要填写次数、价格和赠送积分。'}
            </div>

            <div className="flex gap-2">
              <button
                type="button"
                onClick={onSave}
                disabled={busy === 'save-package'}
                className="inline-flex items-center gap-2 rounded-md bg-emerald-400 px-3 py-2 text-sm font-medium text-slate-950 transition hover:bg-emerald-300 disabled:cursor-not-allowed disabled:opacity-60"
              >
                {busy === 'save-package' ? <LoaderCircle className="h-4 w-4 animate-spin" /> : <Save className="h-4 w-4" />}
                保存
              </button>
            </div>
          </div>
        </div>

        <div className="rounded-lg border border-slate-800 bg-slate-950/60 p-4">
          <div className="mb-3 text-sm text-slate-400">点击条目即可加载到编辑区。</div>
          <TableShell columns="minmax(100px, 140px) minmax(180px, 1fr) minmax(120px, 160px) minmax(120px, 160px) minmax(100px, 120px) minmax(120px, 140px)">
            <thead className="bg-slate-950/70 text-xs uppercase tracking-wide text-slate-500">
              <tr className="[&>th]:px-3 [&>th]:py-3 [&>th]:text-left">
                <th>编码</th>
                <th>名称</th>
                <th>类型</th>
                <th>次数/价格</th>
                <th>状态</th>
                <th>操作</th>
              </tr>
            </thead>
            <tbody className="divide-y divide-slate-800 text-sm">
              {items.length ? (
                items.map((item) => (
                  <tr
                    key={item.id}
                    className="cursor-pointer align-top transition hover:bg-slate-900/70 [&>td]:px-3 [&>td]:py-3"
                    onClick={() => onStartEdit(item)}
                  >
                    <td className="whitespace-nowrap text-slate-400">#{item.code}</td>
                    <td className="min-w-0">
                      <div className="truncate font-medium text-slate-100">{item.name}</div>
                      <div className="mt-1 text-xs text-slate-500">{item.note || '-'}</div>
                    </td>
                    <td>
                      <span className="rounded-md bg-violet-500/15 px-2 py-1 text-xs text-violet-200">
                        {getPackageTypeLabel(item.packageType)}
                      </span>
                    </td>
                    <td className="text-slate-300">
                      <div>次数 {item.creditsAmount}</div>
                      <div className="mt-1 text-xs text-slate-500">
                        {item.packageType === 'redeem' ? `积分 ${item.pointsCost}` : `金额 ${(item.amountCents / 100).toFixed(2)} 元 / 赠送 ${item.bonusPoints}`}
                      </div>
                    </td>
                    <td>
                      <span className={`rounded-md px-2 py-1 text-xs ${item.enabled ? 'bg-emerald-400/15 text-emerald-300' : 'bg-rose-500/15 text-rose-300'}`}>
                        {item.enabled ? '启用' : '停用'}
                      </span>
                    </td>
                    <td>
                      <div className="flex flex-wrap gap-2">
                        <button
                          type="button"
                          onClick={(event) => {
                            event.stopPropagation()
                            onStartEdit(item)
                          }}
                          className="inline-flex items-center gap-1 rounded-md border border-slate-700 px-2 py-1 text-xs text-slate-200 transition hover:border-emerald-400"
                        >
                          <Edit3 className="h-3.5 w-3.5" />
                          编辑
                        </button>
                        <button
                          type="button"
                          onClick={(event) => {
                            event.stopPropagation()
                            onDelete(item)
                          }}
                          disabled={busy === `delete-package:${item.id}`}
                          className="inline-flex items-center gap-1 rounded-md border border-slate-700 px-2 py-1 text-xs text-slate-200 transition hover:border-rose-400 disabled:cursor-not-allowed disabled:opacity-60"
                        >
                          {busy === `delete-package:${item.id}` ? (
                            <LoaderCircle className="h-3.5 w-3.5 animate-spin" />
                          ) : (
                            <Trash2 className="h-3.5 w-3.5" />
                          )}
                          删除
                        </button>
                      </div>
                    </td>
                  </tr>
                ))
              ) : (
                <tr>
                  <td colSpan={6}>
                    <EmptyState text="暂无套餐数据" />
                  </td>
                </tr>
              )}
            </tbody>
          </TableShell>

          <PaginationBar
            page={response?.page ?? 1}
            pageSize={response?.pageSize ?? PAGE_SIZE}
            total={response?.total ?? 0}
            onPageChange={onPageChange}
          />
        </div>
      </div>
    </SectionShell>
  )
}

function TasksTab({
  response,
  busy,
  statusFilter,
  onStatusChange,
  onSearch,
  onPageChange,
  onOpenDetail,
}: {
  response: AdminListImageTasksResponse | null
  busy: BusyState
  statusFilter: string
  onStatusChange: (value: string) => void
  onSearch: () => void
  onPageChange: (page: number) => void
  onOpenDetail: (task: AdminImageTaskItem) => void
}) {
  const items = response?.items ?? []

  return (
    <SectionShell
      title="任务管理"
      description="查看全站任务进度、结果和失败原因。"
      toolbar={
        <div className="flex flex-wrap items-center gap-2">
          <select
            value={statusFilter}
            onChange={(event) => onStatusChange(event.target.value)}
            className="h-10 rounded-md border border-slate-700 bg-slate-900 px-3 text-sm text-slate-100 outline-none"
          >
            <option value="">全部状态</option>
            <option value="generating">生成中</option>
            <option value="success">成功</option>
            <option value="failed">失败</option>
          </select>
          <button
            type="button"
            onClick={onSearch}
            className="inline-flex items-center gap-2 rounded-md border border-slate-700 bg-slate-900 px-3 py-2 text-sm text-slate-200 transition hover:border-emerald-400 hover:text-white"
          >
            <Filter className="h-4 w-4" />
            筛选
          </button>
        </div>
      }
    >
      <TableShell columns="minmax(90px, 120px) minmax(100px, 130px) minmax(120px, 150px) minmax(130px, 160px) minmax(140px, 170px) minmax(180px, 200px) minmax(100px, 120px)">
        <thead className="bg-slate-950/70 text-xs uppercase tracking-wide text-slate-500">
          <tr className="[&>th]:px-3 [&>th]:py-3 [&>th]:text-left">
            <th>ID</th>
            <th>用户</th>
            <th>状态</th>
            <th>消耗</th>
            <th>来源</th>
            <th>时间</th>
            <th>详情</th>
          </tr>
        </thead>
        <tbody className="divide-y divide-slate-800 text-sm">
          {items.length ? (
            items.map((task) => (
              <tr key={task.id} className="align-top [&>td]:px-3 [&>td]:py-3">
                <td className="whitespace-nowrap text-slate-500">#{task.id}</td>
                <td className="text-slate-300">#{task.userId}</td>
                <td>
                  <span className={`rounded-md px-2 py-1 text-xs ${getTaskStatusClass(task.status)}`}>
                    {getImageTaskStatusLabel(task.status)}
                  </span>
                </td>
                <td className="text-slate-300">
                  <div>{task.costCredits || task.costPoints} 次</div>
                  <div className="mt-1 text-xs text-slate-500">
                    免费 {task.freeCreditsCost} / 充值 {task.paidCreditsCost}
                  </div>
                </td>
                <td className="text-slate-300">{getCreditSourceLabel(task.creditSource)}</td>
                <td className="whitespace-nowrap text-slate-400">{formatDateTime(task.createdAt)}</td>
                <td>
                  <button
                    type="button"
                    onClick={() => onOpenDetail(task)}
                    className="inline-flex items-center gap-1 rounded-md border border-slate-700 px-2 py-1 text-xs text-slate-200 transition hover:border-violet-400 hover:text-white"
                  >
                    <Eye className="h-3.5 w-3.5" />
                    详情
                  </button>
                </td>
              </tr>
            ))
          ) : (
            <tr>
              <td colSpan={7}>
                <EmptyState text="暂无任务数据" />
              </td>
            </tr>
          )}
        </tbody>
      </TableShell>

      <PaginationBar
        page={response?.page ?? 1}
        pageSize={response?.pageSize ?? PAGE_SIZE}
        total={response?.total ?? 0}
        onPageChange={onPageChange}
      />

      {busy === 'tasks' ? <div className="mt-3 text-xs text-slate-500">正在加载任务列表...</div> : null}
    </SectionShell>
  )
}

function OrdersTab({
  response,
  busy,
  statusFilter,
  onStatusChange,
  onSearch,
  onPageChange,
}: {
  response: AdminListRechargeOrdersResponse | null
  busy: BusyState
  statusFilter: string
  onStatusChange: (value: string) => void
  onSearch: () => void
  onPageChange: (page: number) => void
}) {
  const items = response?.items ?? []

  return (
    <SectionShell
      title="订单管理"
      description="查看充值订单与支付状态。"
      toolbar={
        <div className="flex flex-wrap items-center gap-2">
          <select
            value={statusFilter}
            onChange={(event) => onStatusChange(event.target.value)}
            className="h-10 rounded-md border border-slate-700 bg-slate-900 px-3 text-sm text-slate-100 outline-none"
          >
            <option value="">全部状态</option>
            <option value="pending">待支付</option>
            <option value="paid">已支付</option>
            <option value="canceled">已取消</option>
            <option value="refunded">已退款</option>
          </select>
          <button
            type="button"
            onClick={onSearch}
            className="inline-flex items-center gap-2 rounded-md border border-slate-700 bg-slate-900 px-3 py-2 text-sm text-slate-200 transition hover:border-emerald-400 hover:text-white"
          >
            <Filter className="h-4 w-4" />
            筛选
          </button>
        </div>
      }
    >
      <TableShell columns="minmax(140px, 180px) minmax(100px, 130px) minmax(200px, 1.3fr) minmax(120px, 150px) minmax(140px, 160px) minmax(120px, 140px) minmax(150px, 180px) minmax(160px, 180px)">
        <thead className="bg-slate-950/70 text-xs uppercase tracking-wide text-slate-500">
          <tr className="[&>th]:px-3 [&>th]:py-3 [&>th]:text-left">
            <th>订单号</th>
            <th>用户</th>
            <th>套餐</th>
            <th>金额</th>
            <th>次数/积分</th>
            <th>状态</th>
            <th>支付时间</th>
            <th>创建时间</th>
          </tr>
        </thead>
        <tbody className="divide-y divide-slate-800 text-sm">
          {items.length ? (
            items.map((order) => (
              <tr key={order.id} className="align-top [&>td]:px-3 [&>td]:py-3">
                <td className="whitespace-nowrap text-slate-300">{order.orderNo}</td>
                <td className="text-slate-300">#{order.userId}</td>
                <td>
                  <div className="font-medium text-slate-100">{order.packageName}</div>
                  <div className="mt-1 text-xs text-slate-500">{order.packageCode}</div>
                </td>
                <td className="text-slate-300">{(order.amountCents / 100).toFixed(2)} 元</td>
                <td className="text-slate-300">
                  <div>{order.creditsAmount} 次</div>
                  <div className="mt-1 text-xs text-slate-500">赠送 {order.bonusPoints} 积分</div>
                </td>
                <td>
                  <span className={`rounded-md px-2 py-1 text-xs ${getOrderStatusClass(order.status)}`}>
                    {getRechargeOrderStatusLabel(order.status)}
                  </span>
                </td>
                <td className="whitespace-nowrap text-slate-400">{formatDateTime(order.paidAt)}</td>
                <td className="whitespace-nowrap text-slate-400">{formatDateTime(order.createdAt)}</td>
              </tr>
            ))
          ) : (
            <tr>
              <td colSpan={8}>
                <EmptyState text="暂无订单数据" />
              </td>
            </tr>
          )}
        </tbody>
      </TableShell>

      <PaginationBar
        page={response?.page ?? 1}
        pageSize={response?.pageSize ?? PAGE_SIZE}
        total={response?.total ?? 0}
        onPageChange={onPageChange}
      />

      {busy === 'orders' ? <div className="mt-3 text-xs text-slate-500">正在加载订单列表...</div> : null}
    </SectionShell>
  )
}

function AuditsTab({
  response,
  busy,
  statusFilter,
  onStatusChange,
  onSearch,
  onPageChange,
  onReview,
}: {
  response: AdminListContentAuditsResponse | null
  busy: BusyState
  statusFilter: string
  onStatusChange: (value: string) => void
  onSearch: () => void
  onPageChange: (page: number) => void
  onReview: (item: AdminContentAuditItem, status: 'pass' | 'rejected') => void
}) {
  const items = response?.items ?? []

  return (
    <SectionShell
      title="内容审核"
      description="查看自动审核结果，并可由管理员复审。"
      toolbar={
        <div className="flex flex-wrap items-center gap-2">
          <select
            value={statusFilter}
            onChange={(event) => onStatusChange(event.target.value)}
            className="h-10 rounded-md border border-slate-700 bg-slate-900 px-3 text-sm text-slate-100 outline-none"
          >
            <option value="">全部状态</option>
            <option value="pass">通过</option>
            <option value="rejected">拒绝</option>
          </select>
          <button
            type="button"
            onClick={onSearch}
            className="inline-flex items-center gap-2 rounded-md border border-slate-700 bg-slate-900 px-3 py-2 text-sm text-slate-200 transition hover:border-emerald-400 hover:text-white"
          >
            <Filter className="h-4 w-4" />
            筛选
          </button>
        </div>
      }
    >
      <TableShell columns="minmax(80px, 110px) minmax(100px, 130px) minmax(150px, 190px) minmax(320px, 1.5fr) minmax(130px, 160px) minmax(170px, 220px) minmax(180px, 220px) minmax(180px, 220px) minmax(180px, 220px)">
        <thead className="bg-slate-950/70 text-xs uppercase tracking-wide text-slate-500">
          <tr className="[&>th]:px-3 [&>th]:py-3 [&>th]:text-left">
            <th>ID</th>
            <th>用户</th>
            <th>业务</th>
            <th>内容</th>
            <th>状态</th>
            <th>审核信息</th>
            <th>创建时间</th>
            <th>操作</th>
          </tr>
        </thead>
        <tbody className="divide-y divide-slate-800 text-sm">
          {items.length ? (
            items.map((item) => (
              <tr key={item.id} className="align-top [&>td]:px-3 [&>td]:py-3">
                <td className="whitespace-nowrap text-slate-500">#{item.id}</td>
                <td className="text-slate-300">#{item.userId}</td>
                <td>
                  <div className="font-medium text-slate-100">{item.bizType}</div>
                  <div className="mt-1 text-xs text-slate-500">{item.bizId || '-'}</div>
                </td>
                <td>
                  <div className="line-clamp-3 text-slate-200">{item.content}</div>
                  {item.matchedKeywords?.length ? (
                    <div className="mt-2 flex flex-wrap gap-1">
                      {item.matchedKeywords.map((keyword) => (
                        <span key={keyword} className="rounded-md bg-slate-800 px-2 py-1 text-[11px] text-slate-300">
                          {keyword}
                        </span>
                      ))}
                    </div>
                  ) : null}
                </td>
                <td>
                  <span className={`rounded-md px-2 py-1 text-xs ${item.status === 'rejected' ? 'bg-rose-500/15 text-rose-300' : 'bg-emerald-400/15 text-emerald-300'}`}>
                    {getAuditStatusLabel(item.status)}
                  </span>
                </td>
                <td className="text-slate-300">
                  <div>{item.action}</div>
                  <div className="mt-1 text-xs text-slate-500">{item.reason || '-'}</div>
                  <div className="mt-1 text-xs text-slate-500">审核人 {item.reviewerUserId || '-'}</div>
                  <div className="mt-1 text-xs text-slate-500">{formatDateTime(item.reviewedAt)}</div>
                </td>
                <td className="whitespace-nowrap text-slate-400">{formatDateTime(item.createdAt)}</td>
                <td>
                  <div className="flex flex-wrap gap-2">
                    <button
                      type="button"
                      onClick={() => onReview(item, 'pass')}
                      disabled={busy === `audit-review:${item.id}`}
                      className="inline-flex items-center gap-1 rounded-md border border-emerald-500/40 px-2 py-1 text-xs text-emerald-300 transition hover:border-emerald-400 disabled:cursor-not-allowed disabled:opacity-60"
                    >
                      <CheckCircle2 className="h-3.5 w-3.5" />
                      通过
                    </button>
                    <button
                      type="button"
                      onClick={() => onReview(item, 'rejected')}
                      disabled={busy === `audit-review:${item.id}`}
                      className="inline-flex items-center gap-1 rounded-md border border-rose-500/40 px-2 py-1 text-xs text-rose-300 transition hover:border-rose-400 disabled:cursor-not-allowed disabled:opacity-60"
                    >
                      <XCircle className="h-3.5 w-3.5" />
                      驳回
                    </button>
                  </div>
                </td>
              </tr>
            ))
          ) : (
            <tr>
              <td colSpan={8}>
                <EmptyState text="暂无审核记录" />
              </td>
            </tr>
          )}
        </tbody>
      </TableShell>

      <PaginationBar
        page={response?.page ?? 1}
        pageSize={response?.pageSize ?? PAGE_SIZE}
        total={response?.total ?? 0}
        onPageChange={onPageChange}
      />
    </SectionShell>
  )
}

function LogsTab({
  response,
  busy,
  targetType,
  targetId,
  onTargetTypeChange,
  onTargetIdChange,
  onSearch,
  onPageChange,
}: {
  response: AdminListActionLogsResponse | null
  busy: BusyState
  targetType: string
  targetId: string
  onTargetTypeChange: (value: string) => void
  onTargetIdChange: (value: string) => void
  onSearch: () => void
  onPageChange: (page: number) => void
}) {
  const items = response?.items ?? []

  return (
    <SectionShell
      title="操作日志"
      description="记录管理员的配置修改、套餐管理和审核动作。"
      toolbar={
        <div className="flex flex-wrap items-center gap-2">
          <div className="flex h-10 items-center gap-2 rounded-md border border-slate-700 bg-slate-900 px-3">
            <Filter className="h-4 w-4 text-slate-500" />
            <input
              value={targetType}
              onChange={(event) => onTargetTypeChange(event.target.value)}
              placeholder="目标类型"
              className="w-28 bg-transparent text-sm text-slate-100 outline-none placeholder:text-slate-500"
            />
          </div>
          <div className="flex h-10 items-center gap-2 rounded-md border border-slate-700 bg-slate-900 px-3">
            <Search className="h-4 w-4 text-slate-500" />
            <input
              value={targetId}
              onChange={(event) => onTargetIdChange(event.target.value)}
              placeholder="目标 ID"
              className="w-28 bg-transparent text-sm text-slate-100 outline-none placeholder:text-slate-500"
            />
          </div>
          <button
            type="button"
            onClick={onSearch}
            className="inline-flex items-center gap-2 rounded-md border border-slate-700 bg-slate-900 px-3 py-2 text-sm text-slate-200 transition hover:border-emerald-400 hover:text-white"
          >
            <Filter className="h-4 w-4" />
            筛选
          </button>
        </div>
      }
    >
      <TableShell columns="minmax(90px, 120px) minmax(90px, 120px) minmax(160px, 220px) minmax(150px, 180px) minmax(120px, 140px) minmax(220px, 1fr)">
        <thead className="bg-slate-950/70 text-xs uppercase tracking-wide text-slate-500">
          <tr className="[&>th]:px-3 [&>th]:py-3 [&>th]:text-left">
            <th>ID</th>
            <th>管理员</th>
            <th>动作</th>
            <th>目标</th>
            <th>备注</th>
            <th>时间 / 详情</th>
          </tr>
        </thead>
        <tbody className="divide-y divide-slate-800 text-sm">
          {items.length ? (
            items.map((item) => (
              <tr key={item.id} className="align-top [&>td]:px-3 [&>td]:py-3">
                <td className="whitespace-nowrap text-slate-500">#{item.id}</td>
                <td className="text-slate-300">#{item.adminId}</td>
                <td className="text-slate-200">{item.action}</td>
                <td className="text-slate-300">
                  <div>{item.targetType}</div>
                  <div className="mt-1 text-xs text-slate-500">{item.targetId}</div>
                </td>
                <td className="text-slate-300">{item.remark || '-'}</td>
                <td>
                  <div className="text-slate-400">{formatDateTime(item.createdAt)}</div>
                  <details className="mt-2 rounded-md border border-slate-800 bg-slate-950 p-2 text-xs text-slate-400">
                    <summary className="cursor-pointer select-none text-slate-300">查看 JSON</summary>
                    <div className="mt-2 space-y-2">
                      {item.beforeJson ? <pre className="overflow-x-auto whitespace-pre-wrap break-words">{item.beforeJson}</pre> : null}
                      {item.afterJson ? <pre className="overflow-x-auto whitespace-pre-wrap break-words">{item.afterJson}</pre> : null}
                    </div>
                  </details>
                </td>
              </tr>
            ))
          ) : (
            <tr>
              <td colSpan={6}>
                <EmptyState text="暂无操作日志" />
              </td>
            </tr>
          )}
        </tbody>
      </TableShell>

      <PaginationBar
        page={response?.page ?? 1}
        pageSize={response?.pageSize ?? PAGE_SIZE}
        total={response?.total ?? 0}
        onPageChange={onPageChange}
      />

      {busy === 'logs' ? <div className="mt-3 text-xs text-slate-500">正在加载操作日志...</div> : null}
    </SectionShell>
  )
}

function ModalShell({
  title,
  description,
  children,
  onClose,
  maxWidthClass = 'max-w-2xl',
}: {
  title: string
  description?: string
  children: ReactNode
  onClose: () => void
  maxWidthClass?: string
}) {
  return (
    <div className="fixed inset-0 z-50 flex items-center justify-center px-4 py-6">
      <button
        type="button"
        aria-label="关闭弹窗"
        onClick={onClose}
        className="absolute inset-0 bg-slate-950/75 backdrop-blur-sm"
      />
      <section
        className={`relative z-10 max-h-[88vh] w-full overflow-hidden rounded-lg border border-slate-800 bg-slate-900 shadow-panel ${maxWidthClass}`}
      >
        <div className="flex items-start justify-between gap-4 border-b border-slate-800 px-5 py-4">
          <div className="min-w-0">
            <h2 className="text-lg font-semibold text-slate-50">{title}</h2>
            {description ? <p className="mt-1 text-sm leading-6 text-slate-400">{description}</p> : null}
          </div>
          <button
            type="button"
            onClick={onClose}
            className="inline-flex h-9 w-9 shrink-0 items-center justify-center rounded-md border border-slate-700 bg-slate-950 text-slate-300 transition hover:border-slate-500 hover:text-white"
            title="关闭"
          >
            <XCircle className="h-4 w-4" />
          </button>
        </div>
        <div className="max-h-[calc(88vh-84px)] overflow-y-auto px-5 py-4">{children}</div>
      </section>
    </div>
  )
}

function PointAdjustDialog({
  dialog,
  busy,
  onChange,
  onClose,
  onSubmit,
}: {
  dialog: UserPointsDialogState
  busy: BusyState
  onChange: Dispatch<SetStateAction<UserPointsDialogState>>
  onClose: () => void
  onSubmit: () => void
}) {
  const isIncrease = dialog.action === 'increase'
  const user = dialog.user

  return (
    <ModalShell
      title={isIncrease ? '增加积分' : '减少积分'}
      description={user ? `正在调整用户「${user.username}」的积分。` : undefined}
      onClose={onClose}
      maxWidthClass="max-w-lg"
    >
      <form
        className="space-y-4"
        onSubmit={(event) => {
          event.preventDefault()
          onSubmit()
        }}
      >
        <div className="rounded-lg border border-slate-800 bg-slate-950 p-3 text-sm text-slate-300">
          <div>当前积分：{user?.pointsBalance ?? '-'}</div>
          <div className="mt-1 text-xs text-slate-500">请输入正整数，系统会按当前动作自动增加或减少。</div>
        </div>

        <FieldLabel label="积分数量">
          <input
            autoFocus
            type="number"
            min={1}
            step={1}
            value={dialog.amount}
            onChange={(event) => onChange((current) => ({ ...current, amount: event.target.value }))}
            className="h-11 w-full rounded-md border border-slate-700 bg-slate-950 px-3 text-sm text-slate-100 outline-none transition focus:border-violet-400"
            placeholder="请输入积分数量"
          />
        </FieldLabel>

        <FieldLabel label="备注">
          <input
            value={dialog.remark}
            onChange={(event) => onChange((current) => ({ ...current, remark: event.target.value }))}
            className="h-11 w-full rounded-md border border-slate-700 bg-slate-950 px-3 text-sm text-slate-100 outline-none transition focus:border-violet-400"
            placeholder="例如：后台人工调整"
          />
        </FieldLabel>

        <div className="flex flex-col-reverse gap-2 sm:flex-row sm:justify-end">
          <button
            type="button"
            onClick={onClose}
            className="inline-flex h-10 items-center justify-center rounded-md border border-slate-700 px-4 text-sm text-slate-200 transition hover:border-slate-500 hover:text-white"
          >
            取消
          </button>
          <button
            type="submit"
            disabled={busy === `user-points:${user?.id ?? ''}`}
            className={`inline-flex h-10 items-center justify-center gap-2 rounded-md px-4 text-sm font-medium transition disabled:cursor-not-allowed disabled:opacity-60 ${
              isIncrease
                ? 'bg-emerald-400 text-slate-950 hover:bg-emerald-300'
                : 'bg-amber-400 text-slate-950 hover:bg-amber-300'
            }`}
          >
            {busy === `user-points:${user?.id ?? ''}` ? <LoaderCircle className="h-4 w-4 animate-spin" /> : null}
            确认{isIncrease ? '增加' : '减少'}
          </button>
        </div>
      </form>
    </ModalShell>
  )
}

function UserActivityDialog({
  dialog,
  onClose,
  onPageChange,
}: {
  dialog: UserActivityDialogState
  onClose: () => void
  onPageChange: (page: number) => void
}) {
  const user = dialog.user
  const items = dialog.response?.items ?? []

  return (
    <ModalShell
      title="用户日志"
      description={user ? `${user.username} / ${user.email}` : undefined}
      onClose={onClose}
      maxWidthClass="max-w-3xl"
    >
      <div className="space-y-4">
        {user ? (
          <div className="grid gap-3 rounded-lg border border-slate-800 bg-slate-950 p-3 text-sm text-slate-300 sm:grid-cols-3">
            <div>
              <div className="text-xs text-slate-500">积分</div>
              <div className="mt-1 text-slate-100">{user.pointsBalance}</div>
            </div>
            <div>
              <div className="text-xs text-slate-500">免费次数</div>
              <div className="mt-1 text-slate-100">{user.freeCredits}</div>
            </div>
            <div>
              <div className="text-xs text-slate-500">充值次数</div>
              <div className="mt-1 text-slate-100">{user.paidCredits}</div>
            </div>
          </div>
        ) : null}

        {dialog.error ? (
          <div className="rounded-md border border-rose-500/30 bg-rose-500/10 px-3 py-2 text-sm text-rose-200">
            {dialog.error}
          </div>
        ) : null}

        {dialog.loading ? (
          <div className="flex items-center gap-2 rounded-md border border-slate-800 bg-slate-950 p-4 text-sm text-slate-400">
            <LoaderCircle className="h-4 w-4 animate-spin" />
            正在加载用户日志...
          </div>
        ) : items.length ? (
          <div className="space-y-3">
            {items.map((item) => (
              <UserActivityRow key={`${item.recordType}-${item.id}`} item={item} />
            ))}
          </div>
        ) : (
          <EmptyState text="暂无用户日志" />
        )}

        <PaginationBar
          page={dialog.response?.page ?? dialog.page}
          pageSize={dialog.response?.pageSize ?? USER_ACTIVITY_PAGE_SIZE}
          total={dialog.response?.total ?? 0}
          onPageChange={onPageChange}
        />
      </div>
    </ModalShell>
  )
}

function UserActivityRow({ item }: { item: UserActivityItem }) {
  return (
    <div className="flex gap-3 rounded-lg border border-slate-800 bg-slate-950 p-3">
      <div className="flex h-12 w-12 shrink-0 items-center justify-center overflow-hidden rounded-md border border-slate-800 bg-slate-900 text-[11px] text-slate-400">
        {item.imageUrl ? (
          <img src={item.imageUrl} alt={item.title} className="h-full w-full object-cover" />
        ) : (
          activityRecordShortLabel(item.recordType)
        )}
      </div>
      <div className="min-w-0 flex-1">
        <div className="flex flex-wrap items-start justify-between gap-2">
          <div className="min-w-0">
            <div className="truncate text-sm font-medium text-slate-100">{item.title}</div>
            <p className="mt-1 line-clamp-2 text-xs leading-5 text-slate-500">{item.description}</p>
          </div>
          <span className="rounded-md border border-slate-700 bg-slate-900 px-2 py-0.5 text-xs text-slate-300">
            {activityRecordLabel(item.recordType)}
          </span>
        </div>
        <div className="mt-2 flex flex-wrap items-center justify-between gap-2 text-xs text-slate-500">
          <span>{formatDateTime(item.createdAt)}</span>
          <span>{activityRecordValue(item)}</span>
        </div>
      </div>
    </div>
  )
}

function TaskDetailDialog({
  dialog,
  onClose,
}: {
  dialog: TaskDetailDialogState
  onClose: () => void
}) {
  const task = dialog.task
  const imageURLs = task ? collectTaskImageURLs(task) : []
  const requestOptions = task ? buildTaskRequestOptions(task) : []
  const requestParamsText = task ? formatTaskRequestParams(task.requestParams) : ''

  return (
    <ModalShell
      title="任务详情"
      description={task ? `任务 #${task.id} / 用户 #${task.userId}` : undefined}
      onClose={onClose}
      maxWidthClass="max-w-4xl"
    >
      {task ? (
        <div className="space-y-4">
          <div className="grid gap-3 sm:grid-cols-2 xl:grid-cols-4">
            <StatTile label="状态" value={getImageTaskStatusLabel(task.status)} />
            <StatTile label="消耗次数" value={String(task.costCredits || task.costPoints)} />
            <StatTile label="消耗来源" value={getCreditSourceLabel(task.creditSource)} />
            <StatTile label="创建时间" value={formatDateTime(task.createdAt)} />
          </div>

          <div className="rounded-lg border border-slate-800 bg-slate-950 p-4">
            <div className="mb-2 text-xs text-slate-500">提示词</div>
            <p className="whitespace-pre-wrap break-words text-sm leading-7 text-slate-100">{task.prompt || '-'}</p>
          </div>

          <div className="rounded-lg border border-slate-800 bg-slate-950 p-4">
            <div className="mb-3 text-sm font-medium text-slate-100">生成选项</div>
            {requestOptions.length ? (
              <div className="grid gap-3 sm:grid-cols-2 xl:grid-cols-3">
                {requestOptions.map((item) => (
                  <div key={item.label} className="min-w-0 rounded-md border border-slate-800 bg-slate-900/70 px-3 py-2">
                    <div className="text-xs text-slate-500">{item.label}</div>
                    <div className="mt-1 break-words text-sm text-slate-100">{item.value}</div>
                  </div>
                ))}
              </div>
            ) : (
              <EmptyState text="没有记录生成选项" />
            )}
          </div>

          <details className="rounded-lg border border-slate-800 bg-slate-950 p-4">
            <summary className="cursor-pointer text-sm font-medium text-slate-100">完整请求 JSON</summary>
            <pre className="mt-3 max-h-72 overflow-auto whitespace-pre-wrap break-words rounded-md border border-slate-800 bg-slate-900 p-3 text-xs leading-5 text-slate-300">
              {requestParamsText || '-'}
            </pre>
          </details>

          {task.errorMessage ? (
            <div className="rounded-lg border border-rose-500/30 bg-rose-500/10 p-4 text-sm leading-6 text-rose-200">
              <div className="mb-1 flex items-center gap-2 font-medium">
                <AlertTriangle className="h-4 w-4" />
                失败原因
              </div>
              {task.errorMessage}
            </div>
          ) : null}

          <div className="rounded-lg border border-slate-800 bg-slate-950 p-4">
            <div className="mb-3 flex items-center justify-between gap-3">
              <div className="text-sm font-medium text-slate-100">生成图片</div>
              {task.provider ? <span className="text-xs text-slate-500">服务商：{task.provider}</span> : null}
            </div>
            {imageURLs.length ? (
              <div className={`grid gap-3 ${imageURLs.length === 1 ? 'grid-cols-1' : 'grid-cols-1 sm:grid-cols-2'}`}>
                {imageURLs.map((imageUrl, index) => (
                  <a
                    key={imageUrl}
                    href={imageUrl}
                    target="_blank"
                    rel="noreferrer"
                    className="group overflow-hidden rounded-md border border-slate-800 bg-slate-900"
                  >
                    <img
                      src={imageUrl}
                      alt={`生成图片 ${index + 1}`}
                      className="max-h-[420px] w-full object-contain transition group-hover:opacity-90"
                    />
                    <div className="border-t border-slate-800 px-3 py-2 text-xs text-slate-500">打开图片 {index + 1}</div>
                  </a>
                ))}
              </div>
            ) : (
              <div className="flex min-h-[220px] flex-col items-center justify-center rounded-md border border-dashed border-slate-700 bg-slate-900/50 p-4 text-center text-sm text-slate-500">
                <ImageIcon className="mb-3 h-8 w-8 text-slate-600" />
                暂无生成图片。
              </div>
            )}
          </div>

          <div className="grid gap-3 rounded-lg border border-slate-800 bg-slate-950 p-4 text-sm text-slate-300 sm:grid-cols-3">
            <div>
              <div className="text-xs text-slate-500">免费次数消耗</div>
              <div className="mt-1 text-slate-100">{task.freeCreditsCost}</div>
            </div>
            <div>
              <div className="text-xs text-slate-500">充值次数消耗</div>
              <div className="mt-1 text-slate-100">{task.paidCreditsCost}</div>
            </div>
            <div>
              <div className="text-xs text-slate-500">旧积分字段</div>
              <div className="mt-1 text-slate-100">{task.costPoints}</div>
            </div>
          </div>
        </div>
      ) : (
        <EmptyState text="没有可展示的任务详情" />
      )}
    </ModalShell>
  )
}

function SectionShell({
  title,
  description,
  toolbar,
  children,
}: {
  title: string
  description: string
  toolbar?: ReactNode
  children: ReactNode
}) {
  return (
    <section className="space-y-4">
      <div className="flex flex-col gap-3 border-b border-slate-800 pb-4 lg:flex-row lg:items-center lg:justify-between">
        <div>
          <h1 className="text-xl font-semibold text-slate-50">{title}</h1>
          <p className="mt-1 text-sm text-slate-400">{description}</p>
        </div>
        {toolbar}
      </div>
      {children}
    </section>
  )
}

function TableShell({
  columns,
  children,
}: {
  columns: string
  children: ReactNode
}) {
  return (
    <div className="overflow-hidden rounded-lg border border-slate-800 bg-slate-900/70">
      <div className="overflow-x-auto">
        <table className="min-w-full" style={{ gridTemplateColumns: columns }}>
          {children}
        </table>
      </div>
    </div>
  )
}

function FieldLabel({ label, children }: { label: string; children: ReactNode }) {
  return (
    <label className="block">
      <span className="mb-2 block text-xs text-slate-500">{label}</span>
      {children}
    </label>
  )
}

function StatTile({ label, value }: { label: string; value: string }) {
  return (
    <div className="rounded-lg border border-slate-800 bg-slate-900/70 p-3">
      <div className="text-xs text-slate-500">{label}</div>
      <div className="mt-2 text-lg font-semibold text-slate-100">{value}</div>
    </div>
  )
}

function EmptyState({ text }: { text: string }) {
  return <div className="rounded-md border border-dashed border-slate-700 bg-slate-950 p-4 text-sm text-slate-500">{text}</div>
}

function PaginationBar({
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
  const canPrev = page > 1
  const canNext = page < totalPages

  return (
    <div className="mt-4 flex flex-col gap-3 border-t border-slate-800 pt-4 sm:flex-row sm:items-center sm:justify-between">
      <div className="text-xs text-slate-500">
        第 {page} / {totalPages} 页，共 {total} 条
      </div>
      <div className="flex items-center gap-2">
        <button
          type="button"
          onClick={() => onPageChange(Math.max(1, page - 1))}
          disabled={!canPrev}
          className="inline-flex items-center gap-2 rounded-md border border-slate-700 px-3 py-2 text-sm text-slate-200 transition hover:border-emerald-400 disabled:cursor-not-allowed disabled:opacity-50"
        >
          <ChevronLeft className="h-4 w-4" />
          上一页
        </button>
        <button
          type="button"
          onClick={() => onPageChange(Math.min(totalPages, page + 1))}
          disabled={!canNext}
          className="inline-flex items-center gap-2 rounded-md border border-slate-700 px-3 py-2 text-sm text-slate-200 transition hover:border-emerald-400 disabled:cursor-not-allowed disabled:opacity-50"
        >
          下一页
          <ChevronRight className="h-4 w-4" />
        </button>
      </div>
    </div>
  )
}

function collectTaskImageURLs(task: AdminImageTaskItem): string[] {
  const candidates = [...(task.imageUrls ?? []), task.imageUrl ?? '']
  return Array.from(new Set(candidates.map((item) => item.trim()).filter(Boolean)))
}

type TaskRequestSnapshot = {
  prompt?: string
  size?: string
  style?: string
  count?: number
  quality?: string
  outputFormat?: string
  referenceImageUrl?: string
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

function buildTaskRequestOptions(task: AdminImageTaskItem): Array<{ label: string; value: string }> {
  const parsed = parseTaskRequestParams(task.requestParams)
  if (!parsed) {
    return []
  }

  return [
    { label: '提示词', value: parsed.prompt || task.prompt || '-' },
    { label: '图片尺寸', value: formatTaskOptionValue(parsed.size, taskOptionLabels.size) },
    { label: '生成张数', value: parsed.count ? `${parsed.count} 张` : '-' },
    { label: '风格', value: formatTaskOptionValue(parsed.style, taskOptionLabels.style) },
    { label: '质量', value: formatTaskOptionValue(parsed.quality, taskOptionLabels.quality) },
    { label: '输出格式', value: formatTaskOptionValue(parsed.outputFormat, taskOptionLabels.outputFormat) },
    { label: '参考图片', value: parsed.referenceImageUrl || '-' },
  ]
}

function formatTaskRequestParams(raw?: string): string {
  if (!raw?.trim()) {
    return ''
  }

  try {
    return JSON.stringify(JSON.parse(raw), null, 2)
  } catch {
    return raw
  }
}

const taskOptionLabels = {
  size: {
    '1024x1024': '1024 x 1024',
    '768x1344': '768 x 1344',
    '1344x768': '1344 x 768',
  },
  style: {
    '': '无',
    realistic: '写实',
    anime: '动漫',
    cinematic: '电影感',
    sketch: '素描',
    watercolor: '水彩',
  },
  quality: {
    '': '默认',
    auto: '自动',
    low: '低',
    medium: '中',
    high: '高',
  },
  outputFormat: {
    '': '默认',
    default: '默认',
    jpeg: 'JPEG',
    webp: 'WebP',
    png: 'PNG',
  },
} satisfies Record<string, Record<string, string>>

function formatTaskOptionValue(value: string | undefined, labels: Record<string, string>): string {
  const trimmed = value?.trim() ?? ''
  if (Object.prototype.hasOwnProperty.call(labels, trimmed)) {
    return labels[trimmed]
  }

  return trimmed || '-'
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

function getPackageTypeLabel(value: string): string {
  switch (value) {
    case 'recharge':
      return '充值档位'
    case 'redeem':
      return '积分兑换'
    default:
      return value || '未知'
  }
}

function getImageTaskStatusLabel(status: string): string {
  switch (status) {
    case 'generating':
    case 'running':
      return '生成中'
    case 'success':
      return '成功'
    case 'failed':
      return '失败'
    case 'idle':
      return '待开始'
    default:
      return status || '未知'
  }
}

function getTaskStatusClass(status: string): string {
  switch (status) {
    case 'success':
      return 'bg-emerald-400/15 text-emerald-300'
    case 'failed':
      return 'bg-rose-500/15 text-rose-300'
    case 'generating':
    case 'running':
      return 'bg-amber-500/15 text-amber-300'
    default:
      return 'bg-slate-800 text-slate-300'
  }
}

function getOrderStatusClass(status: string): string {
  switch (status) {
    case 'paid':
      return 'bg-emerald-400/15 text-emerald-300'
    case 'pending':
      return 'bg-amber-500/15 text-amber-300'
    case 'canceled':
      return 'bg-slate-800 text-slate-300'
    case 'refunded':
      return 'bg-rose-500/15 text-rose-300'
    default:
      return 'bg-slate-800 text-slate-300'
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

function getCreditSourceLabel(value: string): string {
  switch (value) {
    case 'free':
      return '免费次数'
    case 'paid':
      return '充值次数'
    case 'mixed':
      return '混合消耗'
    default:
      return value || '未知'
  }
}

function getAuditStatusLabel(value: string): string {
  switch (value) {
    case 'pass':
      return '通过'
    case 'rejected':
      return '拒绝'
    default:
      return value || '未知'
  }
}

function formatDateTime(value?: string | null): string {
  if (!value) {
    return '-'
  }
  const date = new Date(value)
  if (Number.isNaN(date.getTime())) {
    return value
  }
  return new Intl.DateTimeFormat('zh-CN', {
    year: 'numeric',
    month: '2-digit',
    day: '2-digit',
    hour: '2-digit',
    minute: '2-digit',
    second: '2-digit',
    hour12: false,
  }).format(date)
}
