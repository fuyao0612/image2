import { useEffect, useMemo, useState, type ReactNode } from 'react'
import { createPortal } from 'react-dom'
import { Activity, AlertTriangle, ArrowLeft, Database, Edit, Eye, FileText, KeyRound, LayoutDashboard, LoaderCircle, LogOut, Menu, RefreshCcw, Save, Search, Sparkles, Timer, TrendingUp, Users, X } from 'lucide-react'
import { api } from '@/api/client'
import { TopNotice } from '@/components/TopNotice'
import type { AdminActionLogItem, AdminAnnouncementPayload, AdminCreateRedeemCodesRequest, AdminImageTaskItem, AdminListActionLogsResponse, AdminListAnnouncementsResponse, AdminListImageTasksResponse, AdminListPackagesResponse, AdminListProductCategoriesResponse, AdminListProductsResponse, AdminListPromptPlazaResponse, AdminListProviderKeysResponse, AdminListRedeemCodesResponse, AdminListUsersResponse, AdminPackageItem, AdminPackagePayload, AdminProductCategoryPayload, AdminProductPayload, AdminPromptPlazaPayload, AdminProviderKeyItem, AdminProviderKeyPayload, AdminRedeemCodeItem, AdminStatsResponse, AdminUserItem, AnnouncementItem, BillingSettingsResponse, DataCleanupRequest, DataCleanupResponse, ListUserActivityResponse, ProductCategoryItem, ProductItem, PromptPlazaItem, UpdateBillingSettingsRequest, UserInfo } from '@/types/api'

type Tab = 'overview' | 'tasks' | 'redeemCodes' | 'promptPlaza' | 'packages' | 'products' | 'providerKeys' | 'announcements' | 'rules' | 'users' | 'cleanup' | 'logs'
type Busy = string | null
type ConfirmState = { title: string; content: ReactNode; danger?: boolean; confirmText?: string; onConfirm: () => Promise<void> | void }
type Modal =
  | { type: 'none' }
  | { type: 'confirm'; data: ConfirmState }
  | { type: 'task'; item: AdminImageTaskItem }
  | { type: 'redeemCode'; item: AdminRedeemCodeItem }
  | { type: 'prompt'; item: PromptPlazaItem | null }
  | { type: 'promptReason'; action: 'reject' | 'delete'; item: PromptPlazaItem; reason: string }
  | { type: 'package'; item: AdminPackageItem | null }
  | { type: 'product'; item: ProductItem | null }
  | { type: 'category'; item: ProductCategoryItem | null }
  | { type: 'providerKey'; item: AdminProviderKeyItem | null }
  | { type: 'announcement'; item: AnnouncementItem | null }
  | { type: 'points'; user: AdminUserItem; delta: string; remark: string }
  | { type: 'userLogs'; user: AdminUserItem; response: ListUserActivityResponse | null; page: number; loading: boolean; error: string }
  | { type: 'log'; item: AdminActionLogItem }

const PAGE_SIZE = 10
const STORAGE_KEY = 'image2-admin-tab'
const defaultBilling: UpdateBillingSettingsRequest = { imageGeneratePointCost: 20, imageGenerationEnabled: true, emailNotifyEnabled: false, emailNotifyPointCost: 1, checkinPoints: 3, checkinCredits: 1, inviteRewardPoints: 0, inviteRewardCredits: 1, registrationGiftCredits: 3 }
const tabs: Array<{ id: Tab; label: string; icon: ReactNode }> = [
  { id: 'overview', label: '概览', icon: <LayoutDashboard className="h-4 w-4" /> },
  { id: 'tasks', label: '任务', icon: <FileText className="h-4 w-4" /> },
  { id: 'redeemCodes', label: '兑换码', icon: <KeyRound className="h-4 w-4" /> },
  { id: 'promptPlaza', label: '提示词广场', icon: <Sparkles className="h-4 w-4" /> },
  { id: 'packages', label: '商品套餐', icon: <Sparkles className="h-4 w-4" /> },
  { id: 'products', label: '商品管理', icon: <FileText className="h-4 w-4" /> },
  { id: 'providerKeys', label: 'apikey', icon: <KeyRound className="h-4 w-4" /> },
  { id: 'announcements', label: '公告', icon: <FileText className="h-4 w-4" /> },
  { id: 'rules', label: '规则', icon: <Activity className="h-4 w-4" /> },
  { id: 'users', label: '用户', icon: <Users className="h-4 w-4" /> },
  { id: 'cleanup', label: '数据清理', icon: <Database className="h-4 w-4" /> },
  { id: 'logs', label: '日志', icon: <Activity className="h-4 w-4" /> },
]
function getInitialTab(): Tab { try { const v = localStorage.getItem(STORAGE_KEY) as Tab | null; return tabs.some(t => t.id === v) ? v! : 'overview' } catch { return 'overview' } }

export function AdminConsole({ currentUser, onBackToApp, onLogout }: { currentUser: UserInfo | null; onBackToApp: () => void; onLogout: () => void }) {
  const [activeTab, setActiveTab] = useState<Tab>(getInitialTab)
  const [mobileNav, setMobileNav] = useState(false)
  const [busy, setBusy] = useState<Busy>(null)
  const [message, setMessage] = useState('')
  const [modal, setModal] = useState<Modal>({ type: 'none' })
  const [stats, setStats] = useState<AdminStatsResponse | null>(null)
  const [tasks, setTasks] = useState<AdminListImageTasksResponse | null>(null)
  const [taskStatus, setTaskStatus] = useState('')
  const [taskKeyword, setTaskKeyword] = useState('')
  const [taskPage, setTaskPage] = useState(1)
  const [redeem, setRedeem] = useState<AdminListRedeemCodesResponse | null>(null)
  const [redeemStatus, setRedeemStatus] = useState('')
  const [redeemKeyword, setRedeemKeyword] = useState('')
  const [redeemPage, setRedeemPage] = useState(1)
  const [redeemForm, setRedeemForm] = useState({ count: '10', pointsAmount: '0', creditsAmount: '10', prefix: '', note: '' })
  const [generatedCodes, setGeneratedCodes] = useState<AdminRedeemCodeItem[]>([])
  const [prompts, setPrompts] = useState<AdminListPromptPlazaResponse | null>(null)
  const [promptStatus, setPromptStatus] = useState<'approved' | 'pending' | 'rejected'>('approved')
  const [packages, setPackages] = useState<AdminListPackagesResponse | null>(null)
  const [packagePage, setPackagePage] = useState(1)
  const [products, setProducts] = useState<AdminListProductsResponse | null>(null)
  const [categories, setCategories] = useState<AdminListProductCategoriesResponse | null>(null)
  const [productKeyword, setProductKeyword] = useState('')
  const [productCategory, setProductCategory] = useState('')
  const [productEnabled, setProductEnabled] = useState('')
  const [productPage, setProductPage] = useState(1)
  const [providerKeys, setProviderKeys] = useState<AdminListProviderKeysResponse | null>(null)
  const [providerPage, setProviderPage] = useState(1)
  const [announcements, setAnnouncements] = useState<AdminListAnnouncementsResponse | null>(null)
  const [announcementPage, setAnnouncementPage] = useState(1)
  const [billing, setBilling] = useState<BillingSettingsResponse | null>(null)
  const [billingForm, setBillingForm] = useState<UpdateBillingSettingsRequest>(defaultBilling)
  const [users, setUsers] = useState<AdminListUsersResponse | null>(null)
  const [userKeyword, setUserKeyword] = useState('')
  const [userStatus, setUserStatus] = useState('')
  const [userPage, setUserPage] = useState(1)
  const [cleanupForm, setCleanupForm] = useState<DataCleanupRequest>({ type: 'failed_image_tasks', mode: 'retention', keepDays: 30 })
  const [cleanupResult, setCleanupResult] = useState<DataCleanupResponse | null>(null)
  const [logs, setLogs] = useState<AdminListActionLogsResponse | null>(null)
  const [logTargetType, setLogTargetType] = useState('')
  const [logTargetId, setLogTargetId] = useState('')
  const [logPage, setLogPage] = useState(1)
  const isAdmin = currentUser?.role === 'admin'

  useEffect(() => { if (currentUser && !isAdmin) onBackToApp() }, [currentUser, isAdmin, onBackToApp])
  useEffect(() => { try { localStorage.setItem(STORAGE_KEY, activeTab) } catch {} }, [activeTab])
  useEffect(() => { if (isAdmin) void loadTab(activeTab) }, [activeTab, isAdmin])
  useEffect(() => { if (isAdmin) void Promise.all([loadOverview(), loadPrompts('pending')]) }, [isAdmin])

  function errorText(error: unknown, fallback: string) { return error instanceof Error ? error.message : fallback }
  async function run(key: string, fn: () => Promise<void>) { setBusy(key); try { await fn() } catch (error) { setMessage(errorText(error, '操作失败')) } finally { setBusy(null) } }
  async function loadOverview() { try { setStats(await api.adminStats()) } catch (e) { setMessage(errorText(e, '概览加载失败')) } }
  async function loadTasks(page = taskPage, status = taskStatus, keyword = taskKeyword) { try { const r = await api.adminListImageTasks(page, PAGE_SIZE, status, keyword); setTasks(r); setTaskPage(r.page) } catch (e) { setMessage(errorText(e, '任务加载失败')) } }
  async function loadRedeem(page = redeemPage, status = redeemStatus, keyword = redeemKeyword) { try { const r = await api.adminListRedeemCodes(page, PAGE_SIZE, status, keyword); setRedeem(r); setRedeemPage(r.page) } catch (e) { setMessage(errorText(e, '兑换码加载失败')) } }
  async function loadPrompts(status = promptStatus) { try { const r = await api.adminListPromptPlaza(1, 100, status); setPrompts({ ...r, items: r.items.map(i => ({ ...i, tags: Array.isArray(i.tags) ? i.tags : [] })) }); setPromptStatus(status as typeof promptStatus) } catch (e) { setMessage(errorText(e, '提示词广场加载失败')) } }
  async function loadPackages(page = packagePage) { try { const r = await api.adminListPackages(page, PAGE_SIZE, ''); setPackages(r); setPackagePage(r.page) } catch (e) { setMessage(errorText(e, '商品套餐加载失败')) } }
  async function loadProducts(page = productPage, keyword = productKeyword, categoryId = productCategory, enabled = productEnabled) { try { const [p, c] = await Promise.all([api.adminListProducts(page, PAGE_SIZE, keyword, categoryId, enabled), api.adminListProductCategories(1, 50)]); setProducts(p); setProductPage(p.page); setCategories(c) } catch (e) { setMessage(errorText(e, '商品管理加载失败')) } }
  async function loadProviderKeys(page = providerPage) { try { const r = await api.adminListProviderKeys(page, 20); setProviderKeys(r); setProviderPage(r.page) } catch (e) { setMessage(errorText(e, 'API Key 加载失败')) } }
  async function loadAnnouncements(page = announcementPage) { try { const r = await api.adminListAnnouncements(page, PAGE_SIZE); setAnnouncements(r); setAnnouncementPage(r.page) } catch (e) { setMessage(errorText(e, '公告加载失败')) } }
  async function loadBilling() { try { const r = await api.adminGetBillingSettings(); setBilling(r); setBillingForm(r) } catch (e) { setMessage(errorText(e, '规则加载失败')) } }
  async function loadUsers(page = userPage, keyword = userKeyword, status = userStatus) { try { const r = await api.adminListUsers(page, PAGE_SIZE, keyword, status); setUsers(r); setUserPage(r.page) } catch (e) { setMessage(errorText(e, '用户加载失败')) } }
  async function loadLogs(page = logPage, targetType = logTargetType, targetId = logTargetId) { try { const r = await api.adminListActionLogs(page, PAGE_SIZE, targetType, targetId); setLogs(r); setLogPage(r.page) } catch (e) { setMessage(errorText(e, '操作日志加载失败')) } }
  async function loadTab(tab: Tab) { if (tab === 'overview') return loadOverview(); if (tab === 'tasks') return loadTasks(); if (tab === 'redeemCodes') return loadRedeem(); if (tab === 'promptPlaza') return loadPrompts(); if (tab === 'packages') return loadPackages(); if (tab === 'products') return loadProducts(); if (tab === 'providerKeys') return loadProviderKeys(); if (tab === 'announcements') return loadAnnouncements(); if (tab === 'rules') return loadBilling(); if (tab === 'users') return loadUsers(); if (tab === 'logs') return loadLogs() }

  async function refresh() { await run('refresh', async () => { await loadTab(activeTab); setMessage('已刷新') }) }
  async function toggleGeneration() { await run('toggle-generation', async () => { const payload = { ...billingForm, imageGenerationEnabled: !billingForm.imageGenerationEnabled }; const r = await api.adminUpdateBillingSettings(payload); setBilling(r); setBillingForm(r); setMessage(payload.imageGenerationEnabled ? '已开启生图' : '已关闭生图') }) }
  async function createRedeem() { const payload: AdminCreateRedeemCodesRequest = { count: Math.max(1, Number(redeemForm.count) || 1), pointsAmount: Math.max(0, Number(redeemForm.pointsAmount) || 0), creditsAmount: Math.max(0, Number(redeemForm.creditsAmount) || 0), prefix: redeemForm.prefix.trim() || undefined, note: redeemForm.note.trim() || undefined }; if (!payload.pointsAmount && !payload.creditsAmount) return setMessage('兑换码至少包含积分或福利次数'); await run('redeem-create', async () => { const r = await api.adminCreateRedeemCodes(payload); setGeneratedCodes(r.items); await loadRedeem(1); setMessage(`已生成 ${r.items.length} 个兑换码`) }) }
  async function saveBilling() { await run('rules-save', async () => { const r = await api.adminUpdateBillingSettings({ ...billingForm }); setBilling(r); setBillingForm(r); setMessage('规则已更新') }) }
  async function savePrompt(item: PromptPlazaItem | null, payload: AdminPromptPlazaPayload) { await run('prompt-save', async () => { item ? await api.adminUpdatePromptPlaza(item.id, payload) : await api.adminCreatePromptPlaza(payload); setModal({ type: 'none' }); await loadPrompts(promptStatus); setMessage(item ? '提示词已更新' : '提示词已创建') }) }
  async function reviewPrompt(item: PromptPlazaItem, status: 'approved' | 'rejected', reason = '') { await run(`prompt-review-${item.id}`, async () => { await api.adminReviewPromptPlaza(item.id, status, reason); setModal({ type: 'none' }); await loadPrompts(promptStatus); setMessage(status === 'approved' ? '提示词已通过' : '提示词已驳回') }) }
  async function deletePrompt(item: PromptPlazaItem, reason = '') { await run(`prompt-delete-${item.id}`, async () => { await api.adminDeletePromptPlaza(item.id, reason); setModal({ type: 'none' }); await loadPrompts(promptStatus); setMessage('提示词已删除') }) }
  async function savePackage(item: AdminPackageItem | null, payload: AdminPackagePayload) { await run('package-save', async () => { item ? await api.adminUpdatePackage(item.id, payload) : await api.adminCreatePackage(payload); setModal({ type: 'none' }); await loadPackages(packagePage); setMessage(item ? '商品套餐已更新' : '商品套餐已创建') }) }
  async function saveProduct(item: ProductItem | null, payload: AdminProductPayload) { await run('product-save', async () => { item ? await api.adminUpdateProduct(item.id, payload) : await api.adminCreateProduct(payload); setModal({ type: 'none' }); await loadProducts(productPage); setMessage(item ? '商品已更新' : '商品已创建') }) }
  async function saveCategory(item: ProductCategoryItem | null, payload: AdminProductCategoryPayload) { await run('category-save', async () => { item ? await api.adminUpdateProductCategory(item.id, payload) : await api.adminCreateProductCategory(payload); setModal({ type: 'none' }); await loadProducts(productPage); setMessage(item ? '分类已更新' : '分类已创建') }) }
  async function saveProviderKey(item: AdminProviderKeyItem | null, payload: AdminProviderKeyPayload) { await run('key-save', async () => { item ? await api.adminUpdateProviderKey(item.id, payload) : await api.adminCreateProviderKey(payload); setModal({ type: 'none' }); await loadProviderKeys(providerPage); setMessage(item ? 'API Key 已更新' : 'API Key 已创建') }) }
  async function saveAnnouncement(item: AnnouncementItem | null, payload: AdminAnnouncementPayload) { await run('announcement-save', async () => { item ? await api.adminUpdateAnnouncement(item.id, payload) : await api.adminCreateAnnouncement(payload); setModal({ type: 'none' }); await loadAnnouncements(announcementPage); setMessage(item ? '公告已更新' : '公告已创建') }) }
  async function openUserLogs(user: AdminUserItem, page = 1) { setModal({ type: 'userLogs', user, response: null, page, loading: true, error: '' }); try { const r = await api.adminListUserActivity(user.id, page, PAGE_SIZE); setModal({ type: 'userLogs', user, response: r, page, loading: false, error: '' }) } catch (e) { setModal({ type: 'userLogs', user, response: null, page, loading: false, error: errorText(e, '日志加载失败') }) } }
  function askConfirm(data: ConfirmState) { setModal({ type: 'confirm', data }) }

  if (!currentUser || !isAdmin) return <div className="flex min-h-screen items-center justify-center bg-slate-50 p-6 text-slate-700"><TopNotice message={message} onDismiss={() => setMessage('')} /><div className="rounded-2xl bg-white p-5 shadow-sm">正在进入管理员后台...</div></div>

  return <div className="admin-shell min-h-screen bg-gradient-to-br from-[#f7f8ff] via-white to-emerald-50 text-slate-900"><TopNotice message={message} onDismiss={() => setMessage('')} /><div className="admin-layout flex min-h-screen"><Sidebar active={activeTab} open={mobileNav} user={currentUser} pending={prompts?.pending ?? 0} onClose={() => setMobileNav(false)} onNav={(tab) => { setActiveTab(tab); setMobileNav(false) }} /><div className="admin-content min-w-0 flex-1"><header className="admin-header sticky top-0 z-30 border-b border-slate-100 bg-white/85 px-4 py-3 backdrop-blur"><div className="flex flex-wrap items-center justify-between gap-3"><div className="flex items-center gap-3"><button className="icon-btn lg:hidden" onClick={() => setMobileNav(true)}><Menu className="h-4 w-4" /></button><div><div className="text-sm text-slate-500">Administrator</div><h1 className="text-xl font-bold">{tabs.find(t => t.id === activeTab)?.label}</h1></div></div><div className="admin-header-actions flex flex-wrap gap-2"><Button onClick={refresh} loading={busy === 'refresh'}><RefreshCcw className="h-4 w-4" /><span className="admin-btn-label">刷新</span></Button><Button onClick={toggleGeneration} loading={busy === 'toggle-generation'}><Sparkles className="h-4 w-4" /><span className="admin-btn-label">{billingForm.imageGenerationEnabled ? '关闭生图' : '开启生图'}</span></Button><Button onClick={onBackToApp}><ArrowLeft className="h-4 w-4" /><span className="admin-btn-label">用户端</span></Button><Button onClick={onLogout}><LogOut className="h-4 w-4" /><span className="admin-btn-label">退出</span></Button></div></div></header><main className="admin-main p-3 sm:p-5 lg:p-6">
    {activeTab === 'overview' && <Overview stats={stats} />}
    {activeTab === 'tasks' && <Tasks data={tasks} status={taskStatus} keyword={taskKeyword} setStatus={setTaskStatus} setKeyword={setTaskKeyword} search={() => loadTasks(1, taskStatus, taskKeyword)} page={taskPage} setPage={(p) => loadTasks(p, taskStatus, taskKeyword)} open={(item) => setModal({ type: 'task', item })} />}
    {activeTab === 'redeemCodes' && <Redeem data={redeem} status={redeemStatus} keyword={redeemKeyword} form={redeemForm} generated={generatedCodes} setStatus={setRedeemStatus} setKeyword={setRedeemKeyword} setForm={setRedeemForm} create={createRedeem} page={redeemPage} setPage={(p) => loadRedeem(p, redeemStatus, redeemKeyword)} search={() => loadRedeem(1, redeemStatus, redeemKeyword)} detail={(item) => setModal({ type: 'redeemCode', item })} del={(item) => askConfirm({ title: '删除兑换码', content: `确认删除兑换码 ${item.code}？`, danger: true, confirmText: '删除', onConfirm: async () => { await api.adminDeleteRedeemCode(item.id); setModal({ type: 'none' }); await loadRedeem(redeemPage) } })} />}
    {activeTab === 'promptPlaza' && <PromptAdmin data={prompts} status={promptStatus} busy={busy} setStatus={(s) => loadPrompts(s)} create={() => setModal({ type: 'prompt', item: null })} edit={(item) => setModal({ type: 'prompt', item })} approve={(item) => reviewPrompt(item, 'approved')} reject={(item) => setModal({ type: 'promptReason', action: 'reject', item, reason: '' })} del={(item) => setModal({ type: 'promptReason', action: 'delete', item, reason: '' })} />}
    {activeTab === 'packages' && <Packages data={packages} page={packagePage} setPage={loadPackages} create={() => setModal({ type: 'package', item: null })} edit={(item) => setModal({ type: 'package', item })} del={(item) => askConfirm({ title: '删除商品套餐', content: `确认删除「${item.name}」？`, danger: true, confirmText: '删除', onConfirm: async () => { await api.adminDeletePackage(item.id); setModal({ type: 'none' }); await loadPackages(packagePage) } })} />}
    {activeTab === 'products' && <Products data={products} categories={categories?.items ?? []} keyword={productKeyword} category={productCategory} enabled={productEnabled} setKeyword={setProductKeyword} setCategory={setProductCategory} setEnabled={setProductEnabled} search={() => loadProducts(1, productKeyword, productCategory, productEnabled)} page={productPage} setPage={(p) => loadProducts(p, productKeyword, productCategory, productEnabled)} create={() => setModal({ type: 'product', item: null })} edit={(item) => setModal({ type: 'product', item })} del={(item) => askConfirm({ title: '删除商品', content: `确认删除商品「${item.name}」？`, danger: true, confirmText: '删除', onConfirm: async () => { await api.adminDeleteProduct(item.id); setModal({ type: 'none' }); await loadProducts(productPage) } })} createCat={() => setModal({ type: 'category', item: null })} editCat={(item) => setModal({ type: 'category', item })} delCat={(item) => askConfirm({ title: '删除分类', content: `确认删除分类「${item.name}」？如已绑定商品，后端会阻止删除。`, danger: true, confirmText: '删除', onConfirm: async () => { await api.adminDeleteProductCategory(item.id); setModal({ type: 'none' }); await loadProducts(productPage) } })} />}
    {activeTab === 'providerKeys' && <ProviderKeys data={providerKeys} busy={busy} page={providerPage} setPage={loadProviderKeys} create={() => setModal({ type: 'providerKey', item: null })} edit={(item) => setModal({ type: 'providerKey', item })} useKey={(item) => run(`use-key-${item.id}`, async () => { await api.adminUseProviderKey(item.id); await loadProviderKeys(providerPage); setMessage('已切换使用中的 API Key') })} del={(item) => askConfirm({ title: '删除 API Key', content: `确认删除「${item.name}」？`, danger: true, confirmText: '删除', onConfirm: async () => { await api.adminDeleteProviderKey(item.id); setModal({ type: 'none' }); await loadProviderKeys(providerPage) } })} />}
    {activeTab === 'announcements' && <Announcements data={announcements} page={announcementPage} setPage={loadAnnouncements} create={() => setModal({ type: 'announcement', item: null })} edit={(item) => setModal({ type: 'announcement', item })} del={(item) => askConfirm({ title: '删除公告', content: `确认删除公告「${item.title}」？`, danger: true, confirmText: '删除', onConfirm: async () => { await api.adminDeleteAnnouncement(item.id); setModal({ type: 'none' }); await loadAnnouncements(announcementPage) } })} />}
    {activeTab === 'rules' && <Rules form={billingForm} setForm={setBillingForm} save={saveBilling} busy={busy === 'rules-save'} billing={billing} />}
    {activeTab === 'users' && <UsersPage data={users} keyword={userKeyword} status={userStatus} setKeyword={setUserKeyword} setStatus={setUserStatus} search={() => loadUsers(1, userKeyword, userStatus)} page={userPage} setPage={(p) => loadUsers(p, userKeyword, userStatus)} toggle={(u) => run(`user-${u.id}`, async () => { await api.adminUpdateUserStatus(u.id, u.status === 1 ? 2 : 1); await loadUsers(userPage); setMessage(u.status === 1 ? '用户已禁用' : '用户已启用') })} points={(u) => setModal({ type: 'points', user: u, delta: '10', remark: '后台调整' })} logs={openUserLogs} />}
    {activeTab === 'cleanup' && <Cleanup form={cleanupForm} setForm={setCleanupForm} result={cleanupResult} preview={() => run('cleanup-preview', async () => { setCleanupResult(await api.adminPreviewDataCleanup(cleanupForm)); setMessage('清理预览完成') })} execute={() => askConfirm({ title: '执行数据清理', content: '这是危险操作，请确认已经预览结果，并确定要执行清理。', danger: true, confirmText: '执行清理', onConfirm: async () => { await run('cleanup-execute', async () => { setCleanupResult(await api.adminExecuteDataCleanup(cleanupForm)); setModal({ type: 'none' }); setMessage('数据清理已执行') }) } })} busy={busy} />}
    {activeTab === 'logs' && <Logs data={logs} targetType={logTargetType} targetId={logTargetId} setTargetType={setLogTargetType} setTargetId={setLogTargetId} search={() => loadLogs(1, logTargetType, logTargetId)} page={logPage} setPage={(p) => loadLogs(p, logTargetType, logTargetId)} open={(item) => setModal({ type: 'log', item })} />}
  </main></div></div><AllModals state={modal} setState={setModal} busy={busy} categories={categories?.items ?? []} onSavePrompt={savePrompt} onReviewPrompt={reviewPrompt} onDeletePrompt={deletePrompt} onSavePackage={savePackage} onSaveProduct={saveProduct} onSaveCategory={saveCategory} onSaveProviderKey={saveProviderKey} onSaveAnnouncement={saveAnnouncement} onSavePoints={async () => { if (modal.type !== 'points') return; await run(`points-${modal.user.id}`, async () => { await api.adminAdjustPoints(modal.user.id, Math.trunc(Number(modal.delta) || 0), modal.remark || '后台调整'); setModal({ type: 'none' }); await loadUsers(userPage); setMessage('积分已调整') }) }} onOpenUserLogs={openUserLogs} /></div>
}

function Sidebar({ active, open, user, pending, onNav, onClose }: { active: Tab; open: boolean; user: UserInfo; pending: number; onNav: (tab: Tab) => void; onClose: () => void }) {
  return <aside className={`admin-sidebar fixed inset-y-0 left-0 z-40 flex w-[284px] max-w-[86vw] flex-col border-r border-slate-100 bg-[#f0f2ff] p-4 shadow-xl transition-transform lg:sticky lg:translate-x-0 ${open ? 'translate-x-0' : '-translate-x-full'}`}><div className="flex items-center justify-between"><div className="flex items-center gap-2"><Sparkles className="h-6 w-6 text-emerald-500" /><div><div className="text-lg font-bold">管理员后台</div><div className="text-xs text-slate-500">兑换码与任务运营</div></div></div><button className="icon-btn lg:hidden" onClick={onClose}><X className="h-4 w-4" /></button></div><nav className="admin-nav mt-7 space-y-1 overflow-y-auto pr-1">{tabs.map(tab => <button key={tab.id} type="button" onClick={() => onNav(tab.id)} className={`flex w-full items-center gap-3 rounded-2xl px-4 py-2.5 text-left text-sm font-medium transition ${active === tab.id ? 'bg-white text-indigo-600 shadow-sm' : 'text-slate-600 hover:bg-white/70'}`}>{tab.icon}<span className="flex-1">{tab.label}</span>{tab.id === 'promptPlaza' && pending > 0 ? <span className="h-2.5 w-2.5 rounded-full bg-rose-500" /> : null}</button>)}</nav><div className="mt-auto rounded-3xl bg-white/80 p-4 shadow-sm"><div className="font-semibold">{user.username}</div><div className="truncate text-sm text-slate-500">{user.email}</div></div></aside>
}

function Overview({ stats }: { stats: AdminStatsResponse | null }) {
  const [range, setRange] = useState<'24h' | '3d' | '7d'>('24h')
  const trend = range === '24h' ? stats?.taskTrend24h : range === '3d' ? stats?.taskTrend3d : stats?.taskTrend7d
  const points = normalizedTrendPoints(trend, stats)
  const max = Math.max(1, ...points.map(p => p.success + p.failed))
  const redeemAll = Number(stats?.redeemCodesActive ?? 0) + Number(stats?.redeemCodesUsed ?? 0) + Number(stats?.redeemCodesDisabled ?? 0)
  return <div className="overview-page space-y-5"><div className="overview-metrics grid gap-4 md:grid-cols-2 xl:grid-cols-3"><OverviewCard icon={<Activity className="h-5 w-5" />} label="今日消耗" value={`${num(stats?.tasksPointsConsumedToday)} 积分 + ${num(stats?.tasksCreditsConsumedToday)} 次数`} sub="统计今日 0 点后的任务扣减" aside={`${num(stats?.tasksImagesGeneratedToday)}张`} /><OverviewCard icon={<Users className="h-5 w-5" />} label="今日活跃用户" value={`${num(stats?.tasksActiveUsersToday)} 人`} sub="今日使用生图功能的用户数量" /><OverviewCard icon={<Users className="h-5 w-5" />} label="今日新增用户" value={`${num(stats?.usersCreatedToday)} 人`} sub="今日新注册的普通用户数量" aside={`总用户 ${num(stats?.usersTotal)}`} /><OverviewCard icon={<Timer className="h-5 w-5" />} label="昨日消耗" value={`${num(stats?.tasksPointsConsumedYesterday)} 积分 + ${num(stats?.tasksCreditsConsumedYesterday)} 次数`} sub="统计昨日 0 点至今日 0 点的任务扣减" aside={`${num(stats?.tasksImagesGeneratedYesterday)}张`} /><OverviewCard icon={<Sparkles className="h-5 w-5" />} label="资源池" value={`${num(stats?.pointsBalanceTotal)} 积分 + ${num(stats?.creditsBalanceTotal)} 次数`} sub="当前用户积分与次数余额汇总" /><OverviewCard icon={<TrendingUp className="h-5 w-5" />} label="历史消耗" value={`${num(stats?.tasksPointsConsumedTotal)} 积分 + ${num(stats?.tasksCreditsConsumedTotal)} 次数`} sub="按历史任务实际扣减累计" aside={`${num(stats?.tasksImagesGeneratedTotal)}张`} /></div><section className="overview-chart rounded-[30px] border border-white/80 bg-white/90 p-4 shadow-sm sm:p-5"><div className="flex flex-wrap items-start justify-between gap-3"><div><h2 className="text-lg font-bold">任务次数趋势</h2><p className="mt-1 text-sm text-slate-500">成功与失败次数趋势，柱顶显示数值。</p></div><div className="flex rounded-full bg-white p-1 shadow-lg shadow-indigo-100/70">{(['24h','3d','7d'] as const).map(v => <button key={v} onClick={() => setRange(v)} className={`rounded-full px-4 py-2 text-sm font-semibold ${range === v ? 'bg-indigo-500 text-white shadow' : 'text-slate-500'}`}>{v}</button>)}</div></div><div className="mt-10 flex items-center justify-between text-sm"><div className="flex gap-4"><span className="inline-flex items-center gap-2"><span className="h-2 w-5 rounded-full bg-emerald-400" />成功</span><span className="inline-flex items-center gap-2"><span className="h-2 w-5 rounded-full bg-rose-400" />失败</span></div><span className="text-slate-500">柱顶显示数值</span></div><div className="overview-bars mt-8 overflow-x-auto rounded-[26px] bg-white p-5">{points.length ? <div className="flex h-72 min-w-[760px] items-end gap-2 border-b border-slate-200 px-2">{points.map((p, index) => <div key={`${p.label}-${index}`} className="flex min-w-8 flex-1 flex-col items-center gap-2"><div className="flex h-52 items-end gap-1 border-b-0 border-slate-100"><TrendBar value={p.success} max={max} color="bg-emerald-400" /><TrendBar value={p.failed} max={max} color="bg-rose-400" /></div><span className="max-w-16 truncate text-xs text-slate-500">{p.label}</span></div>)}</div> : <Empty text="暂无趋势数据" />}</div></section><div className="grid gap-4 md:grid-cols-3"><RedeemSummary label="未使用兑换码" value={num(stats?.redeemCodesActive)} sub={`${num(stats?.redeemCodeActivePointsTotal)} 积分`} /><RedeemSummary label="已使用兑换码" value={num(stats?.redeemCodesUsed)} sub={`${num(stats?.redeemCodeUsedPointsTotal)} 积分`} /><RedeemSummary label="全部兑换码" value={num(redeemAll)} sub={`${num(stats?.redeemCodePointsTotal)} 积分`} /></div></div>
}

function Tasks(p: { data: AdminListImageTasksResponse | null; status: string; keyword: string; setStatus: (v: string) => void; setKeyword: (v: string) => void; search: () => void; page: number; setPage: (v: number) => void; open: (item: AdminImageTaskItem) => void }) {
  return <Panel title="任务" actions={<Filter keyword={p.keyword} setKeyword={p.setKeyword} placeholder="搜索任务ID / 用户 / 提示词" select={p.status} setSelect={p.setStatus} options={[["","全部状态"],["generating","生成中"],["success","成功"],["failed","失败"]]} search={p.search} />}><ResponsiveTable headers={['任务','用户名','状态','消耗','来源','耗时','创建时间','操作']} rows={(p.data?.items ?? []).map(item => [<div className="min-w-[220px]"><div className="font-mono text-sm font-bold text-slate-800">{taskNo(item.id)}</div><div className="mt-1 max-w-[340px] truncate text-xs text-slate-500">{clip(item.prompt, 64) || '无提示词'}</div></div>, <div className="min-w-[88px]">{item.username || item.userId || '-'}</div>, <StatusPill value={item.status} />, <div className="text-sm">{item.costPoints}积分 / {item.costCredits}次</div>, <span className="text-slate-500">{taskSource(item)}</span>, <span className="font-mono text-xs">{durationText(item.createdAt, item.updatedAt)}</span>, date(item.createdAt), <Button small onClick={() => p.open(item)}><Eye className="h-3.5 w-3.5" />详情</Button>])} empty="暂无任务" /><Pagination page={p.page} total={p.data?.total ?? 0} pageSize={p.data?.pageSize ?? PAGE_SIZE} onChange={p.setPage} /></Panel>
}

function Redeem(p: { data: AdminListRedeemCodesResponse | null; status: string; keyword: string; form: any; generated: AdminRedeemCodeItem[]; setStatus: (v: string) => void; setKeyword: (v: string) => void; setForm: (v: any) => void; create: () => void; page: number; setPage: (v: number) => void; search: () => void; detail: (item: AdminRedeemCodeItem) => void; del: (item: AdminRedeemCodeItem) => void }) {
  return <div className="redeem-layout grid gap-5 xl:grid-cols-[360px,minmax(0,1fr)]"><Panel title="生成兑换码"><div className="space-y-3"><LabeledInput label="生成数量" value={p.form.count} onChange={v => p.setForm({ ...p.form, count: v })} /><LabeledInput label="每张积分" value={p.form.pointsAmount} onChange={v => p.setForm({ ...p.form, pointsAmount: v })} /><LabeledInput label="每张福利次数" value={p.form.creditsAmount} onChange={v => p.setForm({ ...p.form, creditsAmount: v })} /><LabeledInput label="前缀" value={p.form.prefix} onChange={v => p.setForm({ ...p.form, prefix: v })} /><TextAreaInput label="备注" value={p.form.note} onChange={v => p.setForm({ ...p.form, note: v })} /><Button primary onClick={p.create}><KeyRound className="h-4 w-4" />生成兑换码</Button>{p.generated.length ? <div className="rounded-2xl bg-slate-50 p-3 text-xs"><div className="mb-2 flex items-center justify-between gap-2"><span className="font-semibold">本次生成 {p.generated.length} 个</span><Button small onClick={() => copyText(p.generated.map(c => c.code).join('\n'))}>一键复制</Button></div><div className="max-h-48 space-y-1 overflow-auto rounded-xl bg-white/70 p-2">{p.generated.map(c => <div key={c.id} className="font-mono text-slate-700">{c.code}</div>)}</div></div> : null}</div></Panel><Panel title="兑换码" actions={<Filter keyword={p.keyword} setKeyword={p.setKeyword} placeholder="搜索兑换码 / 批次 / 备注" select={p.status} setSelect={p.setStatus} options={[["","全部状态"],["active","未使用"],["used","已使用"],["disabled","已禁用"],["expired","已过期"]]} search={p.search} />}><ResponsiveTable headers={['兑换码','积分','福利次数','状态','使用者','操作']} rows={(p.data?.items ?? []).map(item => [<Mono>{item.code}</Mono>, item.pointsAmount, item.creditsAmount, <StatusPill value={item.status} />, item.usedBy || '-', <div className="flex flex-wrap gap-2"><Button small onClick={() => p.detail(item)}>详情</Button><Button small onClick={() => copyText(item.code)}>复制</Button><Button small danger onClick={() => p.del(item)}>删除</Button></div>])} empty="暂无兑换码" /><Pagination page={p.page} total={p.data?.total ?? 0} pageSize={p.data?.pageSize ?? PAGE_SIZE} onChange={p.setPage} /></Panel></div>
}

function PromptAdmin(p: { data: AdminListPromptPlazaResponse | null; status: 'approved' | 'pending' | 'rejected'; busy: Busy; setStatus: (v: 'approved' | 'pending' | 'rejected') => void; create: () => void; edit: (item: PromptPlazaItem) => void; approve: (item: PromptPlazaItem) => void; reject: (item: PromptPlazaItem) => void; del: (item: PromptPlazaItem) => void }) {
  return <Panel title="提示词广场" actions={<div className="flex flex-wrap gap-2"><Select value={p.status} onChange={v => p.setStatus(v as any)} options={[["approved","已通过"],["pending","待审核"],["rejected","已驳回"]]} /><Button primary onClick={p.create}>新建提示词</Button></div>}><ResponsiveTable headers={['预览','标题','标签','上传人','状态','收藏','创建时间','操作']} rows={(p.data?.items ?? []).map(item => [item.previewThumbUrl || item.previewUrl ? <img src={item.previewThumbUrl || item.previewUrl} alt={item.title} className="h-14 w-14 rounded-2xl object-cover" /> : '-', <div><div className="font-semibold">{item.title}</div><div className="max-w-xl truncate text-xs text-slate-500">{item.prompt}</div></div>, <div className="flex flex-wrap gap-1">{item.tags?.map(t => <span key={t} className="rounded-full bg-amber-100 px-2 py-1 text-xs text-amber-700">#{t}</span>)}</div>, item.author || '-', <StatusPill value={item.status} />, item.favorites, date(item.createdAt), <div className="flex flex-wrap gap-2"><Button small onClick={() => p.edit(item)}>编辑</Button>{item.status !== 'approved' ? <Button small onClick={() => p.approve(item)} disabled={p.busy === `prompt-review-${item.id}`}>通过</Button> : null}<Button small danger onClick={() => p.reject(item)}>驳回</Button><Button small danger onClick={() => p.del(item)}>删除</Button></div>])} empty="暂无提示词" /></Panel>
}

function Packages(p: { data: AdminListPackagesResponse | null; page: number; setPage: (v: number) => void; create: () => void; edit: (item: AdminPackageItem) => void; del: (item: AdminPackageItem) => void }) {
  return <Panel title="商品套餐配置" desc="维护前台兑换码/充值商品展示。" actions={<Button primary onClick={p.create}>新建商品套餐</Button>}><ResponsiveTable headers={['编码','名称','类型','福利次数','积分/价格','状态','排序','更新时间','操作']} rows={(p.data?.items ?? []).map(item => [item.code, item.name, item.packageType, item.creditsAmount, `${item.pointsCost}积分 / ${money(item.amountCents)}`, <StatusPill value={item.enabled ? '启用' : '停用'} />, item.sortOrder, date(item.updatedAt), <div className="flex gap-2"><Button small onClick={() => p.edit(item)}>编辑</Button><Button small danger onClick={() => p.del(item)}>删除</Button></div>])} empty="暂无商品套餐" /><Pagination page={p.page} total={p.data?.total ?? 0} pageSize={p.data?.pageSize ?? PAGE_SIZE} onChange={p.setPage} /></Panel>
}

function Products(p: { data: AdminListProductsResponse | null; categories: ProductCategoryItem[]; keyword: string; category: string; enabled: string; setKeyword: (v: string) => void; setCategory: (v: string) => void; setEnabled: (v: string) => void; search: () => void; page: number; setPage: (v: number) => void; create: () => void; edit: (item: ProductItem) => void; del: (item: ProductItem) => void; createCat: () => void; editCat: (item: ProductCategoryItem) => void; delCat: (item: ProductCategoryItem) => void }) {
  const catOptions: Array<[string, string]> = [['','全部分类'], ...p.categories.map(c => [c.id, c.name] as [string,string])]
  return <div className="space-y-5"><Panel title="商品列表" actions={<div className="flex flex-wrap gap-2"><Filter keyword={p.keyword} setKeyword={p.setKeyword} placeholder="搜索商品 / 说明 / 分类" select={p.category} setSelect={p.setCategory} options={catOptions} search={p.search} /><Select value={p.enabled} onChange={p.setEnabled} options={[["","全部状态"],["true","启用"],["false","停用"]]} /><Button primary onClick={p.create}>新建商品</Button></div>}><ResponsiveTable headers={['图片','名称','价格','分类','状态','排序','更新时间','操作']} rows={(p.data?.items ?? []).map(item => [item.previewThumbUrl || item.previewUrl ? <img src={item.previewThumbUrl || item.previewUrl} alt={item.name} className="h-14 w-14 rounded-2xl object-cover" /> : '-', <div><div className="font-semibold">{item.name}</div><div className="max-w-md truncate text-xs text-slate-500">{item.shortDescription || item.description || '-'}</div></div>, money(item.amountCents), <div className="flex flex-wrap gap-1">{item.categories?.length ? item.categories.map(c => <span key={c.id} className="rounded-full bg-indigo-50 px-2 py-1 text-xs text-indigo-600">{c.name}</span>) : '未分类'}</div>, <StatusPill value={item.enabled ? '启用' : '停用'} />, item.sortOrder, date(item.updatedAt), <div className="flex gap-2"><Button small onClick={() => p.edit(item)}>编辑</Button><Button small danger onClick={() => p.del(item)}>删除</Button></div>])} empty="暂无商品" /><Pagination page={p.page} total={p.data?.total ?? 0} pageSize={p.data?.pageSize ?? PAGE_SIZE} onChange={p.setPage} /></Panel><Panel title="商品分类管理" actions={<Button primary onClick={p.createCat}>新建分类</Button>}><ResponsiveTable headers={['名称','描述','排序','状态','操作']} rows={p.categories.map(c => [c.name, c.description || '-', c.sortOrder, <StatusPill value={c.enabled ? '启用' : '停用'} />, <div className="flex gap-2"><Button small onClick={() => p.editCat(c)}>编辑</Button><Button small danger onClick={() => p.delCat(c)}>删除</Button></div>])} empty="暂无分类" /></Panel></div>
}

function ProviderKeys(p: { data: AdminListProviderKeysResponse | null; busy: Busy; page: number; setPage: (v: number) => void; create: () => void; edit: (item: AdminProviderKeyItem) => void; useKey: (item: AdminProviderKeyItem) => void; del: (item: AdminProviderKeyItem) => void }) {
  return <Panel title="API Key 配置" desc="维护生图供应商 API Key、优先级和自动切换策略。" actions={<Button primary onClick={p.create}>新建 API Key</Button>}><ResponsiveTable headers={['名称','状态','使用状态','优先级','自动切换','调用统计','最近使用','最近错误','操作']} rows={(p.data?.items ?? []).map(item => [<div><div className="font-semibold">{item.name}</div><div className="text-xs text-slate-500">{item.provider} / {item.apiKeyMasked}</div></div>, <StatusPill value={item.status} />, <StatusPill value={item.isUsing ? '使用中' : '未使用'} />, item.priority, item.autoSwitchEnabled ? '开启' : '关闭', `成功 ${item.successCount} / 失败 ${item.failureCount}`, date(item.lastUsedAt), clip(item.lastError, 32) || '-', <div className="flex flex-wrap gap-2"><Button small disabled={item.isUsing || p.busy === `use-key-${item.id}`} onClick={() => p.useKey(item)}>{item.isUsing ? '使用中' : '使用'}</Button><Button small onClick={() => p.edit(item)}>编辑</Button><Button small danger onClick={() => p.del(item)}>删除</Button></div>])} empty="暂无 API Key" /><Pagination page={p.page} total={p.data?.total ?? 0} pageSize={p.data?.pageSize ?? 20} onChange={p.setPage} /></Panel>
}

function Announcements(p: { data: AdminListAnnouncementsResponse | null; page: number; setPage: (v: number) => void; create: () => void; edit: (item: AnnouncementItem) => void; del: (item: AnnouncementItem) => void }) {
  return <Panel title="公告管理" desc="维护用户端公告、置顶提示和个人公告。" actions={<div className="flex gap-2"><Button>公告记录</Button><Button primary onClick={p.create}>新建公告</Button></div>}><ResponsiveTable headers={['标题','类型','目标用户','状态','排序','更新时间','操作']} rows={(p.data?.items ?? []).map(item => [<div><div className="font-semibold">{item.title}</div><div className="max-w-xl truncate text-xs text-slate-500">{item.content}</div></div>, kindLabel(item.kind), item.targetUserId || '-', <StatusPill value={item.enabled ? '启用' : '停用'} />, item.sortOrder, date(item.updatedAt), <div className="flex gap-2"><Button small onClick={() => p.edit(item)}>编辑</Button><Button small danger onClick={() => p.del(item)}>删除</Button></div>])} empty="暂无公告" /><Pagination page={p.page} total={p.data?.total ?? 0} pageSize={p.data?.pageSize ?? PAGE_SIZE} onChange={p.setPage} /></Panel>
}

function Rules({ form, setForm, save, busy, billing }: { form: UpdateBillingSettingsRequest; setForm: (v: UpdateBillingSettingsRequest) => void; save: () => void; busy: boolean; billing: BillingSettingsResponse | null }) {
  const set = (key: keyof UpdateBillingSettingsRequest, value: number | boolean) => setForm({ ...form, [key]: value })
  return <Panel title="积分与奖励规则" desc="配置生成消耗、签到、邀请和注册赠送规则。" actions={<Button primary onClick={save} loading={busy}><Save className="h-4 w-4" />保存规则</Button>}><div className="grid gap-4 md:grid-cols-2 xl:grid-cols-3"><NumberField label="生图积分成本" value={form.imageGeneratePointCost} onChange={v => set('imageGeneratePointCost', v)} /><Toggle label="生图开关" checked={form.imageGenerationEnabled} onChange={v => set('imageGenerationEnabled', v)} /><Toggle label="邮件通知开关" checked={form.emailNotifyEnabled} onChange={v => set('emailNotifyEnabled', v)} /><NumberField label="邮件通知积分消耗" value={form.emailNotifyPointCost} onChange={v => set('emailNotifyPointCost', v)} /><NumberField label="签到奖励积分" value={form.checkinPoints} onChange={v => set('checkinPoints', v)} /><NumberField label="签到奖励福利次数" value={form.checkinCredits} onChange={v => set('checkinCredits', v)} /><NumberField label="邀请奖励积分" value={form.inviteRewardPoints} onChange={v => set('inviteRewardPoints', v)} /><NumberField label="邀请奖励福利次数" value={form.inviteRewardCredits} onChange={v => set('inviteRewardCredits', v)} /><NumberField label="注册赠送福利次数" value={form.registrationGiftCredits} onChange={v => set('registrationGiftCredits', v)} /></div><div className="mt-6 text-sm text-slate-500">当前生图状态：{billing?.imageGenerationEnabled ? '启用' : '停用'}</div></Panel>
}

function UsersPage(p: { data: AdminListUsersResponse | null; keyword: string; status: string; setKeyword: (v: string) => void; setStatus: (v: string) => void; search: () => void; page: number; setPage: (v: number) => void; toggle: (item: AdminUserItem) => void; points: (item: AdminUserItem) => void; logs: (item: AdminUserItem) => void }) {
  return <Panel title="用户" actions={<Filter keyword={p.keyword} setKeyword={p.setKeyword} placeholder="搜索用户ID / 用户名 / 邮箱" select={p.status} setSelect={p.setStatus} options={[["","全部状态"],["1","启用"],["2","禁用"]]} search={p.search} />}><ResponsiveTable headers={['ID','用户','角色','状态','积分','免费次数','付费次数','注册时间','最近登录','操作']} rows={(p.data?.items ?? []).map(item => [<Mono>{item.id}</Mono>, <div className="min-w-[150px]"><div className="font-semibold text-slate-900">{item.username}</div><div className="mt-0.5 text-xs text-slate-500">{item.email}</div></div>, item.role, <StatusPill value={item.status === 1 ? '启用' : '禁用'} />, item.pointsBalance, item.freeCredits ?? '-', item.paidCredits ?? '-', date(item.createdAt), date(item.lastLoginAt), <div className="flex flex-nowrap gap-2"><Button small onClick={() => p.points(item)}>调积分</Button><Button small onClick={() => p.logs(item)}>日志</Button><Button small danger={item.status === 1} onClick={() => p.toggle(item)}>{item.status === 1 ? '禁用' : '启用'}</Button></div>])} empty="暂无用户" /><Pagination page={p.page} total={p.data?.total ?? 0} pageSize={p.data?.pageSize ?? PAGE_SIZE} onChange={p.setPage} /></Panel>
}

function Cleanup(p: { form: DataCleanupRequest; setForm: (v: DataCleanupRequest) => void; result: DataCleanupResponse | null; preview: () => void; execute: () => void; busy: Busy }) {
  const typeOptions: Array<[DataCleanupRequest['type'], string]> = [
    ['failed_image_tasks', '失败生图任务记录'],
    ['generated_images', '生成图原图和缩略图'],
    ['user_reference_images', '用户上传参考图和缩略图'],
    ['temporary_keys', '过期验证码 / Redis Key'],
    ['action_logs', '操作日志'],
    ['email_failure_notifications', '邮件失败通知记录'],
    ['successful_image_tasks', '成功生图任务记录'],
    ['balance_logs', '用户积分 / 福利次数流水'],
    ['user_activity', '用户活动记录'],
    ['announcement_reads', '公告已读记录'],
    ['rejected_prompt_plaza', '提示词广场被拒绝记录'],
  ]
  const keepDays = p.form.keepDays ?? 30
  return <Panel title="数据清理" desc="危险操作，请先预览空间和记录数量，再执行清理。生成图清理只处理任务生成图和缩略图，不会删除用户参考图。">
    <div className="grid gap-4 md:grid-cols-2">
      <label className="field-label">清理类型<Select value={p.form.type} onChange={v => p.setForm({ ...p.form, type: v as DataCleanupRequest['type'] })} options={typeOptions} /></label>
      <NumberField label="保留最近多少天内记录" value={keepDays} onChange={keepDays => p.setForm({ ...p.form, mode: 'retention', keepDays: Math.max(0, Math.trunc(keepDays)) })} />
    </div>
    <div className="mt-3 rounded-2xl border border-amber-100 bg-amber-50 px-4 py-3 text-sm leading-6 text-amber-800">
      将清理 <b>{cleanupTypeLabel(p.form.type)}</b> 中早于保留天数的数据；Redis 临时 Key 没有业务创建时间，会清理当前残留的验证码、邮箱验证码和限流 Key。
    </div>
    <div className="mt-4 flex gap-2"><Button onClick={p.preview} loading={p.busy === 'cleanup-preview'}>空间预览</Button><Button danger onClick={p.execute} loading={p.busy === 'cleanup-execute'}>执行清理</Button></div>
    {p.result ? <div className="mt-5 rounded-3xl bg-slate-50 p-4"><div className="mb-3 font-semibold">{p.result.dryRun ? '预览结果' : '执行结果'}：{p.result.typeLabel || cleanupTypeLabel(p.result.type)}，记录 {p.result.recordsRemoved}，文件 {p.result.filesRemoved}，预计释放 {formatBytes(p.result.bytesRemoved)}</div><ResponsiveTable headers={['清理对象','数量','空间','说明']} rows={p.result.tableStats.map(s => [s.table, s.count, formatBytes(s.bytes ?? 0), s.note || '-'])} empty="暂无清理项" /></div> : null}
  </Panel>
}

function cleanupTypeLabel(type: DataCleanupRequest['type']) {
  const labels: Record<DataCleanupRequest['type'], string> = {
    failed_image_tasks: '失败生图任务记录',
    generated_images: '生成图原图和缩略图',
    user_reference_images: '用户上传参考图和缩略图',
    temporary_keys: '过期验证码 / Redis Key',
    action_logs: '操作日志',
    email_failure_notifications: '邮件失败通知记录',
    successful_image_tasks: '成功生图任务记录',
    balance_logs: '用户积分 / 福利次数流水',
    user_activity: '用户活动记录',
    announcement_reads: '公告已读记录',
    rejected_prompt_plaza: '提示词广场被拒绝记录',
  }
  return labels[type] || type
}

function Logs(p: { data: AdminListActionLogsResponse | null; targetType: string; targetId: string; setTargetType: (v: string) => void; setTargetId: (v: string) => void; search: () => void; page: number; setPage: (v: number) => void; open: (item: AdminActionLogItem) => void }) {
  return <Panel title="网站日志" desc="展示全站用户行为和管理员操作，包括生图任务、积分流水、福利次数流水、后台操作等。" actions={<div className="flex flex-wrap gap-2"><LabeledInput label="" value={p.targetType} onChange={p.setTargetType} placeholder="对象类型，可空" /><LabeledInput label="" value={p.targetId} onChange={p.setTargetId} placeholder="对象编号，可空" /><Button onClick={p.search}>搜索</Button></div>}><ResponsiveTable headers={['ID','操作人','日志类型','操作内容','操作对象','简略信息','创建时间','详情']} rows={(p.data?.items ?? []).map(item => [<Mono>{item.id}</Mono>, <span>{item.actorLabel || (item.userId ? `用户 #${item.userId}` : `管理员 #${item.adminId}`)}</span>, siteLogTypeLabel(item), item.title || logActionLabel(item.action), logTargetLabel(item), clip(item.description || item.remark, 64) || '-', date(item.createdAt), <Button small onClick={() => p.open(item)}>详情</Button>])} empty="暂无日志" /><Pagination page={p.page} total={p.data?.total ?? 0} pageSize={p.data?.pageSize ?? PAGE_SIZE} onChange={p.setPage} /></Panel>
}

function AllModals(props: { state: Modal; setState: (m: Modal) => void; busy: Busy; categories: ProductCategoryItem[]; onSavePrompt: (item: PromptPlazaItem | null, payload: AdminPromptPlazaPayload) => void; onReviewPrompt: (item: PromptPlazaItem, status: 'approved' | 'rejected', reason?: string) => void; onDeletePrompt: (item: PromptPlazaItem, reason?: string) => void; onSavePackage: (item: AdminPackageItem | null, payload: AdminPackagePayload) => void; onSaveProduct: (item: ProductItem | null, payload: AdminProductPayload) => void; onSaveCategory: (item: ProductCategoryItem | null, payload: AdminProductCategoryPayload) => void; onSaveProviderKey: (item: AdminProviderKeyItem | null, payload: AdminProviderKeyPayload) => void; onSaveAnnouncement: (item: AnnouncementItem | null, payload: AdminAnnouncementPayload) => void; onSavePoints: () => void; onOpenUserLogs: (user: AdminUserItem, page: number) => void }) {
  const s = props.state
  if (s.type === 'none') return null
  return createPortal(<div className="fixed inset-0 z-[1200] flex items-center justify-center bg-slate-900/35 p-4 backdrop-blur-sm"><div className="max-h-[88dvh] w-full max-w-3xl overflow-hidden rounded-3xl bg-white shadow-2xl"><div className="flex items-center justify-between border-b px-5 py-4"><h2 className="text-lg font-bold">{modalTitle(s)}</h2><button className="icon-btn" onClick={() => props.setState({ type: 'none' })}><X className="h-4 w-4" /></button></div><div className="max-h-[72dvh] overflow-y-auto p-5">{renderModal(s, props)}</div></div></div>, document.body)
}
function renderModal(s: Modal, p: Parameters<typeof AllModals>[0]): ReactNode {
  if (s.type === 'confirm') return <div className="space-y-5"><div className="text-slate-600">{s.data.content}</div><div className="flex justify-end gap-2"><Button onClick={() => p.setState({ type: 'none' })}>取消</Button><Button danger={s.data.danger} primary={!s.data.danger} onClick={s.data.onConfirm}>{s.data.confirmText || '确定'}</Button></div></div>
  if (s.type === 'task') return <Detail rows={[["任务ID", s.item.id], ["用户", s.item.username || s.item.userId], ["状态", s.item.status], ["提示词", s.item.prompt], ["错误信息", s.item.errorMessage], ["图片", <TaskImages item={s.item} />], ["请求参数", pretty(s.item.requestParams)], ["创建时间", date(s.item.createdAt)], ["更新时间", date(s.item.updatedAt)]]} />
  if (s.type === 'redeemCode') return <RedeemCodeDetail item={s.item} />
  if (s.type === 'prompt') return <PromptForm item={s.item} save={payload => p.onSavePrompt(s.item, payload)} />
  if (s.type === 'promptReason') return <ReasonForm state={s} setState={p.setState} submit={reason => s.action === 'reject' ? p.onReviewPrompt(s.item, 'rejected', reason) : p.onDeletePrompt(s.item, reason)} />
  if (s.type === 'package') return <PackageForm item={s.item} save={payload => p.onSavePackage(s.item, payload)} />
  if (s.type === 'product') return <ProductForm item={s.item} categories={p.categories} save={payload => p.onSaveProduct(s.item, payload)} />
  if (s.type === 'category') return <CategoryForm item={s.item} save={payload => p.onSaveCategory(s.item, payload)} />
  if (s.type === 'providerKey') return <ProviderKeyForm item={s.item} save={payload => p.onSaveProviderKey(s.item, payload)} />
  if (s.type === 'announcement') return <AnnouncementForm item={s.item} save={payload => p.onSaveAnnouncement(s.item, payload)} />
  if (s.type === 'points') return <PointsForm state={s} setState={p.setState} save={p.onSavePoints} />
  if (s.type === 'userLogs') return <UserLogs state={s} page={(page) => p.onOpenUserLogs(s.user, page)} />
  if (s.type === 'log') return <Detail rows={[["日志ID", s.item.id], ["操作人", s.item.actorLabel || (s.item.userId ? `用户 #${s.item.userId}` : `管理员 #${s.item.adminId}`)], ["日志类型", siteLogTypeLabel(s.item)], ["操作内容", s.item.title || logActionLabel(s.item.action)], ["操作对象", logTargetLabel(s.item)], ["简略信息", s.item.description || s.item.remark || '-'], ["状态", s.item.status ? statusText(s.item.status) : '-'], ["变更前", <PreText value={pretty(s.item.beforeJson)} />], ["详情", <PreText value={pretty(s.item.afterJson)} />], ["时间", date(s.item.createdAt)]]} />
  return null
}

function RedeemCodeDetail({ item }: { item: AdminRedeemCodeItem }) {
  return <Detail rows={[["兑换码", <Mono>{item.code}</Mono>], ["积分", item.pointsAmount], ["福利次数", item.creditsAmount], ["状态", <StatusPill value={item.status} />], ["使用者", item.usedBy || '-'], ["使用时间", date(item.usedAt)], ["创建时间", date(item.createdAt)], ["批次", item.batchNo || '-'], ["备注", item.note || '-'], ["创建者", item.createdBy || '-']]} />
}

function PromptForm({ item, save }: { item: PromptPlazaItem | null; save: (payload: AdminPromptPlazaPayload) => void }) { const [d, setD] = useState({ title: item?.title || '', prompt: item?.prompt || '', previewUrl: item?.previewUrl || '', tags: (item?.tags || []).join(','), author: item?.author || '', status: item?.status || 'approved' }); return <Form onSave={() => save({ title: d.title, prompt: d.prompt, previewUrl: d.previewUrl || undefined, tags: d.tags.split(/[,，\n]/).map(x => x.trim()).filter(Boolean).slice(0, 4), author: d.author || undefined, status: d.status as any })}><LabeledInput label="标题" value={d.title} onChange={v => setD({ ...d, title: v })} /><TextAreaInput label="提示词" value={d.prompt} onChange={v => setD({ ...d, prompt: v })} rows={6} /><UploadField label="预览图" value={d.previewUrl} onChange={v => setD({ ...d, previewUrl: v })} upload={api.adminUploadPromptPlazaImage} /><LabeledInput label="上传人" value={d.author} onChange={v => setD({ ...d, author: v })} /><TextAreaInput label="标签，逗号或换行分隔，最多4个" value={d.tags} onChange={v => setD({ ...d, tags: v })} /><Select value={d.status} onChange={v => setD({ ...d, status: v as PromptPlazaItem['status'] })} options={[["approved","已通过"],["pending","待审核"],["rejected","已驳回"]]} /></Form> }
function PackageForm({ item, save }: { item: AdminPackageItem | null; save: (payload: AdminPackagePayload) => void }) { const [d, setD] = useState({ code: item?.code || '', name: item?.name || '', packageType: item?.packageType || 'recharge', creditsAmount: String(item?.creditsAmount ?? 0), pointsCost: String(item?.pointsCost ?? 0), amountCents: String(item?.amountCents ?? 0), bonusPoints: String(item?.bonusPoints ?? 0), productUrl: item?.productUrl || '', enabled: item?.enabled ?? true, sortOrder: String(item?.sortOrder ?? 0), note: item?.note || '' }); return <Form onSave={() => save({ code: d.code || undefined, name: d.name, packageType: d.packageType as any, creditsAmount: +d.creditsAmount || 0, pointsCost: +d.pointsCost || 0, amountCents: +d.amountCents || 0, bonusPoints: +d.bonusPoints || 0, productUrl: d.productUrl || undefined, enabled: d.enabled, sortOrder: +d.sortOrder || 0, note: d.note || undefined })}><LabeledInput label="编码" value={d.code} onChange={v => setD({ ...d, code: v })} /><LabeledInput label="名称" value={d.name} onChange={v => setD({ ...d, name: v })} /><LabeledInput label="福利次数" value={d.creditsAmount} onChange={v => setD({ ...d, creditsAmount: v })} /><LabeledInput label="积分价格" value={d.pointsCost} onChange={v => setD({ ...d, pointsCost: v })} /><LabeledInput label="金额，分" value={d.amountCents} onChange={v => setD({ ...d, amountCents: v })} /><LabeledInput label="赠送积分" value={d.bonusPoints} onChange={v => setD({ ...d, bonusPoints: v })} /><LabeledInput label="外部链接" value={d.productUrl} onChange={v => setD({ ...d, productUrl: v })} /><LabeledInput label="排序" value={d.sortOrder} onChange={v => setD({ ...d, sortOrder: v })} /><TextAreaInput label="备注" value={d.note} onChange={v => setD({ ...d, note: v })} /><Toggle label="启用" checked={d.enabled} onChange={v => setD({ ...d, enabled: v })} /></Form> }
function ProductForm({ item, categories, save }: { item: ProductItem | null; categories: ProductCategoryItem[]; save: (payload: AdminProductPayload) => void }) {
  const [d, setD] = useState({
    name: item?.name || '',
    shortDescription: item?.shortDescription || '',
    description: item?.description || '',
    previewUrl: item?.previewUrl || '',
    amountCents: String(item?.amountCents ?? ''),
    productUrl: item?.productUrl || '',
    categoryIds: item?.categories?.map(c => c.id) || [],
    sortOrder: String(item?.sortOrder ?? 0),
    enabled: item?.enabled ?? true,
  })
  const toggle = (id: string) => setD({ ...d, categoryIds: d.categoryIds.includes(id) ? d.categoryIds.filter(x => x !== id) : [...d.categoryIds, id] })
  const submit = () => {
    const amountCents = Math.round(Number(d.amountCents) || 0)
    const payload: AdminProductPayload = {
      name: d.name,
      shortDescription: d.shortDescription || undefined,
      description: d.description || undefined,
      previewUrl: d.previewUrl || undefined,
      amountCents,
      productUrl: d.productUrl || undefined,
      categoryIds: d.categoryIds,
      sortOrder: +d.sortOrder || 0,
      enabled: d.enabled,
    }
    save(payload)
  }
  return <Form onSave={submit}><LabeledInput label="商品名称" value={d.name} onChange={v => setD({ ...d, name: v })} /><LabeledInput label="商品价格，分（必填）" value={d.amountCents} onChange={v => setD({ ...d, amountCents: v })} /><TextAreaInput label="简短说明，可空" value={d.shortDescription} onChange={v => setD({ ...d, shortDescription: v })} /><TextAreaInput label="详细说明，可空" value={d.description} onChange={v => setD({ ...d, description: v })} rows={6} /><UploadField label="缩略图" value={d.previewUrl} onChange={v => setD({ ...d, previewUrl: v })} upload={api.adminUploadProductImage} /><LabeledInput label="外部购买链接，可空" value={d.productUrl} onChange={v => setD({ ...d, productUrl: v })} /><LabeledInput label="排序" value={d.sortOrder} onChange={v => setD({ ...d, sortOrder: v })} /><div><div className="mb-2 text-sm font-medium text-slate-600">商品分类，可多选</div><div className="flex flex-wrap gap-2">{categories.map(c => <button type="button" key={c.id} onClick={() => toggle(c.id)} className={d.categoryIds.includes(c.id) ? 'btn-primary' : 'btn'}>{c.name}</button>)}</div></div><Toggle label="启用" checked={d.enabled} onChange={v => setD({ ...d, enabled: v })} /></Form>
}
function CategoryForm({ item, save }: { item: ProductCategoryItem | null; save: (payload: AdminProductCategoryPayload) => void }) { const [d, setD] = useState({ name: item?.name || '', description: item?.description || '', sortOrder: String(item?.sortOrder ?? 0), enabled: item?.enabled ?? true }); return <Form onSave={() => save({ name: d.name, description: d.description || undefined, sortOrder: +d.sortOrder || 0, enabled: d.enabled })}><LabeledInput label="分类名称" value={d.name} onChange={v => setD({ ...d, name: v })} /><TextAreaInput label="分类描述" value={d.description} onChange={v => setD({ ...d, description: v })} /><LabeledInput label="排序" value={d.sortOrder} onChange={v => setD({ ...d, sortOrder: v })} /><Toggle label="启用" checked={d.enabled} onChange={v => setD({ ...d, enabled: v })} /></Form> }
function ProviderKeyForm({ item, save }: { item: AdminProviderKeyItem | null; save: (payload: AdminProviderKeyPayload) => void }) { const [d, setD] = useState({ name: item?.name || '', provider: item?.provider || 'mentalout', batchServiceUrl: item?.batchServiceUrl || 'https://image.mentalout.top', apiBaseUrl: item?.apiBaseUrl || 'https://chunfeng.mentalout.top', apiKey: '', status: (item?.status as AdminProviderKeyPayload['status']) || 'active', priority: String(item?.priority ?? 0), autoSwitchEnabled: item?.autoSwitchEnabled ?? true }); return <Form onSave={() => save({ name: d.name, provider: d.provider, batchServiceUrl: d.batchServiceUrl, apiBaseUrl: d.apiBaseUrl, apiKey: d.apiKey || undefined, status: d.status, priority: +d.priority || 0, autoSwitchEnabled: d.autoSwitchEnabled })}><LabeledInput label="名称" value={d.name} onChange={v => setD({ ...d, name: v })} /><LabeledInput label="provider" value={d.provider} onChange={v => setD({ ...d, provider: v })} /><LabeledInput label="batchServiceUrl" value={d.batchServiceUrl} onChange={v => setD({ ...d, batchServiceUrl: v })} /><LabeledInput label="apiBaseUrl" value={d.apiBaseUrl} onChange={v => setD({ ...d, apiBaseUrl: v })} /><LabeledInput label={item ? 'apiKey，留空不修改' : 'apiKey'} value={d.apiKey} onChange={v => setD({ ...d, apiKey: v })} /><Select value={d.status || 'active'} onChange={v => setD({ ...d, status: v as any })} options={[["active","active"],["disabled","disabled"],["error","error"],["low_balance","low_balance"]]} /><LabeledInput label="优先级" value={d.priority} onChange={v => setD({ ...d, priority: v })} /><Toggle label="自动切换" checked={d.autoSwitchEnabled} onChange={v => setD({ ...d, autoSwitchEnabled: v })} /></Form> }

function AnnouncementForm({ item, save }: { item: AnnouncementItem | null; save: (payload: AdminAnnouncementPayload) => void }) { const [d, setD] = useState({ title: item?.title || '', content: item?.content || '', imageUrl: item?.imageUrl || '', kind: item?.kind || 'normal', targetUserId: item?.targetUserId || '', enabled: item?.enabled ?? true, sortOrder: String(item?.sortOrder ?? 0) }); return <Form onSave={() => save({ title: d.title, content: d.content, imageUrl: d.imageUrl || undefined, kind: d.kind as any, targetUserId: d.kind === 'personal' ? d.targetUserId || undefined : undefined, enabled: d.enabled, sortOrder: +d.sortOrder || 0 })}><LabeledInput label="标题" value={d.title} onChange={v => setD({ ...d, title: v })} /><Select value={d.kind} onChange={v => setD({ ...d, kind: v as AnnouncementItem['kind'] })} options={[["normal","普通公告"],["pinned","置顶公告"],["personal","个人公告"]]} />{d.kind === 'personal' ? <LabeledInput label="目标用户ID" value={d.targetUserId} onChange={v => setD({ ...d, targetUserId: v })} /> : null}<UploadField label="公告图片" value={d.imageUrl} onChange={v => setD({ ...d, imageUrl: v })} upload={f => api.adminUploadAnnouncementImage(f, 'announcement')} /><LabeledInput label="排序" value={d.sortOrder} onChange={v => setD({ ...d, sortOrder: v })} /><TextAreaInput label="内容" value={d.content} onChange={v => setD({ ...d, content: v })} rows={8} /><Toggle label="启用" checked={d.enabled} onChange={v => setD({ ...d, enabled: v })} /></Form> }
function PointsForm({ state, setState, save }: { state: Extract<Modal, { type: 'points' }>; setState: (m: Modal) => void; save: () => void }) { return <Form onSave={save}><Detail rows={[["用户", `${state.user.username} / ${state.user.id}`], ["当前积分", state.user.pointsBalance]]} /><LabeledInput label="调整积分，可正可负" value={state.delta} onChange={v => setState({ ...state, delta: v })} /><TextAreaInput label="备注" value={state.remark} onChange={v => setState({ ...state, remark: v })} /></Form> }
function ReasonForm({ state, setState, submit }: { state: Extract<Modal, { type: 'promptReason' }>; setState: (m: Modal) => void; submit: (reason: string) => void }) { return <div className="space-y-4"><div className="rounded-2xl bg-amber-50 p-3 text-sm text-amber-700">请填写原因。该原因会作为个人公告发送给上传用户。</div><TextAreaInput label={state.action === 'reject' ? '驳回理由' : '删除原因'} value={state.reason} onChange={v => setState({ ...state, reason: v })} /><Detail rows={[["提示词标题", state.item.title], ["提示词内容", state.item.prompt]]} /><div className="flex justify-end gap-2"><Button onClick={() => setState({ type: 'none' })}>取消</Button><Button danger onClick={() => submit(state.reason)}>确定</Button></div></div> }
function UserLogs({ state, page }: { state: Extract<Modal, { type: 'userLogs' }>; page: (p: number) => void }) { if (state.loading) return <Empty text="日志加载中..." />; if (state.error) return <Empty text={state.error} />; return <div><ResponsiveTable headers={['类型','标题','描述','变化','时间']} rows={(state.response?.items ?? []).map(i => [i.recordType, i.title, i.description, `${i.deltaPoints ?? 0}积分 / ${i.deltaCredits ?? 0}次数`, date(i.createdAt)])} empty="暂无用户日志" />{state.response ? <Pagination page={state.page} total={state.response.total} pageSize={state.response.pageSize} onChange={page} /> : null}</div> }
function TaskImages({ item }: { item: AdminImageTaskItem }) {
  const images = taskImagePairs(item)
  if (!images.length) return <span className="text-slate-400">暂无图片</span>
  return <div className="grid gap-3 sm:grid-cols-2">{images.map((image, index) => <div key={`${image.original}-${index}`} className="overflow-hidden rounded-2xl border border-slate-100 bg-white shadow-sm"><img src={image.thumb || image.original} alt={`生成图片 ${index + 1}`} className="h-44 w-full object-cover" /><div className="flex items-center justify-between gap-2 px-3 py-2 text-xs text-slate-500"><span>图片 {index + 1}，默认显示缩略图</span><a href={image.original} target="_blank" rel="noreferrer" className="rounded-full border border-slate-200 px-3 py-1 font-semibold text-indigo-600 hover:bg-indigo-50">查看原图</a></div></div>)}</div>
}
function Form({ children, onSave }: { children: ReactNode; onSave: () => void }) { return <div className="space-y-4">{children}<div className="flex justify-end gap-2"><Button primary onClick={onSave}>保存</Button></div></div> }
function UploadField({ label, value, onChange, upload }: { label: string; value: string; onChange: (v: string) => void; upload: (file: File) => Promise<{ imageUrl: string }> }) { const [loading, setLoading] = useState(false); return <div><LabeledInput label={label} value={value} onChange={onChange} /><label className="mt-2 inline-flex cursor-pointer items-center gap-2 rounded-full border border-slate-200 px-3 py-1.5 text-sm hover:bg-slate-50">{loading ? <LoaderCircle className="h-4 w-4 animate-spin" /> : null}上传本地图片<input className="hidden" type="file" accept="image/*" onChange={async e => { const file = e.target.files?.[0]; if (!file) return; setLoading(true); try { const r = await upload(file); onChange(r.imageUrl) } finally { setLoading(false); e.target.value = '' } }} /></label></div> }

function Panel({ title, desc, actions, children }: { title: string; desc?: string; actions?: ReactNode; children: ReactNode }) { return <section className="admin-panel rounded-[28px] border border-white/80 bg-white/90 p-4 shadow-sm sm:p-5"><div className="mb-4 flex flex-wrap items-start justify-between gap-3"><div><h2 className="text-lg font-bold">{title}</h2>{desc ? <p className="mt-1 text-sm text-slate-500">{desc}</p> : null}</div>{actions}</div>{children}</section> }
function MetricCard({ label, value, sub }: { label: string; value: ReactNode; sub?: ReactNode }) { return <div className="admin-metric rounded-[26px] bg-white/90 p-4 shadow-sm sm:p-5"><div className="text-sm text-slate-500">{label}</div><div className="mt-3 text-2xl font-bold">{value}</div>{sub ? <div className="mt-2 text-sm text-slate-500">{sub}</div> : null}</div> }
function OverviewCard({ icon, label, value, sub, aside }: { icon: ReactNode; label: string; value: ReactNode; sub?: ReactNode; aside?: ReactNode }) { return <div className="overview-card rounded-[26px] bg-white/90 p-4 shadow-sm sm:p-5"><div className="flex items-start justify-between gap-3"><div className="text-emerald-500">{icon}</div><div className="text-sm text-slate-500">{label}</div></div><div className="mt-4 text-2xl font-black tracking-tight text-slate-950">{value}</div><div className="mt-2 flex items-end justify-between gap-3 text-sm"><span className="text-slate-500">{sub}</span>{aside ? <span className="font-bold text-emerald-600">{aside}</span> : null}</div></div> }
function RedeemSummary({ label, value, sub }: { label: string; value: ReactNode; sub?: ReactNode }) { return <div className="overview-redeem-card rounded-[22px] bg-white/90 p-4 shadow-sm"><div className="text-sm text-slate-500">{label}</div><div className="mt-2 flex items-end justify-between gap-3"><span className="text-2xl font-black text-slate-800">{value}</span>{sub ? <span className="text-sm font-semibold text-emerald-600">{sub}</span> : null}</div></div> }
function TrendBar({ value, max, color }: { value: number; max: number; color: string }) { return <div className="flex h-full w-4 items-end justify-center"><div className={`relative w-3 rounded-full ${color}`} style={{ height: value ? Math.max(6, (value / max) * 190) : 3 }}><span className="absolute -top-5 left-1/2 -translate-x-1/2 text-xs font-bold text-slate-800">{value}</span></div></div> }
function ResponsiveTable({ headers, rows, empty }: { headers: string[]; rows: ReactNode[][]; empty: string }) { return <div className="admin-table-wrap overflow-hidden rounded-3xl border border-slate-100 bg-white"><div className="overflow-x-auto"><table className="admin-table min-w-full text-left text-sm"><thead className="bg-slate-50 text-slate-500"><tr>{headers.map(h => <th key={h} className="whitespace-nowrap px-4 py-3 font-semibold">{h}</th>)}</tr></thead><tbody className="divide-y divide-slate-100">{rows.length ? rows.map((row, i) => <tr key={i}>{row.map((cell, j) => <td key={j} className="whitespace-nowrap px-4 py-3 align-middle text-slate-700">{cell}</td>)}</tr>) : <tr><td colSpan={headers.length} className="p-8"><Empty text={empty} /></td></tr>}</tbody></table></div></div> }
function Pagination({ page, total, pageSize, onChange }: { page: number; total: number; pageSize: number; onChange: (p: number) => void }) { const pages = Math.max(1, Math.ceil(total / Math.max(1, pageSize))); return <div className="mt-4 flex items-center justify-between text-sm text-slate-500"><span>共 {total} 条 / 第 {page} 页</span><div className="flex gap-2"><Button disabled={page <= 1} onClick={() => onChange(page - 1)}>上一页</Button><Button disabled={page >= pages} onClick={() => onChange(page + 1)}>下一页</Button></div></div> }
function StatusPill({ value }: { value: string }) { const v = statusText(value); const green = ['success','approved','active','启用','正常','使用中','可用','未使用'].includes(value) || ['成功','已通过','启用','正常','使用中','可用','未使用'].includes(v); const red = ['failed','rejected','disabled','error','禁用','停用'].includes(value) || ['失败','已驳回','禁用','停用'].includes(v); return <span className={`rounded-lg px-2 py-1 text-xs font-semibold ${green ? 'bg-emerald-100 text-emerald-700' : red ? 'bg-rose-100 text-rose-700' : 'bg-slate-100 text-slate-600'}`}>{v}</span> }
function Button({ children, onClick, primary, danger, disabled, loading, small }: { children: ReactNode; onClick?: () => void; primary?: boolean; danger?: boolean; disabled?: boolean; loading?: boolean; small?: boolean }) { return <button type="button" disabled={disabled || loading} onClick={onClick} className={`${primary ? 'btn-primary' : danger ? 'btn-danger' : 'btn'} ${small ? 'px-3 py-1.5 text-xs' : ''} disabled:opacity-50`}>{loading ? <LoaderCircle className="h-4 w-4 animate-spin" /> : null}{children}</button> }
function Filter({ keyword, setKeyword, placeholder, select, setSelect, options, search }: { keyword: string; setKeyword: (v: string) => void; placeholder: string; select: string; setSelect: (v: string) => void; options: Array<[string, string]>; search: () => void }) { return <div className="flex flex-wrap gap-2"><Select value={select} onChange={setSelect} options={options} /><div className="relative"><Search className="absolute left-3 top-1/2 h-4 w-4 -translate-y-1/2 text-slate-400" /><input className="input pl-9" value={keyword} onChange={e => setKeyword(e.target.value)} placeholder={placeholder} /></div><Button primary onClick={search}>搜索</Button></div> }
function Select({ value, onChange, options }: { value: string; onChange: (v: string) => void; options: Array<[string, string]> }) { return <select className="input" value={value} onChange={e => onChange(e.target.value)}>{options.map(([v, label]) => <option key={v} value={v}>{label}</option>)}</select> }
function LabeledInput({ label, value, onChange, type = 'text', placeholder }: { label: string; value: string; onChange: (v: string) => void; type?: string; placeholder?: string }) { return <label className="block"><span className="mb-1 block text-sm font-medium text-slate-600">{label}</span><input className="input w-full" type={type} value={value} placeholder={placeholder} onChange={e => onChange(e.target.value)} /></label> }
function TextAreaInput({ label, value, onChange, rows = 4 }: { label: string; value: string; onChange: (v: string) => void; rows?: number }) { return <label className="block"><span className="mb-1 block text-sm font-medium text-slate-600">{label}</span><textarea className="input min-h-28 w-full resize-y" rows={rows} value={value} onChange={e => onChange(e.target.value)} /></label> }
function NumberField({ label, value, onChange }: { label: string; value: number; onChange: (v: number) => void }) { return <LabeledInput label={label} value={String(value)} onChange={v => onChange(Number(v) || 0)} /> }
function Toggle({ label, checked, onChange }: { label: string; checked: boolean; onChange: (v: boolean) => void }) { return <label className="flex items-center justify-between rounded-2xl border border-slate-100 p-3 text-sm"><span>{label}</span><input type="checkbox" checked={checked} onChange={e => onChange(e.target.checked)} /></label> }
function Empty({ text }: { text: string }) { return <div className="w-full py-8 text-center text-sm text-slate-400">{text}</div> }
function Mono({ children }: { children: ReactNode }) { return <span className="font-mono text-xs">{children}</span> }
function Detail({ rows }: { rows: Array<[string, ReactNode]> }) { return <dl className="space-y-3">{rows.filter(([, v]) => v !== undefined && v !== null && v !== '').map(([k, v]) => <div key={k} className="rounded-2xl bg-slate-50 p-3"><dt className="text-xs font-semibold text-slate-500">{k}</dt><dd className="mt-1 whitespace-pre-wrap break-words text-sm text-slate-700">{v}</dd></div>)}</dl> }
function PreText({ value }: { value: string }) { return value ? <pre className="max-h-72 overflow-auto rounded-xl bg-slate-900 p-3 text-xs leading-5 text-slate-100">{value}</pre> : <span>-</span> }

function modalTitle(s: Modal): string {
  switch (s.type) {
    case 'confirm': return s.data.title
    case 'task': return '任务详情'
    case 'redeemCode': return '兑换码详情'
    case 'prompt': return s.item ? '编辑提示词' : '新建提示词'
    case 'promptReason': return s.action === 'reject' ? '驳回提示词' : '删除提示词'
    case 'package': return s.item ? '编辑商品套餐' : '新建商品套餐'
    case 'product': return s.item ? '编辑商品' : '新建商品'
    case 'category': return s.item ? '编辑商品分类' : '新建商品分类'
    case 'providerKey': return s.item ? '编辑 API Key' : '新建 API Key'
    case 'announcement': return s.item ? '编辑公告' : '新建公告'
    case 'points': return '调整积分'
    case 'userLogs': return '用户日志'
    case 'log': return '日志详情'
    default: return ''
  }
}
function num(v: unknown): string { const n = Number(v ?? 0); return Number.isFinite(n) ? n.toLocaleString() : '0' }
function date(v?: string | null): string { if (!v) return '-'; const d = new Date(v); return Number.isNaN(d.getTime()) ? v : d.toLocaleString('zh-CN', { hour12: false }) }
function money(cents: number): string { return `${(Number(cents || 0) / 100).toFixed(2)} 元` }
function clip(v: unknown, len = 24): string { const s = String(v || ''); return s.length > len ? `${s.slice(0, len)}...` : s }
function pretty(v: unknown): string { if (!v) return ''; try { return JSON.stringify(typeof v === 'string' ? JSON.parse(v) : v, null, 2) } catch { return String(v) } }
function kindLabel(kind: AnnouncementItem['kind']): string { if (kind === 'pinned') return '置顶'; if (kind === 'personal') return '个人公告'; return '普通' }
function statusText(v: string): string { const map: Record<string, string> = { success: '成功', failed: '失败', generating: '生成中', running: '进行中', approved: '已通过', rejected: '已驳回', pending: '待审核', active: '可用', used: '已使用', disabled: '已禁用', expired: '已过期', low_balance: '余额低', error: '异常' }; return map[v] || v }
function logActionLabel(action: string): string {
  const map: Record<string, string> = {
    'product.create': '新建商品',
    'product.update': '修改商品',
    'product.delete': '删除商品',
    'product_category.create': '新建商品分类',
    'product_category.update': '修改商品分类',
    'product_category.delete': '删除商品分类',
    'billing_package.create': '新建商品套餐',
    'billing_package.update': '修改商品套餐',
    'billing_package.delete': '删除商品套餐',
    'prompt_plaza.review': '审核提示词',
    'prompt_plaza.create': '新建提示词',
    'prompt_plaza.update': '修改提示词',
    'prompt_plaza.delete': '删除提示词',
    'redeem_code.create': '生成兑换码',
    'redeem_code.delete': '删除兑换码',
    'announcement.create': '新建公告',
    'announcement.update': '修改公告',
    'announcement.delete': '删除公告',
    'provider_key.create': '新建 API Key',
    'provider_key.update': '修改 API Key',
    'provider_key.delete': '删除 API Key',
    'user.status': '修改用户状态',
    'user.points_adjust': '调整用户积分',
  }
  return map[action] || action.replace(/\./g, ' / ')
}
function siteLogTypeLabel(item: AdminActionLogItem): string {
  const type = item.recordType || item.targetType || ''
  const map: Record<string, string> = {
    admin_action: '后台操作',
    points_log: '积分流水',
    credit_log: '福利次数流水',
    image_task: '生图任务',
  }
  return map[type] || type || '-'
}
function logTargetLabel(item: AdminActionLogItem): string {
  if (!item.targetType && !item.targetId) return '-'
  const typeLabel: Record<string, string> = {
    product: '商品',
    product_category: '商品分类',
    billing_package: '商品套餐',
    prompt_plaza: '提示词广场',
    redeem_code: '兑换码',
    announcement: '公告',
    provider_key: 'API Key',
    user: '用户',
    image_task: '生图任务',
    order: '订单',
    redeem: '兑换',
    checkin: '签到',
    invite: '邀请',
  }
  const label = typeLabel[item.targetType] || item.targetType || '对象'
  return item.targetId ? `${label} #${item.targetId}` : label
}
function paramSummary(raw?: string): string { if (!raw) return '-'; try { const p = JSON.parse(raw); return [p.size, p.count ? `${p.count}张` : '', p.quality, p.outputFormat].filter(Boolean).join(' / ') || '-' } catch { return '-' } }
function taskNo(id: string): string { const s = String(id || '').trim(); const m = s.match(/\d+$/); return `#${m ? m[0] : clip(s, 10)}` }
function taskSource(item: AdminImageTaskItem): string { return (item.creditSource || item.provider || '-').toString() }
function durationText(createdAt?: string | null, updatedAt?: string | null): string { if (!createdAt || !updatedAt) return '-'; const start = new Date(createdAt).getTime(); const end = new Date(updatedAt).getTime(); if (!Number.isFinite(start) || !Number.isFinite(end) || end < start) return '-'; const seconds = Math.round((end - start) / 1000); if (seconds < 60) return `${seconds}s`; const minutes = Math.floor(seconds / 60); const rest = seconds % 60; return rest ? `${minutes}m${rest}s` : `${minutes}m` }
function taskImagePairs(item: AdminImageTaskItem): Array<{ original: string; thumb?: string }> {
  const originals = uniqueImageUrls([...(item.imageUrls || []), item.imageUrl])
  const thumbs = uniqueImageUrls([...(item.thumbnailUrls || []), item.thumbnailUrl])
  const thumbByKey = new Map(thumbs.map(url => [imageDedupeKey(url), url]))
  const source = originals.length ? originals : thumbs
  return source.map(original => ({ original, thumb: thumbByKey.get(imageDedupeKey(original)) })).filter(image => image.original)
}
function normalizedTrendPoints(trend: AdminStatsResponse['taskTrend24h'] | undefined, stats: AdminStatsResponse | null): Array<{ label: string; success: number; failed: number }> {
  const points = (trend ?? []).filter(point => point && (Number(point.success) > 0 || Number(point.failed) > 0))
  if (points.length) return points
  const success = Number(stats?.tasksSuccess ?? 0)
  const failed = Number(stats?.tasksFailed ?? 0)
  if (success > 0 || failed > 0) return [{ label: '历史累计', success, failed }]
  return trend ?? []
}
function uniqueImageUrls(values: Array<string | undefined>): string[] {
  const seen = new Set<string>()
  const urls: string[] = []
  for (const value of values) {
    const url = String(value || '').trim()
    if (!url) continue
    const key = imageDedupeKey(url)
    if (seen.has(key)) continue
    seen.add(key)
    urls.push(url)
  }
  return urls
}
function imageDedupeKey(value: string): string {
  let pathValue = value
  try { pathValue = new URL(value, 'http://local.invalid').pathname } catch {}
  pathValue = pathValue.replace(/\/thumbs\//g, '/').replace(/\\/g, '/')
  return pathValue.replace(/\.(jpe?g|png|webp|gif|bmp|avif)$/i, '')
}
function formatBytes(v: number): string { if (!v) return '0 B'; if (v < 1024) return `${v} B`; if (v < 1024 * 1024) return `${(v / 1024).toFixed(1)} KB`; return `${(v / 1024 / 1024).toFixed(1)} MB` }
function toNumbers<T extends Record<string, unknown>>(obj: T): T { const out: Record<string, unknown> = { ...obj }; for (const key of Object.keys(out)) { const value = out[key]; if (typeof value === 'string' && value.trim() !== '' && !Number.isNaN(Number(value))) out[key] = Number(value) } return out as T }
async function copyText(text: string) { try { await navigator.clipboard.writeText(text) } catch {} }

const adminStyle = document.createElement('style')
adminStyle.textContent = `.admin-shell{height:100vh;overflow:hidden}.admin-layout{height:100vh;overflow:hidden}.admin-sidebar{height:100vh;flex:0 0 284px;background:linear-gradient(180deg,#eef1ff 0%,#f8fbff 100%);box-shadow:18px 0 45px rgba(99,102,241,.08)}.admin-content{height:100vh;display:flex;flex-direction:column;overflow:hidden;min-width:0}.admin-header{flex:0 0 auto;min-height:76px}.admin-main{flex:1;overflow:auto;max-width:1680px;width:100%;margin:0 auto;box-sizing:border-box}.admin-panel{min-width:0;box-shadow:0 18px 48px rgba(15,23,42,.06);backdrop-filter:blur(12px);overflow:hidden}.admin-metric{box-shadow:0 16px 42px rgba(15,23,42,.055);min-height:132px}.admin-table-wrap{max-width:100%;box-shadow:0 12px 34px rgba(15,23,42,.04)}.admin-table{width:100%}.admin-table th{height:50px}.admin-table td{height:56px}.admin-nav{max-height:calc(100vh - 190px)}.redeem-layout{align-items:start;grid-template-columns:340px minmax(0,1fr)}.redeem-layout>*{min-width:0}.redeem-layout .admin-table{min-width:1040px}.redeem-layout .admin-panel{padding:1.25rem}.overview-page{position:relative}.overview-page:before{content:"";position:fixed;inset:76px 0 0 284px;pointer-events:none;background:radial-gradient(circle at 8% 8%,rgba(99,102,241,.17),transparent 31%),radial-gradient(circle at 92% 86%,rgba(20,184,166,.18),transparent 28%);z-index:-1}.overview-card{min-height:118px;box-shadow:0 20px 52px rgba(15,23,42,.055);backdrop-filter:blur(10px)}.overview-chart{box-shadow:0 24px 70px rgba(15,23,42,.07);backdrop-filter:blur(12px)}.overview-bars{box-shadow:inset 0 1px 0 rgba(148,163,184,.16)}.overview-redeem-card{min-height:94px;box-shadow:0 18px 48px rgba(15,23,42,.055)}.btn,.btn-primary,.btn-danger,.icon-btn{display:inline-flex;align-items:center;justify-content:center;gap:.45rem;border-radius:999px;font-weight:700;transition:.15s;white-space:nowrap;line-height:1}.btn{border:1px solid #e2e8f0;background:rgba(255,255,255,.86);color:#334155;padding:.62rem 1rem;box-shadow:0 10px 26px rgba(15,23,42,.035)}.btn:hover{background:white;transform:translateY(-1px)}.btn-primary{border:1px solid #6366f1;background:#6366f1;color:white;padding:.62rem 1rem;box-shadow:0 14px 30px rgba(99,102,241,.24)}.btn-primary:hover{background:#5558ef;transform:translateY(-1px)}.btn-danger{border:1px solid #fecdd3;background:#fff1f2;color:#be123c;padding:.62rem 1rem}.icon-btn{border:1px solid #e2e8f0;background:white;color:#334155;padding:.62rem;box-shadow:0 10px 24px rgba(15,23,42,.04)}.input{border:1px solid #e2e8f0;background:rgba(255,255,255,.9);border-radius:1rem;padding:.68rem .95rem;color:#1e293b;outline:none;min-height:2.75rem;box-shadow:0 10px 24px rgba(15,23,42,.025)}.input:focus{border-color:#818cf8;box-shadow:0 0 0 3px rgba(99,102,241,.12)}.field-label{display:block;font-size:.875rem;font-weight:500;color:#475569}@media (min-width:1024px){.admin-main{padding-top:1.75rem}.admin-panel{border-radius:30px}.admin-table td{padding-top:.68rem;padding-bottom:.68rem}}@media (max-width:1440px) and (min-width:1024px){.admin-sidebar{flex-basis:250px;width:250px}.overview-page:before{left:250px}.admin-main{padding-left:1rem;padding-right:1rem}.redeem-layout{grid-template-columns:310px minmax(0,1fr);gap:1rem}.admin-table th,.admin-table td{padding-left:.72rem;padding-right:.72rem}.btn,.btn-primary,.btn-danger{padding:.56rem .85rem}.input{min-height:2.55rem}}@media (max-width:1180px) and (min-width:1024px){.redeem-layout{grid-template-columns:1fr}.redeem-layout .admin-table{min-width:980px}}@media (max-width:1023px){.admin-shell,.admin-layout,.admin-content{height:auto;min-height:100vh;overflow:visible}.admin-header{position:sticky}.admin-header>div{align-items:flex-start}.admin-header>div>div:last-child{margin-left:auto;max-width:100%;justify-content:flex-end;overflow-x:auto;padding-bottom:.25rem}.admin-main{overflow:visible;padding-bottom:2rem}.admin-sidebar{height:100vh}.admin-panel{border-radius:24px}.overview-page:before{inset:70px 0 0 0}.admin-table{min-width:760px}.admin-table th,.admin-table td{height:auto}.btn,.btn-primary,.btn-danger{padding:.58rem .86rem;font-size:.875rem}.input{width:100%;min-width:0}}@media (max-width:640px){.admin-header .icon-btn{flex:0 0 auto}.admin-header>div{display:grid;grid-template-columns:auto 1fr;align-items:start}.admin-header>div>div:first-of-type{min-width:0}.admin-header>div>div:last-child{grid-column:1 / -1;margin-left:auto;justify-content:flex-end;flex-wrap:nowrap;width:100%;padding-left:2.5rem}.admin-header .btn{padding:.52rem .72rem}.admin-main{padding:.75rem}.admin-panel{padding:1rem;border-radius:22px}.admin-metric{min-height:108px}.overview-card{min-height:108px}.overview-chart{padding:1rem;border-radius:24px}.overview-bars{padding:.75rem}.overview-bars>div{min-width:620px}.overview-redeem-card{min-height:84px}.admin-table{min-width:680px}.admin-table th,.admin-table td{padding:.72rem .85rem}.redeem-layout{grid-template-columns:1fr!important}.redeem-layout .admin-table{min-width:900px}.admin-nav{max-height:calc(100vh - 170px)}}`
if (typeof document !== 'undefined' && !document.getElementById('admin-console-inline-style')) { adminStyle.id = 'admin-console-inline-style'; document.head.appendChild(adminStyle) }
